package a2a

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"iter"

	"github.com/mark3labs/mcp-go/mcp"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// Translator converts between MCP and A2A protocol data types.
// It provides deterministic, zero-cost translation without LLM involvement.
//
// The zero value is ready to use with default settings.
type Translator struct {
	// SkillTags are default tags appended to every generated skill.
	SkillTags []string
}

// ToolToSkill converts an mcpkit ToolDefinition into an A2A AgentSkill
// for inclusion in an agent card.
//
// Mapping rules:
//   - Tool.Name      -> Skill.ID and Skill.Name
//   - Tool.Description -> Skill.Description
//   - Category        -> first tag
//   - Tags            -> appended tags
//   - IsWrite         -> "write" or "read" tag
//   - InputSchema     -> embedded in examples as JSON for programmatic clients
func (t *Translator) ToolToSkill(td registry.ToolDefinition) a2atypes.AgentSkill {
	skill := a2atypes.AgentSkill{
		ID:          td.Tool.Name,
		Name:        td.Tool.Name,
		Description: td.Tool.Description,
		InputModes:  []string{"application/json"},
		OutputModes: []string{"application/json", "text/plain"},
	}

	// Build tags from category, tool tags, read/write, and default skill tags.
	var tags []string
	if td.Category != "" {
		tags = append(tags, td.Category)
	}
	tags = append(tags, td.Tags...)
	if td.IsWrite {
		tags = append(tags, "write")
	} else {
		tags = append(tags, "read")
	}
	tags = append(tags, t.SkillTags...)
	skill.Tags = tags

	// Embed input schema as an example for programmatic A2A clients.
	// This follows the schema-in-metadata strategy: the full MCP JSON Schema
	// is serialized into the examples field so A2A clients can construct
	// valid tool calls without LLM extraction.
	schemaJSON := marshalInputSchema(td.Tool.InputSchema)
	if schemaJSON != "" {
		skill.Examples = []string{schemaJSON}
	}

	return skill
}

// CallResultToArtifact converts an mcpkit CallToolResult into an A2A Artifact.
// Each MCP content block maps to an A2A Part:
//   - TextContent  -> TextPart
//   - ImageContent -> RawPart with mediaType
//   - Other        -> DataPart with JSON representation
//
// If the result is nil or has no content, an empty artifact is returned.
// If the result is marked as an error (IsError), the artifact contains the
// error text as a text part.
func (t *Translator) CallResultToArtifact(result *registry.CallToolResult) a2atypes.Artifact {
	if result == nil {
		return a2atypes.Artifact{
			ID:    a2atypes.NewArtifactID(),
			Parts: a2atypes.ContentParts{},
		}
	}

	var parts []*a2atypes.Part

	for _, content := range result.Content {
		part := contentToPart(content)
		if part != nil {
			parts = append(parts, part)
		}
	}

	// Guarantee non-nil parts slice for valid JSON serialization.
	if parts == nil {
		parts = []*a2atypes.Part{}
	}

	artifact := a2atypes.Artifact{
		ID:    a2atypes.NewArtifactID(),
		Parts: parts,
	}

	if result.IsError {
		artifact.Description = "error"
	}

	return artifact
}

// ErrorToTaskStatus converts an MCP error code and message into an A2A TaskStatus
// with the appropriate failure state.
//
// Error code mapping:
//   - handler.ErrInvalidParam   -> TASK_STATE_FAILED
//   - handler.ErrNotFound       -> TASK_STATE_FAILED
//   - handler.ErrPermission     -> TASK_STATE_REJECTED
//   - handler.ErrTimeout        -> TASK_STATE_FAILED
//   - handler.ErrRateLimited    -> TASK_STATE_FAILED
//   - (any other)               -> TASK_STATE_FAILED
func (t *Translator) ErrorToTaskStatus(code string, msg string) a2atypes.TaskStatus {
	state := a2atypes.TaskStateFailed

	// Permission denied maps to REJECTED (agent refuses the task).
	if code == handler.ErrPermission {
		state = a2atypes.TaskStateRejected
	}

	// Build a status message with the error details.
	statusMsg := a2atypes.NewMessage(
		a2atypes.MessageRoleAgent,
		a2atypes.NewTextPart(fmt.Sprintf("[%s] %s", code, msg)),
	)

	return a2atypes.TaskStatus{
		State:   state,
		Message: statusMsg,
	}
}

// MessageToCallToolRequest extracts the tool name and arguments from an A2A
// Message for dispatching as an MCP tool call.
//
// Strategy 1 (preferred): Look for a DataPart containing {"skill": "...", "arguments": {...}}.
// Strategy 2 (fallback): If a skill is provided, use the first TextPart as a JSON argument source.
//
// Returns the tool name, the argument map, and any error.
func (t *Translator) MessageToCallToolRequest(msg a2atypes.Message, skill a2atypes.AgentSkill) (string, map[string]any, error) {
	// Strategy 1: DataPart with structured arguments.
	for _, part := range msg.Parts {
		if part == nil {
			continue
		}
		data := part.Data()
		if data == nil {
			continue
		}

		// The data should be a map with "skill" and "arguments" keys.
		dataMap, ok := toStringMap(data)
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

	// Strategy 2: Use the provided skill ID and try to parse the first TextPart as JSON args.
	if skill.ID != "" {
		for _, part := range msg.Parts {
			if part == nil {
				continue
			}
			text := part.Text()
			if text == "" {
				continue
			}

			// Try to parse the text as JSON arguments.
			var args map[string]any
			if err := json.Unmarshal([]byte(text), &args); err == nil {
				return skill.ID, args, nil
			}

			// Non-JSON text: wrap as a single "input" argument.
			return skill.ID, map[string]any{"input": text}, nil
		}

		// No parts with content, return skill ID with empty args.
		return skill.ID, make(map[string]any), nil
	}

	return "", nil, errors.New("no skill identifier found in message: expected DataPart with 'skill' field or non-empty skill hint")
}

// contentToPart converts a single MCP content block to an A2A Part.
func contentToPart(content registry.Content) *a2atypes.Part {
	if content == nil {
		return nil
	}

	switch v := content.(type) {
	case mcp.TextContent:
		return a2atypes.NewTextPart(v.Text)

	case mcp.ImageContent:
		// Decode base64 image data to raw bytes for A2A RawPart.
		raw, err := base64.StdEncoding.DecodeString(v.Data)
		if err != nil {
			// If decoding fails, fall back to text with the base64 string.
			return a2atypes.NewTextPart(v.Data)
		}
		part := a2atypes.NewRawPart(raw)
		part.MediaType = v.MIMEType
		return part

	case mcp.EmbeddedResource:
		// Serialize the embedded resource as structured data.
		return a2atypes.NewDataPart(v.Resource)

	default:
		// Unknown content type: attempt JSON serialization as data.
		return a2atypes.NewDataPart(content)
	}
}

// marshalInputSchema serializes a ToolInputSchema to a JSON string for
// embedding in skill examples. Returns empty string on error.
func marshalInputSchema(schema registry.ToolInputSchema) string {
	// Build a clean schema representation.
	schemaMap := map[string]any{
		"type": schema.Type,
	}
	if len(schema.Properties) > 0 {
		schemaMap["properties"] = schema.Properties
	}
	if len(schema.Required) > 0 {
		schemaMap["required"] = schema.Required
	}
	if schema.AdditionalProperties != nil {
		schemaMap["additionalProperties"] = schema.AdditionalProperties
	}

	data, err := json.Marshal(schemaMap)
	if err != nil {
		return ""
	}
	return string(data)
}

// CallResultToEvents converts an mcpkit CallToolResult and optional error into
// an iterator of A2A events for the AgentExecutor return type. This synthesizes
// the task lifecycle around a synchronous tool result:
//
//   - On success: ArtifactUpdateEvent + StatusUpdateEvent(COMPLETED)
//   - On error result: StatusUpdateEvent(FAILED) with error message
//   - On handler error: StatusUpdateEvent(FAILED) with error message
func (t *Translator) CallResultToEvents(
	taskInfo a2atypes.TaskInfo,
	result *registry.CallToolResult,
	err error,
) iter.Seq2[a2atypes.Event, error] {
	return func(yield func(a2atypes.Event, error) bool) {
		// Handle handler-level errors (panics, context cancellation, etc.).
		if err != nil {
			errMsg := a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, taskInfo,
				a2atypes.NewTextPart(fmt.Sprintf("[%s] %s", handler.ErrInternal, err.Error())),
			)
			yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateFailed, errMsg), nil)
			return
		}

		// Handle error results (tool returned IsError: true).
		if registry.IsResultError(result) {
			var errText string
			if result != nil && len(result.Content) > 0 {
				if text, ok := registry.ExtractTextContent(result.Content[0]); ok {
					errText = text
				}
			}
			if errText == "" {
				errText = "tool returned an error"
			}
			errMsg := a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, taskInfo,
				a2atypes.NewTextPart(errText),
			)
			yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateFailed, errMsg), nil)
			return
		}

		// Success path: emit artifact then completed status.
		artifact := t.CallResultToArtifact(result)
		artifactEvent := &a2atypes.TaskArtifactUpdateEvent{
			ContextID: taskInfo.ContextID,
			TaskID:    taskInfo.TaskID,
			Artifact:  &artifact,
			LastChunk: true,
		}
		if !yield(artifactEvent, nil) {
			return
		}

		yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateCompleted, nil), nil)
	}
}

// BuildCallToolRequest constructs a registry.CallToolRequest from a tool name
// and argument map, suitable for passing to a tool handler.
func BuildCallToolRequest(toolName string, args map[string]any) registry.CallToolRequest {
	return registry.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	}
}

// toStringMap attempts to convert an any value to map[string]any.
// Handles both direct map[string]any and map[string]interface{} (from JSON).
func toStringMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	return nil, false
}
