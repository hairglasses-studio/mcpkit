package fulfillment

// ExecuteStepInput requests execution of a fulfillment pipeline step.
type ExecuteStepInput struct {
	DeployDir  string `json:"deploy_dir,omitempty"`
	StepName   string `json:"step_name"`
	DryRun     bool   `json:"dry_run,omitempty"`
	TimeoutSec int    `json:"timeout_sec,omitempty"`
}

// ExecuteStepOutput describes the execution result.
type ExecuteStepOutput struct {
	StepName      string   `json:"step_name"`
	Status        string   `json:"status"`
	DurationMs    int64    `json:"duration_ms"`
	OutputSummary string   `json:"output_summary"`
	Errors        []string `json:"errors,omitempty"`
}

// CheckInvariantsInput allows overriding safety thresholds.
type CheckInvariantsInput struct {
	MinNVMeFreeGB float64 `json:"min_nvme_free_gb,omitempty"`
	MaxVRAMUsedMB float64 `json:"max_vram_used_mb,omitempty"`
}

// CheckInvariantsOutput provides health and guardrail checks.
type CheckInvariantsOutput struct {
	Report  InvariantReport `json:"report"`
	Details []string        `json:"details"`
}

// SyncMailboxInput coordinates inter-session messaging.
type SyncMailboxInput struct {
	SessionID     string   `json:"session_id"`
	TeamID        string   `json:"team_id"`
	Subject       string   `json:"subject"`
	Deliverables  []string `json:"deliverables,omitempty"`
	InFlightTasks []string `json:"in_flight_tasks,omitempty"`
}

// SyncMailboxOutput returns status of mailbox delivery.
type SyncMailboxOutput struct {
	MailboxFile string `json:"mailbox_file"`
	Success     bool   `json:"success"`
	Timestamp   string `json:"timestamp"`
}

// RunE2EInput initiates the complete autonomous self-fulfillment cycle.
type RunE2EInput struct {
	DeployDir string `json:"deploy_dir,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
	MaxSteps  int    `json:"max_steps,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// RunE2EOutput summarizes the end-to-end self-fulfillment loop execution.
type RunE2EOutput struct {
	Success           bool     `json:"success"`
	InvariantPassed   bool     `json:"invariant_passed"`
	StepsExecuted     []string `json:"steps_executed"`
	ResolvedGapsCount int      `json:"resolved_gaps_count"`
	MailboxSynced     bool     `json:"mailbox_synced"`
	Duration          string   `json:"duration"`
	Summary           string   `json:"summary"`
}
