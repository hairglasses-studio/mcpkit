//go:build official_sdk

// portable_prompts_official.go is the official_sdk counterpart to
// portable_prompts.go — same PortablePromptsModule shape and 8 prompts,
// adapted to go-sdk's Prompt/PromptArgument struct-literal construction (no
// mcp-go-style functional options) and its pointer-based
// []*mcp.PromptMessage / *mcp.TextContent / *mcp.ImageContent /
// *mcp.EmbeddedResource / *mcp.ResourceContents shapes.
package conformance

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	mcpprompts "github.com/hairglasses-studio/mcpkit/prompts"
	"github.com/hairglasses-studio/mcpkit/registry"
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
			Prompt: mcp.Prompt{
				Name:        "simple_prompt",
				Description: "A simple prompt with no arguments",
			},
			Handler: mcpprompts.TextPromptHandler(func(_ context.Context, _ map[string]string) (string, string, error) {
				return "A simple prompt", "This is a simple prompt with no arguments.", nil
			}),
			Category: "conformance",
		},
		{
			Prompt: mcp.Prompt{
				Name:        "complex_prompt",
				Description: "A complex prompt with arguments",
				Arguments: []*mcp.PromptArgument{
					{Name: "name", Description: "The user's name", Required: true},
					{Name: "style", Description: "Response style: formal or casual (default: formal)"},
				},
			},
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
			Prompt: mcp.Prompt{
				Name:        "resource_prompt",
				Description: "A prompt with an embedded resource",
			},
			Handler: func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return &mcp.GetPromptResult{
					Description: "A prompt with embedded resource content",
					Messages: []*mcp.PromptMessage{
						{Role: registry.RoleUser, Content: &mcp.TextContent{Text: "Please review the following resource:"}},
						{
							Role: registry.RoleUser,
							Content: &mcp.EmbeddedResource{
								Resource: &mcp.ResourceContents{
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
			Prompt: mcp.Prompt{
				Name:        "image_prompt",
				Description: "A prompt with an image",
			},
			Handler: func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				imgData, err := base64.StdEncoding.DecodeString(tinyImageBase64)
				if err != nil {
					return nil, err
				}
				return &mcp.GetPromptResult{
					Description: "A prompt with image content",
					Messages: []*mcp.PromptMessage{
						{Role: registry.RoleUser, Content: &mcp.TextContent{Text: "Please describe this image:"}},
						{Role: registry.RoleUser, Content: &mcp.ImageContent{Data: imgData, MIMEType: "image/png"}},
					},
				}, nil
			},
			Category: "conformance",
		},
		{
			Prompt: mcp.Prompt{
				Name:        "test_simple_prompt",
				Description: "A simple prompt for conformance testing",
			},
			Handler: mcpprompts.TextPromptHandler(func(_ context.Context, _ map[string]string) (string, string, error) {
				return "", "This is a simple prompt for testing.", nil
			}),
			Category: "conformance",
		},
		{
			Prompt: mcp.Prompt{
				Name:        "test_prompt_with_arguments",
				Description: "A prompt with required arguments for conformance testing",
				Arguments: []*mcp.PromptArgument{
					{Name: "arg1", Description: "First test argument", Required: true},
					{Name: "arg2", Description: "Second test argument", Required: true},
				},
			},
			Handler: mcpprompts.TextPromptHandler(func(_ context.Context, args map[string]string) (string, string, error) {
				return "", fmt.Sprintf("Prompt with arguments: arg1='%s', arg2='%s'", args["arg1"], args["arg2"]), nil
			}),
			Category: "conformance",
		},
		{
			Prompt: mcp.Prompt{
				Name:        "test_prompt_with_embedded_resource",
				Description: "A prompt with an embedded resource for conformance testing",
				Arguments: []*mcp.PromptArgument{
					{Name: "resourceUri", Description: "URI of the resource to embed", Required: true},
				},
			},
			Handler: func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				resourceURI := req.Params.Arguments["resourceUri"]
				return &mcp.GetPromptResult{
					Messages: []*mcp.PromptMessage{
						{
							Role: registry.RoleUser,
							Content: &mcp.EmbeddedResource{
								Resource: &mcp.ResourceContents{
									URI:      resourceURI,
									MIMEType: "text/plain",
									Text:     "Embedded resource content for testing.",
								},
							},
						},
						{Role: registry.RoleUser, Content: &mcp.TextContent{Text: "Please process the embedded resource above."}},
					},
				}, nil
			},
			Category: "conformance",
		},
		{
			Prompt: mcp.Prompt{
				Name:        "test_prompt_with_image",
				Description: "A prompt with an image for conformance testing",
			},
			Handler: func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				imgData, err := base64.StdEncoding.DecodeString(tinyImageBase64)
				if err != nil {
					return nil, err
				}
				return &mcp.GetPromptResult{
					Messages: []*mcp.PromptMessage{
						{Role: registry.RoleUser, Content: &mcp.ImageContent{Data: imgData, MIMEType: "image/png"}},
						{Role: registry.RoleUser, Content: &mcp.TextContent{Text: "Please analyze the image above."}},
					},
				}, nil
			},
			Category: "conformance",
		},
	}
}
