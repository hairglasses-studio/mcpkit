//go:build darwin

package device

import (
	"bytes"
	"errors"
	"image/jpeg"
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

// ---------------------------------------------------------------------------
// darwinIOKitFeedback tests — Stream Deck LED control
// ---------------------------------------------------------------------------

func TestDarwinIOKitFeedback_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	var _ DeviceFeedback = (*darwinIOKitFeedback)(nil)
}

func TestDarwinIOKitFeedback_SetRumble_NotSupported(t *testing.T) {
	t.Parallel()
	fb := &darwinIOKitFeedback{
		model: streamDeckModels[0x0080],
	}
	err := fb.SetRumble(0, 1.0, time.Second)
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("SetRumble: got %v, want ErrNotSupported", err)
	}
}

func TestDarwinIOKitFeedback_SendMIDI_NotSupported(t *testing.T) {
	t.Parallel()
	fb := &darwinIOKitFeedback{
		model: streamDeckModels[0x0080],
	}
	err := fb.SendMIDI([]byte{0x90, 60, 100})
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("SendMIDI: got %v, want ErrNotSupported", err)
	}
}

func TestDarwinIOKitFeedback_SendRaw_EmptyData(t *testing.T) {
	t.Parallel()
	fb := &darwinIOKitFeedback{
		model: streamDeckModels[0x0080],
	}
	if err := fb.SendRaw(nil); err != nil {
		t.Errorf("SendRaw(nil): got %v, want nil", err)
	}
	if err := fb.SendRaw([]byte{}); err != nil {
		t.Errorf("SendRaw([]byte{}): got %v, want nil", err)
	}
}

func TestDarwinIOKitFeedback_SetLED_IndexOutOfRange(t *testing.T) {
	t.Parallel()
	fb := &darwinIOKitFeedback{
		model: streamDeckModels[0x0080], // MK.2: 15 keys
	}

	// Negative index.
	err := fb.SetLED(-1, 255, 0, 0, 255)
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("SetLED(-1): got %v, want ErrNotSupported", err)
	}

	// Index equal to key count (out of bounds).
	err = fb.SetLED(15, 255, 0, 0, 255)
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("SetLED(15): got %v, want ErrNotSupported", err)
	}

	// Way out of range.
	err = fb.SetLED(100, 255, 0, 0, 255)
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("SetLED(100): got %v, want ErrNotSupported", err)
	}
}

func TestDarwinIOKitFeedback_SetLED_PedalNoDisplay(t *testing.T) {
	t.Parallel()
	fb := &darwinIOKitFeedback{
		model: streamDeckModels[0x0084], // Pedal: 3 keys, no display
	}
	err := fb.SetLED(0, 255, 0, 0, 255)
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("SetLED on pedal: got %v, want ErrNotSupported", err)
	}
}

func TestDarwinIOKitConnection_Feedback_NilWithoutStreamDeck(t *testing.T) {
	t.Parallel()
	conn := &darwinIOKitConnection{
		deviceInfo: Info{ID: "iokit_hid:045e:0b13:0", Type: TypeGamepad},
		events:     make(chan Event, 1),
	}
	if fb := conn.Feedback(); fb != nil {
		t.Errorf("Feedback() for non-StreamDeck = %v, want nil", fb)
	}
}

func TestDarwinIOKitConnection_Feedback_NonNilForStreamDeck(t *testing.T) {
	t.Parallel()
	conn := &darwinIOKitConnection{
		deviceInfo: Info{ID: "iokit_hid:0fd9:0080:0", Type: TypeGenericHID},
		events:     make(chan Event, 1),
		feedback: &darwinIOKitFeedback{
			model: streamDeckModels[0x0080],
		},
	}
	if fb := conn.Feedback(); fb == nil {
		t.Error("Feedback() for Stream Deck MK.2 = nil, want non-nil")
	}
}

// ---------------------------------------------------------------------------
// solidColorJPEG tests
// ---------------------------------------------------------------------------

func TestSolidColorJPEG_ValidOutput(t *testing.T) {
	t.Parallel()
	data, err := solidColorJPEG(72, 72, 255, 0, 0)
	if err != nil {
		t.Fatalf("solidColorJPEG: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("solidColorJPEG returned empty data")
	}

	// Verify it's valid JPEG by decoding.
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("jpeg.Decode: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 72 || bounds.Dy() != 72 {
		t.Errorf("image size = %dx%d, want 72x72", bounds.Dx(), bounds.Dy())
	}
}

func TestSolidColorJPEG_DifferentSizes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		w, h int
	}{
		{"80x80 (Mini)", 80, 80},
		{"96x96 (XL)", 96, 96},
		{"120x120 (Plus)", 120, 120},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := solidColorJPEG(tt.w, tt.h, 0, 255, 0)
			if err != nil {
				t.Fatalf("solidColorJPEG(%d,%d): %v", tt.w, tt.h, err)
			}
			img, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("jpeg.Decode: %v", err)
			}
			b := img.Bounds()
			if b.Dx() != tt.w || b.Dy() != tt.h {
				t.Errorf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), tt.w, tt.h)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Stream Deck model database tests
// ---------------------------------------------------------------------------

func TestStreamDeckModels_AllRegistered(t *testing.T) {
	t.Parallel()
	// Every known Stream Deck product ID from classify.go should have a model entry.
	knownPIDs := []uint16{0x0060, 0x006d, 0x0063, 0x0080, 0x0084, 0x0086, 0x008f}
	for _, pid := range knownPIDs {
		if _, ok := streamDeckModels[pid]; !ok {
			t.Errorf("Stream Deck product 0x%04x missing from streamDeckModels", pid)
		}
	}
}

func TestStreamDeckModels_ValidProtocolParams(t *testing.T) {
	t.Parallel()
	for pid, model := range streamDeckModels {
		if model.keys <= 0 {
			t.Errorf("0x%04x: keys = %d, want > 0", pid, model.keys)
		}
		if model.pageSize <= model.headerLen {
			t.Errorf("0x%04x: pageSize %d <= headerLen %d", pid, model.pageSize, model.headerLen)
		}
		if model.imgReport <= 0 {
			t.Errorf("0x%04x: imgReport = %d, want > 0", pid, model.imgReport)
		}
		// Non-pedal models must have positive icon dimensions.
		if pid != 0x0084 {
			if model.iconW <= 0 || model.iconH <= 0 {
				t.Errorf("0x%04x: icon size %dx%d, want positive for display models", pid, model.iconW, model.iconH)
			}
		}
	}
}

func TestElgatoVendorID(t *testing.T) {
	t.Parallel()
	if elgatoVendorID != 0x0fd9 {
		t.Errorf("elgatoVendorID = 0x%04x, want 0x0fd9", elgatoVendorID)
	}
}
