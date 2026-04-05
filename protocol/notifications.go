package protocol

// Standard MCP notification methods.
// Notifications are JSON-RPC messages with no "id" field; the server must
// not send a response for them.
const (
	// NotificationInitialized is sent by the client after the initialize
	// handshake completes.
	NotificationInitialized = "notifications/initialized"

	// NotificationCancelled is sent by the client to cancel a pending request.
	// The params must include a "requestId" field identifying the request to cancel.
	NotificationCancelled = "notifications/cancelled"

	// NotificationProgress is sent by the server to report progress on a
	// long-running request. The params include "progressToken", "progress",
	// and optionally "total" and "message".
	NotificationProgress = "notifications/progress"

	// NotificationToolsListChanged is sent by the server when the set of
	// available tools changes.
	NotificationToolsListChanged = "notifications/tools/list_changed"

	// NotificationResourcesListChanged is sent by the server when the set of
	// available resources changes.
	NotificationResourcesListChanged = "notifications/resources/list_changed"

	// NotificationResourceUpdated is sent by the server when a subscribed
	// resource's content changes.
	NotificationResourceUpdated = "notifications/resources/updated"

	// NotificationPromptsListChanged is sent by the server when the set of
	// available prompts changes.
	NotificationPromptsListChanged = "notifications/prompts/list_changed"

	// NotificationRootsListChanged is sent by the client when the set of
	// roots changes.
	NotificationRootsListChanged = "notifications/roots/list_changed"

	// NotificationMessage is sent by the server for logging purposes.
	NotificationMessage = "notifications/message"
)

// Standard MCP request methods.
const (
	MethodInitialize     = "initialize"
	MethodPing           = "ping"
	MethodToolsList      = "tools/list"
	MethodToolsCall      = "tools/call"
	MethodResourcesList  = "resources/list"
	MethodResourcesRead  = "resources/read"
	MethodPromptsList    = "prompts/list"
	MethodPromptsGet     = "prompts/get"
	MethodLoggingSetLevel = "logging/setLevel"
	MethodCompletionComplete = "completion/complete"
)

// CancelledParams represents the parameters of a notifications/cancelled message.
type CancelledParams struct {
	// RequestID is the ID of the request to cancel.
	RequestID any `json:"requestId"`
	// Reason is an optional human-readable reason for the cancellation.
	Reason string `json:"reason,omitempty"`
}
