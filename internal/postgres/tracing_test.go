package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/example/go-enterprise/internal/postgres"
)

func recordingTracer(t *testing.T) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp, rec
}

// trace drives the pgx tracer the way the driver does.
//
// conn is nil on purpose: the tracer must not dereference it, because doing so
// would make every span depend on a live connection and would break the moment
// pgx traces an acquisition failure.
func trace(tracer pgx.QueryTracer, sql string, args []any, err error) {
	ctx := tracer.TraceQueryStart(context.Background(), nil,
		pgx.TraceQueryStartData{SQL: sql, Args: args})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: err})
}

const sqlcQuery = `-- name: GetUserByEmail :one
SELECT id, email, name FROM users WHERE email = $1
`

// sqlc writes the query name into the statement as a comment, which makes it
// the one label available that is both meaningful and bounded: there are as
// many values as there are queries in the repository, and that number only
// changes when someone edits queries/.
func TestTheSpanIsNamedAfterTheQueryNotTheSQL(t *testing.T) {
	tp, rec := recordingTracer(t)

	trace(postgres.NewTracer(tp), sqlcQuery, []any{"nobody@example.com"}, nil)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if got := spans[0].Name(); got != "db.query GetUserByEmail" {
		t.Errorf("span name = %q, want %q", got, "db.query GetUserByEmail")
	}
}

// §5.5. The arguments are the whole risk: this query's argument is an email
// address, and the next one's is a password hash or a refresh token. A trace
// backend is a second copy of the database with none of its access controls,
// so nothing derived from the arguments may reach it.
func TestNoArgumentValueEverReachesTheSpan(t *testing.T) {
	tp, rec := recordingTracer(t)

	const (
		email = "victim@example.com"
		token = "HIRytbaTFK_u3zPWJ2vw_Vh336bgGegVo9unOV5eQv0"
		hash  = "$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHQ$aGFzaA"
	)

	trace(postgres.NewTracer(tp),
		`-- name: CreateUser :one
INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id
`,
		[]any{email, hash, token}, nil)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}

	haystack := spans[0].Name()
	for _, kv := range spans[0].Attributes() {
		haystack += "\x00" + string(kv.Key) + "\x00" + kv.Value.String()
	}
	for _, ev := range spans[0].Events() {
		haystack += "\x00" + ev.Name
		for _, kv := range ev.Attributes {
			haystack += "\x00" + string(kv.Key) + "\x00" + kv.Value.String()
		}
	}

	for _, secret := range []string{email, token, hash} {
		if strings.Contains(haystack, secret) {
			t.Errorf("span carries an argument value: %q", secret)
		}
	}
}

// The statement text is excluded too. It is less obviously dangerous than the
// arguments — sqlc only emits parameterised SQL — but "less obviously" is the
// problem: one hand-written query with an inlined literal, one migration
// replayed through the pool, and the trace backend has the value. Excluding
// the category is cheaper than auditing every member of it.
func TestTheStatementTextIsNotRecorded(t *testing.T) {
	tp, rec := recordingTracer(t)

	trace(postgres.NewTracer(tp), sqlcQuery, nil, nil)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	for _, kv := range spans[0].Attributes() {
		if strings.Contains(kv.Value.String(), "SELECT") {
			t.Errorf("attribute %s carries the statement: %q", kv.Key, kv.Value.String())
		}
	}
}

// A statement with no sqlc header still needs a bounded name. Falling back to
// the SQL would put an unbounded string in the span name; falling back to the
// verb keeps it to the handful of values SQL has.
func TestAnUnnamedStatementFallsBackToItsVerb(t *testing.T) {
	cases := map[string]string{
		"SELECT 1":                             "db.query SELECT",
		"  insert into t values (1)":           "db.query INSERT",
		"\n\nUPDATE t SET a = 1":               "db.query UPDATE",
		"DELETE FROM t":                        "db.query DELETE",
		"BEGIN":                                "db.query BEGIN",
		"":                                     "db.query UNKNOWN",
		"WITH x AS (SELECT 1) SELECT * FROM x": "db.query WITH",
	}

	for sql, want := range cases {
		t.Run(want, func(t *testing.T) {
			tp, rec := recordingTracer(t)

			trace(postgres.NewTracer(tp), sql, nil, nil)

			spans := rec.Ended()
			if len(spans) != 1 {
				t.Fatalf("recorded %d spans, want 1", len(spans))
			}
			if got := spans[0].Name(); got != want {
				t.Errorf("span name = %q, want %q", got, want)
			}
		})
	}
}

// A failed query must be visible as a failure, or the trace shows a fast,
// successful call where there was an error.
func TestAFailedQueryMarksTheSpan(t *testing.T) {
	tp, rec := recordingTracer(t)

	trace(postgres.NewTracer(tp), sqlcQuery, nil, errors.New("connection refused"))

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if spans[0].Status().Code.String() != "Error" {
		t.Errorf("span status = %v, want Error", spans[0].Status().Code)
	}
}

// pgx.ErrNoRows means the row is not there, which is an answer. Marking it as
// a span error would paint every "does this email already exist?" check red.
func TestNoRowsIsNotASpanError(t *testing.T) {
	tp, rec := recordingTracer(t)

	trace(postgres.NewTracer(tp), sqlcQuery, nil, pgx.ErrNoRows)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if spans[0].Status().Code.String() == "Error" {
		t.Error("ErrNoRows marked the span as failed")
	}
}

// The mutation that found this: recording data.Err.Error() on the span passed
// every other test in this file, because they all use errors with harmless
// text. Real driver errors are not harmless. PostgreSQL reports a unique
// violation as
//
//	duplicate key value violates unique constraint "users_email_key"
//	DETAIL: Key (email)=(victim@example.com) already exists.
//
// so the error message for the most common failure in the system contains the
// address that failed. Others quote the parameter that was out of range.
//
// The span therefore records that the query failed and nothing about why; the
// error itself travels up the call stack to the logger, which is where the
// error model in ADR-0003 already decides what a caller may see.
func TestADriverErrorMessageDoesNotReachTheSpan(t *testing.T) {
	tp, rec := recordingTracer(t)

	const email = "victim@example.com"
	err := errors.New(`ERROR: duplicate key value violates unique constraint ` +
		`"users_email_key" (SQLSTATE 23505) DETAIL: Key (email)=(` + email + `) already exists.`)

	trace(postgres.NewTracer(tp), sqlcQuery, nil, err)

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

	if strings.Contains(haystack, email) {
		t.Errorf("the span carries the colliding value: %q", haystack)
	}
	if strings.Contains(haystack, "users_email_key") {
		t.Errorf("the span carries the constraint name and its DETAIL: %q", haystack)
	}

	// It must still be visible as a failure.
	if spans[0].Status().Code.String() != "Error" {
		t.Errorf("span status = %v, want Error", spans[0].Status().Code)
	}
}
