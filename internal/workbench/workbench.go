// Package workbench aggregates other features' own read paths into the
// one screen a member lands on after signing in — "what is there to do,
// and what does the system currently look like." It has no storage of its
// own and writes nothing: every number here is a live call into the
// feature that actually owns it (ticket, notification, audit, messaging),
// so there is nothing here that can drift out of sync with those features
// or fabricate a trend they were never asked to compute.
package workbench

import (
	"context"

	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/ticket"
)

// Overview is what every signed-in principal sees, regardless of role.
type Overview struct {
	// MyOpenTickets is how many tickets the caller submitted that are not
	// yet completed or closed — see ticket.OpenStatuses.
	MyOpenTickets int64
	// AssignedToMe is the same count, scoped to tickets currently assigned
	// to the caller. It is zero for a role that never receives
	// assignments, which is a true count, not an error.
	AssignedToMe int64
	// UnreadNotifications is the caller's current unread count.
	UnreadNotifications int64
	// Admin is present only for a caller holding at least one of the
	// permissions it depends on (ticket:read_all, audit:read,
	// messaging:read); each of its own fields is further conditioned on
	// the specific permission it needs, so one admin-ish role that holds
	// only some of them still gets the parts it can see.
	Admin *AdminOverview
}

// AdminOverview is the admin-facing section: an org-wide ticket
// breakdown, a look at recent management actions, and whether anything
// about async delivery currently needs attention.
type AdminOverview struct {
	// TicketsByStatus is present when the caller holds ticket:read_all —
	// see ticket.Service.StatusOverview.
	TicketsByStatus map[ticket.Status]int64
	// RecentAuditEvents is present when the caller holds audit:read: the
	// 10 most recent audit_events rows across every feature, newest
	// first — "最近管理操作."
	RecentAuditEvents []audit.Record
	// Outbox is present when the caller holds messaging:read: the current
	// depth and failure counts of the transactional outbox — "系统异常摘要."
	// Nil, not a zeroed struct, when messaging is disabled or the
	// permission is absent, so the frontend can tell "nothing to report"
	// apart from "not shown to you."
	Outbox *messaging.OutboxStatus
}

// recentAuditEventCount is how many recent management actions the
// overview surfaces — enough to be useful on a dashboard card without
// turning it into the full audit log, which already has its own page.
const recentAuditEventCount = 10

// Tickets is the narrow slice of ticket.Service the workbench depends on
// — declared here rather than importing *ticket.Service directly so this
// package can be tested against a fake (ADR-0002).
type Tickets interface {
	CountOpen(ctx context.Context, scope ticket.Scope) (int64, error)
	StatusOverview(ctx context.Context) (map[ticket.Status]int64, error)
}

// Notifications is the narrow slice of notification.Service the workbench
// depends on.
type Notifications interface {
	UnreadCount(ctx context.Context) (int64, error)
}

// AuditReader is the narrow slice of audit.Service the workbench depends
// on. It already self-checks audit:read (see audit.Service.List), so the
// permission gate here exists only to decide whether to call it at all,
// not to re-authorize the call.
type AuditReader interface {
	List(ctx context.Context, f audit.Filter, page, pageSize int) (audit.Page, error)
}

// MessagingHealth is the narrow slice of *messaging.Admin the workbench
// depends on. Unlike AuditReader, OutboxStatus performs no permission
// check of its own — messaging.Admin is only ever reached through a
// requirePermission-gated HTTP route today — so the gate below is load-
// bearing here, not a redundant belt-and-braces check.
type MessagingHealth interface {
	OutboxStatus(ctx context.Context) (messaging.OutboxStatus, error)
}

// Service holds the workbench use case.
type Service struct {
	tickets         Tickets
	notifications   Notifications
	auditReader     AuditReader
	messagingHealth MessagingHealth
}

// NewService wires the service.
func NewService(tickets Tickets, notifications Notifications, auditReader AuditReader, messagingHealth MessagingHealth) *Service {
	return &Service{tickets: tickets, notifications: notifications, auditReader: auditReader, messagingHealth: messagingHealth}
}

// Overview builds the caller's workbench snapshot.
func (s *Service) Overview(ctx context.Context) (Overview, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return Overview{}, auth.ErrMissingCredentials()
	}

	myOpen, err := s.tickets.CountOpen(ctx, ticket.ScopeMine)
	if err != nil {
		return Overview{}, err
	}
	assigned, err := s.tickets.CountOpen(ctx, ticket.ScopeAssigned)
	if err != nil {
		return Overview{}, err
	}
	unread, err := s.notifications.UnreadCount(ctx)
	if err != nil {
		return Overview{}, err
	}

	out := Overview{MyOpenTickets: myOpen, AssignedToMe: assigned, UnreadNotifications: unread}

	if principal.Role.Can(auth.PermTicketReadAll) {
		counts, err := s.tickets.StatusOverview(ctx)
		if err != nil {
			return Overview{}, err
		}
		out.admin().TicketsByStatus = counts
	}
	if principal.Role.Can(auth.PermAuditRead) {
		page, err := s.auditReader.List(ctx, audit.Filter{}, 1, recentAuditEventCount)
		if err != nil {
			return Overview{}, err
		}
		out.admin().RecentAuditEvents = page.Items
	}
	if principal.Role.Can(auth.PermMessagingRead) {
		status, err := s.messagingHealth.OutboxStatus(ctx)
		if err != nil {
			return Overview{}, err
		}
		out.admin().Outbox = &status
	}

	return out, nil
}

// admin returns o.Admin, allocating it on first use so the three permission
// branches above can each populate their own field without needing to know
// whether an earlier branch already allocated it.
func (o *Overview) admin() *AdminOverview {
	if o.Admin == nil {
		o.Admin = &AdminOverview{}
	}
	return o.Admin
}
