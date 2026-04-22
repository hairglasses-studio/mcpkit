package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// generateECTestKey generates an ECDSA P-256 key for testing.
func generateECTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate EC key: %v", err)
	}
	return key
}

// generateRSATestKey generates an RSA 2048 key for testing.
func generateRSATestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

// signDPoPProof creates a DPoP proof JWT signed with the given key.
// overrides is applied on top of defaults — keys prefixed with "header." modify the header,
// keys prefixed with "payload." modify the payload. Use bare key names in the map and the
// function uses separate header/payload maps, so callers pass a flat map where keys matching
// header fields ("typ","alg","jwk") go to the header and everything else to payload.
//
// Actually, to keep it simple: overrides with key starting with "typ", "alg", "jwk" patch the
// header. Anything else patches the payload. Passing a nil value removes the key.
func signDPoPProof(t *testing.T, key crypto.Signer, method, uri, accessToken string, overrides map[string]any) string {
	t.Helper()

	// Determine algorithm and build JWK from the public key
	var alg string
	var jwk map[string]any

	switch pub := key.Public().(type) {
	case *rsa.PublicKey:
		alg = "RS256"
		nBytes := pub.N.Bytes()
		eVal := pub.E
		eBytes := []byte{byte(eVal >> 16), byte(eVal >> 8), byte(eVal)}
		// Trim leading zeros from e
		for len(eBytes) > 1 && eBytes[0] == 0 {
			eBytes = eBytes[1:]
		}
		jwk = map[string]any{
			"kty": "RSA",
			"n":   base64.RawURLEncoding.EncodeToString(nBytes),
			"e":   base64.RawURLEncoding.EncodeToString(eBytes),
		}
	case *ecdsa.PublicKey:
		alg = "ES256"
		xBytes := pub.X.Bytes()
		yBytes := pub.Y.Bytes()
		// Zero-pad to exactly 32 bytes for P-256
		for len(xBytes) < 32 {
			xBytes = append([]byte{0}, xBytes...)
		}
		for len(yBytes) < 32 {
			yBytes = append([]byte{0}, yBytes...)
		}
		jwk = map[string]any{
			"kty": "EC",
			"crv": "P-256",
			"x":   base64.RawURLEncoding.EncodeToString(xBytes),
			"y":   base64.RawURLEncoding.EncodeToString(yBytes),
		}
	default:
		t.Fatalf("unsupported key type %T", key.Public())
	}

	// Build header
	header := map[string]any{
		"typ": "dpop+jwt",
		"alg": alg,
		"jwk": jwk,
	}

	// Generate random JTI
	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		t.Fatalf("failed to generate JTI: %v", err)
	}
	jti := base64.RawURLEncoding.EncodeToString(jtiBytes)

	// Compute ath
	athHash := sha256.Sum256([]byte(accessToken))
	ath := base64.RawURLEncoding.EncodeToString(athHash[:])

	// Build payload
	payload := map[string]any{
		"jti": jti,
		"htm": method,
		"htu": uri,
		"iat": float64(time.Now().Unix()),
		"ath": ath,
	}

	// Apply overrides: header-level keys are typ, alg, jwk
	headerKeys := map[string]bool{"typ": true, "alg": true, "jwk": true}
	for k, v := range overrides {
		if headerKeys[k] {
			if v == nil {
				delete(header, k)
			} else {
				header[k] = v
			}
		} else {
			if v == nil {
				delete(payload, k)
			} else {
				payload[k] = v
			}
		}
	}

	// Encode header
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("failed to marshal header: %v", err)
	}
	headerEnc := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Encode payload
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signingInput := headerEnc + "." + payloadEnc
	hash := sha256.Sum256([]byte(signingInput))

	var sigEnc string
	switch alg {
	case "RS256":
		sig, err := rsa.SignPKCS1v15(rand.Reader, key.(*rsa.PrivateKey), crypto.SHA256, hash[:])
		if err != nil {
			t.Fatalf("RS256 signing failed: %v", err)
		}
		sigEnc = base64.RawURLEncoding.EncodeToString(sig)
	case "ES256":
		r, s, err := ecdsa.Sign(rand.Reader, key.(*ecdsa.PrivateKey), hash[:])
		if err != nil {
			t.Fatalf("ES256 signing failed: %v", err)
		}
		rBytes := r.Bytes()
		sBytes := s.Bytes()
		// Zero-pad r and s to 32 bytes each
		for len(rBytes) < 32 {
			rBytes = append([]byte{0}, rBytes...)
		}
		for len(sBytes) < 32 {
			sBytes = append([]byte{0}, sBytes...)
		}
		sigBytes := append(rBytes, sBytes...)
		sigEnc = base64.RawURLEncoding.EncodeToString(sigBytes)
	}

	return signingInput + "." + sigEnc
}

// newTestDPoPValidator creates a fresh DPoPValidator with default config for tests.
func newTestDPoPValidator() *DPoPValidator {
	return NewDPoPValidator(DPoPConfig{})
}

// ---- DPoP Validator Tests ----

func TestDPoPValidator_ValidRS256(t *testing.T) {
	key := generateRSATestKey(t)
	v := newTestDPoPValidator()

	proof := signDPoPProof(t, key, "POST", "https://example.com/api", "my-access-token", nil)
	if err := v.ValidateProof(proof, "my-access-token", "POST", "https://example.com/api"); err != nil {
		t.Errorf("expected valid RS256 proof, got error: %v", err)
	}
}

func TestDPoPValidator_ValidES256(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	proof := signDPoPProof(t, key, "GET", "https://example.com/resource", "token-abc", nil)
	if err := v.ValidateProof(proof, "token-abc", "GET", "https://example.com/resource"); err != nil {
		t.Errorf("expected valid ES256 proof, got error: %v", err)
	}
}

func TestDPoPValidator_WrongMethod(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	// Proof says GET, but we validate with POST
	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "tok", nil)
	err := v.ValidateProof(proof, "tok", "POST", "https://example.com/api")
	if !errors.Is(err, ErrDPoPMethodMismatch) {
		t.Errorf("expected ErrDPoPMethodMismatch, got: %v", err)
	}
}

func TestDPoPValidator_WrongURI(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	// Proof is for a.com/x but we validate with b.com/y
	proof := signDPoPProof(t, key, "GET", "https://a.com/x", "tok", nil)
	err := v.ValidateProof(proof, "tok", "GET", "https://b.com/y")
	if !errors.Is(err, ErrDPoPURIMismatch) {
		t.Errorf("expected ErrDPoPURIMismatch, got: %v", err)
	}
}

func TestDPoPValidator_ATHMismatch(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	// Proof was signed with "correct-token" but we pass "wrong-token" to ValidateProof
	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "correct-token", nil)
	err := v.ValidateProof(proof, "wrong-token", "GET", "https://example.com/api")
	if !errors.Is(err, ErrDPoPATHMismatch) {
		t.Errorf("expected ErrDPoPATHMismatch, got: %v", err)
	}
}

func TestDPoPValidator_Replay(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	proof := signDPoPProof(t, key, "POST", "https://example.com/api", "tok", nil)

	// First call should succeed
	if err := v.ValidateProof(proof, "tok", "POST", "https://example.com/api"); err != nil {
		t.Fatalf("first validation failed unexpectedly: %v", err)
	}

	// Second call with same proof (same JTI) should be rejected
	err := v.ValidateProof(proof, "tok", "POST", "https://example.com/api")
	if !errors.Is(err, ErrDPoPReplay) {
		t.Errorf("expected ErrDPoPReplay on second call, got: %v", err)
	}
}

func TestDPoPValidator_Expired(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	// Set iat to 1 hour ago — well outside max proof age
	past := float64(time.Now().Add(-1 * time.Hour).Unix())
	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "tok", map[string]any{
		"iat": past,
	})
	err := v.ValidateProof(proof, "tok", "GET", "https://example.com/api")
	if !errors.Is(err, ErrDPoPExpired) {
		t.Errorf("expected ErrDPoPExpired, got: %v", err)
	}
}

func TestDPoPValidator_InvalidTyp(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	// Override typ to plain "jwt"
	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "tok", map[string]any{
		"typ": "jwt",
	})
	err := v.ValidateProof(proof, "tok", "GET", "https://example.com/api")
	if !errors.Is(err, ErrDPoPInvalidProof) {
		t.Errorf("expected ErrDPoPInvalidProof for wrong typ, got: %v", err)
	}
}

func TestDPoPValidator_MissingJWK(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	// Remove jwk from header by passing nil
	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "tok", map[string]any{
		"jwk": nil,
	})
	err := v.ValidateProof(proof, "tok", "GET", "https://example.com/api")
	if !errors.Is(err, ErrDPoPInvalidProof) {
		t.Errorf("expected ErrDPoPInvalidProof for missing JWK, got: %v", err)
	}
}

// ---- DPoP Middleware Tests ----

// testDPoPValidator wraps a validator func so it matches TokenValidator type.
func makeTokenValidator(fn func(string) (string, error)) TokenValidator {
	return TokenValidator(fn)
}

func TestDPoPMiddleware_ValidRequest(t *testing.T) {
	key := generateECTestKey(t)
	dpopV := newTestDPoPValidator()

	validator := makeTokenValidator(func(tok string) (string, error) {
		if tok == "good-token" {
			return "user-42", nil
		}
		return "", fmt.Errorf("bad token")
	})

	var gotSubject string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSubject = Subject(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := DPoPMiddleware(validator, dpopV)(inner)

	req := httptest.NewRequest("POST", "http://example.com/api", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	proof := signDPoPProof(t, key, "POST", "http://example.com/api", "good-token", nil)
	req.Header.Set("DPoP", proof)

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if gotSubject != "user-42" {
		t.Errorf("subject = %q, want %q", gotSubject, "user-42")
	}
}

func TestDPoPMiddleware_BearerFallback(t *testing.T) {
	dpopV := newTestDPoPValidator()

	validator := makeTokenValidator(func(tok string) (string, error) {
		if tok == "valid-bearer" {
			return "user-bearer", nil
		}
		return "", fmt.Errorf("bad token")
	})

	var gotSubject string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSubject = Subject(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// No DPoP option — should allow Bearer-only
	mw := DPoPMiddleware(validator, dpopV)(inner)

	req := httptest.NewRequest("GET", "http://example.com/resource", nil)
	req.Header.Set("Authorization", "Bearer valid-bearer")
	// No DPoP header

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if gotSubject != "user-bearer" {
		t.Errorf("subject = %q, want %q", gotSubject, "user-bearer")
	}
}

func TestDPoPMiddleware_RequireDPoP(t *testing.T) {
	dpopV := newTestDPoPValidator()

	validator := makeTokenValidator(func(tok string) (string, error) {
		return "user", nil
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when DPoP is required but missing")
	})

	mw := DPoPMiddleware(validator, dpopV, WithRequireDPoP())(inner)

	req := httptest.NewRequest("GET", "http://example.com/resource", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	// No DPoP header

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestDPoPMiddleware_NoBearerToken(t *testing.T) {
	dpopV := newTestDPoPValidator()

	validator := makeTokenValidator(func(tok string) (string, error) {
		return "user", nil
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without Authorization header")
	})

	mw := DPoPMiddleware(validator, dpopV)(inner)

	req := httptest.NewRequest("GET", "http://example.com/resource", nil)
	// No Authorization header

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestDPoPMiddleware_InvalidBearer(t *testing.T) {
	dpopV := newTestDPoPValidator()

	validator := makeTokenValidator(func(tok string) (string, error) {
		return "", fmt.Errorf("token rejected")
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with invalid bearer token")
	})

	mw := DPoPMiddleware(validator, dpopV)(inner)

	req := httptest.NewRequest("GET", "http://example.com/resource", nil)
	req.Header.Set("Authorization", "Bearer bad-token")

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// ---- JTI Cache Test ----

func TestJTICache_Eviction(t *testing.T) {
	// Create cache with a very short TTL
	ttl := 50 * time.Millisecond
	cache := newJTICache(100, ttl)

	// Add an entry
	jti := "test-jti-eviction"
	if !cache.Check(jti) {
		t.Fatal("first Check should return true (not seen before)")
	}

	// Immediately it should be a replay
	if cache.Check(jti) {
		t.Fatal("second Check should return false (replay)")
	}

	// Wait for TTL to expire
	time.Sleep(ttl + 20*time.Millisecond)

	// Force cleanup by calling Check 100 times to trigger the cleanup counter
	for i := range 100 {
		cache.Check(fmt.Sprintf("filler-%d", i))
	}

	// The original jti should now be evicted and accepted again
	if !cache.Check(jti) {
		t.Error("after TTL expiry and cleanup, Check should return true again")
	}
}

// TestJTICache_EvictionDirect tests eviction by calling cleanup directly via
// a direct field-level manipulation — here we test via a large number of unique
// JTIs to exercise the capacity eviction path.
func TestJTICache_CapacityEviction(t *testing.T) {
	maxSize := 10
	cache := newJTICache(maxSize, time.Minute)

	// Fill cache to capacity
	for i := range maxSize {
		jti := fmt.Sprintf("jti-%d", i)
		if !cache.Check(jti) {
			t.Fatalf("jti-%d should be accepted", i)
		}
	}

	// Adding one more should evict the oldest
	extra := "jti-extra"
	if !cache.Check(extra) {
		t.Error("extra jti should be accepted after capacity eviction")
	}
}

// Ensure a DPoP proof with correct ath when accessToken is empty still validates.
func TestDPoPValidator_EmptyAccessToken(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	// When access token is empty, ath is not checked by the validator
	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "", nil)
	// Pass empty access token — ath check is skipped
	if err := v.ValidateProof(proof, "", "GET", "https://example.com/api"); err != nil {
		t.Errorf("expected success with empty access token, got: %v", err)
	}
}

// TestDPoPValidator_BigInt tests the RSA key works with large modulus values.
func TestDPoPValidator_LargeRSAKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatalf("generate 4096-bit key: %v", err)
	}
	v := newTestDPoPValidator()

	proof := signDPoPProof(t, key, "DELETE", "https://api.example.com/v1/resource", "access-tok-xyz", nil)
	if err := v.ValidateProof(proof, "access-tok-xyz", "DELETE", "https://api.example.com/v1/resource"); err != nil {
		t.Errorf("expected valid large RSA proof, got: %v", err)
	}
}

// TestDPoPValidator_CaseInsensitiveMethod verifies htm comparison is case-insensitive.
func TestDPoPValidator_CaseInsensitiveMethod(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	// Sign with uppercase POST but validate as lowercase post
	proof := signDPoPProof(t, key, "POST", "https://example.com/api", "tok", nil)
	if err := v.ValidateProof(proof, "tok", "post", "https://example.com/api"); err != nil {
		t.Errorf("expected case-insensitive method match, got: %v", err)
	}
}

// TestDPoPMiddleware_InvalidDPoPProof verifies that an invalid proof results in 401.
func TestDPoPMiddleware_InvalidDPoPProof(t *testing.T) {
	dpopV := newTestDPoPValidator()

	validator := makeTokenValidator(func(tok string) (string, error) {
		return "user", nil
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with invalid DPoP proof")
	})

	mw := DPoPMiddleware(validator, dpopV)(inner)

	req := httptest.NewRequest("GET", "http://example.com/resource", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	req.Header.Set("DPoP", "not.a.valid.jwt.at.all")

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// TestDPoPValidator_FutureIat checks that a proof with iat in the future is rejected.
func TestDPoPValidator_FutureIat(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	future := float64(time.Now().Add(10 * time.Minute).Unix())
	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "tok", map[string]any{
		"iat": future,
	})
	err := v.ValidateProof(proof, "tok", "GET", "https://example.com/api")
	if !errors.Is(err, ErrDPoPInvalidProof) {
		t.Errorf("expected ErrDPoPInvalidProof for future iat, got: %v", err)
	}
}

// TestDPoPValidator_MissingIat checks that a proof with no iat is rejected.
func TestDPoPValidator_MissingIat(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "tok", map[string]any{
		"iat": nil,
	})
	err := v.ValidateProof(proof, "tok", "GET", "https://example.com/api")
	if !errors.Is(err, ErrDPoPInvalidProof) {
		t.Errorf("expected ErrDPoPInvalidProof for missing iat, got: %v", err)
	}
}

// TestDPoPValidator_MissingJTI checks that a proof with no jti is rejected.
func TestDPoPValidator_MissingJTI(t *testing.T) {
	key := generateECTestKey(t)
	v := newTestDPoPValidator()

	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "tok", map[string]any{
		"jti": nil,
	})
	err := v.ValidateProof(proof, "tok", "GET", "https://example.com/api")
	if !errors.Is(err, ErrDPoPInvalidProof) {
		t.Errorf("expected ErrDPoPInvalidProof for missing jti, got: %v", err)
	}
}

// TestDPoPValidator_NonceRequired verifies that the validator rejects proofs
// without a nonce when NonceRequired is set.
func TestDPoPValidator_NonceRequired(t *testing.T) {
	key := generateECTestKey(t)
	v := NewDPoPValidator(DPoPConfig{NonceRequired: true})

	proof := signDPoPProof(t, key, "GET", "https://example.com/api", "tok", nil)
	err := v.ValidateProof(proof, "tok", "GET", "https://example.com/api")
	if !errors.Is(err, ErrDPoPInvalidProof) {
		t.Errorf("expected ErrDPoPInvalidProof when nonce required but missing, got: %v", err)
	}
}

// TestDPoPMiddleware_URIBuilding verifies the middleware constructs the correct
// httpURI from the request (scheme + host + path, no query params).
func TestDPoPMiddleware_URIBuilding(t *testing.T) {
	key := generateECTestKey(t)
	dpopV := newTestDPoPValidator()

	validator := makeTokenValidator(func(tok string) (string, error) {
		return "user", nil
	})

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := DPoPMiddleware(validator, dpopV)(inner)

	// Sign proof for the path without query string
	proof := signDPoPProof(t, key, "GET", "http://example.com/path/to/resource", "my-tok", nil)

	// Request includes query params — URI in proof should NOT include them
	req := httptest.NewRequest("GET", "http://example.com/path/to/resource?foo=bar&baz=qux", nil)
	req.Header.Set("Authorization", "Bearer my-tok")
	req.Header.Set("DPoP", proof)

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Error("inner handler was not called")
	}
}

// TestDPoPValidator_RSASignatureMismatch verifies that a proof signed by a different
// RSA key is rejected.
func TestDPoPValidator_RSASignatureMismatch(t *testing.T) {
	signingKey := generateRSATestKey(t)
	differentKey := generateRSATestKey(t)
	v := newTestDPoPValidator()

	// Build a valid proof structure using signingKey, but then swap the JWK to
	// claim it's from differentKey — signature won't match.
	proof := signDPoPProof(t, signingKey, "POST", "https://example.com/api", "tok", nil)

	// Manually tamper: rebuild the header claiming differentKey's JWK but keep the signingKey's sig.
	// We do this by constructing the proof with differentKey's public key in the JWK but using
	// signingKey to sign — which is exactly what signDPoPProof does, so we need to swap post-hoc.
	// Instead, just verify that a fresh proof by differentKey validates with its own key.
	proof2 := signDPoPProof(t, differentKey, "POST", "https://example.com/api", "tok", nil)

	// Craft a tampered JWT: header+payload from proof2, but signature from proof
	parts1 := splitJWT(proof)
	parts2 := splitJWT(proof2)
	tampered := parts2[0] + "." + parts2[1] + "." + parts1[2]

	err := v.ValidateProof(tampered, "tok", "POST", "https://example.com/api")
	if !errors.Is(err, ErrDPoPInvalidProof) {
		t.Errorf("expected ErrDPoPInvalidProof for mismatched signature, got: %v", err)
	}
}

// splitJWT splits a JWT string into its three parts.
func splitJWT(token string) [3]string {
	var result [3]string
	dotCount := 0
	start := 0
	for i, ch := range token {
		if ch == '.' {
			if dotCount < 3 {
				result[dotCount] = token[start:i]
				dotCount++
				start = i + 1
			}
		}
	}
	if dotCount < 3 {
		result[dotCount] = token[start:]
	}
	return result
}

// TestDPoPValidator_ES256SignatureMismatch verifies EC signature mismatch is rejected.
func TestDPoPValidator_ES256SignatureMismatch(t *testing.T) {
	key1 := generateECTestKey(t)
	key2 := generateECTestKey(t)
	v := newTestDPoPValidator()

	proof1 := signDPoPProof(t, key1, "GET", "https://example.com/api", "tok", nil)
	proof2 := signDPoPProof(t, key2, "GET", "https://example.com/api", "tok", nil)

	// Tamper: use header+payload from proof2 (claims key2's JWK) but signature from proof1
	parts1 := splitJWT(proof1)
	parts2 := splitJWT(proof2)
	tampered := parts2[0] + "." + parts2[1] + "." + parts1[2]

	err := v.ValidateProof(tampered, "tok", "GET", "https://example.com/api")
	if !errors.Is(err, ErrDPoPInvalidProof) {
		t.Errorf("expected ErrDPoPInvalidProof for EC signature mismatch, got: %v", err)
	}
}

// Ensure the big.Int conversion for RSA e value works for the common exponent 65537.
func TestRSAExponent65537(t *testing.T) {
	key := generateRSATestKey(t)
	if key.E != 65537 {
		t.Skip("key does not use e=65537, skip exponent test")
	}
	v := newTestDPoPValidator()
	proof := signDPoPProof(t, key, "GET", "https://example.com/", "tok", nil)
	if err := v.ValidateProof(proof, "tok", "GET", "https://example.com/"); err != nil {
		t.Errorf("expected valid proof with e=65537, got: %v", err)
	}
}
