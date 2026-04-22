package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key
}

func signJWT(t *testing.T, key *rsa.PrivateKey, header, payload map[string]any) string {
	t.Helper()
	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	h := base64.RawURLEncoding.EncodeToString(headerJSON)
	p := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := h + "." + p

	hash := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, 0x05, hash[:]) // crypto.SHA256 = 0x05
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func serveJWKS(t *testing.T, kid string, pub *rsa.PublicKey) *httptest.Server {
	t.Helper()
	jwks := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": kid,
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
}

func TestParseJWT(t *testing.T) {
	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": "key1"}
	payload := map[string]any{"sub": "user-1", "exp": time.Now().Add(time.Hour).Unix()}

	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	token := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON) + ".fakesig"

	h, p, err := parseJWT(token)
	if err != nil {
		t.Fatalf("parseJWT: %v", err)
	}
	if h.Kid != "key1" {
		t.Errorf("kid = %q, want key1", h.Kid)
	}
	if p.Sub != "user-1" {
		t.Errorf("sub = %q, want user-1", p.Sub)
	}
}

func TestParseJWT_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"one part", "abc"},
		{"two parts", "abc.def"},
		{"bad base64 header", "!!!.def.ghi"},
		{"bad base64 payload", "eyJhbGciOiJSUzI1NiJ9.!!!.ghi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseJWT(tt.token)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestJWKSValidator_ValidToken(t *testing.T) {
	key := generateTestKey(t)
	kid := "test-key-1"

	jwksServer := serveJWKS(t, kid, &key.PublicKey)
	defer jwksServer.Close()

	validator := NewJWKSValidator(JWKSConfig{JWKSURL: jwksServer.URL})

	token := signJWT(t, key,
		map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid},
		map[string]any{
			"sub": "alice",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(),
		},
	)

	subject, err := validator.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if subject != "alice" {
		t.Errorf("subject = %q, want alice", subject)
	}
}

func TestJWKSValidator_ExpiredToken(t *testing.T) {
	key := generateTestKey(t)
	kid := "test-key-1"

	jwksServer := serveJWKS(t, kid, &key.PublicKey)
	defer jwksServer.Close()

	validator := NewJWKSValidator(JWKSConfig{JWKSURL: jwksServer.URL})

	token := signJWT(t, key,
		map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid},
		map[string]any{
			"sub": "alice",
			"exp": time.Now().Add(-time.Hour).Unix(),
		},
	)

	_, err := validator.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestJWKSValidator_UnknownKey(t *testing.T) {
	key := generateTestKey(t)

	jwksServer := serveJWKS(t, "known-key", &key.PublicKey)
	defer jwksServer.Close()

	validator := NewJWKSValidator(JWKSConfig{JWKSURL: jwksServer.URL})

	token := signJWT(t, key,
		map[string]any{"alg": "RS256", "typ": "JWT", "kid": "unknown-key"},
		map[string]any{"sub": "alice", "exp": time.Now().Add(time.Hour).Unix()},
	)

	_, err := validator.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for unknown key ID")
	}
}

func TestJWKSValidator_WrongSignature(t *testing.T) {
	key := generateTestKey(t)
	otherKey := generateTestKey(t)
	kid := "test-key-1"

	// Serve key1 but sign with key2
	jwksServer := serveJWKS(t, kid, &key.PublicKey)
	defer jwksServer.Close()

	validator := NewJWKSValidator(JWKSConfig{JWKSURL: jwksServer.URL})

	token := signJWT(t, otherKey,
		map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid},
		map[string]any{"sub": "alice", "exp": time.Now().Add(time.Hour).Unix()},
	)

	_, err := validator.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for wrong signature")
	}
}

func TestJWKSValidator_NoSubject(t *testing.T) {
	key := generateTestKey(t)
	kid := "test-key-1"

	jwksServer := serveJWKS(t, kid, &key.PublicKey)
	defer jwksServer.Close()

	validator := NewJWKSValidator(JWKSConfig{JWKSURL: jwksServer.URL})

	token := signJWT(t, key,
		map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid},
		map[string]any{"exp": time.Now().Add(time.Hour).Unix()},
	)

	_, err := validator.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for missing subject")
	}
}

func TestJWKSValidator_TokenValidatorFunc(t *testing.T) {
	key := generateTestKey(t)
	kid := "test-key-1"

	jwksServer := serveJWKS(t, kid, &key.PublicKey)
	defer jwksServer.Close()

	validator := NewJWKSValidator(JWKSConfig{JWKSURL: jwksServer.URL})
	fn := validator.TokenValidator()

	token := signJWT(t, key,
		map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid},
		map[string]any{"sub": "bob", "exp": time.Now().Add(time.Hour).Unix()},
	)

	subject, err := fn(token)
	if err != nil {
		t.Fatalf("TokenValidator: %v", err)
	}
	if subject != "bob" {
		t.Errorf("subject = %q, want bob", subject)
	}
}

func TestJWKSValidator_CachesKeys(t *testing.T) {
	key := generateTestKey(t)
	kid := "test-key-1"

	fetchCount := 0
	jwks := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA", "use": "sig", "kid": kid, "alg": "RS256",
				"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	validator := NewJWKSValidator(JWKSConfig{JWKSURL: srv.URL, CacheTTL: time.Hour})

	for i := range 3 {
		token := signJWT(t, key,
			map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid},
			map[string]any{"sub": fmt.Sprintf("user-%d", i), "exp": time.Now().Add(time.Hour).Unix()},
		)
		_, err := validator.ValidateToken(context.Background(), token)
		if err != nil {
			t.Fatalf("validation %d: %v", i, err)
		}
	}

	if fetchCount != 1 {
		t.Errorf("JWKS fetched %d times, expected 1 (cached)", fetchCount)
	}
}
