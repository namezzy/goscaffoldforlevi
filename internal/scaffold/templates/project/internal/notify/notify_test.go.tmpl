package notify_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/notify"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// recordingSender captures what would have been sent.
type recordingSender struct {
	mu   sync.Mutex
	sent []notify.Email
	err  error
}

func (s *recordingSender) Send(_ context.Context, msg notify.Email) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, msg)
	return nil
}

func (s *recordingSender) messages() []notify.Email {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]notify.Email(nil), s.sent...)
}

func userCreatedEvent(t *testing.T, payload user.CreatedPayload) messaging.Event {
	t.Helper()
	event, err := messaging.NewEvent(user.EventUserCreated, payload)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	return event
}

// TestTheWelcomeHandlerSendsToTheAddressInTheEvent is the base case.
func TestTheWelcomeHandlerSendsToTheAddressInTheEvent(t *testing.T) {
	t.Parallel()

	sender := &recordingSender{}
	handler := notify.NewWelcomeHandler(sender, discardLogger())

	event := userCreatedEvent(t, user.CreatedPayload{
		Email: "ada@example.com",
		Name:  "Ada",
	})
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	sent := sender.messages()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	if sent[0].To != "ada@example.com" {
		t.Errorf("To = %q, want the address from the event", sent[0].To)
	}
	if !strings.Contains(sent[0].Body, "Ada") {
		t.Errorf("body %q does not greet the user by name", sent[0].Body)
	}
}

// TestAnEventOfAnotherTypeIsIgnoredWithoutError covers the routing rule.
// Treating "not mine" as a failure would retry the message four times and then
// dead-letter something that was never addressed to this handler, turning a
// routine subscription change into a page.
func TestAnEventOfAnotherTypeIsIgnoredWithoutError(t *testing.T) {
	t.Parallel()

	sender := &recordingSender{}
	handler := notify.NewWelcomeHandler(sender, discardLogger())

	event := messaging.Event{
		ID:      uuid.Must(uuid.NewV7()),
		Type:    "order.placed.v1",
		Payload: json.RawMessage(`{"total":100}`),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle returned %v for an unrelated event; it must be ignored, not retried", err)
	}
	if len(sender.messages()) != 0 {
		t.Error("the handler sent an email for an order event")
	}
}

// TestAnUndecodablePayloadIsPermanentNotRetried is the poison-message rule.
// Bytes that are not valid JSON will not become valid on the fifth attempt, so
// retrying only delays telling a human about it.
func TestAnUndecodablePayloadIsPermanentNotRetried(t *testing.T) {
	t.Parallel()

	handler := notify.NewWelcomeHandler(&recordingSender{}, discardLogger())
	event := messaging.Event{
		ID:      uuid.Must(uuid.NewV7()),
		Type:    user.EventUserCreated,
		Payload: json.RawMessage(`{"email": `), // truncated
	}

	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("Handle accepted a payload it could not decode")
	}
	if !errors.Is(err, messaging.ErrPermanent) {
		t.Errorf("Handle returned %v, which is retryable; an undecodable payload must be permanent", err)
	}
}

// TestAnEventWithNoAddressIsPermanent covers the structurally valid but
// unusable payload. Retrying cannot conjure an address, and without the
// permanent marker the event would occupy the retry budget five times over.
func TestAnEventWithNoAddressIsPermanent(t *testing.T) {
	t.Parallel()

	sender := &recordingSender{}
	handler := notify.NewWelcomeHandler(sender, discardLogger())
	event := userCreatedEvent(t, user.CreatedPayload{Name: "Nameless"})

	err := handler.Handle(context.Background(), event)
	if !errors.Is(err, messaging.ErrPermanent) {
		t.Errorf("Handle returned %v for an event with no email, want a permanent failure", err)
	}
	if len(sender.messages()) != 0 {
		t.Error("the handler tried to send a message with no recipient")
	}
}

// TestAFailingProviderIsRetryable is the mirror image of the permanent cases.
// A mail provider being down is exactly what the backoff exists for, and
// marking it permanent would dead-letter every signup during an outage.
func TestAFailingProviderIsRetryable(t *testing.T) {
	t.Parallel()

	sender := &recordingSender{err: errors.New("smtp: connection refused")}
	handler := notify.NewWelcomeHandler(sender, discardLogger())
	event := userCreatedEvent(t, user.CreatedPayload{Email: "ada@example.com", Name: "Ada"})

	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("Handle ignored a provider failure, so the welcome email would be lost silently")
	}
	if errors.Is(err, messaging.ErrPermanent) {
		t.Error("a provider outage was marked permanent; it must be retried")
	}
}

// TestTheGreetingFallsBackToTheAddress keeps the message readable when a user
// signed up without a name.
func TestTheGreetingFallsBackToTheAddress(t *testing.T) {
	t.Parallel()

	sender := &recordingSender{}
	handler := notify.NewWelcomeHandler(sender, discardLogger())
	event := userCreatedEvent(t, user.CreatedPayload{Email: "ada@example.com"})

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	body := sender.messages()[0].Body
	if strings.Contains(body, "Hello ,") || strings.Contains(body, "Hello \n") {
		t.Errorf("body %q opens with an empty greeting", body)
	}
	if !strings.Contains(body, "ada@example.com") {
		t.Errorf("body %q does not fall back to the address", body)
	}
}

// TestTheLogSenderReportsThatNothingWasSent guards the placeholder against
// being mistaken for a working provider. The log line is the only signal an
// operator gets, so it must say plainly that no mail left the process.
func TestTheLogSenderReportsThatNothingWasSent(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	sender := notify.NewLogSender(logger)

	err := sender.Send(context.Background(), notify.Email{
		To: "ada@example.com", Subject: "Welcome", Body: "Hello",
	})
	if err != nil {
		t.Fatalf("the log sender returned %v; it must not fail the handler", err)
	}

	line := buf.String()
	if !strings.Contains(line, "not sent") {
		t.Errorf("log line %q does not make clear that no email was sent", line)
	}
	if !strings.Contains(line, "ada@example.com") {
		t.Errorf("log line %q does not record the intended recipient", line)
	}
	// The body may contain anything the user typed into their profile, so it
	// does not belong in the log.
	if strings.Contains(line, "Hello") {
		t.Errorf("log line %q contains the message body", line)
	}
}
