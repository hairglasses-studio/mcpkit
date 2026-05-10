package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/client"
)

// RegistryTokenSource supplies bearer tokens for registry publisher requests.
type RegistryTokenSource interface {
	Token(ctx context.Context) (string, error)
}

type staticTokenSource string

func (s staticTokenSource) Token(context.Context) (string, error) {
	return string(s), nil
}

// ClientCredentialsConfig configures OAuth2 client-credentials auth for the registry.
type ClientCredentialsConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	Audience     string
	UseBasicAuth bool
	HTTPClient   *http.Client
}

// ClientCredentialsTokenSource fetches and caches OAuth2 client-credentials tokens.
type ClientCredentialsTokenSource struct {
	cfg        ClientCredentialsConfig
	httpClient *http.Client

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// NewClientCredentialsTokenSource creates a registry token source using the
// OAuth2 client_credentials grant.
func NewClientCredentialsTokenSource(cfg ClientCredentialsConfig) (*ClientCredentialsTokenSource, error) {
	if strings.TrimSpace(cfg.TokenURL) == "" {
		return nil, fmt.Errorf("discovery: token URL is required")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, fmt.Errorf("discovery: client ID is required")
	}
	if strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, fmt.Errorf("discovery: client secret is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = client.Standard()
	}
	return &ClientCredentialsTokenSource{cfg: cfg, httpClient: httpClient}, nil
}

// Token returns a cached access token or fetches a new one when needed.
func (s *ClientCredentialsTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != "" && time.Now().Before(s.expiresAt.Add(-30*time.Second)) {
		return s.token, nil
	}

	token, expiresAt, err := s.fetch(ctx)
	if err != nil {
		return "", err
	}
	s.token = token
	s.expiresAt = expiresAt
	return token, nil
}

func (s *ClientCredentialsTokenSource) fetch(ctx context.Context) (string, time.Time, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if len(s.cfg.Scopes) > 0 {
		form.Set("scope", strings.Join(s.cfg.Scopes, " "))
	}
	if s.cfg.Audience != "" {
		form.Set("audience", s.cfg.Audience)
	}
	if !s.cfg.UseBasicAuth {
		form.Set("client_id", s.cfg.ClientID)
		form.Set("client_secret", s.cfg.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("discovery: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if s.cfg.UseBasicAuth {
		req.SetBasicAuth(s.cfg.ClientID, s.cfg.ClientSecret)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("discovery: token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("discovery: read token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("discovery: token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", time.Time{}, fmt.Errorf("discovery: decode token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("discovery: token response missing access_token")
	}
	if parsed.TokenType != "" && !strings.EqualFold(parsed.TokenType, "bearer") {
		return "", time.Time{}, fmt.Errorf("discovery: unsupported token_type %q", parsed.TokenType)
	}
	expiresIn := parsed.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	return parsed.AccessToken, time.Now().Add(time.Duration(expiresIn) * time.Second), nil
}
