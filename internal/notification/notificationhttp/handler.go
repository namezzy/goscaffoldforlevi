package notificationhttp

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
	"github.com/namezzy/goscaffoldforlevi/internal/notification"
)

// Handler serves the notification endpoints.
type Handler struct {
	svc *notification.Service
	log *slog.Logger
}

// NewHandler builds the handler.
func NewHandler(svc *notification.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the notification endpoints. None of them carry
// requirePermission: every one is scoped to the caller's own account by
// the service itself, which is the only authorization any of them need —
// see package notification's doc comment.
func (h *Handler) Routes() func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/", h.List)
		r.Get("/unread-count", h.UnreadCount)
		r.Post("/{id}/read", h.MarkRead)
		r.Post("/read-all", h.MarkAllRead)
	}
}

// List returns a page of the caller's own notifications.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	unreadOnly := q.Get("unread_only") == "true"

	page, err := intQuery(r, "page", 1)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	pageSize, err := intQuery(r, "page_size", notification.DefaultPageSize)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	result, err := h.svc.List(r.Context(), unreadOnly, page, pageSize)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toListResponse(result))
}

// UnreadCount returns how many of the caller's notifications are unread.
func (h *Handler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.UnreadCount(r.Context())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, unreadCountResponse{Count: count})
}

// MarkRead marks one notification read.
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.MarkRead(r.Context(), chi.URLParam(r, "id")); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusNoContent, nil)
}

// MarkAllRead marks every one of the caller's unread notifications read.
func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.MarkAllRead(r.Context()); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusNoContent, nil)
}

func intQuery(r *http.Request, name string, fallback int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, apperr.Invalid("INVALID_QUERY_PARAM",
			"query parameter must be an integer",
			apperr.Detail{Field: name, Reason: "not an integer"})
	}
	return n, nil
}
