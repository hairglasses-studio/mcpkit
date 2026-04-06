//go:build !official_sdk

package sampling

import (
	"errors"
	"fmt"
	"strings"
)

// Validation errors returned by ValidateRequest.
var (
	ErrNoMessages         = errors.New("sampling: messages must not be empty")
	ErrInvalidRole        = errors.New("sampling: message role must be 'user' or 'assistant'")
	ErrNilContent         = errors.New("sampling: message content must not be nil")
	ErrMaxTokensNegative  = errors.New("sampling: maxTokens must not be negative")
	ErrTemperatureRange   = errors.New("sampling: temperature must be between 0.0 and 1.0")
	ErrIncludeContextVal  = errors.New("sampling: includeContext must be 'none', 'thisServer', or 'allServers'")
	ErrStopSequenceEmpty  = errors.New("sampling: stop sequences must not contain empty strings")
	ErrFirstMessageRole   = errors.New("sampling: first message must have role 'user'")
	ErrConsecutiveRoles   = errors.New("sampling: consecutive messages must not have the same role")
	ErrMaxTokensZero      = errors.New("sampling: maxTokens must be greater than zero")
	ErrModelHintEmpty     = errors.New("sampling: model preference hint name must not be empty")
	ErrCostPriorityRange  = errors.New("sampling: costPriority must be between 0.0 and 1.0")
	ErrSpeedPriorityRange = errors.New("sampling: speedPriority must be between 0.0 and 1.0")
	ErrIntelPriorityRange = errors.New("sampling: intelligencePriority must be between 0.0 and 1.0")
)

// ValidateRequest checks a CreateMessageRequest for structural validity
// according to the MCP sampling specification. It returns a multi-error
// containing all validation failures, or nil if the request is valid.
//
// Checks performed:
//  1. Messages must not be empty
//  2. Each message must have a valid role ("user" or "assistant")
//  3. Each message must have non-nil content
//  4. First message must have role "user"
//  5. Consecutive messages must alternate roles
//  6. MaxTokens must be positive
//  7. Temperature must be in [0.0, 1.0] (when non-zero)
//  8. IncludeContext must be a valid enum value (when non-empty)
//  9. StopSequences must not contain empty strings
//  10. ModelPreferences priorities must be in [0.0, 1.0]
//  11. ModelPreferences hints must have non-empty names
func ValidateRequest(req CreateMessageRequest) error {
	var errs []error

	// 1. Messages must not be empty.
	if len(req.Messages) == 0 {
		errs = append(errs, ErrNoMessages)
	}

	// 2-5. Validate individual messages.
	var prevRole string
	for i, msg := range req.Messages {
		role := string(msg.Role)

		// 2. Valid role.
		if role != "user" && role != "assistant" {
			errs = append(errs, fmt.Errorf("%w: message[%d] has role %q", ErrInvalidRole, i, role))
		}

		// 3. Non-nil content.
		if msg.Content == nil {
			errs = append(errs, fmt.Errorf("%w: message[%d]", ErrNilContent, i))
		}

		// 4. First message must be "user".
		if i == 0 && role != "user" {
			errs = append(errs, ErrFirstMessageRole)
		}

		// 5. No consecutive same-role messages.
		if i > 0 && role == prevRole {
			errs = append(errs, fmt.Errorf("%w: messages[%d] and [%d] both have role %q", ErrConsecutiveRoles, i-1, i, role))
		}
		prevRole = role
	}

	// 6. MaxTokens must not be negative. Zero is treated as "use default" by
	// the API client and is therefore valid at the validation layer.
	if req.MaxTokens < 0 {
		errs = append(errs, ErrMaxTokensNegative)
	}

	// 7. Temperature must be in [0.0, 1.0] when non-zero.
	if req.Temperature != 0 && (req.Temperature < 0 || req.Temperature > 1.0) {
		errs = append(errs, ErrTemperatureRange)
	}

	// 8. IncludeContext must be a valid enum value.
	if req.IncludeContext != "" {
		switch req.IncludeContext {
		case "none", "thisServer", "allServers":
			// valid
		default:
			errs = append(errs, fmt.Errorf("%w: got %q", ErrIncludeContextVal, req.IncludeContext))
		}
	}

	// 9. StopSequences must not contain empty strings.
	for i, seq := range req.StopSequences {
		if strings.TrimSpace(seq) == "" {
			errs = append(errs, fmt.Errorf("%w: index %d", ErrStopSequenceEmpty, i))
		}
	}

	// 10-11. ModelPreferences validation.
	if mp := req.ModelPreferences; mp != nil {
		if mp.CostPriority < 0 || mp.CostPriority > 1.0 {
			errs = append(errs, ErrCostPriorityRange)
		}
		if mp.SpeedPriority < 0 || mp.SpeedPriority > 1.0 {
			errs = append(errs, ErrSpeedPriorityRange)
		}
		if mp.IntelligencePriority < 0 || mp.IntelligencePriority > 1.0 {
			errs = append(errs, ErrIntelPriorityRange)
		}
		for i, hint := range mp.Hints {
			if hint.Name == "" {
				errs = append(errs, fmt.Errorf("%w: hint[%d]", ErrModelHintEmpty, i))
			}
		}
	}

	// Combine all errors.
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// MustValidateRequest is a convenience wrapper that calls ValidateRequest and
// formats the error as a tool error result suitable for returning from an MCP handler.
// Returns nil if validation passes, otherwise returns a formatted error.
func MustValidateRequest(req CreateMessageRequest) error {
	return ValidateRequest(req)
}
