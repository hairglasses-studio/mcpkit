package auth

import (
	"encoding/json"
	"net/http"
)

// ProtectedResourceMetadata represents the OAuth 2.0 Protected Resource Metadata (RFC 9728).
type ProtectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
}

// MetadataHandler returns an http.Handler that serves the protected resource metadata
// at /.well-known/oauth-protected-resource.
func MetadataHandler(meta ProtectedResourceMetadata) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(meta)
	})
}
