
package research

import (
	"context"
	"testing"
)

func TestDiffTool_NoChanges(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	snapshot := SummaryOutput{
		Report: "# Report",
		Sections: []Section{
			{Title: "MCP Specification", Content: "Coverage: 72%\n"},
		},
		ActionItems: []string{"Upgrade mcp-go to v0.47.0"},
		UpdatedFeatureMatrix: []FeatureEntry{
			{1, "Tools (registration, middleware, search)", "Draft", "Implemented", ConfidenceHigh, ""},
		},
	}

	out, err := m.handleDiffAnalysis(ctx, DiffInput{Before: snapshot, After: snapshot})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.NewActionItems) != 0 {
		t.Errorf("new action items = %v, want none", out.NewActionItems)
	}
	if len(out.ResolvedItems) != 0 {
		t.Errorf("resolved items = %v, want none", out.ResolvedItems)
	}
	if len(out.ChangedSections) != 0 {
		t.Errorf("changed sections = %v, want none", out.ChangedSections)
	}
	if len(out.FeatureChanges) != 0 {
		t.Errorf("feature changes = %v, want none", out.FeatureChanges)
	}
	if out.Summary != "No changes detected between snapshots" {
		t.Errorf("summary = %q, want 'No changes detected between snapshots'", out.Summary)
	}
}

func TestDiffTool_NewActionItems(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	before := SummaryOutput{
		ActionItems: []string{"Upgrade mcp-go to v0.47.0"},
	}
	after := SummaryOutput{
		ActionItems: []string{
			"Upgrade mcp-go to v0.47.0",
			"Investigate spec change: elicitation renamed",
		},
	}

	out, err := m.handleDiffAnalysis(ctx, DiffInput{Before: before, After: after})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.NewActionItems) != 1 {
		t.Fatalf("new action items count = %d, want 1", len(out.NewActionItems))
	}
	if out.NewActionItems[0] != "Investigate spec change: elicitation renamed" {
		t.Errorf("new action item = %q, want 'Investigate spec change: elicitation renamed'", out.NewActionItems[0])
	}
	if len(out.ResolvedItems) != 0 {
		t.Errorf("resolved items = %v, want none", out.ResolvedItems)
	}
}

func TestDiffTool_ResolvedItems(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	before := SummaryOutput{
		ActionItems: []string{
			"Upgrade mcp-go to v0.47.0",
			"Fix OAuth token exchange",
		},
	}
	after := SummaryOutput{
		ActionItems: []string{"Upgrade mcp-go to v0.47.0"},
	}

	out, err := m.handleDiffAnalysis(ctx, DiffInput{Before: before, After: after})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.ResolvedItems) != 1 {
		t.Fatalf("resolved items count = %d, want 1", len(out.ResolvedItems))
	}
	if out.ResolvedItems[0] != "Fix OAuth token exchange" {
		t.Errorf("resolved item = %q, want 'Fix OAuth token exchange'", out.ResolvedItems[0])
	}
	if len(out.NewActionItems) != 0 {
		t.Errorf("new action items = %v, want none", out.NewActionItems)
	}
}

func TestDiffTool_ChangedSections(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	before := SummaryOutput{
		Sections: []Section{
			{Title: "MCP Specification", Content: "Coverage: 50%\n"},
			{Title: "SDK Releases", Content: "Current: v0.45.0\n"},
		},
	}
	after := SummaryOutput{
		Sections: []Section{
			{Title: "MCP Specification", Content: "Coverage: 72%\n"},
			{Title: "Assessment", Content: "High priority: upgrade mcp-go\n"},
		},
	}

	out, err := m.handleDiffAnalysis(ctx, DiffInput{Before: before, After: after})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect: MCP Specification modified, SDK Releases removed, Assessment added
	if len(out.ChangedSections) != 3 {
		t.Fatalf("changed sections count = %d, want 3", len(out.ChangedSections))
	}

	changesByTitle := make(map[string]SectionDiff, len(out.ChangedSections))
	for _, d := range out.ChangedSections {
		changesByTitle[d.Title] = d
	}

	spec, ok := changesByTitle["MCP Specification"]
	if !ok {
		t.Error("expected MCP Specification in changed sections")
	} else if spec.Change != "modified" {
		t.Errorf("MCP Specification change = %q, want 'modified'", spec.Change)
	}

	sdk, ok := changesByTitle["SDK Releases"]
	if !ok {
		t.Error("expected SDK Releases in changed sections")
	} else if sdk.Change != "removed" {
		t.Errorf("SDK Releases change = %q, want 'removed'", sdk.Change)
	}

	assess, ok := changesByTitle["Assessment"]
	if !ok {
		t.Error("expected Assessment in changed sections")
	} else if assess.Change != "added" {
		t.Errorf("Assessment change = %q, want 'added'", assess.Change)
	}
}

func TestDiffTool_FeatureStatusChanges(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	before := SummaryOutput{
		UpdatedFeatureMatrix: []FeatureEntry{
			{1, "Tools (registration, middleware, search)", "Draft", "Implemented", ConfidenceHigh, ""},
			{11, "Resources (URI-based data exposure)", "Draft", "Not implemented", ConfidenceLow, ""},
			{12, "Prompts (reusable prompt templates)", "Draft", "Not implemented", ConfidenceLow, ""},
		},
	}
	after := SummaryOutput{
		UpdatedFeatureMatrix: []FeatureEntry{
			{1, "Tools (registration, middleware, search)", "Draft", "Implemented", ConfidenceHigh, ""},
			{11, "Resources (URI-based data exposure)", "Draft", "Implemented", ConfidenceHigh, "Added resources package"},
			{12, "Prompts (reusable prompt templates)", "Draft", "Partial", ConfidenceMedium, "In progress"},
		},
	}

	out, err := m.handleDiffAnalysis(ctx, DiffInput{Before: before, After: after})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.FeatureChanges) != 2 {
		t.Fatalf("feature changes count = %d, want 2", len(out.FeatureChanges))
	}

	changesByName := make(map[string]FeatureChange, len(out.FeatureChanges))
	for _, c := range out.FeatureChanges {
		changesByName[c.Name] = c
	}

	resources, ok := changesByName["Resources (URI-based data exposure)"]
	if !ok {
		t.Error("expected Resources feature change")
	} else {
		if resources.OldStatus != "Not implemented" {
			t.Errorf("Resources old status = %q, want 'Not implemented'", resources.OldStatus)
		}
		if resources.NewStatus != "Implemented" {
			t.Errorf("Resources new status = %q, want 'Implemented'", resources.NewStatus)
		}
	}

	prompts, ok := changesByName["Prompts (reusable prompt templates)"]
	if !ok {
		t.Error("expected Prompts feature change")
	} else {
		if prompts.OldStatus != "Not implemented" {
			t.Errorf("Prompts old status = %q, want 'Not implemented'", prompts.OldStatus)
		}
		if prompts.NewStatus != "Partial" {
			t.Errorf("Prompts new status = %q, want 'Partial'", prompts.NewStatus)
		}
	}
}

func TestDiffTool_MixedChanges(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	before := SummaryOutput{
		ActionItems: []string{
			"Fix OAuth token exchange",
			"Upgrade mcp-go to v0.47.0",
		},
		Sections: []Section{
			{Title: "MCP Specification", Content: "Coverage: 50%\n"},
		},
		UpdatedFeatureMatrix: []FeatureEntry{
			{7, "OAuth 2.1 Authorization", "2025-03-26", "Partial", ConfidenceMedium, ""},
		},
	}
	after := SummaryOutput{
		ActionItems: []string{
			"Upgrade mcp-go to v0.47.0",
			"Investigate new streaming API",
		},
		Sections: []Section{
			{Title: "MCP Specification", Content: "Coverage: 72%\n"},
			{Title: "GitHub Activity", Content: "3 new commits\n"},
		},
		UpdatedFeatureMatrix: []FeatureEntry{
			{7, "OAuth 2.1 Authorization", "2025-03-26", "Implemented", ConfidenceHigh, "Now complete"},
		},
	}

	out, err := m.handleDiffAnalysis(ctx, DiffInput{Before: before, After: after})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// New action items
	if len(out.NewActionItems) != 1 || out.NewActionItems[0] != "Investigate new streaming API" {
		t.Errorf("new action items = %v, want ['Investigate new streaming API']", out.NewActionItems)
	}
	// Resolved items
	if len(out.ResolvedItems) != 1 || out.ResolvedItems[0] != "Fix OAuth token exchange" {
		t.Errorf("resolved items = %v, want ['Fix OAuth token exchange']", out.ResolvedItems)
	}
	// Changed sections: MCP Specification modified + GitHub Activity added
	if len(out.ChangedSections) != 2 {
		t.Errorf("changed sections count = %d, want 2", len(out.ChangedSections))
	}
	// Feature changes
	if len(out.FeatureChanges) != 1 {
		t.Fatalf("feature changes count = %d, want 1", len(out.FeatureChanges))
	}
	if out.FeatureChanges[0].OldStatus != "Partial" || out.FeatureChanges[0].NewStatus != "Implemented" {
		t.Errorf("feature change = %+v, want Partial→Implemented", out.FeatureChanges[0])
	}

	// Summary should mention all change types
	if out.Summary == "" || out.Summary == "No changes detected between snapshots" {
		t.Error("expected non-trivial summary for mixed changes")
	}
}

func TestDiffTool_EmptySnapshots(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	out, err := m.handleDiffAnalysis(ctx, DiffInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.NewActionItems) != 0 {
		t.Errorf("new action items = %v, want none", out.NewActionItems)
	}
	if out.Summary != "No changes detected between snapshots" {
		t.Errorf("summary = %q", out.Summary)
	}
}
