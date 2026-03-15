package auth

import (
	"context"
	"fmt"
	"net/http"
)

// OAuthTransport is an http.RoundTripper that injects an OAuth Bearer token
// into outgoing requests. It follows the standard Go pattern used by
// golang.org/x/oauth2.Transport.
type OAuthTransport struct {
	// Source provides access tokens.
	Client *OAuthClient

	// IssuerURL is the OAuth issuer for token resolution.
	IssuerURL string

	// Base is the underlying transport. If nil, http.DefaultTransport is used.
	Base http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *OAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.Client.Token(req.Context(), t.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oauth transport: %w", err)
	}

	// Clone the request to avoid mutating the original
	r2 := req.Clone(context.Background())
	r2.Header.Set("Authorization", "Bearer "+token)

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r2)
}
