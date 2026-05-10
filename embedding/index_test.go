package embedding

import "testing"

func TestIndexSearchRanksRelevantDocuments(t *testing.T) {
	index := NewIndex([]Document{
		{ID: "roadmap", Text: "roadmap consolidation semantic cache graph retrieval"},
		{ID: "media", Text: "audio transcription video subtitle workflow"},
		{ID: "legal", Text: "evidence search filings damages declaration"},
	})

	results := index.Search("semantic roadmap cache", 2)
	if len(results) != 1 {
		t.Fatalf("Search returned %d results, want 1: %+v", len(results), results)
	}
	if results[0].Document.ID != "roadmap" {
		t.Fatalf("top result = %q, want roadmap", results[0].Document.ID)
	}
	if results[0].Score <= 0 {
		t.Fatalf("top score = %v, want positive", results[0].Score)
	}
}

func TestIndexClonesMetadata(t *testing.T) {
	doc := Document{
		ID:       "a",
		Text:     "semantic cache",
		Metadata: map[string]string{"repo": "ralphglasses"},
	}
	index := NewIndex([]Document{doc})
	doc.Metadata["repo"] = "mutated"

	docs := index.Documents()
	if docs[0].Metadata["repo"] != "ralphglasses" {
		t.Fatalf("stored metadata = %q, want original", docs[0].Metadata["repo"])
	}
	docs[0].Metadata["repo"] = "changed"
	if index.Documents()[0].Metadata["repo"] != "ralphglasses" {
		t.Fatalf("Documents returned mutable metadata")
	}
}

func TestDocumentsFromMapSortsByID(t *testing.T) {
	docs := DocumentsFromMap(map[string]string{
		"b": "second",
		"a": "first",
	})
	if len(docs) != 2 || docs[0].ID != "a" || docs[1].ID != "b" {
		t.Fatalf("DocumentsFromMap order = %+v, want a then b", docs)
	}
}
