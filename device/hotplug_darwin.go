//go:build darwin

package device

import (
	"context"
	"time"
)

// DarwinHotPlugWatcher monitors device changes by polling IOKit HID and
// CoreMIDI providers every 2 seconds. A future version could use
// IOServiceAddMatchingNotification for real-time IOKit events and
// MIDIClientCreate notifications for CoreMIDI.
type DarwinHotPlugWatcher struct {
	events    chan HotPlugEvent
	cancel    context.CancelFunc
	known     map[DeviceID]Info
	providers []DeviceProvider
}

// NewDarwinHotPlugWatcher creates a hot-plug watcher that polls both the
// IOKit HID provider (gamepads, generic HID) and the CoreMIDI provider.
func NewDarwinHotPlugWatcher() *DarwinHotPlugWatcher {
	return &DarwinHotPlugWatcher{
		events: make(chan HotPlugEvent, 16),
		known:  make(map[DeviceID]Info),
		providers: []DeviceProvider{
			&darwinIOKitProvider{},
			&darwinMIDIProvider{},
		},
	}
}

func (w *DarwinHotPlugWatcher) Start(ctx context.Context) error {
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

func (w *DarwinHotPlugWatcher) Events() <-chan HotPlugEvent { return w.events }

func (w *DarwinHotPlugWatcher) Close() error {
	if w.cancel != nil {
		w.cancel()
	}
	return nil
}
