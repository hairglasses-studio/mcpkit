package rdcycle

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

// TestGitDiffQuiet_CleanRepo creates a temporary git repo with no uncommitted
// changes and confirms gitDiffQuiet returns true (no changes).
func TestGitDiffQuiet_CleanRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Init repo, commit a file so diff has a baseline.
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "config", "user.email", "test@test")
	run("git", "config", "user.name", "test")
	f := filepath.Join(dir, "README")
	if err := os.WriteFile(f, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "README")
	run("git", "commit", "-m", "init")

	// Clean repo — no uncommitted changes.
	if !gitDiffQuiet(context.Background(), dir) {
		t.Error("gitDiffQuiet: want true for clean repo, got false")
	}
}

// TestGitDiffQuiet_DirtyRepo creates a temporary git repo, modifies a tracked
// file without committing, and confirms gitDiffQuiet returns false (has changes).
func TestGitDiffQuiet_DirtyRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "config", "user.email", "test@test")
	run("git", "config", "user.name", "test")
	f := filepath.Join(dir, "README")
	if err := os.WriteFile(f, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "README")
	run("git", "commit", "-m", "init")

	// Modify the file without committing.
	if err := os.WriteFile(f, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Dirty repo — has uncommitted changes.
	if gitDiffQuiet(context.Background(), dir) {
		t.Error("gitDiffQuiet: want false for dirty repo, got true")
	}
}

// TestHandleVerify_NoChangesField confirms that the no_changes field is false
// when the command fails (passed=false), regardless of git state.
func TestHandleVerify_NoChangesField_FailedCommand(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})

	out, err := m.handleVerify(context.Background(), VerifyInput{
		Command: "false", // always exits non-zero
	})
	if err != nil {
		t.Fatalf("handleVerify: unexpected error: %v", err)
	}
	if out.Passed {
		t.Error("Passed: want false for 'false' command")
	}
	if out.NoChanges {
		t.Error("NoChanges: want false when command failed")
	}
}
