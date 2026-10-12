package user_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
)

// TestCreatingAUserEmitsTheCreatedEvent is the contract every consumer is
// written against.
func TestCreatingAUserEmitsTheCreatedEvent(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	publisher := messaging.NewMemoryPublisher()
	svc := newServiceWithPublisher(repo, publisher, rollbackTx{repo: repo})

	created, err := svc.Create(adminContext(), user.CreateInput{
		Email:    "Ada@Example.com",
		Name:     "Ada",
		Password: validPassword,
		Role:     auth.RoleReadOnly,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	events := publisher.EventsOfType(user.EventUserCreated)
	if len(events) != 1 {
		t.Fatalf("published %d %s events, want exactly 1",
			len(events), user.EventUserCreated)
	}

	event := events[0]
	if event.ID.String() == "" || event.OccurredAt.IsZero() {
		t.Error("the event is missing its id or timestamp")
	}

	var payload user.CreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("the payload is not valid JSON: %v", err)
	}
	if payload.UserID != created.ID.String() {
		t.Errorf("payload user_id = %q, want %q", payload.UserID, created.ID)
	}
	// The stored address is normalised, and the event must carry the same
	// value or a consumer keying on it would disagree with the database.
	if payload.Email != created.Email {
		t.Errorf("payload email = %q, want the normalised %q", payload.Email, created.Email)
	}
	if payload.Name != "Ada" || payload.Role != string(auth.RoleReadOnly) {
		t.Errorf("payload = %+v, want the submitted name and role", payload)
	}
}

// TestTheCreatedEventCarriesNoCredentials is the one that would matter most if
// it ever failed. An event is copied into the broker, into every consumer's
// storage and into any log that records it, so a secret in the payload has
// been leaked several times over before a human reviews it.
func TestTheCreatedEventCarriesNoCredentials(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	publisher := messaging.NewMemoryPublisher()
	svc := newServiceWithPublisher(repo, publisher, rollbackTx{repo: repo})

	created, err := svc.Create(adminContext(), user.CreateInput{
		Email:    "ada@example.com",
		Name:     "Ada",
		Password: validPassword,
		Role:     auth.RoleReadOnly,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	raw := string(publisher.EventsOfType(user.EventUserCreated)[0].Payload)

	// Assert on the encoded bytes rather than on a decoded struct: decoding
	// into CreatedPayload would silently discard any extra field, which is
	// precisely the leak this test exists to catch.
	forbidden := map[string]string{
		"the plaintext password": validPassword,
		"the password hash":      created.PasswordHash,
		"a password field":       "password",
		"a hash field":           "hash",
	}
	for what, needle := range forbidden {
		if needle == "" {
			t.Fatalf("the test fixture produced no value for %s", what)
		}
		if strings.Contains(strings.ToLower(raw), strings.ToLower(needle)) {
			t.Errorf("the event payload contains %s: %s", what, raw)
		}
	}
}

// TestAUserIsNotCreatedWhenItsEventCannotBePublished is the atomicity
// guarantee. A user who exists but whose creation was never announced is the
// silent inconsistency the outbox exists to prevent: nothing downstream ever
// hears about them and no error is ever raised.
func TestAUserIsNotCreatedWhenItsEventCannotBePublished(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	publisher := messaging.NewMemoryPublisher()
	publisher.FailWith(errors.New("the outbox insert failed"))
	svc := newServiceWithPublisher(repo, publisher, rollbackTx{repo: repo})

	_, err := svc.Create(adminContext(), user.CreateInput{
		Email:    "ada@example.com",
		Name:     "Ada",
		Password: validPassword,
		Role:     auth.RoleReadOnly,
	})
	if err == nil {
		t.Fatal("Create succeeded even though the event could not be published")
	}

	// The user must be gone. Not merely absent from the response — absent from
	// storage, because the whole transaction rolled back.
	if _, err := repo.GetByEmail(context.Background(), "ada@example.com"); err == nil {
		t.Error("the user was persisted even though its event was never published")
	}

	// And the address must still be usable, or a transient broker failure
	// would permanently burn the email addresses of everyone who tried to
	// sign up during it.
	publisher.FailWith(nil)
	if _, err := svc.Create(adminContext(), user.CreateInput{
		Email:    "ada@example.com",
		Name:     "Ada",
		Password: validPassword,
		Role:     auth.RoleReadOnly,
	}); err != nil {
		t.Errorf("the address was left unusable after the failed attempt: %v", err)
	}
}

// TestTheCreatedEventInheritsTheRequestTrace is what makes the asynchronous
// work appear on the same trace as the call that caused it. Without it the
// welcome email is an orphan span and nobody can tell which request produced
// it.
func TestTheCreatedEventInheritsTheRequestTrace(t *testing.T) {
	t.Parallel()

	const traceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	repo := newFakeRepository()
	publisher := messaging.NewMemoryPublisher()
	svc := newServiceWithPublisher(repo, publisher, rollbackTx{repo: repo})

	ctx := messaging.ContextWithTraceParent(adminContext(), traceparent)
	if _, err := svc.Create(ctx, user.CreateInput{
		Email:    "ada@example.com",
		Name:     "Ada",
		Password: validPassword,
		Role:     auth.RoleReadOnly,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got := publisher.EventsOfType(user.EventUserCreated)[0].TraceParent
	if got != traceparent {
		t.Errorf("event traceparent = %q, want the request's %q", got, traceparent)
	}
}

// TestARejectedUserPublishesNothing checks the ordering. Validating after
// publishing would announce users who were never created.
func TestARejectedUserPublishesNothing(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	publisher := messaging.NewMemoryPublisher()
	svc := newServiceWithPublisher(repo, publisher, rollbackTx{repo: repo})

	if _, err := svc.Create(adminContext(), user.CreateInput{
		Email:    "not-an-email",
		Name:     "Ada",
		Password: validPassword,
		Role:     auth.RoleReadOnly,
	}); err == nil {
		t.Fatal("Create accepted an invalid email address")
	}

	if events := publisher.Events(); len(events) != 0 {
		t.Errorf("publishing happened for a rejected user: %v", events)
	}
}
