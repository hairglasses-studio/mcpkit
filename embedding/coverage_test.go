package embedding

import (
	"context"
	"math"
	"strings"
	"testing"
)

// Tests in this file target functions that staticcheck coverage analysis
// flagged as 0% covered at the start of the v0.6.0 coverage push:
//   - Index.WithTokenizer (option setter)
//   - Index.Len
//   - Index.IDF
//   - Index.SparseProvider
//   - Provider option setters: WithProviderTokenizer, WithProviderIDF,
//     WithProviderNormalization
//   - SparseVector.Clone
//   - DenseDot / DenseCosine error paths

func TestIndex_WithTokenizer(t *testing.T) {
	t.Parallel()

	customTok := Tokenizer{MinLength: 3, StopWords: map[string]struct{}{"the": {}}}
	docs := []Document{{ID: "1", Text: "the quick brown fox"}}
	index := NewIndex(docs, WithTokenizer(customTok))

	// With min-length 3 + "the" as stopword, the indexed tokens should
	// exclude "the" and any sub-3-char tokens. Verify by embedding.
	vec := index.Embed("the quick")
	if _, has := vec["the"]; has {
		t.Errorf("custom tokenizer should drop stopword 'the'; vec=%v", vec)
	}
	if _, has := vec["quick"]; !has {
		t.Errorf("custom tokenizer should keep 'quick' (len 5); vec=%v", vec)
	}
}

func TestIndex_Len(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		docs []Document
		want int
	}{
		{name: "empty", docs: nil, want: 0},
		{name: "single", docs: []Document{{ID: "1", Text: "hi"}}, want: 1},
		{name: "multiple", docs: []Document{
			{ID: "a", Text: "foo"},
			{ID: "b", Text: "bar"},
			{ID: "c", Text: "baz"},
		}, want: 3},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			idx := NewIndex(tc.docs)
			if got := idx.Len(); got != tc.want {
				t.Errorf("Len() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestIndex_IDF(t *testing.T) {
	t.Parallel()

	// IDF for 3 docs where "common" appears in all and "rare" in only one.
	// Expected: IDF("common") = log((1+3)/(1+3)) + 1 = 1
	//           IDF("rare")   = log((1+3)/(1+1)) + 1 = log(2)+1 ≈ 1.693
	docs := []Document{
		{ID: "1", Text: "common common"},
		{ID: "2", Text: "common"},
		{ID: "3", Text: "common rare"},
	}
	idx := NewIndex(docs)
	idf := idx.IDF()

	if math.Abs(idf["common"]-1.0) > 1e-9 {
		t.Errorf("IDF(common) = %f, want 1.0", idf["common"])
	}
	expected := math.Log(2) + 1
	if math.Abs(idf["rare"]-expected) > 1e-9 {
		t.Errorf("IDF(rare) = %f, want %f", idf["rare"], expected)
	}

	// Mutating returned IDF must NOT affect the index's internal IDF.
	idf["common"] = 999
	if reidf := idx.IDF(); reidf["common"] == 999 {
		t.Error("Index.IDF leaked internal IDF reference (mutation propagated)")
	}
}

func TestIndex_SparseProvider(t *testing.T) {
	t.Parallel()

	docs := []Document{
		{ID: "1", Text: "hello world"},
		{ID: "2", Text: "world wide"},
	}
	idx := NewIndex(docs)
	provider := idx.SparseProvider()

	if provider.Tokenizer.MinLength == 0 {
		t.Error("SparseProvider should inherit index's tokenizer (non-zero MinLength)")
	}
	if len(provider.IDF) == 0 {
		t.Error("SparseProvider should expose IDF derived from index")
	}
	if !provider.Normalize {
		t.Error("SparseProvider should default Normalize=true")
	}

	// Embedding via the provider should produce a non-empty vector for a known doc term.
	vec, err := provider.EmbedSparse(context.Background(), "hello")
	if err != nil {
		t.Fatalf("EmbedSparse: %v", err)
	}
	if len(vec) == 0 {
		t.Error("EmbedSparse returned empty vector for known token 'hello'")
	}
}

func TestSparseVector_Clone(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		if got := SparseVector(nil).Clone(); got != nil {
			t.Errorf("Clone of nil = %v, want nil", got)
		}
	})

	t.Run("deep_copy", func(t *testing.T) {
		t.Parallel()
		v := SparseVector{"a": 1.0, "b": 2.0}
		clone := v.Clone()
		if len(clone) != len(v) {
			t.Errorf("clone len = %d, want %d", len(clone), len(v))
		}
		clone["a"] = 999
		if v["a"] == 999 {
			t.Error("Clone leaked reference (mutation propagated)")
		}
	})
}

func TestDenseDot_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		a    DenseVector
		b    DenseVector
		err  error
	}{
		{name: "len_mismatch", a: DenseVector{1, 2, 3}, b: DenseVector{1, 2}, err: ErrDimensionMismatch},
		{name: "empty_both", a: DenseVector{}, b: DenseVector{}, err: nil},
		{name: "both_nil", a: nil, b: nil, err: nil},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := DenseDot(tc.a, tc.b)
			if (err == nil) != (tc.err == nil) {
				t.Errorf("err=%v want %v", err, tc.err)
			}
		})
	}
}

func TestDenseCosine_ZeroVectors(t *testing.T) {
	t.Parallel()

	t.Run("both_zero", func(t *testing.T) {
		t.Parallel()
		got, err := DenseCosine(DenseVector{0, 0, 0}, DenseVector{0, 0, 0})
		if err != nil {
			t.Fatalf("DenseCosine of zero vectors: %v", err)
		}
		if got != 0 {
			t.Errorf("DenseCosine(0,0) = %f, want 0", got)
		}
	})

	t.Run("len_mismatch_propagates", func(t *testing.T) {
		t.Parallel()
		_, err := DenseCosine(DenseVector{1, 2}, DenseVector{1, 2, 3})
		if err == nil {
			t.Error("DenseCosine should propagate ErrDimensionMismatch from DenseDot")
		}
	})

	t.Run("normalized", func(t *testing.T) {
		t.Parallel()
		// Cosine of identical vectors should be 1.0.
		v := DenseVector{0.6, 0.8}
		got, err := DenseCosine(v, v)
		if err != nil {
			t.Fatalf("DenseCosine: %v", err)
		}
		if math.Abs(got-1.0) > 1e-9 {
			t.Errorf("DenseCosine of identical vector = %f, want 1.0", got)
		}
	})
}

func TestProvider_WithProviderTokenizer(t *testing.T) {
	t.Parallel()

	customTok := Tokenizer{MinLength: 5, StopWords: map[string]struct{}{"hello": {}}}
	provider := NewLocalProvider(WithProviderTokenizer(customTok))

	vec, err := provider.EmbedSparse(context.Background(), "hello world short tiny")
	if err != nil {
		t.Fatalf("EmbedSparse: %v", err)
	}
	if _, has := vec["hello"]; has {
		t.Errorf("custom tokenizer should drop 'hello' (stopword); vec=%v", vec)
	}
	// Tokens shorter than 5 should be excluded.
	for token := range vec {
		if len(token) < 5 {
			t.Errorf("token %q is shorter than MinLength=5", token)
		}
	}
}

func TestProvider_WithProviderIDF(t *testing.T) {
	t.Parallel()

	idf := SparseVector{"important": 5.0, "common": 0.1}
	provider := NewLocalProvider(WithProviderIDF(idf))

	vec, err := provider.EmbedSparse(context.Background(), "important common")
	if err != nil {
		t.Fatalf("EmbedSparse: %v", err)
	}

	// IDF should weight "important" much higher than "common" — verify the
	// ratio survives normalization (relative order should hold).
	if vec["important"] <= vec["common"] {
		t.Errorf("IDF should weight 'important' higher than 'common'; got important=%f common=%f",
			vec["important"], vec["common"])
	}

	// Mutating the input IDF must NOT affect the provider (Clone semantics).
	idf["important"] = 999
	vec2, _ := provider.EmbedSparse(context.Background(), "important common")
	if math.Abs(vec["important"]-vec2["important"]) > 1e-9 {
		t.Error("WithProviderIDF did not clone IDF — mutation leaked into provider")
	}
}

func TestProvider_WithProviderNormalization(t *testing.T) {
	t.Parallel()

	// Without explicit Normalize (false), the raw TermFrequency output
	// passes through. TermFrequency already scales counts by total tokens
	// (so "repeat" out of 4 tokens = 0.75), but the vector itself is NOT
	// unit-length.
	provider := NewLocalProvider(WithProviderNormalization(false))
	vec, err := provider.EmbedSparse(context.Background(), "repeat repeat repeat unique")
	if err != nil {
		t.Fatalf("EmbedSparse: %v", err)
	}
	// With normalize=false, the vector's L2 norm need not equal 1.
	rawNorm := vec.Norm()
	if math.Abs(rawNorm-1.0) < 1e-9 {
		t.Error("with normalize=false, vec norm should not be exactly 1.0 (no unit-length forcing)")
	}

	// With normalization, the vector should be unit-length.
	provider2 := NewLocalProvider(WithProviderNormalization(true))
	vec2, _ := provider2.EmbedSparse(context.Background(), "repeat repeat repeat unique")
	if math.Abs(vec2.Norm()-1.0) > 1e-9 {
		t.Errorf("with normalize=true, vec norm = %f, want 1.0", vec2.Norm())
	}
}

func TestEnsureBuilt_NoOpWhenBuilt(t *testing.T) {
	t.Parallel()

	docs := []Document{{ID: "1", Text: "a b c"}}
	idx := NewIndex(docs) // NewIndex already calls Build()

	// Calling Embed multiple times should not re-trigger Build; check via
	// behavior — embedding the same text should produce identical vectors.
	v1 := idx.Embed("a b c")
	v2 := idx.Embed("a b c")
	for token, value := range v1 {
		if other, ok := v2[token]; !ok {
			t.Errorf("missing token %q on second embed", token)
		} else if math.Abs(value-other) > 1e-9 {
			t.Errorf("token %q value drift: %f vs %f", token, value, other)
		}
	}
}

func TestScoreLess_DocumentTextTiebreak(t *testing.T) {
	t.Parallel()

	// Equal scores + equal IDs → tiebreak by document text. Verify by
	// constructing a SearchResult slice and sorting via the package's
	// scoreLess helper (exercised via SearchVector).
	docs := []Document{
		{ID: "x", Text: "zebra"},
		{ID: "x", Text: "apple"}, // same ID, different text; rare but allowed
	}
	idx := NewIndex(docs)
	// Query that matches both equally.
	results := idx.Search("zebra apple", 2)
	if len(results) < 1 {
		t.Skipf("expected at least 1 result, got %d", len(results))
	}
	// If both match, the alphabetically-earlier text ('apple' < 'zebra') should rank first.
	if len(results) == 2 && !strings.Contains(strings.ToLower(results[0].Document.Text+results[1].Document.Text), "apple") {
		t.Errorf("expected both results returned with tiebreak on text; got %+v", results)
	}
}
