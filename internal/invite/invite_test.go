package invite_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/invite"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
)

// fakeRepository is an in-memory stand-in, keyed by invitation id. Writing
// it by hand rather than generating a mock keeps the behaviour under test —
// the service's authorization and its transaction boundaries — clear
// against a store that actually stores things.
type fakeRepository struct {
	byID map[string]invite.Invitation
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{byID: map[string]invite.Invitation{}}
}

func (f *fakeRepository) Create(_ context.Context, inv invite.Invitation, _ []byte) (invite.Invitation, error) {
	inv.CreatedAt = time.Now()
	inv.UpdatedAt = inv.CreatedAt
	f.byID[inv.ID] = inv
	return inv, nil
}

func (f *fakeRepository) GetByID(_ context.Context, id string) (invite.Invitation, error) {
	inv, ok := f.byID[id]
	if !ok {
		return invite.Invitation{}, apperr.NotFound("INVITATION_NOT_FOUND", "invitation not found")
	}
	return inv, nil
}

func (f *fakeRepository) GetByTokenHash(context.Context, []byte) (invite.Invitation, error) {
	return invite.Invitation{}, apperr.NotFound("INVITATION_NOT_FOUND", "invitation not found")
}

func (f *fakeRepository) ListFiltered(context.Context, invite.ListFilter, int32, int32) ([]invite.Invitation, error) {
	return nil, nil
}

func (f *fakeRepository) CountFiltered(context.Context, invite.ListFilter) (int64, error) {
	return 0, nil
}

func (f *fakeRepository) MarkAccepted(_ context.Context, id, acceptedUserID string) error {
	inv, ok := f.byID[id]
	if !ok || inv.Status != invite.StatusPending {
		return apperr.Conflict("INVITATION_NOT_PENDING", "this invitation is no longer pending")
	}
	inv.Status = invite.StatusAccepted
	inv.AcceptedUserID = &acceptedUserID
	f.byID[id] = inv
	return nil
}

func (f *fakeRepository) MarkExpired(_ context.Context, id string) error {
	inv, ok := f.byID[id]
	if !ok || inv.Status != invite.StatusPending {
		return apperr.Conflict("INVITATION_NOT_PENDING", "this invitation is no longer pending")
	}
	inv.Status = invite.StatusExpired
	f.byID[id] = inv
	return nil
}

func (f *fakeRepository) Revoke(_ context.Context, id string) error {
	inv, ok := f.byID[id]
	if !ok || inv.Status != invite.StatusPending {
		return apperr.Conflict("INVITATION_NOT_PENDING", "this invitation is no longer pending")
	}
	inv.Status = invite.StatusRevoked
	f.byID[id] = inv
	return nil
}

// rollbackTx snapshots the repository before running fn and restores it if
// fn fails, the same way internal/user/service_test.go's rollbackTx
// simulates a real transaction's rollback without a database. A nested
// WithTx call (create() called from inside Reinvite's own WithTx) snapshots
// again and, on failure, restores to that inner snapshot — then the outer
// call's own failure restores again to the outer snapshot, so the net
// effect for a caller is still a full rollback regardless of how many
// levels are nested. That is what lets this fake stand in for
// postgres.txManager's real "detect and reuse the ambient transaction"
// behaviour (ADR-0005) well enough for these tests.
type rollbackTx struct{ repo *fakeRepository }

func (t rollbackTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	before := maps.Clone(t.repo.byID)
	if err := fn(ctx); err != nil {
		t.repo.byID = before
		return err
	}
	return nil
}

type memoryPublisher struct {
	published []messaging.Event
	failWith  error
}

func (p *memoryPublisher) Publish(_ context.Context, event messaging.Event) error {
	if p.failWith != nil {
		return p.failWith
	}
	p.published = append(p.published, event)
	return nil
}

type nopAuditRecorder struct{ records []audit.Entry }

func (r *nopAuditRecorder) Record(_ context.Context, e audit.Entry) error {
	r.records = append(r.records, e)
	return nil
}

type stubHasher struct{}

func (stubHasher) Hash(password string) (string, error) { return "$stub$" + password, nil }

type stubProvisioner struct{ nextID string }

func (p stubProvisioner) Provision(context.Context, invite.ProvisionedMember) (string, error) {
	return p.nextID, nil
}

// plaintextCipher is a deliberately non-encrypting TokenEncrypter, used only
// by the tests that are not themselves testing encryption — it exists so
// those tests can assert on the token value directly instead of decrypting
// it first.
type plaintextCipher struct{}

func (plaintextCipher) Encrypt(plaintext string) (string, error) { return plaintext, nil }

func adminContext() context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: "admin-1", Role: auth.RoleSystemAdmin})
}

func memberAdminContext() context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		auth.Principal{UserID: "member-admin-1", Role: auth.RoleMemberAdmin})
}

func newService(repo *fakeRepository, events invite.EventPublisher, auditRecorder invite.AuditRecorder) *invite.Service {
	return invite.NewService(repo, rollbackTx{repo: repo}, events, auditRecorder,
		stubHasher{}, stubProvisioner{nextID: "new-member-1"}, plaintextCipher{}, time.Hour)
}

// TestMemberAdminCannotInviteASystemAdmin is the invite-side half of the
// escalation guard: invite:create is held by member_admin too, and without
// this check it could mint a system administrator account by invitation
// instead of by direct role assignment (see the identically-purposed test
// in internal/user for the direct-assignment half).
func TestMemberAdminCannotInviteASystemAdmin(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := newService(repo, &memoryPublisher{}, &nopAuditRecorder{})

	_, _, err := svc.Invite(memberAdminContext(), invite.NewInput{
		Email: "new-admin@example.com", Role: auth.RoleSystemAdmin,
	})
	if !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("Invite() kind = %v, want forbidden", apperr.KindOf(err))
	}
	if len(repo.byID) != 0 {
		t.Error("an invitation was created despite the refusal")
	}

	// A system administrator issuing the identical invitation is fine.
	if _, _, err := svc.Invite(adminContext(), invite.NewInput{
		Email: "new-admin@example.com", Role: auth.RoleSystemAdmin,
	}); err != nil {
		t.Errorf("Invite() by a system_admin = %v, want nil", err)
	}
}

// TestCreateEncryptsTheTokenInThePublishedEvent is the regression test for
// the token riding the outbox in plaintext: the event this package
// publishes must carry TokenEncrypter's output, not the raw value Accept
// will eventually need.
func TestCreateEncryptsTheTokenInThePublishedEvent(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	publisher := &memoryPublisher{}
	// A real cipher here, not plaintextCipher — this is the one test that
	// needs to see actual encryption happen.
	svc := invite.NewService(repo, rollbackTx{repo: repo}, publisher, &nopAuditRecorder{},
		stubHasher{}, stubProvisioner{}, invite.NewTokenCipher("test-secret-at-least-32-bytes-long"), time.Hour)

	_, plaintextToken, err := svc.Invite(adminContext(), invite.NewInput{
		Email: "invitee@example.com", Role: auth.RoleReadOnly,
	})
	if err != nil {
		t.Fatalf("Invite() = %v", err)
	}
	if len(publisher.published) != 1 {
		t.Fatalf("published %d events, want 1", len(publisher.published))
	}

	var payload invite.InvitationCreatedPayload
	if err := json.Unmarshal(publisher.published[0].Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Token == plaintextToken {
		t.Fatal("the published event carries the plaintext token")
	}
	if payload.Token == "" {
		t.Fatal("the published event carries no token at all")
	}

	decrypted, err := invite.NewTokenCipher("test-secret-at-least-32-bytes-long").Decrypt(payload.Token)
	if err != nil {
		t.Fatalf("Decrypt() = %v", err)
	}
	if decrypted != plaintextToken {
		t.Errorf("decrypted token = %q, want the one Invite() returned (%q)", decrypted, plaintextToken)
	}
}

// TestReinviteIsAtomicWithTheOldInvitationsRevocation is the regression
// test for the bug a code review found: Reinvite used to call Revoke
// directly, outside of and before create()'s own transaction, so a failure
// partway through create() (its outbox publish, say) left the old
// invitation revoked with no replacement — a link that had just been
// working was now silently dead, and the caller only saw an error with no
// indication that anything about the old invitation had changed.
func TestReinviteIsAtomicWithTheOldInvitationsRevocation(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	failingPublisher := &memoryPublisher{failWith: errors.New("outbox unavailable")}
	svc := newService(repo, failingPublisher, &nopAuditRecorder{})

	created, _, err := invite.NewService(repo, rollbackTx{repo: repo}, &memoryPublisher{}, &nopAuditRecorder{},
		stubHasher{}, stubProvisioner{}, plaintextCipher{}, time.Hour,
	).Invite(adminContext(), invite.NewInput{Email: "invitee@example.com", Role: auth.RoleReadOnly})
	if err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	_, _, err = svc.Reinvite(adminContext(), created.ID)
	if err == nil {
		t.Fatal("Reinvite() = nil, want the outbox failure to surface")
	}

	// The assertion: the original invitation must still be exactly as it
	// was — pending — not left revoked by a Reinvite call that ultimately
	// failed.
	prior, err := repo.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if prior.Status != invite.StatusPending {
		t.Errorf("original invitation status = %q after a failed reinvite, want %q (unchanged)",
			prior.Status, invite.StatusPending)
	}
}
