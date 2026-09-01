// Package notify turns domain events into messages to people.
//
// It is the first consumer in the system and exists to prove the asynchronous
// path end to end: an HTTP request creates a user, the event survives the
// commit, and a separate process acts on it. The sending itself is a port with
// a logging implementation — see LogSender for why that is a placeholder and
// not a pretence.
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/example/go-enterprise/internal/messaging"
	"github.com/example/go-enterprise/internal/user"
)

// EmailSender delivers one message.
//
// The interface is this small because that is all the consumer needs. A
// provider SDK behind it can grow templates, attachments and tracking without
// any of that reaching this package.
type EmailSender interface {
	Send(ctx context.Context, msg Email) error
}

// Email is one outgoing message.
type Email struct {
	To      string
	Subject string
	Body    string
}

// LogSender writes the message to the log instead of sending it.
//
// This is a placeholder implementation of a real port, not a feature pretending
// to work. The distinction matters: the wiring, the retry behaviour and the
// idempotency are all real and tested, so adding a provider is implementing one
// interface rather than building the path. STATE.md lists it under Not
// Implemented Yet so that nobody deploys this believing mail goes out.
type LogSender struct{ logger *slog.Logger }

// NewLogSender returns a sender that logs.
func NewLogSender(logger *slog.Logger) *LogSender { return &LogSender{logger: logger} }

// Send logs the message.
func (s *LogSender) Send(ctx context.Context, msg Email) error {
	s.logger.InfoContext(ctx, "email not sent: no provider is configured",
		slog.String("to", msg.To),
		slog.String("subject", msg.Subject))
	return nil
}

// WelcomeHandler sends a welcome message when a user is created.
type WelcomeHandler struct {
	sender EmailSender
	logger *slog.Logger
}

// NewWelcomeHandler wires the handler.
func NewWelcomeHandler(sender EmailSender, logger *slog.Logger) *WelcomeHandler {
	return &WelcomeHandler{sender: sender, logger: logger}
}

// Handle sends the welcome email for a user.created.v1 event.
//
// An event of another type returns nil rather than an error. A consumer that
// treats "not mine" as a failure retries it four times and then dead-letters a
// message that was never addressed to it, which turns a routing change into a
// pile of false alarms.
func (h *WelcomeHandler) Handle(ctx context.Context, event messaging.Event) error {
	if event.Type != user.EventUserCreated {
		return nil
	}

	var payload user.CreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		// Permanent: the bytes will not become valid on the fifth attempt, and
		// retrying only delays the moment a human is told about it.
		return messaging.Permanent(fmt.Errorf("decode %s payload: %w", event.Type, err))
	}
	if payload.Email == "" {
		return messaging.Permanent(fmt.Errorf("event %s carries no email address", event.ID))
	}

	err := h.sender.Send(ctx, Email{
		To:      payload.Email,
		Subject: "Welcome",
		Body: fmt.Sprintf(
			"Hello %s,\n\nYour account has been created.\n", displayName(payload)),
	})
	if err != nil {
		// Retryable: a mail provider being down is temporary, and this is the
		// case the backoff exists for.
		return fmt.Errorf("send welcome email: %w", err)
	}
	return nil
}

// displayName falls back to the address when a name is missing, so the message
// never opens with an empty greeting.
func displayName(p user.CreatedPayload) string {
	if p.Name != "" {
		return p.Name
	}
	return p.Email
}
