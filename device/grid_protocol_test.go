package device

import (
	"testing"
)

func TestGridEncodeFrame(t *testing.T) {
	frame := GridEncodeFrame("led_value(0,1,200)")
	if frame[0] != gridSTX {
		t.Errorf("expected STX (0x02), got 0x%02x", frame[0])
	}
	if frame[len(frame)-1] != gridETX {
		t.Errorf("expected ETX (0x03), got 0x%02x", frame[len(frame)-1])
	}
	payload := string(frame[1 : len(frame)-1])
	if payload != "led_value(0,1,200)" {
		t.Errorf("payload = %q, want %q", payload, "led_value(0,1,200)")
	}
}

func TestGridEncodeLEDColor(t *testing.T) {
	frame := GridEncodeLEDColor(3, GridLEDLayer1, 255, 0, 128)
	payload := string(frame[1 : len(frame)-1])
	want := "led_color(3,1,255,0,128,0)"
	if payload != want {
		t.Errorf("payload = %q, want %q", payload, want)
	}
}

func TestGridEncodeLEDValue(t *testing.T) {
	frame := GridEncodeLEDValue(15, GridLEDLayer2, 200)
	payload := string(frame[1 : len(frame)-1])
	want := "led_value(15,2,200)"
	if payload != want {
		t.Errorf("payload = %q, want %q", payload, want)
	}
}

func TestGridEncodeLEDAnimation(t *testing.T) {
	frame := GridEncodeLEDAnimation(0, GridLEDLayer1, 0, 128, GridAnimSine)
	payload := string(frame[1 : len(frame)-1])
	want := "led_animation_phase_rate_type(0,1,0,128,4)"
	if payload != want {
		t.Errorf("payload = %q, want %q", payload, want)
	}
}

func TestGridEncodeLEDAnimation_Stop(t *testing.T) {
	frame := GridEncodeLEDAnimation(5, GridLEDLayer2, 0, 0, GridAnimNone)
	payload := string(frame[1 : len(frame)-1])
	want := "led_animation_phase_rate_type(5,2,0,0,0)"
	if payload != want {
		t.Errorf("payload = %q, want %q", payload, want)
	}
}

func TestGridEncodeLEDColorMinMidMax(t *testing.T) {
	tests := []struct {
		name    string
		frame   []byte
		wantCmd string
	}{
		{"min", GridEncodeLEDColorMin(0, GridLEDLayer1, 0, 50, 0), "led_color_min(0,1,0,50,0)"},
		{"mid", GridEncodeLEDColorMid(0, GridLEDLayer1, 128, 128, 0), "led_color_mid(0,1,128,128,0)"},
		{"max", GridEncodeLEDColorMax(0, GridLEDLayer1, 255, 0, 0), "led_color_max(0,1,255,0,0)"},
	}
	for _, tt := range tests {
		payload := string(tt.frame[1 : len(tt.frame)-1])
		if payload != tt.wantCmd {
			t.Errorf("%s: payload = %q, want %q", tt.name, payload, tt.wantCmd)
		}
	}
}

func TestGridParseFrames_Single(t *testing.T) {
	data := []byte{gridSTX}
	data = append(data, []byte("hello")...)
	data = append(data, gridETX)

	msgs, remainder := GridParseFrames(data)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Payload != "hello" {
		t.Errorf("payload = %q, want %q", msgs[0].Payload, "hello")
	}
	if remainder != nil {
		t.Errorf("expected nil remainder, got %v", remainder)
	}
}

func TestGridParseFrames_Multiple(t *testing.T) {
	var data []byte
	data = append(data, gridSTX)
	data = append(data, []byte("msg1")...)
	data = append(data, gridETX)
	data = append(data, gridSTX)
	data = append(data, []byte("msg2")...)
	data = append(data, gridETX)

	msgs, remainder := GridParseFrames(data)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Payload != "msg1" || msgs[1].Payload != "msg2" {
		t.Errorf("payloads = %q, %q", msgs[0].Payload, msgs[1].Payload)
	}
	if remainder != nil {
		t.Errorf("expected nil remainder, got %v", remainder)
	}
}

func TestGridParseFrames_Incomplete(t *testing.T) {
	var data []byte
	data = append(data, gridSTX)
	data = append(data, []byte("partial")...)
	// No ETX — incomplete.

	msgs, remainder := GridParseFrames(data)
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages for incomplete frame, got %d", len(msgs))
	}
	if len(remainder) != len(data) {
		t.Errorf("expected %d remainder bytes, got %d", len(data), len(remainder))
	}
}

func TestGridParseFrames_GarbagePrefix(t *testing.T) {
	var data []byte
	data = append(data, []byte("garbage")...) // No STX.
	data = append(data, gridSTX)
	data = append(data, []byte("real")...)
	data = append(data, gridETX)

	msgs, _ := GridParseFrames(data)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message after garbage, got %d", len(msgs))
	}
	if msgs[0].Payload != "real" {
		t.Errorf("payload = %q, want %q", msgs[0].Payload, "real")
	}
}

func TestGridParseFrames_Empty(t *testing.T) {
	frame := GridEncodeHeartbeat()
	msgs, _ := GridParseFrames(frame)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message for heartbeat, got %d", len(msgs))
	}
	if msgs[0].Payload != "" {
		t.Errorf("heartbeat payload = %q, want empty", msgs[0].Payload)
	}
}

func TestIsGridDevice(t *testing.T) {
	tests := []struct {
		vid, pid uint16
		want     bool
	}{
		{GridVIDGen2, GridPIDGen2, true},
		{GridVIDGen1, GridPIDGen1, true},
		{0x1234, 0x5678, false},
		{GridVIDGen2, 0x0000, false},
	}
	for _, tt := range tests {
		got := IsGridDevice(tt.vid, tt.pid)
		if got != tt.want {
			t.Errorf("IsGridDevice(0x%04x, 0x%04x) = %v, want %v", tt.vid, tt.pid, got, tt.want)
		}
	}
}

func TestIsGridDeviceName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Intech Grid CDC device", true},
		{"Intech Grid MIDI device", false}, // MIDI not CDC.
		{"Random USB device", false},
		{"Grid CDC", true}, // Minimum match.
	}
	for _, tt := range tests {
		got := IsGridDeviceName(tt.name)
		if got != tt.want {
			t.Errorf("IsGridDeviceName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
