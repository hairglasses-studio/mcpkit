package resources

import "sync"

// ChangeNotifier is called when a resource is added or removed.
type ChangeNotifier func()

// DynamicRegistry extends ResourceRegistry with runtime resource management.
type DynamicRegistry struct {
	*ResourceRegistry
	notifierMu sync.RWMutex
	notifiers  []ChangeNotifier
}

// NewDynamicRegistry creates a registry that supports runtime resource changes.
func NewDynamicRegistry(config ...Config) *DynamicRegistry {
	return &DynamicRegistry{
		ResourceRegistry: NewResourceRegistry(config...),
	}
}

// OnChange registers a callback invoked when resources are added or removed.
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

// AddResource registers a single resource at runtime and notifies listeners.
func (d *DynamicRegistry) AddResource(rd ResourceDefinition) {
	d.mu.Lock()
	d.resources[rd.Resource.URI] = rd
	d.mu.Unlock()
	d.notify()
}

// RemoveResource removes a resource by URI and notifies listeners.
// Returns true if the resource existed.
func (d *DynamicRegistry) RemoveResource(uri string) bool {
	d.mu.Lock()
	_, ok := d.resources[uri]
	if ok {
		delete(d.resources, uri)
	}
	d.mu.Unlock()
	if ok {
		d.notify()
	}
	return ok
}

// AddTemplate registers a single template at runtime and notifies listeners.
func (d *DynamicRegistry) AddTemplate(td TemplateDefinition) {
	d.mu.Lock()
	d.templates[td.Template.URITemplate.Raw()] = td
	d.mu.Unlock()
	d.notify()
}

// RemoveTemplate removes a template by URI pattern and notifies listeners.
// Returns true if the template existed.
func (d *DynamicRegistry) RemoveTemplate(uriTemplate string) bool {
	d.mu.Lock()
	_, ok := d.templates[uriTemplate]
	if ok {
		delete(d.templates, uriTemplate)
	}
	d.mu.Unlock()
	if ok {
		d.notify()
	}
	return ok
}
