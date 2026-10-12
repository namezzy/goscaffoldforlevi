//go:build integration

package authpg_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/auth/authpg"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/pgtest"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
	"github.com/namezzy/goscaffoldforlevi/internal/user/userpg"
)

type stubHasher struct{}

func (stubHasher) Hash(string) (string, error) { return "$stub$hash", nil }
func (stubHasher) Verify(string, string) auth.VerifyResult {
	return auth.VerifyResult{OK: true}
}
func (stubHasher) VerifyDummy() {}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type staticIssuer struct{}

func (staticIssuer) Issue(auth.AccessClaims) (string, error) { return "stub-access-token", nil }

// plaintextCipher is a no-op TokenEncrypter — what matters to this test is
// the row lock ConfirmPasswordReset depends on, not AES-GCM, which
// auth.TestTokenCipherRoundTrips already covers.
type plaintextCipher struct{}

func (plaintextCipher) Encrypt(s string) (string, error) { return s, nil }

// TestConcurrentPasswordResetConfirmationsCannotBothSucceed is the
// regression test for the row lock ConfirmPasswordReset depends on — see
// its own doc comment on why GetByTokenHash's FOR UPDATE is what makes a
// concurrent double-redemption of the same reset link safe. This exercises
// the real *authpg.PasswordResetRepository and a real
// postgres.NewTxManager, the same way userpg's
// TestConcurrentDemotionsCannotBothSucceed does for the analogous
// last-system-admin race — nothing in-process can simulate two
// transactions genuinely serializing on a row lock.
func TestConcurrentPasswordResetConfirmationsCannotBothSucceed(t *testing.T) {
	pg := pgtest.Start(t)
	ctx := context.Background()

	users := userpg.New(pg.Pool)
	if _, err := users.Create(ctx, user.User{
		ID: user.NewID(), Email: "reset-race@example.com", Name: "Reset Race",
		Status: user.StatusActive, Role: auth.RoleReadOnly, PasswordHash: "$stub$hash",
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	events := messaging.NewMemoryPublisher()
	svc := auth.NewService(
		auth.ServiceConfig{
			Issuer: "test", Audience: "test-api", AccessTTL: time.Minute, RefreshTTL: time.Hour,
			PasswordResetTTL: time.Hour,
		},
		user.NewAccountStore(users),
		authpg.NewSessionRepository(pg.Pool),
		authpg.NewPasswordResetRepository(pg.Pool),
		plaintextCipher{},
		stubHasher{},
		staticIssuer{},
		events,
		postgres.NewTxManager(pg.Pool),
		discardLogger(),
	)

	if err := svc.RequestPasswordReset(ctx, "reset-race@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	published := events.EventsOfType(auth.EventPasswordResetRequested)
	if len(published) != 1 {
		t.Fatalf("published %d events, want 1", len(published))
	}
	var payload auth.PasswordResetRequestedPayload
	if err := json.Unmarshal(published[0].Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	token := payload.Token

	results := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		results[0] = svc.ConfirmPasswordReset(ctx, token, "first new password 123")
	}()
	go func() {
		defer wg.Done()
		results[1] = svc.ConfirmPasswordReset(ctx, token, "second new password 456")
	}()
	wg.Wait()

	succeeded, rejected := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			succeeded++
		case apperr.Is(err, apperr.KindInvalidArgument):
			rejected++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("succeeded=%d rejected=%d, want exactly one of each — a race let both redemptions through, or refused both",
			succeeded, rejected)
	}
}
