package sanitize

import "regexp"

// Pattern describes a single redaction rule: a compiled regex and the
// replacement string to substitute for any match.
type Pattern struct {
	Name        string
	Regex       *regexp.Regexp
	Replacement string
}

// Finding records a single match found during output sanitization.
type Finding struct {
	// Pattern is the name of the pattern that matched (e.g. "aws_access_key").
	Pattern string
	// Position is the byte offset of the match within the original text.
	Position int
}

// OutputPolicy controls which categories of sensitive content are redacted from
// tool output text.
type OutputPolicy struct {
	// RedactSecrets removes AWS keys, API keys, tokens, passwords, and bearer values.
	RedactSecrets bool

	// RedactPII removes email addresses, phone numbers, and Social Security Numbers.
	RedactPII bool

	// StripInjection removes common prompt-injection phrases and control tokens.
	StripInjection bool

	// CustomPatterns are additional redaction rules applied after the built-in ones.
	CustomPatterns []Pattern

	// AllowList contains tool names that are exempt from output sanitization.
	// Tools in this list pass through the middleware without modification.
	AllowList []string
}

// Compiled regex patterns for detecting secrets, PII, and prompt injection in output text.
var (
	// patternAWSKey matches AWS access key IDs (AKIA followed by 16 alphanumeric chars).
	patternAWSKey = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)

	// patternGenericSecret matches common secret assignment patterns like
	// api_key=..., secret=..., token=..., password=..., bearer=...
	patternGenericSecret = regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password|bearer)\s*[:=]\s*\S+`)

	// patternEmail matches simplified RFC 5322 email addresses.
	patternEmail = regexp.MustCompile(`\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`)

	// patternPhone matches US-style phone numbers: 555-867-5309, 555.867.5309, 5558675309.
	patternPhone = regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`)

	// patternSSN matches US Social Security Numbers in the form DDD-DD-DDDD.
	patternSSN = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)

	// patternPromptInjection matches common prompt injection phrases and control tokens.
	patternPromptInjection = regexp.MustCompile(`(?i)(ignore previous|system:\s|<\|im_start\|>|<\|endoftext\|>)`)
)

// builtinSecretPatterns is the ordered set of secret-detection patterns used by SanitizeText
// when OutputPolicy.RedactSecrets is true.
var builtinSecretPatterns = []Pattern{
	{
		Name:        "aws_access_key",
		Regex:       patternAWSKey,
		Replacement: "[REDACTED:AWS_KEY]",
	},
	{
		Name:        "generic_secret",
		Regex:       patternGenericSecret,
		Replacement: "[REDACTED:SECRET]",
	},
}

// builtinPIIPatterns is the ordered set of PII-detection patterns used by SanitizeText
// when OutputPolicy.RedactPII is true.
var builtinPIIPatterns = []Pattern{
	{
		Name:        "ssn",
		Regex:       patternSSN,
		Replacement: "[REDACTED:SSN]",
	},
	{
		Name:        "email",
		Regex:       patternEmail,
		Replacement: "[REDACTED:EMAIL]",
	},
	{
		Name:        "phone",
		Regex:       patternPhone,
		Replacement: "[REDACTED:PHONE]",
	},
}

// builtinInjectionPatterns is the ordered set of prompt-injection patterns used by SanitizeText
// when OutputPolicy.StripInjection is true.
var builtinInjectionPatterns = []Pattern{
	{
		Name:        "prompt_injection",
		Regex:       patternPromptInjection,
		Replacement: "[REMOVED:INJECTION]",
	},
}
