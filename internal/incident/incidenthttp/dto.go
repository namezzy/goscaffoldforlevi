// Package incidenthttp is the HTTP adapter for the exception/processing
// center: request and response types, and the handlers that map between
// them and the service.
package incidenthttp

import (
	"encoding/json"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/audit"
	"github.com/namezzy/goscaffoldforlevi/internal/incident"
)

// incidentResponse is the public representation of an incident.
type incidentResponse struct {
	ID              string     `json:"id"`
	Fingerprint     string     `json:"fingerprint"`
	Severity        string     `json:"severity"`
	Status          string     `json:"status"`
	OwnerID         *string    `json:"owner_id,omitempty"`
	DeadLetterID    string     `json:"dead_letter_id"`
	EventType       string     `json:"event_type"`
	Consumer        string     `json:"consumer"`
	LastError       string     `json:"last_error"`
	OccurrenceCount int32      `json:"occurrence_count"`
	FirstSeenAt     time.Time  `json:"first_seen_at"`
	LastSeenAt      time.Time  `json:"last_seen_at"`
	AcknowledgedAt  *time.Time `json:"acknowledged_at,omitempty"`
	AcknowledgedBy  *string    `json:"acknowledged_by,omitempty"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy      *string    `json:"resolved_by,omitempty"`
	// ReplayStatus is null until a replay has been requested. "pending"
	// must never be read as success — see incident.ReplayStatus's doc
	// comment.
	ReplayStatus      *string    `json:"replay_status,omitempty"`
	ReplayRequestedAt *time.Time `json:"replay_requested_at,omitempty"`
	ReplayResultAt    *time.Time `json:"replay_result_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func toResponse(inc incident.Incident) incidentResponse {
	resp := incidentResponse{
		ID: inc.ID, Fingerprint: inc.Fingerprint, Severity: string(inc.Severity), Status: string(inc.Status),
		OwnerID: inc.OwnerID, DeadLetterID: inc.DeadLetterID, EventType: inc.EventType, Consumer: inc.Consumer,
		LastError: inc.LastError, OccurrenceCount: inc.OccurrenceCount,
		FirstSeenAt: inc.FirstSeenAt, LastSeenAt: inc.LastSeenAt,
		AcknowledgedAt: inc.AcknowledgedAt, AcknowledgedBy: inc.AcknowledgedBy,
		ResolvedAt: inc.ResolvedAt, ResolvedBy: inc.ResolvedBy,
		ReplayRequestedAt: inc.ReplayRequestedAt, ReplayResultAt: inc.ReplayResultAt,
		CreatedAt: inc.CreatedAt, UpdatedAt: inc.UpdatedAt,
	}
	if inc.ReplayStatus != nil {
		s := string(*inc.ReplayStatus)
		resp.ReplayStatus = &s
	}
	return resp
}

type pageInfo struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// listResponse wraps a page of incidents — see userhttp.listResponse for why
// a list, and only a list, gets an envelope (ADR-0004).
type listResponse struct {
	Items    []incidentResponse `json:"items"`
	PageInfo pageInfo           `json:"page_info"`
}

func toListResponse(p incident.Page) listResponse {
	items := make([]incidentResponse, 0, len(p.Items))
	for _, inc := range p.Items {
		items = append(items, toResponse(inc))
	}
	return listResponse{
		Items:    items,
		PageInfo: pageInfo{Page: p.Page, PageSize: p.PageSize, Total: p.Total, TotalPages: p.TotalPages},
	}
}

// summaryResponse is the body of GET /incidents/summary: how many
// non-resolved incidents sit at each severity.
type summaryResponse struct {
	Counts map[string]int64 `json:"counts"`
}

func toSummaryResponse(counts map[incident.Severity]int64) summaryResponse {
	out := make(map[string]int64, len(counts))
	for severity, n := range counts {
		out[string(severity)] = n
	}
	return summaryResponse{Counts: out}
}

// setSeverityRequest is the body of POST /incidents/{id}/severity.
type setSeverityRequest struct {
	Severity string `json:"severity"`
}

// assignOwnerRequest is the body of POST /incidents/{id}/owner. An empty
// OwnerID clears the assignment.
type assignOwnerRequest struct {
	OwnerID string `json:"owner_id"`
}

// noteRequest is the body of POST /incidents/{id}/notes.
type noteRequest struct {
	Note string `json:"note"`
}

// activityRecordResponse is the public representation of one processing
// record — a copy of tickethttp's activityRecordResponse rather than a
// shared type, since each feature's HTTP layer owns its own wire shapes
// (ADR-0001).
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
