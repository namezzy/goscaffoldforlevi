package messaginghttp

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
)

// Handler serves the messaging admin endpoints.
type Handler struct {
	svc *messaging.Admin
	log *slog.Logger
}

// NewHandler builds the handler.
func NewHandler(svc *messaging.Admin, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the messaging admin endpoints.
//
// Both require a permission unconditionally, unlike the user endpoints:
// there is no owner of a dead letter for a self-service path to apply to.
func (h *Handler) Routes(requirePermission func(auth.Permission) func(http.Handler) http.Handler) func(chi.Router) {
	return func(r chi.Router) {
		r.With(requirePermission(auth.PermMessagingRead)).Get("/outbox/status", h.OutboxStatus)
		r.With(requirePermission(auth.PermMessagingRead)).Get("/dlq", h.ListDeadLetters)
		r.With(requirePermission(auth.PermMessagingReplay)).Post("/dlq/{id}/replay", h.ReplayDeadLetter)
	}
}

// OutboxStatus reports the outbox's depth and failure counts.
func (h *Handler) OutboxStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.svc.OutboxStatus(r.Context())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toOutboxStatusResponse(status))
}

// ListDeadLetters returns outstanding dead letters, newest first.
func (h *Handler) ListDeadLetters(w http.ResponseWriter, r *http.Request) {
	limit, err := int32Query(r, "limit", messaging.DefaultDeadLetterLimit)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	eventType := r.URL.Query().Get("event_type")

	page, err := h.svc.ListDeadLetters(r.Context(), eventType, limit)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toDeadLetterListResponse(page))
}

// ReplayDeadLetter puts one dead letter back into the outbox.
func (h *Handler) ReplayDeadLetter(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log,
			apperr.Invalid("INVALID_DEAD_LETTER_ID", "dead letter id must be a UUID"))
		return
	}

	if err := h.svc.ReplayDeadLetter(r.Context(), id); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusNoContent, nil)
}

// int32Query reads an optional integer query parameter. A malformed value is
// an error rather than a silent fallback, matching userhttp's intQuery:
// "?limit=abc" is a client bug, and answering the default hides it.
func int32Query(r *http.Request, name string, fallback int32) (int32, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, apperr.Invalid("INVALID_QUERY_PARAM",
			"query parameter must be an integer",
			apperr.Detail{Field: name, Reason: "not an integer"})
	}
	return int32(n), nil
}
