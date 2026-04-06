// Package providers implements concrete secret provider backends for the
// mcpkit secrets framework.
//
// Included providers: EnvProvider reads secrets from environment variables
// with optional prefix filtering; FileProvider reads secrets from files on
// disk (one secret per file, suitable for Docker/Kubernetes secret mounts).
// Both providers implement the secrets.Provider interface and support
// priority-based resolution when multiple providers are registered.
package providers
