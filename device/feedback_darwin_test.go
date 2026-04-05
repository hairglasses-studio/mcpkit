//go:build darwin

package device

import (
	"errors"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// darwinMIDIFeedback tests — verifies unsupported methods and interface compliance
// ---------------------------------------------------------------------------

func TestDarwinMIDIFeedback_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	// Compile-time check that darwinMIDIFeedback satisfies DeviceFeedback.
	var _ DeviceFeedback = (*darwinMIDIFeedback)(nil)
}

func TestDarwinMIDIFeedback_SetLED_NotSupported(t *testing.T) {
	t.Parallel()
	fb := &darwinMIDIFeedback{}
	err := fb.SetLED(0, 255, 0, 0, 255)
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("SetLED: got %v, want ErrNotSupported", err)
	}
}

func TestDarwinMIDIFeedback_SetRumble_NotSupported(t *testing.T) {
	t.Parallel()
	fb := &darwinMIDIFeedback{}
	err := fb.SetRumble(0, 1.0, time.Second)
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("SetRumble: got %v, want ErrNotSupported", err)
	}
}

func TestDarwinMIDIFeedback_SendRaw_NotSupported(t *testing.T) {
	t.Parallel()
	fb := &darwinMIDIFeedback{}
	err := fb.SendRaw([]byte{0x01, 0x02})
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("SendRaw: got %v, want ErrNotSupported", err)
	}
}

func TestDarwinMIDIFeedback_SendMIDI_EmptyData(t *testing.T) {
	t.Parallel()
	fb := &darwinMIDIFeedback{}
	// Empty data should be a no-op, not an error.
	if err := fb.SendMIDI(nil); err != nil {
		t.Errorf("SendMIDI(nil): got %v, want nil", err)
	}
	if err := fb.SendMIDI([]byte{}); err != nil {
		t.Errorf("SendMIDI([]byte{}): got %v, want nil", err)
	}
}

// TestDarwinMIDIConnection_Feedback_NilBeforeStart verifies that Feedback()
// returns nil when the connection has not been started (no output port setup).
func TestDarwinMIDIConnection_Feedback_NilBeforeStart(t *testing.T) {
	t.Parallel()
	conn := &darwinMIDIConnection{
		deviceInfo: Info{ID: "coremidi:0", Type: TypeMIDI},
		events:     make(chan Event, 1),
	}
	if fb := conn.Feedback(); fb != nil {
		t.Errorf("Feedback() before Start() = %v, want nil", fb)
	}
}
