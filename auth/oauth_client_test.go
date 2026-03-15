package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPKCEVerifier_Length(t *testing.T) {
	v, err := PKCEVerifier()
	if err != nil {
		t.Fatalf("PKCEVerifier: %v", err)
	}
	if len(v) != 43 {
		t.Errorf("verifier length = %d, want 43", len(v))
	}
}

func TestPKCEVerifier_Randomness(t *testing.T) {
	v1, _ := PKCEVerifier()
	v2, _ := PKCEVerifier()
	if v1 == v2 {
		t.Error("two verifiers should not be equal")
	}
}

func TestPKCEChallenge_RFC7636(t *testing.T) {
	// RFC 7636 Appendix B test vector
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	got := PKCEChallenge(verifier)
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got != want {
		t.Errorf("PKCEChallenge = %q, want %q", got, want)
	}
}

// oauthTestServer creates a mock OAuth server for testing.
func oauthTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *MetadataDiscovery) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	discovery := NewMetadataDiscovery(DiscoveryConfig{})
	return srv, discovery
}

func defaultMetadata(issuerURL string) AuthServerMetadata {
	return AuthServerMetadata{
		Issuer:                        issuerURL,
		AuthorizationEndpoint:         issuerURL + "/authorize",
		TokenEndpoint:                 issuerURL + "/token",
		JWKSURI:                       issuerURL + "/jwks",
		ResponseTypesSupported:        []string{"code"},
		GrantTypesSupported:           []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported: []string{"S256"},
	}
}

func TestOAuthClient_AuthorizationURL(t *testing.T) {
	srv, discovery := oauthTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/") {
			meta := defaultMetadata("http://" + r.Host)
			json.NewEncoder(w).Encode(meta)
			return
		}
		http.NotFound(w, r)
	})

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID:    "test-client",
		RedirectURI: "http://localhost:8080/callback",
		Scopes:      []string{"openid", "profile"},
	}, discovery)

	verifier, _ := PKCEVerifier()
	authURL, err := oc.AuthorizationURL(context.Background(), srv.URL, AuthorizationParams{
		CodeVerifier: verifier,
		State:        "test-state",
	})
	if err != nil {
		t.Fatalf("AuthorizationURL: %v", err)
	}

	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}

	q := u.Query()
	if q.Get("response_type") != "code" {
		t.Errorf("response_type = %q, want code", q.Get("response_type"))
	}
	if q.Get("client_id") != "test-client" {
		t.Errorf("client_id = %q, want test-client", q.Get("client_id"))
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", q.Get("code_challenge_method"))
	}
	if q.Get("state") != "test-state" {
		t.Errorf("state = %q, want test-state", q.Get("state"))
	}
	if q.Get("scope") != "openid profile" {
		t.Errorf("scope = %q, want 'openid profile'", q.Get("scope"))
	}

	// Verify challenge matches verifier
	expectedChallenge := PKCEChallenge(verifier)
	if q.Get("code_challenge") != expectedChallenge {
		t.Errorf("code_challenge mismatch")
	}
}

func TestOAuthClient_AuthorizationURL_NoPKCE(t *testing.T) {
	srv, discovery := oauthTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		meta := AuthServerMetadata{
			Issuer:                 "http://" + r.Host,
			AuthorizationEndpoint:  "http://" + r.Host + "/authorize",
			TokenEndpoint:          "http://" + r.Host + "/token",
			ResponseTypesSupported: []string{"code"},
			GrantTypesSupported:    []string{"authorization_code"},
			// No CodeChallengeMethodsSupported
		}
		json.NewEncoder(w).Encode(meta)
	})

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID:    "test-client",
		RedirectURI: "http://localhost:8080/callback",
	}, discovery)

	_, err := oc.AuthorizationURL(context.Background(), srv.URL, AuthorizationParams{
		CodeVerifier: "test-verifier",
	})
	if !errors.Is(err, ErrNoPKCESupport) {
		t.Errorf("expected ErrNoPKCESupport, got %v", err)
	}
}

func TestOAuthClient_ExchangeCode(t *testing.T) {
	srv, discovery := oauthTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/") {
			meta := defaultMetadata("http://" + r.Host)
			json.NewEncoder(w).Encode(meta)
			return
		}
		if r.URL.Path == "/token" && r.Method == http.MethodPost {
			r.ParseForm()
			if r.Form.Get("grant_type") != "authorization_code" {
				t.Errorf("grant_type = %q, want authorization_code", r.Form.Get("grant_type"))
			}
			if r.Form.Get("code") != "test-code" {
				t.Errorf("code = %q, want test-code", r.Form.Get("code"))
			}
			if r.Form.Get("code_verifier") == "" {
				t.Error("missing code_verifier")
			}
			json.NewEncoder(w).Encode(TokenResponse{
				AccessToken:  "access-token-123",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "refresh-token-456",
			})
			return
		}
		http.NotFound(w, r)
	})

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID:    "test-client",
		RedirectURI: "http://localhost:8080/callback",
	}, discovery)

	resp, err := oc.ExchangeCode(context.Background(), srv.URL, "test-code", "test-verifier")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if resp.AccessToken != "access-token-123" {
		t.Errorf("access_token = %q, want access-token-123", resp.AccessToken)
	}
	if resp.RefreshToken != "refresh-token-456" {
		t.Errorf("refresh_token = %q, want refresh-token-456", resp.RefreshToken)
	}
}

func TestOAuthClient_ExchangeCode_Error(t *testing.T) {
	srv, discovery := oauthTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/") {
			meta := defaultMetadata("http://" + r.Host)
			json.NewEncoder(w).Encode(meta)
			return
		}
		if r.URL.Path == "/token" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(TokenErrorResponse{
				Error:            "invalid_grant",
				ErrorDescription: "authorization code expired",
			})
			return
		}
		http.NotFound(w, r)
	})

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID:    "test-client",
		RedirectURI: "http://localhost:8080/callback",
	}, discovery)

	_, err := oc.ExchangeCode(context.Background(), srv.URL, "bad-code", "verifier")
	if !errors.Is(err, ErrTokenExchange) {
		t.Errorf("expected ErrTokenExchange, got %v", err)
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error should contain 'invalid_grant': %v", err)
	}
}

func TestOAuthClient_ExchangeCode_PublicClient(t *testing.T) {
	srv, discovery := oauthTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/") {
			meta := defaultMetadata("http://" + r.Host)
			json.NewEncoder(w).Encode(meta)
			return
		}
		if r.URL.Path == "/token" {
			// Verify no Basic auth header for public client
			if _, _, ok := r.BasicAuth(); ok {
				t.Error("public client should not send Basic auth")
			}
			json.NewEncoder(w).Encode(TokenResponse{
				AccessToken: "public-token",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
			})
			return
		}
		http.NotFound(w, r)
	})

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID:    "public-client",
		RedirectURI: "http://localhost:8080/callback",
		// No ClientSecret — public client
	}, discovery)

	resp, err := oc.ExchangeCode(context.Background(), srv.URL, "code", "verifier")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if resp.AccessToken != "public-token" {
		t.Errorf("access_token = %q, want public-token", resp.AccessToken)
	}
}

func TestOAuthClient_RefreshToken(t *testing.T) {
	srv, discovery := oauthTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/") {
			meta := defaultMetadata("http://" + r.Host)
			json.NewEncoder(w).Encode(meta)
			return
		}
		if r.URL.Path == "/token" {
			r.ParseForm()
			if r.Form.Get("grant_type") != "refresh_token" {
				t.Errorf("grant_type = %q, want refresh_token", r.Form.Get("grant_type"))
			}
			json.NewEncoder(w).Encode(TokenResponse{
				AccessToken:  "new-access-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "new-refresh-token",
			})
			return
		}
		http.NotFound(w, r)
	})

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, discovery)

	resp, err := oc.RefreshToken(context.Background(), srv.URL, "old-refresh-token")
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if resp.AccessToken != "new-access-token" {
		t.Errorf("access_token = %q, want new-access-token", resp.AccessToken)
	}
}

func TestOAuthClient_Token_Cached(t *testing.T) {
	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, NewMetadataDiscovery(DiscoveryConfig{}))

	oc.SetToken(TokenResponse{
		AccessToken: "cached-token",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	})

	token, err := oc.Token(context.Background(), "https://unused.example.com")
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if token != "cached-token" {
		t.Errorf("token = %q, want cached-token", token)
	}
}

func TestOAuthClient_Token_ExpiredWithRefresh(t *testing.T) {
	srv, discovery := oauthTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/") {
			meta := defaultMetadata("http://" + r.Host)
			json.NewEncoder(w).Encode(meta)
			return
		}
		if r.URL.Path == "/token" {
			json.NewEncoder(w).Encode(TokenResponse{
				AccessToken:  "refreshed-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "new-refresh",
			})
			return
		}
		http.NotFound(w, r)
	})

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, discovery)

	// Set an expired token with a refresh token
	oc.mu.Lock()
	oc.cachedToken = &TokenResponse{AccessToken: "expired-token"}
	oc.tokenExpiry = time.Now().Add(-time.Minute)
	oc.refreshToken = "old-refresh"
	oc.mu.Unlock()

	token, err := oc.Token(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if token != "refreshed-token" {
		t.Errorf("token = %q, want refreshed-token", token)
	}
}

func TestOAuthClient_Token_ExpiredNoRefresh(t *testing.T) {
	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, NewMetadataDiscovery(DiscoveryConfig{}))

	// Set an expired token with no refresh token
	oc.mu.Lock()
	oc.cachedToken = &TokenResponse{AccessToken: "expired-token"}
	oc.tokenExpiry = time.Now().Add(-time.Minute)
	oc.mu.Unlock()

	_, err := oc.Token(context.Background(), "https://unused.example.com")
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestOAuthTransport_InjectsHeader(t *testing.T) {
	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, NewMetadataDiscovery(DiscoveryConfig{}))

	oc.SetToken(TokenResponse{
		AccessToken: "transport-token",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	})

	var gotAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	transport := &OAuthTransport{
		Client:    oc,
		IssuerURL: "https://unused.example.com",
	}

	httpClient := &http.Client{Transport: transport}
	resp, err := httpClient.Get(backend.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer transport-token" {
		t.Errorf("Authorization = %q, want 'Bearer transport-token'", gotAuth)
	}
}

func TestOAuthTransport_NoToken(t *testing.T) {
	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, NewMetadataDiscovery(DiscoveryConfig{}))

	transport := &OAuthTransport{
		Client:    oc,
		IssuerURL: "https://unused.example.com",
	}

	httpClient := &http.Client{Transport: transport}
	_, err := httpClient.Get("http://localhost:1/test")
	if err == nil {
		t.Fatal("expected error when no token available")
	}
}

// Verify PKCEChallenge matches the manual SHA256 computation
func TestPKCEChallenge_Manual(t *testing.T) {
	verifier := "test-verifier-string"
	h := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(h[:])
	got := PKCEChallenge(verifier)
	if got != want {
		t.Errorf("PKCEChallenge = %q, want %q", got, want)
	}
}
