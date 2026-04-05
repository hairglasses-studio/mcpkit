//go:build darwin

package device

// Darwin providers will use IOKit HID Manager and CoreMIDI via CGO.
// For now, no providers are registered — the Manager will return
// "no device providers available" on macOS until CGO providers are added.
//
// Future implementation:
//   - IOKit HID Manager for gamepad/HID enumeration + events
//   - CoreMIDI for MIDI device enumeration + events
//   - GameController.framework as alternative gamepad API
//   - IOKit notifications for hot-plug detection
