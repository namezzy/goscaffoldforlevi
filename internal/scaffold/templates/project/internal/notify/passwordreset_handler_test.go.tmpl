package notify_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/notify"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
)

// echoCipher is a no-op TokenDecrypter: what matters to these tests is
// that the handler calls the cipher and uses its result, not that AES-GCM
// in particular runs — that is auth.TestTokenCipherRoundTrips' job.
type echoCipher struct{ err error }

func (c echoCipher) Decrypt(ciphertext string) (string, error) {
	if c.err != nil {
		return "", c.err
	}
	return ciphertext, nil
}

func passwordResetEvent(t *testing.T, payload auth.PasswordResetRequestedPayload) messaging.Event {
	t.Helper()
	event, err := messaging.NewEvent(auth.EventPasswordResetRequested, payload)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	return event
}

func TestThePasswordResetHandlerSendsToTheAddressInTheEventWithTheDecryptedLink(t *testing.T) {
	sender := &recordingSender{}
	handler := notify.NewPasswordResetHandler(sender, "https://app.example.com/reset-password", echoCipher{}, discardLogger())

	event := passwordResetEvent(t, auth.PasswordResetRequestedPayload{
		UserID: "user-1", Email: "a@example.com", Token: "the-plaintext-token",
	})
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() = %v", err)
	}

	sent := sender.messages()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	if sent[0].To != "a@example.com" {
		t.Errorf("To = %q, want a@example.com", sent[0].To)
	}
	if !strings.Contains(sent[0].Body, "https://app.example.com/reset-password?token=the-plaintext-token") {
		t.Errorf("body does not contain the expected reset link: %s", sent[0].Body)
	}
}

func TestPasswordResetHandlerIgnoresAnEventOfAnotherType(t *testing.T) {
	sender := &recordingSender{}
	handler := notify.NewPasswordResetHandler(sender, "https://app.example.com/reset-password", echoCipher{}, discardLogger())

	// userCreatedEvent builds a user.created.v1 event; reused here only for
	// its event-type mismatch, matching TestAnEventOfAnotherTypeIsIgnoredWithoutError's
	// reasoning for WelcomeHandler.
	event := userCreatedEvent(t, user.CreatedPayload{Email: "a@example.com"})
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() on a foreign event type = %v, want nil", err)
	}
	if len(sender.messages()) != 0 {
		t.Error("a message was sent for an event this handler does not own")
	}
}

func TestPasswordResetHandlerRejectsAnEventWithNoEmailOrToken(t *testing.T) {
	sender := &recordingSender{}
	handler := notify.NewPasswordResetHandler(sender, "https://app.example.com/reset-password", echoCipher{}, discardLogger())

	for name, payload := range map[string]auth.PasswordResetRequestedPayload{
		"no email": {UserID: "user-1", Token: "t"},
		"no token": {UserID: "user-1", Email: "a@example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			err := handler.Handle(context.Background(), passwordResetEvent(t, payload))
			if err == nil {
				t.Fatal("Handle() = nil, want a permanent error")
			}
			if !errors.Is(err, messaging.ErrPermanent) {
				t.Errorf("Handle() = %v, want a permanent (non-retryable) error", err)
			}
		})
	}
	if len(sender.messages()) != 0 {
		t.Error("a message was sent despite invalid payload")
	}
}

func TestPasswordResetHandlerTreatsADecryptFailureAsPermanent(t *testing.T) {
	sender := &recordingSender{}
	handler := notify.NewPasswordResetHandler(
		sender, "https://app.example.com/reset-password",
		echoCipher{err: errors.New("gcm: authentication failed")}, discardLogger(),
	)

	event := passwordResetEvent(t, auth.PasswordResetRequestedPayload{
		UserID: "user-1", Email: "a@example.com", Token: "ciphertext",
	})
	err := handler.Handle(context.Background(), event)
	if err == nil || !errors.Is(err, messaging.ErrPermanent) {
		t.Errorf("Handle() with an undecryptable token = %v, want a permanent error", err)
	}
	if len(sender.messages()) != 0 {
		t.Error("a message was sent despite a decrypt failure")
	}
}

func TestPasswordResetHandlerTreatsASendFailureAsRetryable(t *testing.T) {
	sender := &recordingSender{err: errors.New("smtp: connection refused")}
	handler := notify.NewPasswordResetHandler(sender, "https://app.example.com/reset-password", echoCipher{}, discardLogger())

	event := passwordResetEvent(t, auth.PasswordResetRequestedPayload{
		UserID: "user-1", Email: "a@example.com", Token: "t",
	})
	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("Handle() = nil, want the send error")
	}
	if errors.Is(err, messaging.ErrPermanent) {
		t.Error("a mail provider outage was treated as permanent — it should be retried")
	}
}
