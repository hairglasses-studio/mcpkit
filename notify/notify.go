package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"
)

// Urgency represents the priority level of a notification.
type Urgency string

const (
	// UrgencyLow represents a low-priority notification.
	UrgencyLow Urgency = "low"
	// UrgencyNormal represents a normal-priority notification.
	UrgencyNormal Urgency = "normal"
	// UrgencyCritical represents a critical-priority notification.
	UrgencyCritical Urgency = "critical"
)

// Valid returns true if the urgency is one of the defined constants.
func (u Urgency) Valid() bool {
	switch u {
	case UrgencyLow, UrgencyNormal, UrgencyCritical:
		return true
	}
	return false
}

// Notification holds the data for a single notification.
type Notification struct {
	// Title is the notification headline.
	Title string `json:"title"`
	// Body is the notification detail text.
	Body string `json:"body"`
	// Urgency is the priority level (low, normal, critical).
	Urgency Urgency `json:"urgency,omitempty"`
	// Channel is an optional routing key (e.g., "ops", "ci").
	Channel string `json:"channel,omitempty"`
}

// Validate checks that required fields are populated and urgency is valid.
func (n Notification) Validate() error {
	if n.Title == "" {
		return errors.New("notify: title is required")
	}
	if n.Urgency != "" && !n.Urgency.Valid() {
		return fmt.Errorf("notify: invalid urgency %q", n.Urgency)
	}
	return nil
}

// Notifier sends notifications through a specific backend.
type Notifier interface {
	// Notify sends a notification. Implementations must be safe for
	// concurrent use.
	Notify(ctx context.Context, msg Notification) error
}

// DesktopNotifier sends notifications using notify-send (Linux).
type DesktopNotifier struct {
	// ExecCommand is the command runner for testability. If nil, exec.CommandContext is used.
	ExecCommand func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewDesktopNotifier returns a DesktopNotifier that shells out to notify-send.
func NewDesktopNotifier() *DesktopNotifier {
	return &DesktopNotifier{}
}

// Notify sends a desktop notification via notify-send.
func (d *DesktopNotifier) Notify(ctx context.Context, msg Notification) error {
	if err := msg.Validate(); err != nil {
		return err
	}

	urgency := string(msg.Urgency)
	if urgency == "" {
		urgency = string(UrgencyNormal)
	}

	args := []string{"-u", urgency}
	if msg.Channel != "" {
		args = append(args, "-c", msg.Channel)
	}
	args = append(args, msg.Title)
	if msg.Body != "" {
		args = append(args, msg.Body)
	}

	cmdFunc := d.ExecCommand
	if cmdFunc == nil {
		cmdFunc = exec.CommandContext
	}

	cmd := cmdFunc(ctx, "notify-send", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("notify-send: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// WebhookNotifier sends notifications as HTTP POST requests with a JSON body.
type WebhookNotifier struct {
	// URL is the webhook endpoint.
	URL string
	// Client is the HTTP client. If nil, http.DefaultClient is used.
	Client *http.Client
	// Headers are extra HTTP headers added to every request.
	Headers map[string]string
}

// NewWebhookNotifier returns a WebhookNotifier targeting the given URL.
func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{URL: url}
}

// Notify sends a JSON-encoded notification to the webhook URL.
func (w *WebhookNotifier) Notify(ctx context.Context, msg Notification) error {
	if err := msg.Validate(); err != nil {
		return err
	}
	if w.URL == "" {
		return errors.New("notify: webhook URL is required")
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("notify: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}

	client := w.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("notify: webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notify: webhook returned %d", resp.StatusCode)
	}
	return nil
}

// MultiNotifier fans out notifications to multiple notifiers. If any notifier
// returns an error, all errors are collected and returned as a combined error.
type MultiNotifier struct {
	mu        sync.RWMutex
	notifiers []Notifier
}

// NewMultiNotifier returns a MultiNotifier that dispatches to all given notifiers.
func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: notifiers}
}

// Add appends a notifier to the fan-out list.
func (m *MultiNotifier) Add(n Notifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifiers = append(m.notifiers, n)
}

// Notify sends the notification to all registered notifiers concurrently.
// All notifiers are called even if some fail; errors are collected and
// returned as a single combined error.
func (m *MultiNotifier) Notify(ctx context.Context, msg Notification) error {
	if err := msg.Validate(); err != nil {
		return err
	}

	m.mu.RLock()
	notifiers := make([]Notifier, len(m.notifiers))
	copy(notifiers, m.notifiers)
	m.mu.RUnlock()

	if len(notifiers) == 0 {
		return nil
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for _, n := range notifiers {
		wg.Add(1)
		go func(notifier Notifier) {
			defer wg.Done()
			if err := notifier.Notify(ctx, msg); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(n)
	}

	wg.Wait()
	return errors.Join(errs...)
}
