package observability

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all the metric instruments for MCP tool invocations.
type Metrics struct {
	ToolInvocations metric.Int64Counter
	ToolDuration    metric.Float64Histogram
	ToolErrors      metric.Int64Counter
	ToolsActive     metric.Int64UpDownCounter
}

// Config holds observability configuration.
type Config struct {
	// ServiceName is the OTel service name (e.g., "my-mcp-server").
	ServiceName string

	// ServiceVersion is the OTel service version.
	ServiceVersion string

	// OTLPEndpoint is the OTLP collector endpoint (e.g., "localhost:4317").
	OTLPEndpoint string

	// PrometheusPort is the port to expose Prometheus metrics (e.g., "9091").
	// Empty disables the Prometheus HTTP server.
	PrometheusPort string

	// EnableTracing enables OTEL tracing export.
	EnableTracing bool

	// EnableMetrics enables OTEL metrics and Prometheus.
	EnableMetrics bool
}

// Provider holds initialized observability state.
type Provider struct {
	metrics *Metrics
	tracer  trace.Tracer
}

// Init initializes the observability stack and returns a Provider and a shutdown function.
func Init(ctx context.Context, cfg Config) (*Provider, func(context.Context) error, error) {
	if cfg.ServiceName == "" {
		return nil, nil, fmt.Errorf("observability: ServiceName is required")
	}

	var shutdownFuncs []func(context.Context) error
	p := &Provider{}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Tracing
	if cfg.EnableTracing && cfg.OTLPEndpoint != "" {
		traceExporter, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			log.Printf("Warning: failed to create trace exporter: %v", err)
		} else {
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithBatcher(traceExporter),
				sdktrace.WithResource(res),
			)
			otel.SetTracerProvider(tp)
			p.tracer = tp.Tracer(cfg.ServiceName)
			shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
		}
	}

	// Metrics
	if cfg.EnableMetrics {
		promExporter, err := prometheus.New()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
		}

		var meterProvider *sdkmetric.MeterProvider
		if cfg.OTLPEndpoint != "" {
			otlpExporter, err := otlpmetricgrpc.New(ctx,
				otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
				otlpmetricgrpc.WithInsecure(),
			)
			if err != nil {
				log.Printf("Warning: failed to create OTLP metric exporter: %v", err)
				meterProvider = sdkmetric.NewMeterProvider(
					sdkmetric.WithResource(res),
					sdkmetric.WithReader(promExporter),
				)
			} else {
				meterProvider = sdkmetric.NewMeterProvider(
					sdkmetric.WithResource(res),
					sdkmetric.WithReader(promExporter),
					sdkmetric.WithReader(sdkmetric.NewPeriodicReader(otlpExporter, sdkmetric.WithInterval(15*time.Second))),
				)
			}
		} else {
			meterProvider = sdkmetric.NewMeterProvider(
				sdkmetric.WithResource(res),
				sdkmetric.WithReader(promExporter),
			)
		}

		otel.SetMeterProvider(meterProvider)
		shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)

		meter := meterProvider.Meter(cfg.ServiceName)
		p.metrics, err = createMetrics(meter)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create metrics: %w", err)
		}

		if cfg.PrometheusPort != "" {
			go startPrometheusServer(cfg.PrometheusPort)
		}
	}

	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			if e := fn(ctx); e != nil {
				err = e
			}
		}
		return err
	}

	return p, shutdown, nil
}

func createMetrics(meter metric.Meter) (*Metrics, error) {
	invocations, err := meter.Int64Counter("mcp_tool_invocations",
		metric.WithDescription("Total number of tool invocations"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	duration, err := meter.Float64Histogram("mcp_tool_duration_seconds",
		metric.WithDescription("Duration of tool invocations in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, err
	}

	toolErrors, err := meter.Int64Counter("mcp_tool_errors",
		metric.WithDescription("Total number of tool errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	active, err := meter.Int64UpDownCounter("mcp_tools_active",
		metric.WithDescription("Number of currently executing tools"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		ToolInvocations: invocations,
		ToolDuration:    duration,
		ToolErrors:      toolErrors,
		ToolsActive:     active,
	}, nil
}

func startPrometheusServer(port string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(
		promclient.DefaultGatherer,
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	log.Printf("Prometheus metrics server starting on :%s/metrics", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Printf("Prometheus server error: %v", err)
	}
}

// RecordToolInvocation records metrics for a tool invocation.
func (p *Provider) RecordToolInvocation(ctx context.Context, toolName, category string, duration time.Duration, err error) {
	if p == nil || p.metrics == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("tool", toolName),
		attribute.String("category", category),
	}

	p.metrics.ToolInvocations.Add(ctx, 1, metric.WithAttributes(attrs...))
	p.metrics.ToolDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if err != nil {
		errorAttrs := append(attrs, attribute.String("error_type", categorizeError(err)))
		p.metrics.ToolErrors.Add(ctx, 1, metric.WithAttributes(errorAttrs...))
	}
}

// StartToolExecution marks a tool as starting execution.
func (p *Provider) StartToolExecution(ctx context.Context, toolName, category string) {
	if p == nil || p.metrics == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("tool", toolName),
		attribute.String("category", category),
	}
	p.metrics.ToolsActive.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// EndToolExecution marks a tool as finishing execution.
func (p *Provider) EndToolExecution(ctx context.Context, toolName, category string) {
	if p == nil || p.metrics == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("tool", toolName),
		attribute.String("category", category),
	}
	p.metrics.ToolsActive.Add(ctx, -1, metric.WithAttributes(attrs...))
}

// StartSpan starts a new trace span for a tool invocation.
// The span includes GenAI semantic convention attributes identifying the
// system as "mcp" and the operation as "tool_call".
func (p *Provider) StartSpan(ctx context.Context, toolName string) (context.Context, trace.Span) {
	if p == nil || p.tracer == nil {
		return ctx, nil
	}
	return p.tracer.Start(ctx, toolName,
		trace.WithAttributes(
			attribute.String("tool.name", toolName),
			AttrGenAISystem.String("mcp"),
			AttrGenAIOperationName.String("tool_call"),
		),
	)
}

func categorizeError(err error) string {
	if err == nil {
		return "none"
	}
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "timeout"):
		return "timeout"
	case strings.Contains(errStr, "context canceled"):
		return "canceled"
	case strings.Contains(errStr, "connection"):
		return "connection"
	case strings.Contains(errStr, "panic"):
		return "panic"
	default:
		return "other"
	}
}
