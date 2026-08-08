//go:build official_sdk

package surfaceinventory

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// repoRootFromTestFile locates the mcpkit repo root relative to this test
// file (surfaceinventory/live_official_test.go -> repo root is one level up).
func repoRootFromTestFile(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Dir(filepath.Dir(thisFile))
}

// buildExampleServer compiles examples/minimal (a small two-tool mcp-go
// server: greet, word_count) into a temp binary and returns its path. The
// server itself is built under the default tag — an official_sdk client
// connects to it fine, since MCP is a wire protocol, not an SDK-specific
// handshake. Skips (never fails) the test on a build hiccup so a toolchain
// issue unrelated to this package doesn't block CI.
func buildExampleServer(t *testing.T) string {
	t.Helper()

	repoRoot := repoRootFromTestFile(t)
	if _, err := os.Stat(filepath.Join(repoRoot, "examples", "minimal", "main.go")); err != nil {
		t.Skipf("skipping live integration test: examples/minimal not found: %v", err)
	}

	bin := filepath.Join(t.TempDir(), "minimal-example")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, "./examples/minimal")
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("skipping live integration test: failed to build examples/minimal: %v\n%s", err, out)
	}
	return bin
}

// TestConnectAndListNames_Integration spawns mcpkit's own examples/minimal
// server and confirms connectAndListNames enumerates its two real tools
// (greet, word_count) via a genuine stdio client/server round trip.
func TestConnectAndListNames_Integration(t *testing.T) {
	bin := buildExampleServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	names, err := connectAndListNames(ctx, []string{bin}, KindMCPTool)
	if err != nil {
		t.Fatalf("connectAndListNames: %v", err)
	}

	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	for _, want := range []string{"greet", "word_count"} {
		if !got[want] {
			t.Errorf("live tool names = %v, missing %q", names, want)
		}
	}
	if len(names) != 2 {
		t.Errorf("len(names) = %d, want 2 (got %v)", len(names), names)
	}
}

// TestLiveDiff_Integration exercises the full live-vs-static comparison this
// package's surface_inventory_live tool performs: connect to a real running
// server, statically scan the same source directory, and diff the two name
// sets. examples/minimal declares both tools with an inline string literal
// description, so the static scanner should find the same two names the
// live server reports — both should land in Both, with LiveOnly/StaticOnly
// empty.
func TestLiveDiff_Integration(t *testing.T) {
	bin := buildExampleServer(t)
	repoRoot := repoRootFromTestFile(t)
	exampleDir := filepath.Join(repoRoot, "examples", "minimal")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	liveNames, err := connectAndListNames(ctx, []string{bin}, KindMCPTool)
	if err != nil {
		t.Fatalf("connectAndListNames: %v", err)
	}

	staticNames, err := staticNamesForKind(exampleDir, nil, KindMCPTool)
	if err != nil {
		t.Fatalf("staticNamesForKind: %v", err)
	}

	diff := diffNames(liveNames, staticNames)
	if len(diff.LiveOnly) != 0 {
		t.Errorf("LiveOnly = %v, want empty", diff.LiveOnly)
	}
	if len(diff.StaticOnly) != 0 {
		t.Errorf("StaticOnly = %v, want empty", diff.StaticOnly)
	}
	if diff.Counts.Both != 2 {
		t.Errorf("Counts.Both = %d, want 2 (Both=%v)", diff.Counts.Both, diff.Both)
	}
}
