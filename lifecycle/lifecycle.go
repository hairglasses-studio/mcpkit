// Package lifecycle provides production-ready server lifecycle management
// with signal handling, graceful drain, and shutdown hook ordering.
package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ShutdownFunc is called during graceful shutdown with a deadline context.
type ShutdownFunc func(ctx context.Context) error

// Config configures the lifecycle manager.
type Config struct {
	// DrainTimeout is how long to wait for shutdown hooks. Default: 30s.
	DrainTimeout time.Duration

	// Signals to listen for. Default: SIGTERM, SIGINT.
	Signals []os.Signal

	// OnHealthy is called when the server becomes healthy.
	OnHealthy func()

	// OnDraining is called when the server begins draining.
	OnDraining func()
}

func (c *Config) applyDefaults() {
	if c.DrainTimeout <= 0 {
		c.DrainTimeout = 30 * time.Second
	}
	if len(c.Signals) == 0 {
		c.Signals = []os.Signal{syscall.SIGTERM, syscall.SIGINT}
	}
}

// Manager manages the lifecycle of a production server.
type Manager struct {
	config Config
	mu     sync.Mutex
	hooks  []ShutdownFunc
	status string
}

// New creates a lifecycle manager with the given config.
func New(config ...Config) *Manager {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg.applyDefaults()
	return &Manager{
		config: cfg,
		status: "created",
	}
}

// OnShutdown registers a function to be called during graceful shutdown.
// Functions are called in LIFO order (last registered, first called).
func (m *Manager) OnShutdown(fn ShutdownFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, fn)
}

// Status returns the current lifecycle status.
func (m *Manager) Status() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) setStatus(s string) {
	m.mu.Lock()
	m.status = s
	m.mu.Unlock()
}

// Run blocks until a signal is received or the serve function returns,
// then performs graceful shutdown. It transitions through:
// "starting" -> "healthy" -> "draining" -> "stopped".
//
// The serve function should block while the server is running (e.g., serve
// HTTP). When Run receives a signal, it cancels the context passed to serve
// and runs shutdown hooks.
func (m *Manager) Run(ctx context.Context, serve func(ctx context.Context) error) error {
	m.setStatus("starting")

	// Set up signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, m.config.Signals...)
	defer signal.Stop(sigCh)

	// Create a cancellable context for the serve function.
	serveCtx, serveCancel := context.WithCancel(ctx)
	defer serveCancel()

	// Transition to healthy.
	m.setStatus("healthy")
	if m.config.OnHealthy != nil {
		m.config.OnHealthy()
	}

	// Run serve in a goroutine.
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- serve(serveCtx)
	}()

	// Wait for signal or serve to return.
	var err error
	select {
	case sig := <-sigCh:
		_ = sig
		serveCancel()
	case err = <-serveErr:
		// Serve returned — proceed to drain.
	case <-ctx.Done():
		serveCancel()
		err = ctx.Err()
	}

	// Drain phase.
	m.setStatus("draining")
	if m.config.OnDraining != nil {
		m.config.OnDraining()
	}

	// Run shutdown hooks in LIFO order with drain timeout.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), m.config.DrainTimeout)
	defer drainCancel()

	m.mu.Lock()
	hooks := make([]ShutdownFunc, len(m.hooks))
	copy(hooks, m.hooks)
	m.mu.Unlock()

	for i := len(hooks) - 1; i >= 0; i-- {
		if hookErr := hooks[i](drainCtx); hookErr != nil && err == nil {
			err = hookErr
		}
	}

	m.setStatus("stopped")
	return err
}
