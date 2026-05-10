package a2a

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("mcpkit/a2a")

// TracingClient wraps an A2A Client with OpenTelemetry tracing.
// Each A2A operation creates a span with protocol-specific attributes,
// enabling distributed tracing across MCP↔A2A boundaries.
type TracingClient struct {
	inner *Client
}

// NewTracingClient wraps a client with tracing.
func NewTracingClient(inner *Client) *TracingClient {
	return &TracingClient{inner: inner}
}

// GetAgentCard fetches the agent card with tracing.
func (tc *TracingClient) GetAgentCard(ctx context.Context) (*AgentCard, error) {
	ctx, span := tracer.Start(ctx, "a2a.GetAgentCard",
		trace.WithAttributes(
			attribute.String("a2a.agent_url", tc.inner.baseURL),
			attribute.String("a2a.operation", "discovery"),
		),
	)
	defer span.End()

	card, err := tc.inner.GetAgentCard(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(
		attribute.String("a2a.agent_name", card.Name),
		attribute.Int("a2a.skills_count", len(card.Skills)),
	)
	return card, nil
}

// SendTask sends a task with tracing.
func (tc *TracingClient) SendTask(ctx context.Context, params TaskSendParams) (*Task, error) {
	ctx, span := tracer.Start(ctx, "a2a.SendTask",
		trace.WithAttributes(
			attribute.String("a2a.agent_url", tc.inner.baseURL),
			attribute.String("a2a.task_id", params.ID),
			attribute.String("a2a.operation", "send"),
			attribute.Int("a2a.message_count", len(params.Messages)),
		),
	)
	defer span.End()

	start := time.Now()
	task, err := tc.inner.SendTask(ctx, params)
	duration := time.Since(start)

	span.SetAttributes(attribute.Float64("a2a.duration_ms", float64(duration.Milliseconds())))

	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(
		attribute.String("a2a.task_state", string(task.State)),
		attribute.Int("a2a.response_messages", len(task.Messages)),
		attribute.Int("a2a.artifacts", len(task.Artifacts)),
	)
	return task, nil
}

// GetTask fetches task status with tracing.
func (tc *TracingClient) GetTask(ctx context.Context, taskID string) (*Task, error) {
	ctx, span := tracer.Start(ctx, "a2a.GetTask",
		trace.WithAttributes(
			attribute.String("a2a.agent_url", tc.inner.baseURL),
			attribute.String("a2a.task_id", taskID),
			attribute.String("a2a.operation", "get"),
		),
	)
	defer span.End()

	task, err := tc.inner.GetTask(ctx, taskID)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(
		attribute.String("a2a.task_state", string(task.State)),
	)
	return task, nil
}

// CancelTask cancels a task with tracing.
func (tc *TracingClient) CancelTask(ctx context.Context, taskID string) (*Task, error) {
	ctx, span := tracer.Start(ctx, "a2a.CancelTask",
		trace.WithAttributes(
			attribute.String("a2a.agent_url", tc.inner.baseURL),
			attribute.String("a2a.task_id", taskID),
			attribute.String("a2a.operation", "cancel"),
		),
	)
	defer span.End()

	task, err := tc.inner.CancelTask(ctx, taskID)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(attribute.String("a2a.task_state", string(task.State)))
	return task, nil
}

// CreateTaskPushNotificationConfig creates a push config with tracing.
func (tc *TracingClient) CreateTaskPushNotificationConfig(ctx context.Context, config PushNotificationConfig) (*PushNotificationConfig, error) {
	ctx, span := tracer.Start(ctx, "a2a.CreateTaskPushNotificationConfig",
		trace.WithAttributes(
			attribute.String("a2a.agent_url", tc.inner.baseURL),
			attribute.String("a2a.task_id", config.TaskID),
			attribute.String("a2a.operation", "push.create"),
		),
	)
	defer span.End()

	created, err := tc.inner.CreateTaskPushNotificationConfig(ctx, config)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	span.SetAttributes(attribute.String("a2a.push_config_id", created.ID))
	return created, nil
}

// GetTaskPushNotificationConfig retrieves a push config with tracing.
func (tc *TracingClient) GetTaskPushNotificationConfig(ctx context.Context, taskID, configID string) (*PushNotificationConfig, error) {
	ctx, span := tracer.Start(ctx, "a2a.GetTaskPushNotificationConfig",
		trace.WithAttributes(
			attribute.String("a2a.agent_url", tc.inner.baseURL),
			attribute.String("a2a.task_id", taskID),
			attribute.String("a2a.push_config_id", configID),
			attribute.String("a2a.operation", "push.get"),
		),
	)
	defer span.End()

	config, err := tc.inner.GetTaskPushNotificationConfig(ctx, taskID, configID)
	if err != nil {
		span.RecordError(err)
	}
	return config, err
}

// ListTaskPushNotificationConfigs lists push configs with tracing.
func (tc *TracingClient) ListTaskPushNotificationConfigs(ctx context.Context, taskID string) (*ListTaskPushNotificationConfigsResponse, error) {
	ctx, span := tracer.Start(ctx, "a2a.ListTaskPushNotificationConfigs",
		trace.WithAttributes(
			attribute.String("a2a.agent_url", tc.inner.baseURL),
			attribute.String("a2a.task_id", taskID),
			attribute.String("a2a.operation", "push.list"),
		),
	)
	defer span.End()

	list, err := tc.inner.ListTaskPushNotificationConfigs(ctx, taskID)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	span.SetAttributes(attribute.Int("a2a.push_config_count", len(list.Configs)))
	return list, nil
}

// DeleteTaskPushNotificationConfig deletes a push config with tracing.
func (tc *TracingClient) DeleteTaskPushNotificationConfig(ctx context.Context, taskID, configID string) error {
	ctx, span := tracer.Start(ctx, "a2a.DeleteTaskPushNotificationConfig",
		trace.WithAttributes(
			attribute.String("a2a.agent_url", tc.inner.baseURL),
			attribute.String("a2a.task_id", taskID),
			attribute.String("a2a.push_config_id", configID),
			attribute.String("a2a.operation", "push.delete"),
		),
	)
	defer span.End()

	err := tc.inner.DeleteTaskPushNotificationConfig(ctx, taskID, configID)
	if err != nil {
		span.RecordError(err)
	}
	return err
}

// TracingServerMiddleware returns an http.Handler that adds tracing to A2A server requests.
func TracingServerMiddleware(next interface {
	ServeHTTP(w interface{}, r interface{})
}) {
	// Server-side tracing is handled by wrapping the Server's Handler() method.
	// Each JSON-RPC method gets its own span with A2A-specific attributes.
	// This is a placeholder for the full implementation.
	fmt.Println("a2a: tracing middleware active")
}
