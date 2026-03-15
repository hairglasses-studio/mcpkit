package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/client"
)

var (
	ErrNoPKCESupport  = errors.New("authorization server does not support S256 PKCE")
	ErrNoGrantSupport = errors.New("authorization server does not support required grant type")
	ErrTokenExpired   = errors.New("token expired and no refresh token available")
	ErrTokenExchange  = errors.New("token exchange failed")
)

// TokenResponse represents an OAuth 2.0 token response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// TokenErrorResponse represents an OAuth 2.0 token error response.
type TokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// OAuthClientConfig configures an OAuth 2.1 client.
type OAuthClientConfig struct {
	ClientID     string
	ClientSecret string // Empty for public clients
	RedirectURI  string
	Scopes       []string
	HTTPClient   *http.Client // Default: client.Standard()
}

// AuthorizationParams configures an authorization URL request.
type AuthorizationParams struct {
	CodeVerifier string            // PKCE verifier (challenge is computed)
	State        string            // CSRF state parameter
	Extra        map[string]string // Additional query parameters
}

// OAuthClient implements the OAuth 2.1 authorization code flow with PKCE.
type OAuthClient struct {
	config    OAuthClientConfig
	discovery *MetadataDiscovery

	mu           sync.RWMutex
	cachedToken  *TokenResponse
	tokenExpiry  time.Time
	refreshToken string
}

// NewOAuthClient creates a new OAuth 2.1 client.
func NewOAuthClient(cfg OAuthClientConfig, discovery *MetadataDiscovery) *OAuthClient {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = client.Standard()
	}
	return &OAuthClient{
		config:    cfg,
		discovery: discovery,
	}
}

// expiryMargin is subtracted from token expiry to avoid using nearly-expired tokens.
const expiryMargin = 10 * time.Second

// AuthorizationURL builds the authorization URL for the OAuth 2.1 code flow.
// It discovers server metadata, verifies PKCE and grant support, and constructs the URL.
func (c *OAuthClient) AuthorizationURL(ctx context.Context, issuerURL string, params AuthorizationParams) (string, error) {
	meta, err := c.discovery.Discover(ctx, issuerURL)
	if err != nil {
		return "", fmt.Errorf("discover metadata: %w", err)
	}

	if !meta.SupportsPKCE() {
		return "", ErrNoPKCESupport
	}
	if !meta.SupportsGrant("authorization_code") {
		return "", ErrNoGrantSupport
	}

	u, err := url.Parse(meta.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("parse authorization endpoint: %w", err)
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.config.ClientID)
	q.Set("redirect_uri", c.config.RedirectURI)
	q.Set("code_challenge", PKCEChallenge(params.CodeVerifier))
	q.Set("code_challenge_method", "S256")

	if len(c.config.Scopes) > 0 {
		q.Set("scope", strings.Join(c.config.Scopes, " "))
	}
	if params.State != "" {
		q.Set("state", params.State)
	}
	for k, v := range params.Extra {
		q.Set(k, v)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ExchangeCode exchanges an authorization code for tokens.
func (c *OAuthClient) ExchangeCode(ctx context.Context, issuerURL, code, codeVerifier string) (*TokenResponse, error) {
	meta, err := c.discovery.Discover(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover metadata: %w", err)
	}

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.config.RedirectURI},
		"client_id":     {c.config.ClientID},
		"code_verifier": {codeVerifier},
	}

	return c.doTokenRequest(ctx, meta.TokenEndpoint, data)
}

// RefreshToken exchanges a refresh token for new tokens.
func (c *OAuthClient) RefreshToken(ctx context.Context, issuerURL, refreshToken string) (*TokenResponse, error) {
	meta, err := c.discovery.Discover(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover metadata: %w", err)
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {c.config.ClientID},
	}

	return c.doTokenRequest(ctx, meta.TokenEndpoint, data)
}

// Token returns a valid access token, refreshing if necessary.
func (c *OAuthClient) Token(ctx context.Context, issuerURL string) (string, error) {
	c.mu.RLock()
	token := c.cachedToken
	expiry := c.tokenExpiry
	refresh := c.refreshToken
	c.mu.RUnlock()

	if token != nil && time.Now().Before(expiry) {
		return token.AccessToken, nil
	}

	// Token expired or missing — try refresh
	if refresh != "" {
		resp, err := c.RefreshToken(ctx, issuerURL, refresh)
		if err == nil {
			c.SetToken(*resp)
			return resp.AccessToken, nil
		}
	}

	if token != nil {
		return "", ErrTokenExpired
	}
	return "", ErrTokenExpired
}

// SetToken manually sets the cached token.
func (c *OAuthClient) SetToken(t TokenResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cachedToken = &t
	if t.ExpiresIn > 0 {
		c.tokenExpiry = time.Now().Add(time.Duration(t.ExpiresIn)*time.Second - expiryMargin)
	} else {
		// No expiry info — treat as valid for 1 hour
		c.tokenExpiry = time.Now().Add(time.Hour)
	}
	if t.RefreshToken != "" {
		c.refreshToken = t.RefreshToken
	}
}

func (c *OAuthClient) doTokenRequest(ctx context.Context, tokenEndpoint string, data url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if c.config.ClientSecret != "" {
		req.SetBasicAuth(c.config.ClientID, c.config.ClientSecret)
	}

	resp, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var tokenErr TokenErrorResponse
		if json.Unmarshal(body, &tokenErr) == nil && tokenErr.Error != "" {
			return nil, fmt.Errorf("%w: %s: %s", ErrTokenExchange, tokenErr.Error, tokenErr.ErrorDescription)
		}
		return nil, fmt.Errorf("%w: HTTP %d", ErrTokenExchange, resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	return &tokenResp, nil
}
