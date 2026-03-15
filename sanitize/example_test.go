//go:build !official_sdk

package sanitize_test

import (
	"fmt"

	"github.com/hairglasses-studio/mcpkit/sanitize"
)

func ExampleSanitizeText() {
	policy := sanitize.OutputPolicy{
		RedactSecrets: true,
	}

	text := "The token is: api_key=supersecret123"
	sanitized, findings := sanitize.SanitizeText(text, policy)
	fmt.Println(sanitized)
	fmt.Println(len(findings) > 0)
	// Output:
	// The token is: [REDACTED:SECRET]
	// true
}

func ExampleValidateURI() {
	policy := sanitize.DefaultURIPolicy()

	safe, err := sanitize.ValidateURI("https://example.com/path", policy)
	fmt.Println(safe)
	fmt.Println(err)

	_, err = sanitize.ValidateURI("https://localhost/admin", policy)
	fmt.Println(err)
	// Output:
	// https://example.com/path
	// <nil>
	// URI host "localhost" is blocked
}
