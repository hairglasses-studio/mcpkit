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
}

func (m *Module) verifyTool() registry.ToolDefinition {
	desc := "Run a build/test command and return a structured pass/fail result. " +
		"Defaults to 'make check' which runs build, vet, and all tests. " +
		"Use command to specify an alternative command (e.g. 'go test ./...'). " +
		"Use packages to append specific package paths to the command. " +
		"The tool enforces a 5-minute execution timeout."

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

	artifactID := fmt.Sprintf("verify-%d", time.Now().UnixNano())
	_ = m.store.Save(Artifact{
		ID:        artifactID,
		Type:      "verify",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Content: map[string]any{
			"command":  cmdStr,
			"passed":   passed,
			"duration": duration.String(),
			"output":   output,
		},
	})

	return VerifyOutput{
		Passed:     passed,
		Command:    cmdStr,
		Output:     output,
		Duration:   duration.String(),
		ArtifactID: artifactID,
	}, nil
}
