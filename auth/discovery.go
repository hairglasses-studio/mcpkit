package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

// AuthServerMetadata represents OAuth 2.0 Authorization Server Metadata (RFC 8414).
type AuthServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	JWKSURI                           string   `json:"jwks_uri"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint,omitempty"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
}

// SupportsPKCE returns true if the authorization server supports S256 PKCE.
func (m AuthServerMetadata) SupportsPKCE() bool {
	return slices.Contains(m.CodeChallengeMethodsSupported, "S256")
}

// SupportsGrant returns true if the authorization server supports the given grant type.
func (m AuthServerMetadata) SupportsGrant(grantType string) bool {
	return slices.Contains(m.GrantTypesSupported, grantType)
}

// MetadataDiscovery fetches and caches authorization server metadata.
type MetadataDiscovery struct {
	client   *http.Client
	mu       sync.RWMutex
	cache    map[string]*cachedMetadata
	cacheTTL time.Duration
}

type cachedMetadata struct {
	meta      AuthServerMetadata
	fetchedAt time.Time
}

// DiscoveryConfig configures metadata discovery.
type DiscoveryConfig struct {
	// CacheTTL controls how long metadata is cached. Default: 1 hour.
	CacheTTL time.Duration

	// HTTPClient is an optional HTTP client. Default: 10s timeout.
	HTTPClient *http.Client
}

// NewMetadataDiscovery creates a new metadata discovery client.
func NewMetadataDiscovery(cfg DiscoveryConfig) *MetadataDiscovery {
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = time.Hour
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &MetadataDiscovery{
		client:   cfg.HTTPClient,
		cache:    make(map[string]*cachedMetadata),
		cacheTTL: cfg.CacheTTL,
	}
}

// Discover fetches the authorization server metadata for the given issuer URL.
// It tries /.well-known/oauth-authorization-server first, then
// /.well-known/openid-configuration as a fallback.
func (d *MetadataDiscovery) Discover(ctx context.Context, issuerURL string) (AuthServerMetadata, error) {
	issuerURL = strings.TrimRight(issuerURL, "/")

	d.mu.RLock()
	if cached, ok := d.cache[issuerURL]; ok && time.Since(cached.fetchedAt) < d.cacheTTL {
		d.mu.RUnlock()
		return cached.meta, nil
	}
	d.mu.RUnlock()

	// Try OAuth 2.0 metadata endpoint first (RFC 8414)
	meta, err := d.fetchMetadata(ctx, issuerURL+"/.well-known/oauth-authorization-server")
	if err != nil {
		// Fallback to OpenID Connect discovery
		meta, err = d.fetchMetadata(ctx, issuerURL+"/.well-known/openid-configuration")
		if err != nil {
			return AuthServerMetadata{}, fmt.Errorf("discover auth server metadata for %s: %w", issuerURL, err)
		}
	}

	d.mu.Lock()
	d.cache[issuerURL] = &cachedMetadata{meta: meta, fetchedAt: time.Now()}
	d.mu.Unlock()

	return meta, nil
}

func (d *MetadataDiscovery) fetchMetadata(ctx context.Context, url string) (AuthServerMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return AuthServerMetadata{}, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return AuthServerMetadata{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AuthServerMetadata{}, fmt.Errorf("metadata endpoint %s returned %d", url, resp.StatusCode)
	}

	var meta AuthServerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return AuthServerMetadata{}, fmt.Errorf("decode metadata: %w", err)
	}

	if meta.Issuer == "" {
		return AuthServerMetadata{}, fmt.Errorf("metadata missing issuer field")
	}

	return meta, nil
}

// DiscoverJWKSURL is a convenience function that discovers the JWKS URI for an issuer.
func DiscoverJWKSURL(ctx context.Context, issuerURL string) (string, error) {
	d := NewMetadataDiscovery(DiscoveryConfig{})
	meta, err := d.Discover(ctx, issuerURL)
	if err != nil {
		return "", err
	}
	if meta.JWKSURI == "" {
		return "", fmt.Errorf("authorization server %s has no jwks_uri", issuerURL)
	}
	return meta.JWKSURI, nil
}
