import * as authApi from '../api/auth'
import { decodeAccessToken } from './jwt'
import type { Role } from '../api/types'

const REFRESH_TOKEN_KEY = 'goscaffold.refreshToken'

export interface AuthUser {
  id: string
  role: Role
}

export type AuthStatus = 'loading' | 'authenticated' | 'anonymous'

interface AuthState {
  status: AuthStatus
  accessToken: string | null
  refreshToken: string | null
  user: AuthUser | null
}

let state: AuthState = {
  status: 'loading',
  accessToken: null,
  refreshToken: localStorage.getItem(REFRESH_TOKEN_KEY),
  user: null,
}

const listeners = new Set<() => void>()

function emit(): void {
  for (const listener of listeners) listener()
}

export function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function getState(): AuthState {
  return state
}

let refreshTimer: ReturnType<typeof setTimeout> | undefined
let inflightRefresh: Promise<string> | null = null

function scheduleProactiveRefresh(expiresInSeconds: number): void {
  if (refreshTimer) clearTimeout(refreshTimer)
  // Refresh a minute before expiry so a user mid-action never hits a 401.
  const delayMs = Math.max((expiresInSeconds - 60) * 1000, 5_000)
  refreshTimer = setTimeout(() => {
    void refresh().catch(() => {
      // refresh() already clears the session on failure.
    })
  }, delayMs)
}

function setSession(accessToken: string, refreshToken: string, expiresInSeconds: number): void {
  const payload = decodeAccessToken(accessToken)
  state = {
    status: 'authenticated',
    accessToken,
    refreshToken,
    user: payload ? { id: payload.sub, role: payload.role } : null,
  }
  localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken)
  scheduleProactiveRefresh(expiresInSeconds)
  emit()
}

function clearSession(): void {
  if (refreshTimer) clearTimeout(refreshTimer)
  state = { status: 'anonymous', accessToken: null, refreshToken: null, user: null }
  localStorage.removeItem(REFRESH_TOKEN_KEY)
  emit()
}

/** Runs once at app startup: tries to resume a session from the stored refresh token. */
export async function init(): Promise<void> {
  if (!state.refreshToken) {
    clearSession()
    return
  }
  try {
    await refresh()
  } catch {
    // refresh() already cleared the session.
  }
}

export async function login(identifier: string, password: string): Promise<void> {
  const pair = await authApi.login(identifier, password)
  setSession(pair.access_token, pair.refresh_token, pair.expires_in)
}

export async function logout(): Promise<void> {
  const refreshToken = state.refreshToken
  clearSession()
  if (refreshToken) {
    // Best effort — the session is gone locally either way.
    await authApi.logout(refreshToken).catch(() => undefined)
  }
}

/** Called by the authenticated API client when a request comes back 401. */
export function forceLogout(): void {
  clearSession()
}

async function refresh(): Promise<string> {
  const refreshToken = state.refreshToken
  if (!refreshToken) {
    clearSession()
    throw new Error('not authenticated')
  }
  if (!inflightRefresh) {
    inflightRefresh = authApi
      .refreshToken(refreshToken)
      .then((pair) => {
        setSession(pair.access_token, pair.refresh_token, pair.expires_in)
        return pair.access_token
      })
      .catch((err: unknown) => {
        clearSession()
        throw err
      })
      .finally(() => {
        inflightRefresh = null
      })
  }
  return inflightRefresh
}

/** Returns a token that is valid for at least a few more seconds, refreshing first if needed. */
export async function ensureFreshToken(): Promise<string> {
  const { accessToken } = state
  if (accessToken) {
    const payload = decodeAccessToken(accessToken)
    const safetyMarginMs = 15_000
    if (payload && payload.exp * 1000 > Date.now() + safetyMarginMs) {
      return accessToken
    }
  }
  return refresh()
}
