package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// JWKSValidator validates JWT tokens by fetching keys from a JWKS endpoint.
// It caches the key set and refreshes periodically.
type JWKSValidator struct {
	jwksURL    string
	client     *http.Client
	mu         sync.RWMutex
	keys       map[string]*rsa.PublicKey
	lastFetch  time.Time
	cacheTTL   time.Duration
}

// JWKSConfig configures JWKS-based token validation.
type JWKSConfig struct {
	// JWKSURL is the URL of the JWKS endpoint (e.g., "https://auth.example.com/.well-known/jwks.json").
	JWKSURL string

	// CacheTTL controls how long the JWKS response is cached. Default: 1 hour.
	CacheTTL time.Duration

	// HTTPClient is an optional HTTP client for fetching JWKS. Default: 10s timeout.
	HTTPClient *http.Client
}

// NewJWKSValidator creates a validator that fetches public keys from a JWKS endpoint.
func NewJWKSValidator(cfg JWKSConfig) *JWKSValidator {
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = time.Hour
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &JWKSValidator{
		jwksURL:  cfg.JWKSURL,
		client:   cfg.HTTPClient,
		keys:     make(map[string]*rsa.PublicKey),
		cacheTTL: cfg.CacheTTL,
	}
}

// TokenValidator returns a TokenValidator function for use with Middleware.
// It validates the JWT signature against the JWKS keys and extracts the subject.
func (v *JWKSValidator) TokenValidator() TokenValidator {
	return func(token string) (string, error) {
		return v.ValidateToken(context.Background(), token)
	}
}

// ValidateToken validates a JWT token string and returns the subject claim.
func (v *JWKSValidator) ValidateToken(ctx context.Context, tokenStr string) (string, error) {
	header, payload, err := parseJWT(tokenStr)
	if err != nil {
		return "", fmt.Errorf("invalid JWT: %w", err)
	}

	// Get signing key
	key, err := v.getKey(ctx, header.Kid)
	if err != nil {
		return "", fmt.Errorf("key lookup failed: %w", err)
	}

	// Verify signature
	if err := verifyRS256(tokenStr, key); err != nil {
		return "", fmt.Errorf("signature verification failed: %w", err)
	}

	// Check expiration
	if payload.Exp > 0 && time.Now().Unix() > payload.Exp {
		return "", fmt.Errorf("token expired")
	}

	// Check not-before
	if payload.Nbf > 0 && time.Now().Unix() < payload.Nbf {
		return "", fmt.Errorf("token not yet valid")
	}

	if payload.Sub == "" {
		return "", fmt.Errorf("token has no subject claim")
	}

	return payload.Sub, nil
}

func (v *JWKSValidator) getKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	if key, ok := v.keys[kid]; ok && time.Since(v.lastFetch) < v.cacheTTL {
		v.mu.RUnlock()
		return key, nil
	}
	v.mu.RUnlock()

	// Refresh keys
	if err := v.fetchKeys(ctx); err != nil {
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("key %q not found in JWKS", kid)
	}
	return key, nil
}

func (v *JWKSValidator) fetchKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("create JWKS request: %w", err)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []jwkEntry `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decode JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pub, err := k.toRSAPublicKey()
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	v.mu.Lock()
	v.keys = keys
	v.lastFetch = time.Now()
	v.mu.Unlock()

	return nil
}

type jwkEntry struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (k *jwkEntry) toRSAPublicKey() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}
