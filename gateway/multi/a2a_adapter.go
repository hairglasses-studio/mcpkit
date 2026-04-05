package multi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
)

// a2aSendMessageMethod is the JSON-RPC method for A2A message send.
const a2aSendMessageMethod = "a2a.sendMessage"

// A2AAdapter implements the Adapter interface for the Agent-to-Agent (A2A)
// protocol. It translates between A2A JSON-RPC requests and the gateway's
// canonical request/response model.
//
// The adapter handles:
//   - Detection of A2A requests via JSON-RPC methods and well-known REST paths
//   - Decoding of a2a.SendMessage into CanonicalRequest (skill + arguments)
//   - Encoding of CanonicalResponse into an A2A Task with artifacts
//
// It reuses the data extraction strategy from the bridge/a2a Translator:
// look for a DataPart with {"skill": "...", "arguments": {...}} first, then
// fall back to treating the first TextPart as JSON arguments with a skill hint.
//
// A2AAdapter is safe for concurrent use.
type A2AAdapter struct{}

// Verify interface compliance at compile time.
var _ Adapter = (*A2AAdapter)(nil)

// NewA2AAdapter creates a new A2A protocol adapter.
func NewA2AAdapter() *A2AAdapter {
	return &A2AAdapter{}
}

// Protocol returns ProtocolA2A.
func (a *A2AAdapter) Protocol() Protocol {
	return ProtocolA2A
}

// Detect inspects an HTTP request and returns whether it looks like an A2A
// request. Detection signals, in priority order:
//
//  1. Well-known A2A paths (/.well-known/agent-card.json, /agent-card:extended)
//  2. URL path prefix (/a2a/)
//  3. A2A JSON-RPC method in the body peek (a2a.sendMessage, a2a.getTask, etc.)
//
// Detect does not consume the request body; it only inspects the pre-buffered
// bodyPeek bytes.
func (a *A2AAdapter) Detect(r *http.Request, bodyPeek []byte) (bool, Confidence) {
	// Check well-known paths first.
	path := r.URL.Path
	if path == "/.well-known/agent-card.json" || path == "/agent-card:extended" {
		return true, ConfidenceDefinitive
	}

	// Check path prefix.
	normalized := strings.TrimSuffix(path, "/")
	if normalized == "/a2a" || strings.HasPrefix(path, "/a2a/") {
		return true, ConfidenceHigh
	}

	// Check JSON-RPC method in body.
	if r.Method == http.MethodPost && len(bodyPeek) > 0 {
		method := extractJSONRPCMethod(bodyPeek)
		if method != "" && isA2AMethod(method) {
			return true, ConfidenceDefinitive
		}
	}

	return false, ConfidenceLow
}

// a2aJSONRPCRequest is the wire format for an A2A JSON-RPC request. We parse
// just enough to extract the method, id, and params as raw JSON for further
// decoding.
type a2aJSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      json.RawMessage `json:"id"`
	Params  json.RawMessage `json:"params"`
}

// Decode parses an A2A HTTP request into a CanonicalRequest.
//
// For a2a.sendMessage requests, it extracts the skill name and arguments
// from the A2A message parts using the same strategy as bridge/a2a:
//   - Strategy 1: DataPart with {"skill": "name", "arguments": {...}}
//   - Strategy 2: TextPart parsed as JSON with an optional skill hint
//
// The JSON-RPC id is mapped to CanonicalRequest.RequestID. The A2A method
// is preserved in Metadata["a2a.method"] for protocol-specific handling.
func (a *A2AAdapter) Decode(r *http.Request) (*CanonicalRequest, error) {
	// Parse the JSON-RPC envelope.
	var rpcReq a2aJSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&rpcReq); err != nil {
		return nil, fmt.Errorf("a2a: failed to decode JSON-RPC request: %w", err)
	}

	if rpcReq.Method == "" {
		return nil, fmt.Errorf("a2a: missing JSON-RPC method")
	}

	// Extract the request ID as a string for the canonical model.
	requestID := extractRequestID(rpcReq.ID)

	// For non-sendMessage methods, return a metadata-only canonical request
	// that the router or handler can use for direct handling.
	if rpcReq.Method != a2aSendMessageMethod {
		return &CanonicalRequest{
			Protocol:  ProtocolA2A,
			RequestID: requestID,
			Metadata: map[string]string{
				"a2a.method": rpcReq.Method,
			},
		}, nil
	}

	// Parse the SendMessageRequest from params.
	var sendReq a2atypes.SendMessageRequest
	if err := json.Unmarshal(rpcReq.Params, &sendReq); err != nil {
		return nil, fmt.Errorf("a2a: failed to decode SendMessageRequest params: %w", err)
	}

	if sendReq.Message == nil {
		return nil, fmt.Errorf("a2a: SendMessageRequest has no message")
	}

	// Extract the tool name and arguments from the message parts.
	toolName, args, err := extractSkillFromMessage(sendReq.Message)
	if err != nil {
		return nil, fmt.Errorf("a2a: %w", err)
	}

	// Build metadata from context.
	metadata := map[string]string{
		"a2a.method": rpcReq.Method,
	}
	if sendReq.Message.ContextID != "" {
		metadata["a2a.contextId"] = sendReq.Message.ContextID
	}
	if sendReq.Message.TaskID != "" {
		metadata["a2a.taskId"] = string(sendReq.Message.TaskID)
	}

	return &CanonicalRequest{
		Protocol:  ProtocolA2A,
		ToolName:  toolName,
		Arguments: args,
		RequestID: requestID,
		Metadata:  metadata,
	}, nil
}

// Encode translates a CanonicalResponse into the A2A JSON-RPC wire format.
//
// The response is wrapped as a JSON-RPC result containing an A2A Task object.
// On success, the task state is COMPLETED with an artifact containing the
// response content. On failure, the task state is FAILED with an error message.
func (a *A2AAdapter) Encode(resp *CanonicalResponse) ([]byte, string, error) {
	// Build the task ID and context ID from metadata, or generate new ones.
	taskID := a2atypes.NewTaskID()
	contextID := a2atypes.NewContextID()

	if resp.Metadata != nil {
		if tid, ok := resp.Metadata["a2a.taskId"]; ok && tid != "" {
			taskID = a2atypes.TaskID(tid)
		}
		if cid, ok := resp.Metadata["a2a.contextId"]; ok && cid != "" {
			contextID = cid
		}
	}

	var task a2atypes.Task
	task.ID = taskID
	task.ContextID = contextID

	if resp.Success {
		// Build an artifact from the response content.
		var parts []*a2atypes.Part
		for _, cp := range resp.Content {
			part := canonicalPartToA2A(cp)
			if part != nil {
				parts = append(parts, part)
			}
		}
		if parts == nil {
			parts = []*a2atypes.Part{}
		}

		artifact := &a2atypes.Artifact{
			ID:    a2atypes.NewArtifactID(),
			Parts: parts,
		}
		task.Artifacts = []*a2atypes.Artifact{artifact}
		task.Status = a2atypes.TaskStatus{
			State: a2atypes.TaskStateCompleted,
		}
	} else {
		// Build a failed task with the error message.
		errMsg := "unknown error"
		if resp.Error != nil && resp.Error.Message != "" {
			errMsg = resp.Error.Message
		}

		failMsg := a2atypes.NewMessage(
			a2atypes.MessageRoleAgent,
			a2atypes.NewTextPart(errMsg),
		)
		failMsg.TaskID = taskID
		failMsg.ContextID = contextID

		task.Status = a2atypes.TaskStatus{
			State:   a2atypes.TaskStateFailed,
			Message: failMsg,
		}
	}

	// Wrap in a JSON-RPC response envelope.
	rpcResp := a2aJSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"` + resp.RequestID + `"`),
		Result:  &task,
	}

	body, err := json.Marshal(rpcResp)
	if err != nil {
		return nil, "", fmt.Errorf("a2a: failed to encode response: %w", err)
	}

	return body, "application/json", nil
}

// a2aJSONRPCResponse is the wire format for an A2A JSON-RPC response.
type a2aJSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  *a2atypes.Task  `json:"result"`
}

// extractSkillFromMessage extracts the tool name and arguments from an A2A
// message, following the same strategy as bridge/a2a Translator:
//
//  1. Look for a DataPart containing {"skill": "...", "arguments": {...}}
//  2. Fall back to the first TextPart, attempting JSON parse then wrapping as {"input": text}
func extractSkillFromMessage(msg *a2atypes.Message) (string, map[string]any, error) {
	if msg == nil || len(msg.Parts) == 0 {
		return "", nil, fmt.Errorf("no parts in message")
	}

	// Strategy 1: DataPart with structured skill dispatch.
	for _, part := range msg.Parts {
		if part == nil {
			continue
		}
		data := part.Data()
		if data == nil {
			continue
		}

		dataMap, ok := data.(map[string]any)
		if !ok {
			continue
		}

		skillID, _ := dataMap["skill"].(string)
		if skillID == "" {
			continue
		}

		args, _ := dataMap["arguments"].(map[string]any)
		if args == nil {
			args = make(map[string]any)
		}

		return skillID, args, nil
	}

	// Strategy 2: TextPart as JSON or plain text.
	for _, part := range msg.Parts {
		if part == nil {
			continue
		}
		text := part.Text()
		if text == "" {
			continue
		}

		// Try JSON parse.
		var parsed map[string]any
		if err := json.Unmarshal([]byte(text), &parsed); err == nil {
			// If the parsed JSON has a "skill" field, use it.
			if skillID, ok := parsed["skill"].(string); ok && skillID != "" {
				args, _ := parsed["arguments"].(map[string]any)
				if args == nil {
					args = make(map[string]any)
				}
				return skillID, args, nil
			}
		}
	}

	return "", nil, fmt.Errorf("no skill identifier found in message: expected DataPart with 'skill' field")
}

// canonicalPartToA2A converts a canonical ContentPart to an A2A Part.
func canonicalPartToA2A(cp ContentPart) *a2atypes.Part {
	switch cp.Type {
	case ContentTypeText:
		return a2atypes.NewTextPart(cp.Text)
	case ContentTypeJSON:
		return a2atypes.NewDataPart(cp.JSON)
	case ContentTypeData:
		if cp.Data != nil {
			return a2atypes.NewRawPart(cp.Data)
		}
		return nil
	case ContentTypeImage:
		if cp.Data != nil {
			part := a2atypes.NewRawPart(cp.Data)
			part.MediaType = cp.MimeType
			return part
		}
		return nil
	default:
		if cp.Text != "" {
			return a2atypes.NewTextPart(cp.Text)
		}
		return nil
	}
}

// extractRequestID converts a JSON-RPC id (which may be a string, number, or
// null) into a string for the canonical model.
func extractRequestID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	trimmed := strings.TrimSpace(string(raw))

	// JSON null: preserve as the literal string "null" since JSON-RPC
	// allows null IDs for notifications.
	if trimmed == "null" {
		return "null"
	}

	// Try string first (most common for A2A).
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try number (common for generic JSON-RPC clients).
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}

	// Fallback: use the raw JSON representation.
	if trimmed != "" {
		return trimmed
	}

	return ""
}
