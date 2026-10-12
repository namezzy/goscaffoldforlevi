import { API_BASE_URL } from './config'
import { ApiError } from './errors'
import type { ApiErrorBody } from './types'

// How long a request waits before this client gives up on it — independent
// of whatever timeout the server or a proxy in between enforces. Without
// this, a request that never gets a response (a hung connection, a
// firewall silently dropping packets) leaves the caller's "loading" state
// spinning forever instead of reaching an error a user can act on.
const DEFAULT_TIMEOUT_MS = 15_000

function isApiErrorBody(value: unknown): value is ApiErrorBody {
  if (typeof value !== 'object' || value === null) return false
  const error = (value as { error?: unknown }).error
  return (
    typeof error === 'object' &&
    error !== null &&
    typeof (error as { code?: unknown }).code === 'string' &&
    typeof (error as { message?: unknown }).message === 'string'
  )
}

// The transport every other api/* module builds on. It knows nothing about
// auth — login, refresh and logout all call this directly, since none of
// them carry a bearer token.
//
// Every request gets an internal timeout, layered underneath any signal the
// caller already passed in `init.signal` — either one aborting the request
// aborts it, and the two are reported as distinct ApiError codes so a
// caller can tell "the user cancelled this" apart from "this took too long".
export async function rawRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const controller = new AbortController()
  let timedOut = false
  const timeoutId = setTimeout(() => {
    timedOut = true
    controller.abort()
  }, DEFAULT_TIMEOUT_MS)

  const externalSignal = init.signal
  if (externalSignal) {
    if (externalSignal.aborted) controller.abort()
    else externalSignal.addEventListener('abort', () => controller.abort(), { once: true })
  }

  let res: Response
  try {
    res = await fetch(`${API_BASE_URL}${path}`, {
      ...init,
      signal: controller.signal,
      headers: {
        'Content-Type': 'application/json',
        ...init.headers,
      },
    })
  } catch {
    clearTimeout(timeoutId)
    if (controller.signal.aborted) {
      if (timedOut) {
        throw new ApiError(0, { error: { code: 'REQUEST_TIMEOUT', message: '请求超时，请重试' } })
      }
      throw new ApiError(0, { error: { code: 'REQUEST_CANCELLED', message: '请求已取消' } })
    }
    // Offline, DNS failure, CORS rejection — fetch reports all of these as
    // a plain TypeError with no useful detail of its own to relay.
    throw new ApiError(0, { error: { code: 'NETWORK_ERROR', message: '网络连接失败，请检查网络后重试' } })
  }
  clearTimeout(timeoutId)

  // Every mutating endpoint in this API answers 204 with no body on success.
  if (res.status === 204) {
    return undefined as T
  }

  const text = await res.text()
  let body: unknown
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      // A non-JSON body — an intermediary proxy's HTML error page, a bare
      // text response, a truncated stream. Left undefined rather than
      // thrown here: the two branches below decide what that means
      // depending on whether the response was otherwise successful.
      body = undefined
    }
  }

  if (!res.ok) {
    const retryAfterHeader = res.headers.get('Retry-After')
    const errorBody: ApiErrorBody = isApiErrorBody(body)
      ? body
      : {
          error: {
            code: 'NON_JSON_RESPONSE',
            message: res.statusText || `请求失败（状态码 ${res.status}）`,
          },
        }
    throw new ApiError(res.status, errorBody, retryAfterHeader ? Number(retryAfterHeader) : undefined)
  }

  if (text && body === undefined) {
    // A 2xx response this client cannot parse is a contract violation, not
    // a documented API error — thrown as a plain Error so it is not
    // mistaken for one with ApiError's code/details fields.
    throw new Error('服务器返回了无法解析的响应')
  }

  return body as T
}
