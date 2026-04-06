package transport_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/transport"
)

func TestValidateOrigin_Allowed(t *testing.T) {
	t.Parallel()
	allowed := []string{"https://example.com", "https://app.example.com"}

	tests := []struct {
		origin string
		want   bool
	}{
		{"https://example.com", true},
		{"https://app.example.com", true},
		{"https://EXAMPLE.COM", true}, // case-insensitive
		{"https://evil.com", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := transport.ValidateOrigin(tt.origin, allowed); got != tt.want {
			t.Errorf("ValidateOrigin(%q, allowed) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}

func TestValidateOrigin_EmptyAllowAll(t *testing.T) {
	t.Parallel()
	// Empty allowed list permits everything.
	if !transport.ValidateOrigin("https://anything.com", nil) {
		t.Error("expected empty allowed list to permit all origins")
	}
	if !transport.ValidateOrigin("", nil) {
		t.Error("expected empty allowed list to permit empty origin")
	}
}

func TestValidateOrigin_Wildcard(t *testing.T) {
	t.Parallel()
	allowed := []string{"*"}
	if !transport.ValidateOrigin("https://anything.com", allowed) {
		t.Error("expected wildcard to permit any origin")
	}
	if !transport.ValidateOrigin("", allowed) {
		t.Error("expected wildcard to permit empty origin")
	}
}

func TestSecureUpgradeHandler_OriginBlocked(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := transport.WebSocketSecurityConfig{
		AllowedOrigins: []string{"https://trusted.com"},
	}
	handler := transport.SecureUpgradeHandler(cfg, inner)

	// Request from an untrusted origin.
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", rec.Code)
	}
}

func TestSecureUpgradeHandler_OriginAllowed(t *testing.T) {
	t.Parallel()

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	cfg := transport.WebSocketSecurityConfig{
		AllowedOrigins: []string{"https://trusted.com"},
	}
	handler := transport.SecureUpgradeHandler(cfg, inner)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "https://trusted.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
	if !called {
		t.Error("expected inner handler to be called")
	}
}

func TestSecureUpgradeHandler_AuthRequired_NoToken(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := transport.WebSocketSecurityConfig{
		RequireAuth: true,
	}
	handler := transport.SecureUpgradeHandler(cfg, inner)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}
}

func TestSecureUpgradeHandler_AuthRequired_InvalidToken(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := transport.WebSocketSecurityConfig{
		RequireAuth: true,
		AuthValidator: func(token string) error {
			if token != "valid-token" {
				return errors.New("invalid token")
			}
			return nil
		},
	}
	handler := transport.SecureUpgradeHandler(cfg, inner)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}
}

func TestSecureUpgradeHandler_AuthValid_BearerToken(t *testing.T) {
	t.Parallel()

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	cfg := transport.WebSocketSecurityConfig{
		RequireAuth: true,
		AuthValidator: func(token string) error {
			if token != "valid-token" {
				return errors.New("invalid token")
			}
			return nil
		},
	}
	handler := transport.SecureUpgradeHandler(cfg, inner)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
	if !called {
		t.Error("expected inner handler to be called for valid token")
	}
}

func TestSecureUpgradeHandler_AuthValid_QueryToken(t *testing.T) {
	t.Parallel()

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	cfg := transport.WebSocketSecurityConfig{
		RequireAuth: true,
		AuthValidator: func(token string) error {
			if token != "query-token" {
				return errors.New("invalid token")
			}
			return nil
		},
	}
	handler := transport.SecureUpgradeHandler(cfg, inner)

	req := httptest.NewRequest(http.MethodGet, "/ws?token=query-token", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
	if !called {
		t.Error("expected inner handler to be called for valid query token")
	}
}

func TestSecureUpgradeHandler_NoAuthRequired_ValidatorStillRuns(t *testing.T) {
	t.Parallel()

	// When auth is not required but a validator is set, a provided token
	// should still be validated.
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := transport.WebSocketSecurityConfig{
		RequireAuth: false,
		AuthValidator: func(token string) error {
			return errors.New("bad token")
		},
	}
	handler := transport.SecureUpgradeHandler(cfg, inner)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for bad optional token, got %d", rec.Code)
	}
}

func TestSecureUpgradeHandler_NoAuthRequired_NoToken_Passes(t *testing.T) {
	t.Parallel()

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	cfg := transport.WebSocketSecurityConfig{
		RequireAuth: false,
		AuthValidator: func(token string) error {
			return errors.New("should not be called")
		},
	}
	handler := transport.SecureUpgradeHandler(cfg, inner)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK when no token and auth not required, got %d", rec.Code)
	}
	if !called {
		t.Error("expected inner handler to be called")
	}
}

func TestConnectionRateLimiter_WithinLimit(t *testing.T) {
	t.Parallel()

	rl := transport.NewConnectionRateLimiter(5)
	for i := range 5 {
		if !rl.Allow() {
			t.Errorf("expected Allow() = true for message %d", i+1)
		}
	}
}

func TestConnectionRateLimiter_Exceeded(t *testing.T) {
	t.Parallel()

	rl := transport.NewConnectionRateLimiter(3)
	// Exhaust all tokens.
	for range 3 {
		rl.Allow()
	}
	// Next should be denied.
	if rl.Allow() {
		t.Error("expected Allow() = false after exceeding rate limit")
	}
}

func TestConnectionRateLimiter_RefillsAfterSecond(t *testing.T) {
	t.Parallel()

	now := time.Now()
	rl := transport.NewConnectionRateLimiter(2)

	// Override clock for deterministic testing.
	rl.SetClock(func() time.Time { return now })
	rl.Reset() // re-init with the test clock

	// Exhaust tokens.
	rl.Allow()
	rl.Allow()
	if rl.Allow() {
		t.Error("expected denial after exhausting tokens")
	}

	// Advance clock past 1 second — tokens should refill.
	rl.SetClock(func() time.Time { return now.Add(1100 * time.Millisecond) })

	if !rl.Allow() {
		t.Error("expected Allow() = true after token refill")
	}
}

func TestConnectionRateLimiter_Reset(t *testing.T) {
	t.Parallel()

	rl := transport.NewConnectionRateLimiter(2)

	rl.Allow()
	rl.Allow()
	if rl.Allow() {
		t.Error("expected denial after exhausting tokens")
	}

	rl.Reset()
	if !rl.Allow() {
		t.Error("expected Allow() = true after Reset()")
	}
}

func TestSecureUpgradeHandler_ConfigInContext(t *testing.T) {
	t.Parallel()

	var gotCfg *transport.WebSocketSecurityConfig
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCfg = transport.SecurityConfigFromContext(r)
		w.WriteHeader(http.StatusOK)
	})

	cfg := transport.WebSocketSecurityConfig{
		MaxMessageSize:   512 * 1024,
		MessageRateLimit: 50,
		IdleTimeout:      2 * time.Minute,
	}
	handler := transport.SecureUpgradeHandler(cfg, inner)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if gotCfg == nil {
		t.Fatal("expected security config in context")
	}
	if gotCfg.MaxMessageSize != 512*1024 {
		t.Errorf("MaxMessageSize = %d, want %d", gotCfg.MaxMessageSize, 512*1024)
	}
	if gotCfg.MessageRateLimit != 50 {
		t.Errorf("MessageRateLimit = %d, want 50", gotCfg.MessageRateLimit)
	}
	if gotCfg.IdleTimeout != 2*time.Minute {
		t.Errorf("IdleTimeout = %v, want 2m", gotCfg.IdleTimeout)
	}
}

func TestSecureUpgradeHandler_DefaultValues(t *testing.T) {
	t.Parallel()

	var gotCfg *transport.WebSocketSecurityConfig
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCfg = transport.SecurityConfigFromContext(r)
		w.WriteHeader(http.StatusOK)
	})

	// Empty config — defaults should be applied.
	cfg := transport.WebSocketSecurityConfig{}
	handler := transport.SecureUpgradeHandler(cfg, inner)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if gotCfg == nil {
		t.Fatal("expected security config in context")
	}
	if gotCfg.MaxMessageSize != 1<<20 {
		t.Errorf("default MaxMessageSize = %d, want %d", gotCfg.MaxMessageSize, 1<<20)
	}
	if gotCfg.MessageRateLimit != 100 {
		t.Errorf("default MessageRateLimit = %d, want 100", gotCfg.MessageRateLimit)
	}
	if gotCfg.IdleTimeout != 5*time.Minute {
		t.Errorf("default IdleTimeout = %v, want 5m", gotCfg.IdleTimeout)
	}
}

func TestSecurityConfigFromContext_Missing(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	cfg := transport.SecurityConfigFromContext(req)
	if cfg != nil {
		t.Error("expected nil config when not set in context")
	}
}
