import { authedRequest } from './client'
import { rawRequest } from './http'
import type {
  AcceptInvitationRequest,
  AcceptInvitationResponse,
  CreateInvitationRequest,
  CreatedInvitation,
  Invitation,
  InvitationPage,
  ListInvitationsParams,
} from './types'

export function listInvitations(params: ListInvitationsParams): Promise<InvitationPage> {
  const query = new URLSearchParams()
  if (params.status) query.set('status', params.status)
  if (params.email) query.set('email', params.email)
  query.set('page', String(params.page ?? 1))
  query.set('page_size', String(params.page_size ?? 20))
  return authedRequest<InvitationPage>(`/api/v1/invitations?${query.toString()}`)
}

export function getInvitation(id: string): Promise<Invitation> {
  return authedRequest<Invitation>(`/api/v1/invitations/${id}`)
}

export function createInvitation(input: CreateInvitationRequest): Promise<CreatedInvitation> {
  return authedRequest<CreatedInvitation>('/api/v1/invitations', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function revokeInvitation(id: string): Promise<void> {
  return authedRequest<void>(`/api/v1/invitations/${id}/revoke`, { method: 'POST' })
}

export function reinviteMember(id: string): Promise<CreatedInvitation> {
  return authedRequest<CreatedInvitation>(`/api/v1/invitations/${id}/reinvite`, { method: 'POST' })
}

// acceptInvitation uses rawRequest, not authedRequest: the caller redeeming
// an invitation has no account yet to hold a bearer token — see
// invitehttp.Handler.AcceptRoute's doc comment on the Go side.
export function acceptInvitation(input: AcceptInvitationRequest): Promise<AcceptInvitationResponse> {
  return rawRequest<AcceptInvitationResponse>('/api/v1/invitations/accept', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
