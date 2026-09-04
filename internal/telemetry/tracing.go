package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/namezzy/goscaffoldforlevi/internal/config"
)

// TracerProvider is what the composition root holds: something that hands out
// tracers and can be shut down. Narrowing it to this interface means the
// disabled path can return a no-op provider that has no Shutdown of its own,
// and callers cannot accidentally depend on SDK-only methods.
type TracerProvider interface {
	Tracer(name string, opts ...trace.TracerOption) trace.Tracer
	Shutdown(ctx context.Context) error
}

// Propagator is the context propagator every outbound integration uses.
//
// TraceContext is the W3C standard and is what the messaging envelope already
// carries. Baggage rides along because it is free here and is the only way to
// pass a tenant or a debug flag across a service boundary without inventing a
// header, but nothing in this repository writes to it yet.
func Propagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// Resource describes the process every span and metric belongs to.
//
// It is built from configuration and nothing else. resource.Default() merges
// OTEL_RESOURCE_ATTRIBUTES from the environment, which means a variable left
// over in a shell or a base image could rename the service in the trace
// backend while the application's own config says something different — a
// disagreement that surfaces as "the dashboards are empty" long after the
// change that caused it.
func Resource(app config.App) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(app.Name),
		semconv.ServiceVersion(app.Version),
		attribute.String("deployment.environment", string(app.Environment)),
	)
}

// noopProvider adapts the OTel no-op provider to TracerProvider by giving it
// the Shutdown the interface requires.
type noopProvider struct{ trace.TracerProvider }

func (noopProvider) Shutdown(context.Context) error { return nil }

// NewTracerProvider builds the tracer provider.
//
// When disabled it returns a no-op provider without constructing an exporter.
// That matters more than it looks: the OTLP exporters connect lazily, so a
// disabled-but-constructed exporter would sit there holding a gRPC client
// aimed at a collector the operator has just said does not exist, and would
// log connection failures forever. Disabled has to mean nothing is built.
//
// Spans are still created by the instrumentation in either case — the no-op
// provider makes them non-recording, so the cost is a struct that is never
// filled in, and business code needs no `if tracingEnabled` anywhere.
func NewTracerProvider(ctx context.Context, app config.App, cfg config.OTel) (TracerProvider, error) {
	if !cfg.Enabled {
		return noopProvider{noop.NewTracerProvider()}, nil
	}

	exporter, err := newSpanExporter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(Resource(app)),
		// ParentBased keeps a trace whole: once an upstream service has
		// decided to sample a request, every service it calls must agree, or
		// the trace arrives at the backend with holes in it. The ratio applies
		// only to requests that arrive with no decision already made.
		sdktrace.WithSampler(
			sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplingRatio)),
		),
	), nil
}

func newSpanExporter(ctx context.Context, cfg config.OTel) (sdktrace.SpanExporter, error) {
	switch cfg.Protocol {
	case config.OTLPGRPC:
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(ctx, opts...)
	case config.OTLPHTTP:
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)
	default:
		// Falling back to a default here would be worse than failing: the
		// process would start, export nothing the collector understands, and
		// look healthy.
		return nil, fmt.Errorf("unsupported OTLP protocol %q", cfg.Protocol)
	}
}

// NoopTracerProvider returns a provider that records nothing.
//
// It exists so that callers with no configured tracing do not have to hold a
// nil and every use site does not have to nil-check. A nil provider and a
// no-op provider differ only in which one panics.
func NoopTracerProvider() TracerProvider {
	return noopProvider{noop.NewTracerProvider()}
}
