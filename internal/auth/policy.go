// Package auth owns authentication and authorization: who the caller is, and
// what that lets them do.
//
// It deliberately does not import the user package. Authentication needs only a
// narrow view of an account (identity, password hash, role, whether it is
// usable), so it defines that view itself and lets the user package supply an
// adapter. The dependency therefore runs user -> auth, never both ways.
package auth

import (
	"context"
	"slices"

	"github.com/example/go-enterprise/internal/apperr"
)

// Role is the label attached to an account and carried in an access token.
type Role string

// The closed set of roles. An unknown value is not an error at parse time but
// holds no permissions, so a forged token naming a role that does not exist
// grants nothing.
const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// Valid reports whether r is one of the defined roles.
func (r Role) Valid() bool { return r == RoleAdmin || r == RoleUser }

func (r Role) String() string { return string(r) }

// Permission is a single capability. Handlers and services ask for permissions,
// never for roles: a permission is stable ("may delete a user") while the set of
// roles that holds it is a policy decision that changes.
type Permission string

// The permissions the user feature defines.
const (
	PermUserCreate Permission = "user:create"
	PermUserRead   Permission = "user:read"
	PermUserList   Permission = "user:list"
	PermUserUpdate Permission = "user:update"
	PermUserDelete Permission = "user:delete"
)

// rolePermissions is the entire authorization policy. Keeping it in one map is
// the point: a reviewer can answer "what can an admin do?" by reading five
// lines instead of grepping the codebase for role comparisons.
//
// RoleUser is intentionally empty. A normal user can still read and update
// their own record, but that is ownership, not a permission — see
// AuthorizeSelfOrPermission.
var rolePermissions = map[Role][]Permission{
	RoleAdmin: {PermUserCreate, PermUserRead, PermUserList, PermUserUpdate, PermUserDelete},
	RoleUser:  {},
}

// Can reports whether the role holds the permission. An unknown role holds
// nothing, so a corrupted or forged token cannot grant access by naming a role
// that does not exist.
func (r Role) Can(p Permission) bool {
	return slices.Contains(rolePermissions[r], p)
}

// Permissions returns the role's capabilities. The copy prevents a caller from
// editing the policy through the returned slice.
func (r Role) Permissions() []Permission {
	return slices.Clone(rolePermissions[r])
}

// ErrForbidden is the single response to every authorization failure. It never
// says which permission was missing: telling an attacker exactly which
// capability to hunt for is free reconnaissance.
func ErrForbidden() error {
	return apperr.Forbidden("FORBIDDEN", "you do not have permission to perform this action")
}

// Authorize checks a permission for the principal in ctx.
func Authorize(ctx context.Context, p Permission) error {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		return ErrMissingCredentials()
	}
	if !principal.Role.Can(p) {
		return ErrForbidden()
	}
	return nil
}

// AuthorizeSelfOrPermission allows an action when the caller is acting on their
// own record, or when they hold the permission that covers acting on anyone's.
//
// This check cannot live in middleware. Middleware runs before the target of
// the request is known — for a nested or filtered resource the owner is only
// discovered by loading it — so ownership is a service-layer concern while
// role permissions are a transport-layer one. Conflating them either forces the
// router to know about the domain or leaves ownership unenforced.
func AuthorizeSelfOrPermission(ctx context.Context, targetUserID string, p Permission) error {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		return ErrMissingCredentials()
	}
	if principal.UserID == targetUserID {
		return nil
	}
	if principal.Role.Can(p) {
		return nil
	}
	return ErrForbidden()
}
