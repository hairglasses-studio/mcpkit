//go:build !official_sdk

package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
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
			resp.Body.Close()
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
	avgCost := 5.70 // estimated avg per cycle with Opus plan+implement
	estCycles := int(budget / avgCost)
	log.Printf("  budget: $%.0f → ~%d estimated cycles at ~$%.2f/cycle avg", budget, estCycles, avgCost)

	fmt.Println()
}

func preflightProbe(baseURL string) (label, probeURL, method string) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "compatible backend", baseURL, http.MethodHead
	}
	return parsed.Host, parsed.Scheme + "://" + parsed.Host, http.MethodHead
}
