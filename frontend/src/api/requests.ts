import { authedRequest, authedStream } from './client'
import type { RequestEvent, RequestEventPage } from './types'

export function getRecentRequests(): Promise<RequestEventPage> {
  return authedRequest<RequestEventPage>('/api/v1/admin/requests')
}

/**
 * Opens the live request feed and calls onEvent for each one, until the
 * server ends the connection (it does, after the usual request timeout —
 * see api/openapi.yaml) or the caller aborts via signal. Never throws for a
 * server-side end of stream; the caller decides whether to reconnect.
 */
export async function streamRequests(
  onEvent: (e: RequestEvent) => void,
  signal: AbortSignal,
): Promise<void> {
  const res = await authedStream('/api/v1/admin/requests', signal)
  const reader = res.body?.getReader()
  if (!reader) return

  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) return
    buffer += decoder.decode(value, { stream: true })

    // SSE messages are separated by a blank line; a message may itself be
    // split across chunks, so only fully-terminated messages are consumed.
    let boundary: number
    while ((boundary = buffer.indexOf('\n\n')) !== -1) {
      const raw = buffer.slice(0, boundary)
      buffer = buffer.slice(boundary + 2)

      for (const line of raw.split('\n')) {
        if (!line.startsWith('data:')) continue // skip comments/heartbeats
        const payload = line.slice(5).trim()
        try {
          onEvent(JSON.parse(payload) as RequestEvent)
        } catch {
          // A malformed frame is dropped rather than ending the stream over
          // one event a live tail view never needed to begin with.
        }
      }
    }
  }
}
