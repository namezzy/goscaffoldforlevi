package userhttp

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/auth"
	"github.com/example/go-enterprise/internal/httpx"
	"github.com/example/go-enterprise/internal/user"
)

// Handler serves the user endpoints.
type Handler struct {
	svc *user.Service
	log *slog.Logger
}

// NewHandler builds the handler.
func NewHandler(svc *user.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the user endpoints.
//
// Only the checks that depend solely on the caller's role are applied as
// middleware. Read and update carry no RequirePermission, because a user may
// perform them on their own record — that decision needs the target's identity
// and is made in the service. List, create and delete have no such
// self-service path, so they are gated here and never reach the service
// without the permission.
func (h *Handler) Routes(requirePermission func(auth.Permission) func(http.Handler) http.Handler) func(chi.Router) {
	return func(r chi.Router) {
		r.With(requirePermission(auth.PermUserCreate)).Post("/", h.Create)
		r.With(requirePermission(auth.PermUserList)).Get("/", h.List)
		r.Get("/{id}", h.Get)
		r.Patch("/{id}", h.Update)
		r.Put("/{id}/password", h.ChangePassword)
		r.With(requirePermission(auth.PermUserDelete)).Delete("/{id}", h.Delete)
	}
}

// Create registers a user and answers 201 with a Location header.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	created, err := h.svc.Create(r.Context(), req.toInput())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	w.Header().Set("Location", "/api/v1/users/"+created.ID.String())
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusCreated, toResponse(created))
}

// Get returns one user.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := user.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	found, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(found))
}

// List returns a page of users.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, err := intQuery(r, "page", 1)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	pageSize, err := intQuery(r, "page_size", user.DefaultPageSize)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	result, err := h.svc.List(r.Context(), page, pageSize)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toListResponse(result))
}

// Update applies a partial update.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := user.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	var req updateUserRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	updated, err := h.svc.Update(r.Context(), id, req.toPatch())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(updated))
}

// ChangePassword replaces a user's password.
//
// It is a separate endpoint rather than a field on PATCH so that the plaintext
// never travels in the same body as ordinary attributes, where a debug dump of
// the request would print it alongside the name.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	id, err := user.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	var req changePasswordRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	if err := h.svc.ChangePassword(r.Context(), id, req.Password); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusNoContent, nil)
}

// Delete removes a user.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := user.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusNoContent, nil)
}

// intQuery reads an optional integer query parameter.
//
// A malformed value is an error rather than a silent fallback to the default:
// "?page=abc" is a client bug, and answering page 1 hides it until someone
// notices the pagination never advances.
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
