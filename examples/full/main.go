//go:build !official_sdk

// Command full demonstrates a production-grade mcpkit MCP server with the full
// middleware stack: lifecycle, observability, finops, sanitize, security, and resilience.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/auth"
	"github.com/hairglasses-studio/mcpkit/finops"
	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/health"
	"github.com/hairglasses-studio/mcpkit/lifecycle"
	"github.com/hairglasses-studio/mcpkit/logging"
	"github.com/hairglasses-studio/mcpkit/observability"
	"github.com/hairglasses-studio/mcpkit/prompts"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resilience"
	"github.com/hairglasses-studio/mcpkit/resources"
	"github.com/hairglasses-studio/mcpkit/sanitize"
	"github.com/hairglasses-studio/mcpkit/security"
)

// ---------------------------------------------------------------------------
// Tool types
// ---------------------------------------------------------------------------

type SearchInput struct {
	Query string `json:"query" jsonschema:"required,description=Search query string"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Max results (default 10),minimum=1,maximum=100"`
}

type SearchResult struct {
	Results []string `json:"results"`
	Total   int      `json:"total"`
}

type NoteCreateInput struct {
	Title   string   `json:"title" jsonschema:"required,description=Note title"`
	Content string   `json:"content" jsonschema:"required,description=Note body"`
	Tags    []string `json:"tags,omitempty" jsonschema:"description=Optional tags"`
}

type NoteCreateOutput struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
}

// ---------------------------------------------------------------------------
// Tool module
// ---------------------------------------------------------------------------

type NotesModule struct{}

func (m *NotesModule) Name() string        { return "notes" }
func (m *NotesModule) Description() string { return "Note management tools" }
func (m *NotesModule) Tools() []registry.ToolDefinition {
	searchTool := handler.TypedHandler[SearchInput, SearchResult](
		"notes_search",
		"Search notes by query. Returns matching note titles.",
		func(_ context.Context, input SearchInput) (SearchResult, error) {
			limit := input.Limit
			if limit == 0 {
				limit = 10
			}
			allNotes := []string{
				"Getting Started with Go",
				"MCP Protocol Overview",
				"Building MCP Servers",
				"Go Concurrency Patterns",
			}
			var results []string
			for _, note := range allNotes {
				if strings.Contains(strings.ToLower(note), strings.ToLower(input.Query)) {
					results = append(results, note)
					if len(results) >= limit {
						break
					}
				}
			}
			return SearchResult{Results: results, Total: len(results)}, nil
		},
	)
	searchTool.Category = "notes"
	searchTool.Tags = []string{"search", "read-only"}
	searchTool.Complexity = registry.ComplexitySimple

	createTool := handler.TypedHandler[NoteCreateInput, NoteCreateOutput](
		"notes_create",
		"Create a new note with title, content, and optional tags.",
		func(_ context.Context, input NoteCreateInput) (NoteCreateOutput, error) {
			if len(input.Title) > 200 {
				return NoteCreateOutput{}, fmt.Errorf("title exceeds 200 character limit")
			}
			return NoteCreateOutput{
				ID:        "note_001",
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			}, nil
		},
	)
	createTool.Category = "notes"
	createTool.Tags = []string{"create", "write"}

	return []registry.ToolDefinition{searchTool, createTool}
}

// ---------------------------------------------------------------------------
// Resource module
// ---------------------------------------------------------------------------

type DocsResourceModule struct{}

func (m *DocsResourceModule) Name() string        { return "docs" }
func (m *DocsResourceModule) Description() string { return "Documentation resources" }

func (m *DocsResourceModule) Resources() []resources.ResourceDefinition {
	return []resources.ResourceDefinition{
		{
			Resource: mcp.NewResource(
				"docs://api/overview",
				"API Overview",
				mcp.WithResourceDescription("Overview of the notes API"),
				mcp.WithMIMEType("text/markdown"),
			),
			Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.TextResourceContents{
						URI:      "docs://api/overview",
						MIMEType: "text/markdown",
						Text:     "# Notes API\n\nUse `notes_search` to find notes and `notes_create` to add new ones.",
					},
				}, nil
			},
			Category: "documentation",
		},
	}
}

func (m *DocsResourceModule) Templates() []resources.TemplateDefinition {
	return []resources.TemplateDefinition{
		{
			Template: mcp.NewResourceTemplate(
				"docs://notes/{id}",
				"Note Content",
				mcp.WithTemplateDescription("Read a specific note by ID"),
				mcp.WithTemplateMIMEType("application/json"),
			),
			Handler: func(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.TextResourceContents{
						URI:      req.Params.URI,
						MIMEType: "application/json",
						Text:     fmt.Sprintf(`{"id":"%s","title":"Sample Note","content":"This is a sample note."}`, req.Params.URI),
					},
				}, nil
			},
			Category: "notes",
		},
	}
}

// ---------------------------------------------------------------------------
// Prompt module
// ---------------------------------------------------------------------------

type WorkflowPromptModule struct{}

func (m *WorkflowPromptModule) Name() string        { return "workflows" }
func (m *WorkflowPromptModule) Description() string { return "Workflow prompt templates" }

func (m *WorkflowPromptModule) Prompts() []prompts.PromptDefinition {
	return []prompts.PromptDefinition{
		{
			Prompt: mcp.NewPrompt("summarize_notes",
				mcp.WithPromptDescription("Summarize notes matching a search query"),
				mcp.WithArgument("query", mcp.RequiredArgument(), mcp.ArgumentDescription("Search query")),
				mcp.WithArgument("style", mcp.ArgumentDescription("Summary style: brief or detailed (default: brief)")),
			),
			Handler: func(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				query := req.Params.Arguments["query"]
				style := req.Params.Arguments["style"]
				if style == "" {
					style = "brief"
				}
				return &mcp.GetPromptResult{
					Description: "Summarize notes matching: " + query,
					Messages: []mcp.PromptMessage{
						mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(
							fmt.Sprintf("Search for notes about %q and provide a %s summary.", query, style),
						)),
					},
				}, nil
			},
			Category: "workflows",
			Tags:     []string{"summarization", "notes"},
		},
	}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	ctx := context.Background()
	logger := slog.Default()

	// --- Observability ---
	obs, obsShutdown, err := observability.Init(ctx, observability.Config{
		ServiceName:    "full-example",
		ServiceVersion: "2.0.0",
		EnableMetrics:  true,
		PrometheusPort: "9091",
	})
	if err != nil {
		log.Fatalf("observability init: %v", err)
	}

	// --- FinOps ---
	tracker := finops.NewTracker(finops.Config{
		TokenBudget: 1_000_000,
	})

	// --- Health ---
	checker := health.NewChecker(
		health.WithToolCount(func() int { return 2 }),
	)

	// --- Resilience ---
	cbReg := resilience.NewCircuitBreakerRegistry(nil)
	rlReg := resilience.NewRateLimitRegistry()

	// --- Security ---
	auditLog := security.NewAuditLogger(security.AuditLoggerConfig{})
	rbac := security.NewRBAC(security.RBACConfig{})
	userFunc := func(ctx context.Context) string { return auth.Subject(ctx) }

	// --- Tool registry with full middleware stack ---
	// Order (outermost first): observability → logging → finops → sanitize → security → resilience
	reg := registry.NewToolRegistry(registry.Config{
		DefaultTimeout: 30 * time.Second,
		Middleware: []registry.Middleware{
			obs.Middleware(),
			logging.Middleware(logger),
			finops.Middleware(tracker),
			sanitize.OutputMiddleware(sanitize.OutputPolicy{RedactSecrets: true}),
			security.AuditMiddleware(auditLog, userFunc),
			security.RBACMiddleware(rbac, auditLog, userFunc),
			resilience.CircuitBreakerMiddleware(cbReg),
			resilience.RateLimitMiddleware(rlReg),
		},
	})
	reg.RegisterModule(&NotesModule{})

	// --- Resource registry ---
	resReg := resources.NewResourceRegistry()
	resReg.RegisterModule(&DocsResourceModule{})

	// --- Prompt registry ---
	promptReg := prompts.NewPromptRegistry()
	promptReg.RegisterModule(&WorkflowPromptModule{})

	// --- Wire to MCP server ---
	s := server.NewMCPServer("full-example", "2.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, true),
		server.WithPromptCapabilities(true),
		server.WithRecovery(),
	)
	reg.RegisterWithServer(s)
	resReg.RegisterWithServer(s)
	promptReg.RegisterWithServer(s)

	// --- Lifecycle manager ---
	lm := lifecycle.New(lifecycle.Config{
		OnHealthy: func() {
			checker.SetStatus("healthy")
			log.Println("server healthy")
		},
		OnDraining: func() {
			checker.SetStatus("draining")
			log.Println("server draining")
		},
	})
	lm.OnShutdown(func(ctx context.Context) error {
		return obsShutdown(ctx)
	})

	log.Println("full-example server starting on stdio")
	if err := lm.Run(ctx, func(ctx context.Context) error {
		return server.ServeStdio(s)
	}); err != nil {
		log.Fatal(err)
	}
}
