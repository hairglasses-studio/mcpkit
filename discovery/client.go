package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/client"
)

// ClientConfig configures the registry query client.
type ClientConfig struct {
	// BaseURL is the registry API base URL. Default: DefaultRegistryURL.
	BaseURL string

	// CacheTTL controls how long search results are cached. Default: 5 minutes.
	CacheTTL time.Duration

	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client
}

// Client queries the MCP Registry API with TTL-based caching.
type Client struct {
	baseURL    string
	cacheTTL   time.Duration
	httpClient *http.Client

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	result    SearchResult
	expiresAt time.Time
}

// NewClient creates a new registry Client with the given configuration.
// Unset fields are filled with sensible defaults.
func NewClient(cfg ClientConfig) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultRegistryURL
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = client.Standard()
	}
	return &Client{
		baseURL:    cfg.BaseURL,
		cacheTTL:   cfg.CacheTTL,
		httpClient: cfg.HTTPClient,
		cache:      make(map[string]cacheEntry),
	}
}

// Search queries the registry for servers matching the given query.
// Results are cached for CacheTTL.
func (c *Client) Search(ctx context.Context, q SearchQuery) (SearchResult, error) {
	key := q.cacheKey()

	c.mu.RLock()
	if entry, ok := c.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.result, nil
	}
	c.mu.RUnlock()

	params := url.Values{}
	if q.Query != "" {
		params.Set("query", q.Query)
	}
	if q.Category != "" {
		params.Set("category", q.Category)
	}
	if q.Transport != "" {
		params.Set("transport", q.Transport)
	}
	for _, tag := range q.Tags {
		params.Add("tag", tag)
	}
	if q.Limit > 0 {
		params.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Offset > 0 {
		params.Set("offset", strconv.Itoa(q.Offset))
	}

	reqURL := c.baseURL + "/v1/servers"
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var result SearchResult
	if err := c.doGet(ctx, reqURL, &result); err != nil {
		return SearchResult{}, err
	}

	c.mu.Lock()
	c.cache[key] = cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(c.cacheTTL),
	}
	c.mu.Unlock()

	return result, nil
}

// Get fetches a single server by its registry ID. Results are not cached.
func (c *Client) Get(ctx context.Context, id string) (ServerMetadata, error) {
	reqURL := c.baseURL + "/v1/servers/" + url.PathEscape(id)
	var meta ServerMetadata
	if err := c.doGet(ctx, reqURL, &meta); err != nil {
		return ServerMetadata{}, err
	}
	return meta, nil
}

// List returns a paginated list of all servers in the registry.
// Results are cached for CacheTTL using a synthetic cache key.
func (c *Client) List(ctx context.Context, limit, offset int) (SearchResult, error) {
	return c.Search(ctx, SearchQuery{Limit: limit, Offset: offset})
}

// InvalidateCache clears all cached search results.
func (c *Client) InvalidateCache() {
	c.mu.Lock()
	c.cache = make(map[string]cacheEntry)
	c.mu.Unlock()
}

// doGet performs a GET request to the given URL and decodes the JSON response
// into dest. It maps HTTP error codes to sentinel errors.
func (c *Client) doGet(ctx context.Context, reqURL string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("discovery: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("discovery: http request: %w", err)
	}
	defer resp.Body.Close()

	if err := mapStatusError(resp.StatusCode, reqURL); err != nil {
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("discovery: decode response: %w", err)
	}
	return nil
}

// mapStatusError maps HTTP status codes to sentinel errors.
// Returns nil for 2xx responses.
func mapStatusError(status int, reqURL string) error {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, reqURL)
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("%w: %s", ErrUnauthorized, reqURL)
	case status == http.StatusConflict:
		return fmt.Errorf("%w: %s", ErrConflict, reqURL)
	case status == http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrRateLimited, reqURL)
	case status >= 500:
		return fmt.Errorf("%w: status %d at %s", ErrRegistryError, status, reqURL)
	default:
		return fmt.Errorf("discovery: unexpected status %d at %s", status, reqURL)
	}
}
