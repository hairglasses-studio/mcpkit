package auth

import (
	"net/http"
	"strings"
)

// TokenValidator validates a Bearer token and returns the subject (user ID) or an error.
type TokenValidator func(token string) (subject string, err error)

// Middleware returns HTTP middleware that validates Bearer tokens.
// It returns 401 with WWW-Authenticate header on failure.
func Middleware(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				unauthorized(w)
				return
			}

			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				unauthorized(w)
				return
			}

			subject, err := validator(parts[1])
			if err != nil {
				unauthorized(w)
				return
			}

			ctx := WithSubject(r.Context(), subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}
