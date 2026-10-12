package requesttrackhttp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
	"github.com/namezzy/goscaffoldforlevi/internal/requesttrack"
)

// heartbeatInterval is how often a live stream sends an SSE comment line
// when nothing else happened, so an idle connection is distinguishable from
// a dead one on both ends before any proxy's own idle timeout would decide
// for them.
const heartbeatInterval = 15 * time.Second

// Handler serves the live request feed.
type Handler struct {
	tracker *requesttrack.Tracker
	log     *slog.Logger
}

// NewHandler builds the handler.
func NewHandler(tracker *requesttrack.Tracker, log *slog.Logger) *Handler {
	return &Handler{tracker: tracker, log: log}
}

// Routes mounts the request-feed endpoint.
func (h *Handler) Routes(requirePermission func(auth.Permission) func(http.Handler) http.Handler) func(chi.Router) {
	return func(r chi.Router) {
		r.With(requirePermission(auth.PermRequestsRead)).Get("/", h.Get)
	}
}

// Get answers a snapshot by default, or a live stream when the caller asks
// for one with Accept: text/event-stream. One route rather than two: a
// plain GET completes immediately, which is what lets a request against
// this same operation sit in api/bruno's collection without hanging it for
// the length of a live connection.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		h.stream(w, r)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toEventListResponse(h.tracker.Recent()))
}

// stream holds the connection open and pushes one SSE message per request
// recorded from this point on, plus a periodic heartbeat comment.
//
// It ends on its own after a while: the context deadline set by the global
// Timeout middleware fires like it would for any other request, at which
// point this returns and the client is expected to reconnect. That is not a
// bug being tolerated — a live feed that reconnects occasionally is simpler
// and no less correct than carving out a per-route exemption from a timeout
// every other endpoint is bound by, and the JSON snapshot above exists
// exactly so a reconnect loses nothing worth keeping.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rc := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		h.log.WarnContext(ctx, "request stream: the response does not support flushing",
			slog.Any("error", err))
		return
	}

	ch, cancel := h.tracker.Subscribe()
	defer cancel()

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case e, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(toEventResponse(e))
			if err != nil {
				// Not recoverable by retrying the same value, and not worth
				// ending the stream over one skipped event.
				h.log.ErrorContext(ctx, "request stream: marshal event failed", slog.Any("error", err))
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}

		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}
