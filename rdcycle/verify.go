package rdcycle

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// VerifyInput is the input for the rdcycle_verify tool.
type VerifyInput struct {
	Command  string   `json:"command,omitempty" jsonschema:"description=Command to run (default: make check)"`
	Packages []string `json:"packages,omitempty" jsonschema:"description=Specific packages to test (e.g. ['./roadmap/...'])"`
}

// VerifyOutput is the output of the rdcycle_verify tool.
type VerifyOutput struct {
	Passed     bool   `json:"passed"`
	Command    string `json:"command"`
	Output     string `json:"output"`
	Duration   string `json:"duration"`
	ArtifactID string `json:"artifact_id"`
	// NoChanges is true when the command passed and git diff reports no uncommitted
	// changes. When both Passed and NoChanges are true, the verify task is done —
	// call mark_done immediately without re-entering the verify loop.
	NoChanges bool `json:"no_changes"`
}

func (m *Module) verifyTool() registry.ToolDefinition {
	desc := "Run a build/test command and return a structured pass/fail result. " +
		"Defaults to 'make check' which runs build, vet, and all tests. " +
		"Use command to specify an alternative command (e.g. 'go test ./...'). " +
		"Use packages to append specific package paths to the command. " +
		"The tool enforces a 5-minute execution timeout. " +
		"When the result has passed=true and no_changes=true, the check succeeded with no " +
		"uncommitted code changes — call mark_done immediately instead of looping."

	td := handler.TypedHandler[VerifyInput, VerifyOutput](
		"rdcycle_verify",
		desc,
		m.handleVerify,
	)
	td.Category = "rdcycle"
	td.Timeout = 5 * time.Minute
	td.Complexity = registry.ComplexityModerate
	return td
}

func (m *Module) handleVerify(ctx context.Context, input VerifyInput) (VerifyOutput, error) {
	cmdStr := input.Command
	if cmdStr == "" {
		cmdStr = "make check"
	}

	// Append package paths if provided.
	if len(input.Packages) > 0 {
		cmdStr = cmdStr + " " + strings.Join(input.Packages, " ")
	}

	// Use a 5-minute context timeout for the subprocess.
	execCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	start := time.Now()

	// Split the command string into name + args for exec.
	parts := strings.Fields(cmdStr)
	name := parts[0]
	args := parts[1:]

	workDir := m.config.GitRoot

	cmd := exec.CommandContext(execCtx, name, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()
	duration := time.Since(start)

	passed := runErr == nil
	output := buf.String()

	// When the command passed, check whether there are uncommitted changes.
	// A clean diff means no code was written, so the verify task is already done.
	noChanges := false
	if passed {
		noChanges = gitDiffQuiet(ctx, workDir)
	}

	artifactID := fmt.Sprintf("verify-%d", time.Now().UnixNano())
	_ = m.store.Save(Artifact{
		ID:        artifactID,
		Type:      "verify",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Content: map[string]any{
			"command":    cmdStr,
			"passed":     passed,
			"no_changes": noChanges,
			"duration":   duration.String(),
			"output":     output,
		},
	})

	return VerifyOutput{
		Passed:     passed,
		Command:    cmdStr,
		Output:     output,
		Duration:   duration.String(),
		ArtifactID: artifactID,
		NoChanges:  noChanges,
	}, nil
}

// gitDiffQuiet runs "git diff --quiet" in the given directory and returns true
// when the working tree has no uncommitted changes (exit status 0).
// Returns false on any error or when there are changes (exit status 1).
func gitDiffQuiet(ctx context.Context, dir string) bool {
	diffCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(diffCtx, "git", "diff", "--quiet")
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Run() == nil
}
