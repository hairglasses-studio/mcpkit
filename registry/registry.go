package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

// ToolHandlerFunc is the function signature for tool handlers.
type ToolHandlerFunc func(ctx context.Context, request CallToolRequest) (*CallToolResult, error)

// Middleware wraps a tool handler with additional behavior.
// It receives the tool name, definition, and next handler in the chain.
type Middleware func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc

// ToolComplexity indicates the complexity level of a tool.
type ToolComplexity string

const (
	ComplexitySimple   ToolComplexity = "simple"
	ComplexityModerate ToolComplexity = "moderate"
	ComplexityComplex  ToolComplexity = "complex"
)

// CallType constants classify how a tool call is executed.
const (
	// CallTypeSync is the default: the tool runs synchronously and returns immediately.
	CallTypeSync = "sync"
	// CallTypeAsync indicates the tool starts work and returns a handle for polling.
	CallTypeAsync = "async"
	// CallTypeGated indicates the tool requires approval before execution (human-in-the-loop).
	CallTypeGated = "gated"
)

// ToolDefinition represents a complete tool with metadata.
type ToolDefinition struct {
	Tool                Tool
	Handler             ToolHandlerFunc
	Category            string
	Subcategory         string
	Tags                []string
	SearchTerms         []string
	UseCases            []string
	Complexity          ToolComplexity
	IsWrite             bool
	Deprecated          bool
	Successor           string
	Version             string
	Timeout             time.Duration
	CircuitBreakerGroup string
	RuntimeGroup        string
	OutputSchema        *ToolOutputSchema
	MaxResultChars      int
	DeferLoading        bool
	// CallType classifies how this tool executes: sync (default), async, or gated.
	CallType string
	// PreFetch marks this tool for automatic pre-fetching before LLM iterations.
	// When true, the Ralph loop's PreFetchHook may call this tool automatically.
	PreFetch bool
	// PreFetchKeywords are search terms that trigger automatic pre-fetching
	// of this tool when they appear in the current task context.
	PreFetchKeywords []string
	// AlwaysLoad marks this tool so that it is always included in the LLM
	// context even when deferred tool loading is active. Setting this to
	// true causes ApplyToolMetadata to inject the
	// "anthropic/alwaysLoad" = true field into the tool's _meta object.
	AlwaysLoad bool
	// SkipRequiresUserInteraction opts a write tool (IsWrite: true) out of
	// the automatic "anthropic/requiresUserInteraction" = true _meta field
	// that ApplyToolMetadata otherwise injects for every write tool. Not
	// used by any tool definition today — reserved for a write tool that
	// has its own equivalent consent mechanism and would otherwise double
	// -prompt the user. Has no effect on read-only tools.
	SkipRequiresUserInteraction bool
	// DestructiveOverride, when non-nil, replaces ApplyMCPAnnotations'
	// suffix-derived DestructiveHint for this tool. Use for a write tool
	// whose destructiveness the generic "_delete/_remove/_reset/_purge/
	// _clear/_flush/_destroy/_restart/_expire" suffix heuristic gets wrong
	// in either direction (e.g. a "_sync" tool that actually overwrites
	// destination state, or a "_reset" tool that is actually a safe
	// re-initialize-to-defaults). Has no effect on read-only tools, whose
	// DestructiveHint is always false.
	DestructiveOverride *bool
	// IdempotentOverride, when non-nil, replaces ApplyMCPAnnotations'
	// derived IdempotentHint for this tool (default: true for read-only
	// tools, suffix-derived for write tools). Use for a write tool the
	// suffix heuristic gets wrong, e.g. a "_create" that is actually
	// upsert-safe, or a "_restart" whose target does not tolerate a
	// redundant call the same as a single call.
	IdempotentOverride *bool
	// ReadOnlyOverride, when non-nil, forces IsWrite to !*ReadOnlyOverride,
	// overriding InferIsWrite suffix heuristics.
	ReadOnlyOverride *bool
}

// ToolModule is the interface that tool modules implement.
type ToolModule interface {
	Name() string
	Description() string
	Tools() []ToolDefinition
}

// DefaultToolTimeout is the maximum time a tool handler can run.
const DefaultToolTimeout = 30 * time.Second

// DefaultMaxResponseSize is the maximum response size before truncation (128KB).
const DefaultMaxResponseSize = 128 * 1024

// Config configures registry behavior.
type Config struct {
	// DefaultTimeout for tool handlers. Zero uses DefaultToolTimeout (30s).
	DefaultTimeout time.Duration

	// MaxResponseSize for truncation. Zero uses DefaultMaxResponseSize (128KB).
	MaxResponseSize int

	// ToolNamePrefix to strip when generating annotation titles (e.g., "myapp_").
	ToolNamePrefix string

	// RuntimeGroupMapper maps a category to a runtime group.
	// If nil or returns empty string, RuntimeGroup is left as-is from the ToolDefinition.
	RuntimeGroupMapper func(category string) string

	// Middleware to apply to all handlers, in order (outermost first).
	Middleware []Middleware
}

// ToolRegistry manages tool registration and lookup.
type ToolRegistry struct {
	mu       sync.RWMutex
	modules  map[string]ToolModule
	tools    map[string]ToolDefinition
	deferred map[string]bool // tools marked for deferred/lazy loading
	config   Config
}

// NewToolRegistry creates a new tool registry with the given config.
func NewToolRegistry(config ...Config) *ToolRegistry {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = DefaultToolTimeout
	}
	if cfg.MaxResponseSize == 0 {
		cfg.MaxResponseSize = DefaultMaxResponseSize
	}
	return &ToolRegistry{
		modules:  make(map[string]ToolModule),
		tools:    make(map[string]ToolDefinition),
		deferred: make(map[string]bool),
		config:   cfg,
	}
}

// SetMiddleware replaces the middleware chain. This is useful when middleware
// depends on state that isn't available at registry creation time (e.g.,
// an observability provider initialized after module init() functions run).
func (r *ToolRegistry) SetMiddleware(middleware []Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.Middleware = middleware
}

// RegisterModule registers a tool module with the registry.
func (r *ToolRegistry) RegisterModule(module ToolModule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.modules[module.Name()] = module

	for _, tool := range module.Tools() {
		if tool.RuntimeGroup == "" && r.config.RuntimeGroupMapper != nil {
			tool.RuntimeGroup = r.config.RuntimeGroupMapper(tool.Category)
		}
		if tool.ReadOnlyOverride != nil {
			tool.IsWrite = !*tool.ReadOnlyOverride
		} else if !tool.IsWrite {
			tool.IsWrite = InferIsWrite(tool.Tool.Name)
		}
		r.tools[tool.Tool.Name] = tool
	}
}

// GetTool returns a tool definition by name.
func (r *ToolRegistry) GetTool(name string) (ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// GetModule returns a module by name.
func (r *ToolRegistry) GetModule(name string) (ToolModule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	module, ok := r.modules[name]
	return module, ok
}

// ListModules returns all registered module names, sorted.
func (r *ToolRegistry) ListModules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.modules))
	for name := range r.modules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListTools returns all registered tool names, sorted.
func (r *ToolRegistry) ListTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListToolsByCategory returns tools filtered by category, sorted.
func (r *ToolRegistry) ListToolsByCategory(category string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name, tool := range r.tools {
		if tool.Category == category {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// ListToolsByRuntimeGroup returns tools filtered by runtime group, sorted.
func (r *ToolRegistry) ListToolsByRuntimeGroup(group string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name, tool := range r.tools {
		if tool.RuntimeGroup == group {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// GetRuntimeGroupStats returns tool counts per runtime group.
func (r *ToolRegistry) GetRuntimeGroupStats() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stats := make(map[string]int)
	for _, tool := range r.tools {
		group := tool.RuntimeGroup
		if group == "" {
			group = "unassigned"
		}
		stats[group]++
	}
	return stats
}

// GetAllToolDefinitions returns all registered tool definitions.
func (r *ToolRegistry) GetAllToolDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	allTools := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		allTools = append(allTools, tool)
	}
	return allTools
}

// ToolCount returns the number of registered tools.
func (r *ToolRegistry) ToolCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// ModuleCount returns the number of registered modules.
func (r *ToolRegistry) ModuleCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.modules)
}

// ToolStats holds statistics about registered tools.
type ToolStats struct {
	TotalTools      int            `json:"total_tools"`
	ModuleCount     int            `json:"module_count"`
	ByCategory      map[string]int `json:"by_category"`
	ByComplexity    map[string]int `json:"by_complexity"`
	ByRuntimeGroup  map[string]int `json:"by_runtime_group"`
	WriteToolsCount int            `json:"write_tools_count"`
	ReadOnlyCount   int            `json:"read_only_count"`
	DeprecatedCount int            `json:"deprecated_count"`
}

// GetToolStats returns statistics about the registered tools.
func (r *ToolRegistry) GetToolStats() ToolStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := ToolStats{
		TotalTools:     len(r.tools),
		ModuleCount:    len(r.modules),
		ByCategory:     make(map[string]int),
		ByComplexity:   make(map[string]int),
		ByRuntimeGroup: make(map[string]int),
	}
	for _, tool := range r.tools {
		stats.ByCategory[tool.Category]++
		stats.ByComplexity[string(tool.Complexity)]++
		group := tool.RuntimeGroup
		if group == "" {
			group = "unassigned"
		}
		stats.ByRuntimeGroup[group]++
		if tool.IsWrite {
			stats.WriteToolsCount++
		} else {
			stats.ReadOnlyCount++
		}
		if tool.Deprecated {
			stats.DeprecatedCount++
		}
	}
	return stats
}

// GetToolCatalog returns a structured catalog of all tools organized by category/subcategory.
func (r *ToolRegistry) GetToolCatalog() map[string]map[string][]ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	catalog := make(map[string]map[string][]ToolDefinition)
	for _, tool := range r.tools {
		if catalog[tool.Category] == nil {
			catalog[tool.Category] = make(map[string][]ToolDefinition)
		}
		subcategory := tool.Subcategory
		if subcategory == "" {
			subcategory = "general"
		}
		catalog[tool.Category][subcategory] = append(catalog[tool.Category][subcategory], tool)
	}
	return catalog
}

// RegisterWithServer registers all tools with an MCP server, applying
// annotations, output schemas, and the configured middleware chain.
func (r *ToolRegistry) RegisterWithServer(s *MCPServer) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, tool := range r.tools {
		annotated := ApplyToolMetadata(tool, r.config.ToolNamePrefix, r.deferred[tool.Tool.Name])
		wrapped := r.wrapHandler(tool.Tool.Name, tool)
		AddToolToServer(s, annotated.Tool, wrapped)
	}
}

// wrapHandler applies the built-in middleware (timeout, panic recovery, truncation)
// and any configured middleware chain.
func (r *ToolRegistry) wrapHandler(toolName string, td ToolDefinition) ToolHandlerFunc {
	handler := td.Handler
	maxSize := effectiveMaxResponseSize(td, r.config.MaxResponseSize)
	policy := truncationPolicyFor(toolName, td)

	// Truncation runs INNERMOST — closer to the handler than any configured
	// middleware — so that every middleware observes the result the client
	// will actually receive rather than the one the handler produced.
	//
	// It used to run only in the outer wrapper below, after the whole
	// middleware chain, and that made an over-ceiling result invisible to
	// observability: a consumer's telemetry middleware saw the handler's
	// intact result and recorded outcome=ok, while the client was handed a
	// mutilated one it then rejected. Measured live on secretstudios-mcp
	// 2026-08-27: two device_inventory_snapshot calls were fatally rejected
	// by Claude Code and BOTH were written to the invocation ledger as
	// outcome=ok, so nothing in the estate could see the failure.
	//
	// The outer call is kept as a pure size backstop, for the case where a
	// result-rewriting middleware (redaction, error compaction) grows the
	// result back over the cap on the way out. truncateResponse is
	// idempotent — it budgets for its own marker so a bounded result stays
	// bounded — so the second pass is a no-op on anything the first pass
	// already handled.
	handler = truncationHandler(handler, maxSize, policy)

	// Apply user-configured middleware (innermost applied first, so iterate in reverse)
	for i := len(r.config.Middleware) - 1; i >= 0; i-- {
		handler = r.config.Middleware[i](toolName, td, handler)
	}

	timeout := td.Timeout
	if timeout == 0 {
		timeout = r.config.DefaultTimeout
	}

	return func(ctx context.Context, request CallToolRequest) (result *CallToolResult, err error) {
		// Enforce timeout
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		// Panic recovery — this is the one case where both result and err are
		// returned non-nil. The result carries the user-facing error message;
		// err carries the stack trace for logging and upstream error handlers.
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				err = fmt.Errorf("panic in %s: %v\n%s", toolName, r, stack)
				result = MakeErrorResult(fmt.Sprintf("Internal error in %s: recovered from panic", toolName))
				slog.Error("tool panicked", "tool", toolName, "error", r)
			}
		}()

		result, err = handler(ctx, request)

		// Size backstop only; the semantic pass ran innermost (see above).
		result = truncateResponse(result, maxSize, policy)

		// Log errors
		if err != nil {
			slog.Error("tool failed", "tool", toolName, "error", err)
		} else if IsResultError(result) {
			for _, content := range result.Content {
				if text, ok := ExtractTextContent(content); ok && len(text) > 1 && text[0] == '[' {
					if idx := strings.Index(text, "]"); idx > 0 {
						code := text[1:idx]
						slog.Warn("tool error", "tool", toolName, "error_code", code)
					}
				}
			}
		}

		return result, err
	}
}

// TruncationMarker is the prefix mcpkit stamps into the text block it clips
// when a result is over the response cap. Exported so a consumer can detect a
// degraded result programmatically instead of string-matching a literal that
// might change; see IsTruncatedResult.
const TruncationMarker = "[TRUNCATED:"

// ResultTooLargeCode is the error code that prefixes the error result
// returned when an over-cap result CANNOT be degraded — see truncateResponse.
// It follows the "[CODE] message" shape wrapHandler's error logging already
// parses, so an over-cap refusal shows up as error_code=RESULT_TOO_LARGE.
const ResultTooLargeCode = "[RESULT_TOO_LARGE]"

// maxListedParams bounds how many parameter names the over-cap message names,
// so the guidance can never itself become a large payload.
const maxListedParams = 16

// narrowingParamHints are substrings that identify a request parameter as one
// a caller can use to make the response smaller. Matched case-insensitively
// against the tool's OWN declared input properties — the message never names a
// parameter the tool does not accept, which is the whole point of deriving
// this per tool rather than hardcoding "limit/offset/filters" everywhere.
var narrowingParamHints = []string{
	"limit", "offset", "page", "cursor", "max", "top_n", "count",
	"since", "until", "filter", "fields", "scope", "detail", "brief",
}

// truncationPolicy carries the per-tool facts the truncation path needs and
// cannot re-derive from a bare *CallToolResult: which tool this is, whether it
// declares an outputSchema (which decides whether an over-cap result can be
// degraded at all), and the parameter names a caller could narrow with.
type truncationPolicy struct {
	toolName string
	// declaresOutputSchema is true when the tool advertises an outputSchema
	// on the wire. That advertisement changes an over-cap result from a
	// degradation into a fatal one: a schema-validating client rejects a
	// success-shaped result with no structuredContent outright.
	declaresOutputSchema bool
	// paramNames are the tool's declared input properties, sorted.
	paramNames []string
}

// truncationPolicyFor reads the policy off a tool definition. Both SDK shapes
// are covered: ToolDefinition.OutputSchema is the pre-ApplyToolMetadata field,
// and Tool.OutputSchema is where ApplyToolMetadata copies it — wrapHandler is
// handed the raw definition, so either may be the populated one.
func truncationPolicyFor(toolName string, td ToolDefinition) truncationPolicy {
	policy := truncationPolicy{toolName: toolName}

	if td.OutputSchema != nil {
		policy.declaresOutputSchema = true
	}
	if _, ok := OutputSchemaType(td.Tool); ok {
		policy.declaresOutputSchema = true
	}
	if _, ok := OutputSchemaProperties(td.Tool); ok {
		policy.declaresOutputSchema = true
	}

	if props, ok := InputSchemaProperties(td.Tool.InputSchema); ok {
		names := make([]string, 0, len(props))
		for name := range props {
			names = append(names, name)
		}
		sort.Strings(names)
		policy.paramNames = names
	}
	return policy
}

// narrowingParams returns the subset of this tool's own parameters that look
// like they can shrink a response. Empty when the tool declares none.
func (p truncationPolicy) narrowingParams() []string {
	var out []string
	for _, name := range p.paramNames {
		lower := strings.ToLower(name)
		for _, hint := range narrowingParamHints {
			if strings.Contains(lower, hint) {
				out = append(out, name)
				break
			}
		}
	}
	return out
}

// narrowingHint is the short parenthetical used inside the degraded-result
// marker. It names the tool's real parameters when it has narrowing ones and
// falls back to the generic wording only when it does not.
func (p truncationPolicy) narrowingHint() string {
	if names := p.narrowingParams(); len(names) > 0 {
		return strings.Join(capNames(names, 6), "/")
	}
	return "limit/offset/filters"
}

// capNames bounds a name list, reporting the overflow rather than hiding it.
func capNames(names []string, limit int) []string {
	if limit <= 0 || len(names) <= limit {
		return names
	}
	out := make([]string, 0, limit+1)
	out = append(out, names[:limit]...)
	return append(out, fmt.Sprintf("(+%d more)", len(names)-limit))
}

// IsTruncatedResult reports whether a result carries mcpkit's truncation
// marker — i.e. it is a degraded, partial result rather than a complete one.
func IsTruncatedResult(result *CallToolResult) bool {
	if result == nil {
		return false
	}
	for _, content := range result.Content {
		if text, ok := ExtractTextContent(content); ok && strings.Contains(text, TruncationMarker) {
			return true
		}
	}
	return false
}

// IsResultTooLargeError reports whether a result is the error mcpkit returns
// when an over-cap result could not be degraded without breaking the tool's
// declared outputSchema.
func IsResultTooLargeError(result *CallToolResult) bool {
	if result == nil || !IsResultError(result) {
		return false
	}
	for _, content := range result.Content {
		if text, ok := ExtractTextContent(content); ok && strings.HasPrefix(text, ResultTooLargeCode) {
			return true
		}
	}
	return false
}

// truncationHandler wraps a handler so its result is bounded before any
// configured middleware observes it. See wrapHandler for why the ordering
// matters.
func truncationHandler(next ToolHandlerFunc, maxSize int, policy truncationPolicy) ToolHandlerFunc {
	return func(ctx context.Context, request CallToolRequest) (*CallToolResult, error) {
		result, err := next(ctx, request)
		return truncateResponse(result, maxSize, policy), err
	}
}

// truncateResponse bounds a tool result to maxSize across the WHOLE result —
// every text content block plus structuredContent — not per field.
//
// It used to be per-field, and that was a correctness defect rather than a
// tuning choice (secretstudios-mcp notes/tool-result-size-audit-2026-08-23.md
// §4.2). Results built by handler.StructuredResult carry the same data twice:
// indented in content[0].text and again as structuredContent. The old loop
// clipped the text half, appended "[TRUNCATED: response exceeded NKB limit]"
// to it, and shipped the complete structuredContent alongside untouched — so
// the caller was told the response had been truncated while the full payload
// sat in the same result, and the server emitted ~2.7x its own ceiling.
// Measured live on secretstudios_tool_catalog: 353,127 bytes against a
// 131,072-byte cap (text clipped at the cap and marked, structuredContent
// complete with all 369 tools). The same loop also gave every text block its
// own independent maxSize, so N blocks shipped N*maxSize with no marker.
//
// # Why dropping structuredContent is not always a legal degradation
//
// The first fix for the above dropped structuredContent unconditionally and
// clipped the text half. For a tool that declares NO outputSchema that is a
// real degradation and remains the behaviour here. For a tool that DOES
// declare one it is not a degradation at all, it is a fatal result: a
// schema-validating client rejects a success-shaped result carrying no
// structuredContent outright. Claude Code 2.1.247 does exactly this — its
// bundled client throws
//
//	Tool <name> has an output schema but did not return structured content
//
// on `!structuredContent && !isError`, and separately validates any
// structuredContent that IS present against the schema even on an error
// result. Two consequences follow, and both shape the code below:
//
//   - A degraded success is worse than useless for such a tool: the caller
//     sees a schema complaint, not a size problem. Measured live on
//     secretstudios-mcp 2026-08-27: two device_inventory_snapshot calls died
//     this way, and the agent then spent 11 further tool calls and 3 tool
//     searches before falling back to raw shell.
//   - Synthesising a placeholder structuredContent is not available either.
//     This layer registers tools untyped and does not know the schema's
//     shape, and anything it invented would be validated against that schema
//     — failing validation in the good case and, in the bad case, PASSING as
//     a plausible-looking empty payload that reads as "no results" rather
//     than "too large". Fabricating a success is the one outcome worse than
//     an error.
//
// So an over-cap result from a schema-declaring tool becomes a real MCP error
// result. The client surfaces an error result's text (that is the only
// channel an error has), the tool's actual parameter names are named in it,
// and every observer — audit middleware, telemetry, the wrapHandler error log
// — sees a non-ok outcome instead of a silent success.
//
// Enforcement order when a result is over budget:
//
//  1. If the tool declares an outputSchema AND the handler produced
//     structuredContent, return ResultTooLargeCode as an error result naming
//     the sizes and the tool's own narrowing parameters. Nothing is degraded.
//  2. Otherwise structuredContent is dropped. It cannot be "truncated" — it
//     is an arbitrary value that must stay valid JSON, so a partial one is
//     not a thing that can exist. Dropping it is spec-safe here precisely
//     because case 1 already removed the schema-declaring tools.
//  3. The remaining text blocks share a single budget — maxSize minus room
//     reserved for the marker, so the bounded result lands AT or under the
//     cap and a second pass over it is a no-op. The first block that
//     overflows carries the marker; any block after it is emptied rather
//     than given a marker of its own.
//  4. If nothing needed cutting but structuredContent was dropped, a notice
//     block is appended rather than editing an existing block, so a text
//     block that is valid JSON stays parseable.
//
// Dropping the structured half rather than the text half in case 2 is
// deliberate: the text block is the representation every client can read
// (clients that prefer structuredContent fall back to it when absent), so it
// is the half that must survive.
func truncateResponse(result *CallToolResult, maxSize int, policy truncationPolicy) *CallToolResult {
	if result == nil || maxSize <= 0 {
		return result
	}

	textBytes := 0
	for _, content := range result.Content {
		if text, ok := ExtractTextContent(content); ok {
			textBytes += len(text)
		}
	}

	// Only measure structuredContent when the text half has not already blown
	// the budget on its own — the marshal is O(payload) and pointless once we
	// know the answer.
	structBytes := 0
	if result.StructuredContent != nil && textBytes <= maxSize {
		if encoded, err := json.Marshal(result.StructuredContent); err == nil {
			structBytes = len(encoded)
		}
	}
	if textBytes+structBytes <= maxSize {
		return result
	}

	// Case 1: not degradable. Refuse, loudly and with actionable guidance.
	if result.StructuredContent != nil && policy.declaresOutputSchema {
		// The fast path above skips this marshal when the text half alone is
		// already over budget. Here the number is quoted back to the caller,
		// so pay for it rather than reporting a misleading zero.
		if structBytes == 0 {
			if encoded, err := json.Marshal(result.StructuredContent); err == nil {
				structBytes = len(encoded)
			}
		}
		slog.Warn("tool result over the response cap and not degradable",
			"tool", policy.toolName,
			"max_bytes", maxSize,
			"text_bytes", textBytes,
			"structured_bytes", structBytes,
			"reason", "tool declares an outputSchema; a result without structuredContent is rejected by the client")
		return MakeErrorResult(clampMessage(oversizeErrorMessage(policy, maxSize, textBytes, structBytes), maxSize))
	}

	droppedStructured := result.StructuredContent != nil
	result.StructuredContent = nil

	// Report a sub-kilobyte cap in bytes; "%dKB" renders a 100-byte cap as
	// "0KB", which reads as "no limit at all".
	limit := fmt.Sprintf("%dKB", maxSize/1024)
	if maxSize < 1024 {
		limit = fmt.Sprintf("%dB", maxSize)
	}
	suffix := "]"
	if droppedStructured {
		suffix = fmt.Sprintf("; structuredContent omitted for the same reason — narrow the request (%s) for a complete result]", policy.narrowingHint())
	}
	cutMarker := fmt.Sprintf("\n\n%s response exceeded %s limit%s", TruncationMarker, limit, suffix)
	noticeBlock := fmt.Sprintf("%s response exceeded %s limit; structuredContent (%d bytes) omitted — narrow the request (%s) for a complete result]",
		TruncationMarker, limit, structBytes, policy.narrowingHint())

	// Reserve room for whichever notice this pass will append, so the bounded
	// result lands at or under maxSize. That reservation is what makes this
	// function idempotent: the backstop pass in wrapHandler sees an already
	// in-budget result and returns it untouched instead of clipping a second
	// time and stacking a second marker. The notice block is only ever
	// appended when structuredContent was dropped, so reserving room for it
	// otherwise would throw away payload that fits — at a small cap that is
	// most of the response.
	reserve := len(cutMarker)
	if droppedStructured && len(noticeBlock) > reserve {
		reserve = len(noticeBlock)
	}
	remaining := maxSize - reserve
	if remaining < 0 {
		remaining = 0
	}

	cut := false
	for i, content := range result.Content {
		text, ok := ExtractTextContent(content)
		if !ok {
			continue
		}
		if cut {
			// Everything after the cut point is already past the budget.
			// Empty it rather than stamping another marker on it — N markers
			// on N blocks is how the pre-reservation version could still
			// overshoot the cap it had just enforced.
			if text != "" {
				result.Content[i] = MakeTextContent("")
			}
			continue
		}
		if len(text) <= remaining {
			remaining -= len(text)
			continue
		}
		result.Content[i] = MakeTextContent(text[:remaining] + cutMarker)
		remaining = 0
		cut = true
	}

	if !cut && droppedStructured {
		result.Content = append(result.Content, MakeTextContent(noticeBlock))
	}

	slog.Warn("tool result truncated",
		"tool", policy.toolName,
		"max_bytes", maxSize,
		"text_bytes", textBytes,
		"structured_bytes", structBytes,
		"dropped_structured_content", droppedStructured)

	return result
}

// oversizeErrorMessage builds the guidance an agent actually needs: what the
// cap was, how far over the result was, why nothing partial came back, and
// which of THIS tool's parameters can shrink it.
func oversizeErrorMessage(policy truncationPolicy, maxSize, textBytes, structBytes int) string {
	name := policy.toolName
	if name == "" {
		name = "this tool"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s %s produced a %d-byte result (text %d + structuredContent %d), over its %d-byte response limit.\n\n",
		ResultTooLargeCode, name, textBytes+structBytes, textBytes, structBytes, maxSize)
	b.WriteString("No partial result was returned. This tool declares an outputSchema, and structuredContent cannot be clipped and still satisfy it — a truncated success-shaped result is rejected by schema-validating clients with \"has an output schema but did not return structured content\", which hides the real cause.\n\n")

	if narrowing := policy.narrowingParams(); len(narrowing) > 0 {
		fmt.Fprintf(&b, "Retry with a narrower request. Narrowing parameters this tool accepts: %s.\n",
			strings.Join(capNames(narrowing, maxListedParams), ", "))
	} else {
		b.WriteString("Retry with a narrower request.\n")
	}
	if len(policy.paramNames) > 0 {
		fmt.Fprintf(&b, "All parameters %s accepts: %s.\n",
			name, strings.Join(capNames(policy.paramNames, maxListedParams), ", "))
	}
	return b.String()
}

// clampMessage keeps the guidance itself inside the cap it is explaining.
// Cutting on a byte boundary can split a rune, so the tail is scrubbed back to
// valid UTF-8 rather than shipped as a mojibake byte.
func clampMessage(msg string, maxSize int) string {
	if maxSize <= 0 || len(msg) <= maxSize {
		return msg
	}
	return strings.ToValidUTF8(msg[:maxSize], "")
}
