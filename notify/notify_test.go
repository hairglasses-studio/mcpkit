package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestUrgency_Valid(t *testing.T) {
	tests := []struct {
		u    Urgency
		want bool
	}{
		{UrgencyLow, true},
		{UrgencyNormal, true},
		{UrgencyCritical, true},
		{"", false},
		{"medium", false},
	}
	for _, tt := range tests {
		if got := tt.u.Valid(); got != tt.want {
			t.Errorf("Urgency(%q).Valid() = %v, want %v", tt.u, got, tt.want)
		}
	}
}

func TestNotification_Validate(t *testing.T) {
	tests := []struct {
		name    string
		n       Notification
		wantErr bool
	}{
		{"valid full", Notification{Title: "T", Body: "B", Urgency: UrgencyLow}, false},
		{"valid no urgency", Notification{Title: "T"}, false},
		{"missing title", Notification{Body: "B"}, true},
		{"invalid urgency", Notification{Title: "T", Urgency: "extreme"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.n.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDesktopNotifier_Notify(t *testing.T) {
	var capturedArgs []string
	d := &DesktopNotifier{
		ExecCommand: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			capturedArgs = append([]string{name}, args...)
			return exec.CommandContext(ctx, "true")
		},
	}

	err := d.Notify(context.Background(), Notification{
		Title:   "Test",
		Body:    "Body text",
		Urgency: UrgencyCritical,
		Channel: "ci",
	})
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	want := []string{"notify-send", "-u", "critical", "-c", "ci", "Test", "Body text"}
	if len(capturedArgs) != len(want) {
		t.Fatalf("args = %v, want %v", capturedArgs, want)
	}
	for i := range want {
		if capturedArgs[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q", i, capturedArgs[i], want[i])
		}
	}
}

func TestDesktopNotifier_DefaultUrgency(t *testing.T) {
	var capturedArgs []string
	d := &DesktopNotifier{
		ExecCommand: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			capturedArgs = append([]string{name}, args...)
			return exec.CommandContext(ctx, "true")
		},
	}

	err := d.Notify(context.Background(), Notification{Title: "T"})
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	// Should default to normal urgency
	if len(capturedArgs) < 3 || capturedArgs[2] != "normal" {
		t.Errorf("expected default urgency 'normal', got args: %v", capturedArgs)
	}
}

func TestDesktopNotifier_ValidationError(t *testing.T) {
	d := NewDesktopNotifier()
	err := d.Notify(context.Background(), Notification{})
	if err == nil {
		t.Fatal("expected validation error for empty title")
	}
}

func TestDesktopNotifier_CommandFailure(t *testing.T) {
	d := &DesktopNotifier{
		ExecCommand: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "false")
		},
	}

	err := d.Notify(context.Background(), Notification{Title: "T"})
	if err == nil {
		t.Fatal("expected error from failed command")
	}
	if !strings.Contains(err.Error(), "notify-send") {
		t.Errorf("error should mention notify-send, got: %v", err)
	}
}

func TestWebhookNotifier_Notify(t *testing.T) {
	var received Notification
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", ct)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer token123" {
			t.Errorf("Authorization = %q, want 'Bearer token123'", auth)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &received); err != nil {
			t.Errorf("unmarshal: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := &WebhookNotifier{
		URL:     srv.URL,
		Client:  srv.Client(),
		Headers: map[string]string{"Authorization": "Bearer token123"},
	}

	msg := Notification{Title: "Alert", Body: "disk full", Urgency: UrgencyCritical}
	err := wh.Notify(context.Background(), msg)
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if received.Title != "Alert" {
		t.Errorf("received title = %q, want 'Alert'", received.Title)
	}
	if received.Urgency != UrgencyCritical {
		t.Errorf("received urgency = %q, want critical", received.Urgency)
	}
}

func TestWebhookNotifier_EmptyURL(t *testing.T) {
	wh := NewWebhookNotifier("")
	err := wh.Notify(context.Background(), Notification{Title: "T"})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestWebhookNotifier_ValidationError(t *testing.T) {
	wh := NewWebhookNotifier("http://example.com")
	err := wh.Notify(context.Background(), Notification{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestWebhookNotifier_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	wh := &WebhookNotifier{URL: srv.URL, Client: srv.Client()}
	err := wh.Notify(context.Background(), Notification{Title: "T"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

func TestWebhookNotifier_NetworkError(t *testing.T) {
	wh := NewWebhookNotifier("http://192.0.2.1:1/invalid")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	err := wh.Notify(ctx, Notification{Title: "T"})
	if err == nil {
		t.Fatal("expected network error")
	}
}

type failingNotifier struct {
	err error
}

func (f *failingNotifier) Notify(_ context.Context, _ Notification) error {
	return f.err
}

type countingNotifier struct {
	count atomic.Int32
}

func (c *countingNotifier) Notify(_ context.Context, _ Notification) error {
	c.count.Add(1)
	return nil
}

func TestMultiNotifier_Notify(t *testing.T) {
	c1 := &countingNotifier{}
	c2 := &countingNotifier{}
	multi := NewMultiNotifier(c1, c2)

	err := multi.Notify(context.Background(), Notification{Title: "T"})
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if c1.count.Load() != 1 || c2.count.Load() != 1 {
		t.Errorf("counts = %d, %d; want 1, 1", c1.count.Load(), c2.count.Load())
	}
}

func TestMultiNotifier_Empty(t *testing.T) {
	multi := NewMultiNotifier()
	err := multi.Notify(context.Background(), Notification{Title: "T"})
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
}

func TestMultiNotifier_PartialFailure(t *testing.T) {
	c := &countingNotifier{}
	f := &failingNotifier{err: errors.New("boom")}
	multi := NewMultiNotifier(c, f)

	err := multi.Notify(context.Background(), Notification{Title: "T"})
	if err == nil {
		t.Fatal("expected error from failing notifier")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should contain 'boom', got: %v", err)
	}
	// The counting notifier should still have been called
	if c.count.Load() != 1 {
		t.Errorf("counting notifier should have been called, count = %d", c.count.Load())
	}
}

func TestMultiNotifier_AllFail(t *testing.T) {
	f1 := &failingNotifier{err: errors.New("err1")}
	f2 := &failingNotifier{err: errors.New("err2")}
	multi := NewMultiNotifier(f1, f2)

	err := multi.Notify(context.Background(), Notification{Title: "T"})
	if err == nil {
		t.Fatal("expected error when all fail")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "err1") || !strings.Contains(errStr, "err2") {
		t.Errorf("error should contain both errors, got: %v", err)
	}
}

func TestMultiNotifier_ValidationError(t *testing.T) {
	c := &countingNotifier{}
	multi := NewMultiNotifier(c)
	err := multi.Notify(context.Background(), Notification{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	// Should not have dispatched
	if c.count.Load() != 0 {
		t.Error("should not dispatch on validation error")
	}
}

func TestMultiNotifier_Add(t *testing.T) {
	c := &countingNotifier{}
	multi := NewMultiNotifier()
	multi.Add(c)

	err := multi.Notify(context.Background(), Notification{Title: "T"})
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if c.count.Load() != 1 {
		t.Errorf("count = %d, want 1", c.count.Load())
	}
}

func TestMultiNotifier_ConcurrentAdd(t *testing.T) {
	multi := NewMultiNotifier()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			multi.Add(&countingNotifier{})
		}()
	}
	wg.Wait()

	err := multi.Notify(context.Background(), Notification{Title: "T"})
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
}

func TestNewDesktopNotifier(t *testing.T) {
	d := NewDesktopNotifier()
	if d == nil {
		t.Fatal("NewDesktopNotifier() returned nil")
	}
	if d.ExecCommand != nil {
		t.Error("ExecCommand should be nil by default")
	}
}

func TestNewWebhookNotifier(t *testing.T) {
	wh := NewWebhookNotifier("http://example.com")
	if wh == nil {
		t.Fatal("NewWebhookNotifier() returned nil")
	}
	if wh.URL != "http://example.com" {
		t.Errorf("URL = %q, want http://example.com", wh.URL)
	}
}

func TestDesktopNotifier_NoBody(t *testing.T) {
	var capturedArgs []string
	d := &DesktopNotifier{
		ExecCommand: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			capturedArgs = append([]string{name}, args...)
			return exec.CommandContext(ctx, "true")
		},
	}

	err := d.Notify(context.Background(), Notification{Title: "Title Only"})
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	// Should not include body arg
	for _, arg := range capturedArgs {
		if arg == "" {
			t.Error("should not include empty body arg")
		}
	}
	// Last arg should be the title
	if last := capturedArgs[len(capturedArgs)-1]; last != "Title Only" {
		t.Errorf("last arg = %q, want 'Title Only'", last)
	}
}

func ExampleMultiNotifier() {
	desktop := &DesktopNotifier{
		ExecCommand: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "true")
		},
	}
	webhook := NewWebhookNotifier("https://hooks.example.com/alert")

	multi := NewMultiNotifier(desktop, webhook)

	err := multi.Notify(context.Background(), Notification{
		Title:   "Deploy",
		Body:    "v1.2.3 deployed to prod",
		Urgency: UrgencyNormal,
		Channel: "ops",
	})
	// In real usage, handle the error
	_ = err
	fmt.Println("notified")
	// Output: notified
}
