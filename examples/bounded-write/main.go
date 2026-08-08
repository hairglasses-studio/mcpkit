// Command bounded-write demonstrates the boundedwrite middleware using a
// simulated payment tool that requires explicit confirmation before charging.
//
// Any tool tagged with boundedwrite.ConfirmTag will be intercepted. Callers
// must pass confirm=true to proceed; omitting or setting confirm=false returns
// a structured rejection message describing what the tool will do and how to
// confirm.
//
// Tool schema construction (PaymentModule.Tools) is SDK-specific and lives in
// tools.go (!official_sdk) / tools_official.go (official_sdk) — see those
// files for why: mcp-go's mcp.NewTool functional options and the official
// SDK's raw JSON-schema map are not interchangeable at the InputSchema level.
//
// Usage:
//
//	go run ./examples/bounded-write
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hairglasses-studio/mcpkit/middleware/boundedwrite"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- Module ---

// PaymentModule provides simulated payment tools.
// payment_charge and payment_refund declare confirm_required; payment_balance does not.
type PaymentModule struct{}

func (m *PaymentModule) Name() string        { return "payment" }
func (m *PaymentModule) Description() string { return "Simulated payment tools with confirmation gate" }

// --- Handlers ---

func chargeHandler(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
	args := registry.ExtractArguments(req)
	amount, _ := args["amount"].(float64)
	currency, _ := args["currency"].(string)
	description, _ := args["description"].(string)
	return registry.MakeTextResult(
		fmt.Sprintf("Charged %.2f %s for %q — txn_id: txn_demo_001", amount, currency, description),
	), nil
}

func refundHandler(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
	args := registry.ExtractArguments(req)
	chargeID, _ := args["charge_id"].(string)
	return registry.MakeTextResult(
		fmt.Sprintf("Refunded charge %q — refund_id: ref_demo_001", chargeID),
	), nil
}

func balanceHandler(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
	args := registry.ExtractArguments(req)
	accountID, _ := args["account_id"].(string)
	return registry.MakeTextResult(
		fmt.Sprintf("Balance for %q: $1,234.56 USD", accountID),
	), nil
}

func main() {
	reg := registry.NewToolRegistry(registry.Config{
		// BoundedWrite middleware intercepts any tool tagged confirm_required.
		// Place it early in the chain so confirmation is checked before other
		// middleware (rate-limiting, auth, etc.) runs.
		Middleware: []registry.Middleware{
			boundedwrite.Middleware(),
		},
	})

	reg.RegisterModule(&PaymentModule{})

	s := registry.NewMCPServer("bounded-write-example", "1.0.0")
	reg.RegisterWithServer(s)

	if err := registry.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
