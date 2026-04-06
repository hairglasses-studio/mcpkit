// Package trigger provides types and a registry for managing event sources
// that initiate agent actions. A [TriggerSource] represents an external event
// producer (webhook, cron schedule, file watcher, etc.) that can start or
// resume agent work.
//
// The [TriggerRecord] captures when and why an agent action was triggered,
// providing full auditability for 12-Factor Agent compliance. The [Registry]
// manages source registration and maintains an append-only audit log of
// trigger events.
//
// Example:
//
//	reg := trigger.NewRegistry()
//	reg.Register(&trigger.StaticSource{
//	    SourceName: "deploy-hook",
//	    SourceType: "webhook",
//	    IsActive:   true,
//	})
//	reg.RecordTrigger(trigger.TriggerRecord{
//	    ID:     "evt-001",
//	    Source: "deploy-hook",
//	    Type:   "webhook",
//	})
package trigger
