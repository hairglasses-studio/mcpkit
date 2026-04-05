//go:build windows

package device

// Windows providers will use XInput (via syscall) and WinMM MIDI.
// For now, no providers are registered.
//
// Future implementation:
//   - XInput via syscall to xinput1_4.dll for Xbox controllers (no CGO)
//   - WinMM via syscall to winmm.dll for MIDI devices (no CGO)
//   - DirectInput via go-ole for legacy controllers
//   - RegisterDeviceNotification for hot-plug detection
