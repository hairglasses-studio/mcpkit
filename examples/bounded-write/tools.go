//go:build !official_sdk

package main

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/middleware/boundedwrite"
	"github.com/hairglasses-studio/mcpkit/registry"
)

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
