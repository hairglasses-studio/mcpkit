// Package secrets provides a unified interface for retrieving secrets from
// multiple sources with caching and sanitization.
//
// [SecretProvider] is the core interface implemented by [EnvProvider] (reads
// environment variables), [FileProvider] (reads a JSON/YAML file), and
// [WritableProvider] (extends with set/delete). [Manager] aggregates multiple
// providers by priority, caches results with a configurable TTL, and surfaces
// the first successful provider's value. The sanitize helpers
// ([IsSensitiveKey], [MaskValue], [SanitizeHeaders]) identify and redact
// secret-like values before logging or returning them to callers.
//
// Example:
//
//	mgr := secrets.NewManager(
//	    secrets.WithProviders(secrets.NewEnvProvider(), secrets.NewFileProvider("secrets.json")),
//	    secrets.WithCacheTTL(10 * time.Minute),
//	)
//	secret, err := mgr.Get(ctx, "DATABASE_URL")
package secrets
