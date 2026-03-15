// Package auth provides authentication and authorization utilities for MCP servers.
//
// It supports JWT validation with RS256 (via JWKS auto-fetching), OAuth 2.0
// authorization server discovery and client flows (including PKCE), DPoP
// proof validation with replay-attack prevention, a Bearer-token HTTP
// middleware ([Middleware]), and workload identity for GCP and AWS
// environments. Verified identities are stored in the request context and
// retrieved with [SubjectFromContext] and [ClaimsFromContext].
//
// Key types: [JWKSValidator], [OAuthClient], [DPoPMiddleware], [Config].
//
// Example — protect an HTTP handler with Bearer JWT validation:
//
//	validator := auth.NewJWKSValidator("https://auth.example.com/.well-known/jwks.json")
//	mux.Handle("/mcp", auth.Middleware(validator.Validate)(mcpHandler))
package auth
