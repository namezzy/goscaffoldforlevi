// Package authhttp is the HTTP adapter for authentication: the middleware that
// turns a bearer token into a Principal, the middleware that enforces role
// permissions, and the login/refresh/logout handlers.
package authhttp

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/example/go-enterprise/internal/auth"
	"github.com/example/go-enterprise/internal/httpx"
)

// Authenticate verifies a bearer token and attaches the Principal.
//
// It rejects unauthenticated requests rather than passing them through as
// anonymous. An optional-authentication middleware makes every downstream
// handler responsible for remembering to check, and the failure mode of
// forgetting is an unauthenticated caller being served — so this middleware is
// mounted only on routes that require a caller.
func Authenticate(verifier auth.TokenVerifier, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				httpx.WriteError(r.Context(), w, log, auth.ErrMissingCredentials())
				return
			}

			claims, err := verifier.Verify(token)
			if err != nil {
				httpx.WriteError(r.Context(), w, log, err)
				return
			}

			ctx := auth.ContextWithPrincipal(r.Context(), auth.Principal{
				UserID:  claims.Subject,
				Role:    claims.Role,
				TokenID: claims.TokenID,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission enforces a role permission before the handler runs.
//
// This covers "may this kind of caller do this kind of thing". It cannot cover
// "may this caller touch this particular record", because the record is not
// known until the handler loads it; that half lives in the service layer via
// auth.AuthorizeSelfOrPermission. Splitting the two is what keeps role checks
// out of handler bodies.
func RequirePermission(p auth.Permission, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := auth.Authorize(r.Context(), p); err != nil {
				httpx.WriteError(r.Context(), w, log, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bearerToken extracts the credential from the Authorization header.
//
// The scheme comparison is case-insensitive because RFC 7235 says the scheme
// token is, and clients send "bearer" as often as "Bearer".
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}
	scheme, credentials, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	credentials = strings.TrimSpace(credentials)
	if credentials == "" {
		return "", false
	}
	return credentials, true
}
