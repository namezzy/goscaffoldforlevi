import type { Role } from '../api/types'

// The access token carries only the subject and role (see openapi.yaml's
// bearerAuth description) — no email, no name. Anything else the UI needs
// about the current user has to be fetched from the API.
export interface AccessTokenPayload {
  sub: string
  role: Role
  exp: number
  iat: number
}

export function decodeAccessToken(token: string): AccessTokenPayload | null {
  try {
    const payload = token.split('.')[1]
    if (!payload) return null
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/')
    const json = atob(base64)
    return JSON.parse(json) as AccessTokenPayload
  } catch {
    return null
  }
}
