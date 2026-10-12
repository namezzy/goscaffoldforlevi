// Package tickethttp is the HTTP adapter for the ticket feature: request
// and response types, and the handlers that map between them and the
// service.
package tickethttp

import (
	"encoding/json"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/ticket"
)

// ticketResponse is the public representation of a ticket.
type ticketResponse struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Category    string     `json:"category"`
	Priority    string     `json:"priority"`
	Status      string     `json:"status"`
	RequesterID string     `json:"requester_id"`
	AssigneeID  *string    `json:"assignee_id,omitempty"`
	Version     int32      `json:"version"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
}

func toResponse(t ticket.Ticket) ticketResponse {
	return ticketResponse{
		ID: t.ID, Title: t.Title, Description: t.Description, Category: t.Category,
		Priority: string(t.Priority), Status: string(t.Status), RequesterID: t.RequesterID,
		AssigneeID: t.AssigneeID, Version: t.Version,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, ClosedAt: t.ClosedAt,
	}
}

type pageInfo struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// listResponse wraps a page of tickets — see userhttp.listResponse for why
// a list, and only a list, gets an envelope (ADR-0004).
type listResponse struct {
	Items    []ticketResponse `json:"items"`
	PageInfo pageInfo         `json:"page_info"`
}

func toListResponse(p ticket.Page) listResponse {
	items := make([]ticketResponse, 0, len(p.Items))
	for _, t := range p.Items {
		items = append(items, toResponse(t))
	}
	return listResponse{
		Items:    items,
		PageInfo: pageInfo{Page: p.Page, PageSize: p.PageSize, Total: p.Total, TotalPages: p.TotalPages},
	}
}

// createTicketRequest is the body of POST /tickets.
type createTicketRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Priority    string `json:"priority"`
}

func (r createTicketRequest) toInput() ticket.NewTicketInput {
	return ticket.NewTicketInput{
		Title: r.Title, Description: r.Description, Category: r.Category, Priority: ticket.Priority(r.Priority),
	}
}

// assignRequest is the body of POST /tickets/{id}/assign and .../reassign.
type assignRequest struct {
	AssigneeID      string `json:"assignee_id"`
	ExpectedVersion int32  `json:"expected_version"`
}

// noteRequest is the body of POST /tickets/{id}/notes, .../request-info and
// .../provide-info: an action that carries an explanatory note and, like
// every mutating ticket action, the version the client last saw.
type noteRequest struct {
	Note            string `json:"note"`
	ExpectedVersion int32  `json:"expected_version"`
}

// completeRequest is the body of POST /tickets/{id}/complete. The note is
// optional — a straightforward resolution may need no comment beyond the
// processing notes already on the timeline.
type completeRequest struct {
	Note            string `json:"note"`
	ExpectedVersion int32  `json:"expected_version"`
}

// closeRequest is the body of POST /tickets/{id}/close.
type closeRequest struct {
	ExpectedVersion int32 `json:"expected_version"`
}

// statusOverviewResponse is the body of GET /tickets/overview.
type statusOverviewResponse struct {
	Counts map[string]int64 `json:"counts"`
}

func toStatusOverviewResponse(counts map[ticket.Status]int64) statusOverviewResponse {
	out := make(map[string]int64, len(counts))
	for status, n := range counts {
		out[string(status)] = n
	}
	return statusOverviewResponse{Counts: out}
}

// activityRecordResponse is the public representation of one timeline
// entry — a copy of audithttp's recordResponse rather than a shared type,
// since the two HTTP adapters must not depend on each other (ADR-0001:
// each feature package's HTTP layer owns its own wire shapes).
type activityRecordResponse struct {
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

func toActivityResponse(records []audit.Record) []activityRecordResponse {
	out := make([]activityRecordResponse, 0, len(records))
	for _, r := range records {
		out = append(out, activityRecordResponse{
			ID: r.ID, ActorID: r.ActorID, Action: r.Action, TargetType: r.TargetType, TargetID: r.TargetID,
			Result: string(r.Result), Before: r.Before, After: r.After, CreatedAt: r.CreatedAt,
		})
	}
	return out
}
