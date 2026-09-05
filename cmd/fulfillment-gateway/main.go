package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hairglasses-studio/mcpkit/gateway/fulfillment"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func main() {
	deployDir := flag.String("deploy-dir", fulfillment.DefaultDeployDir, "Stash deployment directory path")
	runE2E := flag.Bool("e2e", false, "Execute one-shot autonomous end-to-end self-fulfillment loop")
	daemonMode := flag.Bool("daemon", false, "Run in continuous background daemon mode")
	interval := flag.Duration("interval", 60*time.Second, "Cycle interval for daemon mode")
	dryRun := flag.Bool("dry-run", false, "Execute in dry-run simulation mode")
	maxSteps := flag.Int("max-steps", 3, "Maximum pipeline steps to execute in e2e mode")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mod := fulfillment.NewFulfillmentModule(*deployDir)

	if *runE2E {
		engine := fulfillment.NewPipelineEngine(*deployDir)
		mm := fulfillment.NewMatrixManager(*deployDir)
		out := engine.RunE2E(ctx, mm, *dryRun, *maxSteps, "cli-e2e-runner")
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		if !out.Success {
			os.Exit(1)
		}
		return
	}

	if *daemonMode {
		log.Printf("[fulfillment-gateway] Starting daemon mode (interval: %v, deploy-dir: %s)...", *interval, *deployDir)
		engine := fulfillment.NewPipelineEngine(*deployDir)
		mm := fulfillment.NewMatrixManager(*deployDir)

		ticker := time.NewTicker(*interval)
		defer ticker.Stop()

		// Run initial cycle immediately
		out := engine.RunE2E(ctx, mm, *dryRun, *maxSteps, "daemon-runner")
		log.Printf("[fulfillment-gateway] Initial cycle completed: success=%v, steps=%v", out.Success, out.StepsExecuted)

		for {
			select {
			case <-ctx.Done():
				log.Printf("[fulfillment-gateway] Daemon shutting down cleanly...")
				return
			case <-ticker.C:
				res := engine.RunE2E(ctx, mm, *dryRun, *maxSteps, "daemon-runner")
				log.Printf("[fulfillment-gateway] Interval cycle completed: success=%v, steps=%v", res.Success, res.StepsExecuted)
			}
		}
	}

	reg := registry.NewToolRegistry()
	reg.RegisterModule(mod)

	s := registry.NewMCPServer("fulfillment-gateway", "1.0.0")
	reg.RegisterWithServer(s)

	log.Printf("[fulfillment-gateway] Starting Unified Go MCP Gateway for Self-Fulfillment Matrix (deploy-dir: %s)...", *deployDir)
	if err := registry.ServeStdio(s); err != nil {
		log.Fatalf("[fulfillment-gateway] Stdio server failed: %v", err)
	}
}

