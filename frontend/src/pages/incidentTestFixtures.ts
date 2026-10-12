import type { AuditEvent, Incident } from '../api/types'

export const OPERATOR_ID = '01930f8e-0000-7000-8000-000000000001'

export function makeIncident(overrides: Partial<Incident> = {}): Incident {
  return {
    id: '01930f8e-0000-7000-8000-0000000000aa',
    fingerprint: 'welcome-email:user.created.v1',
    severity: 'warning',
    status: 'open',
    dead_letter_id: '01930f8e-0000-7000-8000-0000000000dd',
    event_type: 'user.created.v1',
    consumer: 'welcome-email',
    last_error: 'smtp refused',
    occurrence_count: 3,
    first_seen_at: '2026-10-01T08:00:00Z',
    last_seen_at: '2026-10-01T09:00:00Z',
    created_at: '2026-10-01T08:00:00Z',
    updated_at: '2026-10-01T09:00:00Z',
    ...overrides,
  }
}

export function makeEvent(overrides: Partial<AuditEvent> = {}): AuditEvent {
  return {
    id: '01930f8e-0000-7000-8000-0000000000e1',
    action: 'incident.note_added',
    target_type: 'incident',
    target_id: makeIncident().id,
    result: 'success',
    created_at: '2026-10-01T09:30:00Z',
    ...overrides,
  } as AuditEvent
}
