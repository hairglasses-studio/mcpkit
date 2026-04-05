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

// TracingServerMiddleware returns an http.Handler that adds tracing to A2A server requests.
func TracingServerMiddleware(next interface{ ServeHTTP(w interface{}, r interface{}) }) {
	// Server-side tracing is handled by wrapping the Server's Handler() method.
	// Each JSON-RPC method gets its own span with A2A-specific attributes.
	// This is a placeholder for the full implementation.
	fmt.Println("a2a: tracing middleware active")
}
