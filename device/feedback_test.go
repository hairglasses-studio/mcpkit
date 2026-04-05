package device

import (
	"errors"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Cross-platform feedback interface tests using a mock
// ---------------------------------------------------------------------------

// mockFeedbackUnsupported implements DeviceFeedback with all methods returning
// ErrNotSupported, simulating a MIDI device that only supports SendMIDI.
type mockFeedbackUnsupported struct{}

func (f *mockFeedbackUnsupported) SetLED(int, uint8, uint8, uint8, uint8) error {
	return ErrNotSupported
}
func (f *mockFeedbackUnsupported) SetRumble(int, float64, time.Duration) error {
	return ErrNotSupported
}
func (f *mockFeedbackUnsupported) SendMIDI([]byte) error { return nil }
func (f *mockFeedbackUnsupported) SendRaw([]byte) error  { return ErrNotSupported }

func TestDeviceFeedback_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	// Verify the mock satisfies the interface at compile time.
	var fb DeviceFeedback = &mockFeedbackUnsupported{}
	_ = fb
}

func TestDeviceFeedback_UnsupportedMethods(t *testing.T) {
	t.Parallel()
	fb := &mockFeedbackUnsupported{}

	if err := fb.SetLED(0, 255, 0, 0, 255); !errors.Is(err, ErrNotSupported) {
		t.Errorf("SetLED: got %v, want ErrNotSupported", err)
	}
	if err := fb.SetRumble(0, 1.0, time.Second); !errors.Is(err, ErrNotSupported) {
		t.Errorf("SetRumble: got %v, want ErrNotSupported", err)
	}
	if err := fb.SendRaw([]byte{0x01}); !errors.Is(err, ErrNotSupported) {
		t.Errorf("SendRaw: got %v, want ErrNotSupported", err)
	}
}

func TestDeviceFeedback_SendMIDI_EmptyData(t *testing.T) {
	t.Parallel()
	fb := &mockFeedbackUnsupported{}
	if err := fb.SendMIDI(nil); err != nil {
		t.Errorf("SendMIDI(nil): got %v, want nil", err)
	}
	if err := fb.SendMIDI([]byte{}); err != nil {
		t.Errorf("SendMIDI([]byte{}): got %v, want nil", err)
	}
}

func TestDeviceFeedback_SendMIDI_NoteOn(t *testing.T) {
	t.Parallel()
	fb := &mockFeedbackUnsupported{}
	// Note On, channel 0, note 60, velocity 100
	data := []byte{0x90, 60, 100}
	if err := fb.SendMIDI(data); err != nil {
		t.Errorf("SendMIDI(NoteOn): got %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// Connection Feedback() nil-return tests
// ---------------------------------------------------------------------------

func TestMockConnection_Feedback_ReturnsNil(t *testing.T) {
	t.Parallel()
	conn := &mockConnection{
		info:   Info{ID: "test-dev"},
		events: make(chan Event),
		alive:  true,
	}
	if fb := conn.Feedback(); fb != nil {
		t.Errorf("mockConnection.Feedback() = %v, want nil", fb)
	}
}
