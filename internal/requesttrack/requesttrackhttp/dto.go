// Package requesttrackhttp is the HTTP adapter for the live request feed:
// request and response types, and the handler that maps between them and
// requesttrack.Tracker.
package requesttrackhttp

import (
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/requesttrack"
)

// eventResponse is the wire shape of one tracked request, sent both in the
// JSON list and as the payload of each SSE message — the two views of the
// same feed carry the same shape on purpose, so a client written against one
// needs no translation to read the other.
type eventResponse struct {
	Time       time.Time `json:"time"`
	RequestID  string    `json:"request_id"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	Bytes      int64     `json:"bytes"`
	DurationMS float64   `json:"duration_ms"`
	// IP and the country fields are omitted rather than sent empty when
	// unresolved — see package geoip — so a client can tell "unknown" apart
	// from an empty string without a sentinel value.
	IP          string `json:"ip,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	CountryName string `json:"country_name,omitempty"`
}

func toEventResponse(e requesttrack.Event) eventResponse {
	return eventResponse{
		Time:        e.Time,
		RequestID:   e.RequestID,
		Method:      e.Method,
		Path:        e.Path,
		Status:      e.Status,
		Bytes:       e.Bytes,
		DurationMS:  e.DurationMS,
		IP:          e.IP,
		CountryCode: e.CountryCode,
		CountryName: e.CountryName,
	}
}

// eventListResponse is the body of GET /admin/requests when Accept does not
// ask for a stream.
type eventListResponse struct {
	Items []eventResponse `json:"items"`
}

func toEventListResponse(events []requesttrack.Event) eventListResponse {
	// Allocated even when empty so the response carries [] and not null.
	items := make([]eventResponse, 0, len(events))
	for _, e := range events {
		items = append(items, toEventResponse(e))
	}
	return eventListResponse{Items: items}
}
