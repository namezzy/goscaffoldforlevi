// Package userhttp is the HTTP adapter for the user feature: request and
// response types, and the handlers that map between them and the service.
package userhttp

import (
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
)

// userResponse is the public representation of a user.
//
// It exists separately from user.User for one concrete reason: user.User
// carries PasswordHash. Serialising the entity directly would publish it, and
// the only thing standing between that and a leak would be a struct tag that
// someone must remember to write. Here, the field simply has nowhere to go.
type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	// Username is omitted rather than sent as "" for an account that never
	// set one — a client checking `if user.username` should not have to also
	// know that empty string means the same as absent.
	Username     *string   `json:"username,omitempty"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Role         string    `json:"role"`
	DepartmentID *string   `json:"department_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toResponse(u user.User) userResponse {
	resp := userResponse{
		ID:           u.ID.String(),
		Email:        u.Email,
		Name:         u.Name,
		Status:       u.Status.String(),
		Role:         u.Role.String(),
		DepartmentID: u.DepartmentID,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
	if u.Username != "" {
		resp.Username = &u.Username
	}
	return resp
}

// createUserRequest is the body of POST /users.
type createUserRequest struct {
	Email string `json:"email"`
	// Username is optional; an account can be created, and can operate
	// indefinitely, without one. Login by username only works once it is set.
	Username string `json:"username"`
	Name     string `json:"name"`
	Password string `json:"password"`
	// Role is optional; omitting it creates an ordinary user. Defaulting to the
	// least privileged value means a forgotten field cannot create an admin.
	Role string `json:"role"`
}

func (r createUserRequest) toInput() user.CreateInput {
	role := auth.Role(r.Role)
	if r.Role == "" {
		role = auth.RoleReadOnly
	}
	return user.CreateInput{
		Email:    r.Email,
		Username: r.Username,
		Name:     r.Name,
		Password: r.Password,
		Role:     role,
	}
}

// updateUserRequest is the body of PATCH /users/{id}.
//
// Every field is a pointer so that "absent" and "present but empty" are
// distinguishable. Without that, clearing a field and not mentioning it look
// identical on the wire, and PATCH becomes PUT with extra steps.
type updateUserRequest struct {
	Email        *string `json:"email"`
	Username     *string `json:"username"`
	Name         *string `json:"name"`
	Status       *string `json:"status"`
	Role         *string `json:"role"`
	DepartmentID *string `json:"department_id"`
	// ClearDepartment unassigns the member's department. It is a separate
	// field from DepartmentID, not "send department_id: null", for the same
	// reason every other field here is a pointer: this patch already needs
	// to distinguish "absent" from "present", and department_id needs a
	// third state on top of that — assign, or explicitly clear — which one
	// field cannot express, matching departments.parent_id/clear_parent in
	// api/openapi.yaml's UpdateDepartmentRequest.
	ClearDepartment bool `json:"clear_department"`
}

func (r updateUserRequest) toPatch() user.Patch {
	p := user.Patch{
		Email: r.Email, Username: r.Username, Name: r.Name,
		DepartmentID: r.DepartmentID, ClearDepartment: r.ClearDepartment,
	}
	if r.Status != nil {
		s := user.Status(*r.Status)
		p.Status = &s
	}
	if r.Role != nil {
		role := auth.Role(*r.Role)
		p.Role = &role
	}
	return p
}

// changePasswordRequest is the body of PUT /users/{id}/password.
type changePasswordRequest struct {
	Password string `json:"password"`
}

// pageInfo is the pagination metadata carried alongside a list.
type pageInfo struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// listResponse wraps a page of users.
//
// This is the one place a wrapper appears. Single resources are returned bare
// (ADR-0004); a list needs somewhere to put the totals, and inventing a global
// envelope to serve that one case would tax every other endpoint.
type listResponse struct {
	Items    []userResponse `json:"items"`
	PageInfo pageInfo       `json:"page_info"`
}

func toListResponse(p user.Page) listResponse {
	// The slice is allocated even when empty so that the response carries [] and
	// not null: a client iterating the result should not have to null-check.
	items := make([]userResponse, 0, len(p.Items))
	for _, u := range p.Items {
		items = append(items, toResponse(u))
	}
	return listResponse{
		Items: items,
		PageInfo: pageInfo{
			Page:       p.Page,
			PageSize:   p.PageSize,
			Total:      p.Total,
			TotalPages: p.TotalPages,
		},
	}
}
