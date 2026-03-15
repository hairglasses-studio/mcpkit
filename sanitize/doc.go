// Package sanitize provides input validation, output redaction, and URI
// security for MCP tool parameters.
//
// Input validation functions ([Username], [SafePath], [MountPoint], etc.)
// enforce allowlist patterns against user-supplied strings that flow into
// shell commands or external APIs. Output sanitization ([SanitizeText],
// [OutputMiddleware]) redacts secrets, PII, and prompt-injection patterns
// from tool responses using configurable [OutputPolicy] rules. URI validation
// ([ValidateURI], [DefaultURIPolicy]) blocks path traversal and SSRF vectors
// including access to the EC2 metadata endpoint and localhost.
//
// Example:
//
//	if err := sanitize.SafePath("/data", userPath); err != nil {
//	    return handler.CodedErrorResult(handler.ErrInvalidParam, err), nil
//	}
//	reg := registry.New(registry.Config{
//	    Middleware: []registry.Middleware{
//	        sanitize.OutputMiddleware(sanitize.OutputPolicy{RedactSecrets: true, RedactPII: true}),
//	    },
//	})
package sanitize
