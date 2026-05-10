package research

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func platformTestServer(t *testing.T) (*httptest.Server, map[string]string) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/cloudflare-workers", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			Cloudflare Workers Agents SDK update.
			MCP tools are now available after waitForMcpConnections settles.
			The RPC transport for MCP supports Durable Object hibernation.
			Deprecated connection settings require migration.
		`)
	})
	mux.HandleFunc("/cloudflare-mcp", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			Cloudflare MCP blog covers remote MCP servers, Code Mode, AI Gateway, and Cloudflare Workers agents.
		`)
	})
	mux.HandleFunc("/cloudflare-runtime", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			Workers runtime changelog: WebSocket streams and Durable Object compatibility flag updates.
		`)
	})
	mux.HandleFunc("/vercel-changelog", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			Vercel AI SDK and AI Gateway now include MCP improvements with OAuth support.
		`)
	})
	mux.HandleFunc("/ai-sdk-mcp", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			createMCPClient supports resources, prompts, elicitation, OAuth, and MCP tools.
			Some prompt APIs are experimental and the client does not support notifications.
		`)
	})
	mux.HandleFunc("/vercel-releases", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			AI SDK release patch changes for gateway streaming and MCP tool conversion.
		`)
	})
	mux.HandleFunc("/azure-start", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			Foundry MCP Server preview runs at https://mcp.ai.azure.com with Entra authentication.
			Preview capabilities are constrained and not recommended for production workloads.
		`)
	})
	mux.HandleFunc("/azure-register", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			Register a remote MCP server in Azure API Center and expose it through the organizational tool catalog for Agent Service.
		`)
	})
	mux.HandleFunc("/azure-news", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			Azure AI Foundry Agent Service supports the Model Context Protocol MCP tool for remote tools.
		`)
	})

	ts := httptest.NewServer(mux)
	return ts, map[string]string{
		"Cloudflare MCP Blog":                   ts.URL + "/cloudflare-mcp",
		"Cloudflare Workers Changelog":          ts.URL + "/cloudflare-workers",
		"Cloudflare Workers Runtime Changelog":  ts.URL + "/cloudflare-runtime",
		"Vercel Changelog":                      ts.URL + "/vercel-changelog",
		"AI SDK MCP Docs":                       ts.URL + "/ai-sdk-mcp",
		"Vercel AI SDK Releases":                ts.URL + "/vercel-releases",
		"Azure Foundry MCP Get Started":         ts.URL + "/azure-start",
		"Azure Foundry MCP Server Registration": ts.URL + "/azure-register",
		"Azure AI Foundry Agent What's New":     ts.URL + "/azure-news",
	}
}

func TestPlatformActivityTool_AllDefaultMonitors(t *testing.T) {
	ts, overrides := platformTestServer(t)
	defer ts.Close()

	m := NewModule(Config{HTTPClient: ts.Client(), SourceOverrides: overrides})
	out, err := m.handlePlatformActivity(context.Background(), PlatformInput{})
	if err != nil {
		t.Fatalf("handlePlatformActivity: %v", err)
	}
	if len(out.Results) != 3 {
		t.Fatalf("results count = %d, want 3", len(out.Results))
	}
	if len(out.Signals) == 0 {
		t.Fatal("expected high-signal findings")
	}
	if len(out.ActionItems) == 0 {
		t.Fatal("expected action items")
	}
	if !strings.Contains(out.Summary, "Scanned 3 cloud platforms") {
		t.Fatalf("unexpected summary: %s", out.Summary)
	}

	var sawCloudflare, sawVercel, sawAzure bool
	for _, result := range out.Results {
		if result.Relevance == 0 {
			t.Fatalf("%s relevance = 0, want positive", result.Platform)
		}
		switch result.Platform {
		case "Cloudflare Workers":
			sawCloudflare = true
		case "Vercel AI SDK":
			sawVercel = true
		case "Azure AI Foundry":
			sawAzure = true
		}
	}
	if !sawCloudflare || !sawVercel || !sawAzure {
		t.Fatalf("missing expected platform results: %+v", out.Results)
	}
}

func TestPlatformActivityTool_FiltersPlatform(t *testing.T) {
	ts, overrides := platformTestServer(t)
	defer ts.Close()

	m := NewModule(Config{HTTPClient: ts.Client(), SourceOverrides: overrides})
	out, err := m.handlePlatformActivity(context.Background(), PlatformInput{
		Platforms: []string{"vercel"},
	})
	if err != nil {
		t.Fatalf("handlePlatformActivity: %v", err)
	}
	if len(out.Results) != 1 {
		t.Fatalf("results count = %d, want 1", len(out.Results))
	}
	if out.Results[0].Platform != "Vercel AI SDK" {
		t.Fatalf("platform = %q, want Vercel AI SDK", out.Results[0].Platform)
	}
}

func TestPlatformActivityTool_NoMatchingPlatform(t *testing.T) {
	m := NewModule()
	out, err := m.handlePlatformActivity(context.Background(), PlatformInput{
		Platforms: []string{"unknown"},
	})
	if err != nil {
		t.Fatalf("handlePlatformActivity: %v", err)
	}
	if len(out.Results) != 0 {
		t.Fatalf("results count = %d, want 0", len(out.Results))
	}
	if out.Summary != "No cloud platform monitors matched the request." {
		t.Fatalf("unexpected summary: %s", out.Summary)
	}
}

func TestPlatformActivityTool_SourceErrorsBecomeActionItems(t *testing.T) {
	m := NewModule(Config{SourceOverrides: map[string]string{
		"Cloudflare MCP Blog":                  "://bad-url",
		"Cloudflare Workers Changelog":         "://bad-url",
		"Cloudflare Workers Runtime Changelog": "://bad-url",
	}})
	out, err := m.handlePlatformActivity(context.Background(), PlatformInput{
		Platforms: []string{"Cloudflare Workers"},
	})
	if err != nil {
		t.Fatalf("handlePlatformActivity: %v", err)
	}
	if len(out.Results) != 1 {
		t.Fatalf("results count = %d, want 1", len(out.Results))
	}
	if out.Results[0].Error == "" {
		t.Fatal("expected platform error when all sources fail")
	}
	if len(out.ActionItems) != 1 || !strings.Contains(out.ActionItems[0], "Restore Cloudflare Workers platform monitoring") {
		t.Fatalf("unexpected action items: %+v", out.ActionItems)
	}
}

func TestSummaryIncludesPlatformFindings(t *testing.T) {
	m := NewModule()
	out, err := m.handleSummary(context.Background(), SummaryInput{
		PlatformFindings: &PlatformOutput{
			Summary: "Scanned 1 cloud platform.",
			Results: []PlatformMonitorResult{
				{
					Platform:  "Vercel AI SDK",
					Relevance: 0.75,
					Sources:   []SourceFinding{{Name: "AI SDK MCP Docs"}},
					Signals: []PlatformSignal{
						{Platform: "Vercel AI SDK", Source: "AI SDK MCP Docs", Keyword: "createMCPClient", Snippet: "createMCPClient supports MCP tools", Severity: "watch"},
					},
				},
			},
			Signals: []PlatformSignal{
				{Platform: "Vercel AI SDK", Source: "AI SDK MCP Docs", Keyword: "createMCPClient", Snippet: "createMCPClient supports MCP tools", Severity: "watch"},
			},
			ActionItems: []string{"Assess Vercel AI SDK MCP client changes."},
		},
	})
	if err != nil {
		t.Fatalf("handleSummary: %v", err)
	}
	if !strings.Contains(out.Report, "Cloud Platforms") {
		t.Fatalf("report missing platform section: %s", out.Report)
	}
	if len(out.ActionItems) != 1 || out.ActionItems[0] != "Assess Vercel AI SDK MCP client changes." {
		t.Fatalf("unexpected action items: %+v", out.ActionItems)
	}
}
