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
//	RDLOOP_DURATION  — max runtime (default: 24h)
//	RDLOOP_BUDGET    — total $ budget across all cycles (default: 200.0)
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
	duration := envDuration("RDLOOP_DURATION", 24*time.Hour)
	budget := envFloat("RDLOOP_BUDGET", 200.0)
	specFile := envOr("RDLOOP_SPEC", "rdcycle/specs/rd_cycle.json")
	roadmapPath := envOr("RDLOOP_ROADMAP", "roadmap.json")
	statePath := envOr("RDLOOP_STATE", ".rdloop_state.json")
	githubToken := os.Getenv("GITHUB_TOKEN")

	sampler := NewClaudeClient(apiKey, model)

	modelTier := rdcycle.ModelTierConfig{
		Default: model,
		TaskOverrides: map[string]string{
			"scan":      model,                // sonnet — fast ecosystem scan
			"plan":      "claude-opus-4-6",    // opus — detailed planning
			"implement": "claude-opus-4-6",    // opus — complex code generation
			"verify":    "claude-haiku-4-5",   // haiku — build/test checks
			"reflect":   "claude-haiku-4-5",   // haiku — lightweight reflection
			"report":    "claude-haiku-4-5",   // haiku — markdown generation
			"schedule":  "claude-haiku-4-5",   // haiku — next cycle spec
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
		Profile:      marathonProfile(),
		RoadmapPath:  roadmapPath,
		StatePath:    statePath,
		GitHubToken:  githubToken,
	})

	LogPreflight(statePath, budget)

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

// marathonProfile returns a budget profile tuned for 24hr fully-autonomous
// operation with Opus on plan+implement phases.
//
// Per-cycle dollar budget: $8 (generous for Opus-heavy 7-task R&D cycles).
// Token budget: 4M per cycle (Opus generates longer output).
// Max iterations: 120 per cycle (scan through schedule with retries + longer phases).
// Daily cap: $200 (matches the global budget — single-session use).
// MaxTokensPerReq: 16384 (Opus generates longer, more detailed output).
func marathonProfile() rdcycle.BudgetProfile {
	return rdcycle.BudgetProfile{
		Name:            "marathon-24h",
		MaxIterations:   120,
		DollarBudget:    8.0,
		DailyDollarCap:  200.0,
		TokenBudget:     4_000_000,
		MaxTokensPerReq: 16384,
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
