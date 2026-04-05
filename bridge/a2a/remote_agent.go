package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// DefaultRemoteTimeout is the maximum duration for a single remote A2A tool call.
const DefaultRemoteTimeout = 60 * time.Second

// RemoteAgent wraps an A2A agent as an mcpkit ToolModule. Each agent skill
// becomes an MCP tool that, when invoked, sends an A2A SendMessage request
// to the remote agent and translates the response into an MCP CallToolResult.
type RemoteAgent struct {
	url        string
	card       *a2atypes.AgentCard
	client     *a2aclient.Client
	translator *Translator
	timeout    time.Duration
	prefix     string

	mu    sync.RWMutex
	tools []registry.ToolDefinition
}

// Verify interface compliance at compile time.
var _ registry.ToolModule = (*RemoteAgent)(nil)

// RemoteOption configures a RemoteAgent.
type RemoteOption func(*remoteOptions)

type remoteOptions struct {
	timeout    time.Duration
	prefix     string
	translator *Translator
	httpClient *agentcard.Resolver
	factoryOpts []a2aclient.FactoryOption
}

// WithRemoteTimeout sets the maximum duration for a single A2A tool call.
func WithRemoteTimeout(d time.Duration) RemoteOption {
	return func(o *remoteOptions) {
		o.timeout = d
	}
}

// WithRemotePrefix sets a prefix prepended to each skill ID when generating
// MCP tool names. For example, prefix "research" and skill "summarize" produces
// tool name "research_summarize".
func WithRemotePrefix(prefix string) RemoteOption {
	return func(o *remoteOptions) {
		o.prefix = prefix
	}
}

// WithRemoteTranslator overrides the default translator.
func WithRemoteTranslator(t *Translator) RemoteOption {
	return func(o *remoteOptions) {
		o.translator = t
	}
}

// WithRemoteFactoryOptions passes additional a2aclient.FactoryOption values
// to the underlying A2A client factory.
func WithRemoteFactoryOptions(opts ...a2aclient.FactoryOption) RemoteOption {
	return func(o *remoteOptions) {
		o.factoryOpts = append(o.factoryOpts, opts...)
	}
}

// NewRemoteAgent discovers an A2A agent at the given URL, fetches its AgentCard,
// creates an a2aclient.Client, and builds MCP tool definitions from the agent's
// skills. The context is used for the initial AgentCard fetch and client creation.
func NewRemoteAgent(ctx context.Context, url string, opts ...RemoteOption) (*RemoteAgent, error) {
	o := remoteOptions{
		timeout: DefaultRemoteTimeout,
	}
	for _, opt := range opts {
		opt(&o)
	}

	// Resolve the agent card from the well-known endpoint.
	resolver := o.httpClient
	if resolver == nil {
		resolver = agentcard.DefaultResolver
	}
	card, err := resolver.Resolve(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("a2a: failed to resolve agent card at %s: %w", url, err)
	}

	return newRemoteAgentFromCard(ctx, url, card, o)
}

// NewRemoteAgentFromCard creates a RemoteAgent from a pre-resolved AgentCard.
// This is useful when the card is obtained from a registry or configuration
// rather than HTTP discovery.
func NewRemoteAgentFromCard(ctx context.Context, card *a2atypes.AgentCard, opts ...RemoteOption) (*RemoteAgent, error) {
	if card == nil {
		return nil, errors.New("a2a: agent card must not be nil")
	}

	o := remoteOptions{
		timeout: DefaultRemoteTimeout,
	}
	for _, opt := range opts {
		opt(&o)
	}

	// Derive URL from the card's first supported interface.
	url := ""
	if len(card.SupportedInterfaces) > 0 {
		url = card.SupportedInterfaces[0].URL
	}

	return newRemoteAgentFromCard(ctx, url, card, o)
}

func newRemoteAgentFromCard(ctx context.Context, url string, card *a2atypes.AgentCard, o remoteOptions) (*RemoteAgent, error) {
	translator := o.translator
	if translator == nil {
		translator = &Translator{}
	}

	// Create the A2A client from the card.
	client, err := a2aclient.NewFromCard(ctx, card, o.factoryOpts...)
	if err != nil {
		return nil, fmt.Errorf("a2a: failed to create client for %s: %w", card.Name, err)
	}

	ra := &RemoteAgent{
		url:        url,
		card:       card,
		client:     client,
		translator: translator,
		timeout:    o.timeout,
		prefix:     o.prefix,
	}

	ra.buildTools()

	return ra, nil
}

// Name returns the module name derived from the AgentCard.
func (ra *RemoteAgent) Name() string {
	return ra.card.Name
}

// Description returns the module description derived from the AgentCard.
func (ra *RemoteAgent) Description() string {
	return ra.card.Description
}

// Tools returns MCP tool definitions for the remote agent.
// Each A2A skill becomes an MCP tool. Implements registry.ToolModule.
func (ra *RemoteAgent) Tools() []registry.ToolDefinition {
	ra.mu.RLock()
	defer ra.mu.RUnlock()
	result := make([]registry.ToolDefinition, len(ra.tools))
	copy(result, ra.tools)
	return result
}

// Close releases resources associated with the remote agent.
func (ra *RemoteAgent) Close() error {
	if ra.client != nil {
		return ra.client.Destroy()
	}
	return nil
}

// buildTools generates MCP ToolDefinition entries from the agent card's skills.
func (ra *RemoteAgent) buildTools() {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	ra.tools = make([]registry.ToolDefinition, 0, len(ra.card.Skills))

	for _, skill := range ra.card.Skills {
		td := ra.skillToToolDefinition(skill)
		ra.tools = append(ra.tools, td)
	}
}

// skillToToolDefinition converts an A2A AgentSkill into an MCP ToolDefinition.
func (ra *RemoteAgent) skillToToolDefinition(skill a2atypes.AgentSkill) registry.ToolDefinition {
	toolName := skill.ID
	if ra.prefix != "" {
		toolName = ra.prefix + "_" + skill.ID
	}

	// Build a description that includes the agent name for context.
	desc := skill.Description
	if desc == "" {
		desc = fmt.Sprintf("Invoke skill %q on remote agent %s", skill.ID, ra.card.Name)
	}

	// Build the MCP tool. The input schema has a single "message" string
	// parameter: callers pass a natural-language message or JSON arguments
	// that will be forwarded to the A2A agent.
	tool := mcp.NewTool(toolName,
		mcp.WithDescription(desc),
		mcp.WithString("message",
			mcp.Description("Message or JSON arguments to send to the remote agent"),
		),
	)

	// Extract tags from the skill, separating category from other tags.
	var category string
	var tags []string
	for _, tag := range skill.Tags {
		if category == "" {
			category = tag
		} else {
			tags = append(tags, tag)
		}
	}

	// Capture the skill ID in the closure for the handler.
	skillID := skill.ID

	return registry.ToolDefinition{
		Tool:     tool,
		Handler:  ra.makeHandler(skillID),
		Category: category,
		Tags:     tags,
	}
}

// makeHandler returns a ToolHandlerFunc that sends a message to the remote A2A
// agent for the given skill and translates the response.
func (ra *RemoteAgent) makeHandler(skillID string) registry.ToolHandlerFunc {
	return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		args := registry.ExtractArguments(req)
		messageText, _ := args["message"].(string)

		// Build the A2A message. We include the skill ID as structured data
		// so the remote agent knows which skill to invoke.
		var parts []*a2atypes.Part

		// If the message looks like JSON, try to parse it and send as a
		// structured DataPart with skill routing info.
		var jsonArgs map[string]any
		if strings.HasPrefix(strings.TrimSpace(messageText), "{") {
			if err := json.Unmarshal([]byte(messageText), &jsonArgs); err == nil {
				parts = append(parts, a2atypes.NewDataPart(map[string]any{
					"skill":     skillID,
					"arguments": jsonArgs,
				}))
			}
		}

		// If no structured data was extracted, send as text with a skill hint.
		if len(parts) == 0 {
			if messageText == "" {
				messageText = skillID
			}
			parts = append(parts, a2atypes.NewTextPart(messageText))
		}

		msg := a2atypes.NewMessage(a2atypes.MessageRoleUser, parts...)

		sendReq := &a2atypes.SendMessageRequest{
			Message: msg,
		}

		// Apply timeout.
		callCtx, cancel := context.WithTimeout(ctx, ra.timeout)
		defer cancel()

		result, err := ra.client.SendMessage(callCtx, sendReq)
		if err != nil {
			return nil, fmt.Errorf("a2a remote call to %s/%s failed: %w", ra.card.Name, skillID, err)
		}

		return ra.translateResult(result)
	}
}

// translateResult converts an A2A SendMessageResult into an MCP CallToolResult.
func (ra *RemoteAgent) translateResult(result a2atypes.SendMessageResult) (*registry.CallToolResult, error) {
	switch v := result.(type) {
	case *a2atypes.Task:
		return ra.translateTask(v), nil
	case *a2atypes.Message:
		return ra.translateMessage(v), nil
	default:
		return registry.MakeTextResult(fmt.Sprintf("unexpected A2A response type: %T", result)), nil
	}
}

// translateTask converts an A2A Task into an MCP CallToolResult.
func (ra *RemoteAgent) translateTask(task *a2atypes.Task) *registry.CallToolResult {
	// Check terminal failure states.
	switch task.Status.State {
	case a2atypes.TaskStateFailed, a2atypes.TaskStateRejected:
		errText := "remote agent task failed"
		if task.Status.Message != nil && len(task.Status.Message.Parts) > 0 {
			if t := task.Status.Message.Parts[0].Text(); t != "" {
				errText = t
			}
		}
		return registry.MakeErrorResult(errText)
	}

	// Extract text from artifacts.
	var texts []string
	for _, artifact := range task.Artifacts {
		for _, part := range artifact.Parts {
			if part == nil {
				continue
			}
			if t := part.Text(); t != "" {
				texts = append(texts, t)
			} else if d := part.Data(); d != nil {
				data, err := json.Marshal(d)
				if err == nil {
					texts = append(texts, string(data))
				}
			}
		}
	}

	if len(texts) == 0 {
		return registry.MakeTextResult("(no output)")
	}

	return registry.MakeTextResult(strings.Join(texts, "\n"))
}

// translateMessage converts an A2A Message into an MCP CallToolResult.
func (ra *RemoteAgent) translateMessage(msg *a2atypes.Message) *registry.CallToolResult {
	var texts []string
	for _, part := range msg.Parts {
		if part == nil {
			continue
		}
		if t := part.Text(); t != "" {
			texts = append(texts, t)
		} else if d := part.Data(); d != nil {
			data, err := json.Marshal(d)
			if err == nil {
				texts = append(texts, string(data))
			}
		}
	}

	if len(texts) == 0 {
		return registry.MakeTextResult("(no output)")
	}

	return registry.MakeTextResult(strings.Join(texts, "\n"))
}
