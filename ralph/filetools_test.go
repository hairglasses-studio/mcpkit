//go:build !official_sdk

package ralph

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileToolModule_NameDescription(t *testing.T) {
	t.Parallel()
	m := &FileToolModule{Root: "/tmp"}
	if m.Name() != "file_tools" {
		t.Errorf("Name() = %q, want %q", m.Name(), "file_tools")
	}
	if m.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestFileToolModule_Tools(t *testing.T) {
	t.Parallel()
	m := &FileToolModule{Root: "/tmp"}
	tools := m.Tools()
	if len(tools) != 5 {
		t.Fatalf("Tools() len = %d, want 5", len(tools))
	}
	names := make(map[string]bool)
	for _, td := range tools {
		names[td.Tool.Name] = true
	}
	for _, want := range []string{"write_file", "read_file", "list_dir", "test_package", "check_coverage"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestHandleWriteFile_Success(t *testing.T) {
	root := t.TempDir()
	m := &FileToolModule{Root: root}

	out, err := m.handleWriteFile(context.Background(), WriteFileInput{
		Path:    "subdir/hello.go",
		Content: "package main\n",
	})
	if err != nil {
		t.Fatalf("handleWriteFile: %v", err)
	}
	if !out.Written {
		t.Error("expected Written=true")
	}
	if out.Path != "subdir/hello.go" {
		t.Errorf("Path = %q, want %q", out.Path, "subdir/hello.go")
	}
	if out.Bytes != len("package main\n") {
		t.Errorf("Bytes = %d, want %d", out.Bytes, len("package main\n"))
	}

	// Verify file exists on disk.
	data, readErr := os.ReadFile(filepath.Join(root, "subdir/hello.go"))
	if readErr != nil {
		t.Fatalf("file not found on disk: %v", readErr)
	}
	if string(data) != "package main\n" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestHandleWriteFile_EmptyPath(t *testing.T) {
	m := &FileToolModule{Root: t.TempDir()}
	_, err := m.handleWriteFile(context.Background(), WriteFileInput{
		Path:    "",
		Content: "x",
	})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestHandleWriteFile_TraversalAttack(t *testing.T) {
	m := &FileToolModule{Root: t.TempDir()}
	_, err := m.handleWriteFile(context.Background(), WriteFileInput{
		Path:    "../escape.txt",
		Content: "x",
	})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestHandleWriteFile_AbsolutePath(t *testing.T) {
	m := &FileToolModule{Root: t.TempDir()}
	_, err := m.handleWriteFile(context.Background(), WriteFileInput{
		Path:    "/etc/passwd",
		Content: "x",
	})
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestHandleReadFile_Success(t *testing.T) {
	root := t.TempDir()
	content := "package test\n"
	os.WriteFile(filepath.Join(root, "test.go"), []byte(content), 0o644)

	m := &FileToolModule{Root: root}
	out, err := m.handleReadFile(context.Background(), ReadFileInput{Path: "test.go"})
	if err != nil {
		t.Fatalf("handleReadFile: %v", err)
	}
	if out.Content != content {
		t.Errorf("Content = %q, want %q", out.Content, content)
	}
	if out.Path != "test.go" {
		t.Errorf("Path = %q, want %q", out.Path, "test.go")
	}
	if out.Bytes != len(content) {
		t.Errorf("Bytes = %d, want %d", out.Bytes, len(content))
	}
}

func TestHandleReadFile_EmptyPath(t *testing.T) {
	m := &FileToolModule{Root: t.TempDir()}
	_, err := m.handleReadFile(context.Background(), ReadFileInput{Path: ""})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestHandleReadFile_TraversalAttack(t *testing.T) {
	m := &FileToolModule{Root: t.TempDir()}
	_, err := m.handleReadFile(context.Background(), ReadFileInput{Path: "../../etc/passwd"})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestHandleReadFile_NotFound(t *testing.T) {
	m := &FileToolModule{Root: t.TempDir()}
	_, err := m.handleReadFile(context.Background(), ReadFileInput{Path: "nonexistent.go"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestHandleReadFile_Truncation(t *testing.T) {
	root := t.TempDir()
	// Write a file larger than maxLen (32000).
	bigContent := strings.Repeat("x", 40000)
	os.WriteFile(filepath.Join(root, "big.txt"), []byte(bigContent), 0o644)

	m := &FileToolModule{Root: root}
	out, err := m.handleReadFile(context.Background(), ReadFileInput{Path: "big.txt"})
	if err != nil {
		t.Fatalf("handleReadFile: %v", err)
	}
	if !strings.HasSuffix(out.Content, "... (truncated)") {
		t.Error("expected content to be truncated")
	}
	if out.Bytes != 40000 {
		t.Errorf("Bytes = %d, want 40000 (original size)", out.Bytes)
	}
}

func TestHandleListDir_Root(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "file.txt"), []byte("hello"), 0o644)
	os.Mkdir(filepath.Join(root, "subdir"), 0o755)

	m := &FileToolModule{Root: root}
	out, err := m.handleListDir(context.Background(), ListDirInput{Path: ""})
	if err != nil {
		t.Fatalf("handleListDir: %v", err)
	}
	if out.Count < 2 {
		t.Errorf("Count = %d, want >= 2", out.Count)
	}
	// Check that directories have trailing slash.
	hasDir := false
	hasFile := false
	for _, name := range out.Entries {
		if name == "subdir/" {
			hasDir = true
		}
		if name == "file.txt" {
			hasFile = true
		}
	}
	if !hasDir {
		t.Error("expected 'subdir/' in entries")
	}
	if !hasFile {
		t.Error("expected 'file.txt' in entries")
	}
}

func TestHandleListDir_Subdir(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "sub", "inner.go"), []byte("package sub"), 0o644)

	m := &FileToolModule{Root: root}
	out, err := m.handleListDir(context.Background(), ListDirInput{Path: "sub"})
	if err != nil {
		t.Fatalf("handleListDir: %v", err)
	}
	if out.Count != 1 {
		t.Errorf("Count = %d, want 1", out.Count)
	}
	if out.Entries[0] != "inner.go" {
		t.Errorf("Entries[0] = %q, want %q", out.Entries[0], "inner.go")
	}
}

func TestHandleListDir_TraversalAttack(t *testing.T) {
	m := &FileToolModule{Root: t.TempDir()}
	_, err := m.handleListDir(context.Background(), ListDirInput{Path: "../.."})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestHandleListDir_NotFound(t *testing.T) {
	m := &FileToolModule{Root: t.TempDir()}
	_, err := m.handleListDir(context.Background(), ListDirInput{Path: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestHandleTestPackage_EmptyPackage(t *testing.T) {
	m := &FileToolModule{Root: t.TempDir()}
	_, err := m.handleTestPackage(context.Background(), TestPackageInput{Package: ""})
	if err == nil {
		t.Fatal("expected error for empty package")
	}
}

func TestHandleTestPackage_MinimalProject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	// Create a minimal Go project.
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main_test.go"), []byte("package main\nimport \"testing\"\nfunc TestNoop(t *testing.T) {}\n"), 0o644)

	m := &FileToolModule{Root: root}
	out, err := m.handleTestPackage(context.Background(), TestPackageInput{Package: "./"})
	if err != nil {
		t.Fatalf("handleTestPackage: %v", err)
	}
	if !out.Passed {
		t.Errorf("expected Passed=true, output: %s", out.Output)
	}
}

func TestHandleTestPackage_FailingProject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main_test.go"), []byte("package main\nimport \"testing\"\nfunc TestFail(t *testing.T) { t.Fatal(\"boom\") }\n"), 0o644)

	m := &FileToolModule{Root: root}
	out, err := m.handleTestPackage(context.Background(), TestPackageInput{Package: "./"})
	if err != nil {
		t.Fatalf("handleTestPackage: %v", err)
	}
	if out.Passed {
		t.Error("expected Passed=false for failing tests")
	}
	if !strings.Contains(out.Output, "boom") {
		t.Errorf("expected 'boom' in output, got: %s", out.Output)
	}
}

func TestHandleCheckCoverage_EmptyPackage(t *testing.T) {
	m := &FileToolModule{Root: t.TempDir()}
	_, err := m.handleCheckCoverage(context.Background(), CheckCoverageInput{Package: ""})
	if err == nil {
		t.Fatal("expected error for empty package")
	}
}

func TestHandleCheckCoverage_MinimalProject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "add.go"), []byte("package main\nfunc Add(a, b int) int { return a + b }\nfunc main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "add_test.go"), []byte("package main\nimport \"testing\"\nfunc TestAdd(t *testing.T) { if Add(1,2) != 3 { t.Fatal() } }\n"), 0o644)

	m := &FileToolModule{Root: root}
	out, err := m.handleCheckCoverage(context.Background(), CheckCoverageInput{Package: "./"})
	if err != nil {
		t.Fatalf("handleCheckCoverage: %v", err)
	}
	if out.Coverage < 50 {
		t.Errorf("expected coverage >= 50%%, got %.1f%%", out.Coverage)
	}
}
