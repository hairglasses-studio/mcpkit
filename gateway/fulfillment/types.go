package fulfillment

import "time"

// SequenceGapItem represents a missing item in a series sequence.
type SequenceGapItem struct {
	Series    string `json:"series"`
	MissingID string `json:"missing_id"`
	Priority  int    `json:"priority"`
	Status    string `json:"status"`
}

// InvariantReport represents the host safety and hardware invariant status.
type InvariantReport struct {
	Passed          bool    `json:"passed"`
	NVMeFreeGB      float64 `json:"nvme_free_gb"`
	NVMeTargetMinGB float64 `json:"nvme_target_min_gb"`
	VRAMUsedMB      float64 `json:"vram_used_mb"`
	VRAMLimitMB     float64 `json:"vram_limit_mb"`
	DesktopToasts   int     `json:"desktop_toasts"`
	StashAppStatus  string  `json:"stashapp_status"`
	Timestamp       string  `json:"timestamp"`
}

// MatrixStatusInput specifies options for querying matrix status.
type MatrixStatusInput struct {
	DeployDir string `json:"deploy_dir,omitempty"`
	Verbose   bool   `json:"verbose,omitempty"`
}

// MatrixStatusOutput summarizes the current self-fulfillment matrix state.
type MatrixStatusOutput struct {
	ResolutionMatrixCount  int             `json:"resolution_matrix_count"`
	SeriesFulfillmentCount int             `json:"series_fulfillment_count"`
	CandidateQueueCount    int             `json:"candidate_queue_count"`
	DedupMatrixCount       int             `json:"dedup_matrix_count"`
	Invariants             InvariantReport `json:"invariants"`
	ActiveExclusiveLeases  []string        `json:"active_exclusive_leases"`
	Timestamp              string          `json:"timestamp"`
}

// ScanGapsInput specifies filters for sequence gap scanning.
type ScanGapsInput struct {
	DeployDir  string `json:"deploy_dir,omitempty"`
	SeriesName string `json:"series_name,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// ScanGapsOutput returns detected sequence gaps.
type ScanGapsOutput struct {
	TotalGapsFound    int               `json:"total_gaps_found"`
	HighPriorityCount int               `json:"high_priority_count"`
	Gaps              []SequenceGapItem `json:"gaps"`
}

// ClaimBatchInput requests an exclusive lease on a sequence batch.
type ClaimBatchInput struct {
	BatchKey         string `json:"batch_key"`
	Scope            string `json:"scope"`
	OwnerSession     string `json:"owner_session"`
	LeaseDurationSec int    `json:"lease_duration_sec,omitempty"`
}

// ClaimBatchOutput confirms whether the lease was granted.
type ClaimBatchOutput struct {
	ClaimKey  string `json:"claim_key"`
	Scope     string `json:"scope"`
	Owner     string `json:"owner"`
	Granted   bool   `json:"granted"`
	ExpiresAt string `json:"expires_at"`
	Message   string `json:"message"`
}

// LeaseRecord tracks active exclusive leases.
type LeaseRecord struct {
	ClaimKey  string    `json:"claim_key"`
	Scope     string    `json:"scope"`
	Owner     string    `json:"owner"`
	GrantedAt time.Time `json:"granted_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
