// Package handler provides helper functions for MCP tool handlers.
package handler

import "github.com/mark3labs/mcp-go/mcp"

// GetStringParam extracts a string parameter from the request.
func GetStringParam(req mcp.CallToolRequest, name string) string {
	if req.Params.Arguments == nil {
		return ""
	}
	args, ok := req.Params.Arguments.(map[string]interface{})
	if !ok {
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
func GetIntParam(req mcp.CallToolRequest, name string, defaultVal int) int {
	if req.Params.Arguments == nil {
		return defaultVal
	}
	args, ok := req.Params.Arguments.(map[string]interface{})
	if !ok {
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
func GetFloatParam(req mcp.CallToolRequest, name string, defaultVal float64) float64 {
	if req.Params.Arguments == nil {
		return defaultVal
	}
	args, ok := req.Params.Arguments.(map[string]interface{})
	if !ok {
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
func GetBoolParam(req mcp.CallToolRequest, name string, defaultVal bool) bool {
	if req.Params.Arguments == nil {
		return defaultVal
	}
	args, ok := req.Params.Arguments.(map[string]interface{})
	if !ok {
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
func HasParam(req mcp.CallToolRequest, name string) bool {
	if req.Params.Arguments == nil {
		return false
	}
	args, ok := req.Params.Arguments.(map[string]interface{})
	if !ok {
		return false
	}
	_, ok = args[name]
	return ok
}

// GetStringArrayParam extracts a string array parameter from the request.
func GetStringArrayParam(req mcp.CallToolRequest, name string) []string {
	if req.Params.Arguments == nil {
		return nil
	}
	args, ok := req.Params.Arguments.(map[string]interface{})
	if !ok {
		return nil
	}
	val, ok := args[name]
	if !ok {
		return nil
	}
	arr, ok := val.([]interface{})
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
