//go:build windows

package device

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

func init() {
	RegisterProvider(func() DeviceProvider { return &windowsXInputProvider{} })
	RegisterProvider(func() DeviceProvider { return &windowsMIDIProvider{} })
}

// ---------------------------------------------------------------------------
// XInput provider — Xbox controllers via xinput1_4.dll
// ---------------------------------------------------------------------------

var (
	xinput14        = syscall.NewLazyDLL("xinput1_4.dll")
	xInputGetState  = xinput14.NewProc("XInputGetState")
	xInputGetCaps   = xinput14.NewProc("XInputGetCapabilities")
	xInputSetState  = xinput14.NewProc("XInputSetState")
)

// xinputVibration matches XINPUT_VIBRATION from XInput.h.
type xinputVibration struct {
	LeftMotorSpeed  uint16
	RightMotorSpeed uint16
}

const (
	xinputMaxControllers     = 4
	errDeviceNotConnected    = 1167
	xinputGamepadDPadUp      = 0x0001
	xinputGamepadDPadDown    = 0x0002
	xinputGamepadDPadLeft    = 0x0004
	xinputGamepadDPadRight   = 0x0008
	xinputGamepadStart       = 0x0010
	xinputGamepadBack        = 0x0020
	xinputGamepadLeftThumb   = 0x0040
	xinputGamepadRightThumb  = 0x0080
	xinputGamepadLB          = 0x0100
	xinputGamepadRB          = 0x0200
	xinputGamepadGuide       = 0x0400
	xinputGamepadA           = 0x1000
	xinputGamepadB           = 0x2000
	xinputGamepadX           = 0x4000
	xinputGamepadY           = 0x8000
)

type xinputGamepad struct {
	Buttons      uint16
	LeftTrigger  uint8
	RightTrigger uint8
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

type xinputState struct {
	PacketNumber uint32
	Gamepad      xinputGamepad
}

type xinputCapabilities struct {
	Type    uint8
	SubType uint8
	Flags   uint16
	Gamepad xinputGamepad
	// Vibration follows but we don't need it.
}

var xinputButtonMap = []struct {
	mask uint16
	name string
}{
	{xinputGamepadA, "BTN_SOUTH"},
	{xinputGamepadB, "BTN_EAST"},
	{xinputGamepadX, "BTN_WEST"},
	{xinputGamepadY, "BTN_NORTH"},
	{xinputGamepadLB, "BTN_TL"},
	{xinputGamepadRB, "BTN_TR"},
	{xinputGamepadStart, "BTN_START"},
	{xinputGamepadBack, "BTN_SELECT"},
	{xinputGamepadLeftThumb, "BTN_THUMBL"},
	{xinputGamepadRightThumb, "BTN_THUMBR"},
	{xinputGamepadGuide, "BTN_MODE"},
}

type windowsXInputProvider struct{}

func (p *windowsXInputProvider) Name() string { return "xinput" }

func (p *windowsXInputProvider) DeviceTypes() []DeviceType {
	return []DeviceType{TypeGamepad}
}

func (p *windowsXInputProvider) Enumerate(_ context.Context) ([]Info, error) {
	var devices []Info
	for i := uint32(0); i < xinputMaxControllers; i++ {
		var state xinputState
		ret, _, _ := xInputGetState.Call(uintptr(i), uintptr(unsafe.Pointer(&state)))
		if ret != 0 {
			continue
		}

		name := fmt.Sprintf("XInput Controller %d", i)
		var caps xinputCapabilities
		ret, _, _ = xInputGetCaps.Call(uintptr(i), 0, uintptr(unsafe.Pointer(&caps)))
		if ret == 0 {
			switch caps.SubType {
			case 1:
				name = fmt.Sprintf("XInput Gamepad %d", i)
			case 2:
				name = fmt.Sprintf("XInput Wheel %d", i)
			case 3:
				name = fmt.Sprintf("XInput Arcade Stick %d", i)
			case 4:
				name = fmt.Sprintf("XInput Flight Stick %d", i)
			case 5:
				name = fmt.Sprintf("XInput Dance Pad %d", i)
			case 6:
				name = fmt.Sprintf("XInput Guitar %d", i)
			case 8:
				name = fmt.Sprintf("XInput Drum Kit %d", i)
			}
		}

		id := DeviceID(fmt.Sprintf("xinput:%d", i))
		devices = append(devices, Info{
			ID:         id,
			Name:       name,
			Type:       TypeGamepad,
			Connection: ConnUSB,
			VendorID:   0x045e, // Microsoft
			Capabilities: Capabilities{
				Buttons:   14,
				Axes:      6,
				Hats:      1,
				HasRumble: true,
			},
			PlatformPath: fmt.Sprintf("xinput:%d", i),
			ProviderName: "xinput",
		})
	}
	return devices, nil
}

func (p *windowsXInputProvider) Open(_ context.Context, id DeviceID) (DeviceConnection, error) {
	var index uint32
	if _, err := fmt.Sscanf(string(id), "xinput:%d", &index); err != nil || index >= xinputMaxControllers {
		return nil, fmt.Errorf("%w: invalid XInput ID: %s", ErrDeviceNotFound, id)
	}

	// Verify device is connected.
	var state xinputState
	ret, _, _ := xInputGetState.Call(uintptr(index), uintptr(unsafe.Pointer(&state)))
	if ret != 0 {
		return nil, fmt.Errorf("%w: XInput slot %d not connected", ErrDeviceNotFound, index)
	}

	return &xinputConnection{
		deviceInfo: Info{
			ID:           id,
			Name:         fmt.Sprintf("XInput Controller %d", index),
			Type:         TypeGamepad,
			ProviderName: "xinput",
			PlatformPath: string(id),
		},
		index:  index,
		events: make(chan Event, 64),
	}, nil
}

func (p *windowsXInputProvider) Close() error { return nil }

// xinputConnection polls XInput state at 200Hz.
type xinputConnection struct {
	deviceInfo Info
	index      uint32
	lastState  xinputState
	events     chan Event
	cancel     context.CancelFunc
	alive      bool
	feedback   *xinputFeedback
}

func (c *xinputConnection) Info() Info           { return c.deviceInfo }
func (c *xinputConnection) Events() <-chan Event { return c.events }
func (c *xinputConnection) Feedback() DeviceFeedback {
	if c.feedback != nil {
		return c.feedback
	}
	return nil
}
func (c *xinputConnection) Alive() bool { return c.alive }

func (c *xinputConnection) Start(ctx context.Context) error {
	// Read initial state.
	ret, _, _ := xInputGetState.Call(uintptr(c.index), uintptr(unsafe.Pointer(&c.lastState)))
	if ret != 0 {
		return fmt.Errorf("%w: XInput slot %d disconnected", ErrDeviceDisconnected, c.index)
	}

	// All XInput controllers support rumble.
	c.feedback = &xinputFeedback{index: c.index}

	ctx, c.cancel = context.WithCancel(ctx)
	c.alive = true
	go c.readLoop(ctx)
	return nil
}

func (c *xinputConnection) readLoop(ctx context.Context) {
	defer func() {
		c.alive = false
		close(c.events)
	}()

	ticker := time.NewTicker(5 * time.Millisecond) // 200Hz
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		var state xinputState
		ret, _, _ := xInputGetState.Call(uintptr(c.index), uintptr(unsafe.Pointer(&state)))
		if ret != 0 {
			return // disconnected
		}
		if state.PacketNumber == c.lastState.PacketNumber {
			continue
		}

		now := time.Now()
		gp := state.Gamepad
		last := c.lastState.Gamepad

		// Button changes.
		buttonDiff := gp.Buttons ^ last.Buttons
		for _, btn := range xinputButtonMap {
			if buttonDiff&btn.mask == 0 {
				continue
			}
			pressed := gp.Buttons&btn.mask != 0
			val := 0.0
			if pressed {
				val = 1.0
			}
			c.emit(ctx, Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventButton,
				Timestamp: now,
				Source:    btn.name,
				Pressed:   pressed,
				Value:     val,
			})
		}

		// D-pad → hat events.
		hatX, hatY := xinputDPadToHat(gp.Buttons)
		lastHatX, lastHatY := xinputDPadToHat(last.Buttons)
		if hatX != lastHatX {
			c.emit(ctx, Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventHat,
				Timestamp: now,
				Source:    "ABS_HAT0X",
				HatX:     hatX,
			})
		}
		if hatY != lastHatY {
			c.emit(ctx, Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventHat,
				Timestamp: now,
				Source:    "ABS_HAT0Y",
				HatY:     hatY,
			})
		}

		// Analog sticks.
		if gp.ThumbLX != last.ThumbLX {
			c.emit(ctx, Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventAxis,
				Timestamp: now,
				Source:    "ABS_X",
				Value:     float64(gp.ThumbLX) / 32767.0,
				RawValue:  int32(gp.ThumbLX),
			})
		}
		if gp.ThumbLY != last.ThumbLY {
			c.emit(ctx, Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventAxis,
				Timestamp: now,
				Source:    "ABS_Y",
				Value:     float64(gp.ThumbLY) / 32767.0,
				RawValue:  int32(gp.ThumbLY),
			})
		}
		if gp.ThumbRX != last.ThumbRX {
			c.emit(ctx, Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventAxis,
				Timestamp: now,
				Source:    "ABS_RX",
				Value:     float64(gp.ThumbRX) / 32767.0,
				RawValue:  int32(gp.ThumbRX),
			})
		}
		if gp.ThumbRY != last.ThumbRY {
			c.emit(ctx, Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventAxis,
				Timestamp: now,
				Source:    "ABS_RY",
				Value:     float64(gp.ThumbRY) / 32767.0,
				RawValue:  int32(gp.ThumbRY),
			})
		}

		// Triggers (unsigned: 0.0 to 1.0).
		if gp.LeftTrigger != last.LeftTrigger {
			c.emit(ctx, Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventAxis,
				Timestamp: now,
				Source:    "ABS_Z",
				Value:     float64(gp.LeftTrigger) / 255.0,
				RawValue:  int32(gp.LeftTrigger),
			})
		}
		if gp.RightTrigger != last.RightTrigger {
			c.emit(ctx, Event{
				DeviceID:  c.deviceInfo.ID,
				Type:      EventAxis,
				Timestamp: now,
				Source:    "ABS_RZ",
				Value:     float64(gp.RightTrigger) / 255.0,
				RawValue:  int32(gp.RightTrigger),
			})
		}

		c.lastState = state
	}
}

func (c *xinputConnection) emit(ctx context.Context, ev Event) {
	select {
	case c.events <- ev:
	case <-ctx.Done():
	}
}

func (c *xinputConnection) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func xinputDPadToHat(buttons uint16) (hatX, hatY int8) {
	if buttons&xinputGamepadDPadLeft != 0 {
		hatX = -1
	} else if buttons&xinputGamepadDPadRight != 0 {
		hatX = 1
	}
	if buttons&xinputGamepadDPadUp != 0 {
		hatY = -1
	} else if buttons&xinputGamepadDPadDown != 0 {
		hatY = 1
	}
	return
}

// ---------------------------------------------------------------------------
// XInput force-feedback (rumble)
// ---------------------------------------------------------------------------

// xinputFeedback implements DeviceFeedback for XInput controllers.
type xinputFeedback struct {
	index uint32
	mu    sync.Mutex
}

// SetRumble sets rumble motor intensity on the XInput controller.
// motor 0 = left (low-frequency), motor 1 = right (high-frequency).
// intensity ranges from 0.0 to 1.0, mapped to uint16 0-65535.
// If duration > 0, a goroutine is spawned to stop the motor after the duration.
func (f *xinputFeedback) SetRumble(motor int, intensity float64, duration time.Duration) error {
	if motor < 0 || motor > 1 {
		return fmt.Errorf("%w: motor index must be 0 (left) or 1 (right)", ErrNotSupported)
	}
	if intensity < 0 {
		intensity = 0
	} else if intensity > 1 {
		intensity = 1
	}

	speed := uint16(intensity * 65535)

	f.mu.Lock()
	defer f.mu.Unlock()

	var vib xinputVibration
	if motor == 0 {
		vib.LeftMotorSpeed = speed
	} else {
		vib.RightMotorSpeed = speed
	}

	ret, _, _ := xInputSetState.Call(uintptr(f.index), uintptr(unsafe.Pointer(&vib)))
	if ret != 0 {
		return fmt.Errorf("XInputSetState failed: error code %d", ret)
	}

	// If a duration is specified, stop the motor after the duration expires.
	if duration > 0 && speed > 0 {
		go func() {
			time.Sleep(duration)
			f.mu.Lock()
			defer f.mu.Unlock()
			var stop xinputVibration
			xInputSetState.Call(uintptr(f.index), uintptr(unsafe.Pointer(&stop)))
		}()
	}

	return nil
}

func (f *xinputFeedback) SetLED(index int, r, g, b, a uint8) error {
	return ErrNotSupported
}

func (f *xinputFeedback) SendMIDI(data []byte) error {
	return ErrNotSupported
}

func (f *xinputFeedback) SendRaw(data []byte) error {
	return ErrNotSupported
}

// ---------------------------------------------------------------------------
// WinMM MIDI provider — MIDI input via winmm.dll
// ---------------------------------------------------------------------------

var (
	winmm             = syscall.NewLazyDLL("winmm.dll")
	midiInGetNumDevs  = winmm.NewProc("midiInGetNumDevs")
	midiInGetDevCapsW = winmm.NewProc("midiInGetDevCapsW")
	midiInOpen        = winmm.NewProc("midiInOpen")
	midiInStart       = winmm.NewProc("midiInStart")
	midiInStop        = winmm.NewProc("midiInStop")
	midiInClose       = winmm.NewProc("midiInClose")
)

const (
	callbackFunction = 0x00030000
	mimData          = 0x3C3 // MIM_DATA
)

type midiInCaps struct {
	Mid       uint16
	Pid       uint16
	DriverVer uint32
	Pname     [32]uint16
	Support   uint32
}

type windowsMIDIProvider struct{}

func (p *windowsMIDIProvider) Name() string { return "winmm_midi" }

func (p *windowsMIDIProvider) DeviceTypes() []DeviceType {
	return []DeviceType{TypeMIDI}
}

func (p *windowsMIDIProvider) Enumerate(_ context.Context) ([]Info, error) {
	numDevs, _, _ := midiInGetNumDevs.Call()
	if numDevs == 0 {
		return nil, nil
	}

	var devices []Info
	for i := uintptr(0); i < numDevs; i++ {
		var caps midiInCaps
		ret, _, _ := midiInGetDevCapsW.Call(i, uintptr(unsafe.Pointer(&caps)), unsafe.Sizeof(caps))
		if ret != 0 {
			continue
		}

		name := syscall.UTF16ToString(caps.Pname[:])
		id := DeviceID(fmt.Sprintf("winmm_midi:%d", i))
		devices = append(devices, Info{
			ID:           id,
			Name:         name,
			Type:         TypeMIDI,
			Connection:   ConnUSB,
			VendorID:     caps.Mid,
			ProductID:    caps.Pid,
			Capabilities: Capabilities{MIDIPorts: 1},
			PlatformPath: fmt.Sprintf("winmm_midi:%d", i),
			ProviderName: "winmm_midi",
		})
	}
	return devices, nil
}

func (p *windowsMIDIProvider) Open(_ context.Context, id DeviceID) (DeviceConnection, error) {
	var index uint32
	if _, err := fmt.Sscanf(string(id), "winmm_midi:%d", &index); err != nil {
		return nil, fmt.Errorf("%w: invalid WinMM MIDI ID: %s", ErrDeviceNotFound, id)
	}

	numDevs, _, _ := midiInGetNumDevs.Call()
	if uintptr(index) >= numDevs {
		return nil, fmt.Errorf("%w: WinMM MIDI device %d not found", ErrDeviceNotFound, index)
	}

	conn := &winmmMIDIConnection{
		deviceInfo: Info{
			ID:           id,
			Type:         TypeMIDI,
			ProviderName: "winmm_midi",
			PlatformPath: string(id),
		},
		deviceIndex: index,
		events:      make(chan Event, 64),
		msgChan:     make(chan uint32, 256),
	}

	// Populate name from caps.
	var caps midiInCaps
	ret, _, _ := midiInGetDevCapsW.Call(uintptr(index), uintptr(unsafe.Pointer(&caps)), unsafe.Sizeof(caps))
	if ret == 0 {
		conn.deviceInfo.Name = syscall.UTF16ToString(caps.Pname[:])
	}

	return conn, nil
}

func (p *windowsMIDIProvider) Close() error { return nil }

// winmmMIDIConnection reads MIDI via WinMM callback.
type winmmMIDIConnection struct {
	deviceInfo  Info
	deviceIndex uint32
	handle      uintptr
	events      chan Event
	msgChan     chan uint32
	cancel      context.CancelFunc
	alive       bool
}

// winmmInstances maps callback instance data back to connections.
var (
	winmmInstances   = make(map[uintptr]*winmmMIDIConnection)
	winmmInstancesMu sync.Mutex
	winmmNextID      uintptr
)

func (c *winmmMIDIConnection) Info() Info               { return c.deviceInfo }
func (c *winmmMIDIConnection) Events() <-chan Event     { return c.events }
func (c *winmmMIDIConnection) Feedback() DeviceFeedback { return nil }
func (c *winmmMIDIConnection) Alive() bool              { return c.alive }

func (c *winmmMIDIConnection) Start(ctx context.Context) error {
	// Register instance for callback lookup.
	winmmInstancesMu.Lock()
	winmmNextID++
	instanceID := winmmNextID
	winmmInstances[instanceID] = c
	winmmInstancesMu.Unlock()

	cb := syscall.NewCallback(winmmMIDICallback)
	ret, _, _ := midiInOpen.Call(
		uintptr(unsafe.Pointer(&c.handle)),
		uintptr(c.deviceIndex),
		cb,
		instanceID,
		callbackFunction,
	)
	if ret != 0 {
		winmmInstancesMu.Lock()
		delete(winmmInstances, instanceID)
		winmmInstancesMu.Unlock()
		return fmt.Errorf("midiInOpen failed: MMRESULT %d", ret)
	}

	ret, _, _ = midiInStart.Call(c.handle)
	if ret != 0 {
		midiInClose.Call(c.handle)
		winmmInstancesMu.Lock()
		delete(winmmInstances, instanceID)
		winmmInstancesMu.Unlock()
		return fmt.Errorf("midiInStart failed: MMRESULT %d", ret)
	}

	ctx, c.cancel = context.WithCancel(ctx)
	c.alive = true
	go c.readLoop(ctx, instanceID)
	return nil
}

func winmmMIDICallback(handle, msg, instance, param1, param2 uintptr) uintptr {
	if msg != mimData {
		return 0
	}
	winmmInstancesMu.Lock()
	conn := winmmInstances[instance]
	winmmInstancesMu.Unlock()
	if conn == nil {
		return 0
	}

	select {
	case conn.msgChan <- uint32(param1):
	default: // drop if buffer full
	}
	return 0
}

func (c *winmmMIDIConnection) readLoop(ctx context.Context, instanceID uintptr) {
	defer func() {
		c.alive = false
		close(c.events)
		winmmInstancesMu.Lock()
		delete(winmmInstances, instanceID)
		winmmInstancesMu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.msgChan:
			if !ok {
				return
			}
			ev := c.parseMIDI(msg)
			if ev != nil {
				select {
				case c.events <- *ev:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (c *winmmMIDIConnection) parseMIDI(packed uint32) *Event {
	status := byte(packed)
	data1 := byte(packed >> 8)
	data2 := byte(packed >> 16)

	if status < 0x80 {
		return nil
	}

	msgType := status & 0xF0
	channel := status & 0x0F

	switch msgType {
	case 0x90, 0x80: // Note On / Note Off
		note := data1 & 0x7F
		velocity := data2 & 0x7F
		isOn := msgType == 0x90 && velocity > 0
		return &Event{
			DeviceID:  c.deviceInfo.ID,
			Type:      EventMIDINote,
			Timestamp: time.Now(),
			Source:    fmt.Sprintf("midi:note:%d", note),
			Channel:   channel,
			MIDINote:  note,
			Velocity:  velocity,
			Pressed:   isOn,
			Value:     float64(velocity) / 127.0,
		}

	case 0xB0: // Control Change
		cc := data1 & 0x7F
		val := data2 & 0x7F
		return &Event{
			DeviceID:   c.deviceInfo.ID,
			Type:       EventMIDICC,
			Timestamp:  time.Now(),
			Source:     fmt.Sprintf("midi:cc:%d", cc),
			Channel:    channel,
			Controller: cc,
			MIDIValue:  val,
			Value:      float64(val) / 127.0,
		}

	case 0xC0: // Program Change
		prog := data1 & 0x7F
		return &Event{
			DeviceID:  c.deviceInfo.ID,
			Type:      EventMIDIProgramChange,
			Timestamp: time.Now(),
			Source:    fmt.Sprintf("midi:pc:%d", prog),
			Channel:   channel,
			Program:   prog,
			Value:     float64(prog) / 127.0,
		}

	case 0xE0: // Pitch Bend
		lsb := int16(data1 & 0x7F)
		msb := int16(data2 & 0x7F)
		bend := (msb << 7) | lsb - 8192
		return &Event{
			DeviceID:  c.deviceInfo.ID,
			Type:      EventMIDIPitchBend,
			Timestamp: time.Now(),
			Source:    "midi:pitch_bend",
			Channel:   channel,
			PitchBend: bend,
			Value:     float64(bend) / 8192.0,
		}
	}

	return nil
}

func (c *winmmMIDIConnection) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.handle != 0 {
		midiInStop.Call(c.handle)
		midiInClose.Call(c.handle)
	}
	return nil
}
