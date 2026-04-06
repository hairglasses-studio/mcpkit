package handler

import (
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// GetStringParam extracts a string parameter from the request.
func GetStringParam(req registry.CallToolRequest, name string) string {
	args := registry.ExtractArguments(req)
	if args == nil {
		return ""
	}
	val, ok := args[name]
	if !ok {
		return ""
	}
	str, ok := val.(string)
	if !ok {
		return ""
	}
	return str
}

// GetIntParam extracts an integer parameter from the request.
func GetIntParam(req registry.CallToolRequest, name string, defaultVal int) int {
	args := registry.ExtractArguments(req)
	if args == nil {
		return defaultVal
	}
	val, ok := args[name]
	if !ok {
		return defaultVal
	}
	num, ok := val.(float64)
	if !ok {
		return defaultVal
	}
	return int(num)
}

// GetFloatParam extracts a float64 parameter from the request.
func GetFloatParam(req registry.CallToolRequest, name string, defaultVal float64) float64 {
	args := registry.ExtractArguments(req)
	if args == nil {
		return defaultVal
	}
	val, ok := args[name]
	if !ok {
		return defaultVal
	}
	num, ok := val.(float64)
	if !ok {
		return defaultVal
	}
	return num
}

// GetBoolParam extracts a boolean parameter from the request.
func GetBoolParam(req registry.CallToolRequest, name string, defaultVal bool) bool {
	args := registry.ExtractArguments(req)
	if args == nil {
		return defaultVal
	}
	val, ok := args[name]
	if !ok {
		return defaultVal
	}
	b, ok := val.(bool)
	if !ok {
		return defaultVal
	}
	return b
}

// HasParam checks if a parameter was explicitly provided in the request.
func HasParam(req registry.CallToolRequest, name string) bool {
	args := registry.ExtractArguments(req)
	if args == nil {
		return false
	}
	_, ok := args[name]
	return ok
}

// GetStringArrayParam extracts a string array parameter from the request.
func GetStringArrayParam(req registry.CallToolRequest, name string) []string {
	args := registry.ExtractArguments(req)
	if args == nil {
		return nil
	}
	val, ok := args[name]
	if !ok {
		return nil
	}
	arr, ok := val.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// RequireStringParam extracts a string parameter and returns an error result
// if the parameter is missing or empty. This is a convenience for handlers
// that need to validate required string parameters in a single call.
//
// Usage:
//
//	name, errResult := handler.RequireStringParam(req, "name")
//	if errResult != nil {
//	    return errResult, nil
//	}
func RequireStringParam(req registry.CallToolRequest, name string) (string, *registry.CallToolResult) {
	val := GetStringParam(req, name)
	if val == "" {
		return "", CodedErrorResult(ErrInvalidParam, fmt.Errorf("required parameter %q is missing or empty", name))
	}
	return val, nil
}

// RequireIntParam extracts an integer parameter and returns an error result
// if the parameter is missing. Unlike GetIntParam, this distinguishes between
// "not provided" and "provided as zero".
func RequireIntParam(req registry.CallToolRequest, name string) (int, *registry.CallToolResult) {
	args := registry.ExtractArguments(req)
	if args == nil {
		return 0, CodedErrorResult(ErrInvalidParam, fmt.Errorf("required parameter %q is missing", name))
	}
	val, ok := args[name]
	if !ok {
		return 0, CodedErrorResult(ErrInvalidParam, fmt.Errorf("required parameter %q is missing", name))
	}
	num, ok := val.(float64)
	if !ok {
		return 0, CodedErrorResult(ErrInvalidParam, fmt.Errorf("parameter %q must be a number", name))
	}
	return int(num), nil
}
