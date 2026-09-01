package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/config"
	"github.com/example/go-enterprise/internal/messaging"
	"github.com/example/go-enterprise/internal/telemetry"
)

// HeaderRequestID is the header carrying the correlation identifier, both
// inbound (from an upstream proxy) and outbound.
const HeaderRequestID = "X-Request-ID"

// corsPreflightMaxAge is how long a browser may cache a preflight result.
// It is a constant rather than a setting: the allowed methods and headers are
// fixed at compile time, so there is nothing an operator would tune here.
const corsPreflightMaxAge = 10 * time.Minute

// statusRecorder captures the status code and byte count for logging and, in
// Phase 5, for metrics. It also records whether the header was written, which
// the timeout middleware needs to avoid writing a second one.
//
// The mutex is not decoration: the timeout middleware runs the handler on a
// separate goroutine and must be able to decide "has anything been written
// yet?" without racing the handler's own write.
type statusRecorder struct {
	http.ResponseWriter

	mu      sync.Mutex
	status  int
	written int64
	wrote   bool
}

func (w *statusRecorder) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writeHeaderLocked(status)
}

func (w *statusRecorder) writeHeaderLocked(status int) {
	if w.wrote {
		return
	}
	w.status = status
	w.wrote = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writeHeaderLocked(http.StatusOK)
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

// snapshot returns the recorded status and byte count.
func (w *statusRecorder) snapshot() (status int, written int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status, w.written
}

// Unwrap exposes the underlying writer so that http.ResponseController can
// reach optional interfaces such as Flusher and Hijacker.
func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Recovery converts a panic into a 500 response.
//
// It is the outermost middleware: a panic in any other middleware must still
// produce a response rather than killing the connection. The stack is logged
// and never sent to the client.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// A panic caused by the client disconnecting is not a bug and
				// no response can be written for it.
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}

				logger.ErrorContext(ctx, "panic recovered",
					slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				)
				WriteError(ctx, w, logger, apperr.Internal(errors.New("panic")))
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RequestID assigns a correlation identifier to the request.
//
// An inbound header is honoured so that a trace spanning several services
// keeps one id, but it is validated as a UUID first: an unvalidated header
// would let a client inject arbitrary text into every log line.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}

		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(telemetry.ContextWithRequestID(r.Context(), id)))
	})
}

// Logging records one line per completed request.
//
// The level is derived from the status so that an error budget can be read off
// the logs directly. Query strings are omitted: they routinely carry tokens
// and personal data.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			status, written := rec.snapshot()
			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int64("bytes", written),
				slog.Duration("duration", time.Since(start)),
			}

			switch {
			case status >= http.StatusInternalServerError:
				logger.ErrorContext(r.Context(), "request completed", attrs...)
			case status >= http.StatusBadRequest:
				logger.WarnContext(r.Context(), "request completed", attrs...)
			default:
				logger.InfoContext(r.Context(), "request completed", attrs...)
			}
		})
	}
}

// Timeout bounds handler execution.
//
// The deadline is placed on the context rather than using http.TimeoutHandler
// so that the handler's own database calls are cancelled too; a timeout that
// leaves queries running does not free the resource that caused it.
//
// The response is written only if the handler produced nothing, which is why
// it needs the statusRecorder installed by Logging.
func Timeout(d time.Duration, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			done := make(chan struct{})
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			go func() {
				defer close(done)
				next.ServeHTTP(rec, r.WithContext(ctx))
			}()

			select {
			case <-done:
				return
			case <-ctx.Done():
				if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return // client cancelled; no response to write
				}

				rec.mu.Lock()
				defer rec.mu.Unlock()
				if rec.wrote {
					// The handler already started responding; truncating it is
					// the best available outcome.
					return
				}

				logger.WarnContext(r.Context(), "request timed out",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Duration("timeout", d),
				)
				rec.Header().Set("Content-Type", "application/json; charset=utf-8")
				rec.writeHeaderLocked(http.StatusGatewayTimeout)
				_, _ = rec.ResponseWriter.Write(
					[]byte(`{"error":{"code":"REQUEST_TIMEOUT","message":"request timed out"}}`))
			}
		})
	}
}

// BodyLimit caps the size of a request body.
//
// Without it, a single request can exhaust memory before any handler runs.
// MaxBytesReader makes the limit the reader's problem, so handlers get a
// normal error instead of having to check sizes themselves.
func BodyLimit(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

// CORS answers preflight requests and adds the response headers.
//
// Only origins on the configured list are echoed back. A wildcard is never
// combined with credentials, which browsers reject anyway and which would
// otherwise let any site read authenticated responses.
func CORS(cfg config.HTTP) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(cfg.CORSAllowedOrigins))
	wildcard := false
	for _, o := range cfg.CORSAllowedOrigins {
		if o == "*" {
			wildcard = true
			continue
		}
		allowed[o] = struct{}{}
	}

	methods := strings.Join([]string{
		http.MethodGet, http.MethodPost, http.MethodPatch,
		http.MethodDelete, http.MethodOptions,
	}, ", ")
	headers := strings.Join([]string{"Authorization", "Content-Type", HeaderRequestID}, ", ")
	maxAge := strconv.Itoa(int(corsPreflightMaxAge.Seconds()))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			_, ok := allowed[origin]

			if origin != "" && (ok || wildcard) {
				if wildcard {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					// The response varies by Origin, so caches must not serve
					// one origin's response to another.
					w.Header().Add("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Expose-Headers", HeaderRequestID)
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", methods)
				w.Header().Set("Access-Control-Allow-Headers", headers)
				w.Header().Set("Access-Control-Max-Age", maxAge)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets defensive response headers.
//
// The API returns only JSON, so a restrictive CSP and nosniff cost nothing and
// remove a class of content-type confusion attacks.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// TraceContext copies an inbound W3C traceparent header into the request
// context so that events published while handling the request carry it, and
// the asynchronous work that follows shows up on the same trace as the call
// that caused it.
//
// This forwards the caller's header rather than starting a span; real
// instrumentation arrives in Phase 5 and will overwrite the value from the
// active span. A malformed or absent header leaves the context untouched —
// messaging.ContextWithTraceParent validates it, because a fabricated
// traceparent produces traces that look joined but are not, which is worse
// than having none.
func TraceContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if header := r.Header.Get("traceparent"); header != "" {
			ctx := messaging.ContextWithTraceParent(r.Context(), header)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}
