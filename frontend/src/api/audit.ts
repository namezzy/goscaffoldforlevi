import { authedRequest } from './client'
import type { AuditEvent, AuditEventPage, ListAuditEventsParams } from './types'

export function listAuditEvents(params: ListAuditEventsParams): Promise<AuditEventPage> {
  const query = new URLSearchParams()
  if (params.from) query.set('from', params.from)
  if (params.to) query.set('to', params.to)
  if (params.actor_id) query.set('actor_id', params.actor_id)
  if (params.action) query.set('action', params.action)
  if (params.result) query.set('result', params.result)
  if (params.target_type) query.set('target_type', params.target_type)
  if (params.target_id) query.set('target_id', params.target_id)
  query.set('page', String(params.page ?? 1))
  query.set('page_size', String(params.page_size ?? 20))
  return authedRequest<AuditEventPage>(`/api/v1/admin/audit?${query.toString()}`)
}

export function getAuditEvent(id: string): Promise<AuditEvent> {
  return authedRequest<AuditEvent>(`/api/v1/admin/audit/${id}`)
}
