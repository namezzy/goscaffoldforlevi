import { authedRequest } from './client'
import type { AuditEvent, Incident, IncidentPage, IncidentSeverity, IncidentSummary, ListIncidentsParams } from './types'

export function listIncidents(params: ListIncidentsParams): Promise<IncidentPage> {
  const query = new URLSearchParams()
  query.set('page', String(params.page ?? 1))
  query.set('page_size', String(params.page_size ?? 20))
  if (params.status) query.set('status', params.status)
  if (params.severity) query.set('severity', params.severity)
  if (params.owner_id) query.set('owner_id', params.owner_id)
  if (params.search) query.set('search', params.search)
  return authedRequest<IncidentPage>(`/api/v1/incidents?${query.toString()}`)
}

export function getIncidentSummary(): Promise<IncidentSummary> {
  return authedRequest<IncidentSummary>('/api/v1/incidents/summary')
}

export function getIncident(id: string): Promise<Incident> {
  return authedRequest<Incident>(`/api/v1/incidents/${id}`)
}

export function getIncidentActivity(id: string): Promise<AuditEvent[]> {
  return authedRequest<AuditEvent[]>(`/api/v1/incidents/${id}/activity`)
}

function post(id: string, action: string, body?: unknown): Promise<Incident> {
  return authedRequest<Incident>(`/api/v1/incidents/${id}/${action}`, {
    method: 'POST',
    body: body === undefined ? undefined : JSON.stringify(body),
  })
}

export function setIncidentSeverity(id: string, severity: IncidentSeverity): Promise<Incident> {
  return post(id, 'severity', { severity })
}

/** An empty ownerId clears the assignment. */
export function assignIncidentOwner(id: string, ownerId: string): Promise<Incident> {
  return post(id, 'owner', { owner_id: ownerId })
}

export function acknowledgeIncident(id: string): Promise<Incident> {
  return post(id, 'acknowledge')
}

export function addIncidentNote(id: string, note: string): Promise<Incident> {
  return post(id, 'notes', { note })
}

export function resolveIncident(id: string): Promise<Incident> {
  return post(id, 'resolve')
}

/** Answers 202 with replay_status "pending" — submission, not success. */
export function requestIncidentReplay(id: string): Promise<Incident> {
  return post(id, 'replay')
}
