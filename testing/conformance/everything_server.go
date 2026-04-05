//go:build !official_sdk

// Package conformance provides the "everything-server" for MCP conformance testing.
//
// The everything-server implements all testable MCP capabilities so the official
// MCP conformance suite (https://github.com/modelcontextprotocol/conformance)
// can validate mcpkit against the protocol specification.
//
// Capabilities implemented:
//   - Tools: echo, add, longRunningOperation, sampleLLM, getTinyImage, annotatedMessage
//   - Resources: static text, static binary, dynamic template
//   - Prompts: simple, complex (with arguments), embedded resource, with image
//   - Logging: log emission during tool calls
//   - Completions: argument completion for prompts and resource templates
package conformance

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/prompts"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resources"
)

// ---------------------------------------------------------------------------
// Tiny 1x1 red PNG (base64), used by getTinyImage and prompts-get-with-image.
// ---------------------------------------------------------------------------

const tinyImageBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="

// ---------------------------------------------------------------------------
// Tool input/output types
// ---------------------------------------------------------------------------

// EchoInput is the input for the echo tool.
type EchoInput struct {
	Message string `json:"message" jsonschema:"required,description=Message to echo back"`
}

// EchoOutput is the output for the echo tool.
type EchoOutput struct {
	Echo string `json:"echo"`
}

// AddInput is the input for the add tool.
type AddInput struct {
	A float64 `json:"a" jsonschema:"required,description=First number"`
	B float64 `json:"b" jsonschema:"required,description=Second number"`
}

// AddOutput is the output for the add tool.
type AddOutput struct {
	Result float64 `json:"result"`
}

// LongRunningInput is the input for the longRunningOperation tool.
type LongRunningInput struct {
	Duration  int    `json:"duration,omitempty" jsonschema:"description=Duration in seconds (default 10)"`
	Steps     int    `json:"steps,omitempty" jsonschema:"description=Number of progress steps (default 5)"`
	StepLabel string `json:"stepLabel,omitempty" jsonschema:"description=Label prefix for progress messages"`
}

// SampleLLMInput is the input for the sampleLLM tool.
type SampleLLMInput struct {
	Prompt   string `json:"prompt" jsonschema:"required,description=Prompt to send to the LLM"`
	MaxWords int    `json:"maxWords,omitempty" jsonschema:"description=Maximum number of words in the response"`
}

// AnnotatedMessageInput is the input for the annotatedMessage tool.
type AnnotatedMessageInput struct {
	MessageType string  `json:"messageType" jsonschema:"required,description=Type of message to return,enum=error,enum=success,enum=debug"`
	IncludeImage bool   `json:"includeImage,omitempty" jsonschema:"description=Whether to include an image in the response"`
}

// ---------------------------------------------------------------------------
// Tool module
// ---------------------------------------------------------------------------

// ToolsModule implements all conformance test tools.
type ToolsModule struct {
	MCPServer *server.MCPServer
}

// Name returns the module name.
func (m *ToolsModule) Name() string { return "conformance-tools" }

// Description returns the module description.
func (m *ToolsModule) Description() string {
	return "MCP conformance suite tools: echo, add, longRunningOperation, sampleLLM, getTinyImage, annotatedMessage"
}

// Tools returns all conformance tool definitions.
func (m *ToolsModule) Tools() []registry.ToolDefinition {
	echoTool := handler.TypedHandler[EchoInput, EchoOutput](
		"echo",
		"Echoes back the provided message. Used for basic tool call validation.",
		func(_ context.Context, input EchoInput) (EchoOutput, error) {
			return EchoOutput{Echo: input.Message}, nil
		},
	)

	addTool := handler.TypedHandler[AddInput, AddOutput](
		"add",
		"Adds two numbers together. Used for numeric argument validation.",
		func(_ context.Context, input AddInput) (AddOutput, error) {
			return AddOutput{Result: input.A + input.B}, nil
		},
	)

	// longRunningOperation: sends progress notifications, then returns.
	longRunningTool := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        "longRunningOperation",
			Description: "Simulates a long-running operation with progress notifications.",
			InputSchema: mcp.ToolInputSchema(mcp.ToolArgumentsSchema{
				Type: "object",
				Properties: map[string]any{
					"duration":  map[string]any{"type": "integer", "description": "Duration in seconds (default 10)", "default": 10},
					"steps":     map[string]any{"type": "integer", "description": "Number of progress steps (default 5)", "default": 5},
					"stepLabel": map[string]any{"type": "string", "description": "Label prefix for progress messages"},
				},
			}),
		},
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			duration := handler.GetIntParam(req, "duration", 10)
			steps := handler.GetIntParam(req, "steps", 5)
			label := handler.GetStringParam(req, "stepLabel")
			if label == "" {
				label = "Processing"
			}

			if steps < 1 {
				steps = 1
			}

			// Send progress notifications if a progress token was provided.
			if m.MCPServer != nil && req.Params.Meta.ProgressToken != nil {
				reporter := registry.NewServerProgressReporter(m.MCPServer, req.Params.Meta.ProgressToken, float64(steps))
				stepDelay := time.Duration(duration) * time.Second / time.Duration(steps)

				for i := 1; i <= steps; i++ {
					select {
					case <-ctx.Done():
						return handler.ErrorResult(ctx.Err()), nil
					case <-time.After(stepDelay):
					}
					msg := fmt.Sprintf("%s step %d/%d", label, i, steps)
					_ = reporter.Report(ctx, float64(i), msg)
				}
			}

			return handler.TextResult(fmt.Sprintf("Operation completed. Duration: %ds, Steps: %d", duration, steps)), nil
		},
	}

	// sampleLLM: requests sampling from the client, returns the result.
	sampleLLMTool := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        "sampleLLM",
			Description: "Requests an LLM sampling from the client. Tests server-initiated sampling.",
			InputSchema: mcp.ToolInputSchema(mcp.ToolArgumentsSchema{
				Type: "object",
				Properties: map[string]any{
					"prompt":   map[string]any{"type": "string", "description": "Prompt to send to the LLM"},
					"maxWords": map[string]any{"type": "integer", "description": "Maximum number of words"},
				},
				Required: []string{"prompt"},
			}),
		},
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			prompt := handler.GetStringParam(req, "prompt")
			maxTokens := handler.GetIntParam(req, "maxWords", 100)

			if m.MCPServer == nil {
				return handler.TextResult("sampleLLM: no server reference available for sampling"), nil
			}

			result, err := m.MCPServer.RequestSampling(ctx, mcp.CreateMessageRequest{
				Request: mcp.Request{Method: string(mcp.MethodSamplingCreateMessage)},
				CreateMessageParams: mcp.CreateMessageParams{
					Messages: []mcp.SamplingMessage{
						{
							Role:    mcp.RoleUser,
							Content: mcp.NewTextContent(prompt),
						},
					},
					MaxTokens: maxTokens,
				},
			})
			if err != nil {
				return handler.TextResult(fmt.Sprintf("sampleLLM sampling failed: %v", err)), nil
			}

			// Extract text from result
			if tc, ok := result.Content.(mcp.TextContent); ok {
				return handler.TextResult(fmt.Sprintf("LLM response: %s", tc.Text)), nil
			}
			return handler.TextResult("sampleLLM: received non-text response from sampling"), nil
		},
	}

	// getTinyImage: returns a 1x1 PNG image.
	getTinyImageTool := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        "getTinyImage",
			Description: "Returns a tiny 1x1 red PNG image. Tests image content in tool responses.",
			InputSchema: mcp.ToolInputSchema(mcp.ToolArgumentsSchema{
				Type:       "object",
				Properties: map[string]any{},
			}),
		},
		Handler: func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.ImageContent{
						Type:     "image",
						Data:     tinyImageBase64,
						MIMEType: "image/png",
					},
				},
			}, nil
		},
	}

	// annotatedMessage: returns content with annotations (audience, priority).
	annotatedMessageTool := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        "annotatedMessage",
			Description: "Returns a message with annotations (audience, priority). Tests content annotations.",
			InputSchema: mcp.ToolInputSchema(mcp.ToolArgumentsSchema{
				Type: "object",
				Properties: map[string]any{
					"messageType":  map[string]any{"type": "string", "description": "Type of message: error, success, or debug", "enum": []string{"error", "success", "debug"}},
					"includeImage": map[string]any{"type": "boolean", "description": "Whether to include an image"},
				},
				Required: []string{"messageType"},
			}),
		},
		Handler: func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			msgType := handler.GetStringParam(req, "messageType")
			includeImage := handler.GetBoolParam(req, "includeImage", false)

			var priority float64
			var audience []mcp.Role
			var text string
			var isError bool

			switch msgType {
			case "error":
				priority = 1.0
				audience = []mcp.Role{mcp.RoleUser, mcp.RoleAssistant}
				text = "Error: something went wrong"
				isError = true
			case "success":
				priority = 0.7
				audience = []mcp.Role{mcp.RoleUser}
				text = "Operation completed successfully"
			case "debug":
				priority = 0.1
				audience = []mcp.Role{mcp.RoleAssistant}
				text = "Debug: internal state details"
			default:
				priority = 0.5
				audience = []mcp.Role{mcp.RoleUser}
				text = "Unknown message type: " + msgType
			}

			var content []mcp.Content
			content = append(content, mcp.TextContent{
				Annotated: mcp.Annotated{
					Annotations: &mcp.Annotations{
						Audience: audience,
						Priority: &priority,
					},
				},
				Type: "text",
				Text: text,
			})

			if includeImage {
				imgPriority := 0.5
				content = append(content, mcp.ImageContent{
					Annotated: mcp.Annotated{
						Annotations: &mcp.Annotations{
							Audience: []mcp.Role{mcp.RoleUser},
							Priority: &imgPriority,
						},
					},
					Type:     "image",
					Data:     tinyImageBase64,
					MIMEType: "image/png",
				})
			}

			return &mcp.CallToolResult{
				Content: content,
				IsError: isError,
			}, nil
		},
	}

	// loggingTool: emits log messages, then returns. Tests logging during tool execution.
	loggingTool := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        "logMessage",
			Description: "Emits log messages at various levels during execution. Tests logging capability.",
			InputSchema: mcp.ToolInputSchema(mcp.ToolArgumentsSchema{
				Type: "object",
				Properties: map[string]any{
					"level":   map[string]any{"type": "string", "description": "Log level", "enum": []string{"debug", "info", "warning", "error"}},
					"message": map[string]any{"type": "string", "description": "Message to log"},
				},
				Required: []string{"message"},
			}),
		},
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			level := handler.GetStringParam(req, "level")
			if level == "" {
				level = "info"
			}
			message := handler.GetStringParam(req, "message")

			if m.MCPServer != nil {
				logLevel := mcp.LoggingLevel(level)
				_ = m.MCPServer.SendLogMessageToClient(ctx, mcp.LoggingMessageNotification{
					Notification: mcp.Notification{
						Method: "notifications/message",
					},
					Params: mcp.LoggingMessageNotificationParams{
						Level:  logLevel,
						Logger: "conformance-server",
						Data:   message,
					},
				})
			}

			return handler.TextResult(fmt.Sprintf("Logged [%s]: %s", level, message)), nil
		},
	}

	return []registry.ToolDefinition{
		echoTool,
		addTool,
		longRunningTool,
		sampleLLMTool,
		getTinyImageTool,
		annotatedMessageTool,
		loggingTool,
	}
}

// ---------------------------------------------------------------------------
// Resource module
// ---------------------------------------------------------------------------

// ResourcesModule implements all conformance test resources.
type ResourcesModule struct{}

// Name returns the module name.
func (m *ResourcesModule) Name() string { return "conformance-resources" }

// Description returns the module description.
func (m *ResourcesModule) Description() string {
	return "MCP conformance suite resources: static text, static binary, dynamic template"
}

// Resources returns conformance resource definitions.
func (m *ResourcesModule) Resources() []resources.ResourceDefinition {
	return []resources.ResourceDefinition{
		{
			Resource: mcp.NewResource(
				"test://static-text",
				"Static Text Resource",
				mcp.WithResourceDescription("A static text resource for conformance testing"),
				mcp.WithMIMEType("text/plain"),
			),
			Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.TextResourceContents{
						URI:      "test://static-text",
						MIMEType: "text/plain",
						Text:     "This is a static text resource for conformance testing.",
					},
				}, nil
			},
			Category: "conformance",
		},
		{
			Resource: mcp.NewResource(
				"test://static-binary",
				"Static Binary Resource",
				mcp.WithResourceDescription("A static binary resource (base64 PNG) for conformance testing"),
				mcp.WithMIMEType("image/png"),
			),
			Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.BlobResourceContents{
						URI:      "test://static-binary",
						MIMEType: "image/png",
						Blob:     tinyImageBase64,
					},
				}, nil
			},
			Category: "conformance",
		},
	}
}

// Templates returns conformance resource template definitions.
func (m *ResourcesModule) Templates() []resources.TemplateDefinition {
	return []resources.TemplateDefinition{
		{
			Template: mcp.NewResourceTemplate(
				"test://dynamic/{name}",
				"Dynamic Resource",
				mcp.WithTemplateDescription("A dynamic text resource that echoes the URI parameter"),
				mcp.WithTemplateMIMEType("text/plain"),
			),
			Handler: func(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.TextResourceContents{
						URI:      req.Params.URI,
						MIMEType: "text/plain",
						Text:     fmt.Sprintf("Dynamic resource content for URI: %s", req.Params.URI),
					},
				}, nil
			},
			Category: "conformance",
		},
	}
}

// ---------------------------------------------------------------------------
// Prompt module
// ---------------------------------------------------------------------------

// PromptsModule implements all conformance test prompts.
type PromptsModule struct{}

// Name returns the module name.
func (m *PromptsModule) Name() string { return "conformance-prompts" }

// Description returns the module description.
func (m *PromptsModule) Description() string {
	return "MCP conformance suite prompts: simple, complex with args, embedded resource, with image"
}

// Prompts returns conformance prompt definitions.
func (m *PromptsModule) Prompts() []prompts.PromptDefinition {
	return []prompts.PromptDefinition{
		{
			Prompt: mcp.NewPrompt("simple_prompt",
				mcp.WithPromptDescription("A simple prompt with no arguments"),
			),
			Handler: func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return &mcp.GetPromptResult{
					Description: "A simple prompt",
					Messages: []mcp.PromptMessage{
						mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("This is a simple prompt with no arguments.")),
					},
				}, nil
			},
			Category: "conformance",
		},
		{
			Prompt: mcp.NewPrompt("complex_prompt",
				mcp.WithPromptDescription("A complex prompt with arguments"),
				mcp.WithArgument("name", mcp.RequiredArgument(), mcp.ArgumentDescription("The user's name")),
				mcp.WithArgument("style", mcp.ArgumentDescription("Response style: formal or casual (default: formal)")),
			),
			Handler: func(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				name := req.Params.Arguments["name"]
				style := req.Params.Arguments["style"]
				if style == "" {
					style = "formal"
				}
				return &mcp.GetPromptResult{
					Description: fmt.Sprintf("Complex prompt for %s (%s style)", name, style),
					Messages: []mcp.PromptMessage{
						mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(
							fmt.Sprintf("Please greet %s in a %s style.", name, style),
						)),
					},
				}, nil
			},
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
	}
}

// ---------------------------------------------------------------------------
// Completion providers
// ---------------------------------------------------------------------------

// PromptCompletionProvider provides argument completions for conformance prompts.
type PromptCompletionProvider struct{}

// CompletePromptArgument returns completions for prompt arguments.
func (p *PromptCompletionProvider) CompletePromptArgument(_ context.Context, promptName string, argument mcp.CompleteArgument, _ mcp.CompleteContext) (*mcp.Completion, error) {
	switch promptName {
	case "complex_prompt":
		switch argument.Name {
		case "style":
			options := []string{"formal", "casual", "technical", "friendly"}
			return filterCompletions(options, argument.Value), nil
		case "name":
			options := []string{"Alice", "Bob", "Charlie"}
			return filterCompletions(options, argument.Value), nil
		}
	}
	return &mcp.Completion{Values: []string{}}, nil
}

// ResourceCompletionProvider provides argument completions for conformance resource templates.
type ResourceCompletionProvider struct{}

// CompleteResourceArgument returns completions for resource template arguments.
func (p *ResourceCompletionProvider) CompleteResourceArgument(_ context.Context, uri string, argument mcp.CompleteArgument, _ mcp.CompleteContext) (*mcp.Completion, error) {
	if uri == "test://dynamic/{name}" && argument.Name == "name" {
		options := []string{"example", "test", "demo", "sample"}
		return filterCompletions(options, argument.Value), nil
	}
	return &mcp.Completion{Values: []string{}}, nil
}

// filterCompletions filters completion options by a prefix value.
func filterCompletions(options []string, prefix string) *mcp.Completion {
	if prefix == "" {
		return &mcp.Completion{
			Values:  options,
			Total:   len(options),
			HasMore: false,
		}
	}
	var matches []string
	for _, opt := range options {
		if len(opt) >= len(prefix) && opt[:len(prefix)] == prefix {
			matches = append(matches, opt)
		}
	}
	if matches == nil {
		matches = []string{}
	}
	return &mcp.Completion{
		Values:  matches,
		Total:   len(matches),
		HasMore: false,
	}
}

// ---------------------------------------------------------------------------
// Server builder
// ---------------------------------------------------------------------------

// ServerConfig holds configuration for the everything-server.
type ServerConfig struct {
	Name    string
	Version string
}

// DefaultConfig returns the default server configuration.
func DefaultConfig() ServerConfig {
	return ServerConfig{
		Name:    "mcpkit-everything-server",
		Version: "0.1.0",
	}
}

// NewEverythingServer creates a fully-configured MCP server implementing all
// testable capabilities for the MCP conformance suite.
func NewEverythingServer(cfg ServerConfig) *server.MCPServer {
	s := server.NewMCPServer(cfg.Name, cfg.Version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithLogging(),
		server.WithCompletions(),
		server.WithPromptCompletionProvider(&PromptCompletionProvider{}),
		server.WithResourceCompletionProvider(&ResourceCompletionProvider{}),
		server.WithRecovery(),
	)

	// Register tools
	toolReg := registry.NewToolRegistry()
	toolReg.RegisterModule(&ToolsModule{MCPServer: s})
	toolReg.RegisterWithServer(s)

	// Register resources
	resReg := resources.NewResourceRegistry()
	resReg.RegisterModule(&ResourcesModule{})
	resReg.RegisterWithServer(s)

	// Register prompts
	promptReg := prompts.NewPromptRegistry()
	promptReg.RegisterModule(&PromptsModule{})
	promptReg.RegisterWithServer(s)

	return s
}

