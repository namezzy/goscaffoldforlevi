// Package audithttp is the HTTP adapter for the audit feature: read-only —
// there is no create/update/delete request type here because the log itself
// has no write endpoint, see package audit.
package audithttp

import (
	"encoding/json"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/audit"
)

// recordResponse is the public representation of one audit event.
type recordResponse struct {
	ID         string          `json:"id"`
	ActorID    *string         `json:"actor_id,omitempty"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type"`
	TargetID   string          `json:"target_id"`
	Result     string          `json:"result"`
	RequestID  string          `json:"request_id,omitempty"`
	Before     json.RawMessage `json:"before,omitempty"`
	After      json.RawMessage `json:"after,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

func toResponse(r audit.Record) recordResponse {
	return recordResponse{
		ID: r.ID, ActorID: r.ActorID, Action: r.Action,
		TargetType: r.TargetType, TargetID: r.TargetID, Result: string(r.Result),
		RequestID: r.RequestID, Before: r.Before, After: r.After, CreatedAt: r.CreatedAt,
	}
}

type pageInfo struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// listResponse wraps a page of audit events — see userhttp.listResponse for
// why a list, and only a list, gets an envelope (ADR-0004).
type listResponse struct {
	Items    []recordResponse `json:"items"`
	PageInfo pageInfo         `json:"page_info"`
}

func toListResponse(p audit.Page) listResponse {
	items := make([]recordResponse, 0, len(p.Items))
	for _, r := range p.Items {
		items = append(items, toResponse(r))
	}
	return listResponse{
		Items: items,
		PageInfo: pageInfo{
			Page: p.Page, PageSize: p.PageSize, Total: p.Total, TotalPages: p.TotalPages,
		},
	}
}
