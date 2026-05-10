package discovery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunPublishWorkflow_Register(t *testing.T) {
	t.Parallel()

	var capturedMethod, capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		writeJSON(w, ServerMetadata{ID: "srv-1", Name: "published"})
	}))
	defer srv.Close()

	result, err := RunPublishWorkflow(context.Background(), PublishWorkflowConfig{
		PublisherConfig: PublisherConfig{BaseURL: srv.URL, Token: "tok"},
		Metadata:        validPublishMetadata(),
	})
	if err != nil {
		t.Fatalf("RunPublishWorkflow: %v", err)
	}
	if result.Published.ID != "srv-1" {
		t.Fatalf("published ID = %q, want srv-1", result.Published.ID)
	}
	if capturedMethod != http.MethodPost || capturedPath != "/v1/servers" {
		t.Fatalf("request = %s %s, want POST /v1/servers", capturedMethod, capturedPath)
	}
	assertWorkflowStep(t, result.Steps, "validate", "complete")
	assertWorkflowStep(t, result.Steps, "publisher", "complete")
	assertWorkflowStep(t, result.Steps, "publish", "complete")
}

func TestRunPublishWorkflow_Update(t *testing.T) {
	t.Parallel()

	var capturedMethod, capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		writeJSON(w, ServerMetadata{ID: "srv-2", Name: "updated"})
	}))
	defer srv.Close()

	meta := validPublishMetadata()
	meta.ID = "srv-2"
	result, err := RunPublishWorkflow(context.Background(), PublishWorkflowConfig{
		PublisherConfig: PublisherConfig{BaseURL: srv.URL, Token: "tok"},
		Metadata:        meta,
		Mode:            PublishModeUpdate,
	})
	if err != nil {
		t.Fatalf("RunPublishWorkflow: %v", err)
	}
	if result.Mode != PublishModeUpdate {
		t.Fatalf("mode = %s, want update", result.Mode)
	}
	if capturedMethod != http.MethodPut || capturedPath != "/v1/servers/srv-2" {
		t.Fatalf("request = %s %s, want PUT /v1/servers/srv-2", capturedMethod, capturedPath)
	}
}

func TestRunPublishWorkflow_ValidateOnlySkipsRegistry(t *testing.T) {
	t.Parallel()

	result, err := RunPublishWorkflow(context.Background(), PublishWorkflowConfig{
		Metadata:     validPublishMetadata(),
		ValidateOnly: true,
	})
	if err != nil {
		t.Fatalf("RunPublishWorkflow: %v", err)
	}
	if !result.Skipped {
		t.Fatal("Skipped = false, want true")
	}
	assertWorkflowStep(t, result.Steps, "publish", "skipped")
}

func TestRunPublishWorkflow_InvalidMetadataDoesNotCallRegistry(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()

	result, err := RunPublishWorkflow(context.Background(), PublishWorkflowConfig{
		PublisherConfig: PublisherConfig{BaseURL: srv.URL, Token: "tok"},
		Metadata:        ServerMetadata{Name: "missing-fields"},
	})
	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want ErrValidationFailed", err)
	}
	if result == nil || result.Validation.Valid {
		t.Fatalf("validation = %#v, want invalid report", result)
	}
	if called {
		t.Fatal("registry was called despite invalid metadata")
	}
	assertWorkflowStep(t, result.Steps, "validate", "failed")
}

func TestRunPublishWorkflow_UpdateRequiresID(t *testing.T) {
	t.Parallel()

	meta := validPublishMetadata()
	result, err := RunPublishWorkflow(context.Background(), PublishWorkflowConfig{
		Publisher: &Publisher{},
		Metadata:  meta,
		Mode:      PublishModeUpdate,
	})
	if err == nil {
		t.Fatal("expected update without id error")
	}
	if result == nil {
		t.Fatal("expected workflow result")
	}
	assertWorkflowStep(t, result.Steps, "publish", "failed")
}

func assertWorkflowStep(t *testing.T, steps []PublishWorkflowStep, name, status string) {
	t.Helper()
	for _, step := range steps {
		if step.Name == name {
			if step.Status != status {
				t.Fatalf("step %s status = %s, want %s", name, step.Status, status)
			}
			return
		}
	}
	t.Fatalf("missing step %q in %#v", name, steps)
}
