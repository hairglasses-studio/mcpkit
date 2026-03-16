package session_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hairglasses-studio/mcpkit/session"
)

func TestMiddleware_CreatesSession(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	var capturedID string
	handler := session.Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := session.FromContext(r.Context())
		if !ok {
			http.Error(w, "no session", http.StatusInternalServerError)
			return
		}
		capturedID = sess.ID()
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if capturedID == "" {
		t.Fatal("expected session ID to be set")
	}
	// Cookie should be set.
	cookies := rr.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == session.DefaultCookieName {
			found = true
			if c.Value != capturedID {
				t.Fatalf("cookie value %q != session ID %q", c.Value, capturedID)
			}
		}
	}
	if !found {
		t.Fatal("expected session cookie to be set")
	}
}

func TestMiddleware_ReusesExistingSession(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	// Create a session first.
	sess, _ := store.Create(nil) // nolint:staticcheck — intentional nil ctx for test
	// Use context.Background() to avoid nil deref in Create.
	sess2, _ := store.Create(nil)
	_ = sess2

	var capturedID string
	handler := session.Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		capturedID = s.ID()
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: session.DefaultCookieName, Value: sess.ID()})
	handler.ServeHTTP(rr, req)

	if capturedID != sess.ID() {
		t.Fatalf("got %q, want %q", capturedID, sess.ID())
	}
}

func TestTokenMiddleware_HeaderExtraction(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	existingSess, _ := store.Create(nil)

	var capturedID string
	handler := session.TokenMiddleware(store, session.TokenMiddlewareOptions{
		Header: "X-Session-Token",
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext(r.Context())
		if ok {
			capturedID = s.ID()
		}
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Session-Token", existingSess.ID())
	handler.ServeHTTP(rr, req)

	if capturedID != existingSess.ID() {
		t.Fatalf("got %q, want %q", capturedID, existingSess.ID())
	}
}
