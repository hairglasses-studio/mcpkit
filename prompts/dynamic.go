//go:build !official_sdk

package prompts

import "sync"

// ChangeNotifier is called when a prompt is added or removed.
type ChangeNotifier func()

// DynamicRegistry extends PromptRegistry with runtime prompt management.
type DynamicRegistry struct {
	*PromptRegistry
	notifierMu sync.RWMutex
	notifiers  []ChangeNotifier
}

// NewDynamicRegistry creates a registry that supports runtime prompt changes.
func NewDynamicRegistry(config ...Config) *DynamicRegistry {
	return &DynamicRegistry{
		PromptRegistry: NewPromptRegistry(config...),
	}
}

// OnChange registers a callback invoked when prompts are added or removed.
func (d *DynamicRegistry) OnChange(fn ChangeNotifier) {
	d.notifierMu.Lock()
	defer d.notifierMu.Unlock()
	d.notifiers = append(d.notifiers, fn)
}

func (d *DynamicRegistry) notify() {
	d.notifierMu.RLock()
	defer d.notifierMu.RUnlock()
	for _, fn := range d.notifiers {
		fn()
	}
}

// AddPrompt registers a single prompt at runtime and notifies listeners.
func (d *DynamicRegistry) AddPrompt(pd PromptDefinition) {
	d.mu.Lock()
	d.prompts[pd.Prompt.Name] = pd
	d.mu.Unlock()
	d.notify()
}

// RemovePrompt removes a prompt by name and notifies listeners.
// Returns true if the prompt existed.
func (d *DynamicRegistry) RemovePrompt(name string) bool {
	d.mu.Lock()
	_, ok := d.prompts[name]
	if ok {
		delete(d.prompts, name)
	}
	d.mu.Unlock()
	if ok {
		d.notify()
	}
	return ok
}
