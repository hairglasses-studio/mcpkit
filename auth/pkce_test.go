package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestPKCEVerifier_ValidBase64URL(t *testing.T) {
	v, err := PKCEVerifier()
	if err != nil {
		t.Fatalf("PKCEVerifier() error: %v", err)
	}
	if strings.ContainsAny(v, "+/=") {
		t.Errorf("PKCEVerifier() contains standard base64 chars (+, /, =): %q", v)
	}
	// Confirm it decodes cleanly as raw URL encoding
	if _, err := base64.RawURLEncoding.DecodeString(v); err != nil {
		t.Errorf("PKCEVerifier() not valid base64url: %v", err)
	}
}

func TestPKCEChallenge_Deterministic(t *testing.T) {
	verifier := "test-verifier-string"
	c1 := PKCEChallenge(verifier)
	c2 := PKCEChallenge(verifier)
	if c1 != c2 {
		t.Errorf("PKCEChallenge() not deterministic: %q != %q", c1, c2)
	}
}

func TestPKCEChallenge_ValidBase64URL(t *testing.T) {
	c := PKCEChallenge("some-verifier")
	if strings.ContainsAny(c, "+/=") {
		t.Errorf("PKCEChallenge() contains standard base64 chars (+, /, =): %q", c)
	}
	if _, err := base64.RawURLEncoding.DecodeString(c); err != nil {
		t.Errorf("PKCEChallenge() not valid base64url: %v", err)
	}
}

func TestPKCEChallenge_S256Correctness(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	h := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])

	got := PKCEChallenge(verifier)
	if got != expected {
		t.Errorf("PKCEChallenge() = %q, want %q", got, expected)
	}
}

func TestPKCE_RoundTrip(t *testing.T) {
	verifier, err := PKCEVerifier()
	if err != nil {
		t.Fatalf("PKCEVerifier() error: %v", err)
	}

	challenge := PKCEChallenge(verifier)

	// Manually verify: base64url(SHA256(verifier)) must equal challenge
	h := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])

	if challenge != expected {
		t.Errorf("PKCE round-trip failed: challenge %q != expected %q", challenge, expected)
	}
}
