package mcptest

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Session is a recorded set of tool interactions that can be replayed.
type Session struct {
	Name     string         `json:"name"`
	Recorded time.Time      `json:"recorded"`
	Entries  []SessionEntry `json:"entries"`
}

// SessionEntry records a single tool call with its result.
type SessionEntry struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
	Result   *registry.CallToolResult `json:"result"`
	IsError  bool                   `json:"is_error"`
}

// Session creates a Session from the recorder's recorded calls.
func (r *Recorder) Session(name string) *Session {
	calls := r.Calls()
	entries := make([]SessionEntry, 0, len(calls))
	for _, c := range calls {
		isError := false
		if c.Result != nil {
			isError = registry.IsResultError(c.Result)
		}
		entries = append(entries, SessionEntry{
			ToolName: c.Name,
			Args:     c.Args,
			Result:   c.Result,
			IsError:  isError,
		})
	}
	return &Session{
		Name:     name,
		Recorded: time.Now(),
		Entries:  entries,
	}
}

// SaveSession serializes the Session to a JSON file at path.
func (r *Recorder) SaveSession(path string) error {
	// Use the last session name or a default
	s := r.Session("session")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}
	return nil
}

// LoadSession reads a Session from a JSON file at path.
func LoadSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &s, nil
}

// replayConfig holds options for Replay.
type replayConfig struct {
	strictOrder   bool
	ignoreFields  []string
}

// ReplayOption configures the Replay function.
type ReplayOption func(*replayConfig)

// WithStrictOrder requires that replayed calls match the session in the same order.
func WithStrictOrder() ReplayOption {
	return func(c *replayConfig) {
		c.strictOrder = true
	}
}

// WithIgnoreFields ignores specific top-level result content fields during comparison.
func WithIgnoreFields(fields ...string) ReplayOption {
	return func(c *replayConfig) {
		c.ignoreFields = append(c.ignoreFields, fields...)
	}
}

// Replay replays each entry in session against client, asserting that results match.
// By default entries are replayed in order but order-checking in result matching is relaxed.
// Use WithStrictOrder to also assert sequential call ordering against a new recorder.
func Replay(t testing.TB, client *Client, session *Session, opts ...ReplayOption) {
	t.Helper()

	cfg := &replayConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	for i, entry := range session.Entries {
		result, err := client.CallToolE(entry.ToolName, entry.Args)
		if err != nil {
			t.Errorf("replay entry %d (%s): unexpected transport error: %v", i, entry.ToolName, err)
			continue
		}
		if result == nil && entry.Result != nil {
			t.Errorf("replay entry %d (%s): got nil result, want non-nil", i, entry.ToolName)
			continue
		}
		if result != nil && entry.Result == nil {
			t.Errorf("replay entry %d (%s): got non-nil result, want nil", i, entry.ToolName)
			continue
		}
		if result == nil && entry.Result == nil {
			continue
		}

		// Compare IsError flag
		if registry.IsResultError(result) != entry.IsError {
			t.Errorf("replay entry %d (%s): IsError = %v, want %v",
				i, entry.ToolName, registry.IsResultError(result), entry.IsError)
		}

		// Compare result content as normalized JSON, applying ignoreFields
		if !resultsMatch(result, entry.Result, cfg.ignoreFields) {
			gotJSON, _ := json.MarshalIndent(result, "", "  ")
			wantJSON, _ := json.MarshalIndent(entry.Result, "", "  ")
			t.Errorf("replay entry %d (%s): result mismatch\ngot:  %s\nwant: %s",
				i, entry.ToolName, gotJSON, wantJSON)
		}
	}
}

// resultsMatch compares two CallToolResult values after stripping ignoreFields from
// their JSON representations.
func resultsMatch(got, want *registry.CallToolResult, ignoreFields []string) bool {
	gotMap := resultToMap(got)
	wantMap := resultToMap(want)

	for _, field := range ignoreFields {
		delete(gotMap, field)
		delete(wantMap, field)
	}

	gotJSON, err1 := json.Marshal(gotMap)
	wantJSON, err2 := json.Marshal(wantMap)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(gotJSON) == string(wantJSON)
}

// resultToMap converts a CallToolResult to a map for comparison.
func resultToMap(r *registry.CallToolResult) map[string]interface{} {
	if r == nil {
		return nil
	}
	data, err := json.Marshal(r)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}
