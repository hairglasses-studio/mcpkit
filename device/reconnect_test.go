package device

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// reconnectConnection is a mock connection that can be closed externally
// to simulate a device disconnecting.
type reconnectConnection struct {
	info   Info
	events chan Event
	alive  int32 // atomic
}

func (c *reconnectConnection) Info() Info { return c.info }
func (c *reconnectConnection) Start(_ context.Context) error {
	atomic.StoreInt32(&c.alive, 1)
	return nil
}
func (c *reconnectConnection) Events() <-chan Event     { return c.events }
func (c *reconnectConnection) Feedback() DeviceFeedback { return nil }
func (c *reconnectConnection) Close() error {
	if atomic.CompareAndSwapInt32(&c.alive, 1, 0) {
		close(c.events)
	}
	return nil
}
func (c *reconnectConnection) Alive() bool { return atomic.LoadInt32(&c.alive) == 1 }

// newReconnectConnection creates a ready-to-use mock connection.
func newReconnectConnection(info Info) *reconnectConnection {
	return &reconnectConnection{
		info:   info,
		events: make(chan Event, 10),
	}
}

// setupReconnectManager creates a Manager wired to a single mock provider
// without touching the global providerRegistry. The caller receives the
// manager, a cancel func for the context used by Connect(), and the
// provider struct so tests can swap openFn.
func setupReconnectManager(t *testing.T, cfg ManagerConfig, devInfo Info, openFn func(DeviceID) (DeviceConnection, error)) (*Manager, context.Context, context.CancelFunc, *mockProvider) {
	t.Helper()

	provider := &mockProvider{
		name:    "rc-mock",
		types:   []DeviceType{TypeGamepad},
		devices: []Info{devInfo},
		openFn:  openFn,
	}

	mgr := NewManager(cfg)
	// Inject provider and device directly — avoids global registry.
	mgr.providers = []DeviceProvider{provider}
	mgr.mu.Lock()
	mgr.devices[devInfo.ID] = &managedDevice{
		info:     devInfo,
		provider: provider,
		lastSeen: time.Now(),
	}
	mgr.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	return mgr, ctx, cancel, provider
}

// TestManager_AutoReconnect verifies that when a device connection drops
// unexpectedly, the manager automatically reconnects after the configured
// delay. The mock provider fails Open() on the first reconnect attempt
// and succeeds on the second.
func TestManager_AutoReconnect(t *testing.T) {
	devID := DeviceID("rc-dev1")
	devInfo := Info{ID: devID, Name: "Reconnect Pad", Type: TypeGamepad}

	var openCalls atomic.Int32
	firstConn := newReconnectConnection(devInfo)

	openFn := func(id DeviceID) (DeviceConnection, error) {
		n := openCalls.Add(1)
		if n == 1 {
			return firstConn, nil
		}
		if n == 2 {
			// First reconnect attempt — fail.
			return nil, fmt.Errorf("device busy")
		}
		// Second reconnect attempt — succeed with a new connection.
		return newReconnectConnection(devInfo), nil
	}

	mgr, ctx, cancel, _ := setupReconnectManager(t, ManagerConfig{
		ReconnectDelay: 10 * time.Millisecond,
		MaxReconnects:  5,
		EventBuffer:    10,
	}, devInfo, openFn)
	defer func() {
		cancel()
		mgr.Close()
	}()

	if err := mgr.Connect(ctx, devID); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Simulate unexpected disconnect.
	firstConn.Close()

	// Wait for reconnect to succeed.
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for reconnect; openCalls=%d", openCalls.Load())
		default:
		}

		mgr.mu.RLock()
		md, ok := mgr.devices[devID]
		connected := ok && md.connected
		mgr.mu.RUnlock()

		if connected && openCalls.Load() >= 3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got := openCalls.Load(); got < 3 {
		t.Errorf("expected at least 3 Open calls, got %d", got)
	}
}

// TestManager_MaxReconnectsExceeded verifies that when MaxReconnects is set
// and the provider always fails, reconnection stops after the configured
// number of attempts.
func TestManager_MaxReconnectsExceeded(t *testing.T) {
	devID := DeviceID("rc-dev2")
	devInfo := Info{ID: devID, Name: "Flaky Pad", Type: TypeGamepad}

	var openCalls atomic.Int32
	firstConn := newReconnectConnection(devInfo)

	openFn := func(id DeviceID) (DeviceConnection, error) {
		n := openCalls.Add(1)
		if n == 1 {
			return firstConn, nil
		}
		return nil, fmt.Errorf("device gone")
	}

	maxReconnects := 2
	mgr, ctx, cancel, _ := setupReconnectManager(t, ManagerConfig{
		ReconnectDelay: 10 * time.Millisecond,
		MaxReconnects:  maxReconnects,
		EventBuffer:    10,
	}, devInfo, openFn)
	defer func() {
		cancel()
		mgr.Close()
	}()

	if err := mgr.Connect(ctx, devID); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Simulate unexpected disconnect.
	firstConn.Close()

	// Wait for all reconnect attempts to finish.
	time.Sleep(200 * time.Millisecond)

	// 1 initial + 2 reconnect attempts = 3.
	got := openCalls.Load()
	expected := int32(1 + maxReconnects)
	if got != expected {
		t.Errorf("expected %d Open calls (1 initial + %d reconnects), got %d", expected, maxReconnects, got)
	}

	// Device should remain disconnected.
	mgr.mu.RLock()
	md, ok := mgr.devices[devID]
	connected := ok && md.connected
	mgr.mu.RUnlock()

	if connected {
		t.Error("device should not be connected after exhausting reconnect attempts")
	}
}

// TestManager_ReconnectCancelled verifies that cancelling the context stops
// the reconnect loop promptly.
func TestManager_ReconnectCancelled(t *testing.T) {
	devID := DeviceID("rc-dev3")
	devInfo := Info{ID: devID, Name: "Cancel Pad", Type: TypeGamepad}

	var openCalls atomic.Int32
	firstConn := newReconnectConnection(devInfo)

	openFn := func(id DeviceID) (DeviceConnection, error) {
		n := openCalls.Add(1)
		if n == 1 {
			return firstConn, nil
		}
		return nil, fmt.Errorf("device gone")
	}

	mgr, ctx, cancel, _ := setupReconnectManager(t, ManagerConfig{
		ReconnectDelay: 50 * time.Millisecond,
		MaxReconnects:  0, // Unlimited — would loop forever without cancel.
		EventBuffer:    10,
	}, devInfo, openFn)
	defer mgr.Close()

	if err := mgr.Connect(ctx, devID); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Simulate disconnect.
	firstConn.Close()

	// Let a couple of reconnect attempts happen, then cancel.
	time.Sleep(130 * time.Millisecond)
	cancel()

	// Record the open count shortly after cancel.
	time.Sleep(30 * time.Millisecond)
	countAfterCancel := openCalls.Load()

	// Wait to confirm no further attempts happen.
	time.Sleep(200 * time.Millisecond)
	countLater := openCalls.Load()

	// Allow at most 1 extra call (one may have been in-flight when we cancelled).
	if countLater > countAfterCancel+1 {
		t.Errorf("reconnect continued after context cancel: calls shortly after=%d, calls later=%d",
			countAfterCancel, countLater)
	}
}

// TestManager_DisconnectPreventsReconnect verifies that calling Disconnect()
// explicitly prevents the auto-reconnect logic from firing.
func TestManager_DisconnectPreventsReconnect(t *testing.T) {
	devID := DeviceID("rc-dev4")
	devInfo := Info{ID: devID, Name: "Manual DC Pad", Type: TypeGamepad}

	var openCalls atomic.Int32
	firstConn := newReconnectConnection(devInfo)

	openFn := func(id DeviceID) (DeviceConnection, error) {
		openCalls.Add(1)
		return firstConn, nil
	}

	mgr, ctx, cancel, _ := setupReconnectManager(t, ManagerConfig{
		ReconnectDelay: 10 * time.Millisecond,
		MaxReconnects:  5,
		EventBuffer:    10,
	}, devInfo, openFn)
	defer func() {
		cancel()
		mgr.Close()
	}()

	if err := mgr.Connect(ctx, devID); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Explicitly disconnect (user-initiated).
	if err := mgr.Disconnect(devID); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	// Wait to ensure no reconnect attempts happen.
	time.Sleep(100 * time.Millisecond)

	if got := openCalls.Load(); got != 1 {
		t.Errorf("expected 1 Open call (initial only, no reconnect), got %d", got)
	}
}
