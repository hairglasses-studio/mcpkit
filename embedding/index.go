package embedding

import (
	"math"
	"sort"
)

// Document is a searchable text record.
type Document struct {
	ID       string
	Text     string
	Metadata map[string]string
}

// SearchResult is a document with its similarity score.
type SearchResult struct {
	Document Document
	Score    float64
}

// Index stores TF-IDF weighted sparse vectors for a document corpus.
type Index struct {
	tokenizer Tokenizer
	docs      []Document
	vectors   []SparseVector
	idf       SparseVector
	built     bool
}

// IndexOption customizes a new Index.
type IndexOption func(*Index)

// WithTokenizer sets the tokenizer used for documents and queries.
func WithTokenizer(tokenizer Tokenizer) IndexOption {
	return func(index *Index) {
		index.tokenizer = tokenizer
	}
}

// NewIndex creates and builds an index for docs.
func NewIndex(docs []Document, opts ...IndexOption) *Index {
	index := NewEmptyIndex(opts...)
	for _, doc := range docs {
		index.Add(doc)
	}
	index.Build()
	return index
}

// NewEmptyIndex creates an index ready for documents to be added.
func NewEmptyIndex(opts ...IndexOption) *Index {
	index := &Index{tokenizer: DefaultTokenizer()}
	for _, opt := range opts {
		opt(index)
	}
	return index
}

// Add appends a document and marks the index for rebuild.
func (index *Index) Add(doc Document) {
	index.docs = append(index.docs, cloneDocument(doc))
	index.built = false
}

// Len returns the number of indexed documents.
func (index *Index) Len() int {
	return len(index.docs)
}

// Documents returns a copy of the indexed document list.
func (index *Index) Documents() []Document {
	docs := make([]Document, len(index.docs))
	for i, doc := range index.docs {
		docs[i] = cloneDocument(doc)
	}
	return docs
}

// IDF returns the index inverse-document-frequency weights.
func (index *Index) IDF() SparseVector {
	index.ensureBuilt()
	return index.idf.Clone()
}

// Build rebuilds TF-IDF vectors for the current document set.
func (index *Index) Build() {
	docTokens := make([][]string, len(index.docs))
	df := make(map[string]int)
	for i, doc := range index.docs {
		tokens := index.tokenizer.Tokenize(doc.Text)
		docTokens[i] = tokens
		seen := make(map[string]struct{})
		for _, token := range tokens {
			seen[token] = struct{}{}
		}
		for token := range seen {
			df[token]++
		}
	}

	index.idf = make(SparseVector, len(df))
	totalDocs := float64(len(index.docs))
	for token, count := range df {
		index.idf[token] = math.Log((1+totalDocs)/(1+float64(count))) + 1
	}

	index.vectors = make([]SparseVector, len(index.docs))
	for i, tokens := range docTokens {
		index.vectors[i] = ApplyIDF(TermFrequency(tokens), index.idf).Normalize()
	}
	index.built = true
}

// Embed returns the sparse TF-IDF query vector used by this index.
func (index *Index) Embed(text string) SparseVector {
	index.ensureBuilt()
	return ApplyIDF(TermFrequency(index.tokenizer.Tokenize(text)), index.idf).Normalize()
}

// Search returns the highest-scoring document matches for query.
func (index *Index) Search(query string, limit int) []SearchResult {
	return index.SearchVector(index.Embed(query), limit)
}

// SearchVector returns the highest-scoring document matches for query.
func (index *Index) SearchVector(query SparseVector, limit int) []SearchResult {
	if limit <= 0 {
		return nil
	}
	index.ensureBuilt()
	matches := make([]SearchResult, 0, len(index.docs))
	query = query.Normalize()
	for i, vector := range index.vectors {
		score := Dot(query, vector)
		if score <= 0 {
			continue
		}
		matches = append(matches, SearchResult{
			Document: cloneDocument(index.docs[i]),
			Score:    score,
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		if matches[i].Document.ID != matches[j].Document.ID {
			return matches[i].Document.ID < matches[j].Document.ID
		}
		return matches[i].Document.Text < matches[j].Document.Text
	})
	if limit > len(matches) {
		limit = len(matches)
	}
	return matches[:limit]
}

// SparseProvider returns a local provider using this index's tokenizer and IDF.
func (index *Index) SparseProvider() LocalProvider {
	index.ensureBuilt()
	return LocalProvider{
		Tokenizer: index.tokenizer,
		IDF:       index.idf.Clone(),
		Normalize: true,
	}
}

func (index *Index) ensureBuilt() {
	if !index.built {
		index.Build()
	}
}

func cloneDocument(doc Document) Document {
	clone := doc
	if doc.Metadata != nil {
		clone.Metadata = make(map[string]string, len(doc.Metadata))
		for key, value := range doc.Metadata {
			clone.Metadata[key] = value
		}
	}
	return clone
}
