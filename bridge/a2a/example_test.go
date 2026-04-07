package a2a_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"

	bridgea2a "github.com/hairglasses-studio/mcpkit/bridge/a2a"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- example tool modules ---

type exampleModule struct {
	name  string
	desc  string
	tools []registry.ToolDefinition
}

func (m *exampleModule) Name() string                     { return m.name }
func (m *exampleModule) Description() string              { return m.desc }
func (m *exampleModule) Tools() []registry.ToolDefinition { return m.tools }

// --- examples ---

// ExampleBridge demonstrates using the high-level Bridge to expose MCP tools
// as an A2A agent. The bridge creates the executor, agent card generator,
// and HTTP handler internally.
func ExampleBridge() {
	// 1. Register MCP tools.
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&exampleModule{
		name: "demo",
		desc: "Demo tools",
		tools: []registry.ToolDefinition{
			{
				Tool: mcp.NewTool("greet",
					mcp.WithDescription("Say hello to someone"),
					mcp.WithString("name", mcp.Description("Who to greet")),
				),
				Handler: func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
					args := registry.ExtractArguments(req)
					name, _ := args["name"].(string)
					if name == "" {
						name = "world"
					}
					return registry.MakeTextResult("hello " + name), nil
				},
			},
		},
	})

	// 2. Create the bridge.
	bridge, err := bridgea2a.NewBridge(reg, bridgea2a.BridgeConfig{
		Name:        "example-agent",
		Description: "An example A2A agent",
		Version:     "1.0.0",
		URL:         "http://localhost:8080",
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// 3. Use Handler() for embedding in your own server.
	ts := httptest.NewServer(bridge.Handler())
	defer ts.Close()

	// 4. Read the agent card.
	card := bridge.AgentCard()
	fmt.Println("Agent:", card.Name)
	fmt.Println("Skills:", len(card.Skills))
	if len(card.Skills) > 0 {
		fmt.Println("First skill:", card.Skills[0].ID)
	}
	// Output:
	// Agent: example-agent
	// Skills: 1
	// First skill: greet
}

// ExampleNewBridge_withMiddleware demonstrates adding middleware to the A2A
// bridge. Bridge-level middleware runs on every tool invocation that arrives
// via the A2A protocol, enabling logging, auth, and rate limiting.
func ExampleNewBridge_withMiddleware() {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&exampleModule{
		name: "tools",
		tools: []registry.ToolDefinition{
			{
				Tool: mcp.NewTool("ping",
					mcp.WithDescription("Ping-pong"),
				),
				Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
					return registry.MakeTextResult("pong"), nil
				},
			},
		},
	})

	// Create a logging middleware.
	loggingMW := func(name string, _ registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			fmt.Printf("A2A bridge: invoking tool %q\n", name)
			return next(ctx, req)
		}
	}

	bridge, err := bridgea2a.NewBridge(reg, bridgea2a.BridgeConfig{
		Name:       "mw-agent",
		Middleware: []registry.Middleware{loggingMW},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// The bridge handler can be tested directly with httptest.
	ts := httptest.NewServer(bridge.Handler())
	defer ts.Close()

	// Send a JSON-RPC SendMessage to invoke the tool.
	msg := a2atypes.NewMessage(
		a2atypes.MessageRoleUser,
		a2atypes.NewDataPart(map[string]any{
			"skill":     "ping",
			"arguments": map[string]any{},
		}),
	)

	jsonrpcReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "SendMessage",
		"params":  map[string]any{"message": msg},
	}
	body, _ := json.Marshal(jsonrpcReq)
	resp, err := http.Post(ts.URL+"/", "application/json", strings.NewReader(string(body)))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Status:", resp.StatusCode)
	// Output:
	// A2A bridge: invoking tool "ping"
	// Status: 200
}

// ExampleNewBridgeExecutor demonstrates using the lower-level BridgeExecutor
// directly, which is useful when you want full control over the HTTP server
// and agent card configuration.
func ExampleNewBridgeExecutor() {
	// 1. Register tools.
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&exampleModule{
		name: "math",
		tools: []registry.ToolDefinition{
			{
				Tool: mcp.NewTool("add",
					mcp.WithDescription("Add two numbers"),
					mcp.WithNumber("a", mcp.Description("First number")),
					mcp.WithNumber("b", mcp.Description("Second number")),
				),
				Category: "math",
				Handler: func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
					args := registry.ExtractArguments(req)
					a, _ := args["a"].(float64)
					b, _ := args["b"].(float64)
					return registry.MakeTextResult(fmt.Sprintf("%.0f", a+b)), nil
				},
			},
		},
	})

	// 2. Create executor and agent card generator separately.
	executor := bridgea2a.NewBridgeExecutor(reg, bridgea2a.ExecutorConfig{
		Logger:      slog.Default(),
		TaskTimeout: 10 * time.Second,
	})

	cardGen := bridgea2a.NewAgentCardGenerator(reg, nil, bridgea2a.CardConfig{
		Name:        "math-agent",
		Description: "A calculator agent",
		Version:     "1.0.0",
		URL:         "http://localhost:9090",
	})
	card := cardGen.Generate()

	// 3. Wire up the HTTP handler using the official A2A SDK.
	a2aHandler := a2asrv.NewHandler(executor)
	mux := http.NewServeMux()
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(card))
	mux.Handle("/", a2asrv.NewJSONRPCHandler(a2aHandler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 4. Verify the agent card.
	resp, err := http.Get(ts.URL + a2asrv.WellKnownAgentCardPath)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer resp.Body.Close()
	var fetchedCard a2atypes.AgentCard
	_ = json.NewDecoder(resp.Body).Decode(&fetchedCard)

	fmt.Println("Agent:", fetchedCard.Name)
	fmt.Println("Skills:", len(fetchedCard.Skills))
	// Output:
	// Agent: math-agent
	// Skills: 1
}

// ExampleTranslator demonstrates the Translator converting between MCP and
// A2A protocol types. The translator is zero-cost and deterministic -- no
// LLM involvement.
func ExampleTranslator() {
	tr := &bridgea2a.Translator{
		SkillTags: []string{"mcpkit"},
	}

	// Convert an MCP tool definition to an A2A skill.
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("systemd_status",
			mcp.WithDescription("Get the status of a systemd unit"),
			mcp.WithString("unit", mcp.Required(), mcp.Description("Unit name")),
		),
		Category: "system",
		Tags:     []string{"systemd"},
		IsWrite:  false,
	}

	skill := tr.ToolToSkill(td)

	fmt.Println("Skill ID:", skill.ID)
	fmt.Println("Skill Description:", skill.Description)
	fmt.Println("Tags:", skill.Tags)
	// Output:
	// Skill ID: systemd_status
	// Skill Description: Get the status of a systemd unit
	// Tags: [system systemd read mcpkit]
}

// ExampleNewRemoteAgent demonstrates wrapping a remote A2A agent as MCP
// tools. Each skill in the agent's card becomes an MCP tool that can be
// registered on a local registry.
func ExampleNewRemoteAgent() {
	// Set up a mock A2A agent that has a "summarize" skill.
	mockHandler := http.NewServeMux()

	card := &a2atypes.AgentCard{
		Name:               "research-agent",
		Description:        "A research agent with summarization skills",
		Version:            "1.0.0",
		DefaultInputModes:  []string{"application/json"},
		DefaultOutputModes: []string{"text/plain"},
		Skills: []a2atypes.AgentSkill{
			{
				ID:          "summarize",
				Name:        "Summarize",
				Description: "Summarize text input",
				Tags:        []string{"nlp"},
			},
		},
	}

	// Serve the agent card at the well-known endpoint.
	mockHandler.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(card)
	})

	// Handle REST SendMessage requests.
	mockHandler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Read the request body to determine what's being sent.
		body, _ := io.ReadAll(r.Body)

		// Build a completed task response.
		task := &a2atypes.Task{
			ID: a2atypes.NewTaskID(),
			Status: a2atypes.TaskStatus{
				State: a2atypes.TaskStateCompleted,
			},
			Artifacts: []*a2atypes.Artifact{
				{
					ID: a2atypes.NewArtifactID(),
					Parts: a2atypes.ContentParts{
						a2atypes.NewTextPart("Summary: the document discusses Go and A2A integration."),
					},
				},
			},
		}

		// Wrap in StreamResponse envelope.
		sr := a2atypes.StreamResponse{Event: task}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sr)
		_ = body
	})

	ts := httptest.NewServer(mockHandler)
	defer ts.Close()

	// Set the URL in the card now that we know the server address.
	card.SupportedInterfaces = []*a2atypes.AgentInterface{
		a2atypes.NewAgentInterface(ts.URL, a2atypes.TransportProtocolHTTPJSON),
	}

	// Discover and wrap the remote agent.
	ctx := context.Background()
	remote, err := bridgea2a.NewRemoteAgent(ctx, ts.URL)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer func() { _ = remote.Close() }()

	fmt.Println("Remote agent:", remote.Name())
	fmt.Println("Tools:", len(remote.Tools()))

	// Register with a local MCP registry.
	reg := registry.NewToolRegistry()
	reg.RegisterModule(remote)

	// The tool is now available via the local registry.
	td, ok := reg.GetTool("summarize")
	if ok {
		fmt.Println("Registered tool:", td.Tool.Name)
	}
	// Output:
	// Remote agent: research-agent
	// Tools: 1
	// Registered tool: summarize
}

// ExampleAuthMiddleware demonstrates adding authentication middleware to the
// A2A bridge. The middleware extracts Bearer tokens from the incoming context
// and makes them available to downstream tool handlers.
func ExampleAuthMiddleware() {
	authMW := bridgea2a.AuthMiddleware(bridgea2a.AuthConfig{
		Required: true,
	})

	// Wrap a simple handler.
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("secret_tool", mcp.WithDescription("Requires auth")),
	}
	inner := func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		token := bridgea2a.TokenFromContext(ctx)
		return registry.MakeTextResult("authenticated with: " + token), nil
	}

	wrapped := authMW("secret_tool", td, inner)

	// Without auth: returns error result.
	result, _ := wrapped(context.Background(), registry.CallToolRequest{})
	if registry.IsResultError(result) {
		fmt.Println("No auth: rejected")
	}

	// With auth: succeeds.
	ctx := bridgea2a.WithAuthHeader(context.Background(), "Authorization", "Bearer my-token")
	result, _ = wrapped(ctx, registry.CallToolRequest{})
	if !registry.IsResultError(result) {
		text, _ := registry.ExtractTextContent(result.Content[0])
		fmt.Println("With auth:", text)
	}
	// Output:
	// No auth: rejected
	// With auth: authenticated with: my-token
}
