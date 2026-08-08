//go:build !official_sdk

// compat.go — MCP SDK compatibility / migration adapter layer (mcp-go variant).
//
// Current SDK:  github.com/mark3labs/mcp-go
// Target SDK:   github.com/modelcontextprotocol/go-sdk (when stable)
//
// When the official SDK ships, the official_sdk build tag activates compat_official.go
// instead of this file. Tool modules that import types through mcpkit need zero changes.
package registry

import (
	"encoding/base64"
	"errors"

	"github.com/mark3labs/mcp-go/mcp"
)

type (
	Tool             = mcp.Tool
	CallToolRequest  = mcp.CallToolRequest
	CallToolResult   = mcp.CallToolResult
	ToolInputSchema  = mcp.ToolInputSchema
	ToolOutputSchema = mcp.ToolOutputSchema
	ToolAnnotation   = mcp.ToolAnnotation
	ToolMeta         = mcp.Meta
	TextContent      = mcp.TextContent
	Content          = mcp.Content
	TaskStatus       = mcp.TaskStatus
	Task             = mcp.Task
	TaskSupport      = mcp.TaskSupport
	ToolExecution    = mcp.ToolExecution

	// Resource types
	Resource             = mcp.Resource
	ResourceTemplate     = mcp.ResourceTemplate
	ResourceContents     = mcp.ResourceContents
	TextResourceContents = mcp.TextResourceContents
	BlobResourceContents = mcp.BlobResourceContents
	ReadResourceRequest  = mcp.ReadResourceRequest
	ReadResourceResult   = mcp.ReadResourceResult
	// Annotations carries resource audience/priority hints (Resource.Annotations).
	// A plain alias suffices for the struct/field itself (both SDKs' fields
	// have the same names), but the two SDKs' Priority field type differs
	// (mcp-go: *float64; official: float64) — see MakeResourceAnnotations
	// for the constructor that papers over that.
	Annotations = mcp.Annotations

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
	Root             = mcp.Root
	ListRootsRequest = mcp.ListRootsRequest
	ListRootsResult  = mcp.ListRootsResult
)

var (
	NewToolResultText  = mcp.NewToolResultText
	NewToolResultError = mcp.NewToolResultError

	// Resource constructors
	NewResource         = mcp.NewResource
	NewResourceTemplate = mcp.NewResourceTemplate

	// Prompt constructors
	NewPrompt        = mcp.NewPrompt
	NewPromptMessage = mcp.NewPromptMessage
	NewTextContent   = mcp.NewTextContent
)

// ProgressTokenFromRequest returns the client-supplied progress token from
// req's _meta field, or nil if none was provided. mcp-go's Meta is a struct
// with a typed ProgressToken field; the official SDK's Meta is a plain
// map[string]any with no such field, and go-sdk v1.7.0 does not yet expose
// per-session progress notifications at all (see
// registry.ServerProgressReporter's doc comment in progress_server_official.go
// — an existing, already-accepted no-op), so this always returns nil there
// (compat_official.go's ProgressTokenFromRequest) rather than a new gap.
// Consumer code that reaches into req.Params.Meta.ProgressToken directly
// (mcp-go-specific) should go through this accessor instead.
func ProgressTokenFromRequest(req CallToolRequest) any {
	if req.Params.Meta == nil {
		return nil
	}
	return req.Params.Meta.ProgressToken
}

// TemplateURI returns the raw URI template string from a ResourceTemplate.
// mcp-go's ResourceTemplate.URITemplate is a parsed *mcp.URITemplate (a
// uritemplate.Template wrapper with a .Raw() method), not a plain string —
// unlike the official SDK's, which is (see compat_official.go's
// TemplateURI). Consumer code that needs the template string for display or
// sorting (e.g. secretstudios-mcp's internal/surface/server_catalog.go)
// should go through this accessor instead of reaching into .URITemplate.Raw()
// directly, which does not compile under official_sdk. Returns "" for a
// zero-value/nil URITemplate.
func TemplateURI(tpl ResourceTemplate) string {
	if tpl.URITemplate == nil {
		return ""
	}
	return tpl.URITemplate.Raw()
}

// MakeResourceAnnotations constructs Resource/ResourceTemplate audience and
// priority hints (the Annotations struct assigned to Resource.Annotations).
// mcp-go's Annotations.Priority is *float64 (nil means unset); the official
// SDK's is a plain float64 (see compat_official.go's counterpart) — this
// constructor takes a plain priority and always sets it, so a caller that
// wants "unset" should not call this at all and leave Resource.Annotations
// nil, rather than passing a zero priority (both SDKs' JSON encoding omits
// a zero-valued priority via `omitempty` either way, so a zero priority
// here and an unset one are wire-equivalent, just not construction-equivalent).
func MakeResourceAnnotations(audience []Role, priority float64) *Annotations {
	return &mcp.Annotations{Audience: audience, Priority: &priority}
}

// MakePrompt constructs a Prompt with SDK-neutral PromptArgument values.
// mcp-go's mcp.Prompt.Arguments is []mcp.PromptArgument — a value slice that
// PromptArgument (aliased to mcp.PromptArgument) matches directly — so args
// is assigned as-is. See compat_official.go's MakePrompt for the pointer-
// slice counterpart this exists to paper over: consumer code that builds a
// Prompt with `Arguments: args` from a `...PromptArgument` variadic (e.g.
// secretstudios-mcp's internal/surface/prompts.go newPrompt helper) does not
// compile under official_sdk without going through this constructor instead.
func MakePrompt(name, description string, args ...PromptArgument) Prompt {
	return mcp.Prompt{Name: name, Description: description, Arguments: args}
}

// PromptArguments returns p's arguments as a value slice — the read-back
// mirror of MakePrompt. mcp-go's Prompt.Arguments is already
// []mcp.PromptArgument, so this returns it as-is (nil-safe: a nil slice
// stays nil, never allocated into an empty non-nil one). See
// compat_official.go's PromptArguments for the pointer-slice-dereferencing
// counterpart.
func PromptArguments(p Prompt) []PromptArgument {
	return p.Arguments
}

// OutputSchemaType returns t.OutputSchema.Type, or ("", false) if unset.
// mcp-go's Tool.OutputSchema is a value ToolOutputSchema struct; the
// official SDK's is `any` holding a map[string]any — see
// compat_official.go's OutputSchemaType for that side's type assertion.
func OutputSchemaType(t Tool) (string, bool) {
	if t.OutputSchema.Type == "" {
		return "", false
	}
	return t.OutputSchema.Type, true
}

// OutputSchemaProperties returns t.OutputSchema.Properties, or (nil, false)
// if unset/empty. See compat_official.go's OutputSchemaProperties for the
// official SDK's map-assertion counterpart.
func OutputSchemaProperties(t Tool) (map[string]any, bool) {
	if len(t.OutputSchema.Properties) == 0 {
		return nil, false
	}
	return t.OutputSchema.Properties, true
}

// InputSchemaType returns schema.Type, or ("", false) if unset. mcp-go's
// ToolInputSchema is a value struct; the official SDK's is `any` holding a
// map[string]any — see compat_official.go's InputSchemaType for that side's
// type assertion. Added for bridge/a2a's translator.go, which needs to
// re-serialize an existing Tool's input schema (e.g. into an A2A skill
// example) without knowing which SDK built it.
func InputSchemaType(schema ToolInputSchema) (string, bool) {
	if schema.Type == "" {
		return "", false
	}
	return schema.Type, true
}

// InputSchemaProperties returns schema.Properties, or (nil, false) if
// unset/empty. See compat_official.go's InputSchemaProperties.
func InputSchemaProperties(schema ToolInputSchema) (map[string]any, bool) {
	if len(schema.Properties) == 0 {
		return nil, false
	}
	return schema.Properties, true
}

// InputSchemaRequired returns schema.Required, or (nil, false) if
// unset/empty. See compat_official.go's InputSchemaRequired.
func InputSchemaRequired(schema ToolInputSchema) ([]string, bool) {
	if len(schema.Required) == 0 {
		return nil, false
	}
	return schema.Required, true
}

// InputSchemaAdditionalProperties returns schema.AdditionalProperties, or
// (nil, false) if unset. See compat_official.go's
// InputSchemaAdditionalProperties.
func InputSchemaAdditionalProperties(schema ToolInputSchema) (any, bool) {
	if schema.AdditionalProperties == nil {
		return nil, false
	}
	return schema.AdditionalProperties, true
}

// MakeToolInputSchema constructs a ToolInputSchema (object type) from a
// properties map, a required-fields list, and an optional
// additionalProperties value (pass nil to omit it). mcp-go's ToolInputSchema
// is a typed struct; this builds one directly. See compat_official.go's
// MakeToolInputSchema for the official SDK's map[string]any counterpart.
// Added so consumer code that needs to hand-build a simple tool schema
// (bridge/a2a's remote_agent.go: wrapping a remote A2A skill as an MCP
// tool) doesn't need mcp-go's functional-option Tool builder
// (mcp.NewTool/mcp.WithString/...), which has no compat-layer equivalent.
func MakeToolInputSchema(properties map[string]any, required []string, additionalProperties any) ToolInputSchema {
	return mcp.ToolInputSchema(mcp.ToolArgumentsSchema{
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		AdditionalProperties: additionalProperties,
	})
}

// ExtractImageContent returns the raw (already base64-decoded) image bytes
// and MIME type if c is image content, or (nil, "", false) otherwise —
// including when c is image content whose Data fails to base64-decode
// (mcp-go's ImageContent.Data is a base64 string; decoding here means
// callers never need to know that — see compat_official.go's
// ExtractImageContent, where the official SDK's Data is already []byte).
func ExtractImageContent(c Content) (data []byte, mimeType string, ok bool) {
	ic, isImage := c.(mcp.ImageContent)
	if !isImage {
		return nil, "", false
	}
	raw, err := base64.StdEncoding.DecodeString(ic.Data)
	if err != nil {
		return nil, "", false
	}
	return raw, ic.MIMEType, true
}

// MakeImageContent constructs a Content value from raw image bytes and a
// MIME type — the make-side counterpart to ExtractImageContent. mcp-go's
// ImageContent.Data is a base64 string, so data is encoded here; the
// official SDK's is already []byte (see compat_official.go's
// MakeImageContent).
func MakeImageContent(data []byte, mimeType string) Content {
	return mcp.ImageContent{
		Type:     "image",
		Data:     base64.StdEncoding.EncodeToString(data),
		MIMEType: mimeType,
	}
}

// ExtractEmbeddedResource returns the wrapped resource value if c is
// embedded-resource content, as an opaque any suitable for JSON marshaling
// by the caller — the underlying resource-contents shape differs per SDK
// (mcp-go: mcp.ResourceContents, an interface implemented by
// TextResourceContents/BlobResourceContents; official: *mcp.ResourceContents,
// a single struct — see compat_official.go's ExtractEmbeddedResource), so
// this deliberately does not attempt to normalize it further than "the
// thing the caller can json.Marshal".
func ExtractEmbeddedResource(c Content) (resource any, ok bool) {
	er, isEmbedded := c.(mcp.EmbeddedResource)
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
	return mcp.EmbeddedResource{
		Type: "resource",
		Resource: mcp.TextResourceContents{
			URI:      uri,
			MIMEType: mimeType,
			Text:     text,
		},
	}
}

// MakeTextContent constructs a Content value containing text.
// In mcp-go this is a value type; in the official SDK it would be a pointer.
func MakeTextContent(text string) Content {
	return mcp.TextContent{Type: "text", Text: text}
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

// ExtractTextContent extracts the text from a Content value if it is a TextContent.
// Returns the text and true if successful, or empty string and false otherwise.
func ExtractTextContent(c Content) (string, bool) {
	tc, ok := c.(mcp.TextContent)
	if !ok {
		return "", false
	}
	return tc.Text, true
}

// ExtractResourceText extracts the text from the first resource content in a ReadResourceResult.
// Returns the text and true if the first content is a TextResourceContents.
func ExtractResourceText(result *ReadResourceResult) (string, bool) {
	if result == nil || len(result.Contents) == 0 {
		return "", false
	}
	tc, ok := result.Contents[0].(mcp.TextResourceContents)
	if !ok {
		return "", false
	}
	return tc.Text, true
}

// ExtractArguments returns the tool arguments as map[string]interface{}.
// In mcp-go, Arguments is type `any` and needs a type assertion.
// In the official SDK, Arguments is json.RawMessage and needs unmarshaling.
func ExtractArguments(req CallToolRequest) map[string]any {
	if req.Params.Arguments == nil {
		return nil
	}
	args, ok := req.Params.Arguments.(map[string]any)
	if !ok {
		return nil
	}
	return args
}

// NewCallToolRequest constructs a CallToolRequest with SDK-compatible arguments.
func NewCallToolRequest(name string, args map[string]any) (CallToolRequest, error) {
	req := CallToolRequest{}
	req.Params.Name = name
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
	req.Params.Arguments = args
	return nil
}

// GetToolTaskSupport returns the TaskSupport setting from a Tool, or TaskSupportForbidden if not set.
func GetToolTaskSupport(tool Tool) TaskSupport {
	if tool.Execution == nil {
		return TaskSupportForbidden
	}
	return tool.Execution.TaskSupport
}

// HasTaskParams returns true if the request includes task augmentation params.
func HasTaskParams(req CallToolRequest) bool {
	return req.Params.Task != nil
}

// ExtractTaskTTL returns the task TTL from the request, or 0 if not specified.
func ExtractTaskTTL(req CallToolRequest) int64 {
	if req.Params.Task == nil || req.Params.Task.TTL == nil {
		return 0
	}
	return *req.Params.Task.TTL
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
		tool.Meta = &ToolMeta{AdditionalFields: map[string]any{}}
	}
	if tool.Meta.AdditionalFields == nil {
		tool.Meta.AdditionalFields = map[string]any{}
	}
	tool.Meta.AdditionalFields[key] = value
}

// ToolMetaField reads a metadata field previously set by SetToolMetaField —
// the read mirror of that function. mcp-go stores fields under
// tool.Meta.AdditionalFields[key]; see compat_official.go's ToolMetaField
// for the official SDK's plain-map counterpart. Returns (nil, false) if
// tool.Meta, its AdditionalFields, or the key itself is absent.
func ToolMetaField(t Tool, key string) (any, bool) {
	if t.Meta == nil || t.Meta.AdditionalFields == nil {
		return nil, false
	}
	v, ok := t.Meta.AdditionalFields[key]
	return v, ok
}

// RequestMetaCarrier returns req's _meta additional fields as a string-keyed
// carrier suitable for OTel propagation.Extract (e.g. trace-context bridging
// in observability.Provider), or nil if req carries no _meta / no string
// values. mcp-go's CallToolRequest.Params.Meta is a *mcp.Meta struct with an
// AdditionalFields map; see compat_official.go's RequestMetaCarrier for the
// official SDK's plain-map counterpart, which has no such sub-field.
// Consumer code that reaches into req.Params.Meta.AdditionalFields directly
// (mcp-go-specific) should go through this accessor instead.
func RequestMetaCarrier(req CallToolRequest) map[string]string {
	if req.Params.Meta == nil || req.Params.Meta.AdditionalFields == nil {
		return nil
	}
	carrier := make(map[string]string, len(req.Params.Meta.AdditionalFields))
	for k, v := range req.Params.Meta.AdditionalFields {
		if s, ok := v.(string); ok {
			carrier[k] = s
		}
	}
	return carrier
}

// SetResultMetaCarrier merges carrier's key/value pairs into result's _meta
// additional fields, allocating result.Meta (and its AdditionalFields map)
// if necessary. No-op if result is nil or carrier is empty. See
// compat_official.go's SetResultMetaCarrier for the official SDK's
// plain-map counterpart.
func SetResultMetaCarrier(result *CallToolResult, carrier map[string]string) {
	if result == nil || len(carrier) == 0 {
		return
	}
	if result.Meta == nil {
		result.Meta = &ToolMeta{}
	}
	if result.Meta.AdditionalFields == nil {
		result.Meta.AdditionalFields = make(map[string]any, len(carrier))
	}
	for k, v := range carrier {
		result.Meta.AdditionalFields[k] = v
	}
}

// SetToolDeferLoading sets the SDK's defer_loading flag when supported.
func SetToolDeferLoading(tool *Tool, deferred bool) {
	tool.DeferLoading = deferred
}

// SetToolTitle sets the top-level Tool.Title field when the SDK supports it.
// In mcp-go v0.54.1+ this is a first-class field. For older SDK versions that
// lack the field, this is a no-op (handled by the SDK-specific compat file).
func SetToolTitle(tool *Tool, title string) {
	if tool.Title == "" {
		tool.Title = title
	}
}

// Task status constants re-exported for convenience.
const (
	TaskStatusWorking       = mcp.TaskStatusWorking
	TaskStatusInputRequired = mcp.TaskStatusInputRequired
	TaskStatusCompleted     = mcp.TaskStatusCompleted
	TaskStatusFailed        = mcp.TaskStatusFailed
	TaskStatusCancelled     = mcp.TaskStatusCancelled

	TaskSupportForbidden = mcp.TaskSupportForbidden
	TaskSupportOptional  = mcp.TaskSupportOptional
	TaskSupportRequired  = mcp.TaskSupportRequired

	RoleUser      = mcp.RoleUser
	RoleAssistant = mcp.RoleAssistant
)
