// Package multi implements a multi-protocol HTTP gateway for mcpkit.
//
// It defines the Adapter interface for translating between agent protocols
// (MCP, A2A, OpenAI function calling, etc.) and a canonical request/response
// model. Each protocol gets a single adapter that implements Detect, Decode,
// and Encode. The Router composes adapters and dispatches incoming HTTP
// requests to the correct one based on protocol auto-detection.
//
// This package is the multi-protocol superset of the single-protocol
// gateway package. Operators who only need MCP aggregation continue using
// the parent gateway package; those who need multi-protocol support import
// this one.
package multi

import "net/http"

// Protocol identifies a supported agent protocol.
type Protocol string

const (
	// ProtocolMCP is the Model Context Protocol (JSON-RPC over HTTP).
	ProtocolMCP Protocol = "mcp"

	// ProtocolA2A is the Agent-to-Agent protocol (JSON-RPC over HTTP).
	ProtocolA2A Protocol = "a2a"

	// ProtocolOpenAI is the OpenAI function calling format.
	ProtocolOpenAI Protocol = "openai"

	// ProtocolUnknown is returned when no protocol can be detected.
	ProtocolUnknown Protocol = "unknown"
)

// Confidence indicates how certain the detection layer is about a match.
type Confidence int

const (
	// ConfidenceLow means the detection is a guess (e.g., fallback default).
	ConfidenceLow Confidence = iota

	// ConfidenceMedium means partial signals match (e.g., Content-Type only).
	ConfidenceMedium

	// ConfidenceHigh means strong structural signals match (e.g., body shape).
	ConfidenceHigh

	// ConfidenceDefinitive means an unambiguous signal matched (e.g., JSON-RPC method).
	ConfidenceDefinitive
)

// String returns the human-readable name of the confidence level.
func (c Confidence) String() string {
	switch c {
	case ConfidenceLow:
		return "low"
	case ConfidenceMedium:
		return "medium"
	case ConfidenceHigh:
		return "high"
	case ConfidenceDefinitive:
		return "definitive"
	default:
		return "unknown"
	}
}

// Adapter translates between a specific agent protocol and the
// gateway's canonical data model. Each supported protocol has exactly
// one Adapter implementation.
//
// Contract:
//   - Detect must not consume the request body; it receives a pre-buffered peek.
//   - Decode may fully read the request body.
//   - Encode writes the HTTP response; the caller must not write after Encode returns.
//   - All methods must be safe for concurrent use.
type Adapter interface {
	// Protocol returns the identifier for this adapter.
	Protocol() Protocol

	// Detect inspects an HTTP request and returns whether this adapter
	// handles it and how confident the match is. The bodyPeek parameter
	// contains up to the first 512 bytes of the request body (pre-buffered
	// by the router). Adapters must not read from r.Body during detection.
	Detect(r *http.Request, bodyPeek []byte) (matches bool, confidence Confidence)

	// Decode translates a protocol-specific HTTP request into a
	// canonical CanonicalRequest. The request body is available for
	// full reading (it has been buffered by the router).
	Decode(r *http.Request) (*CanonicalRequest, error)

	// Encode translates a canonical CanonicalResponse into the protocol's
	// wire format. Returns the response body, content-type header, and
	// any encoding error.
	Encode(resp *CanonicalResponse) (body []byte, contentType string, err error)
}

// AuthContext carries caller identity extracted from the request.
type AuthContext struct {
	// Token is the raw bearer token or API key.
	Token string

	// Subject is the authenticated principal (e.g., user ID, service name).
	Subject string

	// Scopes lists the authorized scopes.
	Scopes []string
}

// CanonicalRequest is the protocol-agnostic representation of a tool call.
// Every adapter's Decode method produces one of these.
type CanonicalRequest struct {
	// Protocol that originated this request.
	Protocol Protocol

	// ToolName is the normalized tool name.
	ToolName string

	// Arguments is the tool input as a JSON-compatible map.
	Arguments map[string]any

	// RequestID is the caller-provided request identifier.
	// Mapped from JSON-RPC id, A2A task ID, OpenAI tool_call_id, etc.
	RequestID string

	// Auth carries the caller's identity, if authenticated.
	Auth *AuthContext

	// Metadata carries protocol-specific context that must survive
	// the round-trip but does not map to the canonical model.
	Metadata map[string]string
}

// ContentType classifies the kind of content in a response part.
type ContentType string

const (
	ContentTypeText  ContentType = "text"
	ContentTypeJSON  ContentType = "json"
	ContentTypeImage ContentType = "image"
	ContentTypeData  ContentType = "data"
)

// ContentPart represents a single piece of content in a response.
type ContentPart struct {
	Type     ContentType `json:"type"`
	Text     string      `json:"text,omitempty"`
	MimeType string      `json:"mimeType,omitempty"`
	Data     []byte      `json:"data,omitempty"`
	JSON     any         `json:"json,omitempty"`
}

// ErrorCode is a protocol-agnostic error classification.
type ErrorCode int

const (
	ErrNone          ErrorCode = iota
	ErrInvalidParams           // Bad or missing parameters.
	ErrNotFound                // Tool or resource not found.
	ErrUnauthorized            // Authentication required or failed.
	ErrForbidden               // Authenticated but not authorized.
	ErrRateLimit               // Rate limit exceeded.
	ErrTimeout                 // Handler timed out.
	ErrInternal                // Unexpected internal error.
)

// ErrorInfo carries error details in a canonical response.
type ErrorInfo struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// CanonicalResponse is the protocol-agnostic representation of a tool result.
// The router constructs one of these from the tool handler's output, and the
// adapter's Encode method translates it to the caller's wire format.
type CanonicalResponse struct {
	// Success indicates whether the tool invocation succeeded.
	Success bool

	// Content holds the result payload.
	Content []ContentPart

	// Error carries error details when Success is false.
	Error *ErrorInfo

	// RequestID matches the CanonicalRequest.RequestID.
	RequestID string

	// Metadata carries protocol-specific context for the response encoder.
	Metadata map[string]string
}
