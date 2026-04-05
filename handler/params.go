package handler

import "github.com/hairglasses-studio/mcpkit/registry"

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
