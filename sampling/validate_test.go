//go:build !official_sdk

package sampling

import (
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func validRequest() CreateMessageRequest {
	return CompletionRequest(
		[]SamplingMessage{TextMessage("user", "hello")},
		WithMaxTokens(256),
	)
}

func TestValidateRequest_ValidMinimal(t *testing.T) {
	t.Parallel()
	req := validRequest()
	if err := ValidateRequest(req); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateRequest_EmptyMessages(t *testing.T) {
	t.Parallel()
	req := CompletionRequest(nil)
	req.Messages = nil
	req.MaxTokens = 256
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
	if !errors.Is(err, ErrNoMessages) {
		t.Errorf("expected ErrNoMessages, got %v", err)
	}
}

func TestValidateRequest_InvalidRole(t *testing.T) {
	t.Parallel()
	req := CreateMessageRequest{}
	req.Messages = []SamplingMessage{
		{Role: "system", Content: registry.MakeTextContent("bad role")},
	}
	req.MaxTokens = 256
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !errors.Is(err, ErrInvalidRole) {
		t.Errorf("expected ErrInvalidRole, got %v", err)
	}
}

func TestValidateRequest_NilContent(t *testing.T) {
	t.Parallel()
	req := CreateMessageRequest{}
	req.Messages = []SamplingMessage{
		{Role: "user", Content: nil},
	}
	req.MaxTokens = 256
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for nil content")
	}
	if !errors.Is(err, ErrNilContent) {
		t.Errorf("expected ErrNilContent, got %v", err)
	}
}

func TestValidateRequest_FirstMessageMustBeUser(t *testing.T) {
	t.Parallel()
	req := CreateMessageRequest{}
	req.Messages = []SamplingMessage{
		{Role: "assistant", Content: registry.MakeTextContent("not user")},
	}
	req.MaxTokens = 256
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for first message not being user")
	}
	if !errors.Is(err, ErrFirstMessageRole) {
		t.Errorf("expected ErrFirstMessageRole, got %v", err)
	}
}

func TestValidateRequest_ConsecutiveRoles(t *testing.T) {
	t.Parallel()
	req := CreateMessageRequest{}
	req.Messages = []SamplingMessage{
		TextMessage("user", "first"),
		TextMessage("user", "second"),
	}
	req.MaxTokens = 256
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for consecutive same roles")
	}
	if !errors.Is(err, ErrConsecutiveRoles) {
		t.Errorf("expected ErrConsecutiveRoles, got %v", err)
	}
}

func TestValidateRequest_MaxTokensNegative(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.MaxTokens = -1
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for negative maxTokens")
	}
	if !errors.Is(err, ErrMaxTokensNegative) {
		t.Errorf("expected ErrMaxTokensNegative, got %v", err)
	}
}

func TestValidateRequest_MaxTokensZero_Valid(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.MaxTokens = 0 // zero means "use default", valid at validation layer
	if err := ValidateRequest(req); err != nil {
		t.Errorf("expected nil for zero maxTokens (use default), got %v", err)
	}
}

func TestValidateRequest_TemperatureTooHigh(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.Temperature = 1.5
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for temperature > 1.0")
	}
	if !errors.Is(err, ErrTemperatureRange) {
		t.Errorf("expected ErrTemperatureRange, got %v", err)
	}
}

func TestValidateRequest_TemperatureNegative(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.Temperature = -0.1
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for negative temperature")
	}
	if !errors.Is(err, ErrTemperatureRange) {
		t.Errorf("expected ErrTemperatureRange, got %v", err)
	}
}

func TestValidateRequest_TemperatureZero_Valid(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.Temperature = 0 // zero is valid (omitted/default)
	if err := ValidateRequest(req); err != nil {
		t.Errorf("expected nil for zero temperature, got %v", err)
	}
}

func TestValidateRequest_InvalidIncludeContext(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.IncludeContext = "invalid_value"
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for invalid includeContext")
	}
	if !errors.Is(err, ErrIncludeContextVal) {
		t.Errorf("expected ErrIncludeContextVal, got %v", err)
	}
}

func TestValidateRequest_ValidIncludeContext(t *testing.T) {
	t.Parallel()
	for _, val := range []string{"none", "thisServer", "allServers"} {
		req := validRequest()
		req.IncludeContext = val
		if err := ValidateRequest(req); err != nil {
			t.Errorf("expected nil for includeContext=%q, got %v", val, err)
		}
	}
}

func TestValidateRequest_EmptyStopSequence(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.StopSequences = []string{"valid", "", "also_valid"}
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for empty stop sequence")
	}
	if !errors.Is(err, ErrStopSequenceEmpty) {
		t.Errorf("expected ErrStopSequenceEmpty, got %v", err)
	}
}

func TestValidateRequest_ValidStopSequences(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.StopSequences = []string{"END", "STOP", "---"}
	if err := ValidateRequest(req); err != nil {
		t.Errorf("expected nil for valid stop sequences, got %v", err)
	}
}

func TestValidateRequest_ModelPreferences_CostPriorityTooHigh(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.ModelPreferences = &mcp.ModelPreferences{
		CostPriority: 1.5,
	}
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for costPriority > 1.0")
	}
	if !errors.Is(err, ErrCostPriorityRange) {
		t.Errorf("expected ErrCostPriorityRange, got %v", err)
	}
}

func TestValidateRequest_ModelPreferences_SpeedPriorityNegative(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.ModelPreferences = &mcp.ModelPreferences{
		SpeedPriority: -0.1,
	}
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for negative speedPriority")
	}
	if !errors.Is(err, ErrSpeedPriorityRange) {
		t.Errorf("expected ErrSpeedPriorityRange, got %v", err)
	}
}

func TestValidateRequest_ModelPreferences_IntelligencePriorityRange(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.ModelPreferences = &mcp.ModelPreferences{
		IntelligencePriority: 2.0,
	}
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for intelligencePriority > 1.0")
	}
	if !errors.Is(err, ErrIntelPriorityRange) {
		t.Errorf("expected ErrIntelPriorityRange, got %v", err)
	}
}

func TestValidateRequest_ModelPreferences_EmptyHintName(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.ModelPreferences = &mcp.ModelPreferences{
		Hints: []mcp.ModelHint{{Name: ""}, {Name: "claude"}},
	}
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected error for empty hint name")
	}
	if !errors.Is(err, ErrModelHintEmpty) {
		t.Errorf("expected ErrModelHintEmpty, got %v", err)
	}
}

func TestValidateRequest_ModelPreferences_ValidHints(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.ModelPreferences = &mcp.ModelPreferences{
		Hints:                []mcp.ModelHint{{Name: "claude"}, {Name: "sonnet"}},
		CostPriority:         0.5,
		SpeedPriority:        0.3,
		IntelligencePriority: 0.8,
	}
	if err := ValidateRequest(req); err != nil {
		t.Errorf("expected nil for valid model preferences, got %v", err)
	}
}

func TestValidateRequest_MultipleErrors(t *testing.T) {
	t.Parallel()
	// Construct a request with many validation errors.
	req := CreateMessageRequest{}
	req.Messages = nil // empty messages
	req.MaxTokens = -1
	req.Temperature = 2.0
	req.IncludeContext = "bad"
	req.StopSequences = []string{""}

	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected multiple errors")
	}

	// Check that all expected errors are present.
	if !errors.Is(err, ErrNoMessages) {
		t.Error("expected ErrNoMessages in combined error")
	}
	if !errors.Is(err, ErrMaxTokensNegative) {
		t.Error("expected ErrMaxTokensNegative in combined error")
	}
	if !errors.Is(err, ErrTemperatureRange) {
		t.Error("expected ErrTemperatureRange in combined error")
	}
	if !errors.Is(err, ErrIncludeContextVal) {
		t.Error("expected ErrIncludeContextVal in combined error")
	}
	if !errors.Is(err, ErrStopSequenceEmpty) {
		t.Error("expected ErrStopSequenceEmpty in combined error")
	}
}

func TestValidateRequest_AlternatingRoles_Valid(t *testing.T) {
	t.Parallel()
	req := CreateMessageRequest{}
	req.Messages = []SamplingMessage{
		TextMessage("user", "first"),
		TextMessage("assistant", "response"),
		TextMessage("user", "followup"),
	}
	req.MaxTokens = 256
	if err := ValidateRequest(req); err != nil {
		t.Errorf("expected nil for alternating roles, got %v", err)
	}
}

func TestValidateRequest_WithSystemPrompt_Valid(t *testing.T) {
	t.Parallel()
	req := CompletionRequest(
		[]SamplingMessage{TextMessage("user", "hello")},
		WithMaxTokens(256),
		WithSystemPrompt("You are helpful."),
		WithTemperature(0.7),
	)
	if err := ValidateRequest(req); err != nil {
		t.Errorf("expected nil for valid request with system prompt, got %v", err)
	}
}
