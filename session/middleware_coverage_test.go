package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hairglasses-studio/mcpkit/session"
)

func TestMiddlewareFunc_ContextPropagation(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	var capturedSess session.Session
	h := session.MiddlewareFunc(store, func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext(ctx)
		if !ok {
			http.Error(w, "no session", http.StatusInternalServerError)
			return
		}
		capturedSess = s
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if capturedSess == nil || capturedSess.ID() == "" {
		t.Fatal("expected session to be propagated via context")
	}
}

func TestRequireSession_WithSession(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain Middleware (creates session) → RequireSession → inner handler
	handler := session.Middleware(store)(session.RequireSession(inner))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with session present, got %d", rr.Code)
	}
}

func TestRequireSession_WithoutSession(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// RequireSession without Middleware — no session in context.
	handler := session.RequireSession(inner)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", rr.Code)
	}
}

func TestTokenMiddleware_CookieFallback(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	existingSess, _ := store.Create(context.Background())

	var capturedID string
	handler := session.TokenMiddleware(store, session.TokenMiddlewareOptions{
		Header: "X-Session-Token",
		// No cookie name override — uses default.
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext(r.Context())
		if ok {
			capturedID = s.ID()
		}
		w.WriteHeader(http.StatusOK)
	}))

	// No header, but session ID in cookie — should fall back.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: session.DefaultCookieName, Value: existingSess.ID()})
	handler.ServeHTTP(rr, req)

	if capturedID != existingSess.ID() {
		t.Fatalf("expected cookie fallback to find session %q, got %q", existingSess.ID(), capturedID)
	}
}

func TestTokenMiddleware_NoToken(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	var hasSess bool
	handler := session.TokenMiddleware(store, session.TokenMiddlewareOptions{
		Header: "X-Session-Token",
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasSess = session.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	if hasSess {
		t.Fatal("expected no session when no token provided")
	}
}

func TestTokenMiddleware_InvalidToken(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	var hasSess bool
	handler := session.TokenMiddleware(store, session.TokenMiddlewareOptions{
		Header: "X-Session-Token",
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasSess = session.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Session-Token", "nonexistent-id")
	handler.ServeHTTP(rr, req)

	if hasSess {
		t.Fatal("expected no session for invalid token")
	}
}

func TestTokenMiddleware_CustomCookieName(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	existingSess, _ := store.Create(context.Background())

	var capturedID string
	handler := session.TokenMiddleware(store, session.TokenMiddlewareOptions{
		CookieName: "my_session",
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext(r.Context())
		if ok {
			capturedID = s.ID()
		}
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "my_session", Value: existingSess.ID()})
	handler.ServeHTTP(rr, req)

	if capturedID != existingSess.ID() {
		t.Fatalf("expected session from custom cookie, got %q", capturedID)
	}
}

func TestMiddlewareWithCookie_InvalidCookieCreatesNew(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	var capturedID string
	handler := session.MiddlewareWithCookie(store, "custom_sess")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext(r.Context())
		if !ok {
			http.Error(w, "no session", http.StatusInternalServerError)
			return
		}
		capturedID = s.ID()
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "custom_sess", Value: "nonexistent-session-id"})
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if capturedID == "" {
		t.Fatal("expected a new session to be created")
	}
	if capturedID == "nonexistent-session-id" {
		t.Fatal("expected a NEW session, not the invalid one")
	}
}
