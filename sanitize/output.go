//go:build !official_sdk

package sanitize

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// SanitizeText applies the redaction rules defined by policy to text, replacing
// any matching substrings with the pattern's Replacement value.
//
// It returns the sanitized string and a slice of Findings (one per match, in
// left-to-right order). The findings reference positions in the original text.
func SanitizeText(text string, policy OutputPolicy) (string, []Finding) {
	var findings []Finding

	// Collect all active patterns in application order.
	var active []Pattern
	if policy.RedactSecrets {
		active = append(active, builtinSecretPatterns...)
	}
	if policy.RedactPII {
		active = append(active, builtinPIIPatterns...)
	}
	if policy.StripInjection {
		active = append(active, builtinInjectionPatterns...)
	}
	active = append(active, policy.CustomPatterns...)

	out := text
	for _, p := range active {
		if p.Regex == nil {
			continue
		}
		// Find all match positions in the current (already partially-sanitized) text.
		// We record positions against the pre-replacement version for each pattern pass.
		locs := p.Regex.FindAllStringIndex(out, -1)
		for _, loc := range locs {
			findings = append(findings, Finding{
				Pattern:  p.Name,
				Position: loc[0],
			})
		}
		out = p.Regex.ReplaceAllString(out, p.Replacement)
	}

	return out, findings
}

// OutputMiddleware returns a registry.Middleware that sanitizes text content in
// tool results according to policy.
//
// For each tool invocation the middleware:
//  1. Checks whether the tool name is in the AllowList; if so it is a no-op.
//  2. Calls the next handler to obtain the result.
//  3. Iterates over every Content element in the result, applying SanitizeText
//     to any text content.
//  4. Returns the sanitized result.
//
// Non-text content (e.g. image or resource content) is passed through unchanged.
func OutputMiddleware(policy OutputPolicy) registry.Middleware {
	// Build an allowList lookup map once at construction time for O(1) access.
	allow := make(map[string]bool, len(policy.AllowList))
	for _, name := range policy.AllowList {
		allow[name] = true
	}

	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			result, err := next(ctx, req)
			if err != nil {
				return result, err
			}

			// Skip sanitization for allow-listed tools.
			if allow[name] {
				return result, nil
			}

			if result == nil {
				return result, nil
			}

			for i, c := range result.Content {
				text, ok := registry.ExtractTextContent(c)
				if !ok {
					continue
				}
				sanitized, _ := SanitizeText(text, policy)
				result.Content[i] = registry.MakeTextContent(sanitized)
			}

			return result, nil
		}
	}
}
