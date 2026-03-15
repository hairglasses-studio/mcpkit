package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCEVerifier generates a cryptographically random PKCE code verifier.
// Returns a 43-character base64url-encoded string (32 random bytes).
func PKCEVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// PKCEChallenge computes the S256 PKCE code challenge for a given verifier.
// challenge = base64url(SHA256(verifier))
func PKCEChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
