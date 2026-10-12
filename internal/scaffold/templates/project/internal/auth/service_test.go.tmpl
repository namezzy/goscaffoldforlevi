package auth_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
)

// The fakes below are hand-written rather than generated. Each is a few lines,
// and a generated mock would express the same behaviour in a language of
// expectations that has to be read separately from the test.

type fakeAccounts struct {
	byEmail map[string]auth.Account
	byID    map[string]auth.Account
	updated map[string]string
	err     error
}

func newFakeAccounts(accounts ...auth.Account) *fakeAccounts {
	f := &fakeAccounts{
		byEmail: make(map[string]auth.Account),
		byID:    make(map[string]auth.Account),
		updated: make(map[string]string),
	}
	for _, a := range accounts {
		f.byID[a.UserID] = a
	}
	return f
}

func (f *fakeAccounts) with(email string, a auth.Account) *fakeAccounts {
	f.byEmail[email] = a
	f.byID[a.UserID] = a
	return f
}

func (f *fakeAccounts) FindByIdentifier(_ context.Context, identifier string) (auth.Account, error) {
	if f.err != nil {
		return auth.Account{}, f.err
	}
	a, ok := f.byEmail[identifier]
	if !ok {
		return auth.Account{}, apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	return a, nil
}

func (f *fakeAccounts) FindByID(_ context.Context, id string) (auth.Account, error) {
	a, ok := f.byID[id]
	if !ok {
		return auth.Account{}, apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	return a, nil
}

func (f *fakeAccounts) UpdatePasswordHash(_ context.Context, userID, hash string) error {
	f.updated[userID] = hash
	return nil
}

type fakeSessions struct {
	rows []auth.Session
	now  func() time.Time
}

func (f *fakeSessions) Create(_ context.Context, s auth.NewSession) (auth.Session, error) {
	row := auth.Session{
		ID:        s.ID,
		UserID:    s.UserID,
		TokenHash: s.TokenHash,
		FamilyID:  s.FamilyID,
		ParentID:  s.ParentID,
		ExpiresAt: s.ExpiresAt,
		CreatedAt: f.now(),
		UserAgent: s.UserAgent,
		IP:        s.IP,
	}
	f.rows = append(f.rows, row)
	return row, nil
}

func (f *fakeSessions) GetByTokenHash(_ context.Context, hash []byte) (auth.Session, error) {
	for _, row := range f.rows {
		if bytes.Equal(row.TokenHash, hash) {
			return row, nil
		}
	}
	return auth.Session{}, apperr.NotFound("SESSION_NOT_FOUND", "session not found")
}

func (f *fakeSessions) Revoke(_ context.Context, id auth.SessionID) error {
	for i := range f.rows {
		if f.rows[i].ID == id && f.rows[i].RevokedAt == nil {
			at := f.now()
			f.rows[i].RevokedAt = &at
		}
	}
	return nil
}

func (f *fakeSessions) RevokeFamily(_ context.Context, id auth.FamilyID) (int64, error) {
	var n int64
	for i := range f.rows {
		if f.rows[i].FamilyID == id && f.rows[i].RevokedAt == nil {
			at := f.now()
			f.rows[i].RevokedAt = &at
			n++
		}
	}
	return n, nil
}

func (f *fakeSessions) activeCount() int {
	n := 0
	for _, row := range f.rows {
		if row.RevokedAt == nil {
			n++
		}
	}
	return n
}

// ListRecentByUser collapses to one row per family — its most recent row —
// matching queries/auth.sql's ListRecentRefreshSessionsByUser exactly:
// a "session" in this list's terms is one login lineage, not one
// refresh-token rotation.
func (f *fakeSessions) ListRecentByUser(_ context.Context, userID string, limit int32) ([]auth.Session, error) {
	latest := map[auth.FamilyID]auth.Session{}
	for _, row := range f.rows {
		if row.UserID != userID {
			continue
		}
		if current, ok := latest[row.FamilyID]; !ok || row.CreatedAt.After(current.CreatedAt) {
			latest[row.FamilyID] = row
		}
	}
	out := make([]auth.Session, 0, len(latest))
	for _, row := range latest {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if int32(len(out)) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeSessions) RevokeOwned(_ context.Context, family auth.FamilyID, userID string) (bool, error) {
	var revoked bool
	for i := range f.rows {
		if f.rows[i].FamilyID == family && f.rows[i].UserID == userID && f.rows[i].RevokedAt == nil {
			at := f.now()
			f.rows[i].RevokedAt = &at
			revoked = true
		}
	}
	return revoked, nil
}

func (f *fakeSessions) RevokeAllForUserExceptFamily(_ context.Context, userID string, exceptFamily auth.FamilyID) (int64, error) {
	var n int64
	for i := range f.rows {
		if f.rows[i].UserID == userID && f.rows[i].FamilyID != exceptFamily && f.rows[i].RevokedAt == nil {
			at := f.now()
			f.rows[i].RevokedAt = &at
			n++
		}
	}
	return n, nil
}

func (f *fakeSessions) RevokeAllForUser(_ context.Context, userID string) (int64, error) {
	var n int64
	for i := range f.rows {
		if f.rows[i].UserID == userID && f.rows[i].RevokedAt == nil {
			at := f.now()
			f.rows[i].RevokedAt = &at
			n++
		}
	}
	return n, nil
}

// fakeResets is a hand-written PasswordResetRepository, matching
// fakeSessions' style.
type fakeResets struct {
	rows map[string]auth.PasswordResetToken
	now  func() time.Time
}

func newFakeResets(now func() time.Time) *fakeResets {
	return &fakeResets{rows: make(map[string]auth.PasswordResetToken), now: now}
}

func (f *fakeResets) Create(_ context.Context, t auth.NewPasswordResetToken) error {
	f.rows[t.ID] = auth.PasswordResetToken{
		ID: t.ID, UserID: t.UserID, TokenHash: t.TokenHash, ExpiresAt: t.ExpiresAt, CreatedAt: f.now(),
	}
	return nil
}

func (f *fakeResets) GetByTokenHash(_ context.Context, hash []byte) (auth.PasswordResetToken, error) {
	for _, row := range f.rows {
		if bytes.Equal(row.TokenHash, hash) {
			return row, nil
		}
	}
	return auth.PasswordResetToken{}, apperr.NotFound("PASSWORD_RESET_TOKEN_NOT_FOUND", "password reset token not found")
}

func (f *fakeResets) MarkUsed(_ context.Context, id string) (bool, error) {
	row, ok := f.rows[id]
	if !ok || row.UsedAt != nil {
		return false, nil
	}
	at := f.now()
	row.UsedAt = &at
	f.rows[id] = row
	return true, nil
}

func (f *fakeResets) InvalidatePendingForUser(_ context.Context, userID string) (int64, error) {
	var n int64
	for id, row := range f.rows {
		if row.UserID == userID && row.UsedAt == nil {
			delete(f.rows, id)
			n++
		}
	}
	return n, nil
}

// plaintextCipher is a no-op TokenEncrypter, matching invite's own test
// fixture: what matters to these tests is that the service calls the
// cipher, not that AES-GCM in particular runs (that is TokenCipher's own
// test's job).
type plaintextCipher struct{}

func (plaintextCipher) Encrypt(s string) (string, error) { return s, nil }

// rollbackTx models the one property of a real transaction that changes the
// meaning of the code under test: an error discards every write made inside it.
//
// An earlier version of this fake simply called fn and returned its error. That
// looks harmless and is not: reuse detection revokes the token family and then
// returns an error to the caller, so under a real transaction the revocation
// was rolled back and the stolen family stayed live. The fake passed, the
// database did not. A fake that omits the behaviour a caller depends on is
// worse than no test, because it reports success.
type rollbackTx struct {
	sessions *fakeSessions
	resets   *fakeResets
}

func (t rollbackTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	sessionsSnapshot := append([]auth.Session(nil), t.sessions.rows...)
	var resetsSnapshot map[string]auth.PasswordResetToken
	if t.resets != nil {
		resetsSnapshot = make(map[string]auth.PasswordResetToken, len(t.resets.rows))
		for k, v := range t.resets.rows {
			resetsSnapshot[k] = v
		}
	}
	if err := fn(ctx); err != nil {
		t.sessions.rows = sessionsSnapshot
		if t.resets != nil {
			t.resets.rows = resetsSnapshot
		}
		return err
	}
	return nil
}

// staticIssuer produces a predictable token so tests can assert on the claims
// the service passes in rather than on a signed blob.
type staticIssuer struct{ last auth.AccessClaims }

func (s *staticIssuer) Issue(c auth.AccessClaims) (string, error) {
	s.last = c
	return "access-token-for-" + c.Subject, nil
}

type serviceFixture struct {
	svc      *auth.Service
	accounts *fakeAccounts
	sessions *fakeSessions
	resets   *fakeResets
	events   *messaging.MemoryPublisher
	issuer   *staticIssuer
	now      time.Time
}

func newServiceFixture(t *testing.T, accounts *fakeAccounts) *serviceFixture {
	t.Helper()

	f := &serviceFixture{
		accounts: accounts,
		issuer:   &staticIssuer{},
		events:   messaging.NewMemoryPublisher(),
		now:      time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
	}
	f.sessions = &fakeSessions{now: func() time.Time { return f.now }}
	f.resets = newFakeResets(func() time.Time { return f.now })

	f.svc = auth.NewService(
		auth.ServiceConfig{
			Issuer:           "test-issuer",
			Audience:         "test-audience",
			AccessTTL:        15 * time.Minute,
			RefreshTTL:       30 * 24 * time.Hour,
			PasswordResetTTL: time.Hour,
		},
		accounts,
		f.sessions,
		f.resets,
		plaintextCipher{},
		newTestHasher(t),
		f.issuer,
		f.events,
		rollbackTx{sessions: f.sessions, resets: f.resets},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	f.svc.SetClock(func() time.Time { return f.now })
	return f
}

const testPassword = "correct horse battery staple"

func accountFor(t *testing.T, userID string, active bool) auth.Account {
	t.Helper()
	hash, err := newTestHasher(t).Hash(testPassword)
	if err != nil {
		t.Fatalf("Hash() = %v", err)
	}
	return auth.Account{UserID: userID, PasswordHash: hash, Role: auth.RoleReadOnly, Active: active}
}

func TestLoginIssuesATokenPair(t *testing.T) {
	t.Parallel()

	account := accountFor(t, "user-1", true)
	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", account))

	pair, err := f.svc.Login(context.Background(), "A@Example.COM", testPassword, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Login() = %v", err)
	}

	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("Login() returned an incomplete token pair")
	}
	if pair.ExpiresIn != int((15 * time.Minute).Seconds()) {
		t.Errorf("ExpiresIn = %d, want 900", pair.ExpiresIn)
	}
	// The jti is what lets an audited action be tied to one token.
	if f.issuer.last.TokenID == "" {
		t.Error("access claims have no token id")
	}
	if f.issuer.last.Subject != "user-1" {
		t.Errorf("subject = %q, want user-1", f.issuer.last.Subject)
	}
	if len(f.sessions.rows) != 1 {
		t.Fatalf("recorded %d sessions, want 1", len(f.sessions.rows))
	}
	// Only the digest is stored; the token itself must not be recoverable.
	if bytes.Equal(f.sessions.rows[0].TokenHash, []byte(pair.RefreshToken)) {
		t.Error("the refresh token was stored verbatim")
	}
}

// TestLoginAcceptsAUsernameAsTheIdentifier is the reason the field is called
// identifier rather than email: AccountStore.FindByIdentifier resolves
// whatever it is given against both columns, and the service does not need
// to know, or normalise differently, which kind of value arrived.
func TestLoginAcceptsAUsernameAsTheIdentifier(t *testing.T) {
	t.Parallel()

	account := accountFor(t, "user-1", true)
	f := newServiceFixture(t, newFakeAccounts().with("ada_lovelace", account))

	pair, err := f.svc.Login(context.Background(), "  Ada_Lovelace  ", testPassword, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Login() with a username = %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("Login() returned an incomplete token pair")
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	t.Parallel()

	active := accountFor(t, "user-1", true)
	disabled := accountFor(t, "user-2", false)
	accounts := newFakeAccounts().
		with("active@example.com", active).
		with("disabled@example.com", disabled)
	f := newServiceFixture(t, accounts)

	cases := []struct {
		name     string
		email    string
		password string
	}{
		{"unknown email", "nobody@example.com", testPassword},
		{"wrong password", "active@example.com", "wrong password value"},
		// A disabled account must not be distinguishable either: "this account
		// is disabled" confirms the address is registered.
		{"disabled account", "disabled@example.com", testPassword},
	}

	var messages []string
	for _, tc := range cases {
		_, err := f.svc.Login(context.Background(), tc.email, tc.password, auth.RequestMeta{})
		if err == nil {
			t.Fatalf("%s: Login() = nil, want an error", tc.name)
		}
		if !apperr.Is(err, apperr.KindUnauthorized) {
			t.Errorf("%s: kind = %v, want unauthorized", tc.name, apperr.KindOf(err))
		}
		var appErr *apperr.Error
		if !errors.As(err, &appErr) {
			t.Fatalf("%s: Login() = %v, want *apperr.Error", tc.name, err)
		}
		if appErr.Code != "INVALID_CREDENTIALS" {
			t.Errorf("%s: code = %q, want INVALID_CREDENTIALS", tc.name, appErr.Code)
		}
		messages = append(messages, appErr.Message)
	}

	for _, m := range messages[1:] {
		if m != messages[0] {
			t.Errorf("login failure messages differ: %q vs %q", messages[0], m)
		}
	}

	// No session may be created on any failure path.
	if len(f.sessions.rows) != 0 {
		t.Errorf("failed logins created %d sessions", len(f.sessions.rows))
	}
}

func TestLoginRehashesWeakStoredPasswords(t *testing.T) {
	t.Parallel()

	weak, err := auth.NewArgon2Hasher(auth.Argon2Params{
		Time: 1, Memory: 8 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32,
	})
	if err != nil {
		t.Fatalf("NewArgon2Hasher() = %v", err)
	}
	oldHash, err := weak.Hash(testPassword)
	if err != nil {
		t.Fatalf("Hash() = %v", err)
	}

	accounts := newFakeAccounts().with("a@example.com", auth.Account{
		UserID: "user-1", PasswordHash: oldHash, Role: auth.RoleReadOnly, Active: true,
	})

	strong, err := auth.NewArgon2Hasher(auth.Argon2Params{
		Time: 2, Memory: 16 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32,
	})
	if err != nil {
		t.Fatalf("NewArgon2Hasher() = %v", err)
	}

	sessions := &fakeSessions{now: time.Now}
	resets := newFakeResets(time.Now)
	svc := auth.NewService(
		auth.ServiceConfig{
			Issuer: "i", Audience: "a", AccessTTL: time.Minute, RefreshTTL: time.Hour,
			PasswordResetTTL: time.Hour,
		},
		accounts,
		sessions,
		resets,
		plaintextCipher{},
		strong,
		&staticIssuer{},
		messaging.NewMemoryPublisher(),
		rollbackTx{sessions: sessions, resets: resets},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	if _, err := svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{}); err != nil {
		t.Fatalf("Login() = %v", err)
	}

	upgraded, ok := accounts.updated["user-1"]
	if !ok {
		t.Fatal("the password was not re-hashed after a successful login")
	}
	if upgraded == oldHash {
		t.Error("the stored hash was rewritten unchanged")
	}
	if res := strong.Verify(testPassword, upgraded); !res.OK || res.NeedsRehash {
		t.Errorf("Verify(upgraded) = %+v, want OK without a further rehash", res)
	}
}

func TestRefreshRotatesTheToken(t *testing.T) {
	t.Parallel()

	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", accountFor(t, "user-1", true)))

	first, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Login() = %v", err)
	}

	second, err := f.svc.Refresh(context.Background(), first.RefreshToken, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Refresh() = %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("Refresh() returned the same refresh token; rotation did not happen")
	}

	// The old session is revoked and the new one belongs to the same family, so
	// a later logout still reaches both.
	if f.sessions.activeCount() != 1 {
		t.Errorf("active sessions = %d, want 1", f.sessions.activeCount())
	}
	if f.sessions.rows[0].FamilyID != f.sessions.rows[1].FamilyID {
		t.Error("rotation started a new family")
	}
	if f.sessions.rows[1].ParentID == nil || *f.sessions.rows[1].ParentID != f.sessions.rows[0].ID {
		t.Error("the rotated session does not point at its parent")
	}
}

func TestRefreshReuseRevokesTheWholeFamily(t *testing.T) {
	t.Parallel()

	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", accountFor(t, "user-1", true)))

	first, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Login() = %v", err)
	}
	second, err := f.svc.Refresh(context.Background(), first.RefreshToken, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Refresh() = %v", err)
	}

	// Replaying the already-rotated token is the signal that a copy exists.
	if _, err := f.svc.Refresh(context.Background(), first.RefreshToken, auth.RequestMeta{}); err == nil {
		t.Fatal("Refresh() accepted a replayed token")
	}

	// Both the attacker's and the victim's tokens must now be dead. Logging the
	// legitimate user out is the intended cost: the alternative is leaving an
	// attacker holding a working session.
	if f.sessions.activeCount() != 0 {
		t.Errorf("active sessions after reuse = %d, want 0", f.sessions.activeCount())
	}
	if _, err := f.svc.Refresh(context.Background(), second.RefreshToken, auth.RequestMeta{}); err == nil {
		t.Error("the legitimate token still works after a reuse was detected")
	}
}

func TestRefreshRejectsExpiredSessions(t *testing.T) {
	t.Parallel()

	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", accountFor(t, "user-1", true)))

	pair, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Login() = %v", err)
	}

	f.now = f.now.Add(31 * 24 * time.Hour)

	if _, err := f.svc.Refresh(context.Background(), pair.RefreshToken, auth.RequestMeta{}); err == nil {
		t.Fatal("Refresh() accepted an expired session")
	}
	// Expiry is not reuse, so the family must not be treated as compromised —
	// but the session is no longer usable either way.
	if f.sessions.activeCount() != 1 {
		t.Errorf("active sessions = %d, want the expired row left alone", f.sessions.activeCount())
	}
}

func TestRefreshRejectsDisabledAccounts(t *testing.T) {
	t.Parallel()

	account := accountFor(t, "user-1", true)
	accounts := newFakeAccounts().with("a@example.com", account)
	f := newServiceFixture(t, accounts)

	pair, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Login() = %v", err)
	}

	// Deactivation must take effect at the next rotation rather than after the
	// refresh lifetime.
	disabled := account
	disabled.Active = false
	accounts.byID["user-1"] = disabled

	if _, err := f.svc.Refresh(context.Background(), pair.RefreshToken, auth.RequestMeta{}); err == nil {
		t.Fatal("Refresh() succeeded for a disabled account")
	}
	if f.sessions.activeCount() != 0 {
		t.Errorf("active sessions = %d, want 0 after the account was disabled", f.sessions.activeCount())
	}
}

func TestLogoutRevokesTheFamilyAndIsIdempotent(t *testing.T) {
	t.Parallel()

	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", accountFor(t, "user-1", true)))

	pair, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Login() = %v", err)
	}
	rotated, err := f.svc.Refresh(context.Background(), pair.RefreshToken, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Refresh() = %v", err)
	}

	if err := f.svc.Logout(context.Background(), rotated.RefreshToken); err != nil {
		t.Fatalf("Logout() = %v", err)
	}
	if f.sessions.activeCount() != 0 {
		t.Errorf("active sessions after logout = %d, want 0", f.sessions.activeCount())
	}

	// An unknown or already-used token must not report an error: logout is
	// unauthenticated, so a distinguishable failure would let anyone probe
	// which tokens exist.
	if err := f.svc.Logout(context.Background(), "a token that was never issued"); err != nil {
		t.Errorf("Logout() with an unknown token = %v, want nil", err)
	}
	if err := f.svc.Logout(context.Background(), ""); err != nil {
		t.Errorf("Logout() with an empty token = %v, want nil", err)
	}
}
