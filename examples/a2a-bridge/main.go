// Command a2a-bridge demonstrates exposing mcpkit MCP tools as an A2A agent.
//
// It registers two simple MCP tools (calculator and greeter), creates a
// bridge/a2a BridgeExecutor, and serves them over HTTP as an A2A agent.
// Remote A2A clients can discover skills via the agent card and invoke
// tools via the standard A2A task protocol.
//
// Run:
//
//	go run ./examples/a2a-bridge/
//
// Then:
//
//	curl http://localhost:8080/.well-known/agent-card.json   # discover skills
//	curl -X POST http://localhost:8080/ -d '...'             # send A2A task (JSON-RPC)
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"

	"github.com/hairglasses-studio/mcpkit/bridge/a2a"
	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ---------------------------------------------------------------------------
// Tool types
// ---------------------------------------------------------------------------

// AddInput is the input schema for the add tool.
type AddInput struct {
	A float64 `json:"a" jsonschema:"required,description=First operand"`
	B float64 `json:"b" jsonschema:"required,description=Second operand"`
}

// AddOutput is the output schema for the add tool.
type AddOutput struct {
	Result float64 `json:"result"`
}

// GreetInput is the input schema for the greet tool.
type GreetInput struct {
	Name string `json:"name" jsonschema:"required,description=Name to greet"`
}

// GreetOutput is the output schema for the greet tool.
type GreetOutput struct {
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// Tool module
// ---------------------------------------------------------------------------

// DemoModule provides example tools exposed over the A2A bridge.
type DemoModule struct{}

func (m *DemoModule) Name() string        { return "demo" }
func (m *DemoModule) Description() string { return "Demo tools for the A2A bridge example" }

func (m *DemoModule) Tools() []registry.ToolDefinition {
	addTool := handler.TypedHandler[AddInput, AddOutput](
		"add",
		"Add two numbers together and return the sum.",
		func(_ context.Context, input AddInput) (AddOutput, error) {
			return AddOutput{Result: input.A + input.B}, nil
		},
	)
	addTool.Category = "math"
	addTool.Tags = []string{"calculator", "arithmetic"}

	greetTool := handler.TypedHandler[GreetInput, GreetOutput](
		"greet",
		"Greet a user by name.",
		func(_ context.Context, input GreetInput) (GreetOutput, error) {
			if input.Name == "" {
				return GreetOutput{}, fmt.Errorf("name must not be empty")
			}
			return GreetOutput{Message: fmt.Sprintf("Hello, %s!", input.Name)}, nil
		},
	)
	greetTool.Category = "util"
	greetTool.Tags = []string{"greeting"}

	return []registry.ToolDefinition{addTool, greetTool}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	logger := slog.Default()

	// 1. Create the MCP tool registry and register tools.
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&DemoModule{})

	// 2. Create the A2A bridge executor (translates A2A tasks to MCP tool calls).
	executor := a2a.NewBridgeExecutor(reg, a2a.ExecutorConfig{
		Logger: logger,
	})

	// 3. Generate the A2A agent card from registered tools.
	cardGen := a2a.NewAgentCardGenerator(reg, nil, a2a.CardConfig{
		Name:        "mcpkit-demo",
		Description: "A demo A2A agent powered by mcpkit MCP tools",
		Version:     "1.0.0",
		URL:         "http://localhost:8080",
	})
	card := cardGen.Generate()

	// 4. Create the A2A request handler and HTTP transport.
	a2aHandler := a2asrv.NewHandler(executor)

	mux := http.NewServeMux()
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(card))
	mux.Handle("/", a2asrv.NewJSONRPCHandler(a2aHandler))

	// 5. Print startup info and serve.
	log.Printf("A2A bridge listening on :8080")
	log.Printf("  AgentCard:  http://localhost:8080%s", a2asrv.WellKnownAgentCardPath)
	log.Printf("  Skills:     %d (%s)", len(card.Skills), formatSkillNames(card.Skills))
	log.Printf("  JSON-RPC:   http://localhost:8080/")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

// formatSkillNames returns a comma-separated list of skill IDs.
func formatSkillNames(skills []a2atypes.AgentSkill) string {
	var names string
	for i, s := range skills {
		if i > 0 {
			names += ", "
		}
		names += s.ID
	}
	return names
}
