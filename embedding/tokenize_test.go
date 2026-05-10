package embedding

import "testing"

func TestTokenizeNormalizesWordsAndDigits(t *testing.T) {
	got := Tokenize("Roadmap: P43 embeds TF-IDF, GraphRAG, and cache_v2!")
	want := []string{"roadmap", "p43", "embeds", "tf", "idf", "graphrag", "and", "cache", "v2"}
	if len(got) != len(want) {
		t.Fatalf("Tokenize length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Tokenize()[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func TestTokenizerStopWordsAndMinLength(t *testing.T) {
	tokenizer := Tokenizer{
		MinLength: 1,
		StopWords: NewStopWordSet("and", "the"),
	}
	got := tokenizer.Tokenize("A roadmap and the TF-IDF index")
	want := []string{"a", "roadmap", "tf", "idf", "index"}
	if len(got) != len(want) {
		t.Fatalf("Tokenize length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Tokenize()[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}
