//go:build !official_sdk

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RunnerState tracks cross-cycle progress for resumability.
type RunnerState struct {
	CycleNumber     int            `json:"cycle_number"`
	NextSpec        string         `json:"next_spec"`
	TotalCost       float64        `json:"total_cost"`
	TotalIterations int            `json:"total_iterations"`
	StartedAt       time.Time      `json:"started_at"`
	LastCycleAt     time.Time      `json:"last_cycle_at"`
	History         []CycleSummary `json:"history"`
}

// CycleSummary records what happened in a single R&D cycle.
type CycleSummary struct {
	Number   int           `json:"number"`
	SpecFile string        `json:"spec_file"`
	Status   string        `json:"status"`
	Iters    int           `json:"iterations"`
	Cost     float64       `json:"cost_dollars"`
	Duration time.Duration `json:"duration"`
	Tasks    []string      `json:"completed_tasks"`
}

// LoadState reads runner state from a JSON file.
// Returns zero state if the file doesn't exist.
func LoadState(path string) (RunnerState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RunnerState{}, nil
		}
		return RunnerState{}, fmt.Errorf("rdloop: load state: %w", err)
	}
	var s RunnerState
	if err := json.Unmarshal(data, &s); err != nil {
		return RunnerState{}, fmt.Errorf("rdloop: parse state: %w", err)
	}
	return s, nil
}

// SaveState atomically writes runner state to a JSON file.
func SaveState(path string, s RunnerState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("rdloop: marshal state: %w", err)
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}
	tmp, err := os.CreateTemp(dir, ".rdloop-state-*.tmp")
	if err != nil {
		return fmt.Errorf("rdloop: create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("rdloop: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rdloop: close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rdloop: rename state: %w", err)
	}
	return nil
}
