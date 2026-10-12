import { ensureFreshToken, forceLogout } from '../auth/store'
import { API_BASE_URL } from './config'
import { ApiError } from './errors'
import { rawRequest } from './http'

/** Wraps rawRequest with a bearer token, refreshing it first if it is close to expiry. */
export async function authedRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = await ensureFreshToken()
  try {
    return await rawRequest<T>(path, {
      ...init,
      headers: {
        ...init.headers,
        Authorization: `Bearer ${token}`,
      },
    })
  } catch (err) {
    // A 401 here means the token was rejected for a reason a refresh cannot
    // fix (revoked, tampered) — the response never says which. Ownership
    // failures are 403, not 401, so this never fires for those.
    if (err instanceof ApiError && err.status === 401) {
      forceLogout()
    }
    throw err
  }
}

/**
 * Authenticated fetch that returns the raw Response instead of parsing it —
 * for a streaming body (SSE), where authedRequest's JSON parsing does not
 * apply. Native EventSource cannot carry an Authorization header, which is
 * the whole reason this exists instead of just using it.
 */
export async function authedStream(path: string, signal: AbortSignal): Promise<Response> {
  const token = await ensureFreshToken()
  const res = await fetch(`${API_BASE_URL}${path}`, {
    headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' },
    signal,
  })
  if (!res.ok) {
    if (res.status === 401) forceLogout()
    const body = await res.json().catch(() => ({ error: { code: 'UNKNOWN', message: res.statusText } }))
    throw new ApiError(res.status, body)
  }
  return res
}
