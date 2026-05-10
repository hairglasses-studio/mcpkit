package discovery

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateServerMetadata_Valid(t *testing.T) {
	report := ValidateServerMetadata(validPublishMetadata())

	if !report.Valid {
		t.Fatalf("Valid = false, issues: %#v", report.Issues)
	}
	if err := report.Err(); err != nil {
		t.Fatalf("Err = %v, want nil", err)
	}
}

func TestValidateServerMetadata_RequiredFields(t *testing.T) {
	report := ValidateServerMetadata(ServerMetadata{})

	if report.Valid {
		t.Fatal("Valid = true, want false")
	}
	if !errors.Is(report.Err(), ErrValidationFailed) {
		t.Fatalf("Err = %v, want ErrValidationFailed", report.Err())
	}
	summary := report.Summary()
	for _, want := range []string{"name", "description", "version", "organization", "repository", "transports"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary %q does not mention %s", summary, want)
		}
	}
}

func TestValidateServerMetadata_URLsAndEnums(t *testing.T) {
	meta := validPublishMetadata()
	meta.Repository = "not-a-url"
	meta.Homepage = "ftp://example.com/project"
	meta.Transports = []TransportInfo{{Type: "websocket", URL: "http://example.com/ws"}}
	meta.Auth = &AuthRequirement{Type: "oauth2", TokenURL: "bad-url"}
	meta.Tags = []string{"prod", "prod"}

	report := ValidateServerMetadata(meta)

	if report.Valid {
		t.Fatal("Valid = true, want false")
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Path != "tags[1]" {
		t.Fatalf("warnings = %#v, want duplicate tag warning", report.Warnings)
	}
	for _, wantPath := range []string{"repository", "homepage", "transports[0].type", "auth.token_url"} {
		if !hasIssue(report, wantPath) {
			t.Fatalf("expected issue at %s in %#v", wantPath, report.Issues)
		}
	}
}

func validPublishMetadata() ServerMetadata {
	return ServerMetadata{
		Name:         "example-server",
		Description:  "Example MCP server",
		Version:      "1.2.3",
		Organization: "Example Org",
		Repository:   "https://github.com/example/mcp-server",
		Homepage:     "https://example.com/mcp-server",
		License:      "MIT",
		Categories:   []string{"developer-tools"},
		Tags:         []string{"example", "mcp"},
		Tools: []ToolSummary{{
			Name:        "search",
			Description: "Search examples",
		}},
		Transports: []TransportInfo{{
			Type: "streamable-http",
			URL:  "https://example.com/mcp",
		}},
		Auth: &AuthRequirement{Type: "none"},
	}
}

func hasIssue(report ValidationReport, path string) bool {
	for _, issue := range report.Issues {
		if issue.Path == path {
			return true
		}
	}
	return false
}
