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
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toResponse(u user.User) userResponse {
	return userResponse{
		ID:        u.ID.String(),
		Email:     u.Email,
		Name:      u.Name,
		Status:    u.Status.String(),
		Role:      u.Role.String(),
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// createUserRequest is the body of POST /users.
type createUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	// Role is optional; omitting it creates an ordinary user. Defaulting to the
	// least privileged value means a forgotten field cannot create an admin.
	Role string `json:"role"`
}

func (r createUserRequest) toInput() user.CreateInput {
	role := auth.Role(r.Role)
	if r.Role == "" {
		role = auth.RoleUser
	}
	return user.CreateInput{
		Email:    r.Email,
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
	Name   *string `json:"name"`
	Status *string `json:"status"`
	Role   *string `json:"role"`
}

func (r updateUserRequest) toPatch() user.Patch {
	p := user.Patch{Name: r.Name}
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
