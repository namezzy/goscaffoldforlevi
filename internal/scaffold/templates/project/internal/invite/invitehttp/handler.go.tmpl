package invitehttp

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
	"github.com/namezzy/goscaffoldforlevi/internal/invite"
)

// Handler serves the invitation endpoints.
type Handler struct {
	svc *invite.Service
	log *slog.Logger
}

// NewHandler builds the handler.
func NewHandler(svc *invite.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the invitation management endpoints — everything except
// Accept, which is not a management operation on an existing account and
// carries no principal to check a permission against. See AcceptRoute.
func (h *Handler) Routes(requirePermission func(auth.Permission) func(http.Handler) http.Handler) func(chi.Router) {
	return func(r chi.Router) {
		r.With(requirePermission(auth.PermInviteCreate)).Post("/", h.Create)
		r.With(requirePermission(auth.PermInviteRead)).Get("/", h.List)
		r.With(requirePermission(auth.PermInviteRead)).Get("/{id}", h.Get)
		r.With(requirePermission(auth.PermInviteRevoke)).Post("/{id}/revoke", h.Revoke)
		r.With(requirePermission(auth.PermInviteCreate)).Post("/{id}/reinvite", h.Reinvite)
	}
}

// AcceptRoute mounts Accept on its own, for the caller to register outside
// the authenticated route group — the same place POST /auth/login lives,
// since a caller redeeming an invitation has no account yet to authenticate
// as.
func (h *Handler) AcceptRoute() func(chi.Router) {
	return func(r chi.Router) {
		r.Post("/", h.Accept)
	}
}

// Create issues a new invitation.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createInvitationRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	created, token, err := h.svc.Invite(r.Context(), req.toInput())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	w.Header().Set("Location", "/api/v1/invitations/"+created.ID)
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusCreated, toCreatedResponse(created, token))
}

// Get returns one invitation.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	found, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toResponse(found))
}

// List returns a filtered, paginated page of invitations.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var f invite.ListFilter
	if raw := q.Get("status"); raw != "" {
		status := invite.Status(raw)
		f.Status = &status
	}
	f.Email = q.Get("email")

	page, err := intQuery(r, "page", 1)
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	pageSize, err := intQuery(r, "page_size", invite.DefaultPageSize)
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

// Revoke withdraws a pending invitation.
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Revoke(r.Context(), chi.URLParam(r, "id")); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusNoContent, nil)
}

// Reinvite revokes the existing invitation (if still pending) and issues a
// new one to the same email.
func (h *Handler) Reinvite(w http.ResponseWriter, r *http.Request) {
	created, token, err := h.svc.Reinvite(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	w.Header().Set("Location", "/api/v1/invitations/"+created.ID)
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusCreated, toCreatedResponse(created, token))
}

// Accept redeems a one-time invitation token and provisions the member's
// account.
func (h *Handler) Accept(w http.ResponseWriter, r *http.Request) {
	var req acceptInvitationRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	memberID, err := h.svc.Accept(r.Context(), req.toInput())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusCreated, acceptInvitationResponse{MemberID: memberID})
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
