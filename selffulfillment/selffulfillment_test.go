package selffulfillment

import (
	"context"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/gateway"
	"github.com/hairglasses-studio/mcpkit/mcptest"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestSelfFulfillmentModule(t *testing.T) {
	t.Parallel()

	m := &Module{}
	if m.Name() != "selffulfillment" {
		t.Fatalf("expected name selffulfillment, got %s", m.Name())
	}
	if m.Description() == "" {
		t.Fatal("expected non-empty description")
	}

	tools := m.Tools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	reg := registry.NewToolRegistry()
	reg.RegisterModule(m)

	srv := mcptest.NewServer(t, reg)

	expectedNames := []string{
		"selffulfillment_run_cycle",
		"selffulfillment_status",
		"selffulfillment_guardrails_check",
	}
	for _, name := range expectedNames {
		if !srv.HasTool(name) {
			t.Errorf("server missing tool: %s", name)
		}
	}

	ctx := context.Background()
	out, err := m.handleGuardrailsCheck(ctx, GuardrailsInput{})
	if err != nil {
		t.Fatalf("guardrails check error: %v", err)
	}
	if out.Status != "PASS" {
		t.Errorf("expected PASS status, got %s", out.Status)
	}
}

func TestGatewaySelfFulfillmentIntegration(t *testing.T) {
	t.Parallel()

	m := &Module{}
	reg := registry.NewToolRegistry()
	reg.RegisterModule(m)

	httpServer := mcptest.NewHTTPServer(t, reg)
	t.Cleanup(httpServer.Close)

	gw, gwReg := gateway.NewGateway()
	defer gw.Close()

	cfg := gateway.UpstreamConfig{
		Name:           "fulfillment",
		URL:            httpServer.Endpoint(),
		HealthInterval: 24 * time.Hour,
	}

	count, err := gw.AddUpstream(context.Background(), cfg)
	if err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 tools from upstream, got %d", count)
	}

	tools := gwReg.ListTools()
	expectedTools := map[string]bool{
		"fulfillment.selffulfillment_run_cycle":        true,
		"fulfillment.selffulfillment_status":           true,
		"fulfillment.selffulfillment_guardrails_check": true,
	}
	for _, name := range tools {
		if !expectedTools[name] {
			t.Errorf("unexpected namespaced tool: %s", name)
		}
	}
}
