package httpx

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
)

// instrumentationName identifies this package as the source of its spans, so
// that a trace backend can tell instrumentation apart from application spans.
const instrumentationName = "github.com/namezzy/goscaffoldforlevi/internal/httpx"

// Tracing starts a server span for every request.
//
// It is written by hand rather than mounted from otelhttp because of one
// detail otelhttp cannot know: chi resolves the route template *during*
// routing, so the template is only readable after the inner handler has run.
// otelhttp names the span when it starts, which for a chi router means naming
// it after the method alone or after the raw path — the first is useless and
// the second is the high-cardinality mistake §5.3 forbids. Starting the span
// here and renaming it on the way out gets the template without giving up the
// span's coverage of the handler.
func Tracing(tp telemetry.TracerProvider) func(http.Handler) http.Handler {
	tracer := tp.Tracer(instrumentationName)
	propagator := telemetry.Propagator()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Continue the caller's trace when there is one. Extraction is
			// unconditional: if the header is absent or malformed the
			// propagator leaves the context alone and the span below becomes a
			// root, which is the correct outcome for an entry point.
			ctx := propagator.Extract(r.Context(),
				propagation.HeaderCarrier(r.Header))

			// The name is provisional. The template is not known yet, and a
			// span must exist before the handler runs so that anything the
			// handler starts is parented correctly.
			ctx, span := tracer.Start(ctx, r.Method,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.request.method", r.Method),
				),
			)
			defer span.End()

			ctx = traceParentForEvents(ctx, span, r)

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r.WithContext(ctx))

			status, _ := rec.snapshot()
			route := routeTemplate(r)

			span.SetName(r.Method + " " + route)
			span.SetAttributes(
				attribute.String("http.route", route),
				attribute.Int("http.response.status_code", status),
			)

			// Only 5xx. A 404 or a 401 is the API behaving correctly, and a
			// trace list in which every mistyped URL is red is a trace list
			// people stop looking at.
			if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, strconv.Itoa(status))
			}
		})
	}
}

// traceParentForEvents puts the identifier that asynchronous work should be
// parented on into the context.
//
// The server span wins over the inbound header. Forwarding the header would
// make a consumer a child of whoever called *us*, so the trace would show a
// publish that no request in this service appears to have caused. When tracing
// is off there is no span to name, and the caller's header is then the best
// correlation available — dropping it would blank the outbox's traceparent
// column for every deployment running without a collector, which is the
// default one.
func traceParentForEvents(ctx context.Context, span trace.Span, r *http.Request) context.Context {
	if sc := span.SpanContext(); sc.IsValid() {
		return messaging.ContextWithTraceParent(ctx, formatTraceParent(sc))
	}
	if header := r.Header.Get("traceparent"); header != "" {
		return messaging.ContextWithTraceParent(ctx, header)
	}
	return ctx
}

// formatTraceParent renders a span context as a W3C traceparent.
func formatTraceParent(sc trace.SpanContext) string {
	return "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() +
		"-" + sc.TraceFlags().String()
}

// Metrics records one observation per request.
//
// It is separate from Tracing because the two are enabled independently:
// metrics are cheap and on by default, traces cost a collector. Mounting them
// together would mean turning off the expensive one to turn off the cheap one.
func Metrics(m *telemetry.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			status, _ := rec.snapshot()
			m.HTTPRequest(r.Method, routeTemplate(r), status,
				time.Since(start).Seconds())
		})
	}
}

// routeTemplate returns the chi route pattern for a served request, or
// telemetry.RouteUnmatched when nothing matched.
//
// This must be called after the inner handler has run: chi fills the pattern
// in while routing, so reading it on the way in yields "".
//
// The empty case is the one that matters. A request that matched no route has
// no template, and labelling it with the path would let anyone mint an
// unbounded number of time series and span names by requesting /aaaa, /aaab,
// and so on — a memory exhaustion bug with a remote trigger.
func routeTemplate(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return telemetry.RouteUnmatched
	}
	if pattern := rctx.RoutePattern(); pattern != "" {
		return pattern
	}
	return telemetry.RouteUnmatched
}
