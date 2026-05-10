// Package embedding provides dependency-light primitives for local semantic
// search and embedding adapters.
//
// The package intentionally keeps the default path sparse and deterministic:
// text is tokenized locally, represented as [SparseVector] values, ranked with
// cosine similarity, and optionally weighted with TF-IDF through [Index].
// Dense-model integrations can depend on [DenseProvider] without adding model
// dependencies to mcpkit itself.
//
// Downstream roadmap consolidation work should migrate local token-bag, TF-IDF,
// prompt-cache, GraphRAG scaffold, and evidence-search code to these primitives
// before adding repo-specific persistence or model-provider behavior.
package embedding
