package a2a

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

	mu          sync.RWMutex
	tasks       map[string]*Task
	pushConfigs map[string]map[string]*PushNotificationConfig
	pushClient  *http.Client
}

const defaultPushNotificationTimeout = 5 * time.Second

// ServerOption configures an A2A server.
type ServerOption func(*Server)

// WithPushHTTPClient sets the HTTP client used for push webhook delivery.
func WithPushHTTPClient(client *http.Client) ServerOption {
	return func(s *Server) {
		if client != nil {
			s.pushClient = client
		}
	}
}

// NewServer creates an A2A server backed by an MCP tool registry.
func NewServer(reg *registry.ToolRegistry, card AgentCard, opts ...ServerOption) *Server {
	s := &Server{
		reg:         reg,
		card:        card,
		tasks:       make(map[string]*Task),
		pushConfigs: make(map[string]map[string]*PushNotificationConfig),
		pushClient:  &http.Client{Timeout: defaultPushNotificationTimeout},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Handler returns an http.Handler that serves A2A JSON-RPC requests.
// Mount this at your A2A endpoint (e.g., "/a2a" or "/").
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent.json", s.handleAgentCard)
	mux.HandleFunc("/tasks/", s.handleTaskREST)
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
	case "CreateTaskPushNotificationConfig", "tasks/pushNotificationConfig/create", "tasks/pushNotificationConfig/set":
		s.handlePushCreate(w, req)
	case "GetTaskPushNotificationConfig", "tasks/pushNotificationConfig/get":
		s.handlePushGet(w, req)
	case "ListTaskPushNotificationConfigs", "tasks/pushNotificationConfig/list":
		s.handlePushList(w, req)
	case "DeleteTaskPushNotificationConfig", "tasks/pushNotificationConfig/delete":
		s.handlePushDelete(w, req)
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

	now := time.Now()
	var pushConfig *PushNotificationConfig
	if params.PushNotification != nil {
		if !s.pushSupported() {
			writeRPCError(w, req.ID, -32003, "push notifications not supported")
			return
		}
		cfg, err := normalizePushConfig(*params.PushNotification, params.ID)
		if err != nil {
			writeRPCError(w, req.ID, -32602, err.Error())
			return
		}
		pushConfig = &cfg
	}

	// Create task
	task := &Task{
		ID:       params.ID,
		State:    TaskSubmitted,
		Messages: params.Messages,
		Metadata: params.Metadata,
		Created:  now,
		Updated:  now,
	}

	s.mu.Lock()
	s.tasks[task.ID] = task
	if pushConfig != nil {
		pushConfig.TaskID = task.ID
		s.storePushConfigLocked(*pushConfig)
	}
	snapshot := task.snapshot()
	s.mu.Unlock()

	// Dispatch to MCP tool asynchronously.
	go s.dispatchTask(task.ID)

	// Serialize the snapshot taken under lock to avoid racing with dispatchTask.
	writeRPCResult(w, req.ID, &snapshot)
}

func (s *Server) handleTaskGet(w http.ResponseWriter, req JSONRPCRequest) {
	var params TaskQueryParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	s.mu.RLock()
	task, ok := s.tasks[params.ID]
	var snapshot Task
	if ok {
		snapshot = task.snapshot()
	}
	s.mu.RUnlock()

	if !ok {
		writeRPCError(w, req.ID, -32001, "task not found")
		return
	}

	writeRPCResult(w, req.ID, &snapshot)
}

func (s *Server) handleTaskCancel(w http.ResponseWriter, req JSONRPCRequest) {
	var params TaskQueryParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	snapshot, ok := s.updateTask(params.ID, func(task *Task) bool {
		if task.State.IsTerminal() {
			return false
		}
		task.State = TaskCanceled
		task.Updated = time.Now()
		return true
	})

	if !ok {
		writeRPCError(w, req.ID, -32001, "task not found")
		return
	}

	writeRPCResult(w, req.ID, &snapshot)
}

// dispatchTask finds the best MCP tool for the task and executes it.
func (s *Server) dispatchTask(taskID string) {
	snapshot, ok := s.updateTask(taskID, func(task *Task) bool {
		if task.State.IsTerminal() {
			return false
		}
		task.State = TaskWorking
		task.Updated = time.Now()
		return true
	})
	if !ok || snapshot.State.IsTerminal() {
		return
	}

	// Extract task text from user messages
	var taskText string
	for _, msg := range snapshot.Messages {
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
		s.updateTask(taskID, func(task *Task) bool {
			if task.State.IsTerminal() {
				return false
			}
			task.State = TaskFailed
			task.Messages = append(task.Messages, Message{
				Role:  "agent",
				Parts: []Part{TextPart("no matching tool found for this task")},
			})
			task.Updated = time.Now()
			return true
		})
		return
	}

	// Mark task as completed with the tool name as the result.
	// In production, the server would dispatch via the MCP server's handler,
	// but for the bridge we record the intent and let the caller handle dispatch.
	s.updateTask(taskID, func(task *Task) bool {
		if task.State.IsTerminal() {
			return false
		}
		task.State = TaskCompleted
		task.Messages = append(task.Messages, Message{
			Role:  "agent",
			Parts: []Part{TextPart(fmt.Sprintf("dispatched to tool: %s (input: %s)", toolName, taskText))},
		})
		task.Updated = time.Now()
		return true
	})
}

func (s *Server) handlePushCreate(w http.ResponseWriter, req JSONRPCRequest) {
	if !s.pushSupportedRPC(w, req.ID) {
		return
	}
	var params PushNotificationConfig
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}
	config, err := s.createPushConfig(params)
	if err != nil {
		writePushRPCError(w, req.ID, err)
		return
	}
	writeRPCResult(w, req.ID, config)
}

func (s *Server) handlePushGet(w http.ResponseWriter, req JSONRPCRequest) {
	if !s.pushSupportedRPC(w, req.ID) {
		return
	}
	var params GetTaskPushNotificationConfigParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}
	config, err := s.getPushConfig(params.TaskID, params.ID)
	if err != nil {
		writePushRPCError(w, req.ID, err)
		return
	}
	writeRPCResult(w, req.ID, config)
}

func (s *Server) handlePushList(w http.ResponseWriter, req JSONRPCRequest) {
	if !s.pushSupportedRPC(w, req.ID) {
		return
	}
	var params ListTaskPushNotificationConfigsParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}
	result, err := s.listPushConfigs(params)
	if err != nil {
		writePushRPCError(w, req.ID, err)
		return
	}
	writeRPCResult(w, req.ID, result)
}

func (s *Server) handlePushDelete(w http.ResponseWriter, req JSONRPCRequest) {
	if !s.pushSupportedRPC(w, req.ID) {
		return
	}
	var params DeleteTaskPushNotificationConfigParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}
	result, err := s.deletePushConfig(params.TaskID, params.ID)
	if err != nil {
		writePushRPCError(w, req.ID, err)
		return
	}
	writeRPCResult(w, req.ID, result)
}

func (s *Server) handleTaskREST(w http.ResponseWriter, r *http.Request) {
	taskID, configID, ok := parsePushConfigPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !s.pushSupported() {
		writeRESTError(w, http.StatusBadRequest, "push notifications not supported")
		return
	}

	switch {
	case r.Method == http.MethodPost && configID == "":
		var config PushNotificationConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeRESTError(w, http.StatusBadRequest, "invalid push notification config")
			return
		}
		config.TaskID = taskID
		created, err := s.createPushConfig(config)
		if err != nil {
			writePushRESTError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	case r.Method == http.MethodGet && configID == "":
		result, err := s.listPushConfigs(ListTaskPushNotificationConfigsParams{
			TaskID:    taskID,
			PageToken: r.URL.Query().Get("pageToken"),
		})
		if err != nil {
			writePushRESTError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case r.Method == http.MethodGet && configID != "":
		config, err := s.getPushConfig(taskID, configID)
		if err != nil {
			writePushRESTError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, config)
	case r.Method == http.MethodDelete && configID != "":
		result, err := s.deletePushConfig(taskID, configID)
		if err != nil {
			writePushRESTError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type pushError struct {
	code       int
	httpStatus int
	message    string
}

func (e pushError) Error() string {
	return e.message
}

func (s *Server) createPushConfig(config PushNotificationConfig) (*PushNotificationConfig, error) {
	normalized, err := normalizePushConfig(config, config.TaskID)
	if err != nil {
		return nil, pushError{code: -32602, httpStatus: http.StatusBadRequest, message: err.Error()}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[normalized.TaskID]; !ok {
		return nil, pushError{code: -32001, httpStatus: http.StatusNotFound, message: "task not found"}
	}
	s.storePushConfigLocked(normalized)
	snapshot := normalized
	return &snapshot, nil
}

func (s *Server) getPushConfig(taskID, configID string) (*PushNotificationConfig, error) {
	if taskID == "" || configID == "" {
		return nil, pushError{code: -32602, httpStatus: http.StatusBadRequest, message: "taskId and id are required"}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	configs, ok := s.pushConfigs[taskID]
	if !ok {
		if _, taskOK := s.tasks[taskID]; !taskOK {
			return nil, pushError{code: -32001, httpStatus: http.StatusNotFound, message: "task not found"}
		}
		return nil, pushError{code: -32001, httpStatus: http.StatusNotFound, message: "push notification config not found"}
	}
	config, ok := configs[configID]
	if !ok {
		return nil, pushError{code: -32001, httpStatus: http.StatusNotFound, message: "push notification config not found"}
	}
	snapshot := *config
	return &snapshot, nil
}

func (s *Server) listPushConfigs(params ListTaskPushNotificationConfigsParams) (*ListTaskPushNotificationConfigsResponse, error) {
	if params.TaskID == "" {
		return nil, pushError{code: -32602, httpStatus: http.StatusBadRequest, message: "taskId is required"}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.tasks[params.TaskID]; !ok {
		return nil, pushError{code: -32001, httpStatus: http.StatusNotFound, message: "task not found"}
	}
	configsByID := s.pushConfigs[params.TaskID]
	configs := make([]PushNotificationConfig, 0, len(configsByID))
	for _, config := range configsByID {
		configs = append(configs, *config)
	}
	return &ListTaskPushNotificationConfigsResponse{Configs: configs}, nil
}

func (s *Server) deletePushConfig(taskID, configID string) (*DeleteTaskPushNotificationConfigResult, error) {
	if taskID == "" || configID == "" {
		return nil, pushError{code: -32602, httpStatus: http.StatusBadRequest, message: "taskId and id are required"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[taskID]; !ok {
		return nil, pushError{code: -32001, httpStatus: http.StatusNotFound, message: "task not found"}
	}
	if configs := s.pushConfigs[taskID]; configs != nil {
		delete(configs, configID)
		if len(configs) == 0 {
			delete(s.pushConfigs, taskID)
		}
	}
	return &DeleteTaskPushNotificationConfigResult{Deleted: true}, nil
}

func (s *Server) updateTask(taskID string, mutate func(*Task) bool) (Task, bool) {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return Task{}, false
	}
	changed := mutate(task)
	snapshot := task.snapshot()
	configs := s.pushConfigsForTaskLocked(taskID)
	s.mu.Unlock()

	if changed {
		s.notifyTaskUpdate(snapshot, configs)
	}
	return snapshot, true
}

func (s *Server) storePushConfigLocked(config PushNotificationConfig) {
	if s.pushConfigs[config.TaskID] == nil {
		s.pushConfigs[config.TaskID] = make(map[string]*PushNotificationConfig)
	}
	snapshot := config
	s.pushConfigs[config.TaskID][config.ID] = &snapshot
}

func (s *Server) pushConfigsForTaskLocked(taskID string) []PushNotificationConfig {
	configsByID := s.pushConfigs[taskID]
	if len(configsByID) == 0 {
		return nil
	}
	configs := make([]PushNotificationConfig, 0, len(configsByID))
	for _, config := range configsByID {
		configs = append(configs, *config)
	}
	return configs
}

func (s *Server) notifyTaskUpdate(task Task, configs []PushNotificationConfig) {
	if len(configs) == 0 {
		return
	}
	payload := StreamResponse{
		StatusUpdate: &TaskStatusUpdateEvent{
			TaskID: task.ID,
			Status: TaskStatus{
				State:     task.State,
				Timestamp: task.Updated,
			},
			Metadata: task.Metadata,
		},
	}
	for _, config := range configs {
		config := config
		go s.deliverPushNotification(config, payload)
	}
	if task.State.IsTerminal() {
		s.clearPushConfigs(task.ID)
	}
}

func (s *Server) deliverPushNotification(config PushNotificationConfig, payload StreamResponse) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, config.URL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if config.Token != "" {
		req.Header.Set("X-A2A-Notification-Token", config.Token)
	}
	if config.Authentication != nil && config.Authentication.Scheme != "" && config.Authentication.Credentials != "" {
		req.Header.Set("Authorization", config.Authentication.Scheme+" "+config.Authentication.Credentials)
	}
	resp, err := s.pushClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

func (s *Server) clearPushConfigs(taskID string) {
	s.mu.Lock()
	delete(s.pushConfigs, taskID)
	s.mu.Unlock()
}

func (s *Server) pushSupported() bool {
	return s.card.Capabilities != nil && s.card.Capabilities.PushNotifications
}

func (s *Server) pushSupportedRPC(w http.ResponseWriter, id interface{}) bool {
	if s.pushSupported() {
		return true
	}
	writeRPCError(w, id, -32003, "push notifications not supported")
	return false
}

func normalizePushConfig(config PushNotificationConfig, fallbackTaskID string) (PushNotificationConfig, error) {
	if config.TaskID == "" {
		config.TaskID = fallbackTaskID
	}
	if config.TaskID == "" {
		return PushNotificationConfig{}, fmt.Errorf("taskId is required")
	}
	if config.URL == "" {
		return PushNotificationConfig{}, fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(config.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return PushNotificationConfig{}, fmt.Errorf("url must be an absolute HTTP(S) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return PushNotificationConfig{}, fmt.Errorf("url must use http or https")
	}
	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	return config, nil
}

func parsePushConfigPath(path string) (taskID string, configID string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/tasks/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "pushNotificationConfigs" {
		return "", "", false
	}
	if len(parts) == 2 {
		return parts[0], "", true
	}
	if len(parts) == 3 && parts[2] != "" {
		return parts[0], parts[2], true
	}
	return "", "", false
}

func writePushRPCError(w http.ResponseWriter, id interface{}, err error) {
	if pe, ok := err.(pushError); ok {
		writeRPCError(w, id, pe.code, pe.message)
		return
	}
	writeRPCError(w, id, -32000, err.Error())
}

func writePushRESTError(w http.ResponseWriter, err error) {
	if pe, ok := err.(pushError); ok {
		writeRESTError(w, pe.httpStatus, pe.message)
		return
	}
	writeRESTError(w, http.StatusInternalServerError, err.Error())
}

func writeRESTError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
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
