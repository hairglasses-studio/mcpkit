//go:build !official_sdk

package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// OutputValidationMiddleware returns a registry.Middleware that validates
// CallToolResult.StructuredContent against the tool's OutputSchema at runtime.
// Tools without an OutputSchema are passed through unchanged. Error results
// and nil StructuredContent are also skipped.
func OutputValidationMiddleware() registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		// Short-circuit at wrap time if no output schema
		if td.OutputSchema == nil {
			return next
		}

		schema := td.OutputSchema

		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			result, err := next(ctx, request)
			if err != nil {
				return result, err
			}

			// Skip error results
			if registry.IsResultError(result) {
				return result, nil
			}

			// Skip if no structured content
			if result == nil || result.StructuredContent == nil {
				return result, nil
			}

			// Validate structured content against schema
			if validationErr := validateStructuredContent(result.StructuredContent, schema); validationErr != nil {
				return CodedErrorResult(ErrValidation, validationErr), nil
			}

			return result, nil
		}
	}
}

// validateStructuredContent validates data against a ToolOutputSchema.
// It checks required fields and property type declarations.
func validateStructuredContent(data any, schema *registry.ToolOutputSchema) error {
	// Marshal to map for field-level validation
	var dataMap map[string]any
	switch v := data.(type) {
	case map[string]any:
		dataMap = v
	default:
		// Marshal and unmarshal to get a map
		bytes, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal structured content: %w", err)
		}
		if err := json.Unmarshal(bytes, &dataMap); err != nil {
			return fmt.Errorf("structured content is not an object: %w", err)
		}
	}

	// Check required fields
	for _, field := range schema.Required {
		if _, ok := dataMap[field]; !ok {
			return fmt.Errorf("missing required field %q", field)
		}
	}

	// Check property types
	if schema.Properties != nil {
		for propName, propSchema := range schema.Properties {
			val, ok := dataMap[propName]
			if !ok {
				continue // Not present, already checked required above
			}

			propMap, ok := propSchema.(map[string]any)
			if !ok {
				continue // Schema property isn't a map, skip
			}

			expectedType, ok := propMap["type"].(string)
			if !ok {
				continue // No type declaration, skip
			}

			if err := checkType(propName, val, expectedType); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkType validates that a value matches the expected JSON Schema type.
func checkType(name string, val any, expectedType string) error {
	if val == nil {
		return nil // null is valid for any type in permissive validation
	}

	var valid bool
	switch expectedType {
	case "string":
		_, valid = val.(string)
	case "number":
		valid = isNumber(val)
	case "integer":
		valid = isInteger(val)
	case "boolean":
		_, valid = val.(bool)
	case "array":
		_, valid = val.([]any)
	case "object":
		_, valid = val.(map[string]any)
	default:
		return nil // Unknown type, skip validation
	}

	if !valid {
		return fmt.Errorf("field %q: expected type %q, got %s", name, expectedType, describeType(val))
	}
	return nil
}

// isNumber returns true if val is a JSON number type.
func isNumber(val any) bool {
	switch val.(type) {
	case float64, float32, int, int32, int64, json.Number:
		return true
	}
	return false
}

// isInteger returns true if val is an integer (or a float64 with no fractional part).
func isInteger(val any) bool {
	switch v := val.(type) {
	case int, int32, int64:
		return true
	case float64:
		return v == float64(int64(v))
	case json.Number:
		_, err := v.Int64()
		return err == nil
	}
	return false
}

// describeType returns a human-readable type description.
func describeType(val any) string {
	switch val.(type) {
	case string:
		return "string"
	case float64, float32:
		return "number"
	case int, int32, int64:
		return "integer"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", val)
	}
}
