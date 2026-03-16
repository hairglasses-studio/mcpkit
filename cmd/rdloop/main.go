//go:build !official_sdk

// Command rdloop runs autonomous R&D cycles using the Ralph Loop pattern.
// Each cycle scans the MCP ecosystem, plans roadmap work, implements changes,
// verifies builds, reflects on progress, reports results, and schedules the
// next cycle. Cycles chain via rdcycle/specs/next_cycle.json.
//
// Required: ANTHROPIC_API_KEY environment variable.
//
// Optional env vars:
//
//	RDLOOP_DURATION  — max runtime (default: 12h)
//	RDLOOP_BUDGET    — total $ budget across all cycles (default: 60.0)
//	RDLOOP_MODEL     — default Claude model (default: claude-sonnet-4-6)
//	RDLOOP_SPEC      — initial spec file (default: rdcycle/specs/rd_cycle.json)
//	RDLOOP_ROADMAP   — roadmap JSON path (default: roadmap.json)
//	RDLOOP_STATE     — state file path (default: .rdloop_state.json)
//	GITHUB_TOKEN     — for higher GitHub API rate limits
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hairglasses-studio/mcpkit/finops"
	"github.com/hairglasses-studio/mcpkit/rdcycle"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[rdloop] ")

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "ANTHROPIC_API_KEY is required")
		os.Exit(1)
	}

	model := envOr("RDLOOP_MODEL", "claude-sonnet-4-6")
	duration := envDuration("RDLOOP_DURATION", 12*time.Hour)
	budget := envFloat("RDLOOP_BUDGET", 60.0)
	specFile := envOr("RDLOOP_SPEC", "rdcycle/specs/rd_cycle.json")
	roadmapPath := envOr("RDLOOP_ROADMAP", "roadmap.json")
	statePath := envOr("RDLOOP_STATE", ".rdloop_state.json")
	githubToken := os.Getenv("GITHUB_TOKEN")

	sampler := NewClaudeClient(apiKey, model)

	modelTier := rdcycle.ModelTierConfig{
		Default: model,
		TaskOverrides: map[string]string{
			"scan":      model,
			"verify":    "claude-haiku-4-5",
			"reflect":   "claude-haiku-4-5",
			"report":    "claude-haiku-4-5",
			"schedule":  "claude-haiku-4-5",
			"plan":      model,
			"implement": model,
		},
	}

	runner := NewMultiCycleRunner(RunnerConfig{
		InitialSpec:  specFile,
		TemplateVars: map[string]string{
			"cycle_name":   "auto-1",
			"since_date":   time.Now().AddDate(0, 0, -7).Format("2006-01-02"),
			"roadmap_path": roadmapPath,
		},
		GlobalBudget: budget,
		Duration:     duration,
		Sampler:      sampler,
		ModelTier:    modelTier,
		Profile:      workProfile(),
		RoadmapPath:  roadmapPath,
		StatePath:    statePath,
		GitHubToken:  githubToken,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("starting (model=%s, budget=$%.0f, duration=%s, spec=%s)",
		model, budget, duration, specFile)

	go func() {
		<-ctx.Done()
		log.Println("signal received, stopping after current cycle...")
		runner.Stop()
	}()

	if err := runner.Run(ctx); err != nil && err != context.Canceled {
		log.Printf("runner exited: %v", err)
	}

	state := runner.State()
	log.Printf("=== Final Summary ===")
	log.Printf("  Cycles completed: %d", len(state.History))
	log.Printf("  Total iterations: %d", state.TotalIterations)
	log.Printf("  Total cost:       $%.4f", state.TotalCost)
	log.Printf("  Runtime:          %s", time.Since(state.StartedAt).Truncate(time.Second))
}

// workProfile returns a budget profile tuned for autonomous code generation
// on a direct API key. Sized to sustain ~20 cycles over 12h on $100.
//
// Per-cycle dollar budget: $5 (generous for 7-task R&D cycles).
// Token budget: 2M per cycle (enough for ~100 iterations at 8K max output).
// Max iterations: 100 per cycle (scan through schedule with retries).
// Daily cap: $100 (matches the global budget — single-session use).
func workProfile() rdcycle.BudgetProfile {
	return rdcycle.BudgetProfile{
		Name:            "work-12h",
		MaxIterations:   100,
		DollarBudget:    5.0,
		DailyDollarCap:  100.0,
		TokenBudget:     2_000_000,
		MaxTokensPerReq: 8192,
		ModelPricing: []finops.ModelPricing{
			{Model: "claude-opus-4-6", InputPer1KTokens: 0.015, OutputPer1KTokens: 0.075},
			{Model: "claude-sonnet-4-6", InputPer1KTokens: 0.003, OutputPer1KTokens: 0.015},
			{Model: "claude-haiku-4-5", InputPer1KTokens: 0.0008, OutputPer1KTokens: 0.004},
		},
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("warning: invalid %s=%q, using default %s", key, v, fallback)
		return fallback
	}
	return d
}

func envFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		log.Printf("warning: invalid %s=%q, using default %.1f", key, v, fallback)
		return fallback
	}
	return f
}
