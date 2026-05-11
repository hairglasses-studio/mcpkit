package refactor

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

type DiscoverScriptsInput struct {
	WorkspacePath string `json:"workspace_path" jsonschema:"required,description=Absolute path to the workspace to scan (e.g., /home/hg/hairglasses-studio)"`
}

type DiscoverScriptsOutput struct {
	Summary string   `json:"summary"`
	Count   int      `json:"count"`
	Scripts []string `json:"scripts"`
}

func discoverScriptsToolDef() registry.ToolDefinition {
	td := handler.TypedHandler[DiscoverScriptsInput, DiscoverScriptsOutput](
		"refactor_discover_scripts",
		"Discover bash and python scripts in a workspace directory suitable for refactoring.",
		handleDiscoverScripts,
	)
	td.Category = "refactor"
	return td
}

func handleDiscoverScripts(ctx context.Context, args DiscoverScriptsInput) (DiscoverScriptsOutput, error) {
	var scripts []string
	var out DiscoverScriptsOutput

	err := filepath.WalkDir(args.WorkspacePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors like permission denied
		}

		// Skip common non-relevant directories
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == "venv" || name == ".venv" || name == "__pycache__" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Match by extension
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".sh" || ext == ".bash" || ext == ".py" {
			scripts = append(scripts, path)
		}

		return nil
	})

	if err != nil {
		return out, fmt.Errorf("failed to scan workspace: %w", err)
	}

	out.Count = len(scripts)
	out.Summary = fmt.Sprintf("Found %d bash/python scripts in %s.", out.Count, args.WorkspacePath)

	// Return a limited list to avoid massive payloads
	limit := 100
	if len(scripts) > limit {
		out.Scripts = scripts[:limit]
	} else {
		out.Scripts = scripts
	}

	return out, nil
}
