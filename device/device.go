package device

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Device types and classification
// ---------------------------------------------------------------------------

// DeviceType classifies a device into a functional category.
type DeviceType int

const (
	TypeUnknown     DeviceType = iota
	TypeMIDI                          // USB class-compliant, RTP-MIDI, BLE-MIDI
	TypeGamepad                       // Xbox, PlayStation, Nintendo, generic
	TypeKeyboard                      // QMK/VIA keyboard with encoders
	TypeMouse                         // Mice with extra buttons
	TypeGenericHID                    // Stream Deck, custom macropads
	TypeHOTAS                         // Flight sticks, throttle quadrants
	TypeRacingWheel                   // Steering wheels, pedals
)

func (t DeviceType) String() string {
	switch t {
	case TypeMIDI:
		return "midi"
	case TypeGamepad:
		return "gamepad"
	case TypeKeyboard:
		return "keyboard"
	case TypeMouse:
		return "mouse"
	case TypeGenericHID:
		return "generic_hid"
	case TypeHOTAS:
		return "hotas"
	case TypeRacingWheel:
		return "racing_wheel"
	default:
		return "unknown"
	}
}

// ConnectionType indicates how the device is attached.
type ConnectionType int

const (
	ConnUSB             ConnectionType = iota
	ConnBluetooth
	ConnWirelessDongle
	ConnNetwork                        // RTP-MIDI, network MIDI
	ConnVirtual                        // Software-defined devices
)

func (c ConnectionType) String() string {
	switch c {
	case ConnUSB:
		return "usb"
	case ConnBluetooth:
		return "bluetooth"
	case ConnWirelessDongle:
		return "wireless_dongle"
	case ConnNetwork:
		return "network"
	case ConnVirtual:
		return "virtual"
	default:
		return "unknown"
	}
}

// DeviceID uniquely identifies a connected device within a session.
type DeviceID string

// ---------------------------------------------------------------------------
// Device info
// ---------------------------------------------------------------------------

// Info describes a discovered device without opening a connection.
type Info struct {
	ID           DeviceID       `json:"id"`
	Name         string         `json:"name"`
	Type         DeviceType     `json:"type"`
	Connection   ConnectionType `json:"connection"`
	VendorID     uint16         `json:"vendor_id"`
	ProductID    uint16         `json:"product_id"`
	Manufacturer string         `json:"manufacturer,omitempty"`
	Serial       string         `json:"serial,omitempty"`
	Capabilities Capabilities   `json:"capabilities"`
	PlatformPath string         `json:"platform_path"` // OS-specific path
	ProviderName string         `json:"provider_name"` // Which provider owns this
}

// Capabilities describes what a device can do.
type Capabilities struct {
	Buttons    int  `json:"buttons,omitempty"`
	Axes       int  `json:"axes,omitempty"`
	Hats       int  `json:"hats,omitempty"`
	HasRumble  bool `json:"has_rumble,omitempty"`
	HasLEDs    bool `json:"has_leds,omitempty"`
	MIDIPorts  int  `json:"midi_ports,omitempty"`
	Encoders   int  `json:"encoders,omitempty"`
	Touchpads  int  `json:"touchpads,omitempty"`
	MotionAxes int  `json:"motion_axes,omitempty"` // gyro/accel
}

// ---------------------------------------------------------------------------
// Core interfaces
// ---------------------------------------------------------------------------

// DeviceProvider discovers and enumerates devices of a specific class.
// Each platform has one or more providers (e.g., Linux has evdev + ALSA).
type DeviceProvider interface {
	// Name returns the provider identifier (e.g., "evdev", "coremidi", "xinput").
	Name() string

	// DeviceTypes returns which device types this provider handles.
	DeviceTypes() []DeviceType

	// Enumerate scans for currently connected devices.
	Enumerate(ctx context.Context) ([]Info, error)

	// Open opens a connection to a specific device for reading events.
	Open(ctx context.Context, id DeviceID) (DeviceConnection, error)

	// Close releases all resources held by this provider.
	Close() error
}

// DeviceConnection reads events from a single opened device.
type DeviceConnection interface {
	// Info returns the device info for this connection.
	Info() Info

	// Start begins the event read loop. Events arrive on the Events() channel.
	Start(ctx context.Context) error

	// Events returns a channel that yields device events.
	// The channel is closed when the connection is stopped or the device disconnects.
	Events() <-chan Event

	// Feedback returns a DeviceFeedback if the device supports output, or nil.
	Feedback() DeviceFeedback

	// Close stops reading and releases the device handle.
	Close() error

	// Alive reports whether the device is still connected and responsive.
	Alive() bool
}

// DeviceFeedback sends output to a device (LEDs, rumble motors, MIDI out).
type DeviceFeedback interface {
	// SetLED sets an LED by index to an RGBA color.
	SetLED(index int, r, g, b, a uint8) error

	// SetRumble sets rumble motor intensity (0.0 to 1.0) for the given motor index.
	SetRumble(motor int, intensity float64, duration time.Duration) error

	// SendMIDI sends a raw MIDI message (for controllers with output ports).
	SendMIDI(data []byte) error

	// SendRaw sends raw bytes to the device (for generic HID).
	SendRaw(data []byte) error
}

// HotPlugWatcher monitors for device connect/disconnect events.
type HotPlugWatcher interface {
	// Start begins monitoring. Events are delivered via the channel.
	Start(ctx context.Context) error

	// Events returns the hot-plug event channel.
	Events() <-chan HotPlugEvent

	// Close stops monitoring and releases resources.
	Close() error
}

// HotPlugType indicates connect or disconnect.
type HotPlugType int

const (
	HotPlugConnect    HotPlugType = iota
	HotPlugDisconnect
)

// HotPlugEvent describes a device being plugged or unplugged.
type HotPlugEvent struct {
	Type      HotPlugType `json:"type"`
	Info      Info        `json:"info"`
	Timestamp time.Time   `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// ErrDeviceDisconnected is returned when a device unexpectedly disconnects.
var ErrDeviceDisconnected = errors.New("device disconnected")

// ErrNotSupported is returned when a capability is not available.
var ErrNotSupported = errors.New("not supported by this device")

// ErrDeviceNotFound is returned when a device ID cannot be resolved.
var ErrDeviceNotFound = errors.New("device not found")

// ---------------------------------------------------------------------------
// Provider registry
// ---------------------------------------------------------------------------

// providerRegistry holds platform-registered provider factories.
var providerRegistry []func() DeviceProvider

// RegisterProvider is called from init() in platform-specific files.
func RegisterProvider(factory func() DeviceProvider) {
	providerRegistry = append(providerRegistry, factory)
}

// PlatformProviders returns all providers registered for the current build.
func PlatformProviders() []DeviceProvider {
	providers := make([]DeviceProvider, 0, len(providerRegistry))
	for _, f := range providerRegistry {
		providers = append(providers, f())
	}
	return providers
}

// ---------------------------------------------------------------------------
// Event types
// ---------------------------------------------------------------------------

// EventType classifies what kind of input occurred.
type EventType int

const (
	EventButton        EventType = iota // Digital press/release
	EventAxis                           // Analog axis change
	EventHat                            // D-pad / hat switch
	EventMIDINote                       // MIDI note on/off
	EventMIDICC                         // MIDI continuous controller
	EventMIDIProgramChange              // MIDI program change
	EventMIDIPitchBend                  // MIDI pitch bend
	EventMIDISysEx                      // MIDI system exclusive
	EventKey                            // Keyboard key press/release
	EventEncoder                        // Rotary encoder step
)

func (t EventType) String() string {
	switch t {
	case EventButton:
		return "button"
	case EventAxis:
		return "axis"
	case EventHat:
		return "hat"
	case EventMIDINote:
		return "midi_note"
	case EventMIDICC:
		return "midi_cc"
	case EventMIDIProgramChange:
		return "midi_program_change"
	case EventMIDIPitchBend:
		return "midi_pitch_bend"
	case EventMIDISysEx:
		return "midi_sysex"
	case EventKey:
		return "key"
	case EventEncoder:
		return "encoder"
	default:
		return fmt.Sprintf("event_%d", t)
	}
}

// Event is the unified event emitted by all device types.
type Event struct {
	DeviceID  DeviceID  `json:"device_id"`
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`

	// Source is the canonical input identifier for the mapping engine.
	// For evdev: "BTN_SOUTH", "ABS_X"
	// For MIDI: "midi:cc:1", "midi:note:60"
	// For HID: "hid:usage:0x09:0x01"
	Source string `json:"source"`

	// Button/Key fields
	Code    uint16 `json:"code,omitempty"`
	Pressed bool   `json:"pressed,omitempty"`

	// Axis fields
	Value float64 `json:"value,omitempty"` // Normalized: -1.0 to 1.0 (stick) or 0.0 to 1.0 (trigger)

	// Hat fields
	HatX int8 `json:"hat_x,omitempty"` // -1, 0, +1
	HatY int8 `json:"hat_y,omitempty"` // -1, 0, +1

	// MIDI fields
	Channel    uint8  `json:"channel,omitempty"`
	MIDINote   uint8  `json:"midi_note,omitempty"`
	Velocity   uint8  `json:"velocity,omitempty"`
	Controller uint8  `json:"controller,omitempty"` // CC number
	MIDIValue  uint8  `json:"midi_value,omitempty"` // CC value
	Program    uint8  `json:"program,omitempty"`
	PitchBend  int16  `json:"pitch_bend,omitempty"` // -8192 to 8191
	SysEx      []byte `json:"sysex,omitempty"`

	// Raw platform data (not serialized)
	RawValue int32 `json:"-"`
}
