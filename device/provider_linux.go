//go:build linux

package device

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func init() {
	RegisterProvider(func() DeviceProvider { return &linuxInputProvider{} })
	RegisterProvider(func() DeviceProvider { return &linuxMIDIProvider{} })
}

// ---------------------------------------------------------------------------
// Linux input (evdev) provider — gamepads, keyboards, mice
// ---------------------------------------------------------------------------

type linuxInputProvider struct{}

func (p *linuxInputProvider) Name() string { return "evdev" }

func (p *linuxInputProvider) DeviceTypes() []DeviceType {
	return []DeviceType{TypeGamepad, TypeKeyboard, TypeMouse, TypeGenericHID, TypeHOTAS, TypeRacingWheel}
}

func (p *linuxInputProvider) Enumerate(_ context.Context) ([]Info, error) {
	data, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return nil, fmt.Errorf("read /proc/bus/input/devices: %w", err)
	}

	var devices []Info
	blocks := strings.SplitSeq(string(data), "\n\n")

	for block := range blocks {
		lines := strings.Split(block, "\n")
		var name, vendor, product, eventPath string
		var hasKey, hasAbs, hasFF bool
		isVirtual := false

		for _, line := range lines {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "N: Name="):
				name = strings.Trim(strings.TrimPrefix(line, "N: Name="), "\"")
				lower := strings.ToLower(name)
				if containsAny(lower, "virtual", "ydotool", "antimicrox", "makima", "logiops") {
					isVirtual = true
				}
			case strings.HasPrefix(line, "I: "):
				for part := range strings.FieldsSeq(line) {
					if after, ok := strings.CutPrefix(part, "Vendor="); ok {
						vendor = strings.ToLower(after)
					} else if after, ok := strings.CutPrefix(part, "Product="); ok {
						product = strings.ToLower(after)
					}
				}
			case strings.HasPrefix(line, "H: Handlers="):
				for h := range strings.FieldsSeq(strings.TrimPrefix(line, "H: Handlers=")) {
					if strings.HasPrefix(h, "event") {
						eventPath = "/dev/input/" + h
					}
				}
			case strings.HasPrefix(line, "B: KEY="):
				hasKey = true
			case strings.HasPrefix(line, "B: ABS="):
				val := strings.TrimPrefix(line, "B: ABS=")
				if len(val) > 1 {
					hasAbs = true
				}
			case strings.HasPrefix(line, "B: FF="):
				val := strings.TrimPrefix(line, "B: FF=")
				if val != "0" {
					hasFF = true
				}
			}
		}

		if isVirtual || name == "" || eventPath == "" {
			continue
		}

		// Need at least key or abs capabilities.
		if !hasKey && !hasAbs {
			continue
		}

		vendorID := parseHexUint16(vendor)
		productID := parseHexUint16(product)
		devType := ClassifyDevice(vendorID, productID, name)

		// Skip truly unknown devices (no gamepad/input characteristics).
		if devType == TypeUnknown && !hasAbs {
			continue
		}

		var caps Capabilities
		if hasKey {
			caps.Buttons = 1 // At least one
		}
		if hasAbs {
			caps.Axes = 1
		}
		if hasFF {
			caps.HasRumble = true
		}

		id := DeviceID(fmt.Sprintf("evdev:%s", eventPath))
		devices = append(devices, Info{
			ID:           id,
			Name:         name,
			Type:         devType,
			Connection:   ConnUSB, // Could be BT, but /proc doesn't distinguish easily
			VendorID:     vendorID,
			ProductID:    productID,
			Capabilities: caps,
			PlatformPath: eventPath,
			ProviderName: "evdev",
		})
	}

	return devices, nil
}

func (p *linuxInputProvider) Open(_ context.Context, id DeviceID) (DeviceConnection, error) {
	// Extract event path from ID.
	path := strings.TrimPrefix(string(id), "evdev:")
	if path == "" || !strings.HasPrefix(path, "/dev/input/") {
		return nil, fmt.Errorf("%w: invalid evdev ID: %s", ErrDeviceNotFound, id)
	}

	return &linuxEvdevConnection{
		deviceInfo: Info{ID: id, PlatformPath: path, ProviderName: "evdev"},
		path:       path,
		events:     make(chan Event, 64),
	}, nil
}

func (p *linuxInputProvider) Close() error { return nil }

// inputEvent matches struct input_event from linux/input.h (24 bytes on 64-bit).
type inputEvent struct {
	Sec  int64
	Usec int64
	Type uint16
	Code uint16
	Val  int32
}

const inputEventSize = int(unsafe.Sizeof(inputEvent{}))

// absInfo matches struct input_absinfo from linux/input.h.
type absInfo struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

// ioctl helpers for evdev.
const (
	evIOCGABS  = 0x80184540 // EVIOCGABS(0) base — add ABS code to get specific axis
	evIOCGRAB  = 0x40044590 // EVIOCGRAB
)

func evdevIoctl(fd uintptr, req uintptr, arg uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg)
	if errno != 0 {
		return errno
	}
	return nil
}

// linuxEvdevConnection reads events directly from a Linux input device.
type linuxEvdevConnection struct {
	deviceInfo Info
	path       string
	fd         *os.File
	events     chan Event
	cancel     context.CancelFunc
	alive      bool
	absInfos   map[uint16]*absInfo // cached axis info for normalization
	grabbed    bool
}

func (c *linuxEvdevConnection) Info() Info               { return c.deviceInfo }
func (c *linuxEvdevConnection) Events() <-chan Event     { return c.events }
func (c *linuxEvdevConnection) Feedback() DeviceFeedback { return nil }
func (c *linuxEvdevConnection) Alive() bool              { return c.alive }

func (c *linuxEvdevConnection) Start(ctx context.Context) error {
	f, err := os.OpenFile(c.path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", c.path, err)
	}
	c.fd = f

	// Pre-read axis info for normalization.
	c.absInfos = make(map[uint16]*absInfo)
	for code := uint16(0); code <= 0x3f; code++ {
		var ai absInfo
		req := uintptr(evIOCGABS) + uintptr(code)
		err := evdevIoctl(f.Fd(), req, uintptr(unsafe.Pointer(&ai)))
		if err == nil && (ai.Minimum != 0 || ai.Maximum != 0) {
			c.absInfos[code] = &ai
		}
	}

	ctx, c.cancel = context.WithCancel(ctx)
	c.alive = true

	go c.readLoop(ctx)
	return nil
}

func (c *linuxEvdevConnection) readLoop(ctx context.Context) {
	defer func() {
		c.alive = false
		close(c.events)
	}()

	buf := make([]byte, inputEventSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := c.fd.Read(buf)
		if err != nil {
			return // device disconnected or fd closed
		}
		if n < inputEventSize {
			continue
		}

		var ev inputEvent
		ev.Sec = int64(binary.LittleEndian.Uint64(buf[0:8]))
		ev.Usec = int64(binary.LittleEndian.Uint64(buf[8:16]))
		ev.Type = binary.LittleEndian.Uint16(buf[16:18])
		ev.Code = binary.LittleEndian.Uint16(buf[18:20])
		ev.Val = int32(binary.LittleEndian.Uint32(buf[20:24]))

		devEv := c.convertEvent(ev)
		if devEv == nil {
			continue
		}

		select {
		case c.events <- *devEv:
		case <-ctx.Done():
			return
		}
	}
}

func (c *linuxEvdevConnection) convertEvent(ev inputEvent) *Event {
	switch ev.Type {
	case evKey:
		name := evKeyName(ev.Code)
		if name == "" {
			return nil
		}
		pressed := ev.Val != 0
		val := 0.0
		if pressed {
			val = 1.0
		}
		return &Event{
			DeviceID:  c.deviceInfo.ID,
			Type:      EventButton,
			Timestamp: time.Unix(ev.Sec, ev.Usec*1000),
			Source:    name,
			Code:      ev.Code,
			Pressed:   pressed,
			Value:     val,
			RawValue:  ev.Val,
		}

	case evAbs:
		name := evAbsName(ev.Code)
		if name == "" {
			return nil
		}

		// Hat axes → EventHat
		if ev.Code >= 0x10 && ev.Code <= 0x17 {
			hatEv := &Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventHat,
				Timestamp: time.Unix(ev.Sec, ev.Usec*1000),
				Source:    name,
				RawValue:  ev.Val,
			}
			if ev.Code%2 == 0 { // even codes are X
				hatEv.HatX = int8(ev.Val)
			} else {
				hatEv.HatY = int8(ev.Val)
			}
			return hatEv
		}

		// Regular axis → normalized value
		value := c.normalizeAxis(ev.Code, ev.Val)
		return &Event{
			DeviceID:  c.deviceInfo.ID,
			Type:      EventAxis,
			Timestamp: time.Unix(ev.Sec, ev.Usec*1000),
			Source:    name,
			Value:     value,
			RawValue:  ev.Val,
		}

	case evRel:
		name := evRelName(ev.Code)
		if name == "" {
			return nil
		}
		return &Event{
			DeviceID:  c.deviceInfo.ID,
			Type:      EventAxis,
			Timestamp: time.Unix(ev.Sec, ev.Usec*1000),
			Source:    name,
			Value:     float64(ev.Val),
			RawValue:  ev.Val,
		}

	default:
		return nil // EV_SYN, EV_MSC, etc. — skip
	}
}

// normalizeAxis converts a raw axis value to -1.0..1.0 using cached absinfo.
func (c *linuxEvdevConnection) normalizeAxis(code uint16, raw int32) float64 {
	ai, ok := c.absInfos[code]
	if !ok || ai.Maximum == ai.Minimum {
		return float64(raw)
	}

	// Normalize to -1.0..1.0 for symmetric axes, 0.0..1.0 for triggers.
	min := float64(ai.Minimum)
	max := float64(ai.Maximum)
	v := float64(raw)

	if min >= 0 {
		// Unsigned axis (trigger): 0.0 to 1.0
		return (v - min) / (max - min)
	}
	// Signed axis (stick): -1.0 to 1.0
	mid := (max + min) / 2
	halfRange := (max - min) / 2
	return (v - mid) / halfRange
}

// Grab claims exclusive access to this device via EVIOCGRAB.
func (c *linuxEvdevConnection) Grab() error {
	if c.fd == nil {
		return fmt.Errorf("device not open")
	}
	if err := evdevIoctl(c.fd.Fd(), evIOCGRAB, 1); err != nil {
		return fmt.Errorf("EVIOCGRAB: %w", err)
	}
	c.grabbed = true
	return nil
}

// ReleaseGrab releases exclusive access.
func (c *linuxEvdevConnection) ReleaseGrab() error {
	if c.fd == nil || !c.grabbed {
		return nil
	}
	if err := evdevIoctl(c.fd.Fd(), evIOCGRAB, 0); err != nil {
		return fmt.Errorf("EVIOCGRAB release: %w", err)
	}
	c.grabbed = false
	return nil
}

func (c *linuxEvdevConnection) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.grabbed {
		c.ReleaseGrab()
	}
	if c.fd != nil {
		return c.fd.Close()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Linux MIDI provider — ALSA MIDI devices
// ---------------------------------------------------------------------------

type linuxMIDIProvider struct{}

func (p *linuxMIDIProvider) Name() string { return "alsa_midi" }

func (p *linuxMIDIProvider) DeviceTypes() []DeviceType {
	return []DeviceType{TypeMIDI}
}

func (p *linuxMIDIProvider) Enumerate(_ context.Context) ([]Info, error) {
	var devices []Info

	// Try amidi first.
	if devs := enumMIDIAmidi(); len(devs) > 0 {
		return devs, nil
	}

	// Fallback to /dev/snd/midiC*D* glob.
	matches, _ := filepath.Glob("/dev/snd/midiC*D*")
	for _, m := range matches {
		base := filepath.Base(m)
		var card, dev int
		fmt.Sscanf(base, "midiC%dD%d", &card, &dev)

		nameBytes, _ := os.ReadFile(fmt.Sprintf("/proc/asound/card%d/id", card))
		name := strings.TrimSpace(string(nameBytes))
		if name == "" {
			name = fmt.Sprintf("Card %d Device %d", card, dev)
		}

		id := DeviceID(fmt.Sprintf("alsa_midi:hw:%d,%d", card, dev))
		devices = append(devices, Info{
			ID:           id,
			Name:         name,
			Type:         TypeMIDI,
			Connection:   ConnUSB,
			Capabilities: Capabilities{MIDIPorts: 1},
			PlatformPath: m,
			ProviderName: "alsa_midi",
		})
	}

	return devices, nil
}

func enumMIDIAmidi() []Info {
	cmd := exec.Command("amidi", "-l")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var devices []Info
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Dir") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		hw := fields[1]
		name := strings.Join(fields[2:], " ")

		parts := strings.Split(strings.TrimPrefix(hw, "hw:"), ",")
		card, _ := strconv.Atoi(parts[0])
		dev := 0
		if len(parts) > 1 {
			dev, _ = strconv.Atoi(parts[1])
		}

		devPath := fmt.Sprintf("/dev/snd/midiC%dD%d", card, dev)
		id := DeviceID(fmt.Sprintf("alsa_midi:%s", hw))

		devices = append(devices, Info{
			ID:           id,
			Name:         name,
			Type:         TypeMIDI,
			Connection:   ConnUSB,
			Capabilities: Capabilities{MIDIPorts: 1},
			PlatformPath: devPath,
			ProviderName: "alsa_midi",
		})
	}

	return devices
}

func (p *linuxMIDIProvider) Open(_ context.Context, id DeviceID) (DeviceConnection, error) {
	// Extract hw:X,Y from ID and find the raw MIDI device path.
	hw := strings.TrimPrefix(string(id), "alsa_midi:")
	parts := strings.Split(hw, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("%w: invalid MIDI ID: %s", ErrDeviceNotFound, id)
	}
	hwParts := strings.Split(parts[0], ",")
	if len(hwParts) == 1 {
		hwParts = strings.Split(parts[1], ",")
	}
	card, _ := strconv.Atoi(hwParts[0])
	dev := 0
	if len(hwParts) > 1 {
		dev, _ = strconv.Atoi(hwParts[1])
	}

	devPath := fmt.Sprintf("/dev/snd/midiC%dD%d", card, dev)
	if _, err := os.Stat(devPath); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDeviceNotFound, devPath)
	}

	return &linuxMIDIConnection{
		deviceInfo: Info{ID: id, PlatformPath: devPath, ProviderName: "alsa_midi", Type: TypeMIDI},
		path:       devPath,
		events:     make(chan Event, 64),
	}, nil
}

// linuxMIDIConnection reads raw MIDI bytes from /dev/snd/midiC*D*.
type linuxMIDIConnection struct {
	deviceInfo Info
	path       string
	fd         *os.File
	events     chan Event
	cancel     context.CancelFunc
	alive      bool
}

func (c *linuxMIDIConnection) Info() Info               { return c.deviceInfo }
func (c *linuxMIDIConnection) Events() <-chan Event     { return c.events }
func (c *linuxMIDIConnection) Feedback() DeviceFeedback { return nil }
func (c *linuxMIDIConnection) Alive() bool              { return c.alive }

func (c *linuxMIDIConnection) Start(ctx context.Context) error {
	f, err := os.OpenFile(c.path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", c.path, err)
	}
	c.fd = f

	ctx, c.cancel = context.WithCancel(ctx)
	c.alive = true

	go c.readLoop(ctx)
	return nil
}

func (c *linuxMIDIConnection) readLoop(ctx context.Context) {
	defer func() {
		c.alive = false
		close(c.events)
	}()

	buf := make([]byte, 3)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read one byte to get status.
		n, err := c.fd.Read(buf[:1])
		if err != nil || n < 1 {
			return
		}

		status := buf[0]
		if status < 0x80 {
			continue // running status or data byte — skip
		}

		msgType := status & 0xF0
		channel := status & 0x0F

		var ev *Event
		switch msgType {
		case 0x90, 0x80: // Note On / Note Off
			if _, err := c.fd.Read(buf[1:3]); err != nil {
				return
			}
			note := buf[1] & 0x7F
			velocity := buf[2] & 0x7F
			isOn := msgType == 0x90 && velocity > 0
			evType := EventMIDINote
			src := fmt.Sprintf("midi:note:%d", note)
			ev = &Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      evType,
				Timestamp: time.Now(),
				Source:    src,
				Channel:   channel,
				MIDINote:  note,
				Velocity:  velocity,
				Pressed:   isOn,
				Value:     float64(velocity) / 127.0,
			}

		case 0xB0: // Control Change
			if _, err := c.fd.Read(buf[1:3]); err != nil {
				return
			}
			cc := buf[1] & 0x7F
			val := buf[2] & 0x7F
			src := fmt.Sprintf("midi:cc:%d", cc)
			ev = &Event{
				DeviceID:   c.deviceInfo.ID,
				Type:       EventMIDICC,
				Timestamp:  time.Now(),
				Source:     src,
				Channel:    channel,
				Controller: cc,
				MIDIValue:  val,
				Value:      float64(val) / 127.0,
			}

		case 0xC0: // Program Change (1 data byte)
			if _, err := c.fd.Read(buf[1:2]); err != nil {
				return
			}
			prog := buf[1] & 0x7F
			src := fmt.Sprintf("midi:pc:%d", prog)
			ev = &Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventMIDIProgramChange,
				Timestamp: time.Now(),
				Source:    src,
				Channel:   channel,
				Program:   prog,
				Value:     float64(prog) / 127.0,
			}

		case 0xE0: // Pitch Bend (2 data bytes → 14-bit)
			if _, err := c.fd.Read(buf[1:3]); err != nil {
				return
			}
			lsb := int16(buf[1] & 0x7F)
			msb := int16(buf[2] & 0x7F)
			bend := (msb << 7) | lsb - 8192
			ev = &Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventMIDIPitchBend,
				Timestamp: time.Now(),
				Source:    "midi:pitch_bend",
				Channel:   channel,
				PitchBend: bend,
				Value:     float64(bend) / 8192.0,
			}

		default:
			continue // System messages, etc.
		}

		if ev != nil {
			select {
			case c.events <- *ev:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (c *linuxMIDIConnection) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.fd != nil {
		return c.fd.Close()
	}
	return nil
}

func (p *linuxMIDIProvider) Close() error { return nil }

// ---------------------------------------------------------------------------
// Linux hot-plug watcher stub
// ---------------------------------------------------------------------------

// LinuxHotPlugWatcher monitors device changes via polling /proc/bus/input/devices.
// A future version would use netlink KOBJECT_UEVENT for instant notification.
type LinuxHotPlugWatcher struct {
	events   chan HotPlugEvent
	cancel   context.CancelFunc
	known    map[DeviceID]bool
	provider *linuxInputProvider
}

// NewLinuxHotPlugWatcher creates a hot-plug watcher for Linux.
func NewLinuxHotPlugWatcher() *LinuxHotPlugWatcher {
	return &LinuxHotPlugWatcher{
		events:   make(chan HotPlugEvent, 16),
		known:    make(map[DeviceID]bool),
		provider: &linuxInputProvider{},
	}
}

func (w *LinuxHotPlugWatcher) Start(ctx context.Context) error {
	ctx, w.cancel = context.WithCancel(ctx)

	// Initial snapshot.
	devices, _ := w.provider.Enumerate(ctx)
	for _, d := range devices {
		w.known[d.ID] = true
	}

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		defer close(w.events)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, _ := w.provider.Enumerate(ctx)
				currentIDs := make(map[DeviceID]bool)

				for _, d := range current {
					currentIDs[d.ID] = true
					if !w.known[d.ID] {
						w.known[d.ID] = true
						select {
						case w.events <- HotPlugEvent{
							Type:      HotPlugConnect,
							Info:      d,
							Timestamp: time.Now(),
						}:
						default:
						}
					}
				}

				for id := range w.known {
					if !currentIDs[id] {
						delete(w.known, id)
						select {
						case w.events <- HotPlugEvent{
							Type:      HotPlugDisconnect,
							Info:      Info{ID: id},
							Timestamp: time.Now(),
						}:
						default:
						}
					}
				}
			}
		}
	}()

	return nil
}

func (w *LinuxHotPlugWatcher) Events() <-chan HotPlugEvent { return w.events }

func (w *LinuxHotPlugWatcher) Close() error {
	if w.cancel != nil {
		w.cancel()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseHexUint16(s string) uint16 {
	v, _ := strconv.ParseUint(s, 16, 16)
	return uint16(v)
}
