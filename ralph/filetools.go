//go:build !official_sdk

package ralph

import (
	"context"
	"fmt"
	"os"
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
