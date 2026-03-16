// Package session provides session management for MCP servers.
//
// It includes a Session interface and in-memory SessionStore with TTL-based
// eviction, HTTP middleware for cookie-based session handling, token-based
// session extraction, and context helpers for passing sessions through
// request pipelines.
//
// Basic usage:
//
//	store := session.NewMemStore(session.Options{TTL: 30 * time.Minute})
//	defer store.Close()
//
//	http.Handle("/", session.Middleware(store)(myHandler))
//
// Session data can be read from context inside handlers:
//
//	sess, ok := session.FromContext(r.Context())
//	if ok {
//	    sess.Set("user_id", "alice")
//	}
package session
