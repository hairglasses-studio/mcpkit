package ralph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

// Status represents the current state of a loop execution.
type Status string

const (
	StatusIdle      Status = "idle"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusStopped   Status = "stopped"
)

// Spec defines an autonomous task specification loaded from disk.
type Spec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Completion  string `json:"completion"`
	Tasks       []Task `json:"tasks"`
}

// Task is a single unit of work within a Spec.
type Task struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
}

// Progress tracks the execution state of a loop run.
type Progress struct {
	SpecFile     string         `json:"spec_file"`
	Iteration    int            `json:"iteration"`
	CompletedIDs []string       `json:"completed_ids"`
	Log          []IterationLog `json:"log"`
	Status       Status         `json:"status"`
	StartedAt    time.Time      `json:"started_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// IterationLog records what happened in a single loop iteration.
type IterationLog struct {
	Iteration int       `json:"iteration"`
	TaskID    string    `json:"task_id,omitempty"`
	ToolCalls []string  `json:"tool_calls,omitempty"`
	Result    string    `json:"result"`
	Timestamp time.Time `json:"timestamp"`
}

// Decision is the JSON structure the LLM returns each iteration.
type Decision struct {
	Complete  bool                   `json:"complete"`
	TaskID    string                 `json:"task_id,omitempty"`
	ToolName  string                 `json:"tool_name,omitempty"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Reasoning string                 `json:"reasoning,omitempty"`
	MarkDone  bool                   `json:"mark_done,omitempty"`
}

// Config configures a Loop execution.
type Config struct {
	MaxIterations int
	SpecFile      string
	ProgressFile  string
	ToolRegistry  *registry.ToolRegistry
	Sampler       sampling.SamplingClient
	MaxTokens     int
}

// Loop is the autonomous iteration runner.
type Loop struct {
	config   Config
	mu       sync.Mutex
	progress Progress
	stopCh   chan struct{}
	stopped  bool
}

// DefaultProgressFile returns the default progress file path for a spec file.
func DefaultProgressFile(specFile string) string {
	ext := filepath.Ext(specFile)
	return strings.TrimSuffix(specFile, ext) + ".progress.json"
}

// LoadSpec reads and parses a Spec from a JSON file.
func LoadSpec(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("ralph: load spec: %w", err)
	}
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return Spec{}, fmt.Errorf("ralph: parse spec: %w", err)
	}
	return spec, nil
}

// LoadProgress reads and parses Progress from a JSON file.
// Returns zero Progress if the file does not exist.
func LoadProgress(path string) (Progress, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Progress{}, nil
		}
		return Progress{}, fmt.Errorf("ralph: load progress: %w", err)
	}
	var p Progress
	if err := json.Unmarshal(data, &p); err != nil {
		return Progress{}, fmt.Errorf("ralph: parse progress: %w", err)
	}
	return p, nil
}

// SaveProgress atomically writes Progress to a JSON file (write tmp + rename).
func SaveProgress(path string, p Progress) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("ralph: marshal progress: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ralph-progress-*.tmp")
	if err != nil {
		return fmt.Errorf("ralph: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("ralph: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("ralph: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("ralph: rename progress file: %w", err)
	}
	return nil
}

// NewLoop creates a new Loop with the given config.
func NewLoop(config Config) (*Loop, error) {
	if config.SpecFile == "" {
		return nil, fmt.Errorf("ralph: spec file is required")
	}
	if config.ToolRegistry == nil {
		return nil, fmt.Errorf("ralph: tool registry is required")
	}
	if config.Sampler == nil {
		return nil, fmt.Errorf("ralph: sampler is required")
	}
	if config.MaxIterations <= 0 {
		config.MaxIterations = 100
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = 2048
	}
	if config.ProgressFile == "" {
		config.ProgressFile = DefaultProgressFile(config.SpecFile)
	}
	return &Loop{
		config: config,
		stopCh: make(chan struct{}),
	}, nil
}

// Status returns the current progress snapshot.
func (l *Loop) Status() Progress {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.progress
}

// Stop signals the loop to stop after the current iteration.
func (l *Loop) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.stopped {
		l.stopped = true
		close(l.stopCh)
	}
}

// parseDecision parses a JSON Decision from LLM response text.
// It attempts to find JSON in the response, handling markdown code blocks.
func parseDecision(text string) (Decision, error) {
	text = strings.TrimSpace(text)

	// Try direct parse first
	var d Decision
	if err := json.Unmarshal([]byte(text), &d); err == nil {
		return d, nil
	}

	// Try to extract JSON from markdown code block
	if idx := strings.Index(text, "```json"); idx >= 0 {
		start := idx + len("```json")
		if end := strings.Index(text[start:], "```"); end >= 0 {
			text = strings.TrimSpace(text[start : start+end])
			if err := json.Unmarshal([]byte(text), &d); err == nil {
				return d, nil
			}
		}
	}

	// Try to extract JSON from generic code block
	if idx := strings.Index(text, "```"); idx >= 0 {
		start := idx + len("```")
		// Skip to end of first line (past language identifier)
		if nl := strings.Index(text[start:], "\n"); nl >= 0 {
			start += nl + 1
		}
		if end := strings.Index(text[start:], "```"); end >= 0 {
			text = strings.TrimSpace(text[start : start+end])
			if err := json.Unmarshal([]byte(text), &d); err == nil {
				return d, nil
			}
		}
	}

	// Try to find a JSON object in the text
	if idx := strings.Index(text, "{"); idx >= 0 {
		// Find the matching closing brace
		depth := 0
		for i := idx; i < len(text); i++ {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					candidate := text[idx : i+1]
					if err := json.Unmarshal([]byte(candidate), &d); err == nil {
						return d, nil
					}
				}
			}
		}
	}

	return Decision{}, fmt.Errorf("ralph: could not parse decision from LLM response")
}
