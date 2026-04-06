package transport_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/transport"
)

func TestSessionExtractor_BearerToken(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer sess-abc-123")

	id, err := ext.Extract(r)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if id != "sess-abc-123" {
		t.Errorf("expected 'sess-abc-123', got %q", id)
	}
}

func TestSessionExtractor_BearerTokenEmpty(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer ")

	// Empty bearer token — should fall through to next method.
	_, err := ext.Extract(r)
	if err == nil {
		t.Fatal("expected error for empty bearer token with no other sources")
	}
}

func TestSessionExtractor_CustomHeader(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Session-ID", "header-sess-456")

	id, err := ext.Extract(r)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if id != "header-sess-456" {
		t.Errorf("expected 'header-sess-456', got %q", id)
	}
}

func TestSessionExtractor_CustomHeaderName(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor(transport.SessionExtractorConfig{
		HeaderName: "X-Custom-Session",
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Custom-Session", "custom-789")

	id, err := ext.Extract(r)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if id != "custom-789" {
		t.Errorf("expected 'custom-789', got %q", id)
	}
}

func TestSessionExtractor_Cookie(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "mcp_session", Value: "cookie-sess-001"})

	id, err := ext.Extract(r)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if id != "cookie-sess-001" {
		t.Errorf("expected 'cookie-sess-001', got %q", id)
	}
}

func TestSessionExtractor_CustomCookieName(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor(transport.SessionExtractorConfig{
		CookieName: "my_session",
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "my_session", Value: "custom-cookie-002"})

	id, err := ext.Extract(r)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if id != "custom-cookie-002" {
		t.Errorf("expected 'custom-cookie-002', got %q", id)
	}
}

func TestSessionExtractor_QueryParam(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	r := httptest.NewRequest(http.MethodGet, "/?session_id=query-sess-999", nil)

	id, err := ext.Extract(r)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if id != "query-sess-999" {
		t.Errorf("expected 'query-sess-999', got %q", id)
	}
}

func TestSessionExtractor_CustomQueryParam(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor(transport.SessionExtractorConfig{
		QueryParam: "sid",
	})

	r := httptest.NewRequest(http.MethodGet, "/?sid=custom-query-003", nil)

	id, err := ext.Extract(r)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if id != "custom-query-003" {
		t.Errorf("expected 'custom-query-003', got %q", id)
	}
}

func TestSessionExtractor_NoSessionID(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := ext.Extract(r)
	if err == nil {
		t.Fatal("expected ErrNoSessionID")
	}
	if err != transport.ErrNoSessionID {
		t.Errorf("expected ErrNoSessionID, got %v", err)
	}
}

func TestSessionExtractor_PriorityOrder(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	// All sources present — Bearer should win.
	r := httptest.NewRequest(http.MethodGet, "/?session_id=from-query", nil)
	r.Header.Set("Authorization", "Bearer from-bearer")
	r.Header.Set("X-Session-ID", "from-header")
	r.AddCookie(&http.Cookie{Name: "mcp_session", Value: "from-cookie"})

	id, err := ext.Extract(r)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if id != "from-bearer" {
		t.Errorf("expected bearer to win, got %q", id)
	}

	// Remove bearer — custom header should win.
	r2 := httptest.NewRequest(http.MethodGet, "/?session_id=from-query", nil)
	r2.Header.Set("X-Session-ID", "from-header")
	r2.AddCookie(&http.Cookie{Name: "mcp_session", Value: "from-cookie"})

	id, err = ext.Extract(r2)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if id != "from-header" {
		t.Errorf("expected header to win, got %q", id)
	}

	// Remove header — cookie should win.
	r3 := httptest.NewRequest(http.MethodGet, "/?session_id=from-query", nil)
	r3.AddCookie(&http.Cookie{Name: "mcp_session", Value: "from-cookie"})

	id, err = ext.Extract(r3)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if id != "from-cookie" {
		t.Errorf("expected cookie to win, got %q", id)
	}
}

func TestSessionExtractor_SetCookie(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	w := httptest.NewRecorder()
	ext.SetCookie(w, "test-session-id")

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != "mcp_session" {
		t.Errorf("cookie name: got %q, want 'mcp_session'", c.Name)
	}
	if c.Value != "test-session-id" {
		t.Errorf("cookie value: got %q, want 'test-session-id'", c.Value)
	}
	if !c.HttpOnly {
		t.Error("expected HttpOnly flag")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSiteLax, got %v", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("expected path '/', got %q", c.Path)
	}
	// MaxAge should be 24 hours in seconds.
	expectedMaxAge := int((24 * time.Hour).Seconds())
	if c.MaxAge != expectedMaxAge {
		t.Errorf("expected MaxAge %d, got %d", expectedMaxAge, c.MaxAge)
	}
}

func TestSessionExtractor_SetCookieCustom(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor(transport.SessionExtractorConfig{
		CookieName:   "custom_sess",
		CookieTTL:    time.Hour,
		CookieSecure: true,
		CookiePath:   "/api",
	})

	w := httptest.NewRecorder()
	ext.SetCookie(w, "secure-session")

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != "custom_sess" {
		t.Errorf("cookie name: got %q", c.Name)
	}
	if c.Value != "secure-session" {
		t.Errorf("cookie value: got %q", c.Value)
	}
	if !c.Secure {
		t.Error("expected Secure flag")
	}
	if c.Path != "/api" {
		t.Errorf("expected path '/api', got %q", c.Path)
	}
	expectedMaxAge := int(time.Hour.Seconds())
	if c.MaxAge != expectedMaxAge {
		t.Errorf("expected MaxAge %d, got %d", expectedMaxAge, c.MaxAge)
	}
}

func TestSessionExtractor_ClearCookie(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	w := httptest.NewRecorder()
	ext.ClearCookie(w)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != "mcp_session" {
		t.Errorf("cookie name: got %q", c.Name)
	}
	if c.Value != "" {
		t.Errorf("expected empty value, got %q", c.Value)
	}
	if c.MaxAge != -1 {
		t.Errorf("expected MaxAge -1, got %d", c.MaxAge)
	}
}

func TestSessionExtractor_EmptyCookieValue(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "mcp_session", Value: ""})

	// Empty cookie should be skipped.
	_, err := ext.Extract(r)
	if err == nil {
		t.Fatal("expected ErrNoSessionID for empty cookie value")
	}
}

func TestSessionExtractor_AuthorizationNotBearer(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	// Not a Bearer token — should fall through.
	_, err := ext.Extract(r)
	if err == nil {
		t.Fatal("expected ErrNoSessionID for non-Bearer auth")
	}
}

func TestSessionExtractor_ShortAuthorizationHeader(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bear")

	// Too short for Bearer — should fall through.
	_, err := ext.Extract(r)
	if err == nil {
		t.Fatal("expected ErrNoSessionID for short auth header")
	}
}

func TestNewSessionExtractor_Defaults(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	// Verify defaults by testing extraction behavior.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Session-ID", "default-header-test")

	id, err := ext.Extract(r)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if id != "default-header-test" {
		t.Errorf("expected 'default-header-test', got %q", id)
	}
}

func TestNewSessionExtractor_NoConfig(t *testing.T) {
	t.Parallel()
	// Calling with no args should use defaults.
	ext := transport.NewSessionExtractor()
	if ext == nil {
		t.Fatal("expected non-nil extractor")
	}
}
