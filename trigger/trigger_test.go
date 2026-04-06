package trigger

import (
	"sort"
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry()
	if reg.Count() != 0 {
		t.Errorf("Count() = %d, want 0", reg.Count())
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	src := &StaticSource{
		SourceName:        "webhook-pr",
		SourceType:        "webhook",
		SourceDescription: "Fires on PR events",
		IsActive:          true,
	}
	reg.Register(src)

	got, ok := reg.Get("webhook-pr")
	if !ok {
		t.Fatal("expected to find registered source")
	}
	if got.Name() != "webhook-pr" {
		t.Errorf("Name() = %q, want %q", got.Name(), "webhook-pr")
	}
	if got.Type() != "webhook" {
		t.Errorf("Type() = %q, want %q", got.Type(), "webhook")
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := NewRegistry()
	_, ok := reg.Get("nonexistent")
	if ok {
		t.Error("expected not found for unregistered source")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&StaticSource{SourceName: "cron-daily", SourceType: "cron", IsActive: true})
	reg.Unregister("cron-daily")

	_, ok := reg.Get("cron-daily")
	if ok {
		t.Error("expected not found after unregister")
	}
	if reg.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after unregister", reg.Count())
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&StaticSource{SourceName: "a", SourceType: "cron", IsActive: true})
	reg.Register(&StaticSource{SourceName: "b", SourceType: "webhook", IsActive: false})
	reg.Register(&StaticSource{SourceName: "c", SourceType: "manual", IsActive: true})

	names := reg.List()
	sort.Strings(names)
	if len(names) != 3 {
		t.Fatalf("List() = %d names, want 3", len(names))
	}
	if names[0] != "a" || names[1] != "b" || names[2] != "c" {
		t.Errorf("List() = %v, want [a b c]", names)
	}
}

func TestRegistry_ListActive(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&StaticSource{SourceName: "active1", SourceType: "cron", IsActive: true})
	reg.Register(&StaticSource{SourceName: "inactive", SourceType: "cron", IsActive: false})
	reg.Register(&StaticSource{SourceName: "active2", SourceType: "webhook", IsActive: true})

	active := reg.ListActive()
	sort.Strings(active)
	if len(active) != 2 {
		t.Fatalf("ListActive() = %d, want 2", len(active))
	}
	if active[0] != "active1" || active[1] != "active2" {
		t.Errorf("ListActive() = %v, want [active1 active2]", active)
	}
}

func TestRegistry_RecordTrigger(t *testing.T) {
	reg := NewRegistry()
	now := time.Now()

	reg.RecordTrigger(TriggerRecord{
		ID:        "evt-001",
		Source:    "webhook-pr",
		Type:      "webhook",
		Timestamp: now,
		Payload:   map[string]any{"action": "opened"},
	})

	records := reg.Records()
	if len(records) != 1 {
		t.Fatalf("Records() = %d, want 1", len(records))
	}
	if records[0].ID != "evt-001" {
		t.Errorf("ID = %q, want %q", records[0].ID, "evt-001")
	}
	if records[0].Source != "webhook-pr" {
		t.Errorf("Source = %q, want %q", records[0].Source, "webhook-pr")
	}
}

func TestRegistry_RecordsSince(t *testing.T) {
	reg := NewRegistry()
	t0 := time.Now()
	t1 := t0.Add(time.Second)
	t2 := t0.Add(2 * time.Second)

	reg.RecordTrigger(TriggerRecord{ID: "a", Timestamp: t0})
	reg.RecordTrigger(TriggerRecord{ID: "b", Timestamp: t1})
	reg.RecordTrigger(TriggerRecord{ID: "c", Timestamp: t2})

	recent := reg.RecordsSince(t0.Add(500 * time.Millisecond))
	if len(recent) != 2 {
		t.Fatalf("RecordsSince() = %d, want 2", len(recent))
	}
}

func TestRegistry_Records_Immutable(t *testing.T) {
	reg := NewRegistry()
	reg.RecordTrigger(TriggerRecord{ID: "x", Timestamp: time.Now()})

	records := reg.Records()
	records[0].ID = "modified"

	// Original should be unchanged.
	original := reg.Records()
	if original[0].ID != "x" {
		t.Error("Records() should return a copy, not a reference")
	}
}

func TestStaticSource(t *testing.T) {
	src := &StaticSource{
		SourceName:        "test",
		SourceType:        "manual",
		SourceDescription: "Test source",
		IsActive:          true,
	}
	if src.Name() != "test" {
		t.Errorf("Name() = %q", src.Name())
	}
	if src.Type() != "manual" {
		t.Errorf("Type() = %q", src.Type())
	}
	if src.Description() != "Test source" {
		t.Errorf("Description() = %q", src.Description())
	}
	if !src.Active() {
		t.Error("Active() = false, want true")
	}
}
