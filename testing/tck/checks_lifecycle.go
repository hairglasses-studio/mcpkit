package tck

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// lifecycleChecks returns lifecycle-category conformance checks.
func lifecycleChecks() []Check {
	return []Check{
		{Category: "lifecycle", Name: "InitializeResponds", Fn: checkInitializeResponds},
		{Category: "lifecycle", Name: "CapabilitiesPresent", Fn: checkCapabilitiesPresent},
		{Category: "lifecycle", Name: "ModulesRegistered", Fn: checkModulesRegistered},
		{Category: "lifecycle", Name: "RegistryThreadSafe", Fn: checkRegistryThreadSafe},
	}
}

// checkInitializeResponds verifies the MCP server responds to an initialize request.
// It builds a server from the registry and sends an initialize JSON-RPC message.
func checkInitializeResponds(reg *registry.ToolRegistry) CheckResult {
	srv := registry.NewMCPServer("tck-test", "0.0.0-tck")
	reg.RegisterWithServer(srv)

	session, err := createTestSession(srv)
	if err != nil {
		return CheckResult{Passed: false, Message: fmt.Sprintf("failed to create session: %v", err)}
	}

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "tck-client",
				"version": "0.0.1",
			},
		},
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return CheckResult{Passed: false, Message: fmt.Sprintf("failed to marshal initialize request: %v", err)}
	}

	ctx := srv.WithContext(context.Background(), session)
	resp := srv.HandleMessage(ctx, reqBytes)
	if resp == nil {
		return CheckResult{Passed: false, Message: "server returned nil for initialize request"}
	}

	// Verify the response is valid JSON-RPC with a result.
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return CheckResult{Passed: false, Message: fmt.Sprintf("failed to marshal response: %v", err)}
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return CheckResult{Passed: false, Message: fmt.Sprintf("failed to parse response: %v", err)}
	}

	if rpcResp.Error != nil {
		return CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("initialize returned error: %d %s", rpcResp.Error.Code, rpcResp.Error.Message),
		}
	}

	if rpcResp.Result == nil {
		return CheckResult{Passed: false, Message: "initialize response has no result"}
	}

	return CheckResult{Passed: true, Message: "server responds to initialize"}
}

// checkCapabilitiesPresent verifies the server declares capabilities in
// its initialize response, specifically that it declares tools support.
func checkCapabilitiesPresent(reg *registry.ToolRegistry) CheckResult {
	srv := registry.NewMCPServer("tck-test", "0.0.0-tck")
	reg.RegisterWithServer(srv)

	session, err := createTestSession(srv)
	if err != nil {
		return CheckResult{Passed: false, Message: fmt.Sprintf("failed to create session: %v", err)}
	}

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "tck-client",
				"version": "0.0.1",
			},
		},
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return CheckResult{Passed: false, Message: fmt.Sprintf("failed to marshal request: %v", err)}
	}

	ctx := srv.WithContext(context.Background(), session)
	resp := srv.HandleMessage(ctx, reqBytes)
	if resp == nil {
		return CheckResult{Passed: false, Message: "nil response"}
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		return CheckResult{Passed: false, Message: fmt.Sprintf("failed to marshal response: %v", err)}
	}

	var rpcResp struct {
		Result struct {
			Capabilities map[string]any `json:"capabilities"`
			ServerInfo   map[string]any `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return CheckResult{Passed: false, Message: fmt.Sprintf("failed to parse response: %v", err)}
	}

	if rpcResp.Result.Capabilities == nil {
		return CheckResult{Passed: false, Message: "server declares no capabilities"}
	}

	// A server with tools registered should declare tools capability.
	if _, hasTools := rpcResp.Result.Capabilities["tools"]; !hasTools {
		return CheckResult{Passed: false, Message: "server does not declare tools capability"}
	}

	return CheckResult{Passed: true, Message: "server declares capabilities including tools"}
}

// checkModulesRegistered verifies that the registry has at least one module registered.
func checkModulesRegistered(reg *registry.ToolRegistry) CheckResult {
	modules := reg.ListModules()
	if len(modules) == 0 {
		return CheckResult{Passed: false, Message: "no modules registered in registry"}
	}
	return CheckResult{Passed: true, Message: fmt.Sprintf("%d modules registered", len(modules))}
}

// checkRegistryThreadSafe exercises the registry from multiple goroutines
// concurrently to detect data races (when run with -race).
func checkRegistryThreadSafe(reg *registry.ToolRegistry) CheckResult {
	done := make(chan struct{})
	const goroutines = 8
	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_ = reg.ListTools()
			for _, name := range reg.ListTools() {
				_, _ = reg.GetTool(name)
			}
			_ = reg.ListModules()
		}()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
	return CheckResult{Passed: true, Message: "concurrent registry access completed without panic"}
}
