package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/auth"
)

func TestRolePermissions(t *testing.T) {
	t.Parallel()

	all := []auth.Permission{
		auth.PermUserCreate,
		auth.PermUserRead,
		auth.PermUserList,
		auth.PermUserUpdate,
		auth.PermUserDelete,
	}

	for _, p := range all {
		if !auth.RoleAdmin.Can(p) {
			t.Errorf("RoleAdmin.Can(%q) = false, want true", p)
		}
		// A plain user holds no permission over other people's records. Their
		// access to their own is ownership, which is a separate mechanism.
		if auth.RoleUser.Can(p) {
			t.Errorf("RoleUser.Can(%q) = true, want false", p)
		}
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
	perms := auth.RoleAdmin.Permissions()
	if len(perms) == 0 {
		t.Fatal("RoleAdmin has no permissions")
	}
	perms[0] = "user:everything"

	if !auth.RoleAdmin.Can(auth.PermUserCreate) {
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
			principal: &auth.Principal{UserID: self, Role: auth.RoleUser},
			target:    self,
			perm:      auth.PermUserRead,
			wantAllow: true,
		},
		{
			name:      "owner may not act on someone else",
			principal: &auth.Principal{UserID: self, Role: auth.RoleUser},
			target:    other,
			perm:      auth.PermUserRead,
			wantKind:  apperr.KindForbidden,
		},
		{
			name:      "permission holder may act on anyone",
			principal: &auth.Principal{UserID: self, Role: auth.RoleAdmin},
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
		auth.Principal{UserID: "someone", Role: auth.RoleUser})

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
