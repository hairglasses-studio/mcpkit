//go:build !official_sdk

// Command rdloop runs autonomous R&D cycles using the Ralph Loop pattern.
// Each cycle scans the MCP ecosystem, plans roadmap work, implements changes,
// verifies builds, reflects on progress, reports results, and schedules the
// next cycle. Cycles chain via rdcycle/specs/next_cycle.json.
//
// Optional env vars:
//
//	RDLOOP_PROVIDER  — backend family (default: anthropic; also supports ollama)
//	RDLOOP_BASE_URL  — Anthropic-compatible API base URL (default: https://api.anthropic.com or OLLAMA_BASE_URL)
//	RDLOOP_API_KEY   — explicit API key override
//	RDLOOP_DURATION  — max runtime (default: 24h)
//	RDLOOP_BUDGET    — total $ budget across all cycles (default: 200.0)
//	RDLOOP_MODEL     — default model (default: claude-sonnet-4-6 or OLLAMA_CODE_MODEL)
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
	"strings"
	"syscall"
	"time"

	"github.com/hairglasses-studio/mcpkit/finops"
	"github.com/hairglasses-studio/mcpkit/rdcycle"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[rdloop] ")

	provider := strings.ToLower(envOr("RDLOOP_PROVIDER", "anthropic"))
	baseURL := resolveRDLoopBaseURL(provider)
	apiKey := resolveRDLoopAPIKey(provider, baseURL)
	if apiKey == "" && !isLikelyOllamaTarget(provider, baseURL) {
		fmt.Fprintln(os.Stderr, "RDLOOP_API_KEY or ANTHROPIC_API_KEY is required for hosted Anthropic backends")
		os.Exit(1)
	}

	model := envOr("RDLOOP_MODEL", defaultRDLoopModel(provider, baseURL))
	duration := envDuration("RDLOOP_DURATION", 24*time.Hour)
	budget := envFloat("RDLOOP_BUDGET", 200.0)
	specFile := envOr("RDLOOP_SPEC", "rdcycle/specs/rd_cycle.json")
	roadmapPath := envOr("RDLOOP_ROADMAP", "ROADMAP.md")
	statePath := envOr("RDLOOP_STATE", ".rdloop_state.json")
	githubToken := os.Getenv("GITHUB_TOKEN")

	sampler := &sampling.APISamplingClient{
		APIKey:       apiKey,
		DefaultModel: model,
		BaseURL:      baseURL,
	}

	modelTier := defaultModelTier(provider, baseURL, model)

	runner := NewMultiCycleRunner(RunnerConfig{
		InitialSpec: specFile,
		TemplateVars: map[string]string{
			"cycle_name":   "auto-1",
			"since_date":   time.Now().AddDate(0, 0, -7).Format("2006-01-02"),
			"roadmap_path": roadmapPath,
		},
		GlobalBudget: budget,
		Duration:     duration,
		Sampler:      sampler,
		ModelTier:    modelTier,
		Profile:      marathonProfile(provider, baseURL),
		RoadmapPath:  roadmapPath,
		StatePath:    statePath,
		GitHubToken:  githubToken,
	})

	LogPreflight(statePath, budget, baseURL, model)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("starting (provider=%s, base_url=%s, model=%s, budget=$%.0f, duration=%s, spec=%s)",
		provider, baseURL, model, budget, duration, specFile)

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
// operation. Hosted Anthropic keeps the Opus/Sonnet/Haiku blend, while local
// compatible backends default to zero-dollar accounting and local model lanes.
//
// Per-cycle dollar budget: $8 (generous for Opus-heavy 7-task R&D cycles).
// Token budget: 4M per cycle (Opus generates longer output).
// Max iterations: 120 per cycle (scan through schedule with retries + longer phases).
// Daily cap: $200 (matches the global budget — single-session use).
// MaxTokensPerReq: 16384 (Opus generates longer, more detailed output).
func marathonProfile(provider, baseURL string) rdcycle.BudgetProfile {
	profile := rdcycle.BudgetProfile{
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
	if isLikelyOllamaTarget(provider, baseURL) {
		profile.ModelPricing = nil
	}
	return profile
}

func defaultModelTier(provider, baseURL, model string) rdcycle.ModelTierConfig {
	if isLikelyOllamaTarget(provider, baseURL) {
		codeModel := model
		if candidate := strings.TrimSpace(os.Getenv("OLLAMA_CODE_MODEL")); candidate != "" {
			codeModel = candidate
		}

		fastModel := codeModel
		if candidate := strings.TrimSpace(os.Getenv("OLLAMA_FAST_MODEL")); candidate != "" {
			fastModel = candidate
		} else if candidate := strings.TrimSpace(os.Getenv("OLLAMA_CHAT_MODEL")); candidate != "" {
			fastModel = candidate
		}

		highContextModel := codeModel
		if candidate := strings.TrimSpace(os.Getenv("OLLAMA_HIGH_CONTEXT_CODE_MODEL")); candidate != "" {
			highContextModel = candidate
		}

		return rdcycle.ModelTierConfig{
			Default: codeModel,
			TaskOverrides: map[string]string{
				"scan":      highContextModel,
				"plan":      codeModel,
				"implement": codeModel,
				"verify":    fastModel,
				"reflect":   fastModel,
				"report":    fastModel,
				"schedule":  fastModel,
			},
		}
	}

	return rdcycle.ModelTierConfig{
		Default: model,
		TaskOverrides: map[string]string{
			"scan":      model,
			"plan":      "claude-opus-4-6",
			"implement": "claude-opus-4-6",
			"verify":    "claude-haiku-4-5",
			"reflect":   "claude-haiku-4-5",
			"report":    "claude-haiku-4-5",
			"schedule":  "claude-haiku-4-5",
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

func resolveRDLoopBaseURL(provider string) string {
	if baseURL := strings.TrimSpace(os.Getenv("RDLOOP_BASE_URL")); baseURL != "" {
		return baseURL
	}
	if strings.EqualFold(provider, "ollama") {
		if baseURL := strings.TrimSpace(os.Getenv("OLLAMA_BASE_URL")); baseURL != "" {
			return baseURL
		}
		return "http://127.0.0.1:11434"
	}
	return "https://api.anthropic.com"
}

func resolveRDLoopAPIKey(provider, baseURL string) string {
	if apiKey := strings.TrimSpace(os.Getenv("RDLOOP_API_KEY")); apiKey != "" {
		return apiKey
	}
	if isLikelyOllamaTarget(provider, baseURL) {
		if apiKey := strings.TrimSpace(os.Getenv("OLLAMA_API_KEY")); apiKey != "" {
			return apiKey
		}
		return "ollama"
	}
	if apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); apiKey != "" {
		return apiKey
	}
	return ""
}

func defaultRDLoopModel(provider, baseURL string) string {
	if isLikelyOllamaTarget(provider, baseURL) {
		if model := strings.TrimSpace(os.Getenv("OLLAMA_CODE_MODEL")); model != "" {
			return model
		}
		return "code-primary"
	}
	return "claude-sonnet-4-6"
}

func isLikelyOllamaTarget(provider, baseURL string) bool {
	if strings.EqualFold(strings.TrimSpace(provider), "ollama") {
		return true
	}
	return strings.Contains(baseURL, "11434") || strings.Contains(strings.ToLower(baseURL), "ollama")
}
