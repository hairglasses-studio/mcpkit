package a2a_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/hairglasses-studio/mcpkit/a2a"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ExampleNewServer_basic demonstrates creating a minimal A2A server backed
// by an MCP tool registry. The server receives A2A tasks over JSON-RPC and
// dispatches them to the matching MCP tool.
func ExampleNewServer_basic() {
	// 1. Create the MCP tool registry.
	reg := registry.NewToolRegistry()

	// 2. Generate an agent card from the registry.
	card := a2a.AgentCardFromRegistry(reg,
		a2a.WithName("demo-agent"),
		a2a.WithDescription("A minimal A2A agent powered by mcpkit"),
		a2a.WithURL("http://localhost:8080"),
	)

	// 3. Create the A2A server.
	srv := a2a.NewServer(reg, card)

	// 4. Mount the handler (well-known agent card + JSON-RPC endpoint).
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 5. Discover the agent card.
	resp, err := http.Get(ts.URL + "/.well-known/agent.json")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer resp.Body.Close()

	var discovered a2a.AgentCard
	json.NewDecoder(resp.Body).Decode(&discovered)

	fmt.Println("Agent:", discovered.Name)
	fmt.Println("Version:", discovered.Version)
	// Output:
	// Agent: demo-agent
	// Version: 1.0.0
}

// ExampleNewClient demonstrates using the A2A client to send a task to an
// A2A server and read the response.
func ExampleNewClient() {
	// Set up a test server that returns a completed task.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent.json" {
			json.NewEncoder(w).Encode(a2a.AgentCard{
				Name:    "echo-agent",
				Version: "1.0.0",
			})
			return
		}
		// Parse JSON-RPC and return a completed task.
		var req a2a.JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		task := a2a.Task{
			ID:    "task-1",
			State: a2a.TaskCompleted,
			Messages: []a2a.Message{
				{Role: "agent", Parts: []a2a.Part{a2a.TextPart("hello from agent")}},
			},
		}
		taskJSON, _ := json.Marshal(task)
		resp := a2a.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  taskJSON,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// Create a client and send a task.
	client := a2a.NewClient(ts.URL)
	ctx := context.Background()

	// Discover the agent.
	card, _ := client.GetAgentCard(ctx)
	fmt.Println("Connected to:", card.Name)

	// Send a task.
	task, err := client.SendTask(ctx, a2a.TaskSendParams{
		ID:       "task-1",
		Messages: []a2a.Message{{Role: "user", Parts: []a2a.Part{a2a.TextPart("hello")}}},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("Task state:", task.State)
	fmt.Println("Response:", task.Messages[0].Parts[0].Text)
	// Output:
	// Connected to: echo-agent
	// Task state: completed
	// Response: hello from agent
}

// ExampleNewClient_withAuth demonstrates using the A2A client with Bearer
// token authentication.
func ExampleNewClient_withAuth() {
	// Server that verifies the auth header.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-secret-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(a2a.AgentCard{Name: "secure-agent"})
	}))
	defer ts.Close()

	client := a2a.NewClient(ts.URL, a2a.WithAuthToken("my-secret-token"))
	card, err := client.GetAgentCard(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("Authenticated as:", card.Name)
	// Output:
	// Authenticated as: secure-agent
}

// ExampleAgentCardFromRegistry demonstrates generating an A2A agent card
// from an MCP tool registry with custom options.
func ExampleAgentCardFromRegistry() {
	reg := registry.NewToolRegistry()

	card := a2a.AgentCardFromRegistry(reg,
		a2a.WithName("production-agent"),
		a2a.WithDescription("Production MCP server exposed via A2A"),
		a2a.WithVersion("2.0.0"),
		a2a.WithURL("https://agent.example.com"),
		a2a.WithOrganization("ACME Corp", "https://acme.example.com"),
		a2a.WithStreaming(),
		a2a.WithAuth("bearer", "oauth2"),
	)

	fmt.Println("Name:", card.Name)
	fmt.Println("Version:", card.Version)
	fmt.Println("Provider:", card.Provider.Organization)
	fmt.Println("Streaming:", card.Capabilities.Streaming)
	fmt.Println("Auth schemes:", card.Auth.Schemes)
	// Output:
	// Name: production-agent
	// Version: 2.0.0
	// Provider: ACME Corp
	// Streaming: true
	// Auth schemes: [bearer oauth2]
}

// ExampleToSDKAgentCard demonstrates converting between mcpkit and official
// A2A SDK agent card types.
func ExampleToSDKAgentCard() {
	// mcpkit native card.
	card := a2a.AgentCard{
		Name:    "my-agent",
		URL:     "https://agent.example.com",
		Version: "1.0.0",
		Capabilities: &a2a.Capabilities{Streaming: true},
		Skills: []a2a.Skill{
			{ID: "search", Name: "search", Description: "Search documents"},
		},
	}

	// Convert to official SDK type.
	sdkCard := a2a.ToSDKAgentCard(card)
	fmt.Println("SDK Name:", sdkCard.Name)
	fmt.Println("SDK Skills:", len(sdkCard.Skills))

	// Convert back to mcpkit type.
	roundTripped := a2a.FromSDKAgentCard(sdkCard)
	fmt.Println("Roundtrip Name:", roundTripped.Name)
	fmt.Println("Roundtrip Skills:", len(roundTripped.Skills))
	// Output:
	// SDK Name: my-agent
	// SDK Skills: 1
	// Roundtrip Name: my-agent
	// Roundtrip Skills: 1
}

// ExampleRateLimitInterceptor demonstrates per-agent rate limiting for A2A
// client requests.
func ExampleRateLimitInterceptor() {
	limiter := a2a.NewRateLimitInterceptor(a2a.RateLimitConfig{
		DefaultRate:  2.0, // 2 requests per second
		DefaultBurst: 2,   // burst of 2
	})

	// First two requests succeed (burst).
	err1 := limiter.Allow("http://agent.example.com")
	err2 := limiter.Allow("http://agent.example.com")
	// Third request exceeds the burst.
	err3 := limiter.Allow("http://agent.example.com")

	fmt.Println("Request 1:", err1)
	fmt.Println("Request 2:", err2)
	fmt.Println("Request 3 exceeded:", err3 != nil)
	// Output:
	// Request 1: <nil>
	// Request 2: <nil>
	// Request 3 exceeded: true
}
