package rdcycle

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// CommitInput is the input for the rdcycle_commit tool.
type CommitInput struct {
	Files   []string `json:"files" jsonschema:"required,description=Files to stage and commit (relative to git root)"`
	Message string   `json:"message" jsonschema:"required,description=Commit message"`
	Branch  string   `json:"branch,omitempty" jsonschema:"description=Branch name to commit on (default: current branch). Never pushes to main."`
}

// CommitOutput is the output of the rdcycle_commit tool.
type CommitOutput struct {
	Committed bool   `json:"committed"`
	Branch    string `json:"branch"`
	SHA       string `json:"sha"`
	Message   string `json:"message"`
}

func (m *Module) commitTool() registry.ToolDefinition {
	desc := "Stage files and create a git commit on a feature branch. " +
		"Safety: refuses to commit to main/master branches. " +
		"Validates file paths against the configured git root."

	td := handler.TypedHandler[CommitInput, CommitOutput](
		"rdcycle_commit",
		desc,
		m.handleCommit,
	)
	td.IsWrite = true
	return td
}

func (m *Module) handleCommit(ctx context.Context, input CommitInput) (CommitOutput, error) {
	if len(input.Files) == 0 {
		return CommitOutput{}, fmt.Errorf("at least one file is required")
	}
	if input.Message == "" {
		return CommitOutput{}, fmt.Errorf("commit message is required")
	}

	gitRoot := m.config.GitRoot
	if gitRoot == "" {
		gitRoot = "."
	}

	// Validate file paths are within git root
	for _, f := range input.Files {
		abs := filepath.Join(gitRoot, f)
		rel, err := filepath.Rel(gitRoot, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			return CommitOutput{}, fmt.Errorf("file %q is outside git root", f)
		}
	}

	// Check current branch
	branch, err := gitOutput(ctx, gitRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return CommitOutput{}, fmt.Errorf("get current branch: %w", err)
	}

	// If a branch is specified, check out to it
	if input.Branch != "" {
		branch = input.Branch
		// Create branch if it doesn't exist
		_ = gitRun(ctx, gitRoot, "checkout", "-b", branch)
		if err := gitRun(ctx, gitRoot, "checkout", branch); err != nil {
			return CommitOutput{}, fmt.Errorf("checkout branch %q: %w", branch, err)
		}
	}

	// Safety: refuse main/master
	if branch == "main" || branch == "master" {
		return CommitOutput{}, fmt.Errorf("refusing to commit directly to %q branch", branch)
	}

	// Stage files
	args := append([]string{"add", "--"}, input.Files...)
	if err := gitRun(ctx, gitRoot, args...); err != nil {
		return CommitOutput{}, fmt.Errorf("git add: %w", err)
	}

	// Commit
	if err := gitRun(ctx, gitRoot, "commit", "-m", input.Message); err != nil {
		return CommitOutput{}, fmt.Errorf("git commit: %w", err)
	}

	// Get SHA
	sha, err := gitOutput(ctx, gitRoot, "rev-parse", "--short", "HEAD")
	if err != nil {
		sha = "unknown"
	}

	return CommitOutput{
		Committed: true,
		Branch:    branch,
		SHA:       sha,
		Message:   input.Message,
	}, nil
}

func gitRun(ctx context.Context, dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
