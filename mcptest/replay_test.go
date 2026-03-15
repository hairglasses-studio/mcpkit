//go:build !official_sdk

package mcptest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestRecorder_Session(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})

	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "hello"})
	c.CallTool("test_echo", map[string]interface{}{"message": "world"})

	session := rec.Session("my-session")
	if session.Name != "my-session" {
		t.Errorf("session name = %q, want %q", session.Name, "my-session")
	}
	if len(session.Entries) != 2 {
		t.Fatalf("session entries = %d, want 2", len(session.Entries))
	}
	if session.Entries[0].ToolName != "test_echo" {
		t.Errorf("entry[0].ToolName = %q, want %q", session.Entries[0].ToolName, "test_echo")
	}
}

func TestRecorder_SaveLoadSession_RoundTrip(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})

	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "round-trip"})

	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	if err := rec.SaveSession(path); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file not created: %v", err)
	}

	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}

	if len(loaded.Entries) != 1 {
		t.Fatalf("loaded entries = %d, want 1", len(loaded.Entries))
	}
	if loaded.Entries[0].ToolName != "test_echo" {
		t.Errorf("entry[0].ToolName = %q, want %q", loaded.Entries[0].ToolName, "test_echo")
	}
}

func TestLoadSession_NotFound(t *testing.T) {
	_, err := LoadSession("/nonexistent/path/session.json")
	if err == nil {
		t.Fatal("expected error loading non-existent file")
	}
}

func TestReplay_Match(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})

	s := NewServer(t, reg)
	c := NewClient(t, s)

	// Record a session
	c.CallTool("test_echo", map[string]interface{}{"message": "replay-me"})
	session := rec.Session("replay-test")

	// Replay against the same client — result should match
	Replay(t, c, session)
}

func TestReplay_MismatchDetected(t *testing.T) {
	// Build a session with a known result
	reg1 := registry.NewToolRegistry()
	reg1.RegisterModule(&testModule{})
	s1 := NewServer(t, reg1)
	c1 := NewClient(t, s1)

	rec := NewRecorder()
	reg2 := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg2.RegisterModule(&testModule{})
	s2 := NewServer(t, reg2)
	c2 := NewClient(t, s2)

	// Build the session using c1
	c1.CallTool("test_echo", map[string]interface{}{"message": "original"})

	// Manually construct a session with a different expected result
	session := &Session{
		Name: "mismatch-test",
		Entries: []SessionEntry{
			{
				ToolName: "test_echo",
				Args:     map[string]interface{}{"message": "original"},
				Result:   registry.MakeTextResult("DIFFERENT OUTPUT"),
				IsError:  false,
			},
		},
	}

	// Use a sub-test that we expect to fail
	failed := false
	mockT := &mockTB{TB: t, onError: func() { failed = true }}
	Replay(mockT, c2, session)
	if !failed {
		t.Error("Replay should have reported a mismatch but did not")
	}
}

func TestReplay_WithStrictOrder(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})

	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "first"})
	c.CallTool("test_echo", map[string]interface{}{"message": "second"})
	session := rec.Session("strict-test")

	// Should pass with strict order since we replay in same order
	Replay(t, c, session, WithStrictOrder())
}

func TestReplay_WithIgnoreFields(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})

	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "ignore-test"})
	session := rec.Session("ignore-test")

	// Should still pass ignoring a non-existent field
	Replay(t, c, session, WithIgnoreFields("nonexistent_field"))
}

// mockTB is a minimal testing.TB that captures failure signals.
type mockTB struct {
	testing.TB
	onError func()
}

func (m *mockTB) Helper()                   {}
func (m *mockTB) Errorf(format string, args ...interface{}) {
	m.onError()
}
func (m *mockTB) Fatalf(format string, args ...interface{}) {
	m.onError()
}
