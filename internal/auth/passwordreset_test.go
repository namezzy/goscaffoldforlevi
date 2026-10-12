package auth_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
)

func accountWithEmail(t *testing.T, userID, email string, active bool) auth.Account {
	t.Helper()
	a := accountFor(t, userID, active)
	a.Email = email
	return a
}

func TestRequestPasswordResetIsSilentForAnUnknownIdentifier(t *testing.T) {
	t.Parallel()
	f := newServiceFixture(t, newFakeAccounts())

	if err := f.svc.RequestPasswordReset(context.Background(), "nobody@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset() for an unknown account = %v, want nil (must not reveal non-existence)", err)
	}
	if len(f.resets.rows) != 0 {
		t.Errorf("a token was created for an account that does not exist")
	}
	if len(f.events.Events()) != 0 {
		t.Errorf("an event was published for an account that does not exist")
	}
}

func TestRequestPasswordResetIsSilentForADisabledAccount(t *testing.T) {
	t.Parallel()
	account := accountWithEmail(t, "user-1", "disabled@example.com", false)
	f := newServiceFixture(t, newFakeAccounts().with("disabled@example.com", account))

	if err := f.svc.RequestPasswordReset(context.Background(), "disabled@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset() for a disabled account = %v, want nil", err)
	}
	if len(f.resets.rows) != 0 {
		t.Errorf("a token was created for a disabled account")
	}
}

func TestRequestPasswordResetCreatesATokenAndPublishesAnEncryptedEvent(t *testing.T) {
	t.Parallel()
	account := accountWithEmail(t, "user-1", "a@example.com", true)
	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", account))

	if err := f.svc.RequestPasswordReset(context.Background(), "a@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset() = %v", err)
	}
	if len(f.resets.rows) != 1 {
		t.Fatalf("created %d tokens, want 1", len(f.resets.rows))
	}

	events := f.events.EventsOfType(auth.EventPasswordResetRequested)
	if len(events) != 1 {
		t.Fatalf("published %d %s events, want 1", len(events), auth.EventPasswordResetRequested)
	}
	var payload auth.PasswordResetRequestedPayload
	if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.UserID != "user-1" || payload.Email != "a@example.com" {
		t.Errorf("payload = %+v, want user-1 / a@example.com", payload)
	}
	if payload.Token == "" {
		t.Error("payload carries no token")
	}
}

func TestRequestPasswordResetInvalidatesAnEarlierPendingToken(t *testing.T) {
	t.Parallel()
	account := accountWithEmail(t, "user-1", "a@example.com", true)
	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", account))

	if err := f.svc.RequestPasswordReset(context.Background(), "a@example.com"); err != nil {
		t.Fatalf("first RequestPasswordReset() = %v", err)
	}
	firstToken := firstPublishedResetToken(t, f)

	if err := f.svc.RequestPasswordReset(context.Background(), "a@example.com"); err != nil {
		t.Fatalf("second RequestPasswordReset() = %v", err)
	}
	if len(f.resets.rows) != 1 {
		t.Fatalf("%d tokens are live after a second request, want exactly 1 (the newest)", len(f.resets.rows))
	}

	// The first, now-superseded link must no longer work.
	err := f.svc.ConfirmPasswordReset(context.Background(), firstToken, "a new strong password 123")
	if !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("confirming the superseded token: err = %v, want PASSWORD_RESET_TOKEN_INVALID", err)
	}
}

func TestConfirmPasswordResetUpdatesThePasswordAndRevokesEverySession(t *testing.T) {
	t.Parallel()
	account := accountWithEmail(t, "user-1", "a@example.com", true)
	accounts := newFakeAccounts().with("a@example.com", account)
	f := newServiceFixture(t, accounts)

	// Two independent logins, two independent session families.
	if _, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{}); err != nil {
		t.Fatalf("first Login() = %v", err)
	}
	if _, err := f.svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{}); err != nil {
		t.Fatalf("second Login() = %v", err)
	}
	if f.sessions.activeCount() != 2 {
		t.Fatalf("active sessions = %d, want 2", f.sessions.activeCount())
	}

	if err := f.svc.RequestPasswordReset(context.Background(), "a@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset() = %v", err)
	}
	token := firstPublishedResetToken(t, f)

	const newPassword = "a new strong password 123"
	if err := f.svc.ConfirmPasswordReset(context.Background(), token, newPassword); err != nil {
		t.Fatalf("ConfirmPasswordReset() = %v", err)
	}

	if _, ok := accounts.updated["user-1"]; !ok {
		t.Error("the password hash was never updated")
	}
	if f.sessions.activeCount() != 0 {
		t.Errorf("active sessions after reset = %d, want 0 — every session must be revoked", f.sessions.activeCount())
	}
}

func TestConfirmPasswordResetRejectsAnUnknownToken(t *testing.T) {
	t.Parallel()
	f := newServiceFixture(t, newFakeAccounts())

	err := f.svc.ConfirmPasswordReset(context.Background(), "not-a-real-token", "a new strong password 123")
	if !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("err = %v, want PASSWORD_RESET_TOKEN_INVALID", err)
	}
}

func TestConfirmPasswordResetRejectsAnExpiredToken(t *testing.T) {
	t.Parallel()
	account := accountWithEmail(t, "user-1", "a@example.com", true)
	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", account))

	if err := f.svc.RequestPasswordReset(context.Background(), "a@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset() = %v", err)
	}
	token := firstPublishedResetToken(t, f)

	f.now = f.now.Add(2 * time.Hour) // past the fixture's 1h PasswordResetTTL

	err := f.svc.ConfirmPasswordReset(context.Background(), token, "a new strong password 123")
	if !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("confirming an expired token: err = %v, want PASSWORD_RESET_TOKEN_INVALID", err)
	}
}

func TestConfirmPasswordResetRejectsAnAlreadyUsedToken(t *testing.T) {
	t.Parallel()
	account := accountWithEmail(t, "user-1", "a@example.com", true)
	f := newServiceFixture(t, newFakeAccounts().with("a@example.com", account))

	if err := f.svc.RequestPasswordReset(context.Background(), "a@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset() = %v", err)
	}
	token := firstPublishedResetToken(t, f)

	if err := f.svc.ConfirmPasswordReset(context.Background(), token, "a new strong password 123"); err != nil {
		t.Fatalf("first ConfirmPasswordReset() = %v", err)
	}
	err := f.svc.ConfirmPasswordReset(context.Background(), token, "yet another strong password 456")
	if !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("redeeming an already-used token: err = %v, want PASSWORD_RESET_TOKEN_INVALID", err)
	}
}

func TestConfirmPasswordResetRejectsAWeakPassword(t *testing.T) {
	t.Parallel()
	account := accountWithEmail(t, "user-1", "a@example.com", true)
	accounts := newFakeAccounts().with("a@example.com", account)
	f := newServiceFixture(t, accounts)

	if err := f.svc.RequestPasswordReset(context.Background(), "a@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset() = %v", err)
	}
	token := firstPublishedResetToken(t, f)

	if err := f.svc.ConfirmPasswordReset(context.Background(), token, "short"); !apperr.Is(err, apperr.KindInvalidArgument) {
		t.Errorf("err = %v, want an invalid-argument password policy error", err)
	}
	if _, ok := accounts.updated["user-1"]; ok {
		t.Error("the password was updated despite failing validation")
	}
}

// firstPublishedResetToken decodes the plaintext token from the most
// recently published EventPasswordResetRequested — recoverable here only
// because the fixture's TokenEncrypter (plaintextCipher) does not actually
// encrypt; a real deployment's ciphertext is only ever decrypted by
// notify.PasswordResetHandler, holding the matching key.
func firstPublishedResetToken(t *testing.T, f *serviceFixture) string {
	t.Helper()
	events := f.events.EventsOfType(auth.EventPasswordResetRequested)
	if len(events) == 0 {
		t.Fatal("no password reset event was published")
	}
	var payload auth.PasswordResetRequestedPayload
	if err := json.Unmarshal(events[len(events)-1].Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return payload.Token
}
