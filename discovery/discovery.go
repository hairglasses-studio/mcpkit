// Package discovery provides MCP Registry integration for server discovery and publishing.
//
// It supports querying the MCP Registry API for server metadata, caching results with
// configurable TTL, and publishing server metadata to the registry.
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
type ServerMetadata struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Organization string            `json:"organization"`
	Repository   string            `json:"repository"`
	Tools        []ToolSummary     `json:"tools,omitempty"`
	Resources    []ResourceSummary `json:"resources,omitempty"`
	Prompts      []PromptSummary   `json:"prompts,omitempty"`
	Transports   []TransportInfo   `json:"transports,omitempty"`
	Auth         *AuthRequirement  `json:"auth,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
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
