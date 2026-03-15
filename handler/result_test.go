//go:build !official_sdk

package handler

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ==================== TextResult ====================

func TestResult_Text_Content(t *testing.T) {
	r := TextResult("expected text content")
	got := extractText(t, r)
	if got != "expected text content" {
		t.Errorf("TextResult text = %q, want %q", got, "expected text content")
	}
}

func TestResult_Text_NotError(t *testing.T) {
	r := TextResult("hello")
	if r.IsError {
		t.Error("TextResult should not set IsError")
	}
}

func TestResult_Text_HasContent(t *testing.T) {
	r := TextResult("some text")
	if len(r.Content) == 0 {
		t.Error("TextResult should have at least one content item")
	}
}

// ==================== ErrorResult ====================

func TestResult_Error_SetsIsError(t *testing.T) {
	r := ErrorResult(errors.New("boom"))
	if !r.IsError {
		t.Error("ErrorResult should set IsError = true")
	}
}

func TestResult_Error_MessageInContent(t *testing.T) {
	r := ErrorResult(errors.New("disk full"))
	got := extractText(t, r)
	if got != "disk full" {
		t.Errorf("ErrorResult text = %q, want %q", got, "disk full")
	}
}

func TestResult_Error_WrappedError(t *testing.T) {
	inner := errors.New("inner cause")
	outer := fmt.Errorf("outer: %w", inner)
	r := ErrorResult(outer)
	got := extractText(t, r)
	if !strings.Contains(got, "inner cause") {
		t.Errorf("ErrorResult wrapped error text = %q, should contain inner cause", got)
	}
	if !r.IsError {
		t.Error("ErrorResult wrapped error should set IsError")
	}
}

// ==================== JSONResult ====================

func TestResult_JSON_Struct(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	r := JSONResult(payload{Name: "test", Value: 42})
	got := extractText(t, r)
	if !strings.Contains(got, `"name": "test"`) {
		t.Errorf("JSONResult missing name field, got: %s", got)
	}
	if !strings.Contains(got, `"value": 42`) {
		t.Errorf("JSONResult missing value field, got: %s", got)
	}
	if r.IsError {
		t.Error("JSONResult with valid struct should not be an error")
	}
}

func TestResult_JSON_Slice(t *testing.T) {
	r := JSONResult([]string{"alpha", "beta"})
	got := extractText(t, r)
	if !strings.Contains(got, "alpha") {
		t.Errorf("JSONResult slice missing element, got: %s", got)
	}
}

func TestResult_JSON_Unmarshalable(t *testing.T) {
	// channels cannot be marshaled to JSON
	r := JSONResult(make(chan struct{}))
	if !r.IsError {
		t.Error("JSONResult with unmarshalable input should return an error result")
	}
}

func TestResult_JSON_Unmarshalable_IsErrorNotPanic(t *testing.T) {
	// Ensure no panic, just an error result
	defer func() {
		if rec := recover(); rec != nil {
			t.Errorf("JSONResult panicked on unmarshalable input: %v", rec)
		}
	}()
	r := JSONResult(make(chan int))
	if r == nil {
		t.Error("JSONResult should return non-nil result even for unmarshalable input")
	}
}

// ==================== CodedErrorResult ====================

func TestResult_CodedError_Format(t *testing.T) {
	r := CodedErrorResult(ErrInvalidParam, errors.New("field is required"))
	got := extractText(t, r)
	want := "[INVALID_PARAM] field is required"
	if got != want {
		t.Errorf("CodedErrorResult text = %q, want %q", got, want)
	}
}

func TestResult_CodedError_SetsIsError(t *testing.T) {
	r := CodedErrorResult(ErrNotFound, errors.New("not found"))
	if !r.IsError {
		t.Error("CodedErrorResult should set IsError = true")
	}
}

func TestResult_CodedError_AllBuiltinCodes(t *testing.T) {
	cases := []struct {
		code string
		msg  string
	}{
		{ErrClientInit, "CLIENT_INIT_FAILED"},
		{ErrInvalidParam, "INVALID_PARAM"},
		{ErrTimeout, "TIMEOUT"},
		{ErrNotFound, "NOT_FOUND"},
		{ErrAPIError, "API_ERROR"},
		{ErrPermission, "PERMISSION_DENIED"},
		{ErrValidation, "OUTPUT_VALIDATION_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			r := CodedErrorResult(tc.code, errors.New("err"))
			got := extractText(t, r)
			expected := fmt.Sprintf("[%s] err", tc.code)
			if got != expected {
				t.Errorf("CodedErrorResult(%s) = %q, want %q", tc.code, got, expected)
			}
		})
	}
}

func TestResult_CodedError_CustomCode(t *testing.T) {
	r := CodedErrorResult("MY_CUSTOM_CODE", errors.New("custom"))
	got := extractText(t, r)
	if !strings.HasPrefix(got, "[MY_CUSTOM_CODE]") {
		t.Errorf("CodedErrorResult custom code prefix missing, got: %s", got)
	}
}

// ==================== ActionableErrorResult ====================

func TestResult_ActionableError_ZeroSuggestions(t *testing.T) {
	r := ActionableErrorResult(ErrTimeout, errors.New("request timed out"))
	got := extractText(t, r)
	if !strings.HasPrefix(got, "[TIMEOUT]") {
		t.Errorf("ActionableErrorResult prefix missing, got: %s", got)
	}
	if strings.Contains(got, "Suggested actions") {
		t.Error("ActionableErrorResult with no suggestions should not include 'Suggested actions'")
	}
	if !r.IsError {
		t.Error("ActionableErrorResult should set IsError = true")
	}
}

func TestResult_ActionableError_OneSuggestion(t *testing.T) {
	r := ActionableErrorResult(
		ErrPermission,
		errors.New("access denied"),
		"Request elevated permissions from your admin",
	)
	got := extractText(t, r)
	if !strings.Contains(got, "Suggested actions:") {
		t.Errorf("ActionableErrorResult with suggestion should have header, got: %s", got)
	}
	if !strings.Contains(got, "Request elevated permissions from your admin") {
		t.Errorf("ActionableErrorResult missing suggestion text, got: %s", got)
	}
}

func TestResult_ActionableError_MultipleSuggestions(t *testing.T) {
	r := ActionableErrorResult(
		ErrAPIError,
		errors.New("service unavailable"),
		"Retry after 30 seconds",
		"Check the service status page",
		"Contact support if the issue persists",
	)
	got := extractText(t, r)
	if !strings.Contains(got, "Retry after 30 seconds") {
		t.Errorf("ActionableErrorResult missing first suggestion, got: %s", got)
	}
	if !strings.Contains(got, "Check the service status page") {
		t.Errorf("ActionableErrorResult missing second suggestion, got: %s", got)
	}
	if !strings.Contains(got, "Contact support if the issue persists") {
		t.Errorf("ActionableErrorResult missing third suggestion, got: %s", got)
	}
}

func TestResult_ActionableError_SuggestionsUseBullets(t *testing.T) {
	r := ActionableErrorResult(
		ErrClientInit,
		errors.New("init failed"),
		"Check config",
	)
	got := extractText(t, r)
	if !strings.Contains(got, "•") {
		t.Errorf("ActionableErrorResult suggestions should use bullet (•), got: %s", got)
	}
}
