package device

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ManagerConfig configures the Manager.
type ManagerConfig struct {
	PollRate       time.Duration // Event poll rate (default 1ms)
	ReconnectDelay time.Duration // Delay before reconnect attempt (default 2s)
	MaxReconnects  int           // Max reconnect attempts per device (0=unlimited)
	EventBuffer    int           // Channel buffer size for events (default 256)
}

func (c *ManagerConfig) withDefaults() ManagerConfig {
	out := *c
	if out.PollRate == 0 {
		out.PollRate = time.Millisecond
	}
	if out.ReconnectDelay == 0 {
		out.ReconnectDelay = 2 * time.Second
	}
	if out.EventBuffer == 0 {
		out.EventBuffer = 256
	}
	return out
}

// Manager orchestrates device discovery, connection, and event distribution.
type Manager struct {
	config    ManagerConfig
	mu        sync.RWMutex
	providers []DeviceProvider
	devices   map[DeviceID]*managedDevice
	conns     map[DeviceID]DeviceConnection
	eventBus  chan Event
	hotplug   chan HotPlugEvent
	cancel    context.CancelFunc
	done      chan struct{}
}

type managedDevice struct {
	info       Info
	provider   DeviceProvider
	connected  bool
	lastSeen   time.Time
	reconnects int
}

// NewManager creates a new device manager with the given config.
// Call Start() to begin device discovery and event processing.
func NewManager(config ManagerConfig) *Manager {
	cfg := config.withDefaults()
	return &Manager{
		config:   cfg,
		devices:  make(map[DeviceID]*managedDevice),
		conns:    make(map[DeviceID]DeviceConnection),
		eventBus: make(chan Event, cfg.EventBuffer),
		hotplug:  make(chan HotPlugEvent, 32),
		done:     make(chan struct{}),
	}
}

// Start initializes all registered providers, runs initial enumeration,
// and begins event processing. Call Close() to stop.
func (m *Manager) Start(ctx context.Context) error {
	ctx, m.cancel = context.WithCancel(ctx)

	m.providers = PlatformProviders()
	if len(m.providers) == 0 {
		return fmt.Errorf("no device providers available for this platform")
	}

	// Initial enumeration from all providers.
	for _, p := range m.providers {
		devices, err := p.Enumerate(ctx)
		if err != nil {
			continue // Provider may not be available
		}
		m.mu.Lock()
		for _, d := range devices {
			m.devices[d.ID] = &managedDevice{
				info:     d,
				provider: p,
				lastSeen: time.Now(),
			}
		}
		m.mu.Unlock()
	}

	return nil
}

// Close stops the manager and releases all resources.
func (m *Manager) Close() error {
	if m.cancel != nil {
		m.cancel()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all active connections.
	for id, conn := range m.conns {
		conn.Close()
		delete(m.conns, id)
	}

	// Close all providers.
	for _, p := range m.providers {
		p.Close()
	}

	close(m.done)
	return nil
}

// ListDevices returns all known devices.
func (m *Manager) ListDevices() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]Info, 0, len(m.devices))
	for _, md := range m.devices {
		devices = append(devices, md.info)
	}
	return devices
}

// GetDevice returns info for a specific device.
func (m *Manager) GetDevice(id DeviceID) (Info, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	md, ok := m.devices[id]
	if !ok {
		return Info{}, ErrDeviceNotFound
	}
	return md.info, nil
}

// Connect opens a connection to a device and starts reading events.
func (m *Manager) Connect(ctx context.Context, id DeviceID) error {
	m.mu.Lock()
	md, ok := m.devices[id]
	if !ok {
		m.mu.Unlock()
		return ErrDeviceNotFound
	}

	if _, connected := m.conns[id]; connected {
		m.mu.Unlock()
		return nil // Already connected
	}
	m.mu.Unlock()

	conn, err := md.provider.Open(ctx, id)
	if err != nil {
		return fmt.Errorf("open device %s: %w", id, err)
	}

	if err := conn.Start(ctx); err != nil {
		conn.Close()
		return fmt.Errorf("start device %s: %w", id, err)
	}

	m.mu.Lock()
	m.conns[id] = conn
	md.connected = true
	m.mu.Unlock()

	// Forward events from this connection to the event bus.
	go m.forwardEvents(id, conn)

	return nil
}

// Disconnect closes a connection to a device.
func (m *Manager) Disconnect(id DeviceID) error {
	m.mu.Lock()
	conn, ok := m.conns[id]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	delete(m.conns, id)
	if md, ok := m.devices[id]; ok {
		md.connected = false
	}
	m.mu.Unlock()

	return conn.Close()
}

// Events returns the unified event channel for all connected devices.
func (m *Manager) Events() <-chan Event {
	return m.eventBus
}

// HotPlugEvents returns the hot-plug event channel.
func (m *Manager) HotPlugEvents() <-chan HotPlugEvent {
	return m.hotplug
}

// Refresh re-enumerates all providers and updates the device list.
func (m *Manager) Refresh(ctx context.Context) error {
	for _, p := range m.providers {
		devices, err := p.Enumerate(ctx)
		if err != nil {
			continue
		}
		m.mu.Lock()
		for _, d := range devices {
			if _, exists := m.devices[d.ID]; !exists {
				m.devices[d.ID] = &managedDevice{
					info:     d,
					provider: p,
					lastSeen: time.Now(),
				}
				// Emit hot-plug connect event.
				select {
				case m.hotplug <- HotPlugEvent{
					Type:      HotPlugConnect,
					Info:      d,
					Timestamp: time.Now(),
				}:
				default:
				}
			} else {
				m.devices[d.ID].lastSeen = time.Now()
			}
		}
		m.mu.Unlock()
	}
	return nil
}

func (m *Manager) forwardEvents(id DeviceID, conn DeviceConnection) {
	for event := range conn.Events() {
		select {
		case m.eventBus <- event:
		default:
			// Drop event if bus is full (back-pressure)
		}
	}

	// Connection closed — mark as disconnected.
	m.mu.Lock()
	delete(m.conns, id)
	if md, ok := m.devices[id]; ok {
		md.connected = false
	}
	m.mu.Unlock()

	// Emit hot-plug disconnect.
	m.mu.RLock()
	md, ok := m.devices[id]
	m.mu.RUnlock()
	if ok {
		select {
		case m.hotplug <- HotPlugEvent{
			Type:      HotPlugDisconnect,
			Info:      md.info,
			Timestamp: time.Now(),
		}:
		default:
		}
	}
}
