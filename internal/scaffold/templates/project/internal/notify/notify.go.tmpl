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
	"net/url"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/incident"
	"github.com/namezzy/goscaffoldforlevi/internal/invite"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
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

// TokenDecrypter reverses the encryption invite.Service applies to the
// token before it is queued — see invite.InvitationCreatedPayload's doc
// comment for why the payload never carries it in the clear.
type TokenDecrypter interface {
	Decrypt(ciphertext string) (string, error)
}

// InvitationHandler sends the invitation email when a member is invited.
type InvitationHandler struct {
	sender        EmailSender
	acceptURLBase string
	cipher        TokenDecrypter
	logger        *slog.Logger
}

// NewInvitationHandler wires the handler. acceptURLBase is the frontend page
// the email links to (config.Invite.AcceptURLBase); the invitation's
// one-time token is appended to it as a query parameter. cipher must be
// constructed from the same key invite.Service encrypted the token with —
// cmd/worker builds both cmd/api's and this package's cipher from
// config.Auth.JWTSecret, the same way invite.NewService's caller does.
func NewInvitationHandler(sender EmailSender, acceptURLBase string, cipher TokenDecrypter, logger *slog.Logger) *InvitationHandler {
	return &InvitationHandler{sender: sender, acceptURLBase: acceptURLBase, cipher: cipher, logger: logger}
}

// Handle sends the invitation email for an invitation.created.v1 event. Like
// WelcomeHandler, an event of another type is not an error — see its doc
// comment for why.
func (h *InvitationHandler) Handle(ctx context.Context, event messaging.Event) error {
	if event.Type != invite.EventInvitationCreated {
		return nil
	}

	var payload invite.InvitationCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return messaging.Permanent(fmt.Errorf("decode %s payload: %w", event.Type, err))
	}
	if payload.Email == "" || payload.Token == "" {
		return messaging.Permanent(fmt.Errorf("event %s carries no email or token", event.ID))
	}

	token, err := h.cipher.Decrypt(payload.Token)
	if err != nil {
		// A decrypt failure cannot improve with a retry — it is either the
		// wrong key or a corrupted value, and both are permanent.
		return messaging.Permanent(fmt.Errorf("decrypt token for %s: %w", event.ID, err))
	}

	link := h.acceptURLBase
	if link != "" {
		link += "?token=" + url.QueryEscape(token)
	}

	if err := h.sender.Send(ctx, Email{
		To:      payload.Email,
		Subject: "You have been invited",
		Body: fmt.Sprintf(
			"Hello,\n\nYou have been invited to join. Accept your invitation:\n%s\n\n"+
				"This link expires; if it has, ask whoever invited you to send a new one.\n", link),
	}); err != nil {
		return fmt.Errorf("send invitation email: %w", err)
	}
	return nil
}

// PasswordResetHandler sends the password reset email when a member
// requests one.
type PasswordResetHandler struct {
	sender       EmailSender
	resetURLBase string
	cipher       TokenDecrypter
	logger       *slog.Logger
}

// NewPasswordResetHandler wires the handler. resetURLBase is the frontend
// page the email links to (config.Auth.PasswordResetURLBase); the reset
// token is appended to it as a query parameter — the same pattern
// NewInvitationHandler uses for acceptURLBase. cipher must be constructed
// from the same key auth.Service encrypted the token with — cmd/worker
// builds both cmd/api's and this package's cipher from
// config.Auth.JWTSecret, the same way it does for invite's cipher pair.
func NewPasswordResetHandler(sender EmailSender, resetURLBase string, cipher TokenDecrypter, logger *slog.Logger) *PasswordResetHandler {
	return &PasswordResetHandler{sender: sender, resetURLBase: resetURLBase, cipher: cipher, logger: logger}
}

// Handle sends the password reset email for a
// user.password_reset_requested.v1 event. Like WelcomeHandler, an event of
// another type is not an error — see its doc comment for why.
func (h *PasswordResetHandler) Handle(ctx context.Context, event messaging.Event) error {
	if event.Type != auth.EventPasswordResetRequested {
		return nil
	}

	var payload auth.PasswordResetRequestedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return messaging.Permanent(fmt.Errorf("decode %s payload: %w", event.Type, err))
	}
	if payload.Email == "" || payload.Token == "" {
		return messaging.Permanent(fmt.Errorf("event %s carries no email or token", event.ID))
	}

	token, err := h.cipher.Decrypt(payload.Token)
	if err != nil {
		// A decrypt failure cannot improve with a retry — it is either the
		// wrong key or a corrupted value, and both are permanent.
		return messaging.Permanent(fmt.Errorf("decrypt token for %s: %w", event.ID, err))
	}

	link := h.resetURLBase
	if link != "" {
		link += "?token=" + url.QueryEscape(token)
	}

	if err := h.sender.Send(ctx, Email{
		To:      payload.Email,
		Subject: "Reset your password",
		Body: fmt.Sprintf(
			"Hello,\n\nA password reset was requested for your account. If this was you, "+
				"set a new password here:\n%s\n\n"+
				"This link expires and can only be used once. If you did not request this, "+
				"no action is needed — your password has not changed.\n", link),
	}); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}
	return nil
}

// IncidentAlertHandler is the exception center's external channel: one
// email, to one fixed operations address, per newly opened incident. It is
// wired into cmd/worker only when that address is configured
// (NOTIFY_INCIDENT_ALERT_EMAIL) — an unconfigured deployment still gets the
// in-app notification incident.Watcher sends directly, just not this one.
type IncidentAlertHandler struct {
	sender EmailSender
	to     string
	logger *slog.Logger
}

// NewIncidentAlertHandler wires the handler. to is the configured ops
// address; callers must not construct this with an empty one — see
// IncidentAlertHandler's doc comment on when to wire it at all.
func NewIncidentAlertHandler(sender EmailSender, to string, logger *slog.Logger) *IncidentAlertHandler {
	return &IncidentAlertHandler{sender: sender, to: to, logger: logger}
}

// Handle sends the alert email for an incident.opened.v1 event. Like
// WelcomeHandler, an event of another type is not an error.
func (h *IncidentAlertHandler) Handle(ctx context.Context, event messaging.Event) error {
	if event.Type != incident.EventIncidentOpened {
		return nil
	}

	var payload incident.IncidentOpenedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return messaging.Permanent(fmt.Errorf("decode %s payload: %w", event.Type, err))
	}

	if err := h.sender.Send(ctx, Email{
		To:      h.to,
		Subject: fmt.Sprintf("[%s] New exception: %s", payload.Severity, payload.EventType),
		Body: fmt.Sprintf(
			"A new exception was detected.\n\n"+
				"Event type: %s\nConsumer: %s\nSeverity: %s\nError: %s\n\n"+
				"Incident ID: %s\n", payload.EventType, payload.Consumer, payload.Severity,
			payload.LastError, payload.IncidentID),
	}); err != nil {
		return fmt.Errorf("send incident alert email: %w", err)
	}
	return nil
}

// DispatchHandler combines several handlers into one, trying each in turn.
// Every handler here already ignores an event type that is not its own (see
// WelcomeHandler.Handle and InvitationHandler.Handle), so running them all
// and letting each decide for itself is simpler than a type-to-handler
// lookup table — and one more handler is one more call, not a new map entry
// to keep in sync with a registration list.
//
// This is what lets cmd/worker run one durable consumer across several event
// types — see natsmq.Subscribe's doc comment for the other half of that.
func DispatchHandler(handlers ...messaging.Handler) messaging.Handler {
	return messaging.HandlerFunc(func(ctx context.Context, event messaging.Event) error {
		for _, h := range handlers {
			if err := h.Handle(ctx, event); err != nil {
				return err
			}
		}
		return nil
	})
}
