//go:build !official_sdk

package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ToolFingerprint is a SHA-256 hash of a tool's identity: name, description,
// and input schema. It is used to detect rug-pull attacks (OWASP MCP03:2025
// Tool Poisoning) where tool definitions are mutated after registration.
type ToolFingerprint [32]byte

// Fingerprint computes a deterministic SHA-256 hash over the tool name,
// description, and JSON-serialised InputSchema of the given ToolDefinition.
// RawInputSchema is included when present so that tools defined via raw JSON
// schema are also covered.
func Fingerprint(td ToolDefinition) ToolFingerprint {
	h := sha256.New()

	// name – NUL-terminated so "ab"+"c" != "a"+"bc"
	_, _ = fmt.Fprintf(h, "%s\x00", td.Tool.Name)

	// description
	_, _ = fmt.Fprintf(h, "%s\x00", td.Tool.Description)

	// structured input schema (JSON-serialised for determinism)
	schemaBytes, err := json.Marshal(td.Tool.InputSchema)
	if err == nil {
		h.Write(schemaBytes)
	}
	h.Write([]byte{0x00})

	// raw input schema when present
	if td.Tool.RawInputSchema != nil {
		h.Write(td.Tool.RawInputSchema)
	}
	h.Write([]byte{0x00})

	var fp ToolFingerprint
	copy(fp[:], h.Sum(nil))
	return fp
}

// Violation records a detected tamper event for a registered tool.
type Violation struct {
	Tool      string
	Timestamp time.Time
	Previous  ToolFingerprint
	Current   ToolFingerprint
}

// IntegrityStore records initial fingerprints for registered tools and detects
// when a tool definition changes after first registration.
type IntegrityStore struct {
	mu         sync.RWMutex
	prints     map[string]ToolFingerprint
	violations []Violation
}

// NewIntegrityStore creates an empty IntegrityStore.
func NewIntegrityStore() *IntegrityStore {
	return &IntegrityStore{
		prints: make(map[string]ToolFingerprint),
	}
}

// Register records the fingerprint of td on first call. If the tool has
// already been registered with a different fingerprint an error is returned
// and the violation is appended to the store's violation list.
// Re-registering with the identical fingerprint is a no-op and returns nil.
func (s *IntegrityStore) Register(td ToolDefinition) error {
	fp := Fingerprint(td)
	name := td.Tool.Name

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, seen := s.prints[name]
	if !seen {
		s.prints[name] = fp
		return nil
	}
	if existing == fp {
		return nil
	}

	v := Violation{
		Tool:      name,
		Timestamp: time.Now(),
		Previous:  existing,
		Current:   fp,
	}
	s.violations = append(s.violations, v)
	return fmt.Errorf("integrity: tool %q fingerprint changed on re-registration", name)
}

// Verify checks whether td's current fingerprint matches the stored one.
// Returns nil when the tool is unknown (not yet registered) or when the
// fingerprint matches. Returns a pointer to a Violation — and appends it to
// the store — when a mismatch is detected.
func (s *IntegrityStore) Verify(td ToolDefinition) *Violation {
	fp := Fingerprint(td)
	name := td.Tool.Name

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, seen := s.prints[name]
	if !seen {
		return nil
	}
	if existing == fp {
		return nil
	}

	v := Violation{
		Tool:      name,
		Timestamp: time.Now(),
		Previous:  existing,
		Current:   fp,
	}
	s.violations = append(s.violations, v)
	return &v
}

// Violations returns a snapshot copy of all violations recorded so far.
func (s *IntegrityStore) Violations() []Violation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.violations) == 0 {
		return nil
	}
	out := make([]Violation, len(s.violations))
	copy(out, s.violations)
	return out
}

// IntegrityMiddleware returns a Middleware that calls store.Verify before
// every tool invocation. If a violation is detected, onViolation is called:
//   - when onViolation returns a non-nil error the tool handler is NOT called
//     and an error result is returned to the caller.
//   - when onViolation returns nil (warning-only mode) execution continues
//     and next is called normally.
func IntegrityMiddleware(store *IntegrityStore, onViolation func(Violation) error) Middleware {
	return func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
			v := store.Verify(td)
			if v != nil {
				if err := onViolation(*v); err != nil {
					return MakeErrorResult(fmt.Sprintf(
						"integrity violation for tool %q: fingerprint mismatch", td.Tool.Name,
					)), nil
				}
			}
			return next(ctx, req)
		}
	}
}
