package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
)

// allPermissions is every permission this policy defines. Kept in one place
// so TestRoleSystemAdminHoldsEveryPermission and TestRolePermissionMatrix's
// denied lists (built by subtracting a role's granted set from this one)
// cannot drift out of sync with policy.go by one forgotten entry.
var allPermissions = []auth.Permission{
	auth.PermUserCreate, auth.PermUserRead, auth.PermUserList, auth.PermUserUpdate, auth.PermUserDelete,
	auth.PermMessagingRead, auth.PermMessagingReplay,
	auth.PermRequestsRead,
	auth.PermOrgRead, auth.PermOrgUpdate, auth.PermDepartmentRead, auth.PermDepartmentWrite,
	auth.PermInviteCreate, auth.PermInviteRead, auth.PermInviteRevoke,
	auth.PermRoleAssign,
	auth.PermAuditRead,
	auth.PermTicketCreate, auth.PermTicketReadAll, auth.PermTicketManage,
	auth.PermIncidentRead, auth.PermIncidentManage,
}

func TestRoleSystemAdminHoldsEveryPermission(t *testing.T) {
	t.Parallel()

	for _, p := range allPermissions {
		if !auth.RoleSystemAdmin.Can(p) {
			t.Errorf("RoleSystemAdmin.Can(%q) = false, want true", p)
		}
	}
}

// TestRolePermissionMatrix is the explicit policy the other four fixed roles
// implement. It exists so that widening one role's access — say, giving
// RoleReadOnly a write permission by mistake while editing rolePermissions —
// fails a specific, readable assertion instead of only being noticed later
// against a real request.
func TestRolePermissionMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		role    auth.Role
		granted []auth.Permission
	}{
		{
			role: auth.RoleMemberAdmin,
			granted: []auth.Permission{
				auth.PermUserCreate, auth.PermUserRead, auth.PermUserList, auth.PermUserUpdate,
				auth.PermOrgRead, auth.PermDepartmentRead, auth.PermDepartmentWrite,
				auth.PermInviteCreate, auth.PermInviteRead, auth.PermInviteRevoke,
				auth.PermRoleAssign,
				auth.PermTicketCreate, auth.PermTicketReadAll, auth.PermTicketManage,
			},
		},
		{
			role: auth.RoleOperator,
			granted: []auth.Permission{
				auth.PermMessagingRead, auth.PermMessagingReplay, auth.PermRequestsRead,
				auth.PermTicketCreate,
				auth.PermIncidentRead, auth.PermIncidentManage,
			},
		},
		{
			role: auth.RoleReadOnly,
			granted: []auth.Permission{
				auth.PermUserRead, auth.PermUserList, auth.PermOrgRead, auth.PermDepartmentRead,
				auth.PermTicketCreate,
			},
		},
		{
			role: auth.RoleAuditor,
			granted: []auth.Permission{
				auth.PermUserRead, auth.PermUserList, auth.PermOrgRead, auth.PermAuditRead,
				auth.PermTicketCreate, auth.PermTicketReadAll,
				auth.PermIncidentRead,
			},
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.role), func(t *testing.T) {
			t.Parallel()

			grantedSet := make(map[auth.Permission]bool, len(tc.granted))
			for _, p := range tc.granted {
				grantedSet[p] = true
				if !tc.role.Can(p) {
					t.Errorf("%s.Can(%q) = false, want true", tc.role, p)
				}
			}
			for _, p := range allPermissions {
				if grantedSet[p] {
					continue
				}
				if tc.role.Can(p) {
					t.Errorf("%s.Can(%q) = true, want false", tc.role, p)
				}
			}
		})
	}
}

// TestOwnershipIsNotAPermission documents the boundary
// TestRolePermissionMatrix cannot express: even a role with none of the
// user:* permissions can still act on its own record, because that path is
// AuthorizeSelfOrPermission, not Role.Can. See TestAuthorizeSelfOrPermission
// below for the actual coverage of that mechanism.
func TestOwnershipIsNotAPermission(t *testing.T) {
	t.Parallel()

	if auth.RoleOperator.Can(auth.PermUserRead) {
		t.Fatal("RoleOperator unexpectedly holds PermUserRead — this test's premise no longer holds")
	}
}

func TestUnknownRoleHoldsNoPermissions(t *testing.T) {
	t.Parallel()

	// A role that is not in the policy must grant nothing. If the map returned
	// a default set, a forged or stale token naming an unrecognised role would
	// be an escalation.
	forged := auth.Role("superadmin")
	if forged.Can(auth.PermUserDelete) {
		t.Error("an unknown role was granted a permission")
	}
	if len(forged.Permissions()) != 0 {
		t.Errorf("Permissions() = %v, want empty", forged.Permissions())
	}
	if forged.Valid() {
		t.Error("Valid() = true for an undefined role")
	}
}

func TestPermissionsIsNotAliasedToThePolicy(t *testing.T) {
	t.Parallel()

	// Returning the backing slice would let any caller rewrite the policy for
	// the whole process.
	perms := auth.RoleSystemAdmin.Permissions()
	if len(perms) == 0 {
		t.Fatal("RoleSystemAdmin has no permissions")
	}
	perms[0] = "user:everything"

	if !auth.RoleSystemAdmin.Can(auth.PermUserCreate) {
		t.Error("mutating the returned slice changed the policy")
	}
}

func TestAuthorizeRequiresAPrincipal(t *testing.T) {
	t.Parallel()

	err := auth.Authorize(context.Background(), auth.PermUserList)
	if !apperr.Is(err, apperr.KindUnauthorized) {
		t.Errorf("Authorize() kind = %v, want unauthorized", apperr.KindOf(err))
	}
}

func TestAuthorizeSelfOrPermission(t *testing.T) {
	t.Parallel()

	const self = "01930f8e-0000-7000-8000-000000000001"
	const other = "01930f8e-0000-7000-8000-000000000002"

	cases := []struct {
		name      string
		principal *auth.Principal
		target    string
		perm      auth.Permission
		wantKind  apperr.Kind
		wantAllow bool
	}{
		{
			name:      "owner without the permission may act on themselves",
			principal: &auth.Principal{UserID: self, Role: auth.RoleOperator},
			target:    self,
			perm:      auth.PermUserRead,
			wantAllow: true,
		},
		{
			name:      "owner may not act on someone else",
			principal: &auth.Principal{UserID: self, Role: auth.RoleOperator},
			target:    other,
			perm:      auth.PermUserRead,
			wantKind:  apperr.KindForbidden,
		},
		{
			name:      "permission holder may act on anyone",
			principal: &auth.Principal{UserID: self, Role: auth.RoleSystemAdmin},
			target:    other,
			perm:      auth.PermUserRead,
			wantAllow: true,
		},
		{
			name:     "anonymous is unauthenticated, not forbidden",
			target:   self,
			perm:     auth.PermUserRead,
			wantKind: apperr.KindUnauthorized,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if tc.principal != nil {
				ctx = auth.ContextWithPrincipal(ctx, *tc.principal)
			}

			err := auth.AuthorizeSelfOrPermission(ctx, tc.target, tc.perm)
			if tc.wantAllow {
				if err != nil {
					t.Fatalf("AuthorizeSelfOrPermission() = %v, want nil", err)
				}
				return
			}
			if !apperr.Is(err, tc.wantKind) {
				t.Errorf("AuthorizeSelfOrPermission() kind = %v, want %v", apperr.KindOf(err), tc.wantKind)
			}
		})
	}
}

func TestEmptyPrincipalIsNotTreatedAsAuthenticated(t *testing.T) {
	t.Parallel()

	// A zero Principal in the context would have an empty UserID, which would
	// compare equal to an empty target ID and silently authorise the request.
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{})

	if _, ok := auth.PrincipalFrom(ctx); ok {
		t.Fatal("PrincipalFrom() accepted a zero principal")
	}
	if err := auth.AuthorizeSelfOrPermission(ctx, "", auth.PermUserRead); err == nil {
		t.Error("AuthorizeSelfOrPermission() allowed a zero principal to match an empty target")
	}
}

func TestForbiddenErrorDoesNotNameThePermission(t *testing.T) {
	t.Parallel()

	ctx := auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: "someone", Role: auth.RoleOperator})

	err := auth.Authorize(ctx, auth.PermUserDelete)
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("Authorize() = %v, want an *apperr.Error", err)
	}
	// Naming the missing permission tells an attacker exactly which capability
	// to hunt for.
	if strings.Contains(appErr.Message, string(auth.PermUserDelete)) {
		t.Errorf("error message %q discloses the required permission", appErr.Message)
	}
}

// TestGuardRoleGrantOnlyAdminMayGrantAdmin is the regression test for the
// escalation path where member_admin — which legitimately holds
// role:assign, to manage the four ordinary roles — could use that same
// permission to grant system_admin to any account, including its own.
func TestGuardRoleGrantOnlyAdminMayGrantAdmin(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		role      auth.Role
		newRole   auth.Role
		wantAllow bool
	}{
		{"member_admin granting system_admin is refused", auth.RoleMemberAdmin, auth.RoleSystemAdmin, false},
		{"member_admin granting member_admin is fine", auth.RoleMemberAdmin, auth.RoleMemberAdmin, true},
		{"member_admin granting read_only is fine", auth.RoleMemberAdmin, auth.RoleReadOnly, true},
		{"system_admin granting system_admin is fine", auth.RoleSystemAdmin, auth.RoleSystemAdmin, true},
		{"read_only granting system_admin is refused", auth.RoleReadOnly, auth.RoleSystemAdmin, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := auth.ContextWithPrincipal(context.Background(),
				auth.Principal{UserID: "actor", Role: tc.role})
			err := auth.GuardRoleGrant(ctx, tc.newRole)
			if tc.wantAllow && err != nil {
				t.Errorf("GuardRoleGrant() = %v, want nil", err)
			}
			if !tc.wantAllow && !apperr.Is(err, apperr.KindForbidden) {
				t.Errorf("GuardRoleGrant() kind = %v, want forbidden", apperr.KindOf(err))
			}
		})
	}
}

// TestGuardRoleGrantBlocksSelfPromotion is the specific case a self
// exemption would have missed: a member_admin acting on their own account
// must not be able to grant themselves system_admin, even though ownership
// is otherwise enough to explain most self-service actions.
func TestGuardRoleGrantBlocksSelfPromotion(t *testing.T) {
	t.Parallel()

	ctx := auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: "self", Role: auth.RoleMemberAdmin})
	err := auth.GuardRoleGrant(ctx, auth.RoleSystemAdmin)
	if !apperr.Is(err, apperr.KindForbidden) {
		t.Errorf("GuardRoleGrant() kind = %v, want forbidden", apperr.KindOf(err))
	}
}

// TestGuardActionOnAccountProtectsExistingAdmins is the regression test for
// the second half of the same escalation: even without granting
// system_admin outright, a member_admin holding the ordinary user:update
// permission could otherwise edit, lock out or delete an existing system
// administrator's account.
func TestGuardActionOnAccountProtectsExistingAdmins(t *testing.T) {
	t.Parallel()

	const actorID = "actor"
	const targetID = "target"

	cases := []struct {
		name        string
		actorRole   auth.Role
		targetID    string
		targetRole  auth.Role
		wantAllowed bool
	}{
		{"member_admin acting on a system_admin is refused", auth.RoleMemberAdmin, targetID, auth.RoleSystemAdmin, false},
		{"member_admin acting on an ordinary member is fine", auth.RoleMemberAdmin, targetID, auth.RoleReadOnly, true},
		{"system_admin acting on another system_admin is fine", auth.RoleSystemAdmin, targetID, auth.RoleSystemAdmin, true},
		{"a system_admin acting on their own account is fine", auth.RoleSystemAdmin, actorID, auth.RoleSystemAdmin, true},
		// The exemption is for acting on yourself, not for holding the
		// target role: a member_admin's own account is never system_admin
		// in practice, but if it somehow were, self-service still needs to
		// work rather than lock the account out of editing itself.
		{"acting on your own account is always fine", auth.RoleMemberAdmin, actorID, auth.RoleReadOnly, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := auth.ContextWithPrincipal(context.Background(),
				auth.Principal{UserID: actorID, Role: tc.actorRole})
			err := auth.GuardActionOnAccount(ctx, tc.targetID, tc.targetRole)
			if tc.wantAllowed && err != nil {
				t.Errorf("GuardActionOnAccount() = %v, want nil", err)
			}
			if !tc.wantAllowed && !apperr.Is(err, apperr.KindForbidden) {
				t.Errorf("GuardActionOnAccount() kind = %v, want forbidden", apperr.KindOf(err))
			}
		})
	}
}
