package feedback

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestModuleSubmitWritesMemorySink(t *testing.T) {
	t.Parallel()

	sink := NewMemorySink()
	now := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	mod := NewModule(sink,
		WithSource("unit-test"),
		WithClock(func() time.Time { return now }),
		WithIDGenerator(func() string { return "feedback_1" }),
	)

	out, err := mod.submit(context.Background(), SubmitInput{
		Category:     " bug ",
		Message:      " Something broke ",
		Severity:     SeverityHigh,
		Rating:       2,
		Tags:         []string{" ui ", "", "regression"},
		Context:      map[string]string{" tool ": " feedback_submit ", "empty": ""},
		AllowContact: true,
		Contact:      " user@example.com ",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if out.ID != "feedback_1" || out.Status != "stored" || !out.Stored {
		t.Fatalf("unexpected output: %+v", out)
	}

	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("records len = %d, want 1", len(records))
	}
	record := records[0]
	if record.Category != "bug" || record.Message != "Something broke" {
		t.Fatalf("record was not normalized: %+v", record)
	}
	if record.Source != "unit-test" || !record.CreatedAt.Equal(now) {
		t.Fatalf("unexpected source/time: %+v", record)
	}
	if got := strings.Join(record.Tags, ","); got != "ui,regression" {
		t.Fatalf("tags = %q", got)
	}
	if record.Context["tool"] != "feedback_submit" {
		t.Fatalf("context = %+v", record.Context)
	}
}

func TestModuleSubmitDefaultsSeverity(t *testing.T) {
	t.Parallel()

	sink := NewMemorySink()
	mod := NewModule(sink, WithIDGenerator(func() string { return "feedback_2" }))
	if _, err := mod.submit(context.Background(), SubmitInput{Category: "docs", Message: "Useful"}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := sink.Records()[0].Severity; got != SeverityInfo {
		t.Fatalf("severity = %q, want %q", got, SeverityInfo)
	}
}

func TestModuleSubmitRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input SubmitInput
		want  string
	}{
		{
			name:  "missing category",
			input: SubmitInput{Message: "message"},
			want:  "category is required",
		},
		{
			name:  "missing message",
			input: SubmitInput{Category: "bug"},
			want:  "message is required",
		},
		{
			name:  "bad rating",
			input: SubmitInput{Category: "bug", Message: "msg", Rating: 6},
			want:  "rating must be between 1 and 5",
		},
		{
			name:  "bad severity",
			input: SubmitInput{Category: "bug", Message: "msg", Severity: "urgent"},
			want:  "severity must be one of",
		},
		{
			name:  "contact without consent",
			input: SubmitInput{Category: "bug", Message: "msg", Contact: "me@example.com"},
			want:  "contact requires",
		},
		{
			name:  "message too long",
			input: SubmitInput{Category: "bug", Message: "123456"},
			want:  "message exceeds",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mod := NewModule(NewMemorySink(), WithMaxMessageChars(5))
			_, err := mod.submit(context.Background(), tc.input)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestJSONLSinkWritesRecords(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "feedback", "records.jsonl")
	sink, err := NewJSONLSink(path)
	if err != nil {
		t.Fatalf("NewJSONLSink: %v", err)
	}

	record := Record{
		ID:        "feedback_jsonl",
		Category:  "docs",
		Message:   "Clear tutorial",
		Severity:  SeverityLow,
		CreatedAt: time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
	}
	if err := sink.WriteFeedback(context.Background(), record); err != nil {
		t.Fatalf("WriteFeedback: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatal("expected one JSONL row")
	}
	var got Record
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal row: %v", err)
	}
	if got.ID != record.ID || got.Message != record.Message {
		t.Fatalf("record = %+v, want %+v", got, record)
	}
	if scanner.Scan() {
		t.Fatal("expected only one JSONL row")
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
}

func TestToolDefinitionMetadata(t *testing.T) {
	t.Parallel()

	mod := NewModule(NewMemorySink())
	tools := mod.Tools()
	if len(tools) != 1 {
		t.Fatalf("tools len = %d, want 1", len(tools))
	}
	tool := tools[0]
	if tool.Tool.Name != ToolName {
		t.Fatalf("tool name = %q, want %q", tool.Tool.Name, ToolName)
	}
	if tool.Category != "feedback" || !tool.IsWrite {
		t.Fatalf("unexpected metadata: %+v", tool)
	}
}

func TestNewJSONLSinkRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	if _, err := NewJSONLSink(""); err == nil {
		t.Fatal("expected empty path error")
	}
}
