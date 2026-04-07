//go:build linux

package device

import (
	"context"
	"testing"
)

func TestParseHexUint16(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  uint16
	}{
		{"045e", 0x045e},
		{"054c", 0x054c},
		{"0000", 0x0000},
		{"ffff", 0xffff},
		{"FFFF", 0xFFFF},
		{"1", 0x0001},
		{"", 0},
		{"invalid", 0},
		{"gggg", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := parseHexUint16(tt.input)
			if got != tt.want {
				t.Errorf("parseHexUint16(%q) = 0x%04x, want 0x%04x", tt.input, got, tt.want)
			}
		})
	}
}

func TestEvKeyName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code uint16
		want string
	}{
		{1, "KEY_ESC"},
		{28, "KEY_ENTER"},
		{57, "KEY_SPACE"},
		{0x130, "BTN_SOUTH"},
		{0x131, "BTN_EAST"},
		{0x133, "BTN_NORTH"},
		{0x134, "BTN_WEST"},
		{0x13a, "BTN_SELECT"},
		{0x13b, "BTN_START"},
		{0x110, "BTN_LEFT"},
		{0x111, "BTN_RIGHT"},
		{0x112, "BTN_MIDDLE"},
		{0x220, "BTN_DPAD_UP"},
		{0xFFFF, ""},    // unknown code
		{0x0000, ""},    // zero code
	}
	for _, tt := range tests {
		got := evKeyName(tt.code)
		if got != tt.want {
			t.Errorf("evKeyName(0x%04x) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestEvAbsName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code uint16
		want string
	}{
		{0x00, "ABS_X"},
		{0x01, "ABS_Y"},
		{0x05, "ABS_RZ"},
		{0x10, "ABS_HAT0X"},
		{0x11, "ABS_HAT0Y"},
		{0x09, "ABS_GAS"},
		{0x0a, "ABS_BRAKE"},
		{0xFF, ""},
	}
	for _, tt := range tests {
		got := evAbsName(tt.code)
		if got != tt.want {
			t.Errorf("evAbsName(0x%04x) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestEvRelName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code uint16
		want string
	}{
		{0x00, "REL_X"},
		{0x01, "REL_Y"},
		{0x08, "REL_WHEEL"},
		{0x07, "REL_DIAL"},
		{0xFF, ""},
	}
	for _, tt := range tests {
		got := evRelName(tt.code)
		if got != tt.want {
			t.Errorf("evRelName(0x%04x) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestLinuxEvdevConnection_ConvertEvent_Key(t *testing.T) {
	t.Parallel()

	conn := &linuxEvdevConnection{
		deviceInfo: Info{ID: "evdev:/dev/input/event0"},
		absInfos:   make(map[uint16]*absInfo),
	}

	// Key press (keyboard key, code < 0x100)
	ev := inputEvent{Type: evKey, Code: 28, Val: 1} // KEY_ENTER
	got := conn.convertEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil event for KEY_ENTER press")
	}
	if got.Type != EventKey {
		t.Errorf("Type = %v, want EventKey", got.Type)
	}
	if got.Source != "KEY_ENTER" {
		t.Errorf("Source = %q, want KEY_ENTER", got.Source)
	}
	if !got.Pressed {
		t.Error("Pressed should be true for Val=1")
	}
	if got.Value != 1.0 {
		t.Errorf("Value = %f, want 1.0", got.Value)
	}

	// Key release
	ev2 := inputEvent{Type: evKey, Code: 28, Val: 0}
	got2 := conn.convertEvent(ev2)
	if got2 == nil {
		t.Fatal("expected non-nil event for KEY_ENTER release")
	}
	if got2.Pressed {
		t.Error("Pressed should be false for Val=0")
	}

	// Button press (code >= 0x100)
	ev3 := inputEvent{Type: evKey, Code: 0x130, Val: 1} // BTN_SOUTH
	got3 := conn.convertEvent(ev3)
	if got3 == nil {
		t.Fatal("expected non-nil event for BTN_SOUTH")
	}
	if got3.Type != EventButton {
		t.Errorf("Type = %v, want EventButton for code >= 0x100", got3.Type)
	}
	if got3.Source != "BTN_SOUTH" {
		t.Errorf("Source = %q, want BTN_SOUTH", got3.Source)
	}

	// Unknown key code
	ev4 := inputEvent{Type: evKey, Code: 0xFFFF, Val: 1}
	got4 := conn.convertEvent(ev4)
	if got4 != nil {
		t.Error("expected nil for unknown key code")
	}
}

func TestLinuxEvdevConnection_ConvertEvent_Abs(t *testing.T) {
	t.Parallel()

	conn := &linuxEvdevConnection{
		deviceInfo: Info{ID: "evdev:/dev/input/event0"},
		absInfos: map[uint16]*absInfo{
			0x00: {Minimum: -32768, Maximum: 32767}, // ABS_X (signed stick)
			0x09: {Minimum: 0, Maximum: 255},         // ABS_GAS (unsigned trigger)
		},
	}

	// Regular axis (signed, stick)
	ev := inputEvent{Type: evAbs, Code: 0x00, Val: 0}
	got := conn.convertEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil event for ABS_X")
	}
	if got.Type != EventAxis {
		t.Errorf("Type = %v, want EventAxis", got.Type)
	}
	if got.Source != "ABS_X" {
		t.Errorf("Source = %q, want ABS_X", got.Source)
	}

	// Hat axis (codes 0x10-0x17)
	evHat := inputEvent{Type: evAbs, Code: 0x10, Val: 1} // ABS_HAT0X
	gotHat := conn.convertEvent(evHat)
	if gotHat == nil {
		t.Fatal("expected non-nil event for ABS_HAT0X")
	}
	if gotHat.Type != EventHat {
		t.Errorf("Type = %v, want EventHat for hat axis", gotHat.Type)
	}
	if gotHat.HatX != 1 {
		t.Errorf("HatX = %d, want 1", gotHat.HatX)
	}

	// Hat Y axis (odd code)
	evHatY := inputEvent{Type: evAbs, Code: 0x11, Val: -1} // ABS_HAT0Y
	gotHatY := conn.convertEvent(evHatY)
	if gotHatY == nil {
		t.Fatal("expected non-nil event for ABS_HAT0Y")
	}
	if gotHatY.HatY != -1 {
		t.Errorf("HatY = %d, want -1", gotHatY.HatY)
	}

	// Unknown abs code
	evUnk := inputEvent{Type: evAbs, Code: 0xFF, Val: 0}
	gotUnk := conn.convertEvent(evUnk)
	if gotUnk != nil {
		t.Error("expected nil for unknown ABS code")
	}
}

func TestLinuxEvdevConnection_ConvertEvent_Rel(t *testing.T) {
	t.Parallel()

	conn := &linuxEvdevConnection{
		deviceInfo: Info{ID: "evdev:/dev/input/event0"},
		absInfos:   make(map[uint16]*absInfo),
	}

	// Regular relative axis
	ev := inputEvent{Type: evRel, Code: 0x00, Val: 5} // REL_X
	got := conn.convertEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil event for REL_X")
	}
	if got.Type != EventAxis {
		t.Errorf("Type = %v, want EventAxis", got.Type)
	}
	if got.Value != 5.0 {
		t.Errorf("Value = %f, want 5.0", got.Value)
	}

	// REL_DIAL should be EventEncoder
	evDial := inputEvent{Type: evRel, Code: 0x07, Val: 1}
	gotDial := conn.convertEvent(evDial)
	if gotDial == nil {
		t.Fatal("expected non-nil event for REL_DIAL")
	}
	if gotDial.Type != EventEncoder {
		t.Errorf("Type = %v, want EventEncoder for REL_DIAL", gotDial.Type)
	}

	// REL_WHEEL should be EventEncoder
	evWheel := inputEvent{Type: evRel, Code: 0x08, Val: -1}
	gotWheel := conn.convertEvent(evWheel)
	if gotWheel == nil {
		t.Fatal("expected non-nil event for REL_WHEEL")
	}
	if gotWheel.Type != EventEncoder {
		t.Errorf("Type = %v, want EventEncoder for REL_WHEEL", gotWheel.Type)
	}

	// Unknown REL code
	evUnk := inputEvent{Type: evRel, Code: 0xFF, Val: 0}
	if conn.convertEvent(evUnk) != nil {
		t.Error("expected nil for unknown REL code")
	}
}

func TestLinuxEvdevConnection_ConvertEvent_Unsupported(t *testing.T) {
	t.Parallel()

	conn := &linuxEvdevConnection{
		deviceInfo: Info{ID: "evdev:/dev/input/event0"},
		absInfos:   make(map[uint16]*absInfo),
	}

	// EV_SYN should return nil.
	ev := inputEvent{Type: evSyn, Code: 0, Val: 0}
	if conn.convertEvent(ev) != nil {
		t.Error("expected nil for EV_SYN")
	}

	// EV_MSC should return nil.
	ev2 := inputEvent{Type: evMsc, Code: 0, Val: 0}
	if conn.convertEvent(ev2) != nil {
		t.Error("expected nil for EV_MSC")
	}
}

func TestLinuxEvdevConnection_NormalizeAxis(t *testing.T) {
	t.Parallel()

	conn := &linuxEvdevConnection{
		absInfos: map[uint16]*absInfo{
			0x00: {Minimum: -32768, Maximum: 32767}, // Signed stick
			0x09: {Minimum: 0, Maximum: 255},         // Unsigned trigger
			0x0a: {Minimum: 0, Maximum: 0},           // Degenerate (same min/max)
		},
	}

	// Signed axis: center
	if v := conn.normalizeAxis(0x00, 0); v < -0.01 || v > 0.01 {
		t.Errorf("signed center: %f, want ~0.0", v)
	}

	// Signed axis: full positive
	if v := conn.normalizeAxis(0x00, 32767); v < 0.99 {
		t.Errorf("signed max: %f, want ~1.0", v)
	}

	// Signed axis: full negative
	if v := conn.normalizeAxis(0x00, -32768); v > -0.99 {
		t.Errorf("signed min: %f, want ~-1.0", v)
	}

	// Unsigned axis: zero
	if v := conn.normalizeAxis(0x09, 0); v != 0.0 {
		t.Errorf("unsigned zero: %f, want 0.0", v)
	}

	// Unsigned axis: max
	if v := conn.normalizeAxis(0x09, 255); v != 1.0 {
		t.Errorf("unsigned max: %f, want 1.0", v)
	}

	// Unsigned axis: mid
	if v := conn.normalizeAxis(0x09, 128); v < 0.49 || v > 0.51 {
		t.Errorf("unsigned mid: %f, want ~0.5", v)
	}

	// Degenerate axis (min==max): return raw
	if v := conn.normalizeAxis(0x0a, 42); v != 42.0 {
		t.Errorf("degenerate: %f, want 42.0", v)
	}

	// Unknown axis (not in absInfos): return raw
	if v := conn.normalizeAxis(0xFF, 99); v != 99.0 {
		t.Errorf("unknown axis: %f, want 99.0", v)
	}
}

func TestLinuxInputProvider_Open_InvalidID(t *testing.T) {
	t.Parallel()

	p := &linuxInputProvider{}

	// Empty ID.
	_, err := p.Open(context.Background(), "evdev:")
	if err == nil {
		t.Error("expected error for empty evdev path")
	}

	// Invalid prefix.
	_, err = p.Open(context.Background(), "evdev:/tmp/not-input")
	if err == nil {
		t.Error("expected error for non-/dev/input/ path")
	}
}

func TestLinuxMIDIProvider_Open_InvalidID(t *testing.T) {
	t.Parallel()

	p := &linuxMIDIProvider{}

	// Invalid format.
	_, err := p.Open(context.Background(), "alsa_midi:invalid")
	if err == nil {
		t.Error("expected error for invalid MIDI ID format")
	}
}

func TestLinuxInputProvider_Name(t *testing.T) {
	p := &linuxInputProvider{}
	if p.Name() != "evdev" {
		t.Errorf("Name() = %q, want evdev", p.Name())
	}
}

func TestLinuxMIDIProvider_Name(t *testing.T) {
	p := &linuxMIDIProvider{}
	if p.Name() != "alsa_midi" {
		t.Errorf("Name() = %q, want alsa_midi", p.Name())
	}
}

func TestLinuxInputProvider_DeviceTypes(t *testing.T) {
	p := &linuxInputProvider{}
	types := p.DeviceTypes()
	if len(types) != 6 {
		t.Errorf("DeviceTypes() returned %d types, want 6", len(types))
	}
}

func TestLinuxMIDIProvider_DeviceTypes(t *testing.T) {
	p := &linuxMIDIProvider{}
	types := p.DeviceTypes()
	if len(types) != 1 || types[0] != TypeMIDI {
		t.Errorf("DeviceTypes() = %v, want [TypeMIDI]", types)
	}
}

func TestLinuxInputProvider_Close(t *testing.T) {
	p := &linuxInputProvider{}
	if err := p.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

func TestLinuxEvdevFeedback_SetRumble_InvalidMotor(t *testing.T) {
	t.Parallel()

	f := &linuxEvdevFeedback{effectID: -1}
	err := f.SetRumble(2, 0.5, 100)
	if err == nil {
		t.Error("expected error for motor index > 1")
	}
	err = f.SetRumble(-1, 0.5, 100)
	if err == nil {
		t.Error("expected error for negative motor index")
	}
}

func TestLinuxEvdevFeedback_Unsupported(t *testing.T) {
	t.Parallel()

	f := &linuxEvdevFeedback{effectID: -1}
	if err := f.SetLED(0, 255, 0, 0, 255); err != ErrNotSupported {
		t.Errorf("SetLED = %v, want ErrNotSupported", err)
	}
	if err := f.SendMIDI([]byte{0x90, 60, 127}); err != ErrNotSupported {
		t.Errorf("SendMIDI = %v, want ErrNotSupported", err)
	}
	if err := f.SendRaw([]byte{1, 2, 3}); err != ErrNotSupported {
		t.Errorf("SendRaw = %v, want ErrNotSupported", err)
	}
}

func TestLinuxMIDIFeedback_Unsupported(t *testing.T) {
	t.Parallel()

	f := &linuxMIDIFeedback{}
	if err := f.SetLED(0, 255, 0, 0, 255); err != ErrNotSupported {
		t.Errorf("SetLED = %v, want ErrNotSupported", err)
	}
	if err := f.SetRumble(0, 0.5, 100); err != ErrNotSupported {
		t.Errorf("SetRumble = %v, want ErrNotSupported", err)
	}
	if err := f.SendRaw([]byte{1, 2, 3}); err != ErrNotSupported {
		t.Errorf("SendRaw = %v, want ErrNotSupported", err)
	}
}

func TestLinuxMIDIFeedback_SendMIDI_Empty(t *testing.T) {
	f := &linuxMIDIFeedback{}
	// Empty data should return nil without writing.
	if err := f.SendMIDI(nil); err != nil {
		t.Errorf("SendMIDI(nil) = %v, want nil", err)
	}
	if err := f.SendMIDI([]byte{}); err != nil {
		t.Errorf("SendMIDI([]) = %v, want nil", err)
	}
}
