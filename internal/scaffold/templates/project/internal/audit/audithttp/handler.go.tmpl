package audithttp

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
)

// Handler serves the audit endpoints.
type Handler struct {
	svc *audit.Service
	log *slog.Logger
}

// NewHandler builds the handler.
func NewHandler(svc *audit.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the audit endpoints. Both are gated by PermAuditRead at the
// router: unlike users, an audit record has no ownership concept for a
// self-service path to fall back to, so there is nothing for the service
// layer to decide beyond that permission.
func (h *Handler) Routes(requirePermission func(auth.Permission) func(http.Handler) http.Handler) func(chi.Router) {
	return func(r chi.Router) {
		r.With(requirePermission(auth.PermAuditRead)).Get("/", h.List)
		r.With(requirePermission(auth.PermAuditRead)).Get("/{id}", h.Get)
	}
}

// List returns a filtered, paginated page of the audit log.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var f audit.Filter
	if raw := q.Get("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpx.WriteError(r.Context(), w, h.log, apperr.Invalid("INVALID_QUERY_PARAM",
				"from must be RFC 3339", apperr.Detail{Field: "from", Reason: "not RFC 3339"}))
			return
		}
		f.From = &t
	}
	if raw := q.Get("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpx.WriteError(r.Context(), w, h.log, apperr.Invalid("INVALID_QUERY_PARAM",
				"to must be RFC 3339", apperr.Detail{Field: "to", Reason: "not RFC 3339"}))
			return
		}
		f.To = &t
	}
	if raw := q.Get("actor_id"); raw != "" {
		f.ActorID = &raw
	}
	if raw := q.Get("action"); raw != "" {
		f.Action = &raw
	}
	if raw := q.Get("result"); raw != "" {
		res := audit.Result(raw)
		f.Result = &res
	}
	if raw := q.Get("target_type"); raw != "" {
		f.TargetType = &raw
	}
	if raw := q.Get("target_id"); raw != "" {
		f.TargetID = &raw
	}

	page, err := intQuery(r, "page", 1)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	pageSize, err := intQuery(r, "page_size", audit.DefaultPageSize)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	result, err := h.svc.List(r.Context(), f, page, pageSize)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toListResponse(result))
}

// Get returns one audit event.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	found, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(found))
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
