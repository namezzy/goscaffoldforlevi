// Package telemetry builds the logger, and from Phase 5 the tracer and meter.
//
// It is the only place allowed to know about correlation identifiers, so that
// business code can call logger.InfoContext(ctx, ...) and get request_id and
// (later) trace_id attached without doing anything.
package telemetry

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/example/go-enterprise/internal/config"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
)

// ContextWithRequestID returns a context carrying the request identifier.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// RequestIDFromContext returns the request identifier, or "" if absent.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID).(string)
	return id
}

// contextHandler decorates every record with correlation fields taken from the
// context. Extracting them here rather than at each call site is what keeps
// "log the trace id" from becoming a rule people have to remember.
type contextHandler struct{ slog.Handler }

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	// Phase 5 adds trace_id and span_id here.
	return h.Handler.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}

// LoggerOptions controls logger construction.
type LoggerOptions struct {
	// Level defaults to Info. Debug is enabled by LOG_LEVEL in development.
	Level slog.Level
	// Writer defaults to os.Stdout. Tests substitute a buffer.
	Writer io.Writer
}

// NewLogger builds the application logger.
//
// Development uses human-readable text; every other environment uses JSON so
// that log aggregation can parse it. Base fields identify the process in a
// multi-service deployment.
func NewLogger(app config.App, opts LoggerOptions) *slog.Logger {
	w := opts.Writer
	if w == nil {
		w = os.Stdout
	}

	handlerOpts := &slog.HandlerOptions{Level: opts.Level}

	var base slog.Handler
	if app.Environment == config.EnvDevelopment {
		base = slog.NewTextHandler(w, handlerOpts)
	} else {
		base = slog.NewJSONHandler(w, handlerOpts)
	}

	return slog.New(contextHandler{base}).With(
		slog.String("service", app.Name),
		slog.String("environment", string(app.Environment)),
		slog.String("version", app.Version),
	)
}
