package device

import (
	"context"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Classification tests
// ---------------------------------------------------------------------------

func TestClassifyDevice_Xbox(t *testing.T) {
	dt := ClassifyDevice(0x045e, 0x0b13, "Microsoft Xbox Series S|X Controller")
	if dt != TypeGamepad {
		t.Errorf("Xbox controller classified as %v, want gamepad", dt)
	}
}

func TestClassifyDevice_PlayStation(t *testing.T) {
	dt := ClassifyDevice(0x054c, 0x0ce6, "DualSense Wireless Controller")
	if dt != TypeGamepad {
		t.Errorf("DualSense classified as %v, want gamepad", dt)
	}
}

func TestClassifyDevice_StreamDeck(t *testing.T) {
	dt := ClassifyDevice(0x0fd9, 0x0060, "Stream Deck")
	if dt != TypeGenericHID {
		t.Errorf("Stream Deck classified as %v, want generic_hid", dt)
	}
}

func TestClassifyDevice_MIDI_ByVendor(t *testing.T) {
	dt := ClassifyDevice(0x1c75, 0x0001, "Arturia BeatStep Pro")
	if dt != TypeMIDI {
		t.Errorf("Arturia classified as %v, want midi", dt)
	}
}

func TestClassifyDevice_MIDI_ByName(t *testing.T) {
	dt := ClassifyDevice(0x0000, 0x0000, "USB MIDI Controller")
	if dt != TypeMIDI {
		t.Errorf("MIDI by name classified as %v, want midi", dt)
	}
}

func TestClassifyDevice_HOTAS(t *testing.T) {
	dt := ClassifyDevice(0x044f, 0x0000, "Thrustmaster Flight Stick")
	if dt != TypeHOTAS {
		t.Errorf("HOTAS classified as %v, want hotas", dt)
	}
}

func TestClassifyDevice_Keyboard(t *testing.T) {
	dt := ClassifyDevice(0x0000, 0x0000, "Keychron V2 Keyboard")
	if dt != TypeKeyboard {
		t.Errorf("Keyboard classified as %v, want keyboard", dt)
	}
}

func TestClassifyDevice_Mouse(t *testing.T) {
	dt := ClassifyDevice(0x0000, 0x0000, "Logitech MX Master Mouse")
	if dt != TypeMouse {
		t.Errorf("Mouse classified as %v, want mouse", dt)
	}
}

func TestClassifyDevice_IntechGrid_Gen1(t *testing.T) {
	dt := ClassifyDevice(0x03eb, 0xecad, "Intech Studio: Grid")
	if dt != TypeMIDI {
		t.Errorf("Intech Grid Gen1 classified as %v, want midi", dt)
	}
}

func TestClassifyDevice_IntechGrid_Gen2(t *testing.T) {
	dt := ClassifyDevice(0x303a, 0x8123, "Grid")
	if dt != TypeMIDI {
		t.Errorf("Intech Grid Gen2 classified as %v, want midi", dt)
	}
}

func TestClassifyDevice_IntechGrid_ByName(t *testing.T) {
	dt := ClassifyDevice(0x0000, 0x0000, "Intech Grid MIDI device")
	if dt != TypeMIDI {
		t.Errorf("Intech Grid by name classified as %v, want midi", dt)
	}
}

func TestClassifyDevice_IntechGrid_ByVendor(t *testing.T) {
	// Unknown PID but known Intech VID should classify via vendor brand heuristic.
	dt := ClassifyDevice(0x303a, 0xFFFF, "Some Future Grid Module")
	if dt != TypeMIDI {
		t.Errorf("Intech vendor 0x303a classified as %v, want midi", dt)
	}
}

func TestBrandLabel_Intech(t *testing.T) {
	if got := BrandLabel(0x303a); got != "intech" {
		t.Errorf("BrandLabel(0x303a) = %q, want intech", got)
	}
}

func TestClassifyDevice_Unknown(t *testing.T) {
	dt := ClassifyDevice(0x1234, 0x5678, "Mystery Device")
	if dt != TypeUnknown {
		t.Errorf("Unknown device classified as %v, want unknown", dt)
	}
}

func TestClassifyDevice_RacingWheel(t *testing.T) {
	dt := ClassifyDevice(0x0000, 0x0000, "Racing Wheel Pro")
	if dt != TypeRacingWheel {
		t.Errorf("Racing wheel classified as %v, want racing_wheel", dt)
	}
}

// ---------------------------------------------------------------------------
// Type string tests
// ---------------------------------------------------------------------------

func TestDeviceType_String(t *testing.T) {
	tests := []struct {
		dt   DeviceType
		want string
	}{
		{TypeMIDI, "midi"},
		{TypeGamepad, "gamepad"},
		{TypeKeyboard, "keyboard"},
		{TypeGenericHID, "generic_hid"},
		{TypeHOTAS, "hotas"},
		{TypeRacingWheel, "racing_wheel"},
		{TypeUnknown, "unknown"},
	}
	for _, tt := range tests {
		if got := tt.dt.String(); got != tt.want {
			t.Errorf("DeviceType(%d).String() = %q, want %q", tt.dt, got, tt.want)
		}
	}
}

func TestEventType_String(t *testing.T) {
	tests := []struct {
		et   EventType
		want string
	}{
		{EventButton, "button"},
		{EventAxis, "axis"},
		{EventMIDICC, "midi_cc"},
		{EventMIDINote, "midi_note"},
		{EventEncoder, "encoder"},
	}
	for _, tt := range tests {
		if got := tt.et.String(); got != tt.want {
			t.Errorf("EventType(%d).String() = %q, want %q", tt.et, got, tt.want)
		}
	}
}

func TestBrandLabel(t *testing.T) {
	if got := BrandLabel(0x045e); got != "xbox" {
		t.Errorf("BrandLabel(0x045e) = %q, want xbox", got)
	}
	if got := BrandLabel(0xFFFF); got != "unknown" {
		t.Errorf("BrandLabel(0xFFFF) = %q, want unknown", got)
	}
}

// ---------------------------------------------------------------------------
// Manager tests (with mock provider)
// ---------------------------------------------------------------------------

type mockProvider struct {
	name    string
	types   []DeviceType
	devices []Info
	openFn  func(DeviceID) (DeviceConnection, error)
}

func (m *mockProvider) Name() string                                    { return m.name }
func (m *mockProvider) DeviceTypes() []DeviceType                       { return m.types }
func (m *mockProvider) Enumerate(_ context.Context) ([]Info, error)     { return m.devices, nil }
func (m *mockProvider) Open(_ context.Context, id DeviceID) (DeviceConnection, error) {
	if m.openFn != nil {
		return m.openFn(id)
	}
	return nil, ErrNotSupported
}
func (m *mockProvider) Close() error { return nil }

type mockConnection struct {
	info    Info
	events  chan Event
	alive   bool
	closed  bool
}

func (c *mockConnection) Info() Info                          { return c.info }
func (c *mockConnection) Start(_ context.Context) error       { return nil }
func (c *mockConnection) Events() <-chan Event                { return c.events }
func (c *mockConnection) Feedback() DeviceFeedback            { return nil }
func (c *mockConnection) Close() error                        { c.closed = true; close(c.events); return nil }
func (c *mockConnection) Alive() bool                         { return c.alive }

func TestManager_ListDevices(t *testing.T) {
	devices := []Info{
		{ID: "dev1", Name: "Controller 1", Type: TypeGamepad},
		{ID: "dev2", Name: "MIDI Device", Type: TypeMIDI},
	}

	// Register mock provider.
	origRegistry := providerRegistry
	providerRegistry = []func() DeviceProvider{
		func() DeviceProvider {
			return &mockProvider{name: "mock", types: []DeviceType{TypeGamepad, TypeMIDI}, devices: devices}
		},
	}
	defer func() { providerRegistry = origRegistry }()

	mgr := NewManager(ManagerConfig{})
	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Close()

	listed := mgr.ListDevices()
	if len(listed) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(listed))
	}
}

func TestManager_GetDevice(t *testing.T) {
	devices := []Info{
		{ID: "test-dev", Name: "Test Device", Type: TypeGamepad, VendorID: 0x045e},
	}

	origRegistry := providerRegistry
	providerRegistry = []func() DeviceProvider{
		func() DeviceProvider {
			return &mockProvider{name: "mock", devices: devices}
		},
	}
	defer func() { providerRegistry = origRegistry }()

	mgr := NewManager(ManagerConfig{})
	mgr.Start(context.Background())
	defer mgr.Close()

	info, err := mgr.GetDevice("test-dev")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if info.Name != "Test Device" {
		t.Errorf("Name = %q, want 'Test Device'", info.Name)
	}

	_, err = mgr.GetDevice("nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestManager_ConnectAndEvents(t *testing.T) {
	eventCh := make(chan Event, 10)
	conn := &mockConnection{
		info:   Info{ID: "dev1", Name: "Test"},
		events: eventCh,
		alive:  true,
	}

	devices := []Info{{ID: "dev1", Name: "Test", Type: TypeGamepad}}
	origRegistry := providerRegistry
	providerRegistry = []func() DeviceProvider{
		func() DeviceProvider {
			return &mockProvider{
				name:    "mock",
				devices: devices,
				openFn: func(id DeviceID) (DeviceConnection, error) {
					return conn, nil
				},
			}
		},
	}
	defer func() { providerRegistry = origRegistry }()

	mgr := NewManager(ManagerConfig{EventBuffer: 10})
	mgr.Start(context.Background())
	defer mgr.Close()

	if err := mgr.Connect(context.Background(), "dev1"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Send an event through the mock connection.
	eventCh <- Event{
		DeviceID: "dev1",
		Type:     EventButton,
		Source:   "BTN_SOUTH",
		Pressed:  true,
	}

	// Read from manager's event bus.
	select {
	case evt := <-mgr.Events():
		if evt.Source != "BTN_SOUTH" {
			t.Errorf("Source = %q, want BTN_SOUTH", evt.Source)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestManager_NoProviders(t *testing.T) {
	origRegistry := providerRegistry
	providerRegistry = nil
	defer func() { providerRegistry = origRegistry }()

	mgr := NewManager(ManagerConfig{})
	err := mgr.Start(context.Background())
	if err == nil {
		t.Error("expected error with no providers")
		mgr.Close()
	}
}
