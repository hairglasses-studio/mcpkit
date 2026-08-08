// portable_tools.go — conformance tools that are genuinely SDK-neutral.
//
// echo/add only touch handler.TypedHandler and registry's compat aliases
// (no raw mcp-go or official-SDK types), matching the pattern established in
// d6bb0be (boundedwrite/truncate): this file carries no build tag and
// PortableTools() compiles and works unchanged under both official_sdk and
// mcp-go. everything_server.go's ToolsModule.Tools() (mcp-go, the full
// everything-server) and portable_server.go's PortableToolsModule (both
// tags, the reduced conformance subset that also runs under official_sdk)
// both source echo/add from here, so the two behave identically instead of
// drifting.
package conformance

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// tinyImageBase64 is a tiny 1x1 red PNG (base64), used by getTinyImage and
// prompts-get-with-image across everything_server.go and the portable_*.go
// pair.
const tinyImageBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="

// tinyWAVBase64 is a minimal WAV file (44 bytes header + 1 sample) for audio conformance.
const tinyWAVBase64 = "UklGRiYAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQIAAAAAAA=="

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

// extractTemplateID extracts the {id} value from test://template/{id}/data.
// Pure string parsing, no SDK dependency — shared by everything_server.go's
// ResourcesModule (mcp-go) and both halves of PortableResourcesModule
// (portable_resources.go / portable_resources_official.go).
func extractTemplateID(uri string) string {
	// URI format: test://template/123/data
	const prefix = "test://template/"
	const suffix = "/data"
	if len(uri) > len(prefix)+len(suffix) {
		inner := uri[len(prefix):]
		if idx := len(inner) - len(suffix); idx > 0 {
			return inner[:idx]
		}
	}
	return "unknown"
}

// ServerConfig holds configuration for the everything-server (both
// NewEverythingServer and NewPortableEverythingServer).
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

// PortableToolsModule wraps PortableTools() as a registry.ToolModule for
// NewPortableEverythingServer.
type PortableToolsModule struct{}

// Name returns the module name.
func (m *PortableToolsModule) Name() string { return "conformance-portable-tools" }

// Description returns the module description.
func (m *PortableToolsModule) Description() string {
	return "MCP conformance suite tools (portable subset): echo, add"
}

// Tools returns the portable conformance tool definitions.
func (m *PortableToolsModule) Tools() []registry.ToolDefinition { return PortableTools() }

// PortableTools returns the conformance tools with zero SDK-specific
// dependencies: echo (basic tool call validation) and add (numeric argument
// validation).
func PortableTools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		handler.TypedHandler[EchoInput, EchoOutput](
			"echo",
			"Echoes back the provided message. Used for basic tool call validation.",
			func(_ context.Context, input EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: input.Message}, nil
			},
		),
		handler.TypedHandler[AddInput, AddOutput](
			"add",
			"Adds two numbers together. Used for numeric argument validation.",
			func(_ context.Context, input AddInput) (AddOutput, error) {
				return AddOutput{Result: input.A + input.B}, nil
			},
		),
	}
}
