//go:build windows

package device

import (
	"context"
	"time"
)

// WindowsHotPlugWatcher monitors device changes by polling XInput and WinMM
// MIDI providers every 2 seconds. A future version could use
// RegisterDeviceNotification for real-time USB arrival/removal events.
type WindowsHotPlugWatcher struct {
	events    chan HotPlugEvent
	cancel    context.CancelFunc
	known     map[DeviceID]Info
	providers []DeviceProvider
}

// NewWindowsHotPlugWatcher creates a hot-plug watcher that polls both the
// XInput provider (Xbox gamepads) and the WinMM MIDI provider.
func NewWindowsHotPlugWatcher() *WindowsHotPlugWatcher {
	return &WindowsHotPlugWatcher{
		events: make(chan HotPlugEvent, 16),
		known:  make(map[DeviceID]Info),
		providers: []DeviceProvider{
			&windowsXInputProvider{},
			&windowsMIDIProvider{},
		},
	}
}

func (w *WindowsHotPlugWatcher) Start(ctx context.Context) error {
	ctx, w.cancel = context.WithCancel(ctx)

	// Initial snapshot from all providers.
	for _, p := range w.providers {
		devices, _ := p.Enumerate(ctx)
		for _, d := range devices {
			w.known[d.ID] = d
		}
	}

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		defer close(w.events)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentIDs := make(map[DeviceID]Info)

				for _, p := range w.providers {
					devices, _ := p.Enumerate(ctx)
					for _, d := range devices {
						currentIDs[d.ID] = d
					}
				}

				// Detect new devices.
				for id, info := range currentIDs {
					if _, existed := w.known[id]; !existed {
						w.known[id] = info
						select {
						case w.events <- HotPlugEvent{
							Type:      HotPlugConnect,
							Info:      info,
							Timestamp: time.Now(),
						}:
						default:
						}
					}
				}

				// Detect removed devices.
				for id, info := range w.known {
					if _, exists := currentIDs[id]; !exists {
						delete(w.known, id)
						select {
						case w.events <- HotPlugEvent{
							Type:      HotPlugDisconnect,
							Info:      info,
							Timestamp: time.Now(),
						}:
						default:
						}
					}
				}
			}
		}
	}()

	return nil
}

func (w *WindowsHotPlugWatcher) Events() <-chan HotPlugEvent { return w.events }

func (w *WindowsHotPlugWatcher) Close() error {
	if w.cancel != nil {
		w.cancel()
	}
	return nil
}
