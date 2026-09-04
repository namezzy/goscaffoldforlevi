package telemetry_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/namezzy/goscaffoldforlevi/internal/messaging"

	"github.com/namezzy/goscaffoldforlevi/internal/config"
	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
)

func testApp() config.App {
	return config.App{
		Name:        "goscaffoldforlevi",
		Environment: config.EnvProduction,
		Version:     "1.2.3",
	}
}

// Disabled telemetry must cost nothing and reach nothing. A provider that
// still records spans would keep the allocation and the exporter queue while
// claiming to be off, and the operator who set OTEL_ENABLED=false to shed load
// would not get what they asked for.
func TestSpansAreNotRecordedWhenTracingIsDisabled(t *testing.T) {
	tp, err := telemetry.NewTracerProvider(context.Background(), testApp(),
		config.OTel{Enabled: false})
	if err != nil {
		t.Fatalf("NewTracerProvider() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	_, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	if span.IsRecording() {
		t.Error("span is recording while tracing is disabled")
	}
	if span.SpanContext().IsValid() {
		t.Error("disabled tracing produced a valid span context; " +
			"a trace id that leads to no trace is worse than none")
	}
}

// A disabled provider must not need the collector to exist. If construction
// dialled the endpoint, OTEL_ENABLED=false would still couple startup to a
// service the operator has just said they are not using.
func TestTracingDisabledDoesNotRequireACollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		tp, err := telemetry.NewTracerProvider(ctx, testApp(), config.OTel{
			Enabled:  false,
			Endpoint: "203.0.113.1:4317", // TEST-NET-3: guaranteed unroutable
			Protocol: config.OTLPGRPC,
		})
		if err == nil {
			_ = tp.Shutdown(context.Background())
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("NewTracerProvider() = %v, want nil", err)
		}
	case <-ctx.Done():
		t.Fatal("NewTracerProvider blocked on an unroutable endpoint while disabled")
	}
}

func TestSpansAreRecordedWhenTracingIsEnabled(t *testing.T) {
	tp, err := telemetry.NewTracerProvider(context.Background(), testApp(), config.OTel{
		Enabled:       true,
		Endpoint:      "203.0.113.1:4317",
		Protocol:      config.OTLPGRPC,
		Insecure:      true,
		SamplingRatio: 1,
	})
	if err != nil {
		t.Fatalf("NewTracerProvider() = %v, want nil", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	})

	_, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	if !span.IsRecording() {
		t.Error("span is not recording while tracing is enabled")
	}
	if !span.SpanContext().TraceID().IsValid() {
		t.Error("enabled tracing produced no trace id")
	}
}

// Every span must carry enough identity to tell two deployments of the same
// binary apart. Without these three, a trace from staging and a trace from
// production are indistinguishable in the same backend.
func TestTheResourceIdentifiesTheServiceEnvironmentAndVersion(t *testing.T) {
	res := telemetry.Resource(testApp())

	want := map[attribute.Key]string{
		"service.name":           "goscaffoldforlevi",
		"service.version":        "1.2.3",
		"deployment.environment": "production",
	}

	got := map[attribute.Key]string{}
	for _, kv := range res.Attributes() {
		got[kv.Key] = kv.Value.String()
	}

	for k, v := range want {
		if got[k] != v {
			t.Errorf("resource attribute %s = %q, want %q", k, got[k], v)
		}
	}
}

// The resource is built from configuration, never from the ambient
// environment. OTel's default detectors read OTEL_RESOURCE_ATTRIBUTES, which
// would let a stray variable rename the service in the trace backend while the
// application's own config says otherwise.
func TestTheResourceIgnoresAmbientResourceAttributes(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.name=impostor,deployment.environment=dev")

	res := telemetry.Resource(testApp())

	for _, kv := range res.Attributes() {
		if kv.Key == "service.name" && kv.Value.String() != "goscaffoldforlevi" {
			t.Errorf("service.name = %q, want the configured name to win",
				kv.Value.String())
		}
	}
}

// Shutdown runs on the way out of a process that may already be failing. It
// has to tolerate being called on a provider that never exported anything, and
// being called twice, because deferred cleanup and an explicit shutdown path
// both exist and neither knows about the other.
func TestShutdownIsSafeToCallTwice(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		tp, err := telemetry.NewTracerProvider(context.Background(), testApp(), config.OTel{
			Enabled:       enabled,
			Endpoint:      "203.0.113.1:4317",
			Protocol:      config.OTLPGRPC,
			Insecure:      true,
			SamplingRatio: 1,
		})
		if err != nil {
			t.Fatalf("enabled=%v: NewTracerProvider() = %v", enabled, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := tp.Shutdown(ctx); err != nil {
			t.Errorf("enabled=%v: first Shutdown() = %v, want nil", enabled, err)
		}
		if err := tp.Shutdown(ctx); err != nil {
			t.Errorf("enabled=%v: second Shutdown() = %v, want nil", enabled, err)
		}
		cancel()
	}
}

func TestAnUnknownProtocolIsRejectedRatherThanGuessed(t *testing.T) {
	_, err := telemetry.NewTracerProvider(context.Background(), testApp(), config.OTel{
		Enabled:  true,
		Endpoint: "203.0.113.1:4317",
		Protocol: "thrift",
	})
	if err == nil {
		t.Fatal("NewTracerProvider() = nil error, want a rejected protocol")
	}
	if !strings.Contains(err.Error(), "thrift") {
		t.Errorf("error does not name the offending protocol: %v", err)
	}
}

// §5.7. The consumer span must join the trace of the request that produced the
// event, not start a fresh one. Getting this wrong is not visible from either
// side: the producer's trace looks complete and the consumer's trace looks
// complete, and nothing shows that the second was caused by the first.
func TestASpanStartedFromATraceParentJoinsThatTrace(t *testing.T) {
	tp, err := telemetry.NewTracerProvider(context.Background(), testApp(), config.OTel{
		Enabled: true, Endpoint: "203.0.113.1:4317",
		Protocol: config.OTLPGRPC, Insecure: true, SamplingRatio: 1,
	})
	if err != nil {
		t.Fatalf("NewTracerProvider() = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	})

	const (
		traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
		spanID  = "00f067aa0ba902b7"
	)

	ctx := telemetry.ContextWithTraceParent(context.Background(),
		"00-"+traceID+"-"+spanID+"-01")
	_, span := tp.Tracer("test").Start(ctx, "consume")
	defer span.End()

	if got := span.SpanContext().TraceID().String(); got != traceID {
		t.Errorf("trace id = %q, want the producer's %q", got, traceID)
	}
}

// A malformed or absent traceparent must leave the context alone, so the
// consumer span becomes a root. Fabricating a parent would produce a trace
// that looks joined and is not, which is worse than an obviously separate one.
func TestABadTraceParentDoesNotFabricateAParent(t *testing.T) {
	for _, bad := range []string{
		"",
		"garbage",
		"00-4bf92f3577b34da6a3ce929d0e0e4736",
		"00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
	} {
		t.Run(bad, func(t *testing.T) {
			ctx := telemetry.ContextWithTraceParent(context.Background(), bad)
			if trace.SpanContextFromContext(ctx).IsValid() {
				t.Errorf("%q produced a valid parent span context", bad)
			}
		})
	}
}

// The round trip must be exact: what TraceParentFromSpan writes has to be what
// ContextWithTraceParent can read back, or the outbox column is written in one
// format and parsed in another and the link silently never forms.
func TestATraceParentSurvivesTheRoundTrip(t *testing.T) {
	tp, err := telemetry.NewTracerProvider(context.Background(), testApp(), config.OTel{
		Enabled: true, Endpoint: "203.0.113.1:4317",
		Protocol: config.OTLPGRPC, Insecure: true, SamplingRatio: 1,
	})
	if err != nil {
		t.Fatalf("NewTracerProvider() = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	})

	ctx, producer := tp.Tracer("test").Start(context.Background(), "publish")
	defer producer.End()

	header := telemetry.TraceParentFromContext(ctx)
	if header == "" {
		t.Fatal("TraceParentFromContext returned nothing for a recording span")
	}
	if !messaging.ValidTraceParent(header) {
		t.Fatalf("TraceParentFromContext produced %q, which the outbox would reject", header)
	}

	_, consumer := tp.Tracer("test").Start(
		telemetry.ContextWithTraceParent(context.Background(), header), "consume")
	defer consumer.End()

	if got, want := consumer.SpanContext().TraceID(), producer.SpanContext().TraceID(); got != want {
		t.Errorf("consumer trace id = %v, want %v", got, want)
	}
}

// Found by a mutation that survived: dropping the validity check produced a
// header of all zeroes whenever tracing was off, and every test still passed
// because they all held a real span.
//
// This is the default deployment. An all-zero traceparent is syntactically
// well formed, so anything that stores headers without validating them — a log
// field, a broker header, an outgoing HTTP call — would stamp the same id on
// every event the system ever emits, and the trace backend would render the
// whole installation as one unbounded trace.
func TestNoTraceParentIsEmittedWhenThereIsNoSpan(t *testing.T) {
	for name, ctx := range map[string]context.Context{
		"no span at all": context.Background(),
		"a non-recording span from the noop provider": func() context.Context {
			ctx, span := telemetry.NoopTracerProvider().Tracer("t").
				Start(context.Background(), "op")
			defer span.End()
			return ctx
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			if got := telemetry.TraceParentFromContext(ctx); got != "" {
				t.Errorf("TraceParentFromContext() = %q, want empty", got)
			}
		})
	}
}
