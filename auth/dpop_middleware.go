package auth

import (
	"net/http"
	"strings"
)

// DPoPOption configures DPoP middleware behavior.
type DPoPOption func(*dpopOptions)

type dpopOptions struct {
	requireDPoP bool
}

// WithRequireDPoP makes DPoP proof mandatory — requests without a DPoP header are rejected.
func WithRequireDPoP() DPoPOption {
	return func(o *dpopOptions) { o.requireDPoP = true }
}

// DPoPMiddleware returns HTTP middleware that validates both Bearer tokens and DPoP proofs.
// If no DPoP header is present and requireDPoP is false, it falls back to Bearer-only validation.
func DPoPMiddleware(validator TokenValidator, dpop *DPoPValidator, opts ...DPoPOption) func(http.Handler) http.Handler {
	var cfg dpopOptions
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				unauthorized(w)
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				unauthorized(w)
				return
			}
			accessToken := parts[1]

			// Validate Bearer token
			subject, err := validator(accessToken)
			if err != nil {
				unauthorized(w)
				return
			}

			// Check for DPoP proof
			dpopProof := r.Header.Get("DPoP")
			if dpopProof == "" {
				if cfg.requireDPoP {
					http.Error(w, "DPoP proof required", http.StatusUnauthorized)
					return
				}
				// Fall back to Bearer-only
				ctx := WithSubject(r.Context(), subject)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Build the HTTP URI (scheme + host + path, no query/fragment)
			scheme := "https"
			if r.TLS == nil {
				scheme = "http"
			}
			httpURI := scheme + "://" + r.Host + r.URL.Path

			// Validate DPoP proof
			if err := dpop.ValidateProof(dpopProof, accessToken, r.Method, httpURI); err != nil {
				http.Error(w, "Invalid DPoP proof: "+err.Error(), http.StatusUnauthorized)
				return
			}

			ctx := WithSubject(r.Context(), subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
