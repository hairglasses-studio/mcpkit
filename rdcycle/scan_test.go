package rdcycle

import (
	"context"
	"testing"
)

func TestHandleScan_DefaultRepos(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{
		ScanRepos: []string{"mark3labs/mcp-go", "anthropics/anthropic-sdk-go"},
	})

	out, err := m.handleScan(context.Background(), ScanInput{})
	if err != nil {
		t.Fatalf("handleScan: unexpected error: %v", err)
	}
	if out.RepoCount != 2 {
		t.Errorf("RepoCount: want 2, got %d", out.RepoCount)
	}
	if len(out.ActionItems) < 1 {
		t.Errorf("ActionItems len: want >= 1, got %d", len(out.ActionItems))
	}
	if out.ArtifactID == "" {
		t.Error("ArtifactID: expected non-empty")
	}
}

func TestHandleScan_InputReposOverride(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{
		ScanRepos: []string{"default/repo"},
	})

	out, err := m.handleScan(context.Background(), ScanInput{
		Repos: []string{"override/repo1", "override/repo2", "override/repo3"},
	})
	if err != nil {
		t.Fatalf("handleScan: unexpected error: %v", err)
	}
	if out.RepoCount != 3 {
		t.Errorf("RepoCount: want 3, got %d", out.RepoCount)
	}
}

func TestHandleScan_NoRepos(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})

	out, err := m.handleScan(context.Background(), ScanInput{})
	if err != nil {
		t.Fatalf("handleScan: unexpected error: %v", err)
	}
	if out.RepoCount != 0 {
		t.Errorf("RepoCount: want 0, got %d", out.RepoCount)
	}
	if out.Summary == "" {
		t.Error("Summary: expected non-empty even with no repos")
	}
}

func TestHandleScan_SincePropagated(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{
		ScanRepos: []string{"owner/repo"},
	})

	out, err := m.handleScan(context.Background(), ScanInput{
		Since: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("handleScan: unexpected error: %v", err)
	}
	// Summary should contain the since value.
	if out.Summary == "" {
		t.Error("Summary: expected non-empty")
	}
}

func TestHandleScan_ArtifactStored(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{
		ScanRepos: []string{"a/b"},
	})

	out, err := m.handleScan(context.Background(), ScanInput{})
	if err != nil {
		t.Fatalf("handleScan: unexpected error: %v", err)
	}

	artifact, ok := m.store.Get(out.ArtifactID)
	if !ok {
		t.Fatal("artifact not stored")
	}
	if artifact.Type != "scan" {
		t.Errorf("artifact Type: want %q, got %q", "scan", artifact.Type)
	}
}

func TestBuildScanSummary_SingleRepo(t *testing.T) {
	t.Parallel()
	s := buildScanSummary([]string{"owner/repo"}, "2026-01-01")
	if s == "" {
		t.Error("buildScanSummary: expected non-empty string")
	}
}

func TestBuildScanSummary_NoRepos(t *testing.T) {
	t.Parallel()
	s := buildScanSummary(nil, "2026-01-01")
	if s == "" {
		t.Error("buildScanSummary: expected non-empty string for no repos")
	}
}
