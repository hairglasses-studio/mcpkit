// Package ralph provides an autonomous loop runner (the Ralph Loop pattern)
// for iterative agent task execution with sampling, cost tracking, and DAG support.
package ralph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/finops"
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
	Name        string `json:"name"        yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Completion  string `json:"completion"  yaml:"completion"`
	Tasks       []Task `json:"tasks"       yaml:"tasks"`
}

// Task is a single unit of work within a Spec.
type Task struct {
	ID          string   `json:"id"                    yaml:"id"`
	Description string   `json:"description"           yaml:"description"`
	Done        bool     `json:"done"                  yaml:"done"`
	DependsOn   []string `json:"depends_on,omitempty"  yaml:"depends_on,omitempty"`
}

// Progress tracks the execution state of a loop run.
type Progress struct {
	SpecFile     string         `json:"spec_file"     yaml:"spec_file"`
	Iteration    int            `json:"iteration"     yaml:"iteration"`
	CompletedIDs []string       `json:"completed_ids" yaml:"completed_ids"`
	Log          []IterationLog `json:"log"           yaml:"log"`
	Status       Status         `json:"status"        yaml:"status"`
	StartedAt    time.Time      `json:"started_at"    yaml:"started_at"`
	UpdatedAt    time.Time      `json:"updated_at"    yaml:"updated_at"`
}

// IterationLog records what happened in a single loop iteration.
type IterationLog struct {
	Iteration int       `json:"iteration"          yaml:"iteration"`
	TaskID    string    `json:"task_id,omitempty"  yaml:"task_id,omitempty"`
	ToolCalls []string  `json:"tool_calls,omitempty" yaml:"tool_calls,omitempty"`
	Result    string    `json:"result"             yaml:"result"`
	Timestamp time.Time `json:"timestamp"          yaml:"timestamp"`
}

// ToolCall represents a single tool invocation within a multi-tool decision.
type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// Decision is the JSON structure the LLM returns each iteration.
type Decision struct {
	Complete  bool                   `json:"complete"`
	TaskID    string                 `json:"task_id,omitempty"`
	ToolName  string                 `json:"tool_name,omitempty"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	ToolCalls []ToolCall             `json:"tool_calls,omitempty"`
	Reasoning string                 `json:"reasoning,omitempty"`
	MarkDone  bool                   `json:"mark_done,omitempty"`
}

// ConversationTurn records one full iteration for multi-turn conversation history.
type ConversationTurn struct {
	UserPrompt    string   // the prompt sent to the LLM
	AssistantText string   // the raw LLM response text
	ToolResults   []string // full tool output per call in that iteration
}

// ResolvedToolCalls returns the effective list of tool calls for this decision.
// If ToolCalls is populated it is returned directly. If only ToolName is set,
// it wraps the single tool into a one-element slice. Otherwise returns nil.
func (d Decision) ResolvedToolCalls() []ToolCall {
	if len(d.ToolCalls) > 0 {
		return d.ToolCalls
	}
	if d.ToolName != "" {
		return []ToolCall{{Name: d.ToolName, Arguments: d.Arguments}}
	}
	return nil
}

// Hooks provides optional callbacks for loop lifecycle events.
type Hooks struct {
	// OnIterationStart is called before each iteration. Receives iteration number.
	OnIterationStart func(iteration int)
	// OnIterationEnd is called after each iteration with the log entry.
	OnIterationEnd func(entry IterationLog)
	// OnTaskComplete is called when a task is marked done.
	OnTaskComplete func(taskID string)
	// OnError is called when an iteration encounters an error (tool not found, etc.)
	OnError func(iteration int, err error)
	// OnCostUpdate is called after each iteration when a CostTracker is configured.
	OnCostUpdate func(iteration int, summary finops.UsageSummary)
}

func (h *Hooks) callIterationStart(iteration int) {
	if h.OnIterationStart != nil {
		h.OnIterationStart(iteration)
	}
}

func (h *Hooks) callIterationEnd(entry IterationLog) {
	if h.OnIterationEnd != nil {
		h.OnIterationEnd(entry)
	}
}

func (h *Hooks) callTaskComplete(taskID string) {
	if h.OnTaskComplete != nil {
		h.OnTaskComplete(taskID)
	}
}

func (h *Hooks) callError(iteration int, err error) {
	if h.OnError != nil {
		h.OnError(iteration, err)
	}
}

func (h *Hooks) callCostUpdate(iteration int, summary finops.UsageSummary) {
	if h.OnCostUpdate != nil {
		h.OnCostUpdate(iteration, summary)
	}
}

// Config configures a Loop execution.
type Config struct {
	MaxIterations int
	SpecFile      string
	ProgressFile  string
	ToolRegistry  *registry.ToolRegistry
	Sampler       sampling.SamplingClient
	MaxTokens     int
	Hooks         Hooks
	CostTracker   *finops.Tracker       // optional: records token usage per iteration
	EstimateFunc  func(string) int      // optional: override token estimation (default: len/4)
	ForceRestart  bool              // if true, ignore existing progress and start fresh
	TemplateVars  map[string]string // optional: template variable substitution for spec file
	// ModelSelector optionally returns a model hint for sampling requests.
	// Called each iteration with the current iteration number and completed task IDs.
	// Return empty string for no preference.
	ModelSelector func(iteration int, completedIDs []string) string
	// HistoryWindow is the number of previous conversation turns to include in the
	// prompt context. When > 0, enables multi-turn conversation mode where the LLM
	// sees previous turns instead of the "Recent Activity" summary.
	// Default 0 means legacy single-turn mode.
	HistoryWindow int
	// PhaseMaxTokens maps task IDs to max token overrides for the sampling request.
	// If a task ID is found here, its value is used instead of Config.MaxTokens.
	PhaseMaxTokens map[string]int
	// AutoVerify when true runs `go build` after write_file calls and includes
	// the result in the conversation history so the LLM sees compilation errors.
	AutoVerify bool
	// ProjectRoot is the working directory for auto-verify commands.
	// Required when AutoVerify is true.
	ProjectRoot string
	// TaskDecomposer is called after a task is marked done, allowing injection
	// of sub-tasks into the spec. Return nil to make no changes.
	TaskDecomposer func(taskID string, progress Progress, spec *Spec) []Task
}

// Loop is the autonomous iteration runner.
type Loop struct {
	config   Config
	mu       sync.Mutex
	progress Progress
	stopCh   chan struct{}
	stopped  bool
	history      []ConversationTurn // multi-turn conversation history
	specModified bool               // true when TaskDecomposer has modified the spec
}

// DefaultProgressFile returns the default progress file path for a spec file.
func DefaultProgressFile(specFile string) string {
	ext := filepath.Ext(specFile)
	return strings.TrimSuffix(specFile, ext) + ".progress.json"
}

// LoadSpec reads and parses a Spec from a JSON or YAML file.
// Files ending in .yaml or .yml are parsed as YAML; all others are parsed as JSON.
func LoadSpec(path string) (Spec, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		return LoadSpecYAML(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("ralph: load spec: %w", err)
	}
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return Spec{}, fmt.Errorf("ralph: parse spec: %w", err)
	}
	if err := ValidateSpec(spec); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

// ValidateSpec checks a Spec for structural issues.
// Returns nil if valid, or an error describing all problems found.
func ValidateSpec(spec Spec) error {
	var problems []string
	if spec.Name == "" {
		problems = append(problems, "name is required")
	}
	if spec.Description == "" {
		problems = append(problems, "description is required")
	}
	if len(spec.Tasks) == 0 {
		problems = append(problems, "at least one task is required")
	}
	seen := make(map[string]bool)
	for i, task := range spec.Tasks {
		if task.ID == "" {
			problems = append(problems, fmt.Sprintf("task[%d]: id is required", i))
		}
		if task.Description == "" {
			problems = append(problems, fmt.Sprintf("task[%d]: description is required", i))
		}
		if task.ID != "" && seen[task.ID] {
			problems = append(problems, fmt.Sprintf("task[%d]: duplicate id %q", i, task.ID))
		}
		seen[task.ID] = true
	}
	if len(problems) > 0 {
		return fmt.Errorf("ralph: invalid spec: %s", strings.Join(problems, "; "))
	}
	if err := ValidateDependencies(spec.Tasks); err != nil {
		return err
	}
	return nil
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
