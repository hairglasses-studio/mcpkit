package rdcycle

import (
	"context"
	"strings"
	"testing"
)

func TestHandleVerify_EchoOK(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})

	out, err := m.handleVerify(context.Background(), VerifyInput{
		Command: "echo ok",
	})
	if err != nil {
		t.Fatalf("handleVerify: unexpected error: %v", err)
	}
	if !out.Passed {
		t.Errorf("Passed: want true, got false; output: %s", out.Output)
	}
	if !strings.Contains(out.Output, "ok") {
		t.Errorf("Output: want 'ok', got %q", out.Output)
	}
	if out.Command != "echo ok" {
		t.Errorf("Command: want %q, got %q", "echo ok", out.Command)
	}
	if out.Duration == "" {
		t.Error("Duration: expected non-empty")
	}
	if out.ArtifactID == "" {
		t.Error("ArtifactID: expected non-empty")
	}
}

func TestHandleVerify_DefaultCommand(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})

	// Run a command that should fail (make check in a dir without a Makefile).
	out, err := m.handleVerify(context.Background(), VerifyInput{
		Command: "false", // always exits non-zero
	})
	if err != nil {
		t.Fatalf("handleVerify: unexpected error: %v", err)
	}
	if out.Passed {
		t.Error("Passed: want false for 'false' command")
	}
}

func TestHandleVerify_PackagesAppended(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})

	out, err := m.handleVerify(context.Background(), VerifyInput{
		Command:  "echo",
		Packages: []string{"./pkg/..."},
	})
	if err != nil {
		t.Fatalf("handleVerify: unexpected error: %v", err)
	}
	if !strings.Contains(out.Command, "./pkg/...") {
		t.Errorf("Command: want to contain './pkg/...', got %q", out.Command)
	}
	if !out.Passed {
		t.Errorf("Passed: want true for echo, got false; output: %s", out.Output)
	}
}

func TestHandleVerify_ArtifactStored(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})

	out, err := m.handleVerify(context.Background(), VerifyInput{Command: "echo artifact"})
	if err != nil {
		t.Fatalf("handleVerify: unexpected error: %v", err)
	}

	artifact, ok := m.store.Get(out.ArtifactID)
	if !ok {
		t.Fatal("artifact not stored")
	}
	if artifact.Type != "verify" {
		t.Errorf("artifact Type: want %q, got %q", "verify", artifact.Type)
	}
}

func TestHandleVerify_OutputCaptured(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})

	out, err := m.handleVerify(context.Background(), VerifyInput{
		Command: "echo hello-world",
	})
	if err != nil {
		t.Fatalf("handleVerify: unexpected error: %v", err)
	}
	if !strings.Contains(out.Output, "hello-world") {
		t.Errorf("Output: expected 'hello-world', got %q", out.Output)
	}
}
