package authhttp

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
)

// loginRequest is the wire shape of a login. Request and response types are
// declared here, in the transport layer, rather than reusing domain structs:
// binding the API to the entity means every field added to the entity is
// published, and every field the API needs becomes a domain concern.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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

// Routes mounts the authentication endpoints.
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
	}
}

// Login exchanges credentials for a token pair.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}

	pair, err := h.svc.Login(r.Context(), req.Email, req.Password, h.meta(r))
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
	})
}
