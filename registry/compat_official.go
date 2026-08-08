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

// ProgressTokenFromRequest extracts the request's progress token, if any,
// via CallToolParamsRaw.GetProgressToken() (mcp/protocol.go). go-sdk v1.7.0
// does support per-session progress notifications end to end — see
// progress_server_official.go — so unlike an earlier version of this
// comment claimed, there is no SDK limitation here to work around.
func ProgressTokenFromRequest(req CallToolRequest) any {
	if req.Params == nil {
		return nil
	}
	return req.Params.GetProgressToken()
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

// InputSchemaType returns the "type" field of schema, or ("", false) if
// schema isn't a map[string]any or has no non-empty "type" entry. See
// compat.go's InputSchemaType for the mcp-go side.
func InputSchemaType(schema ToolInputSchema) (string, bool) {
	m, ok := schema.(map[string]any)
	if !ok {
		return "", false
	}
	typ, ok := m["type"].(string)
	if !ok || typ == "" {
		return "", false
	}
	return typ, true
}

// InputSchemaProperties returns the "properties" field of schema, or (nil,
// false) if absent/empty. See compat.go's InputSchemaProperties.
func InputSchemaProperties(schema ToolInputSchema) (map[string]any, bool) {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil, false
	}
	props, ok := m["properties"].(map[string]any)
	if !ok || len(props) == 0 {
		return nil, false
	}
	return props, true
}

// InputSchemaRequired returns the "required" field of schema as []string,
// or (nil, false) if absent/empty. Accepts either []string or []any (the
// shape produced by decoding a hand-built map vs one round-tripped through
// JSON) since a caller may have constructed schema either way. See
// compat.go's InputSchemaRequired.
func InputSchemaRequired(schema ToolInputSchema) ([]string, bool) {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil, false
	}
	switch req := m["required"].(type) {
	case []string:
		if len(req) == 0 {
			return nil, false
		}
		return req, true
	case []any:
		out := make([]string, 0, len(req))
		for _, v := range req {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	}
	return nil, false
}

// InputSchemaAdditionalProperties returns the "additionalProperties" field
// of schema, or (nil, false) if absent. See compat.go's
// InputSchemaAdditionalProperties.
func InputSchemaAdditionalProperties(schema ToolInputSchema) (any, bool) {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil, false
	}
	v, ok := m["additionalProperties"]
	if !ok || v == nil {
		return nil, false
	}
	return v, true
}

// MakeToolInputSchema constructs a ToolInputSchema (object type) from a
// properties map, a required-fields list, and an optional
// additionalProperties value (pass nil to omit it). The official SDK's
// ToolInputSchema is `any`, holding a plain map[string]any here. See
// compat.go's MakeToolInputSchema for the mcp-go side.
func MakeToolInputSchema(properties map[string]any, required []string, additionalProperties any) ToolInputSchema {
	schema := map[string]any{"type": "object"}
	if len(properties) > 0 {
		schema["properties"] = properties
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	if additionalProperties != nil {
		schema["additionalProperties"] = additionalProperties
	}
	return schema
}

// ExtractImageContent returns the raw image bytes and MIME type if c is
// image content, or (nil, "", false) otherwise. The official SDK's
// ImageContent.Data is already []byte (mcp-go's is a base64 string that
// compat.go's counterpart decodes — see that doc comment), so no decoding
// is needed here.
func ExtractImageContent(c Content) (data []byte, mimeType string, ok bool) {
	ic, isImage := c.(*mcp.ImageContent)
	if !isImage {
		return nil, "", false
	}
	return ic.Data, ic.MIMEType, true
}

// MakeImageContent constructs a Content value from raw image bytes and a
// MIME type — the make-side counterpart to ExtractImageContent. See
// compat.go's MakeImageContent for the mcp-go side (which base64-encodes
// data into a string field instead).
func MakeImageContent(data []byte, mimeType string) Content {
	return &mcp.ImageContent{
		Data:     data,
		MIMEType: mimeType,
	}
}

// ExtractEmbeddedResource returns the wrapped resource value if c is
// embedded-resource content, as an opaque any suitable for JSON marshaling
// by the caller. The official SDK's EmbeddedResource.Resource is a single
// *mcp.ResourceContents struct (mcp-go's is an interface — see compat.go's
// ExtractEmbeddedResource), but this accessor deliberately returns it as
// `any` on both sides so callers stay SDK-agnostic.
func ExtractEmbeddedResource(c Content) (resource any, ok bool) {
	er, isEmbedded := c.(*mcp.EmbeddedResource)
	if !isEmbedded {
		return nil, false
	}
	return er.Resource, true
}

// MakeEmbeddedResourceText constructs embedded-resource Content wrapping a
// single text resource — the make-side counterpart to
// ExtractEmbeddedResource, covering the common case (a text resource, not
// binary) that consumer test fixtures need.
func MakeEmbeddedResourceText(uri, mimeType, text string) Content {
	return &mcp.EmbeddedResource{
		Resource: &mcp.ResourceContents{
			URI:      uri,
			MIMEType: mimeType,
			Text:     text,
		},
	}
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
