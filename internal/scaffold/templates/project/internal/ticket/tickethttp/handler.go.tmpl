package tickethttp

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
	"github.com/namezzy/goscaffoldforlevi/internal/ticket"
)

// Handler serves the ticket endpoints.
type Handler struct {
	svc *ticket.Service
	log *slog.Logger
}

// NewHandler builds the handler.
func NewHandler(svc *ticket.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the ticket endpoints.
//
// Assign, Reassign and the status overview require ticket:manage or
// ticket:read_all with no self-service path, so they are gated here —
// matching userhttp's Create/List/Delete. Everything else mixes ownership
// with permission (the requester, the current assignee, or a permission
// holder), which only the service has enough information to decide — see
// each Service method's own doc comment — so those carry no
// requirePermission and are reachable by anyone signed in; ticket:create,
// which List/Get/Create all rely on for "am I signed in at all", is held
// by every role.
func (h *Handler) Routes(requirePermission func(auth.Permission) func(http.Handler) http.Handler) func(chi.Router) {
	return func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.List)
		r.With(requirePermission(auth.PermTicketReadAll)).Get("/overview", h.StatusOverview)
		r.Get("/{id}", h.Get)
		r.Get("/{id}/activity", h.GetActivity)
		r.With(requirePermission(auth.PermTicketManage)).Post("/{id}/assign", h.Assign)
		r.With(requirePermission(auth.PermTicketManage)).Post("/{id}/reassign", h.Reassign)
		r.Post("/{id}/notes", h.ProcessNote)
		r.Post("/{id}/request-info", h.RequestInfo)
		r.Post("/{id}/provide-info", h.ProvideInfo)
		r.Post("/{id}/complete", h.Complete)
		r.Post("/{id}/close", h.Close)
	}
}

// Create submits a new ticket.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createTicketRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	created, err := h.svc.Create(r.Context(), req.toInput())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	w.Header().Set("Location", "/api/v1/tickets/"+created.ID)
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusCreated, toResponse(created))
}

// Get returns one ticket.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	found, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(found))
}

// List returns a filtered, paginated page of tickets, narrowed by the
// scope query parameter (mine, assigned or all — see ticket.Scope). Scope
// defaults to "mine": an unrecognised or absent scope must never fall back
// to showing every ticket in the organisation.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, err := intQuery(r, "page", 1)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	pageSize, err := intQuery(r, "page_size", ticket.DefaultPageSize)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	q := r.URL.Query()
	scope := ticket.Scope(q.Get("scope"))
	if scope == "" {
		scope = ticket.ScopeMine
	}
	f := ticket.ListFilter{
		Search:   q.Get("search"),
		SortBy:   ticket.SortBy(q.Get("sort")),
		SortDesc: q.Get("sort_dir") != "asc",
	}
	if raw := q.Get("status"); raw != "" {
		status := ticket.Status(raw)
		f.Status = &status
	}
	if raw := q.Get("priority"); raw != "" {
		priority := ticket.Priority(raw)
		f.Priority = &priority
	}

	result, err := h.svc.List(r.Context(), scope, f, page, pageSize)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toListResponse(result))
}

// StatusOverview reports how many tickets are in each status, for the
// admin workbench's ticket overview card.
func (h *Handler) StatusOverview(w http.ResponseWriter, r *http.Request) {
	counts, err := h.svc.StatusOverview(r.Context())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toStatusOverviewResponse(counts))
}

// GetActivity returns a ticket's timeline.
func (h *Handler) GetActivity(w http.ResponseWriter, r *http.Request) {
	records, err := h.svc.GetActivity(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toActivityResponse(records))
}

// Assign dispatches a pending ticket.
func (h *Handler) Assign(w http.ResponseWriter, r *http.Request) {
	var req assignRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	updated, err := h.svc.Assign(r.Context(), chi.URLParam(r, "id"), req.AssigneeID, req.ExpectedVersion)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
}

// Reassign moves an already-dispatched ticket to a different assignee.
func (h *Handler) Reassign(w http.ResponseWriter, r *http.Request) {
	var req assignRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	updated, err := h.svc.Reassign(r.Context(), chi.URLParam(r, "id"), req.AssigneeID, req.ExpectedVersion)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
}

// ProcessNote records a processing note without changing status.
func (h *Handler) ProcessNote(w http.ResponseWriter, r *http.Request) {
	var req noteRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	updated, err := h.svc.ProcessNote(r.Context(), chi.URLParam(r, "id"), req.Note, req.ExpectedVersion)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
}

// RequestInfo asks the requester for more information.
func (h *Handler) RequestInfo(w http.ResponseWriter, r *http.Request) {
	var req noteRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	updated, err := h.svc.RequestInfo(r.Context(), chi.URLParam(r, "id"), req.Note, req.ExpectedVersion)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
}

// ProvideInfo answers a needs_info request.
func (h *Handler) ProvideInfo(w http.ResponseWriter, r *http.Request) {
	var req noteRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	updated, err := h.svc.ProvideInfo(r.Context(), chi.URLParam(r, "id"), req.Note, req.ExpectedVersion)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
}

// Complete marks an in-progress ticket done.
func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	var req completeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	updated, err := h.svc.Complete(r.Context(), chi.URLParam(r, "id"), req.Note, req.ExpectedVersion)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
}

// Close ends a ticket's lifecycle.
func (h *Handler) Close(w http.ResponseWriter, r *http.Request) {
	var req closeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	updated, err := h.svc.Close(r.Context(), chi.URLParam(r, "id"), req.ExpectedVersion)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
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
