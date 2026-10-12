import { authedRequest } from './client'
import { rawRequest } from './http'
import type {
  ConfirmPasswordResetRequest,
  RequestPasswordResetRequest,
  RevokeOtherSessionsResponse,
  SessionList,
  TokenPair,
} from './types'

// identifier is an email address or a username — the backend accepts both
// in the same field, see api/openapi.yaml's LoginRequest.
export function login(identifier: string, password: string): Promise<TokenPair> {
  return rawRequest<TokenPair>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ identifier, password }),
  })
}

export function refreshToken(refresh_token: string): Promise<TokenPair> {
  return rawRequest<TokenPair>('/api/v1/auth/refresh', {
    method: 'POST',
    body: JSON.stringify({ refresh_token }),
  })
}

export function logout(refresh_token: string): Promise<void> {
  return rawRequest<void>('/api/v1/auth/logout', {
    method: 'POST',
    body: JSON.stringify({ refresh_token }),
  })
}

// --- Security center: sessions ---

export function listSessions(): Promise<SessionList> {
  return authedRequest<SessionList>('/api/v1/auth/sessions')
}

export function revokeSession(id: string): Promise<void> {
  return authedRequest<void>(`/api/v1/auth/sessions/${id}`, { method: 'DELETE' })
}

export function revokeOtherSessions(): Promise<RevokeOtherSessionsResponse> {
  return authedRequest<RevokeOtherSessionsResponse>('/api/v1/auth/sessions/revoke-others', { method: 'POST' })
}

// --- Self-service password reset ---
//
// Both unauthenticated by necessity — see api/openapi.yaml's own
// description of these two operations.

export function requestPasswordReset(input: RequestPasswordResetRequest): Promise<void> {
  return rawRequest<void>('/api/v1/auth/password-reset/request', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function confirmPasswordReset(input: ConfirmPasswordResetRequest): Promise<void> {
  return rawRequest<void>('/api/v1/auth/password-reset/confirm', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
