// Package device provides cross-platform abstractions for input device
// discovery, connection, event reading, and feedback output.
//
// It supports MIDI controllers, gamepads, keyboards with encoders, and
// generic USB HID devices across Linux (evdev/ALSA), macOS (IOKit/CoreMIDI),
// and Windows (XInput/WinMM).
//
// The package defines interfaces that platform-specific providers implement:
//
//   - [DeviceProvider] discovers and enumerates devices
//   - [DeviceConnection] reads events from an opened device
//   - [DeviceFeedback] sends output (LEDs, rumble, MIDI) to a device
//   - [HotPlugWatcher] monitors for connect/disconnect events
//   - [Manager] orchestrates all providers with auto-reconnect
//
// Platform implementations register via [RegisterProvider] in init() functions
// behind build tags. The [PlatformProviders] function returns all providers
// registered for the current build.
package device
