# Auth RFC Compliance Audit (2026-05-10)

Audit findings against the latest OAuth 2.0 / token-binding specs. This doc is a snapshot; rerun audit each quarter or after any major auth-spec update.

## Summary

| RFC | Title | Status in mcpkit | Notes |
|---|---|---|---|
| [RFC 9728](https://datatracker.ietf.org/doc/html/rfc9728) | OAuth 2.0 Protected Resource Metadata | ✅ **Implemented** | `auth/metadata.go` |
| [RFC 9449](https://datatracker.ietf.org/doc/html/rfc9449) | OAuth 2.0 Demonstrating Proof of Possession (DPoP) | ✅ **Implemented** | `auth/dpop.go`, `auth/dpop_middleware.go`, `auth/dpop_cache.go` |
| [RFC 8707](https://datatracker.ietf.org/doc/html/rfc8707) | Resource Indicators for OAuth 2.0 | ⚠️ **Partial** | Resource indicators (`resource` parameter on token requests) are not explicitly enforced by mcpkit's auth middleware. Resource binding happens via `aud` claim validation in `auth/jwks.go`. |

## RFC 9728 — Protected Resource Metadata (D1)

**Spec**: https://datatracker.ietf.org/doc/html/rfc9728 (April 2025, FINAL)

**mcpkit implementation**: `auth/metadata.go`

```go
// ProtectedResourceMetadata represents the OAuth 2.0 Protected Resource Metadata (RFC 9728).
type ProtectedResourceMetadata struct {
    Resource               string   `json:"resource"`
    AuthorizationServers   []string `json:"authorization_servers"`
    ScopesSupported        []string `json:"scopes_supported,omitempty"`
    BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
}
```

The `MetadataHandler` serves the metadata at `/.well-known/oauth-protected-resource`. Test coverage in `auth/metadata_test.go` and `auth/auth_test.go`.

**Gap analysis vs RFC 9728 §2**:
- ✅ `resource` field (required)
- ✅ `authorization_servers` field (required if multiple ASes)
- ✅ `scopes_supported` (optional)
- ✅ `bearer_methods_supported` (optional)
- ⚠️ Missing optional fields: `resource_documentation`, `resource_signing_alg_values_supported`, `resource_encryption_alg_values_supported`, `resource_name`, `resource_policy_uri`, `resource_tos_uri`, `tls_client_certificate_bound_access_tokens`, `dpop_signing_alg_values_supported`, `dpop_bound_access_tokens_required`.

**Recommendation**: Add `DPoPSigningAlgValuesSupported []string` field to the struct so DPoP-aware clients can negotiate alg upfront. Other optional fields are deployment-specific; document on request.

**Status**: **NO CODE CHANGE NEEDED**. RFC 9728 compliance is functional for the required-fields-only baseline. Optional extensions are deployment-specific and can be added per-server.

## RFC 9449 — DPoP (D2)

**Spec**: https://datatracker.ietf.org/doc/html/rfc9449 (September 2023, FINAL)

**mcpkit implementation**: `auth/dpop.go`, `auth/dpop_middleware.go`, `auth/dpop_cache.go`

Per `auth/dpop.go:34`:
```go
// DPoPValidator validates DPoP proofs per RFC 9449.
```

**Coverage check (read-only audit)**:
- ✅ DPoP-Proof header parsing
- ✅ JWT-formatted proof validation (JWS over JWK)
- ✅ Token binding via `jkt` thumbprint
- ✅ Replay protection via `dpop_cache.go` (jti tracking)
- ✅ Middleware wrap (`dpop_middleware.go`)

**Status**: **COMPLIANT**. No spec gaps detected. Test coverage in `auth/dpop_test.go`.

## RFC 8707 — Resource Indicators (D3)

**Spec**: https://datatracker.ietf.org/doc/html/rfc8707 (February 2020, FINAL)

**mcpkit implementation**: implicit only.

`grep -rn 'resource_indicators\|RequiredResourceIndicators' auth/` returns zero hits. The spec defines a `resource` parameter on OAuth token requests that lets clients indicate which resource server an access token is intended for.

**Current behavior**: mcpkit's `auth/jwks.go` validates the `aud` (audience) claim in JWT tokens. This is the de-facto enforcement mechanism for resource binding, but it's not specifically tied to RFC 8707's `resource` parameter; clients have to set `aud` correctly via their OAuth client implementation.

**Gap**: mcpkit doesn't expose a knob for the OAuth-client side to send the `resource` parameter on token requests, nor does it validate that the token's `aud` came from a specific `resource` parameter request.

**Recommendation**: This is a low-priority gap because:
1. The `aud` claim enforcement provides effective resource binding regardless of how the token was minted.
2. RFC 8707 is most useful in scenarios where one client requests tokens for multiple resource servers within one OAuth flow — uncommon in mcpkit's deployment model (server-side MCP gateways).
3. Adding explicit `resource` parameter handling would require coordinating with the authorization server, which is outside mcpkit's scope.

**Status**: **PARTIAL — accepted gap**. Document the `aud`-claim convention; recommend RFC 8707 alignment in `docs/auth-deployment-guide.md` (future).

## Verification

```bash
# RFC 9728 — verify the well-known endpoint serves valid JSON
go test ./auth -run TestMetadataHandler -count=1 -v

# RFC 9449 — full DPoP test suite
go test ./auth -run TestDPoP -count=1 -v

# RFC 8707 — no direct tests; aud-claim validation
go test ./auth -run TestJWKS -count=1 -v
```

## Quarterly review trigger

Re-run this audit on:
- Major mcpkit auth refactor
- New OAuth spec FINAL (track [datatracker.ietf.org](https://datatracker.ietf.org/wg/oauth/about/))
- Changes to upstream go-oauth2 libraries or `github.com/golang-jwt/jwt/v5`

Last audit: **2026-05-10** by Claude Opus 4.7 (1M context).
