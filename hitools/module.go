//go:build !official_sdk

package hitools

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// Module provides human interaction tools via MCP elicitation.
// It implements registry.ToolModule.
type Module struct{}

// Name returns the module name.
func (m *Module) Name() string { return "hitools" }

// Description returns a human-readable description of the module.
func (m *Module) Description() string {
	return "Human interaction tools for requesting input from the connected user via MCP elicitation"
}

// Tools returns the tool definitions provided by this module.
func (m *Module) Tools() []registry.ToolDefinition {
	td := handler.TypedHandler[RequestInput, RequestOutput](
		"request_human_input",
		"Request input from the connected human. Supports free-text, yes/no, and multiple-choice formats. "+
			"Uses MCP elicitation to present an interactive form to the user and waits for their response.",
		handleRequestHumanInput,
	)
	td.Category = "human-interaction"
	td.Tags = []string{"elicitation", "input", "human", "interactive"}
	td.IsWrite = false
	td.Complexity = registry.ComplexitySimple
	return []registry.ToolDefinition{td}
}

// buildElicitSchema constructs an ElicitationParams based on the request format.
func buildElicitSchema(input RequestInput) mcp.ElicitationParams {
	message := input.Question
	if input.Context != "" {
		message = fmt.Sprintf("%s\n\nContext: %s", input.Question, input.Context)
	}

	var fields []handler.FormField

	switch input.Format {
	case FormatYesNo:
		fields = append(fields, handler.FormField{
			Name:        "answer",
			Type:        "boolean",
			Description: "Yes (true) or No (false)",
			Required:    true,
		})

	case FormatMultipleChoice:
		fields = append(fields, handler.FormField{
			Name:        "answer",
			Type:        "string",
			Description: "Select one of the provided choices",
			Required:    true,
			Enum:        input.Choices,
		})

	default: // free_text or unset
		fields = append(fields, handler.FormField{
			Name:        "answer",
			Type:        "string",
			Description: "Your response",
			Required:    true,
		})
	}

	schema := handler.ElicitFormSchema(fields...)
	return handler.ElicitForm(message, schema)
}

// handleRequestHumanInput is the typed handler for request_human_input.
func handleRequestHumanInput(ctx context.Context, input RequestInput) (RequestOutput, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Validate multiple_choice requires choices.
	if input.Format == FormatMultipleChoice && len(input.Choices) == 0 {
		return RequestOutput{}, fmt.Errorf("multiple_choice format requires at least one choice")
	}

	srv := server.ServerFromContext(ctx)
	if srv == nil {
		// No server in context — return a pending status so callers can
		// handle the case where elicitation is unavailable (e.g., testing).
		return RequestOutput{
			Status:    "pending",
			Response:  "Elicitation unavailable: no MCP server in context. Question: " + input.Question,
			Timestamp: now,
		}, nil
	}

	params := buildElicitSchema(input)

	result, err := srv.RequestElicitation(ctx, mcp.ElicitationRequest{Params: params})
	if err != nil {
		return RequestOutput{}, fmt.Errorf("elicitation request failed: %w", err)
	}

	switch result.Action {
	case mcp.ElicitationResponseActionAccept:
		response := extractAnswer(result.Content)
		return RequestOutput{
			Status:    "accepted",
			Response:  response,
			Timestamp: now,
		}, nil

	case mcp.ElicitationResponseActionDecline:
		return RequestOutput{
			Status:    "declined",
			Response:  "",
			Timestamp: now,
		}, nil

	default: // cancel or unknown
		return RequestOutput{
			Status:    "timeout",
			Response:  "",
			Timestamp: now,
		}, nil
	}
}

// extractAnswer pulls the "answer" field from the elicitation response content.
func extractAnswer(content any) string {
	m, ok := content.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", content)
	}
	answer, ok := m["answer"]
	if !ok {
		// Fall back to stringifying the whole map.
		return fmt.Sprintf("%v", m)
	}
	return fmt.Sprintf("%v", answer)
}
