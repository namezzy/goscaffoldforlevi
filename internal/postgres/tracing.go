package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/example/go-enterprise/internal/telemetry"
)

const instrumentationName = "github.com/example/go-enterprise/internal/postgres"

// Tracer implements pgx.QueryTracer.
//
// It is written here rather than pulled from an instrumentation library
// because the libraries record db.statement by default, and some record the
// arguments as well. That is the wrong default for this repository: the
// arguments to CreateUser are an email address and a password hash, and a
// trace backend is a second copy of the database with none of its access
// controls. What is recorded is therefore an allowlist — the query name and
// nothing else — rather than a denylist of fields to scrub.
type Tracer struct {
	tracer trace.Tracer
}

// NewTracer builds the query tracer.
func NewTracer(tp telemetry.TracerProvider) *Tracer {
	return &Tracer{tracer: tp.Tracer(instrumentationName)}
}

type tracerSpanKey struct{}

// TraceQueryStart opens a span for one statement.
//
// conn is never dereferenced. pgx passes it for context, but a tracer that
// needs a live connection cannot trace the failures that matter most, and the
// pool's own tests drive this with a nil.
func (t *Tracer) TraceQueryStart(
	ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData,
) context.Context {
	ctx, span := t.tracer.Start(ctx, "db.query "+queryName(data.SQL),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", queryName(data.SQL)),
		),
	)
	return context.WithValue(ctx, tracerSpanKey{}, span)
}

// TraceQueryEnd closes the span.
func (t *Tracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span, ok := ctx.Value(tracerSpanKey{}).(trace.Span)
	if !ok {
		return
	}
	defer span.End()

	// ErrNoRows is an answer, not a fault. Marking it would paint every
	// "does this email already exist?" check red, and an error colour that
	// appears on healthy traffic stops meaning anything.
	if data.Err != nil && !errors.Is(data.Err, pgx.ErrNoRows) {
		// The message is not recorded. Driver errors quote the failing
		// statement, and constraint violations quote the value that collided —
		// which for users_email_key is the email address.
		span.SetStatus(codes.Error, "query failed")
	}
}

// queryName reduces a statement to a bounded identifier.
//
// sqlc writes "-- name: GetUserByEmail :one" as the first line of every
// generated query, so the name is available without a registry to maintain and
// changes only when someone edits queries/. Anything else falls back to the
// leading verb, because the alternative — the statement itself — is an
// unbounded span name.
func queryName(sql string) string {
	trimmed := strings.TrimLeft(sql, " \t\r\n")

	if rest, ok := strings.CutPrefix(trimmed, "-- name: "); ok {
		name, _, _ := strings.Cut(rest, "\n")
		if before, _, found := strings.Cut(name, " :"); found {
			name = before
		}
		if name = strings.TrimSpace(name); name != "" {
			return name
		}
	}

	verb, _, _ := strings.Cut(trimmed, " ")
	verb, _, _ = strings.Cut(verb, "\n")
	if verb == "" {
		return "UNKNOWN"
	}
	return strings.ToUpper(verb)
}
