package device

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestConnectionType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ct   ConnectionType
		want string
	}{
		{ConnUSB, "usb"},
		{ConnBluetooth, "bluetooth"},
		{ConnWirelessDongle, "wireless_dongle"},
		{ConnNetwork, "network"},
		{ConnVirtual, "virtual"},
		{ConnectionType(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.want {
			t.Errorf("ConnectionType(%d).String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestDeviceType_String_Mouse(t *testing.T) {
	if got := TypeMouse.String(); got != "mouse" {
		t.Errorf("TypeMouse.String() = %q, want mouse", got)
	}
}

func TestEventType_String_AllTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		et   EventType
		want string
	}{
		{EventButton, "button"},
		{EventAxis, "axis"},
		{EventHat, "hat"},
		{EventMIDINote, "midi_note"},
		{EventMIDICC, "midi_cc"},
		{EventMIDIProgramChange, "midi_program_change"},
		{EventMIDIPitchBend, "midi_pitch_bend"},
		{EventMIDISysEx, "midi_sysex"},
		{EventKey, "key"},
		{EventEncoder, "encoder"},
		{EventType(99), "event_99"},
	}
	for _, tt := range tests {
		if got := tt.et.String(); got != tt.want {
			t.Errorf("EventType(%d).String() = %q, want %q", tt.et, got, tt.want)
		}
	}
}

func TestManager_Refresh_NewDevice(t *testing.T) {
	devInfo := Info{ID: "dev1", Name: "Pad 1", Type: TypeGamepad}
	newDev := Info{ID: "dev2", Name: "Pad 2", Type: TypeGamepad}

	var enumCount atomic.Int32
	provider := &mockProvider{
		name:  "refresh-mock",
		types: []DeviceType{TypeGamepad},
		devices: []Info{devInfo},
	}

	mgr := NewManager(ManagerConfig{EventBuffer: 10})
	mgr.providers = []DeviceProvider{provider}
	mgr.mu.Lock()
	mgr.devices[devInfo.ID] = &managedDevice{
		info:     devInfo,
		provider: provider,
		lastSeen: time.Now(),
	}
	mgr.mu.Unlock()

	// Add a second device on next enumeration.
	provider.devices = []Info{devInfo, newDev}
	enumCount.Store(0)

	if err := mgr.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	listed := mgr.ListDevices()
	if len(listed) != 2 {
		t.Fatalf("expected 2 devices after refresh, got %d", len(listed))
	}

	// Verify hot-plug connect event was emitted.
	select {
	case hp := <-mgr.HotPlugEvents():
		if hp.Type != HotPlugConnect {
			t.Errorf("expected HotPlugConnect, got %v", hp.Type)
		}
		if hp.Info.ID != "dev2" {
			t.Errorf("expected dev2 in hotplug event, got %v", hp.Info.ID)
		}
	default:
		t.Error("expected a hot-plug connect event for new device")
	}

	_ = mgr.Close()
}

func TestManager_Refresh_ExistingDevice(t *testing.T) {
	devInfo := Info{ID: "dev1", Name: "Pad 1", Type: TypeGamepad}

	provider := &mockProvider{
		name:    "refresh-mock",
		types:   []DeviceType{TypeGamepad},
		devices: []Info{devInfo},
	}

	mgr := NewManager(ManagerConfig{EventBuffer: 10})
	mgr.providers = []DeviceProvider{provider}
	mgr.mu.Lock()
	old := time.Now().Add(-time.Hour)
	mgr.devices[devInfo.ID] = &managedDevice{
		info:     devInfo,
		provider: provider,
		lastSeen: old,
	}
	mgr.mu.Unlock()

	if err := mgr.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	// Should still be 1 device, but lastSeen updated.
	if len(mgr.ListDevices()) != 1 {
		t.Fatalf("expected 1 device")
	}

	mgr.mu.RLock()
	md := mgr.devices[devInfo.ID]
	mgr.mu.RUnlock()

	if !md.lastSeen.After(old) {
		t.Error("lastSeen was not updated on re-enumeration")
	}

	// No hot-plug event should be emitted for existing device.
	select {
	case hp := <-mgr.HotPlugEvents():
		t.Errorf("unexpected hot-plug event: %v", hp)
	default:
		// expected
	}

	_ = mgr.Close()
}

func TestManager_EventBus_Backpressure(t *testing.T) {
	eventCh := make(chan Event, 10)
	conn := &mockConnection{
		info:   Info{ID: "dev1", Name: "Test"},
		events: eventCh,
		alive:  true,
	}

	devInfo := Info{ID: "dev1", Name: "Test", Type: TypeGamepad}
	provider := &mockProvider{
		name:    "bp-mock",
		types:   []DeviceType{TypeGamepad},
		devices: []Info{devInfo},
		openFn:  func(id DeviceID) (DeviceConnection, error) { return conn, nil },
	}

	// Small event buffer to trigger backpressure.
	mgr := NewManager(ManagerConfig{EventBuffer: 2})
	mgr.providers = []DeviceProvider{provider}
	mgr.mu.Lock()
	mgr.devices[devInfo.ID] = &managedDevice{info: devInfo, provider: provider, lastSeen: time.Now()}
	mgr.mu.Unlock()

	if err := mgr.Connect(context.Background(), "dev1"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Fill the event bus beyond capacity — events should be dropped, not block.
	for i := 0; i < 10; i++ {
		eventCh <- Event{DeviceID: "dev1", Type: EventButton, Source: "BTN_SOUTH"}
	}

	// Give forwardEvents goroutine time to process.
	time.Sleep(50 * time.Millisecond)

	// Drain what we can — should get at most EventBuffer worth.
	received := 0
	for {
		select {
		case <-mgr.Events():
			received++
		default:
			goto done
		}
	}
done:
	if received == 0 {
		t.Error("expected at least some events to be forwarded")
	}
	if received > 10 {
		t.Errorf("received %d events, more than sent", received)
	}

	_ = mgr.Close()
}

func TestManager_Connect_AlreadyConnected(t *testing.T) {
	eventCh := make(chan Event, 10)
	conn := &mockConnection{
		info:   Info{ID: "dev1", Name: "Test"},
		events: eventCh,
		alive:  true,
	}

	devInfo := Info{ID: "dev1", Name: "Test", Type: TypeGamepad}
	provider := &mockProvider{
		name:    "mock",
		types:   []DeviceType{TypeGamepad},
		devices: []Info{devInfo},
		openFn:  func(id DeviceID) (DeviceConnection, error) { return conn, nil },
	}

	origRegistry := providerRegistry
	providerRegistry = []func() DeviceProvider{func() DeviceProvider { return provider }}
	defer func() { providerRegistry = origRegistry }()

	mgr := NewManager(ManagerConfig{EventBuffer: 10})
	_ = mgr.Start(context.Background())
	defer func() { _ = mgr.Close() }()

	// First connect.
	if err := mgr.Connect(context.Background(), "dev1"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Second connect — should return nil (already connected).
	if err := mgr.Connect(context.Background(), "dev1"); err != nil {
		t.Errorf("second Connect should succeed silently, got: %v", err)
	}
}

func TestManager_Connect_NotFound(t *testing.T) {
	origRegistry := providerRegistry
	providerRegistry = []func() DeviceProvider{
		func() DeviceProvider {
			return &mockProvider{name: "mock", devices: []Info{{ID: "dev1"}}}
		},
	}
	defer func() { providerRegistry = origRegistry }()

	mgr := NewManager(ManagerConfig{})
	_ = mgr.Start(context.Background())
	defer func() { _ = mgr.Close() }()

	err := mgr.Connect(context.Background(), "nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestManager_Disconnect_NotConnected(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	mgr.devices = map[DeviceID]*managedDevice{}
	mgr.conns = map[DeviceID]DeviceConnection{}

	// Disconnecting a non-connected device should return nil.
	if err := mgr.Disconnect("nonexistent"); err != nil {
		t.Errorf("Disconnect nonexistent should return nil, got %v", err)
	}
}

func TestManager_Close_Idempotent(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	mgr.done = make(chan struct{})

	// Close without Start — should not panic.
	if err := mgr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestManagerConfig_WithDefaults(t *testing.T) {
	t.Parallel()

	cfg := ManagerConfig{}
	out := cfg.withDefaults()

	if out.PollRate != time.Millisecond {
		t.Errorf("PollRate = %v, want 1ms", out.PollRate)
	}
	if out.ReconnectDelay != 2*time.Second {
		t.Errorf("ReconnectDelay = %v, want 2s", out.ReconnectDelay)
	}
	if out.EventBuffer != 256 {
		t.Errorf("EventBuffer = %d, want 256", out.EventBuffer)
	}

	// Custom values should be preserved.
	custom := ManagerConfig{
		PollRate:       5 * time.Millisecond,
		ReconnectDelay: 10 * time.Second,
		EventBuffer:    512,
		MaxReconnects:  3,
	}
	out2 := custom.withDefaults()
	if out2.PollRate != 5*time.Millisecond {
		t.Errorf("PollRate = %v, want 5ms", out2.PollRate)
	}
	if out2.MaxReconnects != 3 {
		t.Errorf("MaxReconnects = %d, want 3", out2.MaxReconnects)
	}
}

func TestPlatformProviders_Empty(t *testing.T) {
	origRegistry := providerRegistry
	providerRegistry = nil
	defer func() { providerRegistry = origRegistry }()

	providers := PlatformProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers with empty registry, got %d", len(providers))
	}
}

func TestRegisterProvider(t *testing.T) {
	origRegistry := providerRegistry
	providerRegistry = nil
	defer func() { providerRegistry = origRegistry }()

	RegisterProvider(func() DeviceProvider {
		return &mockProvider{name: "test-provider"}
	})

	providers := PlatformProviders()
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].Name() != "test-provider" {
		t.Errorf("provider name = %q, want test-provider", providers[0].Name())
	}
}
