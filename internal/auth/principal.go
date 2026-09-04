package auth

import (
	"context"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
)

// Principal is the authenticated caller as far as the rest of the application
// is concerned: an identity and a role, nothing else.
//
// It deliberately does not embed the user record. If the full user were in the
// context, handlers would start reading email or name from it, and every
// request would then need a database round trip to populate it. Keeping the
// principal to what the token already carries makes authentication free after
// signature verification.
type Principal struct {
	UserID string
	Role   Role
	// TokenID is the access token's jti. It is kept so that audit logs can tie
	// an action to a specific token rather than only to a user.
	TokenID string
}

type principalContextKey struct{}

// ContextWithPrincipal returns a context carrying the authenticated caller.
func ContextWithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, p)
}

// PrincipalFrom extracts the authenticated caller. The boolean is false for
// anonymous requests; callers must not treat the zero Principal as valid,
// because its zero Role holds no permissions but its zero UserID would match
// any equally empty target ID.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalContextKey{}).(Principal)
	if !ok || p.UserID == "" {
		return Principal{}, false
	}
	return p, true
}

// ErrMissingCredentials reports an unauthenticated request. It is distinct from
// ErrForbidden so the client can tell "log in" apart from "you may not do this",
// which is the only actionable difference between 401 and 403.
func ErrMissingCredentials() error {
	return apperr.Unauthorized("UNAUTHENTICATED", "authentication required")
}

// ErrInvalidToken reports a token that failed verification. The reason is never
// disclosed: distinguishing "expired" from "bad signature" from "wrong
// audience" turns the endpoint into an oracle for forging attempts.
func ErrInvalidToken() error {
	return apperr.Unauthorized("INVALID_TOKEN", "invalid or expired token")
}
