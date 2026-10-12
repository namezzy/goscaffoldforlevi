// Package orghttp is the HTTP adapter for the org feature.
package orghttp

import (
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/org"
)

type organizationResponse struct {
	Name       string    `json:"name"`
	Identifier string    `json:"identifier"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func toOrgResponse(o org.Organization) organizationResponse {
	return organizationResponse{Name: o.Name, Identifier: o.Identifier, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt}
}

type updateOrganizationRequest struct {
	Name       *string `json:"name"`
	Identifier *string `json:"identifier"`
}

func (r updateOrganizationRequest) toPatch() org.OrganizationPatch {
	return org.OrganizationPatch{Name: r.Name, Identifier: r.Identifier}
}

type notificationChannelResponse struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	Detail     string `json:"detail"`
}

func toNotificationChannelListResponse(channels []org.NotificationChannel) []notificationChannelResponse {
	out := make([]notificationChannelResponse, 0, len(channels))
	for _, c := range channels {
		out = append(out, notificationChannelResponse{Name: c.Name, Configured: c.Configured, Detail: c.Detail})
	}
	return out
}

type departmentResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ParentID  *string   `json:"parent_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toDepartmentResponse(d org.Department) departmentResponse {
	return departmentResponse{
		ID: d.ID, Name: d.Name, ParentID: d.ParentID, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

type departmentListResponse struct {
	Items []departmentResponse `json:"items"`
}

func toDepartmentListResponse(ds []org.Department) departmentListResponse {
	items := make([]departmentResponse, 0, len(ds))
	for _, d := range ds {
		items = append(items, toDepartmentResponse(d))
	}
	return departmentListResponse{Items: items}
}

type createDepartmentRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
}

func (r createDepartmentRequest) toInput() org.NewDepartmentInput {
	return org.NewDepartmentInput{Name: r.Name, ParentID: r.ParentID}
}

// updateDepartmentRequest carries an explicit clear_parent flag rather than
// treating a null parent_id as "clear": every field here is already a
// pointer for ordinary PATCH semantics (absent means unchanged), which
// leaves no remaining value to mean "explicitly set to null" — see
// org.DepartmentPatch.
type updateDepartmentRequest struct {
	Name        *string `json:"name"`
	ParentID    *string `json:"parent_id"`
	ClearParent bool    `json:"clear_parent"`
}

func (r updateDepartmentRequest) toPatch() org.DepartmentPatch {
	return org.DepartmentPatch{Name: r.Name, ParentID: r.ParentID, ClearParent: r.ClearParent}
}
