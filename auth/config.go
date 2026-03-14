package auth

// Config holds OAuth 2.1 configuration for the MCP server.
type Config struct {
	// TokenValidator validates Bearer tokens.
	TokenValidator TokenValidator

	// Resource is the protected resource identifier (URL).
	Resource string

	// AuthorizationServers are the OAuth 2.0 authorization server URLs.
	AuthorizationServers []string

	// Scopes are the supported OAuth scopes.
	Scopes []string
}

// NewMetadata creates ProtectedResourceMetadata from the config.
func (c Config) NewMetadata() ProtectedResourceMetadata {
	return ProtectedResourceMetadata{
		Resource:               c.Resource,
		AuthorizationServers:   c.AuthorizationServers,
		ScopesSupported:        c.Scopes,
		BearerMethodsSupported: []string{"header"},
	}
}
