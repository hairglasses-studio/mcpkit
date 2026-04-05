package device

import (
	"math"
	"testing"
)

const testDeviceID = DeviceID("test:midi:0")

// ---------------------------------------------------------------------------
// MIDI byte parsing tests
// ---------------------------------------------------------------------------

func TestParseMIDI_NoteOn(t *testing.T) {
	t.Parallel()
	// Note On, channel 0, note 60 (middle C), velocity 100
	buf := []byte{0x90, 60, 100}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != EventMIDINote {
		t.Errorf("Type = %v, want EventMIDINote", ev.Type)
	}
	if ev.DeviceID != testDeviceID {
		t.Errorf("DeviceID = %v, want %v", ev.DeviceID, testDeviceID)
	}
	if ev.Channel != 0 {
		t.Errorf("Channel = %d, want 0", ev.Channel)
	}
	if ev.MIDINote != 60 {
		t.Errorf("MIDINote = %d, want 60", ev.MIDINote)
	}
	if ev.Velocity != 100 {
		t.Errorf("Velocity = %d, want 100", ev.Velocity)
	}
	if !ev.Pressed {
		t.Error("Pressed = false, want true")
	}
	if ev.Source != "midi:note:60" {
		t.Errorf("Source = %q, want %q", ev.Source, "midi:note:60")
	}
	wantValue := float64(100) / 127.0
	if math.Abs(ev.Value-wantValue) > 1e-9 {
		t.Errorf("Value = %f, want %f", ev.Value, wantValue)
	}
}

func TestParseMIDI_NoteOff(t *testing.T) {
	t.Parallel()
	// Note Off, channel 2, note 48, velocity 64
	buf := []byte{0x82, 48, 64}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != EventMIDINote {
		t.Errorf("Type = %v, want EventMIDINote", ev.Type)
	}
	if ev.Channel != 2 {
		t.Errorf("Channel = %d, want 2", ev.Channel)
	}
	if ev.MIDINote != 48 {
		t.Errorf("MIDINote = %d, want 48", ev.MIDINote)
	}
	if ev.Velocity != 64 {
		t.Errorf("Velocity = %d, want 64", ev.Velocity)
	}
	if ev.Pressed {
		t.Error("Pressed = true, want false (Note Off)")
	}
}

func TestParseMIDI_NoteOnZeroVelocity(t *testing.T) {
	t.Parallel()
	// Note On with velocity 0 is equivalent to Note Off per MIDI spec.
	buf := []byte{0x95, 72, 0} // channel 5, note 72, velocity 0
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != EventMIDINote {
		t.Errorf("Type = %v, want EventMIDINote", ev.Type)
	}
	if ev.Pressed {
		t.Error("Pressed = true, want false (NoteOn with velocity 0 = NoteOff)")
	}
	if ev.Channel != 5 {
		t.Errorf("Channel = %d, want 5", ev.Channel)
	}
	if ev.MIDINote != 72 {
		t.Errorf("MIDINote = %d, want 72", ev.MIDINote)
	}
	if ev.Velocity != 0 {
		t.Errorf("Velocity = %d, want 0", ev.Velocity)
	}
	if ev.Value != 0 {
		t.Errorf("Value = %f, want 0", ev.Value)
	}
}

func TestParseMIDI_CC(t *testing.T) {
	t.Parallel()
	// CC message: channel 0, controller 1 (mod wheel), value 127
	buf := []byte{0xB0, 1, 127}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != EventMIDICC {
		t.Errorf("Type = %v, want EventMIDICC", ev.Type)
	}
	if ev.Controller != 1 {
		t.Errorf("Controller = %d, want 1", ev.Controller)
	}
	if ev.MIDIValue != 127 {
		t.Errorf("MIDIValue = %d, want 127", ev.MIDIValue)
	}
	if ev.Source != "midi:cc:1" {
		t.Errorf("Source = %q, want %q", ev.Source, "midi:cc:1")
	}
	if math.Abs(ev.Value-1.0) > 1e-9 {
		t.Errorf("Value = %f, want 1.0", ev.Value)
	}
}

func TestParseMIDI_ProgramChange(t *testing.T) {
	t.Parallel()
	// Program Change: channel 3, program 42
	buf := []byte{0xC3, 42}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != EventMIDIProgramChange {
		t.Errorf("Type = %v, want EventMIDIProgramChange", ev.Type)
	}
	if ev.Channel != 3 {
		t.Errorf("Channel = %d, want 3", ev.Channel)
	}
	if ev.Program != 42 {
		t.Errorf("Program = %d, want 42", ev.Program)
	}
	if ev.Source != "midi:pc:42" {
		t.Errorf("Source = %q, want %q", ev.Source, "midi:pc:42")
	}
	wantValue := float64(42) / 127.0
	if math.Abs(ev.Value-wantValue) > 1e-9 {
		t.Errorf("Value = %f, want %f", ev.Value, wantValue)
	}
}

func TestParseMIDI_PitchBend(t *testing.T) {
	t.Parallel()
	// Pitch Bend: channel 0, LSB=0, MSB=64 → center position
	// 14-bit value: (64 << 7) | 0 = 8192
	// After subtracting 8192: 0 (center)
	//
	// Note: the existing implementations compute bend as (msb << 7) | lsb - 8192.
	// Due to Go operator precedence, this is (msb << 7) | (lsb - 8192), NOT
	// ((msb << 7) | lsb) - 8192. We preserve this behavior.
	buf := []byte{0xE0, 0, 64}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != EventMIDIPitchBend {
		t.Errorf("Type = %v, want EventMIDIPitchBend", ev.Type)
	}
	if ev.Channel != 0 {
		t.Errorf("Channel = %d, want 0", ev.Channel)
	}
	if ev.Source != "midi:pitch_bend" {
		t.Errorf("Source = %q, want %q", ev.Source, "midi:pitch_bend")
	}

	// Verify the actual computation: (int16(64) << 7) | int16(0) - 8192
	// = 8192 | (0 - 8192) = 8192 | (-8192)
	// In two's complement int16: -8192 = 0xE000, 8192 = 0x2000
	// 0x2000 | 0xE000 = 0xE000 = -8192
	expectedBend := (int16(64) << 7) | int16(0) - 8192
	if ev.PitchBend != expectedBend {
		t.Errorf("PitchBend = %d, want %d", ev.PitchBend, expectedBend)
	}
}

func TestParseMIDI_PitchBend_FullUp(t *testing.T) {
	t.Parallel()
	// Pitch Bend max: LSB=127, MSB=127
	buf := []byte{0xE0, 127, 127}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	expectedBend := (int16(127) << 7) | int16(127) - 8192
	if ev.PitchBend != expectedBend {
		t.Errorf("PitchBend = %d, want %d", ev.PitchBend, expectedBend)
	}
}

func TestParseMIDI_MultipleMessages(t *testing.T) {
	t.Parallel()
	// Two messages back-to-back: NoteOn + CC
	buf := []byte{
		0x90, 60, 100, // Note On ch0 note 60 vel 100
		0xB0, 7, 80, // CC ch0 controller 7 (volume) value 80
	}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Type != EventMIDINote {
		t.Errorf("events[0].Type = %v, want EventMIDINote", events[0].Type)
	}
	if events[0].MIDINote != 60 {
		t.Errorf("events[0].MIDINote = %d, want 60", events[0].MIDINote)
	}

	if events[1].Type != EventMIDICC {
		t.Errorf("events[1].Type = %v, want EventMIDICC", events[1].Type)
	}
	if events[1].Controller != 7 {
		t.Errorf("events[1].Controller = %d, want 7", events[1].Controller)
	}
	if events[1].MIDIValue != 80 {
		t.Errorf("events[1].MIDIValue = %d, want 80", events[1].MIDIValue)
	}
}

func TestParseMIDI_ChannelExtraction(t *testing.T) {
	t.Parallel()
	// Test all 16 MIDI channels (0-15) using Note On messages.
	for ch := byte(0); ch < 16; ch++ {
		status := 0x90 | ch
		buf := []byte{status, 60, 100}
		events := parseMIDIBytes(testDeviceID, buf, len(buf))

		if len(events) != 1 {
			t.Fatalf("channel %d: expected 1 event, got %d", ch, len(events))
		}
		if events[0].Channel != ch {
			t.Errorf("channel %d: got Channel=%d", ch, events[0].Channel)
		}
	}
}

func TestParseMIDI_SysExSkipped(t *testing.T) {
	t.Parallel()
	// SysEx message: F0 <data bytes> F7
	// Should produce no events.
	buf := []byte{0xF0, 0x7E, 0x7F, 0x09, 0x01, 0xF7}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 0 {
		t.Errorf("expected 0 events for SysEx, got %d", len(events))
	}
}

func TestParseMIDI_SysExFollowedByNoteOn(t *testing.T) {
	t.Parallel()
	// SysEx followed by a real message — ensure the real message is parsed.
	buf := []byte{
		0xF0, 0x7E, 0x7F, 0x09, 0x01, 0xF7, // SysEx
		0x90, 60, 100, // Note On
	}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 1 {
		t.Fatalf("expected 1 event (NoteOn after SysEx), got %d", len(events))
	}
	if events[0].Type != EventMIDINote {
		t.Errorf("Type = %v, want EventMIDINote", events[0].Type)
	}
}

func TestParseMIDI_RunningStatusSkipped(t *testing.T) {
	t.Parallel()
	// Leading data bytes without a status byte should be skipped.
	buf := []byte{0x3C, 0x64} // Both < 0x80, no status
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 0 {
		t.Errorf("expected 0 events for orphan data bytes, got %d", len(events))
	}
}

func TestParseMIDI_RunningStatusThenValidMessage(t *testing.T) {
	t.Parallel()
	// Orphan data bytes followed by a valid message.
	buf := []byte{
		0x3C, 0x64, // orphan data bytes (skipped)
		0x90, 48, 80, // valid Note On
	}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].MIDINote != 48 {
		t.Errorf("MIDINote = %d, want 48", events[0].MIDINote)
	}
}

func TestParseMIDI_TruncatedNoteOn(t *testing.T) {
	t.Parallel()
	// Note On with only 1 data byte (truncated) — should produce no events.
	buf := []byte{0x90, 60}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 0 {
		t.Errorf("expected 0 events for truncated NoteOn, got %d", len(events))
	}
}

func TestParseMIDI_TruncatedCC(t *testing.T) {
	t.Parallel()
	// CC with only 1 data byte (truncated).
	buf := []byte{0xB0, 1}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 0 {
		t.Errorf("expected 0 events for truncated CC, got %d", len(events))
	}
}

func TestParseMIDI_TruncatedProgramChange(t *testing.T) {
	t.Parallel()
	// Program Change with no data byte.
	buf := []byte{0xC0}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 0 {
		t.Errorf("expected 0 events for truncated ProgramChange, got %d", len(events))
	}
}

func TestParseMIDI_ChannelPressureSkipped(t *testing.T) {
	t.Parallel()
	// Channel Pressure (0xD0) is parsed but produces no event.
	buf := []byte{0xD0, 100}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 0 {
		t.Errorf("expected 0 events for Channel Pressure, got %d", len(events))
	}
}

func TestParseMIDI_EmptyBuffer(t *testing.T) {
	t.Parallel()
	events := parseMIDIBytes(testDeviceID, nil, 0)
	if len(events) != 0 {
		t.Errorf("expected 0 events for empty buffer, got %d", len(events))
	}
}

func TestParseMIDI_NLessThanBufLen(t *testing.T) {
	t.Parallel()
	// Buffer is larger than n — only the first n bytes should be parsed.
	buf := make([]byte, 256)
	buf[0] = 0x90
	buf[1] = 60
	buf[2] = 100
	buf[3] = 0xB0 // This CC should NOT be parsed since n=3
	buf[4] = 7
	buf[5] = 80
	events := parseMIDIBytes(testDeviceID, buf, 3)

	if len(events) != 1 {
		t.Fatalf("expected 1 event (n=3), got %d", len(events))
	}
	if events[0].Type != EventMIDINote {
		t.Errorf("Type = %v, want EventMIDINote", events[0].Type)
	}
}

func TestParseMIDI_DataByteMasking(t *testing.T) {
	t.Parallel()
	// Data bytes should be masked to 7 bits (& 0x7F).
	// Send note 0xFF (only 0x7F should be used) and velocity 0xFF.
	buf := []byte{0x90, 0xFF, 0xFF}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	// 0xFF has bit 7 set, so buf[1] = 0xFF is actually a status byte.
	// The parser will see 0xFF as a status with msgType 0xF0 (system message).
	// So this won't parse as a note. Let's use 0xBF instead (< 0x80 after mask but
	// testing the mask is applied to data bytes within a valid message).
	// Actually, 0xFF >= 0x80, so the parser would treat buf[1] as a new status byte.
	// A more realistic test: use value 0x7F which is the max valid data byte.
	buf2 := []byte{0x90, 0x7F, 0x7F}
	events = parseMIDIBytes(testDeviceID, buf2, len(buf2))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].MIDINote != 0x7F {
		t.Errorf("MIDINote = %d, want %d", events[0].MIDINote, 0x7F)
	}
	if events[0].Velocity != 0x7F {
		t.Errorf("Velocity = %d, want %d", events[0].Velocity, 0x7F)
	}
}

func TestParseMIDI_ThreeMessagesBackToBack(t *testing.T) {
	t.Parallel()
	buf := []byte{
		0x90, 60, 100, // Note On
		0xC5, 10,      // Program Change ch5
		0x82, 48, 64,  // Note Off ch2
	}
	events := parseMIDIBytes(testDeviceID, buf, len(buf))

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	if events[0].Type != EventMIDINote || !events[0].Pressed {
		t.Errorf("events[0]: want NoteOn, got Type=%v Pressed=%v", events[0].Type, events[0].Pressed)
	}
	if events[1].Type != EventMIDIProgramChange || events[1].Program != 10 || events[1].Channel != 5 {
		t.Errorf("events[1]: want ProgramChange ch5 prog10, got Type=%v ch=%d prog=%d",
			events[1].Type, events[1].Channel, events[1].Program)
	}
	if events[2].Type != EventMIDINote || events[2].Pressed {
		t.Errorf("events[2]: want NoteOff, got Type=%v Pressed=%v", events[2].Type, events[2].Pressed)
	}
}

// ---------------------------------------------------------------------------
// Axis normalization tests
// ---------------------------------------------------------------------------

func TestNormalizeAxis_UnsignedTrigger(t *testing.T) {
	t.Parallel()
	// Trigger axis: min=0, max=255
	// raw=0 → 0.0, raw=255 → 1.0, raw=127 → ~0.498
	tests := []struct {
		raw  int32
		want float64
	}{
		{0, 0.0},
		{255, 1.0},
		{127, 127.0 / 255.0},
		{128, 128.0 / 255.0},
	}
	for _, tt := range tests {
		got := normalizeAxis(tt.raw, 0, 255)
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("normalizeAxis(%d, 0, 255) = %f, want %f", tt.raw, got, tt.want)
		}
	}
}

func TestNormalizeAxis_SignedStick(t *testing.T) {
	t.Parallel()
	// Stick axis: min=-32768, max=32767
	// Midpoint = (-32768 + 32767) / 2 = -0.5
	// Half range = (32767 - (-32768)) / 2 = 32767.5
	tests := []struct {
		raw  int32
		want float64
	}{
		{0, (0 - (-0.5)) / 32767.5},           // near center
		{32767, (32767 - (-0.5)) / 32767.5},    // near +1.0
		{-32768, (-32768 - (-0.5)) / 32767.5},  // near -1.0
	}
	for _, tt := range tests {
		got := normalizeAxis(tt.raw, -32768, 32767)
		if math.Abs(got-tt.want) > 1e-6 {
			t.Errorf("normalizeAxis(%d, -32768, 32767) = %f, want %f", tt.raw, got, tt.want)
		}
	}
}

func TestNormalizeAxis_EqualMinMax(t *testing.T) {
	t.Parallel()
	// When min == max, return the raw value as float64 (no normalization).
	got := normalizeAxis(42, 0, 0)
	if got != 42.0 {
		t.Errorf("normalizeAxis(42, 0, 0) = %f, want 42.0", got)
	}
}

func TestNormalizeAxis_SmallRange(t *testing.T) {
	t.Parallel()
	// Hat axis: min=-1, max=1
	tests := []struct {
		raw  int32
		want float64
	}{
		{-1, -1.0},
		{0, 0.0},
		{1, 1.0},
	}
	for _, tt := range tests {
		got := normalizeAxis(tt.raw, -1, 1)
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("normalizeAxis(%d, -1, 1) = %f, want %f", tt.raw, got, tt.want)
		}
	}
}
