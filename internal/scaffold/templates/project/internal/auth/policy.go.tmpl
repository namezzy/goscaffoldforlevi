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

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
)

// Role is the label attached to an account and carried in an access token.
type Role string

// The closed set of roles.
//
// Phase 1 (organizations, invitations, audit) replaced the original two-role
// model (admin/user) with five fixed roles built around an enterprise
// directory: one still holds every permission, the rest are narrower slices
// of it. There is no "no role" baseline below RoleReadOnly — every account in
// a single-tenant deployment is a member of the one organization, so every
// account holds one of these five. An unknown value is not an error at parse
// time but holds no permissions, so a forged token naming a role that does
// not exist grants nothing.
const (
	// RoleSystemAdmin (系统管理员) holds every permission this policy defines.
	// It is the direct successor of the old RoleAdmin — see the
	// 20260914061450383_org_roles_audit migration, which maps every existing
	// "admin" row onto this value so current administrators keep exactly the
	// access they had.
	RoleSystemAdmin Role = "system_admin"
	// RoleMemberAdmin (成员管理员) manages the organization's people: their
	// accounts, department assignment, invitations and role assignment. It
	// does not see operational (messaging/DLQ) or audit data, and it cannot
	// delete an account or touch another system admin's standing — those stay
	// system-admin-only, as both are effectively irreversible.
	RoleMemberAdmin Role = "member_admin"
	// RoleOperator (运维人员) is unchanged in scope from before this phase: it
	// sees the messaging outbox/DLQ and the live request feed, and nothing
	// about organization membership.
	RoleOperator Role = "operator"
	// RoleReadOnly (只读观察者) can read the member directory and
	// organization/department structure, and nothing else. It is the default
	// role for an account that has not been given a more privileged one — see
	// the same migration for how existing non-admin accounts land here.
	RoleReadOnly Role = "read_only"
	// RoleAuditor (审计人员) can read the audit log (plus enough of the member
	// directory to resolve an actor or target's name) and nothing else. It
	// cannot act on members, invitations or roles.
	RoleAuditor Role = "auditor"
)

// allRoles backs Valid; keeping the check driven by rolePermissions' keys
// would make an entry with no permissions (a typo, say) silently "valid" by
// never being tested.
var allRoles = []Role{RoleSystemAdmin, RoleMemberAdmin, RoleOperator, RoleReadOnly, RoleAuditor}

// Valid reports whether r is one of the defined roles.
func (r Role) Valid() bool { return slices.Contains(allRoles, r) }

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

// The permissions the messaging feature defines, for the outbox/DLQ admin
// endpoints. There is no ownership variant here — a dead letter belongs to
// the system, not to a user — so unlike the user permissions these are
// checked unconditionally in the handler rather than left to the service.
const (
	PermMessagingRead   Permission = "messaging:read"
	PermMessagingReplay Permission = "messaging:replay"
)

// PermRequestsRead covers the live request feed (package requesttrack). It
// is its own permission rather than reuse of PermMessagingRead: the two
// guard unrelated data, and a role that should see one but not the other is
// one policy edit away rather than a refactor away.
const PermRequestsRead Permission = "requests:read"

// The permissions the org feature defines (package org): organization basic
// info and the department tree. There is one update permission for the
// organization record because it is a single row with no ownership concept —
// nobody "owns" the company name.
const (
	PermOrgRead         Permission = "org:read"
	PermOrgUpdate       Permission = "org:update"
	PermDepartmentRead  Permission = "department:read"
	PermDepartmentWrite Permission = "department:write"
)

// The permissions the invite feature defines (package invite). Accepting an
// invitation is deliberately not one of these: it is authorized by
// possession of the token itself, not by the accepting caller's role — most
// callers have none yet.
const (
	PermInviteCreate Permission = "invite:create"
	PermInviteRead   Permission = "invite:read"
	PermInviteRevoke Permission = "invite:revoke"
)

// PermRoleAssign covers changing which role a member holds. It is separate
// from PermUserUpdate even though a role change is technically a user-record
// update: PermUserUpdate lets member_admin fix a name or department, but
// granting or revoking a role is the more sensitive action, and keeping it
// its own permission means a future role that manages members without
// managing their privileges is a policy edit away.
const PermRoleAssign Permission = "role:assign"

// PermAuditRead covers the audit log (package audit). There is no write
// permission: audit records are produced as a side effect of other actions,
// never through a permission-gated write endpoint of their own — see
// audit.Recorder.
const PermAuditRead Permission = "audit:read"

// The permissions the ticket feature defines (package ticket).
//
// There is no PermTicketRead: reading a specific ticket is ownership-based
// (its requester or its assignee, like a user reading their own account) or
// covered by PermTicketReadAll, the same split PermUserRead/PermUserUpdate
// use for accounts. PermTicketManage implies PermTicketReadAll's access —
// assigning or closing a ticket requires seeing it — so a role granted the
// former is always granted the latter too, but they are kept as two
// permissions because a role that should see every ticket without being
// able to act on them (an auditor) is a real, current case, not a
// hypothetical one.
const (
	// PermTicketCreate is granted to every role: submitting a request is a
	// baseline capability of being a member of the organization, not a
	// privileged one.
	PermTicketCreate  Permission = "ticket:create"
	PermTicketReadAll Permission = "ticket:read_all"
	// PermTicketManage covers assign, reassign, and closing (or force-
	// closing) any ticket. Processing a ticket you are the assignee of —
	// adding a note, requesting info, completing it — needs no permission
	// at all beyond ownership; PermTicketManage is only for acting on a
	// ticket that is not yours.
	PermTicketManage Permission = "ticket:manage"
)

// The permissions the incident feature defines (package incident), for the
// exception/processing center. There is no per-incident ownership check the
// way ticket has: an incident belongs to operations as a whole, not to
// whoever happens to be its owner, so read access is a single blanket
// permission rather than split like PermTicketReadAll/PermTicketManage.
const (
	PermIncidentRead   Permission = "incident:read"
	PermIncidentManage Permission = "incident:manage"
)

// rolePermissions is the entire authorization policy. Keeping it in one map is
// the point: a reviewer can answer "what can a role do?" by reading this map
// instead of grepping the codebase for role comparisons.
var rolePermissions = map[Role][]Permission{
	RoleSystemAdmin: {
		PermUserCreate, PermUserRead, PermUserList, PermUserUpdate, PermUserDelete,
		PermMessagingRead, PermMessagingReplay,
		PermRequestsRead,
		PermOrgRead, PermOrgUpdate, PermDepartmentRead, PermDepartmentWrite,
		PermInviteCreate, PermInviteRead, PermInviteRevoke,
		PermRoleAssign,
		PermAuditRead,
		PermTicketCreate, PermTicketReadAll, PermTicketManage,
		PermIncidentRead, PermIncidentManage,
	},
	RoleMemberAdmin: {
		PermUserCreate, PermUserRead, PermUserList, PermUserUpdate,
		PermOrgRead, PermDepartmentRead, PermDepartmentWrite,
		PermInviteCreate, PermInviteRead, PermInviteRevoke,
		PermRoleAssign,
		PermTicketCreate, PermTicketReadAll, PermTicketManage,
	},
	RoleOperator: {
		PermMessagingRead, PermMessagingReplay,
		PermRequestsRead,
		PermTicketCreate,
		PermIncidentRead, PermIncidentManage,
	},
	RoleReadOnly: {
		PermUserRead, PermUserList,
		PermOrgRead, PermDepartmentRead,
		PermTicketCreate,
	},
	RoleAuditor: {
		PermUserRead, PermUserList,
		PermOrgRead,
		PermAuditRead,
		PermTicketCreate, PermTicketReadAll,
		PermIncidentRead,
	},
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

// GuardRoleGrant refuses to let anyone but a system administrator grant the
// system_admin role.
//
// PermRoleAssign is held by both system_admin and member_admin, because
// member_admin needs it to assign the four ordinary roles to members it
// manages. Without this separate check, that same permission would let a
// member_admin mint a new system_admin account — or promote itself to
// one — which is a privilege-escalation path, not a feature of "may assign
// roles". This is checked in addition to PermRoleAssign, never instead of
// it: an ordinary role change still needs the permission this refines.
func GuardRoleGrant(ctx context.Context, newRole Role) error {
	if newRole != RoleSystemAdmin {
		return nil
	}
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		return ErrMissingCredentials()
	}
	if principal.Role != RoleSystemAdmin {
		return ErrForbidden()
	}
	return nil
}

// GuardActionOnAccount refuses to let anyone but a system administrator —
// or the account's own owner — act on an account that currently holds
// RoleSystemAdmin: update its fields, reset its password, disable or
// delete it.
//
// Ordinary permissions like user:update are granted to member_admin so it
// can manage the members it is responsible for, but a system administrator
// account is never one of those: without this check, a caller holding only
// user:update could edit, lock out, or delete a system administrator's
// account. Ownership is exempted because a system administrator acting on
// their own record is not an escalation — GuardRoleGrant above is what
// stops them granting themselves that role in the first place.
func GuardActionOnAccount(ctx context.Context, targetUserID string, targetCurrentRole Role) error {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		return ErrMissingCredentials()
	}
	if principal.UserID == targetUserID || principal.Role == RoleSystemAdmin {
		return nil
	}
	if targetCurrentRole == RoleSystemAdmin {
		return ErrForbidden()
	}
	return nil
}
