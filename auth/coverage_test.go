package auth

// coverage_test.go — additional tests targeting uncovered branches across the auth package.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// discovery.go — SupportsGrant / DiscoverJWKSURL edge cases
// ---------------------------------------------------------------------------

func TestAuthServerMetadata_SupportsGrant_Empty(t *testing.T) {
	t.Parallel()

	meta := AuthServerMetadata{GrantTypesSupported: nil}
	if meta.SupportsGrant("authorization_code") {
		t.Error("SupportsGrant should return false when GrantTypesSupported is nil")
	}
}

func TestAuthServerMetadata_SupportsGrant_NoMatch(t *testing.T) {
	t.Parallel()

	meta := AuthServerMetadata{GrantTypesSupported: []string{"client_credentials"}}
	if meta.SupportsGrant("authorization_code") {
		t.Error("SupportsGrant should return false when grant is not in list")
	}
}

func TestDiscoverJWKSURL_MissingJWKSURI(t *testing.T) {
	t.Parallel()

	// Server returns valid metadata but with no jwks_uri
	meta := AuthServerMetadata{
		Issuer:                 "https://auth.example.com",
		AuthorizationEndpoint:  "https://auth.example.com/authorize",
		TokenEndpoint:          "https://auth.example.com/token",
		ResponseTypesSupported: []string{"code"},
		// JWKSURI intentionally empty
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(meta)
	}))
	defer srv.Close()

	_, err := DiscoverJWKSURL(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error when jwks_uri is missing")
	}
	if !strings.Contains(err.Error(), "no jwks_uri") {
		t.Errorf("error should mention 'no jwks_uri', got: %v", err)
	}
}

func TestDiscoverJWKSURL_DiscoveryFailure(t *testing.T) {
	t.Parallel()

	// Server returns 404 for both discovery endpoints
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	_, err := DiscoverJWKSURL(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error when discovery fails")
	}
}

// ---------------------------------------------------------------------------
// dpop.go — parseDPoPJWT, parseJWKPublicKey, parseECPublicJWK, verifyES256,
//            compareURIs uncovered paths
// ---------------------------------------------------------------------------

func TestParseDPoPJWT_InvalidHeaderJSON(t *testing.T) {
	t.Parallel()

	// Craft a JWT where the header base64-decodes to invalid JSON
	invalidHeaderEnc := base64.RawURLEncoding.EncodeToString([]byte("invalid-json"))
	validPayloadEnc := base64.RawURLEncoding.EncodeToString([]byte("{}"))
	token := invalidHeaderEnc + "." + validPayloadEnc + ".sig"

	_, _, err := parseDPoPJWT(token)
	if err == nil {
		t.Fatal("expected error for invalid header JSON")
	}
}

func TestParseDPoPJWT_InvalidPayloadJSON(t *testing.T) {
	t.Parallel()

	// Valid header JSON but payload base64-decodes to invalid JSON
	headerBytes, _ := json.Marshal(map[string]any{"typ": "dpop+jwt", "alg": "ES256"})
	validHeaderEnc := base64.RawURLEncoding.EncodeToString(headerBytes)
	invalidPayloadEnc := base64.RawURLEncoding.EncodeToString([]byte("invalid-json"))
	token := validHeaderEnc + "." + invalidPayloadEnc + ".sig"

	_, _, err := parseDPoPJWT(token)
	if err == nil {
		t.Fatal("expected error for invalid payload JSON")
	}
}

func TestParseDPoPJWT_TwoParts(t *testing.T) {
	t.Parallel()

	_, _, err := parseDPoPJWT("header.payload")
	if err == nil {
		t.Fatal("expected error for two-part JWT")
	}
}

func TestParseJWKPublicKey_UnsupportedKty(t *testing.T) {
	t.Parallel()

	jwk := map[string]any{"kty": "OKP", "crv": "Ed25519"}
	jwkBytes, _ := json.Marshal(jwk)

	_, _, err := parseJWKPublicKey(jwkBytes)
	if err == nil {
		t.Fatal("expected error for unsupported key type")
	}
	if !strings.Contains(err.Error(), "unsupported key type") {
		t.Errorf("error should mention 'unsupported key type': %v", err)
	}
}

func TestParseJWKPublicKey_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, _, err := parseJWKPublicKey([]byte("not-json"))
	if err == nil {
		t.Fatal("expected error for invalid JWK JSON")
	}
}

func TestParseRSAPublicJWK_MissingParams(t *testing.T) {
	t.Parallel()

	// Missing both n and e
	jwk := map[string]any{"kty": "RSA"}
	_, _, err := parseRSAPublicJWK(jwk)
	if err == nil {
		t.Fatal("expected error for missing RSA parameters")
	}
}

func TestParseECPublicJWK_UnsupportedCurve(t *testing.T) {
	t.Parallel()

	jwk := map[string]any{
		"kty": "EC",
		"crv": "P-384",
		"x":   "somevalue",
		"y":   "somevalue",
	}
	_, _, err := parseECPublicJWK(jwk)
	if err == nil {
		t.Fatal("expected error for unsupported EC curve")
	}
	if !strings.Contains(err.Error(), "unsupported curve") {
		t.Errorf("error should mention 'unsupported curve': %v", err)
	}
}

func TestParseECPublicJWK_MissingCoordinates(t *testing.T) {
	t.Parallel()

	// Correct curve but missing x and y
	jwk := map[string]any{
		"kty": "EC",
		"crv": "P-256",
		// x and y missing
	}
	_, _, err := parseECPublicJWK(jwk)
	if err == nil {
		t.Fatal("expected error for missing EC key parameters")
	}
}

func TestVerifyES256_InvalidSigLength(t *testing.T) {
	t.Parallel()

	key := generateECTestKey(t)

	// Build a valid proof but replace the signature with a wrong-length one.
	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "tok", nil)
	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		t.Fatal("unexpected proof format")
	}

	// Use a signature that decodes to only 32 bytes (half of expected 64)
	shortSigBytes := make([]byte, 32)
	shortSig := base64.RawURLEncoding.EncodeToString(shortSigBytes)
	tampered := parts[0] + "." + parts[1] + "." + shortSig

	v := newTestDPoPValidator()
	err := v.ValidateProof(tampered, "tok", "GET", "https://example.com/api")
	if !errors.Is(err, ErrDPoPInvalidProof) {
		t.Errorf("expected ErrDPoPInvalidProof for wrong sig length, got: %v", err)
	}
}

func TestCompareURIs_SchemeMismatch(t *testing.T) {
	t.Parallel()

	err := compareURIs("https://example.com/api", "http://example.com/api")
	if err == nil {
		t.Fatal("expected error for scheme mismatch")
	}
}

func TestCompareURIs_HostMismatch(t *testing.T) {
	t.Parallel()

	err := compareURIs("https://a.com/api", "https://b.com/api")
	if err == nil {
		t.Fatal("expected error for host mismatch")
	}
}

func TestCompareURIs_PathMismatch(t *testing.T) {
	t.Parallel()

	err := compareURIs("https://example.com/api/v1", "https://example.com/api/v2")
	if err == nil {
		t.Fatal("expected error for path mismatch")
	}
}

func TestCompareURIs_QueryStrippedFromComparison(t *testing.T) {
	t.Parallel()

	// Query params should be ignored — same host/path with different queries must match
	err := compareURIs("https://example.com/api?foo=bar", "https://example.com/api?baz=qux")
	if err != nil {
		t.Errorf("compareURIs should ignore query strings: %v", err)
	}
}

func TestDPoPValidator_UnsupportedAlgorithm(t *testing.T) {
	t.Parallel()

	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	// Override the alg to something unsupported
	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "tok", map[string]any{
		"alg": "HS256",
	})
	err := v.ValidateProof(proof, "tok", "GET", "https://example.com/api")
	if !errors.Is(err, ErrDPoPInvalidProof) {
		t.Errorf("expected ErrDPoPInvalidProof for unsupported algorithm, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// jwks.go — fetchKeys non-200 response, key filtering, nbf check
// ---------------------------------------------------------------------------

func TestJWKSValidator_FetchKeysNon200(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	validator := NewJWKSValidator(JWKSConfig{JWKSURL: srv.URL})
	_, err := validator.ValidateToken(context.Background(), "a.b.c")
	if err == nil {
		t.Fatal("expected error when JWKS endpoint returns non-200")
	}
}

func TestJWKSValidator_FetchKeysInvalidJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "not-json")
	}))
	defer srv.Close()

	validator := NewJWKSValidator(JWKSConfig{JWKSURL: srv.URL})
	_, err := validator.ValidateToken(context.Background(), "a.b.c")
	if err == nil {
		t.Fatal("expected error for invalid JWKS JSON response")
	}
}

func TestJWKSValidator_NbfCheck(t *testing.T) {
	t.Parallel()

	key := generateTestKey(t)
	kid := "nbf-test-key"

	jwksServer := serveJWKS(t, kid, &key.PublicKey)
	defer jwksServer.Close()

	validator := NewJWKSValidator(JWKSConfig{JWKSURL: jwksServer.URL})

	// Token with nbf in the future (not yet valid)
	token := signJWT(t, key,
		map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid},
		map[string]any{
			"sub": "alice",
			"exp": time.Now().Add(time.Hour).Unix(),
			"nbf": time.Now().Add(30 * time.Minute).Unix(),
		},
	)

	_, err := validator.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for token with future nbf")
	}
	if !strings.Contains(err.Error(), "not yet valid") {
		t.Errorf("expected 'not yet valid' error, got: %v", err)
	}
}

func TestJWKSValidator_KeysWithNonRSA(t *testing.T) {
	t.Parallel()

	// Serve a JWKS that contains a non-RSA key (kty != RSA) and verify it's skipped
	key := generateTestKey(t)
	kid := "rsa-key"

	eBytes := coverBigIntBytes(key.PublicKey.E)
	jwks := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "EC",
				"use": "sig",
				"kid": "ec-key",
				"crv": "P-256",
			},
			{
				"kty": "RSA",
				"use": "sig",
				"kid": kid,
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(eBytes),
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	validator := NewJWKSValidator(JWKSConfig{JWKSURL: srv.URL})

	token := signJWT(t, key,
		map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid},
		map[string]any{"sub": "alice", "exp": time.Now().Add(time.Hour).Unix()},
	)

	subject, err := validator.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if subject != "alice" {
		t.Errorf("subject = %q, want alice", subject)
	}
}

// ---------------------------------------------------------------------------
// oauth_client.go — AuthorizationURL no grant support, doTokenRequest HTTP error
// ---------------------------------------------------------------------------

func TestOAuthClient_AuthorizationURL_NoGrantSupport(t *testing.T) {
	t.Parallel()

	srv, discovery := oauthTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		meta := AuthServerMetadata{
			Issuer:                        "http://" + r.Host,
			AuthorizationEndpoint:         "http://" + r.Host + "/authorize",
			TokenEndpoint:                 "http://" + r.Host + "/token",
			ResponseTypesSupported:        []string{"code"},
			GrantTypesSupported:           []string{"client_credentials"}, // no authorization_code
			CodeChallengeMethodsSupported: []string{"S256"},
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
	if !errors.Is(err, ErrNoGrantSupport) {
		t.Errorf("expected ErrNoGrantSupport, got %v", err)
	}
}

func TestOAuthClient_AuthorizationURL_ExtraParams(t *testing.T) {
	t.Parallel()

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
	}, discovery)

	verifier, _ := PKCEVerifier()
	authURL, err := oc.AuthorizationURL(context.Background(), srv.URL, AuthorizationParams{
		CodeVerifier: verifier,
		Extra:        map[string]string{"custom_param": "custom_value"},
	})
	if err != nil {
		t.Fatalf("AuthorizationURL: %v", err)
	}
	if !strings.Contains(authURL, "custom_param=custom_value") {
		t.Errorf("authURL should contain custom_param, got: %s", authURL)
	}
}

func TestOAuthClient_ExchangeCode_DiscoveryError(t *testing.T) {
	t.Parallel()

	// Use a discovery client that points to an unreachable server
	discovery := NewMetadataDiscovery(DiscoveryConfig{
		HTTPClient: &http.Client{Timeout: 100 * time.Millisecond},
	})

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, discovery)

	_, err := oc.ExchangeCode(context.Background(), "http://localhost:0", "code", "verifier")
	if err == nil {
		t.Fatal("expected error when discovery fails")
	}
}

func TestOAuthClient_RefreshToken_DiscoveryError(t *testing.T) {
	t.Parallel()

	discovery := NewMetadataDiscovery(DiscoveryConfig{
		HTTPClient: &http.Client{Timeout: 100 * time.Millisecond},
	})

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, discovery)

	_, err := oc.RefreshToken(context.Background(), "http://localhost:0", "refresh-token")
	if err == nil {
		t.Fatal("expected error when discovery fails during refresh")
	}
}

func TestOAuthClient_DoTokenRequest_NonJSONError(t *testing.T) {
	t.Parallel()

	srv, discovery := oauthTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/") {
			meta := defaultMetadata("http://" + r.Host)
			json.NewEncoder(w).Encode(meta)
			return
		}
		if r.URL.Path == "/token" {
			// Return a non-JSON error body with non-200 status
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "internal server error (not json)")
			return
		}
		http.NotFound(w, r)
	})

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID:    "test-client",
		RedirectURI: "http://localhost:8080/callback",
	}, discovery)

	_, err := oc.ExchangeCode(context.Background(), srv.URL, "code", "verifier")
	if !errors.Is(err, ErrTokenExchange) {
		t.Errorf("expected ErrTokenExchange for non-JSON error response, got: %v", err)
	}
}

func TestOAuthClient_SetToken_NoExpiry(t *testing.T) {
	t.Parallel()

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, NewMetadataDiscovery(DiscoveryConfig{}))

	oc.SetToken(TokenResponse{
		AccessToken: "no-expiry-token",
		TokenType:   "Bearer",
		// ExpiresIn is 0 — should default to 1 hour
	})

	oc.mu.RLock()
	expiry := oc.tokenExpiry
	oc.mu.RUnlock()

	// Should be approximately 1 hour from now
	remaining := time.Until(expiry)
	if remaining < 55*time.Minute || remaining > 65*time.Minute {
		t.Errorf("expected expiry ~1 hour from now, got %v remaining", remaining)
	}
}

func TestOAuthClient_Token_NoCachedToken(t *testing.T) {
	t.Parallel()

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, NewMetadataDiscovery(DiscoveryConfig{}))

	// No token set at all
	_, err := oc.Token(context.Background(), "https://unused.example.com")
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired when no token is set, got: %v", err)
	}
}

func TestOAuthClient_Token_ExpiredWithFailedRefresh(t *testing.T) {
	t.Parallel()

	// Token server will reject the refresh
	srv, discovery := oauthTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/") {
			meta := defaultMetadata("http://" + r.Host)
			json.NewEncoder(w).Encode(meta)
			return
		}
		if r.URL.Path == "/token" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(TokenErrorResponse{
				Error:            "invalid_grant",
				ErrorDescription: "refresh token expired",
			})
			return
		}
		http.NotFound(w, r)
	})

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, discovery)

	oc.mu.Lock()
	oc.cachedToken = &TokenResponse{AccessToken: "expired-token"}
	oc.tokenExpiry = time.Now().Add(-time.Minute)
	oc.refreshToken = "bad-refresh-token"
	oc.mu.Unlock()

	_, err := oc.Token(context.Background(), srv.URL)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired when refresh fails, got: %v", err)
	}
}

func TestOAuthClient_SetToken_WithRefreshToken(t *testing.T) {
	t.Parallel()

	oc := NewOAuthClient(OAuthClientConfig{
		ClientID: "test-client",
	}, NewMetadataDiscovery(DiscoveryConfig{}))

	oc.SetToken(TokenResponse{
		AccessToken:  "access-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "my-refresh-token",
	})

	oc.mu.RLock()
	rt := oc.refreshToken
	oc.mu.RUnlock()

	if rt != "my-refresh-token" {
		t.Errorf("refreshToken = %q, want my-refresh-token", rt)
	}
}

// ---------------------------------------------------------------------------
// workload.go — AWSIMDSProvider role fetch error, cred fetch error
// ---------------------------------------------------------------------------

func TestAWSIMDSProvider_GetToken_RoleFetchError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/latest/api/token":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "mock-imds-token")
		case r.Method == http.MethodGet && r.URL.Path == "/latest/meta-data/iam/security-credentials/":
			// Return non-200 for role fetch
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := &AWSIMDSProvider{
		HTTPClient: &http.Client{Transport: rewriteTransport{target: srv.URL}},
	}

	_, err := p.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error when role fetch returns non-200")
	}
	if !strings.Contains(err.Error(), "aws imds role returned") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAWSIMDSProvider_GetToken_CredFetchError(t *testing.T) {
	t.Parallel()

	const mockRole = "test-role"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/latest/api/token":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "mock-imds-token")
		case r.Method == http.MethodGet && r.URL.Path == "/latest/meta-data/iam/security-credentials/":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, mockRole)
		case r.Method == http.MethodGet && r.URL.Path == "/latest/meta-data/iam/security-credentials/"+mockRole:
			// Return non-200 for credentials fetch
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := &AWSIMDSProvider{
		HTTPClient: &http.Client{Transport: rewriteTransport{target: srv.URL}},
	}

	_, err := p.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error when cred fetch returns non-200")
	}
	if !strings.Contains(err.Error(), "aws imds cred returned") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAWSIMDSProvider_GetToken_InvalidCredJSON(t *testing.T) {
	t.Parallel()

	const mockRole = "test-role"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/latest/api/token":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "mock-imds-token")
		case r.Method == http.MethodGet && r.URL.Path == "/latest/meta-data/iam/security-credentials/":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, mockRole)
		case r.Method == http.MethodGet && r.URL.Path == "/latest/meta-data/iam/security-credentials/"+mockRole:
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "not-valid-json")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := &AWSIMDSProvider{
		HTTPClient: &http.Client{Transport: rewriteTransport{target: srv.URL}},
	}

	_, err := p.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error when credential JSON is invalid")
	}
}

func TestAutoDetect_NoProviderDetected(t *testing.T) {
	t.Parallel()

	// AutoDetect connects to hardcoded endpoints (169.254.x.x / metadata.google.internal),
	// which are unreachable in test environments — so it should return an error.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := AutoDetect(ctx)
	if err == nil {
		t.Skip("AutoDetect returned a provider; may be running in a cloud environment")
	}
	if !strings.Contains(err.Error(), "no workload identity provider detected") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// jwt.go — parseJWT invalid JSON paths, verifyRS256 bad sig base64 / format
// ---------------------------------------------------------------------------

func TestParseJWT_InvalidPayloadJSON(t *testing.T) {
	t.Parallel()

	headerBytes, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": "k1"})
	validHeaderEnc := base64.RawURLEncoding.EncodeToString(headerBytes)
	invalidPayloadEnc := base64.RawURLEncoding.EncodeToString([]byte("invalid-json"))
	token := validHeaderEnc + "." + invalidPayloadEnc + ".sig"

	_, _, err := parseJWT(token)
	if err == nil {
		t.Fatal("expected error for invalid payload JSON")
	}
}

func TestParseJWT_InvalidHeaderJSON(t *testing.T) {
	t.Parallel()

	invalidHeaderEnc := base64.RawURLEncoding.EncodeToString([]byte("invalid-json"))
	validPayloadEnc := base64.RawURLEncoding.EncodeToString([]byte("{}"))
	token := invalidHeaderEnc + "." + validPayloadEnc + ".sig"

	_, _, err := parseJWT(token)
	if err == nil {
		t.Fatal("expected error for invalid header JSON")
	}
}

func TestVerifyRS256_BadSignatureBase64(t *testing.T) {
	t.Parallel()

	key := generateTestKey(t)

	headerJSON, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": "k1"})
	payloadJSON, _ := json.Marshal(map[string]any{"sub": "user"})

	headerEnc := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadJSON)
	invalidSig := "!!!invalid!!!"

	token := headerEnc + "." + payloadEnc + "." + invalidSig

	err := verifyRS256(token, &key.PublicKey)
	if err == nil {
		t.Fatal("expected error for invalid signature base64")
	}
}

func TestVerifyRS256_InvalidTokenFormat(t *testing.T) {
	t.Parallel()

	key := generateTestKey(t)
	err := verifyRS256("only-two.parts", &key.PublicKey)
	if err == nil {
		t.Fatal("expected error for invalid token format")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// coverBigIntBytes converts an int to a big-endian byte slice, suitable for encoding
// RSA public exponents in JWKS format.
func coverBigIntBytes(n int) []byte {
	if n == 0 {
		return []byte{0}
	}
	var result []byte
	for n > 0 {
		result = append([]byte{byte(n & 0xff)}, result...)
		n >>= 8
	}
	return result
}
