import { authedRequest } from './client'
import type { DeadLetterPage, OutboxStatus } from './types'

export function getOutboxStatus(): Promise<OutboxStatus> {
  return authedRequest<OutboxStatus>('/api/v1/admin/outbox/status')
}

export function listDeadLetters(eventType?: string, limit = 50): Promise<DeadLetterPage> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (eventType) params.set('event_type', eventType)
  return authedRequest<DeadLetterPage>(`/api/v1/admin/dlq?${params.toString()}`)
}

export function replayDeadLetter(id: string): Promise<void> {
  return authedRequest<void>(`/api/v1/admin/dlq/${id}/replay`, { method: 'POST' })
}
