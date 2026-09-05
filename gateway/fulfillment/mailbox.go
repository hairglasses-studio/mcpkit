package fulfillment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var defaultMailboxDirs = []string{
	"/home/hg/teamwork_projects/stash_deployment/FLEET_INTER_SESSION_MAILBOX",
	"/home/hg/hairglasses-studio/.agents/session_coordination/FLEET_INTER_SESSION_MAILBOX",
}

// SyncMailbox writes coordination messages to all fleet mailbox hubs.
func SyncMailbox(sessionID, teamID, subject string, deliverables, inFlight []string) (SyncMailboxOutput, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := map[string]any{
		"from_session":   sessionID,
		"from_team":      teamID,
		"from_role":      "Go MCP Unified Self-Fulfillment Gateway",
		"timestamp_utc":  now,
		"subject":        subject,
		"status":         "DELIVERED_AND_BOUND",
		"deliverables":   deliverables,
		"in_flight_tasks": inFlight,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return SyncMailboxOutput{}, err
	}

	var writtenPath string
	for _, dir := range defaultMailboxDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		targetFile := filepath.Join(dir, fmt.Sprintf("FROM_SESSION_%s_TO_GATEWAY.json", sessionID))
		if err := os.WriteFile(targetFile, data, 0644); err == nil {
			writtenPath = targetFile
		}
	}

	if writtenPath == "" {
		writtenPath = "/tmp/fulfillment_gateway_mailbox.json"
		_ = os.WriteFile(writtenPath, data, 0644)
	}

	return SyncMailboxOutput{
		MailboxFile: writtenPath,
		Success:     true,
		Timestamp:   now,
	}, nil
}
