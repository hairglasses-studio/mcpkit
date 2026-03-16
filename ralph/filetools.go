//go:build !official_sdk

package ralph

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// FileToolModule provides read_file, write_file, and list_dir tools
// so the autonomous loop can create and modify source code.
// Root is the project root — all paths are resolved relative to it.
type FileToolModule struct {
	Root string
}

func (m *FileToolModule) Name() string        { return "file_tools" }
func (m *FileToolModule) Description() string { return "File I/O tools for reading, writing, and listing project files" }

func (m *FileToolModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		m.writeFileTool(),
		m.readFileTool(),
		m.listDirTool(),
		m.testPackageTool(),
		m.checkCoverageTool(),
	}
}

// --- write_file ---

type WriteFileInput struct {
	Path    string `json:"path" jsonschema:"required,description=File path relative to project root (e.g. session/session.go)"`
	Content string `json:"content" jsonschema:"required,description=Full file content to write"`
}

type WriteFileOutput struct {
	Written bool   `json:"written"`
	Path    string `json:"path"`
	Bytes   int    `json:"bytes"`
}

func (m *FileToolModule) writeFileTool() registry.ToolDefinition {
	desc := "Write content to a file, creating directories as needed. " +
		"Path is relative to the project root. Overwrites existing files. " +
		"Use this to create new Go source files, test files, or config files."

	td := handler.TypedHandler[WriteFileInput, WriteFileOutput](
		"write_file",
		desc,
		m.handleWriteFile,
	)
	td.Category = "file"
	td.Timeout = 10 * time.Second
	td.IsWrite = true
	return td
}

func (m *FileToolModule) handleWriteFile(ctx context.Context, input WriteFileInput) (WriteFileOutput, error) {
	if input.Path == "" {
		return WriteFileOutput{}, fmt.Errorf("path is required")
	}
	clean := filepath.Clean(input.Path)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return WriteFileOutput{}, fmt.Errorf("path must be relative and within project root")
	}

	full := filepath.Join(m.Root, clean)

	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return WriteFileOutput{}, fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(full, []byte(input.Content), 0644); err != nil {
		return WriteFileOutput{}, fmt.Errorf("write file: %w", err)
	}

	return WriteFileOutput{
		Written: true,
		Path:    clean,
		Bytes:   len(input.Content),
	}, nil
}

// --- read_file ---

type ReadFileInput struct {
	Path string `json:"path" jsonschema:"required,description=File path relative to project root"`
}

type ReadFileOutput struct {
	Content string `json:"content"`
	Path    string `json:"path"`
	Bytes   int    `json:"bytes"`
}

func (m *FileToolModule) readFileTool() registry.ToolDefinition {
	desc := "Read the contents of a file. Path is relative to the project root. " +
		"Use this to inspect existing source files before modifying them."

	td := handler.TypedHandler[ReadFileInput, ReadFileOutput](
		"read_file",
		desc,
		m.handleReadFile,
	)
	td.Category = "file"
	td.Timeout = 10 * time.Second
	return td
}

func (m *FileToolModule) handleReadFile(ctx context.Context, input ReadFileInput) (ReadFileOutput, error) {
	if input.Path == "" {
		return ReadFileOutput{}, fmt.Errorf("path is required")
	}
	clean := filepath.Clean(input.Path)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return ReadFileOutput{}, fmt.Errorf("path must be relative and within project root")
	}

	full := filepath.Join(m.Root, clean)
	data, err := os.ReadFile(full)
	if err != nil {
		return ReadFileOutput{}, fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	const maxLen = 32000
	if len(content) > maxLen {
		content = content[:maxLen] + "\n... (truncated)"
	}

	return ReadFileOutput{
		Content: content,
		Path:    clean,
		Bytes:   len(data),
	}, nil
}

// --- list_dir ---

type ListDirInput struct {
	Path string `json:"path,omitempty" jsonschema:"description=Directory path relative to project root (default: root)"`
}

type ListDirOutput struct {
	Entries []string `json:"entries"`
	Count   int      `json:"count"`
}

func (m *FileToolModule) listDirTool() registry.ToolDefinition {
	desc := "List files and directories in a path. Returns entries as 'name' for files and 'name/' for directories. " +
		"Path is relative to the project root. Defaults to root if empty."

	td := handler.TypedHandler[ListDirInput, ListDirOutput](
		"list_dir",
		desc,
		m.handleListDir,
	)
	td.Category = "file"
	td.Timeout = 10 * time.Second
	return td
}

func (m *FileToolModule) handleListDir(ctx context.Context, input ListDirInput) (ListDirOutput, error) {
	dir := input.Path
	if dir == "" {
		dir = "."
	}
	clean := filepath.Clean(dir)
	if strings.HasPrefix(clean, "..") || (filepath.IsAbs(clean) && clean != ".") {
		return ListDirOutput{}, fmt.Errorf("path must be relative and within project root")
	}

	full := filepath.Join(m.Root, clean)
	entries, err := os.ReadDir(full)
	if err != nil {
		return ListDirOutput{}, fmt.Errorf("list dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}

	return ListDirOutput{
		Entries: names,
		Count:   len(names),
	}, nil
}

// --- test_package ---

type TestPackageInput struct {
	Package string `json:"package" jsonschema:"required,description=Go package path relative to project root (e.g. ralph or ./ralph/...)"`
}

type TestPackageOutput struct {
	Passed bool   `json:"passed"`
	Output string `json:"output"`
}

func (m *FileToolModule) testPackageTool() registry.ToolDefinition {
	desc := "Run go test -count=1 -v on a package and return the output. " +
		"Package path is relative to the project root (e.g. 'ralph' or './ralph/...'). " +
		"Use this to verify that tests pass after making changes."

	td := handler.TypedHandler[TestPackageInput, TestPackageOutput](
		"test_package",
		desc,
		m.handleTestPackage,
	)
	td.Category = "test"
	td.Timeout = 120 * time.Second
	return td
}

func (m *FileToolModule) handleTestPackage(ctx context.Context, input TestPackageInput) (TestPackageOutput, error) {
	if input.Package == "" {
		return TestPackageOutput{}, fmt.Errorf("package is required")
	}

	pkg := input.Package
	if !strings.HasPrefix(pkg, "./") {
		pkg = "./" + pkg
	}
	if !strings.HasSuffix(pkg, "/...") && !strings.HasSuffix(pkg, "/") {
		pkg += "/"
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", pkg)
	cmd.Dir = m.Root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	output := out.String()
	if len(output) > 8000 {
		output = output[:8000] + "\n... (truncated)"
	}

	return TestPackageOutput{
		Passed: err == nil,
		Output: output,
	}, nil
}

// --- check_coverage ---

type CheckCoverageInput struct {
	Package string `json:"package" jsonschema:"required,description=Go package path relative to project root (e.g. ralph)"`
}

type CheckCoverageOutput struct {
	Package  string  `json:"package"`
	Coverage float64 `json:"coverage"`
	Output   string  `json:"output"`
}

func (m *FileToolModule) checkCoverageTool() registry.ToolDefinition {
	desc := "Run go test -cover on a package and parse the coverage percentage. " +
		"Package path is relative to the project root (e.g. 'ralph'). " +
		"Returns the coverage percentage and full output."

	td := handler.TypedHandler[CheckCoverageInput, CheckCoverageOutput](
		"check_coverage",
		desc,
		m.handleCheckCoverage,
	)
	td.Category = "test"
	td.Timeout = 120 * time.Second
	return td
}

func (m *FileToolModule) handleCheckCoverage(ctx context.Context, input CheckCoverageInput) (CheckCoverageOutput, error) {
	if input.Package == "" {
		return CheckCoverageOutput{}, fmt.Errorf("package is required")
	}

	pkg := input.Package
	if !strings.HasPrefix(pkg, "./") {
		pkg = "./" + pkg
	}
	if !strings.HasSuffix(pkg, "/...") && !strings.HasSuffix(pkg, "/") {
		pkg += "/"
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-cover", pkg)
	cmd.Dir = m.Root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	output := out.String()
	coverage := parseCoverage(output)

	if err != nil && coverage == 0 {
		return CheckCoverageOutput{
			Package:  pkg,
			Coverage: 0,
			Output:   output,
		}, nil
	}

	return CheckCoverageOutput{
		Package:  pkg,
		Coverage: coverage,
		Output:   output,
	}, nil
}

// parseCoverage extracts the coverage percentage from `go test -cover` output.
// Looks for patterns like "coverage: 85.2% of statements".
func parseCoverage(output string) float64 {
	// Find "coverage: XX.X% of statements"
	idx := strings.Index(output, "coverage: ")
	if idx < 0 {
		return 0
	}
	rest := output[idx+len("coverage: "):]
	pctIdx := strings.Index(rest, "%")
	if pctIdx < 0 {
		return 0
	}
	pctStr := rest[:pctIdx]
	var pct float64
	fmt.Sscanf(pctStr, "%f", &pct)
	return pct
}
