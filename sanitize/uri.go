package sanitize

import (
	"fmt"
	"net/url"
	"strings"
)

// URIPolicy defines validation rules for resource URIs.
type URIPolicy struct {
	// AllowedSchemes restricts which URI schemes are permitted. An empty slice
	// means all schemes are allowed.
	AllowedSchemes []string

	// BlockedHosts is a list of host values that are not permitted in URIs,
	// used to defend against SSRF attacks.
	BlockedHosts []string

	// MaxLength is the maximum allowed URI length in bytes. Defaults to 4096
	// when zero.
	MaxLength int

	// AllowDotDot permits ".." path segments when true. Default is false
	// (dot-dot segments are blocked).
	AllowDotDot bool
}

// DefaultURIPolicy returns a URIPolicy with safe defaults:
//   - Allowed schemes: http, https, file
//   - Blocked hosts: 169.254.169.254, localhost, 127.0.0.1, [::1], 0.0.0.0
//   - MaxLength: 4096
//   - AllowDotDot: false
func DefaultURIPolicy() URIPolicy {
	return URIPolicy{
		AllowedSchemes: []string{"http", "https", "file"},
		BlockedHosts: []string{
			"169.254.169.254",
			"localhost",
			"127.0.0.1",
			"[::1]",
			"::1",
			"0.0.0.0",
		},
		MaxLength:   4096,
		AllowDotDot: false,
	}
}

// ValidateURI validates and returns the cleaned URI according to policy.
// It checks:
//  1. Length <= MaxLength (default 4096)
//  2. Parseable as a URL
//  3. Scheme in AllowedSchemes (if non-empty)
//  4. Host not in BlockedHosts
//  5. No ".." in path (unless AllowDotDot is true)
//  6. No null bytes
func ValidateURI(raw string, policy URIPolicy) (string, error) {
	maxLen := policy.MaxLength
	if maxLen == 0 {
		maxLen = 4096
	}

	if len(raw) > maxLen {
		return "", fmt.Errorf("URI length %d exceeds maximum %d", len(raw), maxLen)
	}

	if strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("URI contains null byte")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("URI is not parseable: %w", err)
	}

	if len(policy.AllowedSchemes) > 0 {
		scheme := strings.ToLower(parsed.Scheme)
		allowed := false
		for _, s := range policy.AllowedSchemes {
			if strings.ToLower(s) == scheme {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("URI scheme %q is not allowed (allowed: %s)",
				parsed.Scheme, strings.Join(policy.AllowedSchemes, ", "))
		}
	}

	if len(policy.BlockedHosts) > 0 {
		host := strings.ToLower(parsed.Hostname())
		for _, blocked := range policy.BlockedHosts {
			if strings.ToLower(blocked) == host {
				return "", fmt.Errorf("URI host %q is blocked", parsed.Hostname())
			}
		}
	}

	if !policy.AllowDotDot {
		// Check the raw path and the unescaped path for ".." segments.
		pathsToCheck := []string{parsed.Path, parsed.RawPath}
		for _, p := range pathsToCheck {
			if p == "" {
				continue
			}
			for _, segment := range strings.Split(p, "/") {
				if segment == ".." {
					return "", fmt.Errorf("URI path contains disallowed \"..\" segment")
				}
			}
		}
	}

	return raw, nil
}
