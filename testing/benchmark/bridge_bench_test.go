package benchmark

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"

	"github.com/hairglasses-studio/mcpkit/bridge/a2a"
	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// benchmarkToolDef builds a realistic ToolDefinition with 10 schema properties
// for benchmarking translation overhead.
func benchmarkToolDef() registry.ToolDefinition {
	return registry.ToolDefinition{
		Tool: mcp.NewTool("benchmark_tool",
			mcp.WithDescription("A tool for benchmarking translation overhead"),
			mcp.WithString("name", mcp.Required(), mcp.Description("Resource name")),
			mcp.WithString("namespace", mcp.Description("Kubernetes namespace")),
			mcp.WithString("format", mcp.Description("Output format (json, yaml, table)")),
			mcp.WithString("filter", mcp.Description("Filter expression")),
			mcp.WithString("sort_by", mcp.Description("Sort field")),
			mcp.WithString("output", mcp.Description("Output file path")),
			mcp.WithBoolean("verbose", mcp.Description("Enable verbose output")),
			mcp.WithBoolean("recursive", mcp.Description("Recurse into subdirectories")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results")),
			mcp.WithNumber("timeout", mcp.Description("Timeout in seconds")),
		),
		Category: "system",
		Tags:     []string{"kubernetes", "monitoring", "ops"},
		IsWrite:  false,
	}
}

// benchmarkTextResult builds a CallToolResult with text content.
func benchmarkTextResult() *registry.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: "systemd unit nginx.service is active (running) since 2026-01-15T08:30:00Z, PID 1234, memory 45.2MB",
			},
		},
	}
}

// benchmarkImageResult builds a CallToolResult with base64 image content.
func benchmarkImageResult() *registry.CallToolResult {
	// 1KB of pseudo-image data, base64-encoded.
	rawImage := make([]byte, 1024)
	for i := range rawImage {
		rawImage[i] = byte(i % 256)
	}
	encoded := base64.StdEncoding.EncodeToString(rawImage)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.ImageContent{
				Type:     "image",
				Data:     encoded,
				MIMEType: "image/png",
			},
		},
	}
}

// benchmarkDataPartMessage builds an A2A Message with a DataPart containing
// structured tool call arguments.
func benchmarkDataPartMessage() a2atypes.Message {
	return *a2atypes.NewMessage(
		a2atypes.MessageRoleUser,
		a2atypes.NewDataPart(map[string]any{
			"skill": "benchmark_tool",
			"arguments": map[string]any{
				"name":      "nginx",
				"namespace": "production",
				"format":    "json",
				"verbose":   true,
				"limit":     100,
			},
		}),
	)
}

// benchmarkSkill builds a minimal A2A AgentSkill for message parsing benchmarks.
func benchmarkSkill() a2atypes.AgentSkill {
	return a2atypes.AgentSkill{
		ID:   "benchmark_tool",
		Name: "benchmark_tool",
	}
}

// BenchmarkTranslator_ToolToSkill measures the cost of converting an mcpkit
// ToolDefinition (with 10 schema properties) to an A2A AgentSkill.
func BenchmarkTranslator_ToolToSkill(b *testing.B) {
	b.ReportAllocs()

	tr := &a2a.Translator{}
	td := benchmarkToolDef()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tr.ToolToSkill(td)
	}
}

// BenchmarkTranslator_CallResultToArtifact_Text measures text content translation.
func BenchmarkTranslator_CallResultToArtifact_Text(b *testing.B) {
	b.ReportAllocs()

	tr := &a2a.Translator{}
	result := benchmarkTextResult()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tr.CallResultToArtifact(result)
	}
}

// BenchmarkTranslator_CallResultToArtifact_Image measures image content
// translation including base64 decode.
func BenchmarkTranslator_CallResultToArtifact_Image(b *testing.B) {
	b.ReportAllocs()

	tr := &a2a.Translator{}
	result := benchmarkImageResult()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tr.CallResultToArtifact(result)
	}
}

// BenchmarkTranslator_MessageToCallToolRequest measures A2A message parsing
// into an MCP tool call (DataPart strategy with structured arguments).
func BenchmarkTranslator_MessageToCallToolRequest(b *testing.B) {
	b.ReportAllocs()

	tr := &a2a.Translator{}
	msg := benchmarkDataPartMessage()
	skill := benchmarkSkill()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = tr.MessageToCallToolRequest(msg, skill)
	}
}

// BenchmarkTranslator_ErrorToTaskStatus measures error code translation
// from MCP error codes to A2A task status.
func BenchmarkTranslator_ErrorToTaskStatus(b *testing.B) {
	b.ReportAllocs()

	tr := &a2a.Translator{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tr.ErrorToTaskStatus(handler.ErrPermission, "access denied to tool systemd_restart")
	}
}

// BenchmarkAgentCardGenerator_Generate measures card generation with a
// small registry (5 tools), the common case for single-domain MCP servers.
func BenchmarkAgentCardGenerator_Generate(b *testing.B) {
	b.ReportAllocs()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&benchModule{
		name:      "bench",
		toolCount: 5,
	})

	gen := a2a.NewAgentCardGenerator(reg, nil, a2a.CardConfig{
		Name:        "bench-agent",
		Description: "Benchmark agent",
		URL:         "http://localhost:8080",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Invalidate()
		_ = gen.Generate()
	}
}

// BenchmarkAgentCardGenerator_Generate_100Tools measures card generation with
// 100 tools, simulating a large MCP server like dotfiles-mcp.
func BenchmarkAgentCardGenerator_Generate_100Tools(b *testing.B) {
	b.ReportAllocs()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&benchModule{
		name:      "bench",
		toolCount: 100,
	})

	gen := a2a.NewAgentCardGenerator(reg, nil, a2a.CardConfig{
		Name:        "bench-agent",
		Description: "Benchmark agent with 100 tools",
		URL:         "http://localhost:8080",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Invalidate()
		_ = gen.Generate()
	}
}

// benchModule implements registry.ToolModule for benchmark setup.
type benchModule struct {
	name      string
	toolCount int
}

func (m *benchModule) Name() string        { return m.name }
func (m *benchModule) Description() string { return "benchmark module" }

func (m *benchModule) Tools() []registry.ToolDefinition {
	tools := make([]registry.ToolDefinition, m.toolCount)
	for i := range tools {
		tools[i] = registry.ToolDefinition{
			Tool: mcp.NewTool(
				fmt.Sprintf("bench_tool_%03d", i),
				mcp.WithDescription(fmt.Sprintf("Benchmark tool %d for performance testing", i)),
				mcp.WithString("input", mcp.Required(), mcp.Description("Primary input")),
				mcp.WithString("format", mcp.Description("Output format")),
				mcp.WithBoolean("verbose", mcp.Description("Verbose output")),
			),
			Category: "benchmark",
			Tags:     []string{"bench", "test"},
			IsWrite:  i%3 == 0, // Every third tool is a write tool.
		}
	}
	return tools
}
