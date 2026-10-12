// Package notificationhttp is the HTTP adapter for the notification
// feature. Every route is implicitly scoped to the caller's own account —
// see package notification's doc comment — so there is nothing here that
// takes a target user id.
package notificationhttp

import (
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/notification"
)

type notificationResponse struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	TargetType string     `json:"target_type"`
	TargetID   string     `json:"target_id"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func toResponse(n notification.Notification) notificationResponse {
	return notificationResponse{
		ID: n.ID, Type: n.Type, Title: n.Title, Body: n.Body,
		TargetType: n.TargetType, TargetID: n.TargetID, ReadAt: n.ReadAt, CreatedAt: n.CreatedAt,
	}
}

type pageInfo struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type listResponse struct {
	Items    []notificationResponse `json:"items"`
	PageInfo pageInfo               `json:"page_info"`
}

func toListResponse(p notification.Page) listResponse {
	items := make([]notificationResponse, 0, len(p.Items))
	for _, n := range p.Items {
		items = append(items, toResponse(n))
	}
	return listResponse{
		Items:    items,
		PageInfo: pageInfo{Page: p.Page, PageSize: p.PageSize, Total: p.Total, TotalPages: p.TotalPages},
	}
}

type unreadCountResponse struct {
	Count int64 `json:"count"`
}
