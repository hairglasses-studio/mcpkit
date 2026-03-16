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

// workProfile returns a budget profile tuned for autonomous code generation.
// Higher token limits than PersonalProfile to accommodate file writing.
func workProfile() rdcycle.BudgetProfile {
	p := rdcycle.PersonalProfile()
	p.MaxTokensPerReq = 8192
	p.DollarBudget = 10.0 // $10 per cycle with work API key
	return p
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
