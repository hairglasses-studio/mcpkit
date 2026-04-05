package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"
)

var (
	ErrDPoPInvalidProof   = errors.New("invalid DPoP proof")
	ErrDPoPReplay         = errors.New("DPoP proof replay detected")
	ErrDPoPExpired        = errors.New("DPoP proof expired")
	ErrDPoPMethodMismatch = errors.New("DPoP method mismatch")
	ErrDPoPURIMismatch    = errors.New("DPoP URI mismatch")
	ErrDPoPATHMismatch    = errors.New("DPoP access token hash mismatch")
)

// DPoPConfig configures DPoP proof validation behavior.
type DPoPConfig struct {
	MaxClockSkew  time.Duration // Default: 60s
	MaxProofAge   time.Duration // Default: 120s
	NonceRequired bool
}

// DPoPValidator validates DPoP proofs per RFC 9449.
type DPoPValidator struct {
	config   DPoPConfig
	jtiCache *jtiCache
}

// NewDPoPValidator creates a new DPoP validator.
func NewDPoPValidator(cfg DPoPConfig) *DPoPValidator {
	if cfg.MaxClockSkew == 0 {
		cfg.MaxClockSkew = 60 * time.Second
	}
	if cfg.MaxProofAge == 0 {
		cfg.MaxProofAge = 120 * time.Second
	}
	return &DPoPValidator{
		config:   cfg,
		jtiCache: newJTICache(10000, cfg.MaxProofAge+cfg.MaxClockSkew),
	}
}

// ValidateProof validates a DPoP proof JWT.
// proof is the raw DPoP proof JWT from the DPoP header.
// accessToken is the access token from the Authorization header (for ath claim binding).
// httpMethod is the HTTP method of the request (e.g., "POST").
// httpURI is the HTTP URI of the request (scheme + host + path, no query/fragment).
func (v *DPoPValidator) ValidateProof(proof, accessToken, httpMethod, httpURI string) error {
	header, payload, err := parseDPoPJWT(proof)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDPoPInvalidProof, err)
	}

	// Verify typ
	typ, _ := header["typ"].(string)
	if !strings.EqualFold(typ, "dpop+jwt") {
		return fmt.Errorf("%w: invalid typ %q", ErrDPoPInvalidProof, typ)
	}

	// Verify alg
	alg, _ := header["alg"].(string)
	if alg != "RS256" && alg != "ES256" {
		return fmt.Errorf("%w: unsupported algorithm %q", ErrDPoPInvalidProof, alg)
	}

	// Extract and verify JWK from header
	jwkRaw, ok := header["jwk"]
	if !ok {
		return fmt.Errorf("%w: missing jwk in header", ErrDPoPInvalidProof)
	}
	jwkBytes, err := json.Marshal(jwkRaw)
	if err != nil {
		return fmt.Errorf("%w: invalid jwk", ErrDPoPInvalidProof)
	}

	pubKey, keyAlg, err := parseJWKPublicKey(jwkBytes)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDPoPInvalidProof, err)
	}

	// Verify alg matches key type
	if alg != keyAlg {
		return fmt.Errorf("%w: algorithm mismatch: header %q, key %q", ErrDPoPInvalidProof, alg, keyAlg)
	}

	// Verify signature
	switch alg {
	case "RS256":
		rsaKey, ok := pubKey.(rsaPublicKey)
		if !ok {
			return fmt.Errorf("%w: expected RSA key for RS256", ErrDPoPInvalidProof)
		}
		if err := verifyRS256(proof, rsaKey.key); err != nil {
			return fmt.Errorf("%w: signature verification failed", ErrDPoPInvalidProof)
		}
	case "ES256":
		ecKey, ok := pubKey.(ecPublicKey)
		if !ok {
			return fmt.Errorf("%w: expected EC key for ES256", ErrDPoPInvalidProof)
		}
		if err := verifyES256(proof, ecKey.key); err != nil {
			return fmt.Errorf("%w: signature verification failed", ErrDPoPInvalidProof)
		}
	}

	// Verify claims
	jti, _ := payload["jti"].(string)
	if jti == "" {
		return fmt.Errorf("%w: missing jti", ErrDPoPInvalidProof)
	}

	htm, _ := payload["htm"].(string)
	if !strings.EqualFold(htm, httpMethod) {
		return fmt.Errorf("%w: expected %q, got %q", ErrDPoPMethodMismatch, httpMethod, htm)
	}

	htu, _ := payload["htu"].(string)
	if err := compareURIs(htu, httpURI); err != nil {
		return fmt.Errorf("%w: %v", ErrDPoPURIMismatch, err)
	}

	// Verify iat
	iatFloat, ok := payload["iat"].(float64)
	if !ok {
		return fmt.Errorf("%w: missing iat", ErrDPoPInvalidProof)
	}
	iat := time.Unix(int64(iatFloat), 0)
	now := time.Now()
	if now.Sub(iat) > v.config.MaxProofAge+v.config.MaxClockSkew {
		return ErrDPoPExpired
	}
	if iat.Sub(now) > v.config.MaxClockSkew {
		return fmt.Errorf("%w: proof issued in the future", ErrDPoPInvalidProof)
	}

	// Verify ath (access token hash)
	if accessToken != "" {
		ath, _ := payload["ath"].(string)
		expectedATH := computeATH(accessToken)
		if ath != expectedATH {
			return ErrDPoPATHMismatch
		}
	}

	// Verify nonce if required
	if v.config.NonceRequired {
		nonce, _ := payload["nonce"].(string)
		if nonce == "" {
			return fmt.Errorf("%w: missing required nonce", ErrDPoPInvalidProof)
		}
	}

	// Replay detection
	if !v.jtiCache.Check(jti) {
		return ErrDPoPReplay
	}

	return nil
}

// parseDPoPJWT parses a DPoP proof JWT and returns header and payload as maps.
func parseDPoPJWT(token string) (header, payload map[string]any, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, fmt.Errorf("invalid JWT format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid header encoding: %w", err)
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, nil, fmt.Errorf("invalid header JSON: %w", err)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid payload encoding: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, nil, fmt.Errorf("invalid payload JSON: %w", err)
	}

	return header, payload, nil
}

// Wrapper types carry parsed public keys through parseJWKPublicKey.
type rsaPublicKey struct{ key *rsa.PublicKey }
type ecPublicKey struct{ key *ecdsa.PublicKey }

// parseJWKPublicKey parses a JWK JSON into a tagged public key and returns the matching algorithm.
func parseJWKPublicKey(jwkBytes json.RawMessage) (any, string, error) {
	var jwk map[string]any
	if err := json.Unmarshal(jwkBytes, &jwk); err != nil {
		return nil, "", fmt.Errorf("invalid JWK: %w", err)
	}

	kty, _ := jwk["kty"].(string)
	switch kty {
	case "RSA":
		key, alg, err := parseRSAPublicJWK(jwk)
		if err != nil {
			return nil, "", err
		}
		return rsaPublicKey{key: key}, alg, nil
	case "EC":
		key, alg, err := parseECPublicJWK(jwk)
		if err != nil {
			return nil, "", err
		}
		return ecPublicKey{key: key}, alg, nil
	default:
		return nil, "", fmt.Errorf("unsupported key type: %s", kty)
	}
}

func parseRSAPublicJWK(jwk map[string]any) (*rsa.PublicKey, string, error) {
	nStr, _ := jwk["n"].(string)
	eStr, _ := jwk["e"].(string)
	if nStr == "" || eStr == "" {
		return nil, "", fmt.Errorf("missing RSA key parameters")
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, "", fmt.Errorf("invalid n parameter: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, "", fmt.Errorf("invalid e parameter: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, "RS256", nil
}

func parseECPublicJWK(jwk map[string]any) (*ecdsa.PublicKey, string, error) {
	crv, _ := jwk["crv"].(string)
	if crv != "P-256" {
		return nil, "", fmt.Errorf("unsupported curve: %s", crv)
	}

	xStr, _ := jwk["x"].(string)
	yStr, _ := jwk["y"].(string)
	if xStr == "" || yStr == "" {
		return nil, "", fmt.Errorf("missing EC key parameters")
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(xStr)
	if err != nil {
		return nil, "", fmt.Errorf("invalid x parameter: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(yStr)
	if err != nil {
		return nil, "", fmt.Errorf("invalid y parameter: %w", err)
	}

	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)

	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}, "ES256", nil
}

// verifyES256 verifies an ES256 (ECDSA P-256 SHA-256) signature on a JWT.
func verifyES256(tokenStr string, key *ecdsa.PublicKey) error {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid token format")
	}
	signingInput := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	// ES256 signature is r || s, each 32 bytes
	if len(sigBytes) != 64 {
		return fmt.Errorf("invalid ES256 signature length: %d", len(sigBytes))
	}

	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])

	hash := sha256.Sum256([]byte(signingInput))
	if !ecdsa.Verify(key, hash[:], r, s) {
		return fmt.Errorf("ES256 signature verification failed")
	}
	return nil
}

// computeATH computes the access token hash: base64url(SHA-256(access_token)).
func computeATH(accessToken string) string {
	hash := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// compareURIs compares two URIs ignoring query and fragment components.
func compareURIs(htu, requestURI string) error {
	htuParsed, err := url.Parse(htu)
	if err != nil {
		return fmt.Errorf("invalid htu: %w", err)
	}
	reqParsed, err := url.Parse(requestURI)
	if err != nil {
		return fmt.Errorf("invalid request URI: %w", err)
	}

	// Compare scheme + host + path (case-insensitive for scheme and host)
	if !strings.EqualFold(htuParsed.Scheme, reqParsed.Scheme) ||
		!strings.EqualFold(htuParsed.Host, reqParsed.Host) ||
		htuParsed.Path != reqParsed.Path {
		return fmt.Errorf("URI mismatch: %q vs %q", htu, requestURI)
	}
	return nil
}
