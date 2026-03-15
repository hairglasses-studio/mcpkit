package rdcycle

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHandleCommit_EmptyFiles(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	_, err := m.handleCommit(context.Background(), CommitInput{
		Files:   nil,
		Message: "test",
	})
	if err == nil {
		t.Error("expected error for empty files")
	}
}

func TestHandleCommit_EmptyMessage(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	_, err := m.handleCommit(context.Background(), CommitInput{
		Files:   []string{"foo.go"},
		Message: "",
	})
	if err == nil {
		t.Error("expected error for empty message")
	}
}

func TestHandleCommit_PathTraversal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := NewModule(CycleConfig{GitRoot: dir})
	_, err := m.handleCommit(context.Background(), CommitInput{
		Files:   []string{"../../etc/passwd"},
		Message: "hack",
	})
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestHandleCommit_RefuseMain(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t, "main")
	m := NewModule(CycleConfig{GitRoot: dir})

	// Create a file to commit
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test"), 0644)

	_, err := m.handleCommit(context.Background(), CommitInput{
		Files:   []string{"test.txt"},
		Message: "should fail",
	})
	if err == nil {
		t.Error("expected error when committing to main")
	}
}

func TestHandleCommit_Success(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t, "feature-test")
	m := NewModule(CycleConfig{GitRoot: dir})

	// Create a file to commit
	testFile := filepath.Join(dir, "new.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	out, err := m.handleCommit(context.Background(), CommitInput{
		Files:   []string{"new.txt"},
		Message: "add new file",
	})
	if err != nil {
		t.Fatalf("handleCommit: %v", err)
	}
	if !out.Committed {
		t.Error("expected committed=true")
	}
	if out.Branch != "feature-test" {
		t.Errorf("branch = %q; want feature-test", out.Branch)
	}
	if out.SHA == "" || out.SHA == "unknown" {
		t.Error("expected a valid SHA")
	}
}

// setupTestRepo creates a temporary git repo on the given branch.
func setupTestRepo(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "checkout", "-b", branch},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}

	// Need an initial commit for git operations to work
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0644)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("add", "README.md")
	run("commit", "-m", "initial")

	return dir
}
