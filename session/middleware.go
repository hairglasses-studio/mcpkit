package session

import (
	"context"
	"net/http"
)

// DefaultCookieName is the cookie name used by Middleware to carry session IDs.
const DefaultCookieName = "mcpkit_session"

// Middleware returns an HTTP middleware that creates or retrieves a session
// for each request and stores it in the request context. The session ID is
// read from and written to the DefaultCookieName cookie. If no session
// exists for the given ID, a new one is created.
//
// Example:
//
//	store := session.NewMemStore(session.Options{TTL: 30 * time.Minute})
//	http.Handle("/", session.Middleware(store)(myHandler))
func Middleware(store SessionStore) func(http.Handler) http.Handler {
	return MiddlewareWithCookie(store, DefaultCookieName)
}

// MiddlewareWithCookie is like Middleware but allows specifying a custom cookie name.
func MiddlewareWithCookie(store SessionStore, cookieName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			var sess Session

			// Try to load existing session from cookie.
			if cookie, err := r.Cookie(cookieName); err == nil && cookie.Value != "" {
				if s, ok, _ := store.Get(ctx, cookie.Value); ok {
					sess = s
				}
			}

			// Create a new session if none was found.
			if sess == nil {
				var err error
				sess, err = store.Create(ctx)
				if err != nil {
					http.Error(w, "session error", http.StatusInternalServerError)
					return
				}
				http.SetCookie(w, &http.Cookie{
					Name:     cookieName,
					Value:    sess.ID(),
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
				})
			}

			// Store session in context and proceed.
			next.ServeHTTP(w, r.WithContext(WithSession(ctx, sess)))
		})
	}
}

// TokenMiddlewareOptions configures TokenMiddleware.
type TokenMiddlewareOptions struct {
	// Header is the HTTP header name to read the session token from.
	// When set, this takes precedence over the cookie.
	Header string
	// CookieName overrides the default cookie name for token extraction.
	// If empty, DefaultCookieName is used.
	CookieName string
}

// TokenMiddleware returns an HTTP middleware that extracts a session token
// from a configurable header (and optionally a cookie). If a token is found
// and resolves to an existing session, that session is attached to the context.
// No new session is created if the token is missing or invalid.
func TokenMiddleware(store SessionStore, opts TokenMiddlewareOptions) func(http.Handler) http.Handler {
	cookieName := opts.CookieName
	if cookieName == "" {
		cookieName = DefaultCookieName
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			token := ""
			if opts.Header != "" {
				token = r.Header.Get(opts.Header)
			}
			if token == "" {
				if c, err := r.Cookie(cookieName); err == nil {
					token = c.Value
				}
			}

			if token != "" {
				if sess, ok, _ := store.Get(ctx, token); ok {
					ctx = WithSession(ctx, sess)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MiddlewareFunc is like Middleware but works with a context-based handler
// function instead of http.Handler. Useful for lightweight wrappers.
func MiddlewareFunc(store SessionStore, fn func(context.Context, http.ResponseWriter, *http.Request)) http.Handler {
	return Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fn(r.Context(), w, r)
	}))
}

// RequireSession is middleware that rejects requests with no valid session.
// It returns 401 Unauthorized if no session is found in the context.
func RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := FromContext(r.Context()); !ok {
			http.Error(w, "session required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
