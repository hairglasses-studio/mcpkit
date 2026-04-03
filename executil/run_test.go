package executil

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRun_Success(t *testing.T) {
	err := Run(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRun_Failure(t *testing.T) {
	err := Run(context.Background(), "nonexistent-command-xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestOutput_Success(t *testing.T) {
	out, err := Output(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out != "hello" {
		t.Fatalf("expected %q, got %q", "hello", out)
	}
}

func TestOutput_Stderr(t *testing.T) {
	_, err := Output(context.Background(), "sh", "-c", "echo oops >&2; exit 1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "oops") {
		t.Fatalf("expected error to contain stderr output, got %v", err)
	}
}

func TestCombinedOutput(t *testing.T) {
	out, err := CombinedOutput(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out != "hello" {
		t.Fatalf("expected %q, got %q", "hello", out)
	}
}

func TestSeparateOutput(t *testing.T) {
	stdout, stderr, err := SeparateOutput(context.Background(), "sh", "-c", "echo out; echo err >&2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(stdout, "out") {
		t.Fatalf("expected stdout to contain %q, got %q", "out", stdout)
	}
	if !strings.Contains(stderr, "err") {
		t.Fatalf("expected stderr to contain %q, got %q", "err", stderr)
	}
}

func TestOutputTimeout_Success(t *testing.T) {
	out, err := OutputTimeout(5*time.Second, "echo", "fast")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out != "fast" {
		t.Fatalf("expected %q, got %q", "fast", out)
	}
}

func TestOutputTimeout_Expired(t *testing.T) {
	_, err := OutputTimeout(50*time.Millisecond, "sleep", "10")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "signal: killed") {
		t.Fatalf("expected context deadline or kill error, got %v", err)
	}
}

func TestRunDir(t *testing.T) {
	err := RunDir(context.Background(), "/tmp", "pwd")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestOutputDir(t *testing.T) {
	out, err := OutputDir(context.Background(), "/tmp", "pwd")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// /tmp may resolve to a symlink target on some systems
	if !strings.Contains(out, "tmp") {
		t.Fatalf("expected output to contain 'tmp', got %q", out)
	}
}
