package redis_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	goredis "github.com/redis/go-redis/v9"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/example/go-enterprise/internal/redis"
	"github.com/example/go-enterprise/internal/telemetry"
)

func recordingTracer(t *testing.T) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp, rec
}

// run drives the hook the way go-redis does, with a next that returns the
// supplied error and never touches the network.
func run(hook goredis.Hook, cmd goredis.Cmder, err error) {
	next := func(ctx context.Context, cmd goredis.Cmder) error {
		if err != nil {
			cmd.SetErr(err)
		}
		return err
	}
	_ = hook.ProcessHook(next)(context.Background(), cmd)
}

// §5.6 allows the command name, the latency and the error. Everything else in
// a Redis call is the payload: keys in this application are "rl:login:<email>"
// and "session:<user id>", and values are refresh tokens.
func TestTheSpanIsNamedAfterTheCommandOnly(t *testing.T) {
	tp, rec := recordingTracer(t)
	m := telemetry.NewMetrics()

	cmd := goredis.NewStatusCmd(context.Background(),
		"set", "session:01a05c0a-1107-7e48-9698-cdb4fbedd2c2",
		"HIRytbaTFK_u3zPWJ2vw_Vh336bgGegVo9unOV5eQv0")

	run(redis.NewHook(tp, m), cmd, nil)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if got := spans[0].Name(); got != "redis SET" {
		t.Errorf("span name = %q, want %q", got, "redis SET")
	}
}

// The test that matters. A rate limiter key contains the email address being
// throttled and a session value is a live credential; either reaching the
// trace backend is a disclosure, and the default hook in every instrumentation
// library records db.statement, which is the full command.
func TestNoKeyOrValueReachesTheSpan(t *testing.T) {
	tp, rec := recordingTracer(t)
	m := telemetry.NewMetrics()

	const (
		key   = "rl:login:victim@example.com"
		token = "HIRytbaTFK_u3zPWJ2vw_Vh336bgGegVo9unOV5eQv0"
	)

	cmd := goredis.NewStatusCmd(context.Background(), "set", key, token)
	run(redis.NewHook(tp, m), cmd, nil)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}

	haystack := spans[0].Name() + "\x00" + spans[0].Status().Description
	for _, kv := range spans[0].Attributes() {
		haystack += "\x00" + string(kv.Key) + "\x00" + kv.Value.String()
	}
	for _, ev := range spans[0].Events() {
		haystack += "\x00" + ev.Name
		for _, kv := range ev.Attributes {
			haystack += "\x00" + kv.Value.String()
		}
	}

	for _, secret := range []string{key, token, "victim@example.com"} {
		if strings.Contains(haystack, secret) {
			t.Errorf("span carries %q: %s", secret, haystack)
		}
	}
}

// The same rule applies to the metric, and more strictly: a span is sampled
// and expires, a Prometheus label is a series that lives as long as the
// process. One label per rate-limited email address is a memory leak that any
// visitor can grow.
func TestTheMetricIsLabelledByCommandOnly(t *testing.T) {
	tp, _ := recordingTracer(t)
	m := telemetry.NewMetrics()

	hook := redis.NewHook(tp, m)
	for _, email := range []string{"a@example.com", "b@example.com", "c@example.com"} {
		run(hook, goredis.NewIntCmd(context.Background(), "incr", "rl:login:"+email), nil)
	}

	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather() = %v", err)
	}

	series := 0
	for _, f := range families {
		if f.GetName() != "redis_operation_total" {
			continue
		}
		series = len(f.GetMetric())
		for _, metric := range f.GetMetric() {
			for _, label := range metric.GetLabel() {
				if strings.Contains(label.GetValue(), "@example.com") {
					t.Errorf("label %s = %q carries a key",
						label.GetName(), label.GetValue())
				}
			}
		}
	}
	if series != 1 {
		t.Errorf("three keys produced %d series, want 1", series)
	}
}

func TestAFailedCommandIsRecordedAsAnError(t *testing.T) {
	tp, rec := recordingTracer(t)
	m := telemetry.NewMetrics()

	cmd := goredis.NewStatusCmd(context.Background(), "get", "k")
	run(redis.NewHook(tp, m), cmd, errors.New("connection refused"))

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if spans[0].Status().Code.String() != "Error" {
		t.Errorf("span status = %v, want Error", spans[0].Status().Code)
	}

	if outcome := outcomeLabel(t, m); outcome != "error" {
		t.Errorf("redis_operation_total outcome = %q, want error", outcome)
	}
}

// redis.Nil means "the key is not there", which is the expected answer for
// every cache lookup that misses. Counting it as an error would put the error
// rate of a healthy cache somewhere near its miss rate.
func TestACacheMissIsNotAnError(t *testing.T) {
	tp, rec := recordingTracer(t)
	m := telemetry.NewMetrics()

	cmd := goredis.NewStringCmd(context.Background(), "get", "k")
	run(redis.NewHook(tp, m), cmd, goredis.Nil)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if spans[0].Status().Code.String() == "Error" {
		t.Error("a cache miss marked the span as failed")
	}
	if outcome := outcomeLabel(t, m); outcome != "ok" {
		t.Errorf("redis_operation_total outcome = %q, want ok", outcome)
	}
}

func outcomeLabel(t *testing.T, m *telemetry.Metrics) string {
	t.Helper()

	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather() = %v", err)
	}
	for _, f := range families {
		if f.GetName() != "redis_operation_total" {
			continue
		}
		for _, metric := range f.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "outcome" {
					return label.GetValue()
				}
			}
		}
	}
	return ""
}

// Found by a mutation that survived: copying err.Error() onto the span passed
// every test here, because the only failing test used "connection refused".
//
// Redis error text is not reliably free of the command. Cluster redirections
// quote keys and node addresses, CROSSSLOT quotes the key set, and anything
// this application wraps around a Redis error on the way out may add the key
// for its own debugging. The span therefore records that the command failed
// and nothing about why — the error still travels up to the caller, which is
// where ADR-0003 already decides what may be shown.
func TestARedisErrorMessageDoesNotReachTheSpan(t *testing.T) {
	tp, rec := recordingTracer(t)
	m := telemetry.NewMetrics()

	const key = "rl:login:victim@example.com"
	err := errors.New(`MOVED 3999 10.0.0.7:6381 while routing ` + key)

	cmd := goredis.NewIntCmd(context.Background(), "incr", key)
	run(redis.NewHook(tp, m), cmd, err)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}

	haystack := spans[0].Status().Description
	for _, kv := range spans[0].Attributes() {
		haystack += "\x00" + kv.Value.String()
	}
	for _, ev := range spans[0].Events() {
		haystack += "\x00" + ev.Name
		for _, kv := range ev.Attributes {
			haystack += "\x00" + kv.Value.String()
		}
	}

	for _, secret := range []string{key, "victim@example.com", "10.0.0.7:6381"} {
		if strings.Contains(haystack, secret) {
			t.Errorf("span carries %q: %s", secret, haystack)
		}
	}

	if spans[0].Status().Code.String() != "Error" {
		t.Errorf("span status = %v, want it still marked as failed",
			spans[0].Status().Code)
	}
}
