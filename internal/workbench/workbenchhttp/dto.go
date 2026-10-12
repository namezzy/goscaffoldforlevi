// Package workbenchhttp is the HTTP adapter for the workbench feature: a
// single read-only endpoint, so this file is smaller than most of this
// codebase's dto.go siblings.
package workbenchhttp

import (
	"encoding/json"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/workbench"
)

type overviewResponse struct {
	MyOpenTickets       int64                  `json:"my_open_tickets"`
	AssignedToMe        int64                  `json:"assigned_to_me"`
	UnreadNotifications int64                  `json:"unread_notifications"`
	Admin               *adminOverviewResponse `json:"admin,omitempty"`
}

type adminOverviewResponse struct {
	// TicketsByStatus and RecentAuditEvents deliberately do not carry
	// omitempty: encoding/json's omitempty treats a non-nil-but-empty map
	// or slice the same as a nil one, which would make "zero tickets
	// exist" indistinguishable from "you lack ticket:read_all" — exactly
	// the distinction toOverviewResponse's nil-check below exists to
	// preserve. A nil field still serializes as JSON null, which every
	// caller here already treats the same as absent.
	TicketsByStatus   map[string]int64     `json:"tickets_by_status"`
	RecentAuditEvents []auditEventResponse `json:"recent_audit_events"`
	// Outbox is a pointer, so omitempty here only ever triggers on a true
	// nil — a zero-valued OutboxStatus still serializes as an object, not
	// this field being dropped.
	Outbox *outboxStatusResponse `json:"outbox,omitempty"`
}

// auditEventResponse is a copy of audithttp's recordResponse — see
// tickethttp's activityRecordResponse for why each HTTP adapter keeps its
// own copy rather than sharing one across feature boundaries.
type auditEventResponse struct {
	ID         string          `json:"id"`
	ActorID    *string         `json:"actor_id,omitempty"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type"`
	TargetID   string          `json:"target_id"`
	Result     string          `json:"result"`
	Before     json.RawMessage `json:"before,omitempty"`
	After      json.RawMessage `json:"after,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

type outboxStatusResponse struct {
	Pending       int64 `json:"pending"`
	Published     int64 `json:"published"`
	Failing       int64 `json:"failing"`
	WorstAttempts int32 `json:"worst_attempts"`
}

func toOverviewResponse(o workbench.Overview) overviewResponse {
	resp := overviewResponse{
		MyOpenTickets: o.MyOpenTickets, AssignedToMe: o.AssignedToMe, UnreadNotifications: o.UnreadNotifications,
	}
	if o.Admin == nil {
		return resp
	}
	admin := &adminOverviewResponse{}
	if o.Admin.TicketsByStatus != nil {
		counts := make(map[string]int64, len(o.Admin.TicketsByStatus))
		for status, n := range o.Admin.TicketsByStatus {
			counts[string(status)] = n
		}
		admin.TicketsByStatus = counts
	}
	if o.Admin.RecentAuditEvents != nil {
		events := make([]auditEventResponse, 0, len(o.Admin.RecentAuditEvents))
		for _, r := range o.Admin.RecentAuditEvents {
			events = append(events, auditEventResponse{
				ID: r.ID, ActorID: r.ActorID, Action: r.Action, TargetType: r.TargetType, TargetID: r.TargetID,
				Result: string(r.Result), Before: r.Before, After: r.After, CreatedAt: r.CreatedAt,
			})
		}
		admin.RecentAuditEvents = events
	}
	if o.Admin.Outbox != nil {
		admin.Outbox = &outboxStatusResponse{
			Pending: o.Admin.Outbox.Pending, Published: o.Admin.Outbox.Published,
			Failing: o.Admin.Outbox.Failing, WorstAttempts: o.Admin.Outbox.WorstAttempts,
		}
	}
	resp.Admin = admin
	return resp
}
