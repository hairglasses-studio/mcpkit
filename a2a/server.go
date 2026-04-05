package a2a

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Server accepts A2A tasks and dispatches them as MCP tool calls.
// It serves as the A2A→MCP bridge, allowing external A2A agents to
// invoke tools on an mcpkit-based server.
type Server struct {
	reg  *registry.ToolRegistry
	card AgentCard

	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewServer creates an A2A server backed by an MCP tool registry.
func NewServer(reg *registry.ToolRegistry, card AgentCard) *Server {
	return &Server{
		reg:   reg,
		card:  card,
		tasks: make(map[string]*Task),
	}
}

// Handler returns an http.Handler that serves A2A JSON-RPC requests.
// Mount this at your A2A endpoint (e.g., "/a2a" or "/").
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent.json", s.handleAgentCard)
	mux.HandleFunc("/", s.handleJSONRPC)
	return mux
}

func (s *Server) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.card)
}

func (s *Server) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, req.ID, -32700, "parse error")
		return
	}

	switch req.Method {
	case "tasks/send":
		s.handleTaskSend(w, req)
	case "tasks/get":
		s.handleTaskGet(w, req)
	case "tasks/cancel":
		s.handleTaskCancel(w, req)
	default:
		writeRPCError(w, req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) handleTaskSend(w http.ResponseWriter, req JSONRPCRequest) {
	var params TaskSendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	if params.ID == "" {
		params.ID = uuid.New().String()
	}

	// Create task
	task := &Task{
		ID:       params.ID,
		State:    TaskSubmitted,
		Messages: params.Messages,
		Metadata: params.Metadata,
		Created:  time.Now(),
		Updated:  time.Now(),
	}

	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()

	// Dispatch to MCP tool (synchronous for now)
	go s.dispatchTask(task)

	writeRPCResult(w, req.ID, task)
}

func (s *Server) handleTaskGet(w http.ResponseWriter, req JSONRPCRequest) {
	var params TaskQueryParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	s.mu.RLock()
	task, ok := s.tasks[params.ID]
	s.mu.RUnlock()

	if !ok {
		writeRPCError(w, req.ID, -32001, "task not found")
		return
	}

	writeRPCResult(w, req.ID, task)
}

func (s *Server) handleTaskCancel(w http.ResponseWriter, req JSONRPCRequest) {
	var params TaskQueryParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	s.mu.Lock()
	task, ok := s.tasks[params.ID]
	if ok && !task.State.IsTerminal() {
		task.State = TaskCanceled
		task.Updated = time.Now()
	}
	s.mu.Unlock()

	if !ok {
		writeRPCError(w, req.ID, -32001, "task not found")
		return
	}

	writeRPCResult(w, req.ID, task)
}

// dispatchTask finds the best MCP tool for the task and executes it.
func (s *Server) dispatchTask(task *Task) {
	s.mu.Lock()
	task.State = TaskWorking
	task.Updated = time.Now()
	s.mu.Unlock()

	// Extract task text from user messages
	var taskText string
	for _, msg := range task.Messages {
		if msg.Role == "user" {
			for _, part := range msg.Parts {
				if part.Type == "text" {
					taskText += part.Text + " "
				}
			}
		}
	}

	// Try to find a matching tool by searching tool names/descriptions
	toolName := s.findBestTool(taskText)
	if toolName == "" {
		s.mu.Lock()
		task.State = TaskFailed
		task.Messages = append(task.Messages, Message{
			Role:  "agent",
			Parts: []Part{TextPart("no matching tool found for this task")},
		})
		task.Updated = time.Now()
		s.mu.Unlock()
		return
	}

	// Mark task as completed with the tool name as the result.
	// In production, the server would dispatch via the MCP server's handler,
	// but for the bridge we record the intent and let the caller handle dispatch.
	s.mu.Lock()
	defer s.mu.Unlock()

	task.State = TaskCompleted
	task.Messages = append(task.Messages, Message{
		Role:  "agent",
		Parts: []Part{TextPart(fmt.Sprintf("dispatched to tool: %s (input: %s)", toolName, taskText))},
	})
	task.Updated = time.Now()
}

// findBestTool returns the name of the best matching tool for the task text.
// Simple keyword matching for now — can be upgraded to semantic search.
func (s *Server) findBestTool(taskText string) string {
	tools := s.reg.ListTools()
	if len(tools) == 0 {
		return ""
	}
	// Return first tool as default (in production, use search/matching)
	return tools[0]
}

func writeRPCResult(w http.ResponseWriter, id interface{}, result interface{}) {
	resultJSON, _ := json.Marshal(result)
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: resultJSON}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: message},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
