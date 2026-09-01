package redis

import (
	"context"
	"errors"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/example/go-enterprise/internal/telemetry"
)

const instrumentationName = "github.com/example/go-enterprise/internal/redis"

// Hook records the command name, its latency and whether it failed.
//
// It records nothing else, and that is the point. Every off-the-shelf Redis
// instrumentation sets db.statement to the full command, which in this
// application means "rl:login:victim@example.com" and, for a session write,
// the refresh token itself. §5.6 allows the command name; the implementation
// is an allowlist so that a new command cannot quietly add a new field.
type Hook struct {
	tracer  trace.Tracer
	metrics *telemetry.Metrics
}

// NewHook builds the hook. metrics may be nil.
func NewHook(tp telemetry.TracerProvider, m *telemetry.Metrics) *Hook {
	return &Hook{tracer: tp.Tracer(instrumentationName), metrics: m}
}

var _ goredis.Hook = (*Hook)(nil)

// DialHook is required by the interface and adds nothing: a dial has no
// command name, and its failures already surface on the command that
// triggered it.
func (h *Hook) DialHook(next goredis.DialHook) goredis.DialHook { return next }

// ProcessHook instruments a single command.
func (h *Hook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		name := commandName(cmd)

		ctx, span := h.tracer.Start(ctx, "redis "+name,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system", "redis"),
				attribute.String("db.operation", name),
			),
		)
		defer span.End()

		start := time.Now()
		err := next(ctx, cmd)
		elapsed := time.Since(start)

		if failed(err) {
			// The message is omitted for the same reason as in postgres: the
			// error text can quote the command, and the command contains the
			// key.
			span.SetStatus(codes.Error, "command failed")
		} else {
			err = nil
		}

		if h.metrics != nil {
			h.metrics.RedisOperation(name, elapsed.Seconds(), err)
		}
		return err
	}
}

// failed reports whether an error represents a fault.
//
// redis.Nil means the key is absent, which is the expected answer for every
// cache lookup that misses; counting it would put a healthy cache's error rate
// somewhere near its miss rate. Context cancellation is the caller giving up,
// not Redis failing.
func failed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, goredis.Nil) {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	return true
}

// commandName returns the uppercased verb, e.g. "SET".
//
// cmd.Name() is the verb alone; cmd.String() would be the whole command with
// its arguments. The two are one method call apart, which is exactly how
// keys end up in traces.
func commandName(cmd goredis.Cmder) string {
	if cmd == nil {
		return "UNKNOWN"
	}
	name := strings.ToUpper(cmd.Name())
	if name == "" {
		return "UNKNOWN"
	}
	return name
}

// ProcessPipelineHook instruments a pipeline as one span.
//
// The commands in a pipeline are not necessarily the same verb, so the span is
// named for the batch and carries only its size. Naming it after the commands
// would either be unbounded or be a lie about which one was slow.
func (h *Hook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []goredis.Cmder) error {
		ctx, span := h.tracer.Start(ctx, "redis PIPELINE",
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system", "redis"),
				attribute.String("db.operation", "PIPELINE"),
				attribute.Int("db.batch.size", len(cmds)),
			),
		)
		defer span.End()

		start := time.Now()
		err := next(ctx, cmds)
		elapsed := time.Since(start)

		if failed(err) {
			span.SetStatus(codes.Error, "pipeline failed")
		} else {
			err = nil
		}

		if h.metrics != nil {
			h.metrics.RedisOperation("PIPELINE", elapsed.Seconds(), err)
		}
		return err
	}
}
