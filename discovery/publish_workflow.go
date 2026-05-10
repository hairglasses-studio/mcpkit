package discovery

import (
	"context"
	"fmt"
)

// PublishMode selects whether a publish workflow creates or updates metadata.
type PublishMode string

const (
	PublishModeRegister PublishMode = "register"
	PublishModeUpdate   PublishMode = "update"
)

// PublishWorkflowConfig configures a validation-first registry publish run.
type PublishWorkflowConfig struct {
	// Publisher is used directly when non-nil. Otherwise PublisherConfig is used.
	Publisher *Publisher
	// PublisherConfig creates a Publisher when Publisher is nil.
	PublisherConfig PublisherConfig
	// Metadata is the server metadata to validate and publish.
	Metadata ServerMetadata
	// Mode selects register or update. Empty defaults to register.
	Mode PublishMode
	// ServerID is required for update mode. If empty, Metadata.ID is used.
	ServerID string
	// ValidateOnly runs validation and stops before calling the registry.
	ValidateOnly bool
}

// PublishWorkflowStep records one workflow phase.
type PublishWorkflowStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// PublishWorkflowResult is returned by RunPublishWorkflow.
type PublishWorkflowResult struct {
	Mode       PublishMode           `json:"mode"`
	Validation ValidationReport      `json:"validation"`
	Published  ServerMetadata        `json:"published,omitempty"`
	Steps      []PublishWorkflowStep `json:"steps,omitempty"`
	Skipped    bool                  `json:"skipped,omitempty"`
}

// RunPublishWorkflow validates metadata, checks local schema compliance, and
// then registers or updates it in the registry.
func RunPublishWorkflow(ctx context.Context, cfg PublishWorkflowConfig) (*PublishWorkflowResult, error) {
	mode := cfg.Mode
	if mode == "" {
		mode = PublishModeRegister
	}
	result := &PublishWorkflowResult{Mode: mode}

	validation := ValidateServerMetadata(cfg.Metadata)
	result.Validation = validation
	if err := validation.Err(); err != nil {
		result.Steps = append(result.Steps, PublishWorkflowStep{
			Name:   "validate",
			Status: "failed",
			Error:  err.Error(),
		})
		return result, err
	}
	result.Steps = append(result.Steps, PublishWorkflowStep{Name: "validate", Status: "complete"})

	if cfg.ValidateOnly {
		result.Steps = append(result.Steps, PublishWorkflowStep{Name: "publish", Status: "skipped"})
		result.Skipped = true
		return result, nil
	}

	publisher := cfg.Publisher
	if publisher == nil {
		var err error
		publisher, err = NewPublisher(cfg.PublisherConfig)
		if err != nil {
			result.Steps = append(result.Steps, PublishWorkflowStep{
				Name:   "publisher",
				Status: "failed",
				Error:  err.Error(),
			})
			return result, err
		}
	}
	result.Steps = append(result.Steps, PublishWorkflowStep{Name: "publisher", Status: "complete"})

	var (
		published ServerMetadata
		err       error
	)
	switch mode {
	case PublishModeRegister:
		published, err = publisher.Register(ctx, cfg.Metadata)
	case PublishModeUpdate:
		id := cfg.ServerID
		if id == "" {
			id = cfg.Metadata.ID
		}
		if id == "" {
			err = fmt.Errorf("discovery: server id is required for update workflow")
			break
		}
		published, err = publisher.Update(ctx, id, cfg.Metadata)
	default:
		err = fmt.Errorf("discovery: unsupported publish mode %q", mode)
	}
	if err != nil {
		result.Steps = append(result.Steps, PublishWorkflowStep{
			Name:   "publish",
			Status: "failed",
			Error:  err.Error(),
		})
		return result, err
	}

	result.Published = published
	result.Steps = append(result.Steps, PublishWorkflowStep{Name: "publish", Status: "complete"})
	return result, nil
}
