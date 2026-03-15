//go:build !official_sdk

// Command elicitation demonstrates MCP elicitation patterns for requesting
// additional information from the client during tool execution.
//
// Three tools illustrate the three elicitation APIs:
//
//   - delete_item: uses ElicitForm with a simple confirmation schema to guard
//     a destructive action.
//   - connect_service: uses ElicitURL to drive an OAuth-style redirect flow.
//   - survey: uses ElicitFormSchema to build a multi-field form with string,
//     number, and boolean fields.
//
// Run:
//
//	go run ./examples/elicitation/
//
// The server speaks stdio. Connect an MCP client that declares the
// elicitation capability to interact with the tools.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ---------------------------------------------------------------------------
// delete_item — ElicitForm: confirm before a destructive action
// ---------------------------------------------------------------------------

// DeleteItemInput is the input schema for delete_item.
type DeleteItemInput struct {
	ItemID string `json:"item_id" jsonschema:"required,description=ID of the item to delete"`
}

// deleteItemHandler asks the client to confirm before deleting.
func deleteItemHandler(ctx context.Context, input DeleteItemInput) (string, error) {
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		return "", fmt.Errorf("no MCP server in context")
	}

	// Build a confirmation form using ElicitForm.
	// The schema must be a flat object per the MCP elicitation spec.
	schema := handler.ElicitForm(
		fmt.Sprintf("Are you sure you want to permanently delete item %q? This cannot be undone.", input.ItemID),
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"confirmed": map[string]any{
					"type":        "boolean",
					"description": "Set to true to confirm deletion",
				},
			},
			"required": []string{"confirmed"},
		},
	)

	result, err := srv.RequestElicitation(ctx, mcp.ElicitationRequest{Params: schema})
	if err != nil {
		return "", fmt.Errorf("elicitation failed: %w", err)
	}

	switch result.Action {
	case mcp.ElicitationResponseActionAccept:
		// Check the form response.
		content, ok := result.Content.(map[string]any)
		if !ok || content["confirmed"] != true {
			return fmt.Sprintf("Deletion of %q cancelled (not confirmed).", input.ItemID), nil
		}
		// Perform the deletion (simulated here).
		return fmt.Sprintf("Item %q has been permanently deleted.", input.ItemID), nil
	case mcp.ElicitationResponseActionDecline:
		return fmt.Sprintf("Deletion of %q declined by user.", input.ItemID), nil
	default:
		return fmt.Sprintf("Deletion of %q cancelled.", input.ItemID), nil
	}
}

// ---------------------------------------------------------------------------
// connect_service — ElicitURL: OAuth-style redirect flow
// ---------------------------------------------------------------------------

// ConnectServiceInput is the input schema for connect_service.
type ConnectServiceInput struct {
	ServiceName string `json:"service_name" jsonschema:"required,description=Name of the service to connect (e.g. github)"`
}

// connectServiceHandler redirects the user to an OAuth authorization URL.
func connectServiceHandler(ctx context.Context, input ConnectServiceInput) (string, error) {
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		return "", fmt.Errorf("no MCP server in context")
	}

	// In production this URL would be generated with a real OAuth state parameter.
	elicitationID := fmt.Sprintf("oauth-%s-001", input.ServiceName)
	authURL := fmt.Sprintf("https://auth.example.com/oauth/authorize?service=%s&state=%s", input.ServiceName, elicitationID)

	// ElicitURL sends the user to the URL and waits for the elicitation/complete notification.
	params := handler.ElicitURL(
		fmt.Sprintf("Authorize mcpkit to access your %s account. Click the link to continue.", input.ServiceName),
		elicitationID,
		authURL,
	)

	result, err := srv.RequestElicitation(ctx, mcp.ElicitationRequest{Params: params})
	if err != nil {
		return "", fmt.Errorf("elicitation failed: %w", err)
	}

	switch result.Action {
	case mcp.ElicitationResponseActionAccept:
		return fmt.Sprintf("Successfully connected to %s. You can now use %s tools.", input.ServiceName, input.ServiceName), nil
	case mcp.ElicitationResponseActionDecline:
		return fmt.Sprintf("Connection to %s declined.", input.ServiceName), nil
	default:
		return fmt.Sprintf("Connection to %s cancelled.", input.ServiceName), nil
	}
}

// ---------------------------------------------------------------------------
// survey — ElicitFormSchema: multi-field form with mixed types
// ---------------------------------------------------------------------------

// SurveyInput is the input schema for the survey tool.
type SurveyInput struct {
	Topic string `json:"topic" jsonschema:"required,description=Topic of the survey (e.g. onboarding)"`
}

// surveyHandler presents a multi-field form and echoes the responses back.
func surveyHandler(ctx context.Context, input SurveyInput) (string, error) {
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		return "", fmt.Errorf("no MCP server in context")
	}

	// ElicitFormSchema builds a flat JSON Schema from typed field descriptors.
	schema := handler.ElicitFormSchema(
		handler.FormField{
			Name:        "name",
			Type:        "string",
			Description: "Your full name",
			Required:    true,
		},
		handler.FormField{
			Name:        "satisfaction",
			Type:        "number",
			Description: "Overall satisfaction score (1–10)",
			Required:    true,
		},
		handler.FormField{
			Name:        "would_recommend",
			Type:        "boolean",
			Description: "Would you recommend this product?",
		},
		handler.FormField{
			Name:        "preferred_channel",
			Type:        "string",
			Description: "Preferred support channel",
			Enum:        []string{"email", "chat", "phone"},
			Default:     "email",
		},
	)

	result, err := srv.RequestElicitation(ctx, mcp.ElicitationRequest{
		Params: handler.ElicitForm(
			fmt.Sprintf("Please complete the %s survey:", input.Topic),
			schema,
		),
	})
	if err != nil {
		return "", fmt.Errorf("elicitation failed: %w", err)
	}

	if result.Action != mcp.ElicitationResponseActionAccept {
		return "Survey skipped — no response recorded.", nil
	}

	content, ok := result.Content.(map[string]any)
	if !ok {
		return "Survey submitted (no content).", nil
	}

	return fmt.Sprintf(
		"Survey recorded for topic %q:\n  name: %v\n  satisfaction: %v\n  would_recommend: %v\n  preferred_channel: %v",
		input.Topic,
		content["name"],
		content["satisfaction"],
		content["would_recommend"],
		content["preferred_channel"],
	), nil
}

// ---------------------------------------------------------------------------
// Tool module
// ---------------------------------------------------------------------------

// ElicitationModule provides the three elicitation demo tools.
type ElicitationModule struct{}

func (m *ElicitationModule) Name() string        { return "elicitation" }
func (m *ElicitationModule) Description() string { return "Elicitation demo tools" }

func (m *ElicitationModule) Tools() []registry.ToolDefinition {
	// delete_item: destructive action guarded by a confirmation form.
	deleteItem := handler.TypedHandler[DeleteItemInput, string](
		"delete_item",
		"Permanently delete an item by ID. Asks the client to confirm before proceeding.",
		deleteItemHandler,
	)
	deleteItem.Category = "data"
	deleteItem.Tags = []string{"delete", "write", "destructive"}
	deleteItem.IsWrite = true

	// connect_service: OAuth-style URL redirect for authorization.
	connectService := handler.TypedHandler[ConnectServiceInput, string](
		"connect_service",
		"Connect an external service via OAuth. Redirects the client to the authorization URL and waits for completion.",
		connectServiceHandler,
	)
	connectService.Category = "auth"
	connectService.Tags = []string{"oauth", "connect"}

	// survey: multi-field form with string, number, boolean, and enum fields.
	survey := handler.TypedHandler[SurveyInput, string](
		"survey",
		"Run a short satisfaction survey. Presents a form with name, score, recommendation, and channel preference fields.",
		surveyHandler,
	)
	survey.Category = "feedback"
	survey.Tags = []string{"form", "survey"}

	return []registry.ToolDefinition{deleteItem, connectService, survey}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	ctx := context.Background()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&ElicitationModule{})

	// WithElicitation() declares the elicitation server capability during
	// initialization, which signals to the client that it may receive
	// elicitation/create requests.
	s := server.NewMCPServer(
		"elicitation-example",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithElicitation(),
		server.WithRecovery(),
	)
	reg.RegisterWithServer(s)

	log.Println("elicitation-example: serving on stdio")
	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
	_ = ctx
}
