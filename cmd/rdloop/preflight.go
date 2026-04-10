//go:build !official_sdk

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// LogPreflight runs pre-flight checks and logs results.
// Checks: disk space, API reachability, previous state.
func LogPreflight(statePath string, budget float64, baseURL, model string) {
	log.Println("=== Pre-flight checks ===")

	// 1. Disk space: require 5GB free.
	var stat syscall.Statfs_t
	if err := syscall.Statfs(".", &stat); err == nil {
		freeGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1 << 30)
		if freeGB < 5.0 {
			log.Printf("  WARNING: only %.1f GB free disk space (recommend 5+ GB)", freeGB)
		} else {
			log.Printf("  disk: %.1f GB free", freeGB)
		}
	} else {
		log.Printf("  disk: unable to check (%v)", err)
	}

	// 2. Backend reachability: probe the configured compatible backend.
	label, probeURL, method := preflightProbe(baseURL)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(method, probeURL, nil)
	if err != nil {
		log.Printf("  api: unable to build %s probe for %s (%v)", method, probeURL, err)
	} else {
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("  WARNING: %s unreachable (%s %s): %v", label, method, probeURL, err)
		} else {
			if isLikelyOllamaTarget("", baseURL) {
				logOllamaModelPreflight(resp, strings.TrimSpace(model))
			} else {
				resp.Body.Close()
			}
			log.Printf("  api: %s reachable via %s %s (status %d)", label, method, probeURL, resp.StatusCode)
		}
	}

	// 3. Previous state: report if resuming.
	state, err := LoadState(statePath)
	if err != nil {
		log.Printf("  state: no previous state (fresh start)")
	} else if state.CycleNumber > 0 {
		remaining := budget - state.TotalCost
		log.Printf("  state: resuming from cycle %d (%d iters, $%.2f spent, $%.2f remaining)",
			state.CycleNumber, state.TotalIterations, state.TotalCost, remaining)
		if remaining < 10.0 {
			log.Printf("  WARNING: low remaining budget ($%.2f)", remaining)
		}
	} else {
		log.Printf("  state: fresh start")
	}

	// 4. Budget projection.
	if isLikelyOllamaTarget("", baseURL) {
		log.Printf("  budget: $%.0f cap configured; local-compatible backends default to zero-dollar accounting", budget)
	} else {
		avgCost := 5.70 // estimated avg per cycle with Opus plan+implement
		estCycles := int(budget / avgCost)
		log.Printf("  budget: $%.0f → ~%d estimated cycles at ~$%.2f/cycle avg", budget, estCycles, avgCost)
	}

	fmt.Println()
}

type ollamaTagsResponse struct {
	Models []struct {
		Name  string `json:"name"`
		Model string `json:"model"`
	} `json:"models"`
}

func logOllamaModelPreflight(resp *http.Response, model string) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || model == "" {
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		log.Printf("  WARNING: unable to read Ollama model inventory: %v", err)
		return
	}

	var tags ollamaTagsResponse
	if err := json.Unmarshal(body, &tags); err != nil {
		log.Printf("  WARNING: unable to decode Ollama model inventory: %v", err)
		return
	}

	switch {
	case ollamaModelInstalledExact(tags, model):
		log.Printf("  model: %s is installed on the local Ollama backend", model)
	case ollamaAliasSourceModel(model) != "" && ollamaModelInstalledExact(tags, ollamaAliasSourceModel(model)):
		log.Printf("  WARNING: configured model %s is missing; backing model %s is present but the managed alias is absent (run ~/hairglasses-studio/dotfiles/scripts/hg-ollama-sync-aliases.sh)", model, ollamaAliasSourceModel(model))
	default:
		log.Printf("  WARNING: configured model %s is not installed on the local Ollama backend (pull %s)", model, ollamaPullHintCommand(model))
	}
}

func ollamaModelInstalledExact(tags ollamaTagsResponse, model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	candidates := []string{model}
	if strings.HasSuffix(model, ":latest") {
		candidates = append(candidates, strings.TrimSuffix(model, ":latest"))
	} else if !strings.Contains(model, ":") {
		candidates = append(candidates, model+":latest")
	}
	for _, current := range tags.Models {
		for _, candidate := range candidates {
			if current.Name == candidate || current.Model == candidate {
				return true
			}
		}
	}
	return false
}

func ollamaAliasSourceModel(model string) string {
	switch strings.TrimSpace(model) {
	case "code-fast", "code-compact":
		return "qwen2.5-coder:7b"
	case "code-primary", "code-reasoner":
		return "devstral-small-2"
	case "code-long":
		return "qwen3-coder-next"
	case "code-heavy":
		return "devstral-2"
	default:
		return ""
	}
}

func ollamaPullHintCommand(model string) string {
	if sourceModel := ollamaAliasSourceModel(model); sourceModel != "" {
		return "ollama pull " + sourceModel
	}
	return "ollama pull " + strings.TrimSpace(model)
}

func preflightProbe(baseURL string) (label, probeURL, method string) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		trimmed = "https://api.anthropic.com"
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if isLikelyOllamaTarget("", trimmed) {
		root := strings.TrimSuffix(trimmed, "/messages")
		root = strings.TrimSuffix(root, "/v1")
		return "ollama-compatible backend", root + "/api/tags", http.MethodGet
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "compatible backend", trimmed, http.MethodHead
	}
	return parsed.Host, parsed.Scheme + "://" + parsed.Host, http.MethodHead
}
