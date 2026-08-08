//go:build official_sdk

// compat_official.go — MCP SDK compatibility layer (official go-sdk variant).
//
// SDK: github.com/modelcontextprotocol/go-sdk
//
// This file is activated by the official_sdk build tag and provides the same
// exported API as compat.go but backed by the official SDK types.
package registry

import (
	"encoding/json"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type (
	Tool            = mcp.Tool
	CallToolRequest = mcp.CallToolRequest
	CallToolResult  = mcp.CallToolResult
	ToolAnnotation  = mcp.ToolAnnotations // plural in official SDK
	ToolMeta        = mcp.Meta
	TextContent     = mcp.TextContent
	Content         = mcp.Content

	// Resource types
	Resource            = mcp.Resource
	ResourceTemplate    = mcp.ResourceTemplate
	ResourceContents    = mcp.ResourceContents
	ReadResourceRequest = mcp.ReadResourceRequest
	ReadResourceResult  = mcp.ReadResourceResult

	// Prompt types
	Prompt           = mcp.Prompt
	PromptArgument   = mcp.PromptArgument
	PromptMessage    = mcp.PromptMessage
	GetPromptRequest = mcp.GetPromptRequest
	GetPromptResult  = mcp.GetPromptResult
	Role             = mcp.Role

	// Sampling types
	CreateMessageRequest = mcp.CreateMessageRequest
	CreateMessageResult  = mcp.CreateMessageResult
	SamplingMessage      = mcp.SamplingMessage
	CreateMessageParams  = mcp.CreateMessageParams
	ModelPreferences     = mcp.ModelPreferences

	// Root types
	Root            = mcp.Root
	ListRootsParams = mcp.ListRootsParams
	ListRootsResult = mcp.ListRootsResult
)

// Note: The official SDK does not have separate ToolInputSchema/ToolOutputSchema
// types — both Tool.InputSchema and Tool.OutputSchema are typed as `any`.
// These wrapper types maintain API compatibility with mcpkit consumers.
type ToolInputSchema = any
type ToolOutputSchema = any

// Note: The official SDK does not have Task types. These are placeholder types
// for forward compatibility. They will be updated when the official SDK adds
// task support.
type TaskStatus string
type TaskSupport string

type Task struct {
	TaskId string
}

type ToolExecution struct {
	TaskSupport TaskSupport
}

// Task status constants (placeholder — official SDK does not yet support tasks).
const (
	TaskStatusWorking       TaskStatus = "working"
	TaskStatusInputRequired TaskStatus = "input_required"
	TaskStatusCompleted     TaskStatus = "completed"
	TaskStatusFailed        TaskStatus = "failed"
	TaskStatusCancelled     TaskStatus = "cancelled"

	TaskSupportForbidden TaskSupport = ""
	TaskSupportOptional  TaskSupport = "optional"
	TaskSupportRequired  TaskSupport = "required"

	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Note: The official SDK does not have TextResourceContents/BlobResourceContents.
// ResourceContents is a single struct with Text and Blob fields.
// These adapters maintain API compatibility.
type TextResourceContents = mcp.ResourceContents
type BlobResourceContents = mcp.ResourceContents

// ExtractResourceText extracts the text from the first resource content in a ReadResourceResult.
// Returns the text and true if the first content has text.
func ExtractResourceText(result *ReadResourceResult) (string, bool) {
	if result == nil || len(result.Contents) == 0 || result.Contents[0] == nil {
		return "", false
	}
	return result.Contents[0].Text, result.Contents[0].Text != ""
}

// ProgressTokenFromRequest always returns nil on the official_sdk build.
// go-sdk v1.7.0 does not yet expose per-session progress notifications (see
// ServerProgressReporter's doc comment in this file), so there is no
// progress token to extract — an existing, already-accepted limitation, not
// a new gap introduced by this accessor. See compat.go's ProgressTokenFromRequest
// for the mcp-go side, which does extract a real token.
func ProgressTokenFromRequest(req CallToolRequest) any {
	return nil
}

// TemplateURI returns the raw URI template string from a ResourceTemplate.
// The official SDK's ResourceTemplate.URITemplate is already a plain string
// (unlike mcp-go's parsed *mcp.URITemplate — see compat.go's TemplateURI),
// so this just returns the field directly.
func TemplateURI(tpl ResourceTemplate) string {
	return tpl.URITemplate
}

// MakePrompt constructs a Prompt with SDK-neutral PromptArgument values. The
// official SDK's mcp.Prompt.Arguments is []*mcp.PromptArgument — a pointer
// slice, unlike mcp-go's value slice (see compat.go's MakePrompt) — so each
// PromptArgument value is copied to its own address before assignment.
// PromptArgument itself stays a value-type alias on both builds; only this
// constructor's output shape differs.
func MakePrompt(name, description string, args ...PromptArgument) Prompt {
	var arguments []*mcp.PromptArgument
	if len(args) > 0 {
		arguments = make([]*mcp.PromptArgument, len(args))
		for i := range args {
			a := args[i]
			arguments[i] = &a
		}
	}
	return mcp.Prompt{Name: name, Description: description, Arguments: arguments}
}

// PromptArguments returns p's arguments as a value slice — the read-back
// mirror of MakePrompt. The official SDK's Prompt.Arguments is
// []*mcp.PromptArgument (a pointer slice, unlike mcp-go's value slice — see
// compat.go's PromptArguments), so each element is dereferenced into the
// result. Nil-safe: a nil Arguments slice returns nil, not an
// allocated-but-empty one, matching the mcp-go side's behavior. A nil
// element within Arguments (should not occur from any of this package's own
// constructors) is skipped rather than causing a nil dereference.
func PromptArguments(p Prompt) []PromptArgument {
	if p.Arguments == nil {
		return nil
	}
	args := make([]PromptArgument, 0, len(p.Arguments))
	for _, a := range p.Arguments {
		if a == nil {
			continue
		}
		args = append(args, *a)
	}
	return args
}

// OutputSchemaType returns the "type" field of t.OutputSchema, or ("", false)
// if t.OutputSchema isn't a map[string]any or has no non-empty "type" entry.
// The official SDK's Tool.OutputSchema is `any` (mcp-go's is a typed
// ToolOutputSchema struct — see compat.go's OutputSchemaType), holding
// whatever TypedHandler's generateSchemaMap (or a consumer's own hand-built
// schema) produced.
func OutputSchemaType(t Tool) (string, bool) {
	m, ok := t.OutputSchema.(map[string]any)
	if !ok {
		return "", false
	}
	typ, ok := m["type"].(string)
	if !ok || typ == "" {
		return "", false
	}
	return typ, true
}

// OutputSchemaProperties returns the "properties" field of t.OutputSchema, or
// (nil, false) if t.OutputSchema isn't a map[string]any or "properties" is
// absent/empty. See compat.go's OutputSchemaProperties for the mcp-go side.
func OutputSchemaProperties(t Tool) (map[string]any, bool) {
	m, ok := t.OutputSchema.(map[string]any)
	if !ok {
		return nil, false
	}
	props, ok := m["properties"].(map[string]any)
	if !ok || len(props) == 0 {
		return nil, false
	}
	return props, true
}

// MakeTextContent constructs a Content value containing text.
func MakeTextContent(text string) Content {
	return &mcp.TextContent{Text: text}
}

// MakeErrorResult creates a CallToolResult marked as an error with text content.
func MakeErrorResult(text string) *CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{MakeTextContent(text)},
		IsError: true,
	}
}

// MakeTextResult creates a CallToolResult with text content.
func MakeTextResult(text string) *CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{MakeTextContent(text)},
	}
}

// IsResultError returns true if the result is marked as an error.
func IsResultError(r *CallToolResult) bool {
	if r == nil {
		return false
	}
	return r.IsError
}

// ExtractArguments returns the tool arguments as map[string]interface{}.
// In the official SDK, Arguments is json.RawMessage and needs unmarshaling.
func ExtractArguments(req CallToolRequest) map[string]interface{} {
	if req.Params == nil || len(req.Params.Arguments) == 0 {
		return nil
	}
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil
	}
	return args
}

// NewCallToolRequest constructs a CallToolRequest with SDK-compatible arguments.
func NewCallToolRequest(name string, args map[string]any) (CallToolRequest, error) {
	req := CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: name},
	}
	if err := SetCallToolArguments(&req, args); err != nil {
		return CallToolRequest{}, err
	}
	return req, nil
}

// SetCallToolArguments stores arguments on a CallToolRequest.
func SetCallToolArguments(req *CallToolRequest, args map[string]any) error {
	if req == nil {
		return errors.New("registry: nil CallToolRequest")
	}
	if req.Params == nil {
		req.Params = &mcp.CallToolParamsRaw{}
	}
	if args == nil {
		req.Params.Arguments = nil
		return nil
	}
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	req.Params.Arguments = data
	return nil
}

// GetToolTaskSupport returns the TaskSupport setting from a Tool.
// The official SDK does not support tasks, so this always returns TaskSupportForbidden.
func GetToolTaskSupport(_ Tool) TaskSupport {
	return TaskSupportForbidden
}

// HasTaskParams returns true if the request includes task augmentation params.
// The official SDK does not support tasks, so this always returns false.
func HasTaskParams(_ CallToolRequest) bool {
	return false
}

// ExtractTaskTTL returns the task TTL from the request.
// The official SDK does not support tasks, so this always returns 0.
func ExtractTaskTTL(_ CallToolRequest) int64 {
	return 0
}

// MakeStructuredResult creates a CallToolResult with both structured content
// and a text representation.
func MakeStructuredResult(content Content, data any) *CallToolResult {
	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: data,
	}
}

// SetToolMetaField stores an additional metadata field on a tool descriptor.
func SetToolMetaField(tool *Tool, key string, value any) {
	if tool.Meta == nil {
		tool.Meta = ToolMeta{}
	}
	tool.Meta[key] = value
}

// ToolMetaField reads a metadata field previously set by SetToolMetaField —
// the read mirror of that function. The official SDK stores fields directly
// in the plain-map Meta (unlike mcp-go's Meta.AdditionalFields sub-field —
// see compat.go's ToolMetaField). Returns (nil, false) if tool.Meta or the
// key itself is absent.
func ToolMetaField(t Tool, key string) (any, bool) {
	if t.Meta == nil {
		return nil, false
	}
	v, ok := t.Meta[key]
	return v, ok
}

// RequestMetaCarrier returns req's _meta fields as a string-keyed carrier
// suitable for OTel propagation.Extract (e.g. trace-context bridging in
// observability.Provider), or nil if req carries no _meta / no string
// values. The official SDK's CallToolRequest.Params.Meta is a plain
// map[string]any (mcp-go's is a *mcp.Meta struct with an AdditionalFields
// sub-field — see compat.go's RequestMetaCarrier), so this reads the map
// directly.
func RequestMetaCarrier(req CallToolRequest) map[string]string {
	if req.Params == nil || req.Params.Meta == nil {
		return nil
	}
	carrier := make(map[string]string, len(req.Params.Meta))
	for k, v := range req.Params.Meta {
		if s, ok := v.(string); ok {
			carrier[k] = s
		}
	}
	return carrier
}

// SetResultMetaCarrier merges carrier's key/value pairs into result's _meta
// map, allocating it if necessary. No-op if result is nil or carrier is
// empty. See compat.go's SetResultMetaCarrier for the mcp-go side's
// AdditionalFields-sub-field counterpart.
func SetResultMetaCarrier(result *CallToolResult, carrier map[string]string) {
	if result == nil || len(carrier) == 0 {
		return
	}
	if result.Meta == nil {
		result.Meta = ToolMeta{}
	}
	for k, v := range carrier {
		result.Meta[k] = v
	}
}

// SetToolDeferLoading is a no-op for the official SDK until it exposes an
// equivalent field on tool descriptors.
func SetToolDeferLoading(tool *Tool, deferred bool) {
	_ = tool
	_ = deferred
}

// SetToolTitle sets the top-level Tool.Title field when the SDK supports it.
func SetToolTitle(tool *Tool, title string) {
	if tool.Title == "" {
		tool.Title = title
	}
}

// ExtractTextContent extracts the text from a Content value if it is a TextContent.
func ExtractTextContent(c Content) (string, bool) {
	tc, ok := c.(*mcp.TextContent)
	if !ok {
		return "", false
	}
	return tc.Text, true
}
