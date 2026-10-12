package orghttp

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
	"github.com/namezzy/goscaffoldforlevi/internal/org"
)

// Handler serves the organization and department endpoints.
type Handler struct {
	svc *org.Service
	log *slog.Logger
}

// NewHandler builds the handler.
func NewHandler(svc *org.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the org endpoints. Every operation here is gated purely by
// role — there is no ownership concept for an organization or a department,
// unlike a user record — so every route carries requirePermission and none
// of the authorization is left to the service beyond that single check.
func (h *Handler) Routes(requirePermission func(auth.Permission) func(http.Handler) http.Handler) func(chi.Router) {
	return func(r chi.Router) {
		r.With(requirePermission(auth.PermOrgRead)).Get("/", h.GetOrganization)
		r.With(requirePermission(auth.PermOrgUpdate)).Patch("/", h.UpdateOrganization)
		r.With(requirePermission(auth.PermOrgRead)).Get("/notification-channels", h.NotificationChannels)
		r.With(requirePermission(auth.PermDepartmentRead)).Get("/departments", h.ListDepartments)
		r.With(requirePermission(auth.PermDepartmentWrite)).Post("/departments", h.CreateDepartment)
		r.With(requirePermission(auth.PermDepartmentWrite)).Patch("/departments/{id}", h.UpdateDepartment)
		r.With(requirePermission(auth.PermDepartmentWrite)).Delete("/departments/{id}", h.DeleteDepartment)
	}
}

// GetOrganization returns the company record.
func (h *Handler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	o, err := h.svc.GetOrganization(r.Context())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toOrgResponse(o))
}

// UpdateOrganization applies a partial update to the company record.
func (h *Handler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	var req updateOrganizationRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	updated, err := h.svc.UpdateOrganization(r.Context(), req.toPatch())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toOrgResponse(updated))
}

// NotificationChannels reports which notification delivery channels are
// currently configured.
func (h *Handler) NotificationChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.svc.NotificationChannels(r.Context())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toNotificationChannelListResponse(channels))
}

// ListDepartments returns every department.
func (h *Handler) ListDepartments(w http.ResponseWriter, r *http.Request) {
	ds, err := h.svc.ListDepartments(r.Context())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toDepartmentListResponse(ds))
}

// CreateDepartment adds a department to the tree.
func (h *Handler) CreateDepartment(w http.ResponseWriter, r *http.Request) {
	var req createDepartmentRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	created, err := h.svc.CreateDepartment(r.Context(), req.toInput())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	w.Header().Set("Location", "/api/v1/org/departments/"+created.ID)
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusCreated, toDepartmentResponse(created))
}

// UpdateDepartment applies a partial update, including reparenting.
func (h *Handler) UpdateDepartment(w http.ResponseWriter, r *http.Request) {
	var req updateDepartmentRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	updated, err := h.svc.UpdateDepartment(r.Context(), chi.URLParam(r, "id"), req.toPatch())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toDepartmentResponse(updated))
}

// DeleteDepartment removes a department.
func (h *Handler) DeleteDepartment(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteDepartment(r.Context(), chi.URLParam(r, "id")); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusNoContent, nil)
}
