package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Token-efficient result patterns for large-data MCP tools.
//
// Motivation: MCP tools that return query results, search hits, or log
// streams can easily blow out a caller's context window. These helpers
// implement the dbhub/bytebase pattern (https://github.com/bytebase/dbhub):
// return schema or summary first, paginate data, and truncate aggressively
// when output exceeds a byte budget.
//
// The three helpers compose — a typical large-result handler calls
// Paginate to bound row count, then wraps JSON output with TruncateResult
// to guard against oversized rows.

// PageCursor is an opaque cursor threaded through paginated responses.
// Implementations are encouraged to embed it in response JSON as
// `next_cursor` (empty string means no more pages).
type PageCursor string

// String returns the underlying cursor value.
func (c PageCursor) String() string { return string(c) }

// Empty reports whether the cursor has no value.
func (c PageCursor) Empty() bool { return c == "" }

// EncodeOffsetCursor packs an integer offset into an opaque base64 cursor.
// Use this when your backend supports LIMIT/OFFSET pagination.
func EncodeOffsetCursor(offset int) PageCursor {
	raw := fmt.Sprintf("o:%d", offset)
	return PageCursor(base64.RawURLEncoding.EncodeToString([]byte(raw)))
}

// DecodeOffsetCursor unpacks a cursor produced by EncodeOffsetCursor.
// Returns an error if the cursor is malformed or references a different
// pagination scheme.
func DecodeOffsetCursor(c PageCursor) (int, error) {
	if c == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(string(c))
	if err != nil {
		return 0, fmt.Errorf("cursor decode: %w", err)
	}
	var offset int
	if _, err := fmt.Sscanf(string(raw), "o:%d", &offset); err != nil {
		return 0, fmt.Errorf("cursor parse: %w", err)
	}
	if offset < 0 {
		return 0, fmt.Errorf("cursor offset negative: %d", offset)
	}
	return offset, nil
}

// Page is the paginated slice of a larger result set.
type Page[T any] struct {
	Items      []T        `json:"items"`
	NextCursor PageCursor `json:"next_cursor,omitempty"`
	Total      int        `json:"total,omitempty"`
	Truncated  bool       `json:"truncated,omitempty"`
}

// Paginate returns the subslice of items starting at cursor, capped at
// limit. NextCursor is populated when more items remain.
//
// Pattern: handlers call Paginate on their full in-memory result set to
// bound the response size, then return the Page as the structured output.
// Callers pass back the NextCursor on subsequent calls to fetch more.
//
// If limit <= 0, a conservative default of 50 is used. Items are never
// mutated.
func Paginate[T any](items []T, cursor PageCursor, limit int) Page[T] {
	if limit <= 0 {
		limit = 50
	}
	offset, err := DecodeOffsetCursor(cursor)
	if err != nil || offset > len(items) {
		offset = 0
	}

	end := offset + limit
	if end > len(items) {
		end = len(items)
	}

	page := Page[T]{
		Items: items[offset:end],
		Total: len(items),
	}
	if end < len(items) {
		page.NextCursor = EncodeOffsetCursor(end)
	}
	return page
}

// TruncateResult bounds the JSON serialization of a CallToolResult to
// maxBytes. If the rendered text exceeds the budget, text content is
// truncated with a clear suffix and a `truncated: true` marker.
//
// maxBytes <= 0 is treated as unlimited (no-op). The original result is
// returned unmodified when under budget.
//
// This is a defensive layer: even with Paginate bounding row count, a
// single oversized row can blow out context. Apply TruncateResult at
// handler exit as a hard ceiling.
func TruncateResult(result *registry.CallToolResult, maxBytes int) *registry.CallToolResult {
	if result == nil || maxBytes <= 0 {
		return result
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) <= maxBytes {
		return result
	}

	// Budget exceeded — emit a truncation summary result. The body text
	// is replaced with a short notice and the caller is expected to
	// paginate or filter.
	overflow := len(encoded) - maxBytes
	msg := fmt.Sprintf(
		"[RESULT_TRUNCATED] response would be %d bytes (budget %d). "+
			"Reduce result size with a pagination cursor, a more specific query, "+
			"or by requesting only the fields you need. Overflow: %d bytes.",
		len(encoded), maxBytes, overflow,
	)
	return registry.MakeTextResult(msg)
}

// SchemaFirstResult returns the given schema when the caller asks for it
// (schemaOnly=true) or full data otherwise. This matches the dbhub pattern
// where large-data tools document their shape before returning rows — LLMs
// can consult the schema to build a better follow-up query instead of
// burning tokens on unstructured data.
//
// Handlers typically expose a `schema_only: bool` input; pass it here
// along with the schema doc and a deferred data-producer closure. The
// producer is only invoked when full data is requested.
func SchemaFirstResult(schemaOnly bool, schema any, produceData func() (any, error)) *registry.CallToolResult {
	if schemaOnly {
		return JSONResult(map[string]any{
			"schema": schema,
			"hint":   "re-call with schema_only=false (and any filters) to fetch data",
		})
	}
	data, err := produceData()
	if err != nil {
		return ErrorResult(err)
	}
	return JSONResult(data)
}
