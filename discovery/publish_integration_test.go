package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublishWorkflow_FullLifecycleIntegration(t *testing.T) {
	t.Parallel()

	store := map[string]ServerMetadata{}
	var sequence []string
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer lifecycle-token" {
			t.Fatalf("Authorization = %q, want lifecycle token", got)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/servers":
			sequence = append(sequence, "register")
			var meta ServerMetadata
			if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
				t.Fatalf("decode register: %v", err)
			}
			meta.ID = "srv-life"
			store[meta.ID] = meta
			writeJSON(w, meta)
		case r.Method == http.MethodPut && r.URL.Path == "/v1/servers/srv-life":
			sequence = append(sequence, "update")
			var meta ServerMetadata
			if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
				t.Fatalf("decode update: %v", err)
			}
			meta.ID = "srv-life"
			store[meta.ID] = meta
			writeJSON(w, meta)
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/servers/srv-life":
			sequence = append(sequence, "delete")
			delete(store, "srv-life")
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer registry.Close()

	publisher, err := NewPublisher(PublisherConfig{BaseURL: registry.URL, Token: "lifecycle-token"})
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}

	meta := validPublishMetadata()
	registered, err := RunPublishWorkflow(context.Background(), PublishWorkflowConfig{
		Publisher: publisher,
		Metadata:  meta,
	})
	if err != nil {
		t.Fatalf("register workflow: %v", err)
	}
	if registered.Published.ID != "srv-life" {
		t.Fatalf("registered id = %q, want srv-life", registered.Published.ID)
	}

	meta.Description = "Updated description"
	updated, err := RunPublishWorkflow(context.Background(), PublishWorkflowConfig{
		Publisher: publisher,
		Metadata:  meta,
		Mode:      PublishModeUpdate,
		ServerID:  "srv-life",
	})
	if err != nil {
		t.Fatalf("update workflow: %v", err)
	}
	if updated.Published.Description != "Updated description" {
		t.Fatalf("updated description = %q", updated.Published.Description)
	}

	if err := publisher.Deregister(context.Background(), "srv-life"); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if len(store) != 0 {
		t.Fatalf("store = %#v, want empty after delete", store)
	}
	if got := strings.Join(sequence, ","); got != "register,update,delete" {
		t.Fatalf("sequence = %s, want register,update,delete", got)
	}
}
