package incidenthttp

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
	"github.com/namezzy/goscaffoldforlevi/internal/incident"
)

// Handler serves the exception center endpoints.
type Handler struct {
	svc *incident.Service
	log *slog.Logger
}

// NewHandler builds the handler.
func NewHandler(svc *incident.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the exception center endpoints. Every route requires at
// least incident:read; the service itself enforces incident:manage on the
// mutating ones — see incident.Service's doc comments.
func (h *Handler) Routes(requirePermission func(auth.Permission) func(http.Handler) http.Handler) func(chi.Router) {
	return func(r chi.Router) {
		r.Use(requirePermission(auth.PermIncidentRead))
		r.Get("/", h.List)
		r.Get("/summary", h.Summary)
		r.Get("/{id}", h.Get)
		r.Get("/{id}/activity", h.GetActivity)
		r.Post("/{id}/severity", h.SetSeverity)
		r.Post("/{id}/owner", h.AssignOwner)
		r.Post("/{id}/acknowledge", h.Acknowledge)
		r.Post("/{id}/notes", h.ProcessNote)
		r.Post("/{id}/resolve", h.Resolve)
		r.Post("/{id}/replay", h.RequestReplay)
	}
}

// List returns a filtered, paginated page of incidents.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, err := intQuery(r, "page", 1)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	pageSize, err := intQuery(r, "page_size", incident.DefaultPageSize)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	q := r.URL.Query()
	f := incident.ListFilter{Search: q.Get("search")}
	if raw := q.Get("status"); raw != "" {
		status := incident.Status(raw)
		f.Status = &status
	}
	if raw := q.Get("severity"); raw != "" {
		severity := incident.Severity(raw)
		f.Severity = &severity
	}
	if raw := q.Get("owner_id"); raw != "" {
		f.OwnerID = &raw
	}

	result, err := h.svc.List(r.Context(), f, page, pageSize)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toListResponse(result))
}

// Summary reports how many non-resolved incidents sit at each severity.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	counts, err := h.svc.Summary(r.Context())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toSummaryResponse(counts))
}

// Get returns one incident.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	found, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(found))
}

// GetActivity returns an incident's processing record.
func (h *Handler) GetActivity(w http.ResponseWriter, r *http.Request) {
	records, err := h.svc.GetActivity(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toActivityResponse(records))
}

// SetSeverity re-triages an incident.
func (h *Handler) SetSeverity(w http.ResponseWriter, r *http.Request) {
	var req setSeverityRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	updated, err := h.svc.SetSeverity(r.Context(), chi.URLParam(r, "id"), incident.Severity(req.Severity))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
}

// AssignOwner sets or clears the incident's owner.
func (h *Handler) AssignOwner(w http.ResponseWriter, r *http.Request) {
	var req assignOwnerRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	updated, err := h.svc.AssignOwner(r.Context(), chi.URLParam(r, "id"), req.OwnerID)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
}

// Acknowledge marks an open incident seen.
func (h *Handler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	updated, err := h.svc.Acknowledge(r.Context(), chi.URLParam(r, "id"))
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
	updated, err := h.svc.ProcessNote(r.Context(), chi.URLParam(r, "id"), req.Note)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
}

// Resolve closes the incident's triage.
func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	updated, err := h.svc.Resolve(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
}

// RequestReplay submits the incident's underlying dead letter for replay.
// The response's replay_status is "pending" — submission, not a processed
// result; see incident.Service.RequestReplay's doc comment.
func (h *Handler) RequestReplay(w http.ResponseWriter, r *http.Request) {
	updated, err := h.svc.RequestReplay(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusAccepted, toResponse(updated))
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
