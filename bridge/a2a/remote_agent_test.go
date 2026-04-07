package a2a

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- mock A2A agent infrastructure ---

// mockA2AAgent is an httptest.Server that implements a minimal A2A agent.
// It serves the agent card at the well-known endpoint and handles
// SendMessage via the A2A REST handler.
type mockA2AAgent struct {
	Server *httptest.Server
	Card   *a2atypes.AgentCard
}

// mockExecutor implements a2asrv.AgentExecutor with a configurable handler.
type mockExecutor struct {
	handler func(ctx context.Context, execCtx *a2asrv.ExecutorContext) (a2atypes.TaskState, []*a2atypes.Part, string)
}

func (m *mockExecutor) Execute(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2atypes.Event, error] {
	return func(yield func(a2atypes.Event, error) bool) {
		taskInfo := execCtx.TaskInfo()

		// Emit submitted task.
		if execCtx.StoredTask == nil {
			submitted := a2atypes.NewSubmittedTask(execCtx, execCtx.Message)
			if !yield(submitted, nil) {
				return
			}
		}

		// Emit working status.
		if !yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateWorking, nil), nil) {
			return
		}

		state, parts, errText := m.handler(ctx, execCtx)

		if state == a2atypes.TaskStateFailed {
			errMsg := a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, taskInfo,
				a2atypes.NewTextPart(errText),
			)
			yield(a2atypes.NewStatusUpdateEvent(taskInfo, state, errMsg), nil)
			return
		}

		// Emit artifact.
		artifactEvent := a2atypes.NewArtifactEvent(taskInfo, parts...)
		if !yield(artifactEvent, nil) {
			return
		}

		// Emit completed.
		yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateCompleted, nil), nil)
	}
}

func (m *mockExecutor) Cancel(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2atypes.Event, error] {
	return func(yield func(a2atypes.Event, error) bool) {
		taskInfo := execCtx.TaskInfo()
		yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateCanceled, nil), nil)
	}
}

// newMockA2AAgent creates a mock A2A server with the given skills and
// handler function.
func newMockA2AAgent(
	t *testing.T,
	name string,
	description string,
	skills []a2atypes.AgentSkill,
	handler func(ctx context.Context, execCtx *a2asrv.ExecutorContext) (a2atypes.TaskState, []*a2atypes.Part, string),
) *mockA2AAgent {
	t.Helper()

	executor := &mockExecutor{handler: handler}
	reqHandler := a2asrv.NewHandler(executor)
	restHandler := a2asrv.NewRESTHandler(reqHandler)

	// Build agent card — URL will be filled in after server starts.
	card := &a2atypes.AgentCard{
		Name:               name,
		Description:        description,
		Version:            "1.0.0",
		Skills:             skills,
		DefaultInputModes:  []string{"application/json"},
		DefaultOutputModes: []string{"text/plain"},
		Capabilities:       a2atypes.AgentCapabilities{},
	}

	mux := http.NewServeMux()

	// Agent card endpoint.
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(card)
	})

	// A2A REST handler.
	mux.Handle("/", restHandler)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	// Set the URL in the card now that the server is running.
	card.SupportedInterfaces = []*a2atypes.AgentInterface{
		a2atypes.NewAgentInterface(server.URL, a2atypes.TransportProtocolHTTPJSON),
	}

	return &mockA2AAgent{
		Server: server,
		Card:   card,
	}
}

// --- tests ---

func TestRemoteAgent_FetchCardAndCreateTools(t *testing.T) {
	skills := []a2atypes.AgentSkill{
		{
			ID:          "summarize",
			Name:        "Summarize",
			Description: "Summarize text input",
			Tags:        []string{"nlp", "read"},
		},
		{
			ID:          "translate",
			Name:        "Translate",
			Description: "Translate text between languages",
			Tags:        []string{"nlp", "read"},
		},
	}

	mock := newMockA2AAgent(t, "test-agent", "A test agent", skills,
		func(_ context.Context, _ *a2asrv.ExecutorContext) (a2atypes.TaskState, []*a2atypes.Part, string) {
			return a2atypes.TaskStateCompleted, []*a2atypes.Part{a2atypes.NewTextPart("ok")}, ""
		},
	)

	ctx := context.Background()
	remote, err := NewRemoteAgent(ctx, mock.Server.URL)
	if err != nil {
		t.Fatalf("NewRemoteAgent: %v", err)
	}
	defer func() { _ = remote.Close() }()

	// Verify ToolModule interface.
	if remote.Name() != "test-agent" {
		t.Errorf("expected name %q, got %q", "test-agent", remote.Name())
	}
	if remote.Description() != "A test agent" {
		t.Errorf("expected description %q, got %q", "A test agent", remote.Description())
	}

	// Verify tools match skills.
	tools := remote.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	toolNames := make(map[string]bool)
	for _, td := range tools {
		toolNames[td.Tool.Name] = true
	}

	if !toolNames["summarize"] {
		t.Error("expected tool 'summarize'")
	}
	if !toolNames["translate"] {
		t.Error("expected tool 'translate'")
	}

	// Verify tool descriptions come from skills.
	for _, td := range tools {
		if td.Tool.Name == "summarize" && td.Tool.Description != "Summarize text input" {
			t.Errorf("expected description %q, got %q", "Summarize text input", td.Tool.Description)
		}
	}
}

func TestRemoteAgent_ToolCallSendsMessageAndReturnsResult(t *testing.T) {
	skills := []a2atypes.AgentSkill{
		{
			ID:          "echo",
			Name:        "Echo",
			Description: "Echo the input message",
			Tags:        []string{"utility"},
		},
	}

	mock := newMockA2AAgent(t, "echo-agent", "An echo agent", skills,
		func(_ context.Context, execCtx *a2asrv.ExecutorContext) (a2atypes.TaskState, []*a2atypes.Part, string) {
			// Extract the text from the incoming message.
			msg := execCtx.Message
			if msg == nil || len(msg.Parts) == 0 {
				return a2atypes.TaskStateFailed, nil, "no message"
			}
			text := msg.Parts[0].Text()
			return a2atypes.TaskStateCompleted, []*a2atypes.Part{a2atypes.NewTextPart("echo: " + text)}, ""
		},
	)

	ctx := context.Background()
	remote, err := NewRemoteAgent(ctx, mock.Server.URL)
	if err != nil {
		t.Fatalf("NewRemoteAgent: %v", err)
	}
	defer func() { _ = remote.Close() }()

	tools := remote.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	// Call the tool.
	req := registry.CallToolRequest{}
	req.Params.Name = "echo"
	req.Params.Arguments = map[string]any{"message": "hello world"}

	result, err := tools[0].Handler(ctx, req)
	if err != nil {
		t.Fatalf("tool handler error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.IsError {
		t.Error("expected non-error result")
	}

	// Extract text from result.
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content")
	}
	if !strings.Contains(text, "echo: hello world") {
		t.Errorf("expected result to contain %q, got %q", "echo: hello world", text)
	}
}

func TestRemoteAgent_TimeoutHandling(t *testing.T) {
	skills := []a2atypes.AgentSkill{
		{
			ID:          "slow",
			Name:        "Slow",
			Description: "A slow operation",
		},
	}

	mock := newMockA2AAgent(t, "slow-agent", "A slow agent", skills,
		func(ctx context.Context, _ *a2asrv.ExecutorContext) (a2atypes.TaskState, []*a2atypes.Part, string) {
			// Block until context is canceled.
			select {
			case <-ctx.Done():
				return a2atypes.TaskStateFailed, nil, "timeout"
			case <-time.After(30 * time.Second):
				return a2atypes.TaskStateCompleted, []*a2atypes.Part{a2atypes.NewTextPart("done")}, ""
			}
		},
	)

	ctx := context.Background()
	remote, err := NewRemoteAgent(ctx, mock.Server.URL,
		WithRemoteTimeout(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewRemoteAgent: %v", err)
	}
	defer func() { _ = remote.Close() }()

	tools := remote.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	req := registry.CallToolRequest{}
	req.Params.Name = "slow"
	req.Params.Arguments = map[string]any{"message": "please be slow"}

	_, err = tools[0].Handler(ctx, req)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "failed") {
		t.Errorf("expected timeout-related error, got: %v", err)
	}
}

func TestRemoteAgent_NoSkillsProducesEmptyTools(t *testing.T) {
	mock := newMockA2AAgent(t, "empty-agent", "An agent with no skills", nil,
		func(_ context.Context, _ *a2asrv.ExecutorContext) (a2atypes.TaskState, []*a2atypes.Part, string) {
			return a2atypes.TaskStateCompleted, []*a2atypes.Part{a2atypes.NewTextPart("ok")}, ""
		},
	)

	ctx := context.Background()
	remote, err := NewRemoteAgent(ctx, mock.Server.URL)
	if err != nil {
		t.Fatalf("NewRemoteAgent: %v", err)
	}
	defer func() { _ = remote.Close() }()

	tools := remote.Tools()
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestRemoteAgent_ConnectionError(t *testing.T) {
	// Create a server and immediately close it to get an unreachable URL.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	ctx := context.Background()
	_, err := NewRemoteAgent(ctx, url)
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
	if !strings.Contains(err.Error(), "failed to resolve agent card") {
		t.Errorf("expected agent card resolution error, got: %v", err)
	}
}

func TestRemoteAgent_WithPrefix(t *testing.T) {
	skills := []a2atypes.AgentSkill{
		{
			ID:          "search",
			Name:        "Search",
			Description: "Search documents",
		},
	}

	mock := newMockA2AAgent(t, "search-agent", "A search agent", skills,
		func(_ context.Context, _ *a2asrv.ExecutorContext) (a2atypes.TaskState, []*a2atypes.Part, string) {
			return a2atypes.TaskStateCompleted, []*a2atypes.Part{a2atypes.NewTextPart("found it")}, ""
		},
	)

	ctx := context.Background()
	remote, err := NewRemoteAgent(ctx, mock.Server.URL,
		WithRemotePrefix("research"),
	)
	if err != nil {
		t.Fatalf("NewRemoteAgent: %v", err)
	}
	defer func() { _ = remote.Close() }()

	tools := remote.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Tool.Name != "research_search" {
		t.Errorf("expected tool name %q, got %q", "research_search", tools[0].Tool.Name)
	}
}

func TestRemoteAgent_FromCard(t *testing.T) {
	skills := []a2atypes.AgentSkill{
		{
			ID:          "analyze",
			Name:        "Analyze",
			Description: "Analyze data",
		},
	}

	mock := newMockA2AAgent(t, "card-agent", "An agent created from card", skills,
		func(_ context.Context, _ *a2asrv.ExecutorContext) (a2atypes.TaskState, []*a2atypes.Part, string) {
			return a2atypes.TaskStateCompleted, []*a2atypes.Part{a2atypes.NewTextPart("analyzed")}, ""
		},
	)

	ctx := context.Background()
	remote, err := NewRemoteAgentFromCard(ctx, mock.Card)
	if err != nil {
		t.Fatalf("NewRemoteAgentFromCard: %v", err)
	}
	defer func() { _ = remote.Close() }()

	if remote.Name() != "card-agent" {
		t.Errorf("expected name %q, got %q", "card-agent", remote.Name())
	}

	tools := remote.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	// Call the tool to verify the full round-trip works.
	req := registry.CallToolRequest{}
	req.Params.Name = "analyze"
	req.Params.Arguments = map[string]any{"message": "test data"}

	result, err := tools[0].Handler(ctx, req)
	if err != nil {
		t.Fatalf("tool handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected successful result")
	}

	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content")
	}
	if !strings.Contains(text, "analyzed") {
		t.Errorf("expected text to contain 'analyzed', got %q", text)
	}
}

func TestRemoteAgent_FailedTask(t *testing.T) {
	skills := []a2atypes.AgentSkill{
		{
			ID:          "fail",
			Name:        "Fail",
			Description: "Always fails",
		},
	}

	mock := newMockA2AAgent(t, "fail-agent", "A failing agent", skills,
		func(_ context.Context, _ *a2asrv.ExecutorContext) (a2atypes.TaskState, []*a2atypes.Part, string) {
			return a2atypes.TaskStateFailed, nil, "deliberate failure"
		},
	)

	ctx := context.Background()
	remote, err := NewRemoteAgent(ctx, mock.Server.URL)
	if err != nil {
		t.Fatalf("NewRemoteAgent: %v", err)
	}
	defer func() { _ = remote.Close() }()

	tools := remote.Tools()
	req := registry.CallToolRequest{}
	req.Params.Name = "fail"
	req.Params.Arguments = map[string]any{"message": "do something"}

	result, err := tools[0].Handler(ctx, req)
	if err != nil {
		t.Fatalf("expected no Go error, got: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.IsError {
		t.Error("expected error result for failed task")
	}

	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content")
	}
	if !strings.Contains(text, "deliberate failure") {
		t.Errorf("expected error text to contain 'deliberate failure', got %q", text)
	}
}

func TestRemoteAgent_NilCardReturnsError(t *testing.T) {
	ctx := context.Background()
	_, err := NewRemoteAgentFromCard(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil card")
	}
}

func TestRemoteAgent_RegisterWithRegistry(t *testing.T) {
	skills := []a2atypes.AgentSkill{
		{
			ID:          "greet",
			Name:        "Greet",
			Description: "Greet someone",
		},
		{
			ID:          "farewell",
			Name:        "Farewell",
			Description: "Say goodbye",
		},
	}

	mock := newMockA2AAgent(t, "social-agent", "A social agent", skills,
		func(_ context.Context, _ *a2asrv.ExecutorContext) (a2atypes.TaskState, []*a2atypes.Part, string) {
			return a2atypes.TaskStateCompleted, []*a2atypes.Part{a2atypes.NewTextPart("hi")}, ""
		},
	)

	ctx := context.Background()
	remote, err := NewRemoteAgent(ctx, mock.Server.URL)
	if err != nil {
		t.Fatalf("NewRemoteAgent: %v", err)
	}
	defer func() { _ = remote.Close() }()

	// Register with a real mcpkit registry.
	reg := registry.NewToolRegistry()
	reg.RegisterModule(remote)

	// Verify tools are accessible via the registry.
	td, ok := reg.GetTool("greet")
	if !ok {
		t.Fatal("expected tool 'greet' in registry")
	}
	if td.Tool.Description != "Greet someone" {
		t.Errorf("expected description %q, got %q", "Greet someone", td.Tool.Description)
	}

	td2, ok := reg.GetTool("farewell")
	if !ok {
		t.Fatal("expected tool 'farewell' in registry")
	}
	if td2.Tool.Description != "Say goodbye" {
		t.Errorf("expected description %q, got %q", "Say goodbye", td2.Tool.Description)
	}
}
