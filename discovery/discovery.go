package discovery

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// DefaultRegistryURL is the base URL for the MCP Registry API.
const DefaultRegistryURL = "https://registry.modelcontextprotocol.io"

// Sentinel errors returned by Client and Publisher methods.
var (
	ErrNotFound      = errors.New("registry: not found")
	ErrUnauthorized  = errors.New("registry: unauthorized")
	ErrConflict      = errors.New("registry: conflict")
	ErrRateLimited   = errors.New("registry: rate limited")
	ErrRegistryError = errors.New("registry: server error")
)

// ServerMetadata represents a server's entry in the MCP Registry.
//
// The fields here cover both the MCP Registry API schema and the
// .well-known/mcp.json crawl schema used by directory listings.
// Registry-only fields (ID, CreatedAt, UpdatedAt) are omitted from
// file output when they are zero values; directory-focused fields
// (License, Homepage, Categories, Install) are omitted from API
// payloads when empty.
type ServerMetadata struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Organization string            `json:"organization"`
	Repository   string            `json:"repository"`
	Homepage     string            `json:"homepage,omitempty"`
	License      string            `json:"license,omitempty"`
	Categories   []string          `json:"categories,omitempty"`
	Install      *InstallInfo      `json:"install,omitempty"`
	Tools        []ToolSummary     `json:"tools,omitempty"`
	Resources    []ResourceSummary `json:"resources,omitempty"`
	Prompts      []PromptSummary   `json:"prompts,omitempty"`
	Transports   []TransportInfo   `json:"transports,omitempty"`
	Auth         *AuthRequirement  `json:"auth,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// InstallInfo holds per-runtime install commands for the server,
// suitable for inclusion in .well-known/mcp.json so that directory
// crawlers can surface "how to install" metadata without visiting
// the repository README.
//
// Each field maps a runtime or package manager name to the install
// command string. Only non-empty fields are marshalled.
//
// Example:
//
//	install := &discovery.InstallInfo{
//	    Go:  "go install github.com/example/my-mcp-server@latest",
//	    NPM: "npm install -g @example/my-mcp-server",
//	}
type InstallInfo struct {
	Go     string `json:"go,omitempty"`
	NPM    string `json:"npm,omitempty"`
	PyPI   string `json:"pypi,omitempty"`
	Brew   string `json:"brew,omitempty"`
	Docker string `json:"docker,omitempty"`
	Binary string `json:"binary,omitempty"`
}

// ToolSummary is a brief description of a tool offered by a server.
type ToolSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version,omitempty"`
}

// ResourceSummary is a brief description of a resource offered by a server.
type ResourceSummary struct {
	URITemplate string `json:"uri_template"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PromptSummary is a brief description of a prompt offered by a server.
type PromptSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// TransportInfo describes a transport endpoint for a server.
// Type is one of: stdio, sse, streamable-http.
type TransportInfo struct {
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
}

// AuthRequirement describes the authentication required to connect to a server.
// Type is one of: none, bearer, oauth2.
type AuthRequirement struct {
	Type     string   `json:"type"`
	TokenURL string   `json:"token_url,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`
}

// SearchResult is the paginated response from a registry search or list query.
type SearchResult struct {
	Servers []ServerMetadata `json:"servers"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

// SearchQuery holds parameters for querying the registry.
type SearchQuery struct {
	Query     string
	Category  string
	Transport string
	Tags      []string
	Limit     int
	Offset    int
}

// cacheKey returns a deterministic string key for this query, suitable for
// use as a map key. Tags are sorted so order doesn't affect the key.
func (q SearchQuery) cacheKey() string {
	tags := make([]string, len(q.Tags))
	copy(tags, q.Tags)
	sort.Strings(tags)
	return fmt.Sprintf("query=%s&category=%s&transport=%s&tags=%s&limit=%d&offset=%d",
		q.Query,
		q.Category,
		q.Transport,
		strings.Join(tags, ","),
		q.Limit,
		q.Offset,
	)
}
