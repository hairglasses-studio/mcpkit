// Command bounded-write demonstrates the boundedwrite middleware using a
// simulated payment tool that requires explicit confirmation before charging.
//
// Any tool tagged with boundedwrite.ConfirmTag will be intercepted. Callers
// must pass confirm=true to proceed; omitting or setting confirm=false returns
// a structured rejection message describing what the tool will do and how to
// confirm.
//
// Usage:
//
//	go run ./examples/bounded-write
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/middleware/boundedwrite"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- Module ---

// PaymentModule provides simulated payment tools.
// payment_charge and payment_refund declare confirm_required; payment_balance does not.
type PaymentModule struct{}

func (m *PaymentModule) Name() string        { return "payment" }
func (m *PaymentModule) Description() string { return "Simulated payment tools with confirmation gate" }

func (m *PaymentModule) Tools() []registry.ToolDefinition {
	// charge — financial write, requires confirmation
	chargeDef := registry.ToolDefinition{
		Tool: mcp.NewTool(
			"payment_charge",
			mcp.WithDescription("Charge a customer's payment method. This will immediately debit their account."),
			mcp.WithNumber("amount", mcp.Required(), mcp.Description("Amount to charge in USD")),
			mcp.WithString("currency", mcp.Required(), mcp.Description("Currency code (e.g. USD)")),
			mcp.WithString("description", mcp.Required(), mcp.Description("Charge description")),
			mcp.WithBoolean("confirm", mcp.Description("Set to true to confirm the charge")),
		),
		Handler:    chargeHandler,
		IsWrite:    true,
		Category:   "payment",
		Complexity: registry.ComplexityComplex,
	}
	// RequireConfirmation appends the ConfirmTag so the middleware intercepts it.
	chargeDef = boundedwrite.RequireConfirmation(chargeDef)

	// refund — financial write, requires confirmation
	refundDef := registry.ToolDefinition{
		Tool: mcp.NewTool(
			"payment_refund",
			mcp.WithDescription("Refund a previously charged payment. This returns funds to the customer's account."),
			mcp.WithString("charge_id", mcp.Required(), mcp.Description("ID of the charge to refund")),
			mcp.WithNumber("amount", mcp.Description("Partial refund amount (omit for full refund)")),
			mcp.WithBoolean("confirm", mcp.Description("Set to true to confirm the refund")),
		),
		Handler:    refundHandler,
		IsWrite:    true,
		Category:   "payment",
		Complexity: registry.ComplexityModerate,
	}
	refundDef = boundedwrite.RequireConfirmation(refundDef)

	// balance — read-only, no confirmation needed
	balanceDef := registry.ToolDefinition{
		Tool: mcp.NewTool(
			"payment_balance",
			mcp.WithDescription("Look up the current balance for an account. Read-only."),
			mcp.WithString("account_id", mcp.Required(), mcp.Description("Account ID to look up")),
		),
		Handler:  balanceHandler,
		IsWrite:  false,
		Category: "payment",
	}

	return []registry.ToolDefinition{chargeDef, refundDef, balanceDef}
}

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
