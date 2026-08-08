//go:build official_sdk

package main

import (
	"github.com/hairglasses-studio/mcpkit/middleware/boundedwrite"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func (m *PaymentModule) Tools() []registry.ToolDefinition {
	// charge — financial write, requires confirmation
	chargeDef := registry.ToolDefinition{
		Tool: registry.Tool{
			Name:        "payment_charge",
			Description: "Charge a customer's payment method. This will immediately debit their account.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"amount":      map[string]any{"type": "number", "description": "Amount to charge in USD"},
					"currency":    map[string]any{"type": "string", "description": "Currency code (e.g. USD)"},
					"description": map[string]any{"type": "string", "description": "Charge description"},
					"confirm":     map[string]any{"type": "boolean", "description": "Set to true to confirm the charge"},
				},
				"required": []string{"amount", "currency", "description"},
			},
		},
		Handler:    chargeHandler,
		IsWrite:    true,
		Category:   "payment",
		Complexity: registry.ComplexityComplex,
	}
	// RequireConfirmation appends the ConfirmTag so the middleware intercepts it.
	chargeDef = boundedwrite.RequireConfirmation(chargeDef)

	// refund — financial write, requires confirmation
	refundDef := registry.ToolDefinition{
		Tool: registry.Tool{
			Name:        "payment_refund",
			Description: "Refund a previously charged payment. This returns funds to the customer's account.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"charge_id": map[string]any{"type": "string", "description": "ID of the charge to refund"},
					"amount":    map[string]any{"type": "number", "description": "Partial refund amount (omit for full refund)"},
					"confirm":   map[string]any{"type": "boolean", "description": "Set to true to confirm the refund"},
				},
				"required": []string{"charge_id"},
			},
		},
		Handler:    refundHandler,
		IsWrite:    true,
		Category:   "payment",
		Complexity: registry.ComplexityModerate,
	}
	refundDef = boundedwrite.RequireConfirmation(refundDef)

	// balance — read-only, no confirmation needed
	balanceDef := registry.ToolDefinition{
		Tool: registry.Tool{
			Name:        "payment_balance",
			Description: "Look up the current balance for an account. Read-only.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"account_id": map[string]any{"type": "string", "description": "Account ID to look up"},
				},
				"required": []string{"account_id"},
			},
		},
		Handler:  balanceHandler,
		IsWrite:  false,
		Category: "payment",
	}

	return []registry.ToolDefinition{chargeDef, refundDef, balanceDef}
}
