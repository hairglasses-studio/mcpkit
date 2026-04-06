package transport

import (
	"errors"
	"net/http"
	"time"
)

// ErrNoSessionID is returned when no session ID could be extracted from the request.
var ErrNoSessionID = errors.New("transport: no session ID found")

// SessionExtractorConfig configures a SessionExtractor.
type SessionExtractorConfig struct {
	// HeaderName is the custom header to check for session IDs.
	// Default: "X-Session-ID".
	HeaderName string
	// QueryParam is the URL query parameter to check for session IDs.
	// Default: "session_id".
	QueryParam string
	// CookieName is the cookie name to read/write session IDs.
	// Default: "mcp_session".
	CookieName string
	// CookieTTL is how long the session cookie should live.
	// Default: 24 hours.
	CookieTTL time.Duration
	// CookieSecure sets the Secure flag on the cookie.
	// Default: true.
	CookieSecure bool
	// CookiePath is the Path attribute for the cookie.
	// Default: "/".
	CookiePath string
}

func (c *SessionExtractorConfig) applyDefaults() {
	if c.HeaderName == "" {
		c.HeaderName = "X-Session-ID"
	}
	if c.QueryParam == "" {
		c.QueryParam = "session_id"
	}
	if c.CookieName == "" {
		c.CookieName = "mcp_session"
	}
	if c.CookieTTL == 0 {
		c.CookieTTL = 24 * time.Hour
	}
	if c.CookiePath == "" {
		c.CookiePath = "/"
	}
}

// SessionExtractor extracts session IDs from incoming HTTP requests.
// It checks multiple sources in priority order:
//  1. Authorization header (Bearer token)
//  2. Custom header (X-Session-ID by default)
//  3. Cookie (mcp_session by default)
//  4. Query parameter (session_id by default)
type SessionExtractor struct {
	config SessionExtractorConfig
}

// NewSessionExtractor creates a new SessionExtractor with the given config.
// If config is nil, sensible defaults are used.
func NewSessionExtractor(config ...SessionExtractorConfig) *SessionExtractor {
	var cfg SessionExtractorConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg.applyDefaults()
	return &SessionExtractor{config: cfg}
}

// Extract extracts a session ID from the HTTP request. It checks multiple
// sources in priority order and returns the first non-empty value found.
// Returns ErrNoSessionID if no session ID is present in any source.
func (e *SessionExtractor) Extract(r *http.Request) (string, error) {
	// 1. Authorization header (Bearer token).
	if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
		if token := auth[7:]; token != "" {
			return token, nil
		}
	}

	// 2. Custom header (X-Session-ID by default).
	if id := r.Header.Get(e.config.HeaderName); id != "" {
		return id, nil
	}

	// 3. Cookie.
	if cookie, err := r.Cookie(e.config.CookieName); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	// 4. Query parameter.
	if id := r.URL.Query().Get(e.config.QueryParam); id != "" {
		return id, nil
	}

	return "", ErrNoSessionID
}

// SetCookie writes a session ID cookie to the response.
func (e *SessionExtractor) SetCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     e.config.CookieName,
		Value:    sessionID,
		Path:     e.config.CookiePath,
		MaxAge:   int(e.config.CookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   e.config.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCookie removes the session cookie from the response.
func (e *SessionExtractor) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     e.config.CookieName,
		Value:    "",
		Path:     e.config.CookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   e.config.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
