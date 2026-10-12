// Package messaginghttp is the HTTP adapter for the messaging admin feature:
// outbox status and dead-letter triage, request and response types, and the
// handlers that map between them and the service.
package messaginghttp

import (
	"encoding/json"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
)

// outboxStatusResponse is the body of GET /admin/outbox/status.
type outboxStatusResponse struct {
	Pending       int64 `json:"pending"`
	Published     int64 `json:"published"`
	Failing       int64 `json:"failing"`
	WorstAttempts int32 `json:"worst_attempts"`
}

func toOutboxStatusResponse(s messaging.OutboxStatus) outboxStatusResponse {
	return outboxStatusResponse{
		Pending:       s.Pending,
		Published:     s.Published,
		Failing:       s.Failing,
		WorstAttempts: s.WorstAttempts,
	}
}

// deadLetterResponse is one row of GET /admin/dlq.
//
// Payload is included, unlike `make dlq-list`'s truncated last_error-only
// view: an operator deciding whether a message is safe to replay needs to
// see what it actually contains, not just why it failed.
type deadLetterResponse struct {
	ID         string          `json:"id"`
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	Payload    json.RawMessage `json:"payload"`
	Consumer   string          `json:"consumer"`
	Attempts   int32           `json:"attempts"`
	LastError  string          `json:"last_error"`
	FailedAt   time.Time       `json:"failed_at"`
	ReplayedAt *time.Time      `json:"replayed_at,omitempty"`
}

func toDeadLetterResponse(d messaging.DeadLetter) deadLetterResponse {
	return deadLetterResponse{
		ID:         d.ID.String(),
		EventID:    d.Event.ID.String(),
		EventType:  d.Event.Type,
		Payload:    d.Event.Payload,
		Consumer:   d.Consumer,
		Attempts:   d.Attempts,
		LastError:  d.LastError,
		FailedAt:   d.FailedAt,
		ReplayedAt: d.ReplayedAt,
	}
}

// deadLetterListResponse wraps a page of dead letters.
//
// A wrapper appears here, for the same reason it appears on the user list
// (ADR-0004): a bare array would leave nowhere to grow — which is exactly
// what happened once already, before `total` existed and a client had no
// way to tell "this is everything" from "this is the first N of many".
// There is no page_info beyond that: this is a bounded triage queue an
// operator reads end to end, not a paginated resource — see
// messaging.DefaultDeadLetterLimit.
type deadLetterListResponse struct {
	Items []deadLetterResponse `json:"items"`
	Total int64                `json:"total"`
}

func toDeadLetterListResponse(page messaging.DeadLetterPage) deadLetterListResponse {
	// Allocated even when empty so the response carries [] and not null.
	items := make([]deadLetterResponse, 0, len(page.Items))
	for _, d := range page.Items {
		items = append(items, toDeadLetterResponse(d))
	}
	return deadLetterListResponse{Items: items, Total: page.Total}
}
