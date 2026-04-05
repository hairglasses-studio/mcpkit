package multi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/hairglasses-studio/mcpkit/registry"
)

const (
	// maxBodyPeek is the maximum number of bytes to read from the request
	// body for protocol detection. Chosen to be large enough to contain
	// the JSON-RPC method field in any reasonable request.
	maxBodyPeek = 512
)

// Router is an HTTP handler that detects the protocol of incoming requests
// and dispatches them to the appropriate adapter. It holds a reference to
// the mcpkit ToolRegistry for tool resolution and invocation.
//
// Router is safe for concurrent use.
type Router struct {
	mu       sync.RWMutex
	adapters map[Protocol]Adapter
	registry *registry.ToolRegistry
	logger   *slog.Logger
}

// NewRouter creates a Router backed by the given ToolRegistry. The registry
// is used to look up and invoke tools when an adapter decodes a tool call.
func NewRouter(reg *registry.ToolRegistry, opts ...RouterOption) *Router {
	r := &Router{
		adapters: make(map[Protocol]Adapter),
		registry: reg,
		logger:   slog.Default(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RouterOption configures a Router.
type RouterOption func(*Router)

// WithLogger sets the structured logger for the router.
func WithLogger(logger *slog.Logger) RouterOption {
	return func(r *Router) {
		if logger != nil {
			r.logger = logger
		}
	}
}

// Register adds an adapter for a protocol. If an adapter for the same
// protocol is already registered, it is replaced.
func (r *Router) Register(adapter Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Protocol()] = adapter
	r.logger.Info("registered protocol adapter",
		"protocol", adapter.Protocol())
}

// Adapters returns the currently registered protocols.
func (r *Router) Adapters() []Protocol {
	r.mu.RLock()
	defer r.mu.RUnlock()
	protocols := make([]Protocol, 0, len(r.adapters))
	for p := range r.adapters {
		protocols = append(protocols, p)
	}
	return protocols
}

// ServeHTTP implements http.Handler. It peeks the request body for detection,
// selects the best adapter, decodes the request into canonical form, invokes
// the tool via the registry, and encodes the response in the caller's protocol.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Buffer the body peek for protocol detection without consuming it.
	bodyPeek, bufferedReq, err := peekRequestBody(req, maxBodyPeek)
	if err != nil {
		r.logger.Error("failed to peek request body", "error", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	req = bufferedReq

	// Detect protocol using the global detection function.
	protocol, confidence := DetectProtocol(req, bodyPeek)

	// Find a matching adapter, preferring the detected one but also
	// consulting each adapter's own Detect method for refinement.
	adapter := r.resolveAdapter(req, bodyPeek, protocol, confidence)
	if adapter == nil {
		r.logger.Warn("no adapter matched request",
			"detected_protocol", protocol,
			"confidence", confidence.String(),
			"path", req.URL.Path,
			"method", req.Method)
		r.writeUnsupportedProtocol(w, protocol)
		return
	}

	r.logger.Debug("dispatching request",
		"protocol", adapter.Protocol(),
		"path", req.URL.Path,
		"method", req.Method)

	// Decode the request into canonical form.
	canonical, err := adapter.Decode(req)
	if err != nil {
		r.logger.Error("adapter decode failed",
			"protocol", adapter.Protocol(),
			"error", err)
		r.writeDecodeError(w, adapter, err)
		return
	}

	// Look up the tool in the registry.
	toolDef, ok := r.registry.GetTool(canonical.ToolName)
	if !ok {
		resp := &CanonicalResponse{
			Success:   false,
			Error:     &ErrorInfo{Code: ErrNotFound, Message: fmt.Sprintf("tool %q not found", canonical.ToolName)},
			RequestID: canonical.RequestID,
		}
		r.encodeAndWrite(w, adapter, resp)
		return
	}

	// Invoke the tool handler.
	result := r.invokeTool(req.Context(), toolDef, canonical)

	// Encode and write the response.
	r.encodeAndWrite(w, adapter, result)
}

// resolveAdapter finds the best adapter for the request. It first checks
// the globally-detected protocol, then falls back to asking each adapter.
func (r *Router) resolveAdapter(req *http.Request, bodyPeek []byte, detected Protocol, detectedConf Confidence) Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// If detection was definitive or high, use that adapter directly.
	if detectedConf >= ConfidenceHigh {
		if a, ok := r.adapters[detected]; ok {
			return a
		}
	}

	// Consult each adapter's Detect method and pick the highest confidence.
	var best Adapter
	var bestConf Confidence = -1

	for _, a := range r.adapters {
		matches, conf := a.Detect(req, bodyPeek)
		if matches && conf > bestConf {
			best = a
			bestConf = conf
		}
	}

	// Only accept if confidence is at least medium.
	if best != nil && bestConf >= ConfidenceMedium {
		return best
	}

	// Last resort: if global detection had a result, try that adapter.
	if detected != ProtocolUnknown {
		if a, ok := r.adapters[detected]; ok {
			return a
		}
	}

	return nil
}

// invokeTool calls the tool handler and wraps the result in a CanonicalResponse.
func (r *Router) invokeTool(ctx context.Context, td registry.ToolDefinition, req *CanonicalRequest) *CanonicalResponse {
	if td.Handler == nil {
		return &CanonicalResponse{
			Success:   false,
			Error:     &ErrorInfo{Code: ErrInternal, Message: fmt.Sprintf("tool %q has no handler", req.ToolName)},
			RequestID: req.RequestID,
		}
	}

	// Build the MCP CallToolRequest from the canonical request.
	var callReq registry.CallToolRequest
	callReq.Params.Name = req.ToolName
	callReq.Params.Arguments = req.Arguments

	result, err := td.Handler(ctx, callReq)
	if err != nil {
		return &CanonicalResponse{
			Success:   false,
			Error:     &ErrorInfo{Code: ErrInternal, Message: err.Error()},
			RequestID: req.RequestID,
		}
	}

	// Translate MCP CallToolResult to CanonicalResponse.
	resp := &CanonicalResponse{
		Success:   !result.IsError,
		RequestID: req.RequestID,
	}

	if result.IsError {
		// Extract error message from content.
		msg := "tool returned error"
		for _, c := range result.Content {
			if tc, ok := c.(registry.TextContent); ok && tc.Text != "" {
				msg = tc.Text
				break
			}
		}
		resp.Error = &ErrorInfo{Code: ErrInternal, Message: msg}
	}

	for _, c := range result.Content {
		switch v := c.(type) {
		case registry.TextContent:
			resp.Content = append(resp.Content, ContentPart{
				Type: ContentTypeText,
				Text: v.Text,
			})
		default:
			// Other content types: serialize as JSON.
			resp.Content = append(resp.Content, ContentPart{
				Type: ContentTypeJSON,
				JSON: v,
			})
		}
	}

	return resp
}

// encodeAndWrite serializes the response using the adapter and writes it.
func (r *Router) encodeAndWrite(w http.ResponseWriter, adapter Adapter, resp *CanonicalResponse) {
	body, contentType, err := adapter.Encode(resp)
	if err != nil {
		r.logger.Error("adapter encode failed",
			"protocol", adapter.Protocol(),
			"error", err)
		http.Error(w, "internal encoding error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	if !resp.Success {
		// Use 400 for client errors, 500 for server errors.
		if resp.Error != nil && (resp.Error.Code == ErrInvalidParams || resp.Error.Code == ErrNotFound) {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		w.WriteHeader(http.StatusOK)
	}
	w.Write(body)
}

// writeUnsupportedProtocol sends a 400 response listing supported protocols.
func (r *Router) writeUnsupportedProtocol(w http.ResponseWriter, detected Protocol) {
	supported := r.Adapters()
	resp := map[string]any{
		"error":               "unsupported or undetectable protocol",
		"detected_protocol":   detected,
		"supported_protocols": supported,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(resp)
}

// writeDecodeError sends a 400 response for decode failures.
func (r *Router) writeDecodeError(w http.ResponseWriter, adapter Adapter, decodeErr error) {
	resp := &CanonicalResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    ErrInvalidParams,
			Message: decodeErr.Error(),
		},
	}
	body, contentType, err := adapter.Encode(resp)
	if err != nil {
		http.Error(w, decodeErr.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusBadRequest)
	w.Write(body)
}

// peekRequestBody reads up to n bytes from the request body without consuming
// it. It replaces req.Body with a reader that yields the peeked bytes followed
// by the remainder, so subsequent reads see the full body.
func peekRequestBody(req *http.Request, n int) ([]byte, *http.Request, error) {
	if req.Body == nil || req.ContentLength == 0 {
		return nil, req, nil
	}

	peek := make([]byte, n)
	nRead, err := io.ReadAtLeast(req.Body, peek, 1)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, req, fmt.Errorf("peek body: %w", err)
	}
	peek = peek[:nRead]

	// Reconstruct the body so the full content is available for Decode.
	remainder := req.Body
	req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(peek), remainder))

	return peek, req, nil
}
