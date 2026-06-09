// Package toolgroups provides a reusable ToolGroupRegistry pattern for
// deferred, on-demand loading of MCP tool groups. It is extracted from the
// ralphglasses mcpserver package so that runmylife, hg-android, hg-pi, and
// other servers can adopt the same registration model without depending on
// ralphglasses internals.
//
// Typical usage:
//
//	reg := toolgroups.NewRegistry()
//	reg.Register(toolgroups.NewFuncBuilder("audio", func(ctx toolgroups.BuildContext) toolgroups.Group {
//	    return toolgroups.Group{
//	        Name:        "audio",
//	        Description: "Audio device management tools",
//	        LoadFn:      myServer.LoadAudioTools,
//	    }
//	}))
//	groups := reg.BuildAll(ctx)
package toolgroups

import "errors"

// ErrUnknownGroup is returned by [Registry.Build] when the requested group
// name has not been registered.
var ErrUnknownGroup = errors.New("toolgroups: unknown group")

// BuildContext carries per-server context into builders. Concrete servers
// embed or implement this interface to pass their own state (e.g. config,
// logger, handler factories) without the registry knowing about server
// internals.
type BuildContext interface {
	// ServerName returns a human-readable identifier for the server.
	ServerName() string
}

// Group is the output of a Builder.  It carries the group name, a short
// description, and an optional loader function that the MCP server calls when
// the group is first requested by a client.
type Group struct {
	// Name is the unique, machine-readable identifier (e.g. "audio", "gpu").
	Name string
	// Description is a short human-readable summary shown in tool listings.
	Description string
	// LoadFn is an optional callback the server invokes to register the group's
	// individual tools into the running server.  Servers that pre-register all
	// tools can leave this nil.
	LoadFn func() error
}

// Builder constructs a [Group] for a specific deferred-load group.
type Builder interface {
	// Name returns the unique group identifier produced by this builder.
	Name() string
	// Build constructs the Group using the supplied BuildContext.
	Build(ctx BuildContext) Group
}

// FuncBuilder is a convenience adapter that turns a plain function into a
// [Builder]. Use it for simple registrations where a full interface
// implementation would be boilerplate.
type FuncBuilder struct {
	name    string
	buildFn func(ctx BuildContext) Group
}

// Name returns the group identifier.
func (f *FuncBuilder) Name() string { return f.name }

// Build delegates to the wrapped function.
func (f *FuncBuilder) Build(ctx BuildContext) Group { return f.buildFn(ctx) }

// NewFuncBuilder creates a [FuncBuilder] from a name and build function.
func NewFuncBuilder(name string, fn func(ctx BuildContext) Group) *FuncBuilder {
	return &FuncBuilder{name: name, buildFn: fn}
}

// Registry collects builders and produces tool groups on demand.
// Builders are invoked in registration order so that the output of [Registry.BuildAll]
// is deterministic and matches the previous hard-coded ordering.
type Registry struct {
	builders []Builder
	index    map[string]Builder
}

// NewRegistry creates an empty [Registry].
func NewRegistry() *Registry {
	return &Registry{index: make(map[string]Builder)}
}

// Register appends a builder to the registry.  If a builder with the same
// Name() was already registered, Register replaces it (last write wins).
func (r *Registry) Register(b Builder) {
	if _, exists := r.index[b.Name()]; !exists {
		r.builders = append(r.builders, b)
	} else {
		// Replace in-slice while keeping position.
		for i, existing := range r.builders {
			if existing.Name() == b.Name() {
				r.builders[i] = b
				break
			}
		}
	}
	r.index[b.Name()] = b
}

// Build invokes the builder for a single named group.  Returns
// [ErrUnknownGroup] if the name has not been registered.
func (r *Registry) Build(ctx BuildContext, name string) (Group, error) {
	b, ok := r.index[name]
	if !ok {
		return Group{}, ErrUnknownGroup
	}
	return b.Build(ctx), nil
}

// BuildAll invokes every registered builder and returns the resulting groups
// keyed by group name.
func (r *Registry) BuildAll(ctx BuildContext) map[string]Group {
	m := make(map[string]Group, len(r.builders))
	for _, b := range r.builders {
		g := b.Build(ctx)
		m[g.Name] = g
	}
	return m
}

// BuildAllOrdered invokes every registered builder and returns the resulting
// groups as an ordered slice, preserving registration order.
func (r *Registry) BuildAllOrdered(ctx BuildContext) []Group {
	groups := make([]Group, 0, len(r.builders))
	for _, b := range r.builders {
		groups = append(groups, b.Build(ctx))
	}
	return groups
}

// Names returns the group names of all registered builders in order.
func (r *Registry) Names() []string {
	names := make([]string, len(r.builders))
	for i, b := range r.builders {
		names[i] = b.Name()
	}
	return names
}

// Len returns the number of registered builders.
func (r *Registry) Len() int {
	return len(r.builders)
}

// MustBuild is like [Registry.Build] but panics on error. Useful in init
// functions where a missing group is a programming error.
func (r *Registry) MustBuild(ctx BuildContext, name string) Group {
	g, err := r.Build(ctx, name)
	if err != nil {
		panic("toolgroups: MustBuild: " + err.Error() + ": " + name)
	}
	return g
}

// LoadOrder returns builders sorted by Name() — useful for tests that need
// deterministic output independent of registration order.
func (r *Registry) LoadOrder() []string {
	names := r.Names()
	sorted := make([]string, len(names))
	copy(sorted, names)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}
