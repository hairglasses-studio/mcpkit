package secrets

import (
	"regexp"
	"strings"
)

// SensitivePatterns contains patterns for identifying sensitive keys.
var SensitivePatterns = []string{
	"password", "passwd", "secret", "token", "key", "credential",
	"cred", "auth", "bearer", "api_key", "apikey", "api-key",
	"private", "oauth", "jwt", "session", "cookie", "cert",
	"certificate", "pem", "ssh", "rsa", "dsa", "ecdsa",
	"access", "refresh", "signing", "encryption", "decrypt",
	"salt", "hash", "hmac", "pin", "otp", "2fa", "mfa", "totp", "seed",
}

// sensitiveRegex caches compiled regex for performance.
var sensitiveRegex *regexp.Regexp

func init() {
	patterns := make([]string, len(SensitivePatterns))
	for i, p := range SensitivePatterns {
		patterns[i] = regexp.QuoteMeta(p)
	}
	sensitiveRegex = regexp.MustCompile(`(?i)(` + strings.Join(patterns, "|") + `)`)
}

// IsSensitiveKey returns true if the key name suggests sensitive content.
func IsSensitiveKey(key string) bool {
	return sensitiveRegex.MatchString(strings.ToLower(key))
}

// MaskValue masks a value for safe logging.
func MaskValue(value string) string {
	if len(value) == 0 {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	if len(value) <= 8 {
		return value[:1] + "****" + value[len(value)-1:]
	}
	return value[:2] + "****" + value[len(value)-2:]
}

// Sanitize removes or masks sensitive values from a map.
func Sanitize(params map[string]any) map[string]any {
	result := make(map[string]any, len(params))
	for key, value := range params {
		result[key] = sanitizeValue(key, value)
	}
	return result
}

func sanitizeValue(key string, value any) any {
	if IsSensitiveKey(key) {
		switch v := value.(type) {
		case string:
			return MaskValue(v)
		case []byte:
			return MaskValue(string(v))
		default:
			return "[REDACTED]"
		}
	}
	if m, ok := value.(map[string]any); ok {
		return Sanitize(m)
	}
	if slice, ok := value.([]any); ok {
		result := make([]any, len(slice))
		for i, item := range slice {
			if m, ok := item.(map[string]any); ok {
				result[i] = Sanitize(m)
			} else {
				result[i] = item
			}
		}
		return result
	}
	return value
}

// SanitizeString masks sensitive patterns in a string (like URLs with passwords).
func SanitizeString(s string) string {
	urlPattern := regexp.MustCompile(`(://[^:]+:)([^@]+)(@)`)
	s = urlPattern.ReplaceAllString(s, "${1}****${3}")

	bearerPattern := regexp.MustCompile(`(?i)(bearer\s+)(\S+)`)
	s = bearerPattern.ReplaceAllString(s, "${1}****")

	apiKeyPattern := regexp.MustCompile(`(?i)(api[_-]?key=)([^&\s]+)`)
	s = apiKeyPattern.ReplaceAllString(s, "${1}****")

	tokenPattern := regexp.MustCompile(`(?i)(token=)([^&\s]+)`)
	s = tokenPattern.ReplaceAllString(s, "${1}****")

	return s
}

// SanitizeHeaders sanitizes HTTP headers, masking Authorization and other sensitive headers.
func SanitizeHeaders(headers map[string]string) map[string]string {
	sensitiveHeaders := []string{
		"authorization", "x-api-key", "x-auth-token",
		"cookie", "set-cookie", "x-csrf-token", "x-xsrf-token",
	}

	result := make(map[string]string, len(headers))
	for key, value := range headers {
		keyLower := strings.ToLower(key)
		isSensitive := false
		for _, sh := range sensitiveHeaders {
			if keyLower == sh {
				isSensitive = true
				break
			}
		}
		if isSensitive {
			result[key] = MaskValue(value)
		} else {
			result[key] = value
		}
	}
	return result
}

// RedactedString is a string that masks itself when printed.
type RedactedString string

// String returns the masked value.
func (r RedactedString) String() string { return MaskValue(string(r)) }

// Value returns the actual value.
func (r RedactedString) Value() string { return string(r) }

// MarshalJSON returns the masked value for JSON encoding.
func (r RedactedString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + MaskValue(string(r)) + `"`), nil
}

// SecureCompare performs a constant-time comparison of two strings.
func SecureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
