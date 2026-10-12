// Package invitehttp is the HTTP adapter for the invite feature.
package invitehttp

import (
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/invite"
)

// invitationResponse is the public representation of an invitation, as
// returned by Get, List and Revoke. There is no token field here: the
// plaintext is never persisted — see invite.Repository's doc comment — so by
// the time these paths run it no longer exists anywhere to include.
type invitationResponse struct {
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	DepartmentID   *string    `json:"department_id,omitempty"`
	Role           string     `json:"role"`
	Status         string     `json:"status"`
	InvitedBy      string     `json:"invited_by"`
	AcceptedUserID *string    `json:"accepted_user_id,omitempty"`
	ExpiresAt      time.Time  `json:"expires_at"`
	AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func toResponse(inv invite.Invitation) invitationResponse {
	return invitationResponse{
		ID: inv.ID, Email: inv.Email, DepartmentID: inv.DepartmentID,
		Role: inv.Role.String(), Status: string(inv.Status), InvitedBy: inv.InvitedBy,
		AcceptedUserID: inv.AcceptedUserID, ExpiresAt: inv.ExpiresAt,
		AcceptedAt: inv.AcceptedAt, RevokedAt: inv.RevokedAt,
		CreatedAt: inv.CreatedAt, UpdatedAt: inv.UpdatedAt,
	}
}

// createdInvitationResponse is what Create and Reinvite answer with: the
// invitation plus the one-time plaintext token, embedded rather than
// duplicated field-by-field. See invite.Service.Invite's doc comment for why
// handing the token back here, just this once, is intentional rather than a
// leak of something meant to stay secret.
type createdInvitationResponse struct {
	invitationResponse
	Token string `json:"token"`
}

func toCreatedResponse(inv invite.Invitation, token string) createdInvitationResponse {
	return createdInvitationResponse{invitationResponse: toResponse(inv), Token: token}
}

type pageInfo struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// listResponse wraps a page of invitations — see userhttp.listResponse for
// why a list, and only a list, gets an envelope (ADR-0004).
type listResponse struct {
	Items    []invitationResponse `json:"items"`
	PageInfo pageInfo             `json:"page_info"`
}

func toListResponse(p invite.Page) listResponse {
	items := make([]invitationResponse, 0, len(p.Items))
	for _, inv := range p.Items {
		items = append(items, toResponse(inv))
	}
	return listResponse{
		Items: items,
		PageInfo: pageInfo{
			Page: p.Page, PageSize: p.PageSize, Total: p.Total, TotalPages: p.TotalPages,
		},
	}
}

// createInvitationRequest is the body of POST /invitations.
type createInvitationRequest struct {
	Email        string  `json:"email"`
	DepartmentID *string `json:"department_id"`
	Role         string  `json:"role"`
}

func (r createInvitationRequest) toInput() invite.NewInput {
	return invite.NewInput{Email: r.Email, DepartmentID: r.DepartmentID, Role: auth.Role(r.Role)}
}

// acceptInvitationRequest is the body of POST /invitations/accept.
type acceptInvitationRequest struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (r acceptInvitationRequest) toInput() invite.AcceptInput {
	return invite.AcceptInput{Token: r.Token, Name: r.Name, Password: r.Password}
}

// acceptInvitationResponse confirms the account exists. It carries no
// tokens: the newly provisioned member logs in through POST /auth/login like
// anyone else, rather than this endpoint silently starting a session — a
// path with its own security review would be needed to hand out credentials
// from an unauthenticated endpoint, and phase 1 does not require it.
type acceptInvitationResponse struct {
	MemberID string `json:"member_id"`
}
