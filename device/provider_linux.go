//go:build linux

package device

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	blocks := strings.Split(string(data), "\n\n")

	for _, block := range blocks {
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
				for _, part := range strings.Fields(line) {
					if strings.HasPrefix(part, "Vendor=") {
						vendor = strings.ToLower(strings.TrimPrefix(part, "Vendor="))
					} else if strings.HasPrefix(part, "Product=") {
						product = strings.ToLower(strings.TrimPrefix(part, "Product="))
					}
				}
			case strings.HasPrefix(line, "H: Handlers="):
				for _, h := range strings.Fields(strings.TrimPrefix(line, "H: Handlers=")) {
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

// linuxEvdevConnection reads events from a Linux input device via evtest.
// This is a subprocess-based approach matching the existing input-mcp pattern.
// A future optimization would use grafov/evdev for direct ioctl reading.
type linuxEvdevConnection struct {
	deviceInfo Info
	path       string
	events     chan Event
	cancel     context.CancelFunc
	alive      bool
}

func (c *linuxEvdevConnection) Info() Info                { return c.deviceInfo }
func (c *linuxEvdevConnection) Events() <-chan Event      { return c.events }
func (c *linuxEvdevConnection) Feedback() DeviceFeedback  { return nil }
func (c *linuxEvdevConnection) Alive() bool               { return c.alive }

func (c *linuxEvdevConnection) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)
	c.alive = true

	go func() {
		defer func() {
			c.alive = false
			close(c.events)
		}()

		cmd := exec.CommandContext(ctx, "evtest", c.path)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		if err := cmd.Start(); err != nil {
			return
		}

		// Read output until context cancellation.
		<-ctx.Done()
		cmd.Process.Kill()
		cmd.Wait()
	}()

	return nil
}

func (c *linuxEvdevConnection) Close() error {
	if c.cancel != nil {
		c.cancel()
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
	for _, line := range strings.Split(string(out), "\n") {
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
	return nil, fmt.Errorf("%w: MIDI connection not yet implemented (use amidi for raw access)", ErrNotSupported)
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
