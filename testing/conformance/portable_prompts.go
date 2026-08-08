//go:build !official_sdk

// portable_prompts.go — the conformance prompts module used by both
// NewEverythingServer's PromptsModule (unchanged, mcp-go-only,
// everything_server.go) and NewPortableEverythingServer's
// PortablePromptsModule (this file's !official_sdk half plus
// portable_prompts_official.go's official_sdk half). None of
// PromptsModule.Prompts()'s 8 prompts touch MCPServer, so unlike
// ToolsModule's sampling/elicitation tools the full prompt set ports
// cleanly — single-message prompts use the TextPromptHandler adapter
// (prompts/handler_adapters.go); the two-message resource/image prompts are
// hand-rolled per tag since neither adapter covers multi-message results.
package conformance

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	mcpprompts "github.com/hairglasses-studio/mcpkit/prompts"
)

// PortablePromptsModule implements the conformance prompts that have no
// MCPServer dependency (all 8 of PromptsModule's prompts).
type PortablePromptsModule struct{}

// Name returns the module name.
func (m *PortablePromptsModule) Name() string { return "conformance-portable-prompts" }

// Description returns the module description.
func (m *PortablePromptsModule) Description() string {
	return "MCP conformance suite prompts (portable subset): simple, complex with args, embedded resource, with image"
}

// Prompts returns the portable conformance prompt definitions.
func (m *PortablePromptsModule) Prompts() []mcpprompts.PromptDefinition {
	return []mcpprompts.PromptDefinition{
		{
			Prompt: mcp.NewPrompt("simple_prompt",
				mcp.WithPromptDescription("A simple prompt with no arguments"),
			),
			Handler: mcpprompts.TextPromptHandler(func(_ context.Context, _ map[string]string) (string, string, error) {
				return "A simple prompt", "This is a simple prompt with no arguments.", nil
			}),
			Category: "conformance",
		},
		{
			Prompt: mcp.NewPrompt("complex_prompt",
				mcp.WithPromptDescription("A complex prompt with arguments"),
				mcp.WithArgument("name", mcp.RequiredArgument(), mcp.ArgumentDescription("The user's name")),
				mcp.WithArgument("style", mcp.ArgumentDescription("Response style: formal or casual (default: formal)")),
			),
			Handler: mcpprompts.TextPromptHandler(func(_ context.Context, args map[string]string) (string, string, error) {
				name, style := args["name"], args["style"]
				if style == "" {
					style = "formal"
				}
				return fmt.Sprintf("Complex prompt for %s (%s style)", name, style),
					fmt.Sprintf("Please greet %s in a %s style.", name, style), nil
			}),
			Category: "conformance",
		},
		{
			Prompt: mcp.NewPrompt("resource_prompt",
				mcp.WithPromptDescription("A prompt with an embedded resource"),
			),
			Handler: func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return &mcp.GetPromptResult{
					Description: "A prompt with embedded resource content",
					Messages: []mcp.PromptMessage{
						mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("Please review the following resource:")),
						{
							Role: mcp.RoleUser,
							Content: mcp.EmbeddedResource{
								Type: "resource",
								Resource: mcp.TextResourceContents{
									URI:      "test://static-text",
									MIMEType: "text/plain",
									Text:     "This is a static text resource for conformance testing.",
								},
							},
						},
					},
				}, nil
			},
			Category: "conformance",
		},
		{
			Prompt: mcp.NewPrompt("image_prompt",
				mcp.WithPromptDescription("A prompt with an image"),
			),
			Handler: func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return &mcp.GetPromptResult{
					Description: "A prompt with image content",
					Messages: []mcp.PromptMessage{
						mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("Please describe this image:")),
						mcp.NewPromptMessage(mcp.RoleUser, mcp.NewImageContent(tinyImageBase64, "image/png")),
					},
				}, nil
			},
			Category: "conformance",
		},
		{
			Prompt: mcp.NewPrompt("test_simple_prompt",
				mcp.WithPromptDescription("A simple prompt for conformance testing"),
			),
			Handler: mcpprompts.TextPromptHandler(func(_ context.Context, _ map[string]string) (string, string, error) {
				return "", "This is a simple prompt for testing.", nil
			}),
			Category: "conformance",
		},
		{
			Prompt: mcp.NewPrompt("test_prompt_with_arguments",
				mcp.WithPromptDescription("A prompt with required arguments for conformance testing"),
				mcp.WithArgument("arg1", mcp.RequiredArgument(), mcp.ArgumentDescription("First test argument")),
				mcp.WithArgument("arg2", mcp.RequiredArgument(), mcp.ArgumentDescription("Second test argument")),
			),
			Handler: mcpprompts.TextPromptHandler(func(_ context.Context, args map[string]string) (string, string, error) {
				return "", fmt.Sprintf("Prompt with arguments: arg1='%s', arg2='%s'", args["arg1"], args["arg2"]), nil
			}),
			Category: "conformance",
		},
		{
			Prompt: mcp.NewPrompt("test_prompt_with_embedded_resource",
				mcp.WithPromptDescription("A prompt with an embedded resource for conformance testing"),
				mcp.WithArgument("resourceUri", mcp.RequiredArgument(), mcp.ArgumentDescription("URI of the resource to embed")),
			),
			Handler: func(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				resourceURI := req.Params.Arguments["resourceUri"]
				return &mcp.GetPromptResult{
					Messages: []mcp.PromptMessage{
						{
							Role: mcp.RoleUser,
							Content: mcp.EmbeddedResource{
								Type: "resource",
								Resource: mcp.TextResourceContents{
									URI:      resourceURI,
									MIMEType: "text/plain",
									Text:     "Embedded resource content for testing.",
								},
							},
						},
						mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("Please process the embedded resource above.")),
					},
				}, nil
			},
			Category: "conformance",
		},
		{
			Prompt: mcp.NewPrompt("test_prompt_with_image",
				mcp.WithPromptDescription("A prompt with an image for conformance testing"),
			),
			Handler: func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return &mcp.GetPromptResult{
					Messages: []mcp.PromptMessage{
						mcp.NewPromptMessage(mcp.RoleUser, mcp.NewImageContent(tinyImageBase64, "image/png")),
						mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("Please analyze the image above.")),
					},
				}, nil
			},
			Category: "conformance",
		},
	}
}
