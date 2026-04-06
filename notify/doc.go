// Package notify provides a notification abstraction with pluggable backends.
//
// The [Notifier] interface defines a single Notify method that sends a
// [Notification] with title, body, urgency, and channel metadata.
// Implementations include [DesktopNotifier] (Linux notify-send),
// [WebhookNotifier] (HTTP POST), and [MultiNotifier] (fan-out to multiple
// backends). All implementations are safe for concurrent use.
//
// Example:
//
//	desktop := notify.NewDesktopNotifier()
//	webhook := notify.NewWebhookNotifier("https://hooks.example.com/alert")
//	multi := notify.NewMultiNotifier(desktop, webhook)
//	err := multi.Notify(ctx, notify.Notification{
//	    Title:   "Build Complete",
//	    Body:    "All tests passed",
//	    Urgency: notify.UrgencyNormal,
//	})
package notify
