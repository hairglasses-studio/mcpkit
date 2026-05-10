package embedding

import (
	"reflect"
	"testing"
)

func TestDocumentsFromRecords(t *testing.T) {
	t.Parallel()

	records := []Record{
		{ID: "1", Text: "first", Metadata: map[string]string{"category": "a"}},
		{ID: "2", Text: "second", Metadata: nil},
		{ID: "3", Text: "third", Metadata: map[string]string{"category": "b", "weight": "0.5"}},
	}
	docs := DocumentsFromRecords(records)

	if len(docs) != 3 {
		t.Fatalf("got %d docs, want 3", len(docs))
	}
	for i, doc := range docs {
		if doc.ID != records[i].ID {
			t.Errorf("doc[%d].ID = %q, want %q", i, doc.ID, records[i].ID)
		}
		if doc.Text != records[i].Text {
			t.Errorf("doc[%d].Text = %q, want %q", i, doc.Text, records[i].Text)
		}
		// Metadata should be cloned, not shared.
		if !reflect.DeepEqual(doc.Metadata, records[i].Metadata) {
			t.Errorf("doc[%d].Metadata mismatch: got %v, want %v", i, doc.Metadata, records[i].Metadata)
		}
	}

	// Mutate clone and verify source isn't affected.
	if docs[0].Metadata != nil {
		docs[0].Metadata["category"] = "MUTATED"
		if records[0].Metadata["category"] == "MUTATED" {
			t.Error("DocumentsFromRecords leaked metadata reference (mutation propagated to source)")
		}
	}
}

func TestDocumentsFromRecords_Empty(t *testing.T) {
	t.Parallel()

	docs := DocumentsFromRecords(nil)
	if len(docs) != 0 {
		t.Errorf("nil records → %d docs, want 0", len(docs))
	}
	docs = DocumentsFromRecords([]Record{})
	if len(docs) != 0 {
		t.Errorf("empty records → %d docs, want 0", len(docs))
	}
}

func TestCloneMetadata(t *testing.T) {
	t.Parallel()

	t.Run("nil_passthrough", func(t *testing.T) {
		t.Parallel()
		if got := cloneMetadata(nil); got != nil {
			t.Errorf("cloneMetadata(nil) = %v, want nil", got)
		}
	})

	t.Run("deep_copy", func(t *testing.T) {
		t.Parallel()
		src := map[string]string{"a": "1", "b": "2"}
		clone := cloneMetadata(src)
		if !reflect.DeepEqual(src, clone) {
			t.Errorf("clone %v != source %v", clone, src)
		}
		clone["a"] = "MUTATED"
		if src["a"] == "MUTATED" {
			t.Error("cloneMetadata returned shared reference (mutation leaked)")
		}
	})

	t.Run("empty_map", func(t *testing.T) {
		t.Parallel()
		clone := cloneMetadata(map[string]string{})
		if clone == nil {
			t.Error("cloneMetadata of empty map should return empty non-nil map (preserves caller distinction)")
		}
		if len(clone) != 0 {
			t.Errorf("clone len = %d, want 0", len(clone))
		}
	})
}
