import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from './errors'
import { rawRequest } from './http'

// A fetch stand-in that resolves/rejects like the real thing, including
// honoring the AbortSignal the way a real network request would — which is
// the behavior rawRequest's timeout and cancellation handling depends on.
function abortableFetch(run: (signal: AbortSignal) => Promise<Response>): typeof fetch {
  return vi.fn((_input: unknown, init?: RequestInit) => {
    const signal = init?.signal as AbortSignal | undefined
    return new Promise<Response>((resolve, reject) => {
      if (signal?.aborted) {
        reject(new DOMException('The operation was aborted.', 'AbortError'))
        return
      }
      signal?.addEventListener('abort', () => {
        reject(new DOMException('The operation was aborted.', 'AbortError'))
      })
      run(signal ?? new AbortController().signal).then(resolve, reject)
    })
  }) as unknown as typeof fetch
}

function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), { status, headers })
}

describe('rawRequest', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('returns the parsed body for a successful response', async () => {
    vi.stubGlobal('fetch', abortableFetch(async () => jsonResponse(200, { hello: 'world' })))
    await expect(rawRequest('/ping')).resolves.toEqual({ hello: 'world' })
  })

  it('returns undefined for a 204', async () => {
    vi.stubGlobal(
      'fetch',
      abortableFetch(async () => new Response(null, { status: 204 })),
    )
    await expect(rawRequest('/ping')).resolves.toBeUndefined()
  })

  it('throws an ApiError built from the documented error envelope', async () => {
    vi.stubGlobal(
      'fetch',
      abortableFetch(async () =>
        jsonResponse(404, { error: { code: 'USER_NOT_FOUND', message: 'user not found' } }),
      ),
    )
    await expect(rawRequest('/ping')).rejects.toMatchObject({
      status: 404,
      code: 'USER_NOT_FOUND',
      message: 'user not found',
    })
  })

  // The bug this guards: an intermediary proxy's HTML error page (a 502
  // from a load balancer, say) used to reach JSON.parse with no try/catch,
  // so the caller saw a raw SyntaxError instead of something it could
  // display or branch on.
  it('wraps a non-JSON error body in an ApiError instead of throwing a raw parse error', async () => {
    vi.stubGlobal(
      'fetch',
      abortableFetch(
        async () => new Response('<html><body>502 Bad Gateway</body></html>', { status: 502 }),
      ),
    )
    const err = await rawRequest('/ping').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).code).toBe('NON_JSON_RESPONSE')
    expect((err as ApiError).status).toBe(502)
  })

  it('falls back to NON_JSON_RESPONSE for a JSON error body that is not the documented shape', async () => {
    vi.stubGlobal(
      'fetch',
      abortableFetch(async () => jsonResponse(500, { message: 'internal server error' })),
    )
    const err = await rawRequest('/ping').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).code).toBe('NON_JSON_RESPONSE')
  })

  it('throws a plain Error, not an ApiError, for a 2xx response with an unparseable body', async () => {
    vi.stubGlobal(
      'fetch',
      abortableFetch(async () => new Response('not json', { status: 200 })),
    )
    const err = await rawRequest('/ping').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(Error)
    expect(err).not.toBeInstanceOf(ApiError)
  })

  it('reads Retry-After off a rate-limited response', async () => {
    vi.stubGlobal(
      'fetch',
      abortableFetch(async () =>
        jsonResponse(429, { error: { code: 'RATE_LIMITED', message: 'too many requests' } }, { 'Retry-After': '30' }),
      ),
    )
    const err = await rawRequest('/ping').catch((e: unknown) => e)
    expect((err as ApiError).retryAfterSeconds).toBe(30)
  })

  it('times out a request that never resolves', async () => {
    vi.stubGlobal(
      'fetch',
      abortableFetch(() => new Promise<Response>(() => {})),
    )
    const pending = rawRequest('/ping').catch((e: unknown) => e)
    await vi.advanceTimersByTimeAsync(15_000)
    const err = await pending
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).code).toBe('REQUEST_TIMEOUT')
  })

  it('reports cancellation separately from a timeout when the caller aborts', async () => {
    vi.stubGlobal(
      'fetch',
      abortableFetch(() => new Promise<Response>(() => {})),
    )
    const controller = new AbortController()
    const pending = rawRequest('/ping', { signal: controller.signal }).catch((e: unknown) => e)
    controller.abort()
    const err = await pending
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).code).toBe('REQUEST_CANCELLED')
  })

  it('wraps an offline/DNS/CORS failure as a NETWORK_ERROR ApiError', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.reject(new TypeError('Failed to fetch'))),
    )
    const err = await rawRequest('/ping').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).code).toBe('NETWORK_ERROR')
  })
})
