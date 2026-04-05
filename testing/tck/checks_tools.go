package tck

import (
	"context"
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// toolChecks returns all tool-category conformance checks.
func toolChecks() []Check {
	return []Check{
		{Category: "tools", Name: "ToolsListNotEmpty", Fn: checkToolsListNotEmpty},
		{Category: "tools", Name: "ToolsHaveDescriptions", Fn: checkToolsHaveDescriptions},
		{Category: "tools", Name: "ToolsHaveInputSchema", Fn: checkToolsHaveInputSchema},
		{Category: "tools", Name: "ToolHandlerContract", Fn: checkToolHandlerContract},
		{Category: "tools", Name: "ToolErrorCodes", Fn: checkToolErrorCodes},
		{Category: "tools", Name: "ToolNamesNonEmpty", Fn: checkToolNamesNonEmpty},
		{Category: "tools", Name: "ToolNamesNoSpaces", Fn: checkToolNamesNoSpaces},
		{Category: "tools", Name: "ToolHandlersNotNil", Fn: checkToolHandlersNotNil},
	}
}

// checkToolsListNotEmpty verifies the server has at least one tool registered.
func checkToolsListNotEmpty(reg *registry.ToolRegistry) CheckResult {
	names := reg.ListTools()
	if len(names) == 0 {
		return CheckResult{Passed: false, Message: "server has no tools registered"}
	}
	return CheckResult{Passed: true, Message: fmt.Sprintf("%d tools registered", len(names))}
}

// checkToolsHaveDescriptions verifies all tools have non-empty descriptions.
func checkToolsHaveDescriptions(reg *registry.ToolRegistry) CheckResult {
	names := reg.ListTools()
	var missing []string
	for _, name := range names {
		td, ok := reg.GetTool(name)
		if !ok {
			continue
		}
		if strings.TrimSpace(td.Tool.Description) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("tools missing descriptions: %s", strings.Join(missing, ", ")),
		}
	}
	return CheckResult{Passed: true, Message: "all tools have descriptions"}
}

// checkToolsHaveInputSchema verifies all tools have a valid JSON Schema
// with type "object" in their InputSchema.
func checkToolsHaveInputSchema(reg *registry.ToolRegistry) CheckResult {
	names := reg.ListTools()
	var invalid []string
	for _, name := range names {
		td, ok := reg.GetTool(name)
		if !ok {
			continue
		}
		schema := td.Tool.InputSchema
		if schema.Type != "object" {
			invalid = append(invalid, fmt.Sprintf("%s (type=%q)", name, schema.Type))
		}
	}
	if len(invalid) > 0 {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("tools with invalid InputSchema type: %s", strings.Join(invalid, ", ")),
		}
	}
	return CheckResult{Passed: true, Message: "all tools have valid InputSchema"}
}

// checkToolHandlerContract verifies the mcpkit handler contract:
// handlers must return (*CallToolResult, nil) and never (nil, error).
// This calls every tool with empty args and checks the return shape.
func checkToolHandlerContract(reg *registry.ToolRegistry) CheckResult {
	names := reg.ListTools()
	var violations []string
	for _, name := range names {
		td, ok := reg.GetTool(name)
		if !ok {
			continue
		}
		if td.Handler == nil {
			continue // checked separately
		}
		result, err := td.Handler(context.Background(), registry.CallToolRequest{})
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s returned non-nil error: %v", name, err))
		}
		if result == nil && err == nil {
			violations = append(violations, fmt.Sprintf("%s returned (nil, nil)", name))
		}
	}
	if len(violations) > 0 {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("handler contract violations: %s", strings.Join(violations, "; ")),
		}
	}
	return CheckResult{Passed: true, Message: "all handlers return (result, nil)"}
}

// checkToolErrorCodes verifies that tools returning errors use the
// CodedErrorResult pattern with "[CODE] message" format.
// This calls every tool with empty args and inspects error results.
func checkToolErrorCodes(reg *registry.ToolRegistry) CheckResult {
	names := reg.ListTools()
	var uncoded []string
	for _, name := range names {
		td, ok := reg.GetTool(name)
		if !ok {
			continue
		}
		if td.Handler == nil {
			continue
		}
		result, err := td.Handler(context.Background(), registry.CallToolRequest{})
		if err != nil || result == nil {
			continue
		}
		if !registry.IsResultError(result) {
			continue
		}
		// Error results should use coded format: "[CODE] message"
		if len(result.Content) > 0 {
			text, ok := registry.ExtractTextContent(result.Content[0])
			if ok && !strings.HasPrefix(text, "[") {
				uncoded = append(uncoded, name)
			}
		}
	}
	if len(uncoded) > 0 {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("tools returning errors without coded format: %s", strings.Join(uncoded, ", ")),
		}
	}
	return CheckResult{Passed: true, Message: "all error results use coded format"}
}

// checkToolNamesNonEmpty verifies that no tool has an empty name.
func checkToolNamesNonEmpty(reg *registry.ToolRegistry) CheckResult {
	names := reg.ListTools()
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return CheckResult{Passed: false, Message: "found tool with empty name"}
		}
	}
	return CheckResult{Passed: true, Message: "all tool names are non-empty"}
}

// checkToolNamesNoSpaces verifies that tool names contain no whitespace.
// MCP tool names should be identifiers (snake_case by convention).
func checkToolNamesNoSpaces(reg *registry.ToolRegistry) CheckResult {
	names := reg.ListTools()
	var bad []string
	for _, name := range names {
		if strings.ContainsAny(name, " \t\n\r") {
			bad = append(bad, fmt.Sprintf("%q", name))
		}
	}
	if len(bad) > 0 {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("tool names with whitespace: %s", strings.Join(bad, ", ")),
		}
	}
	return CheckResult{Passed: true, Message: "all tool names are whitespace-free"}
}

// checkToolHandlersNotNil verifies that every registered tool has a non-nil handler.
func checkToolHandlersNotNil(reg *registry.ToolRegistry) CheckResult {
	names := reg.ListTools()
	var nilHandlers []string
	for _, name := range names {
		td, ok := reg.GetTool(name)
		if !ok {
			continue
		}
		if td.Handler == nil {
			nilHandlers = append(nilHandlers, name)
		}
	}
	if len(nilHandlers) > 0 {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("tools with nil handlers: %s", strings.Join(nilHandlers, ", ")),
		}
	}
	return CheckResult{Passed: true, Message: "all tools have non-nil handlers"}
}
