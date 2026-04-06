//go:build !official_sdk

package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/hitools"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- test helpers ---

func makeTestTools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{Tool: registry.Tool{Name: "lint_go", Description: "Run Go linter"}, Category: "static", Tags: []string{"lint", "go"}},
		{Tool: registry.Tool{Name: "security_scan", Description: "Scan for vulnerabilities"}, Category: "security", Tags: []string{"security", "scan"}},
		{Tool: registry.Tool{Name: "vet_check", Description: "Run go vet static analysis"}, Category: "analysis", Tags: []string{"vet", "static"}},
		{Tool: registry.Tool{Name: "test_runner", Description: "Run test suite"}, Category: "testing", Tags: []string{"test", "verify"}},
		{Tool: registry.Tool{Name: "logic_validator", Description: "Validate correctness of logic"}, Category: "review", Tags: []string{"logic", "review"}},
		{Tool: registry.Tool{Name: "style_checker", Description: "Check code style conventions"}, Category: "style", Tags: []string{"style", "convention"}},
		{Tool: registry.Tool{Name: "doc_generator", Description: "Generate documentation"}, Category: "docs", Tags: []string{"doc", "naming"}},
		{Tool: registry.Tool{Name: "web_search", Description: "Search the web for sources"}, Category: "research", Tags: []string{"search", "web"}},
		{Tool: registry.Tool{Name: "file_reader", Description: "Read file contents"}, Category: "io", Tags: []string{"read", "file"}},
		{Tool: registry.Tool{Name: "pattern_analyzer", Description: "Analyze patterns in data"}, Category: "analysis", Tags: []string{"analyze", "pattern"}},
		{Tool: registry.Tool{Name: "summarizer", Description: "Summarize and synthesize findings"}, Category: "synthesis", Tags: []string{"summarize", "synthesis"}},
		{Tool: registry.Tool{Name: "draft_writer", Description: "Draft formatted content"}, Category: "writing", Tags: []string{"write", "draft"}},
		{Tool: registry.Tool{Name: "citation_tool", Description: "Format citations from sources"}, Category: "writing", Tags: []string{"cite", "format"}},
		{Tool: registry.Tool{Name: "risk_assessor", Description: "Assess risk level of an action"}, Category: "analysis", Tags: []string{"risk", "assess"}},
		{Tool: registry.Tool{Name: "deploy_tool", Description: "Deploy application to production"}, Category: "ops", Tags: []string{"deploy", "execute"}},
		{Tool: registry.Tool{Name: "db_updater", Description: "Update database records"}, Category: "ops", Tags: []string{"update", "write"}},
	}
}

// --- CodeReviewPipeline ---

func TestCodeReviewPipeline_Structure(t *testing.T) {
	tools := makeTestTools()
	g := CodeReviewPipeline(tools)

	// Validate graph structure.
	if err := g.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Verify all three expected nodes exist.
	for _, name := range []string{"static_analysis", "logic_review", "style_review"} {
		if _, ok := g.nodes[name]; !ok {
			t.Errorf("expected node %q in graph", name)
		}
	}

	// Verify start node.
	if g.start != "static_analysis" {
		t.Errorf("start = %q; want static_analysis", g.start)
	}

	// Run the pipeline through the engine and verify stage progression.
	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "code-review", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v; want completed (error: %s)", result.Status, result.Error)
	}
	if result.Steps != 3 {
		t.Errorf("Steps = %d; want 3", result.Steps)
	}

	// Verify all stages completed.
	for _, prefix := range []string{"static_analysis", "logic_review", "style_review"} {
		status, ok := Get[string](result.FinalState, prefix+"_status")
		if !ok || status != "completed" {
			t.Errorf("%s_status = %q, ok=%v; want completed", prefix, status, ok)
		}
	}

	// Verify stage execution order.
	stages, ok := Get[[]string](result.FinalState, "stages_completed")
	if !ok {
		t.Fatal("expected stages_completed in final state")
	}
	expected := []string{"static_analysis", "logic_review", "style_review"}
	if len(stages) != len(expected) {
		t.Fatalf("stages_completed = %v; want %v", stages, expected)
	}
	for i, name := range expected {
		if stages[i] != name {
			t.Errorf("stages_completed[%d] = %q; want %q", i, stages[i], name)
		}
	}

	// Verify tool filtering: static_analysis should have lint/vet/security tools.
	staticTools, ok := Get[[]registry.ToolDefinition](result.FinalState, "static_analysis_tools")
	if !ok {
		t.Fatal("expected static_analysis_tools in state")
	}
	if len(staticTools) == 0 {
		t.Error("static_analysis_tools should not be empty")
	}
	// At least lint_go and security_scan should be present.
	foundLint := false
	foundSecurity := false
	for _, td := range staticTools {
		if td.Tool.Name == "lint_go" {
			foundLint = true
		}
		if td.Tool.Name == "security_scan" {
			foundSecurity = true
		}
	}
	if !foundLint {
		t.Error("expected lint_go in static_analysis_tools")
	}
	if !foundSecurity {
		t.Error("expected security_scan in static_analysis_tools")
	}
}

// --- ResearchWritePipeline ---

func TestResearchWritePipeline_Structure(t *testing.T) {
	tools := makeTestTools()
	g := ResearchWritePipeline(tools)

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Verify all three expected nodes exist.
	for _, name := range []string{"research", "synthesis", "write"} {
		if _, ok := g.nodes[name]; !ok {
			t.Errorf("expected node %q in graph", name)
		}
	}

	if g.start != "research" {
		t.Errorf("start = %q; want research", g.start)
	}

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "research-write", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v; want completed (error: %s)", result.Status, result.Error)
	}
	if result.Steps != 3 {
		t.Errorf("Steps = %d; want 3", result.Steps)
	}

	// Verify all stages completed.
	for _, prefix := range []string{"research", "synthesis", "write"} {
		status, ok := Get[string](result.FinalState, prefix+"_status")
		if !ok || status != "completed" {
			t.Errorf("%s_status = %q, ok=%v; want completed", prefix, status, ok)
		}
	}

	// Verify stage execution order.
	stages, ok := Get[[]string](result.FinalState, "stages_completed")
	if !ok {
		t.Fatal("expected stages_completed in final state")
	}
	expected := []string{"research", "synthesis", "write"}
	if len(stages) != len(expected) {
		t.Fatalf("stages_completed = %v; want %v", stages, expected)
	}
	for i, name := range expected {
		if stages[i] != name {
			t.Errorf("stages_completed[%d] = %q; want %q", i, stages[i], name)
		}
	}

	// Verify tool filtering: research should have search/read tools.
	researchTools, ok := Get[[]registry.ToolDefinition](result.FinalState, "research_tools")
	if !ok {
		t.Fatal("expected research_tools in state")
	}
	if len(researchTools) == 0 {
		t.Error("research_tools should not be empty")
	}
	foundSearch := false
	foundReader := false
	for _, td := range researchTools {
		if td.Tool.Name == "web_search" {
			foundSearch = true
		}
		if td.Tool.Name == "file_reader" {
			foundReader = true
		}
	}
	if !foundSearch {
		t.Error("expected web_search in research_tools")
	}
	if !foundReader {
		t.Error("expected file_reader in research_tools")
	}

	// Verify write stage has write/draft tools.
	writeTools, ok := Get[[]registry.ToolDefinition](result.FinalState, "write_tools")
	if !ok {
		t.Fatal("expected write_tools in state")
	}
	foundDraft := false
	foundCite := false
	for _, td := range writeTools {
		if td.Tool.Name == "draft_writer" {
			foundDraft = true
		}
		if td.Tool.Name == "citation_tool" {
			foundCite = true
		}
	}
	if !foundDraft {
		t.Error("expected draft_writer in write_tools")
	}
	if !foundCite {
		t.Error("expected citation_tool in write_tools")
	}
}

// --- ApprovalGatePipeline ---

func TestApprovalGatePipeline_WithApproval(t *testing.T) {
	tools := makeTestTools()
	store := hitools.NewInMemoryApprovalStore()
	g := ApprovalGatePipeline(tools, store)

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Verify all three expected nodes exist.
	for _, name := range []string{"analyze", "approval_gate", "execute"} {
		if _, ok := g.nodes[name]; !ok {
			t.Errorf("expected node %q in graph", name)
		}
	}

	if g.start != "analyze" {
		t.Errorf("start = %q; want analyze", g.start)
	}

	// Run in background; approve the request when it appears.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan *RunResult, 1)
	go func() {
		result, err := NewEngine(g, EngineConfig{
			MaxSteps:           100,
			DefaultNodeTimeout: 5 * time.Second,
		})
		if err != nil {
			t.Errorf("NewEngine: %v", err)
			return
		}
		initial := Set(NewState(), "approval_action", "deploy to production")
		r, runErr := result.Run(ctx, "approval-test", initial)
		if runErr != nil {
			t.Errorf("Run: %v", runErr)
			return
		}
		done <- r
	}()

	// Poll for the pending approval request and approve it.
	approved := false
	for !approved {
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for approval request to appear")
		default:
		}

		pending, err := store.Pending(ctx)
		if err != nil {
			t.Fatalf("Pending: %v", err)
		}
		if len(pending) > 0 {
			resp := hitools.ApprovalResponse{
				RequestID: pending[0].ID,
				Decision:  hitools.Approved,
				Comment:   "LGTM",
				Timestamp: time.Now(),
			}
			if err := store.Respond(ctx, resp); err != nil {
				t.Fatalf("Respond: %v", err)
			}
			approved = true
		} else {
			time.Sleep(5 * time.Millisecond)
		}
	}

	select {
	case result := <-done:
		if result.Status != RunStatusCompleted {
			t.Errorf("Status = %v; want completed (error: %s)", result.Status, result.Error)
		}

		// Verify all stages ran.
		stages, ok := Get[[]string](result.FinalState, "stages_completed")
		if !ok {
			t.Fatal("expected stages_completed in final state")
		}
		expected := []string{"analyze", "approval_gate", "execute"}
		if len(stages) != len(expected) {
			t.Fatalf("stages_completed = %v; want %v", stages, expected)
		}
		for i, name := range expected {
			if stages[i] != name {
				t.Errorf("stages_completed[%d] = %q; want %q", i, stages[i], name)
			}
		}

		// Verify approval decision recorded.
		decision, _ := Get[string](result.FinalState, "approval_decision")
		if decision != string(hitools.Approved) {
			t.Errorf("approval_decision = %q; want %q", decision, hitools.Approved)
		}

		// Verify execute stage completed.
		execStatus, _ := Get[string](result.FinalState, "execute_status")
		if execStatus != "completed" {
			t.Errorf("execute_status = %q; want completed", execStatus)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for pipeline to complete")
	}
}

func TestApprovalGatePipeline_WithDenial(t *testing.T) {
	tools := makeTestTools()
	store := hitools.NewInMemoryApprovalStore()
	g := ApprovalGatePipeline(tools, store)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan *RunResult, 1)
	go func() {
		e, err := NewEngine(g, EngineConfig{
			MaxSteps:           100,
			DefaultNodeTimeout: 5 * time.Second,
		})
		if err != nil {
			t.Errorf("NewEngine: %v", err)
			return
		}
		initial := Set(NewState(), "approval_action", "drop production database")
		r, runErr := e.Run(ctx, "denial-test", initial)
		if runErr != nil {
			t.Errorf("Run: %v", runErr)
			return
		}
		done <- r
	}()

	// Poll and deny the request.
	denied := false
	for !denied {
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for approval request to appear")
		default:
		}

		pending, err := store.Pending(ctx)
		if err != nil {
			t.Fatalf("Pending: %v", err)
		}
		if len(pending) > 0 {
			resp := hitools.ApprovalResponse{
				RequestID: pending[0].ID,
				Decision:  hitools.Denied,
				Comment:   "too dangerous",
				Timestamp: time.Now(),
			}
			if err := store.Respond(ctx, resp); err != nil {
				t.Fatalf("Respond: %v", err)
			}
			denied = true
		} else {
			time.Sleep(5 * time.Millisecond)
		}
	}

	select {
	case result := <-done:
		if result.Status != RunStatusCompleted {
			t.Errorf("Status = %v; want completed (error: %s)", result.Status, result.Error)
		}

		// Verify only analyze and approval_gate ran (execute should be skipped).
		stages, ok := Get[[]string](result.FinalState, "stages_completed")
		if !ok {
			t.Fatal("expected stages_completed in final state")
		}
		expected := []string{"analyze", "approval_gate"}
		if len(stages) != len(expected) {
			t.Fatalf("stages_completed = %v; want %v", stages, expected)
		}
		for i, name := range expected {
			if stages[i] != name {
				t.Errorf("stages_completed[%d] = %q; want %q", i, stages[i], name)
			}
		}

		// Verify denial recorded.
		decision, _ := Get[string](result.FinalState, "approval_decision")
		if decision != string(hitools.Denied) {
			t.Errorf("approval_decision = %q; want %q", decision, hitools.Denied)
		}
		comment, _ := Get[string](result.FinalState, "approval_comment")
		if comment != "too dangerous" {
			t.Errorf("approval_comment = %q; want %q", comment, "too dangerous")
		}

		// Verify execute stage did NOT run.
		if _, ok := Get[string](result.FinalState, "execute_status"); ok {
			t.Error("execute_status should not be set when approval is denied")
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for pipeline to complete")
	}
}

// --- toolMatchesAny ---

func TestToolMatchesAny(t *testing.T) {
	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "security_scan", Description: "Scan for vulnerabilities"},
		Category: "security",
		Tags:     []string{"security", "scan"},
	}

	if !toolMatchesAny(td, []string{"security"}) {
		t.Error("expected match on category keyword 'security'")
	}
	if !toolMatchesAny(td, []string{"scan"}) {
		t.Error("expected match on tag keyword 'scan'")
	}
	if !toolMatchesAny(td, []string{"vulnerab"}) {
		t.Error("expected match on description keyword 'vulnerab'")
	}
	if toolMatchesAny(td, []string{"deploy", "write"}) {
		t.Error("expected no match on unrelated keywords")
	}
}

func TestToolMatchesAny_CaseInsensitive(t *testing.T) {
	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "Lint_Tool", Description: "Run Linting"},
		Category: "ANALYSIS",
		Tags:     []string{"LINT"},
	}

	if !toolMatchesAny(td, []string{"lint"}) {
		t.Error("expected case-insensitive match on tag")
	}
	if !toolMatchesAny(td, []string{"analysis"}) {
		t.Error("expected case-insensitive match on category")
	}
	if !toolMatchesAny(td, []string{"LINTING"}) {
		t.Error("expected case-insensitive match on description")
	}
}

// --- Empty tools ---

func TestCodeReviewPipeline_EmptyTools(t *testing.T) {
	g := CodeReviewPipeline(nil)
	if err := g.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "empty-tools", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v; want completed", result.Status)
	}
	// All tool lists should be nil (no matching tools).
	if tools, ok := Get[[]registry.ToolDefinition](result.FinalState, "static_analysis_tools"); ok && len(tools) > 0 {
		t.Error("expected no tools for static_analysis with nil input")
	}
}

func TestResearchWritePipeline_EmptyTools(t *testing.T) {
	g := ResearchWritePipeline(nil)
	if err := g.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "empty-tools-rw", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v; want completed", result.Status)
	}
}
