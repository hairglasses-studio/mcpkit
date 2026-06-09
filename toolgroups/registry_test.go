package toolgroups_test

import (
	"errors"
	"testing"

	"github.com/hairglasses-studio/mcpkit/toolgroups"
)

// testCtx is a minimal BuildContext for tests.
type testCtx struct{ name string }

func (t *testCtx) ServerName() string { return t.name }

var ctx = &testCtx{name: "test-server"}

// makeBuilder returns a simple FuncBuilder that produces a named Group with an
// optional LoadFn.
func makeBuilder(name string, loadFn func() error) toolgroups.Builder {
	return toolgroups.NewFuncBuilder(name, func(_ toolgroups.BuildContext) toolgroups.Group {
		return toolgroups.Group{
			Name:        name,
			Description: "test group: " + name,
			LoadFn:      loadFn,
		}
	})
}

// TestRegisterAndBuildAll verifies that registered builders are returned by BuildAll.
func TestRegisterAndBuildAll(t *testing.T) {
	reg := toolgroups.NewRegistry()
	reg.Register(makeBuilder("audio", nil))
	reg.Register(makeBuilder("gpu", nil))
	reg.Register(makeBuilder("network", nil))

	groups := reg.BuildAll(ctx)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	for _, name := range []string{"audio", "gpu", "network"} {
		g, ok := groups[name]
		if !ok {
			t.Errorf("missing group %q", name)
			continue
		}
		if g.Name != name {
			t.Errorf("group %q has Name=%q", name, g.Name)
		}
	}
}

// TestBuildSingleGroup verifies that Registry.Build returns the correct group.
func TestBuildSingleGroup(t *testing.T) {
	reg := toolgroups.NewRegistry()
	reg.Register(makeBuilder("audio", nil))
	reg.Register(makeBuilder("gpu", nil))

	g, err := reg.Build(ctx, "audio")
	if err != nil {
		t.Fatalf("Build(audio) error: %v", err)
	}
	if g.Name != "audio" {
		t.Errorf("expected Name=audio, got %q", g.Name)
	}
}

// TestLoadGroupIdempotent verifies that calling LoadFn twice is safe (caller
// is responsible for idempotency — the registry does not block double-calls,
// but LoadFn itself must be written idempotently).
func TestLoadGroupIdempotent(t *testing.T) {
	calls := 0
	loadFn := func() error {
		calls++
		return nil
	}

	reg := toolgroups.NewRegistry()
	reg.Register(makeBuilder("audio", loadFn))

	g, _ := reg.Build(ctx, "audio")
	if err := g.LoadFn(); err != nil {
		t.Fatalf("first LoadFn call: %v", err)
	}
	if err := g.LoadFn(); err != nil {
		t.Fatalf("second LoadFn call: %v", err)
	}
	// Both calls should succeed; the registry contract says callers own idempotency.
	if calls != 2 {
		t.Errorf("expected LoadFn called 2 times, got %d", calls)
	}
}

// TestUnknownGroupReturnsError verifies that Build returns ErrUnknownGroup for
// an unregistered name.
func TestUnknownGroupReturnsError(t *testing.T) {
	reg := toolgroups.NewRegistry()
	reg.Register(makeBuilder("audio", nil))

	_, err := reg.Build(ctx, "does-not-exist")
	if !errors.Is(err, toolgroups.ErrUnknownGroup) {
		t.Errorf("expected ErrUnknownGroup, got %v", err)
	}
}

// TestRegisterReplaceExisting verifies that re-registering a name replaces the
// old builder (last write wins), keeping registration order stable.
func TestRegisterReplaceExisting(t *testing.T) {
	reg := toolgroups.NewRegistry()
	reg.Register(makeBuilder("audio", nil))
	reg.Register(makeBuilder("gpu", nil))

	// Replace audio with a new builder that has a different description.
	reg.Register(toolgroups.NewFuncBuilder("audio", func(_ toolgroups.BuildContext) toolgroups.Group {
		return toolgroups.Group{Name: "audio", Description: "replaced"}
	}))

	if reg.Len() != 2 {
		t.Errorf("expected Len=2 after replace, got %d", reg.Len())
	}

	g, err := reg.Build(ctx, "audio")
	if err != nil {
		t.Fatalf("Build after replace: %v", err)
	}
	if g.Description != "replaced" {
		t.Errorf("expected replaced description, got %q", g.Description)
	}
}

// TestBuildAllOrdered verifies that BuildAllOrdered preserves registration order.
func TestBuildAllOrdered(t *testing.T) {
	reg := toolgroups.NewRegistry()
	reg.Register(makeBuilder("z-group", nil))
	reg.Register(makeBuilder("a-group", nil))
	reg.Register(makeBuilder("m-group", nil))

	ordered := reg.BuildAllOrdered(ctx)
	want := []string{"z-group", "a-group", "m-group"}
	if len(ordered) != len(want) {
		t.Fatalf("expected %d groups, got %d", len(want), len(ordered))
	}
	for i, g := range ordered {
		if g.Name != want[i] {
			t.Errorf("position %d: expected %q, got %q", i, want[i], g.Name)
		}
	}
}

// TestNames verifies that Names() returns group names in registration order.
func TestNames(t *testing.T) {
	reg := toolgroups.NewRegistry()
	reg.Register(makeBuilder("alpha", nil))
	reg.Register(makeBuilder("beta", nil))

	names := reg.Names()
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("unexpected Names: %v", names)
	}
}

// TestMustBuildPanicsOnUnknown verifies that MustBuild panics for an unknown group.
func TestMustBuildPanicsOnUnknown(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustBuild to panic for unknown group")
		}
	}()
	reg := toolgroups.NewRegistry()
	reg.MustBuild(ctx, "nope")
}

// TestEmptyRegistry verifies that an empty registry behaves sensibly.
func TestEmptyRegistry(t *testing.T) {
	reg := toolgroups.NewRegistry()
	if reg.Len() != 0 {
		t.Errorf("expected Len=0, got %d", reg.Len())
	}
	if len(reg.Names()) != 0 {
		t.Errorf("expected empty Names, got %v", reg.Names())
	}
	groups := reg.BuildAll(ctx)
	if len(groups) != 0 {
		t.Errorf("expected empty BuildAll, got %v", groups)
	}
}
