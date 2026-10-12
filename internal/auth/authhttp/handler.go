package authhttp

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
)

// loginRequest is the wire shape of a login. Request and response types are
// declared here, in the transport layer, rather than reusing domain structs:
// binding the API to the entity means every field added to the entity is
// published, and every field the API needs becomes a domain concern.
type loginRequest struct {
	// Identifier is an email address or a username; the service does not
	// need to know which.
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// tokenResponse is returned by login and refresh.
//
// token_type is spelled out even though it is always "Bearer": clients that
// build the Authorization header from the response should not have to hardcode
// it, and it is what OAuth-shaped client libraries expect to find.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	// SessionID identifies which of the account's sessions this pair
	// belongs to — see auth.TokenPair.SessionID's doc comment. The
	// security center's session list uses it to mark the current row
	// without decoding the access token.
	SessionID string `json:"session_id"`
}

// sessionResponse is the public representation of one of the caller's own
// sessions.
type sessionResponse struct {
	ID string `json:"id"`
	// Current marks the session the request currently authenticating this
	// call is itself part of.
	Current   bool       `json:"current"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	UserAgent *string    `json:"user_agent,omitempty"`
	IP        *string    `json:"ip,omitempty"`
}

func toSessionResponse(s auth.Session, currentFamily string) sessionResponse {
	// ID is the family id, not the row's own id: one session, in this
	// list's terms, is one login lineage — see
	// queries/auth.sql's ListRecentRefreshSessionsByUser for why — and the
	// family id is also what TokenPair.SessionID/session_id already hands
	// the client at login, so the two line up without a lookup.
	resp := sessionResponse{
		ID: s.FamilyID.String(), Current: s.FamilyID.String() == currentFamily,
		CreatedAt: s.CreatedAt, ExpiresAt: s.ExpiresAt, RevokedAt: s.RevokedAt, UserAgent: s.UserAgent,
	}
	if s.IP != nil {
		ip := s.IP.String()
		resp.IP = &ip
	}
	return resp
}

type sessionListResponse struct {
	// Items is always an array; an empty list is [], never null.
	Items []sessionResponse `json:"items"`
}

type revokeOtherSessionsResponse struct {
	SessionsRevoked int64 `json:"sessions_revoked"`
}

type requestPasswordResetRequest struct {
	Identifier string `json:"identifier"`
}

type confirmPasswordResetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// Handler serves the authentication endpoints.
type Handler struct {
	svc *auth.Service
	log *slog.Logger
	// trustedProxies is needed because the session records the client IP, and
	// recording a forgeable one would make the audit trail worse than useless.
	trustedProxies int
}

// NewHandler builds the handler.
func NewHandler(svc *auth.Service, trustedProxies int, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log, trustedProxies: trustedProxies}
}

// Routes mounts the authentication endpoints that require no prior
// credential: login by definition, and — for the same reason —
// refresh, logout, and both password-reset endpoints. A caller requesting
// a reset or redeeming a reset token has no session to authenticate with
// yet, which is the whole point of the flow.
//
// loginLimiter is applied only to POST /login. It is passed in rather than
// built here so that this package does not depend on the rate limiter, and so
// that the composition root remains the single place where the middleware order
// is visible.
func (h *Handler) Routes(loginLimiter func(http.Handler) http.Handler) func(chi.Router) {
	return func(r chi.Router) {
		r.With(loginLimiter).Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
		r.Post("/password-reset/request", h.RequestPasswordReset)
		r.Post("/password-reset/confirm", h.ConfirmPasswordReset)
	}
}

// SessionRoutes mounts the security center's session-management endpoints.
// Mounted separately from Routes, inside the protected route group in
// cmd/api (unlike everything Routes serves), because every one of these is
// self-service: they need to know who the caller is.
func (h *Handler) SessionRoutes() func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/", h.ListSessions)
		r.Delete("/{id}", h.RevokeSession)
		r.Post("/revoke-others", h.RevokeOtherSessions)
	}
}

// Login exchanges credentials for a token pair.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	pair, err := h.svc.Login(r.Context(), req.Identifier, req.Password, h.meta(r))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	writeTokens(r, w, h.log, pair)
}

// Refresh rotates a refresh token.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	pair, err := h.svc.Refresh(r.Context(), req.RefreshToken, h.meta(r))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	writeTokens(r, w, h.log, pair)
}

// Logout revokes the token's whole session family.
//
// It answers 204 whether or not the token existed. Logout is unauthenticated by
// necessity — the access token may already have expired — so reporting "no such
// session" would let anyone probe which refresh tokens are live.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusNoContent, nil)
}

// RequestPasswordReset queues a reset email if the identifier matches an
// account. The response is 202 regardless — see
// auth.Service.RequestPasswordReset's doc comment on why it can never
// reveal whether it matched.
func (h *Handler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req requestPasswordResetRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	if err := h.svc.RequestPasswordReset(r.Context(), req.Identifier); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusAccepted, nil)
}

// ConfirmPasswordReset redeems a one-time token and sets a new password.
func (h *Handler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req confirmPasswordResetRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	if err := h.svc.ConfirmPasswordReset(r.Context(), req.Token, req.Password); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusNoContent, nil)
}

// ListSessions returns the caller's own recent sessions.
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.svc.ListSessions(r.Context())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	principal, _ := auth.PrincipalFrom(r.Context())
	items := make([]sessionResponse, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, toSessionResponse(s, principal.TokenID))
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, sessionListResponse{Items: items})
}

// RevokeSession revokes one of the caller's own sessions.
func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	id, err := auth.ParseFamilyID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	if err := h.svc.RevokeSession(r.Context(), id); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusNoContent, nil)
}

// RevokeOtherSessions revokes every one of the caller's sessions except the
// one this request is authenticated through.
func (h *Handler) RevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.RevokeOtherSessions(r.Context())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, revokeOtherSessionsResponse{SessionsRevoked: n})
}

func (h *Handler) meta(r *http.Request) auth.RequestMeta {
	return auth.RequestMeta{
		UserAgent: r.UserAgent(),
		IP:        httpx.ClientIP(r, h.trustedProxies),
	}
}

func writeTokens(r *http.Request, w http.ResponseWriter, log *slog.Logger, pair auth.TokenPair) {
	// Tokens must never be cached. Without these headers a shared proxy or a
	// browser back-button can serve one user's tokens to the next request.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	httpx.WriteJSON(r.Context(), w, log, http.StatusOK, tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    pair.ExpiresIn,
		SessionID:    pair.SessionID,
	})
}
