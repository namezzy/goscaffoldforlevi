import { authedRequest } from './client'
import type {
  AssignTicketRequest,
  AuditEvent,
  CloseTicketRequest,
  CompleteTicketRequest,
  CreateTicketRequest,
  ListTicketsParams,
  Ticket,
  TicketNoteRequest,
  TicketPage,
  TicketStatusOverview,
} from './types'

export function createTicket(input: CreateTicketRequest): Promise<Ticket> {
  return authedRequest<Ticket>('/api/v1/tickets', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function listTickets(params: ListTicketsParams): Promise<TicketPage> {
  const query = new URLSearchParams()
  query.set('scope', params.scope ?? 'mine')
  query.set('page', String(params.page ?? 1))
  query.set('page_size', String(params.page_size ?? 20))
  if (params.status) query.set('status', params.status)
  if (params.priority) query.set('priority', params.priority)
  if (params.search) query.set('search', params.search)
  if (params.sort) query.set('sort', params.sort)
  if (params.sort_dir) query.set('sort_dir', params.sort_dir)
  return authedRequest<TicketPage>(`/api/v1/tickets?${query.toString()}`)
}

export function getTicketStatusOverview(): Promise<TicketStatusOverview> {
  return authedRequest<TicketStatusOverview>('/api/v1/tickets/overview')
}

export function getTicket(id: string): Promise<Ticket> {
  return authedRequest<Ticket>(`/api/v1/tickets/${id}`)
}

export function getTicketActivity(id: string): Promise<AuditEvent[]> {
  return authedRequest<AuditEvent[]>(`/api/v1/tickets/${id}/activity`)
}

export function assignTicket(id: string, input: AssignTicketRequest): Promise<Ticket> {
  return authedRequest<Ticket>(`/api/v1/tickets/${id}/assign`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function reassignTicket(id: string, input: AssignTicketRequest): Promise<Ticket> {
  return authedRequest<Ticket>(`/api/v1/tickets/${id}/reassign`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function addTicketNote(id: string, input: TicketNoteRequest): Promise<Ticket> {
  return authedRequest<Ticket>(`/api/v1/tickets/${id}/notes`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function requestTicketInfo(id: string, input: TicketNoteRequest): Promise<Ticket> {
  return authedRequest<Ticket>(`/api/v1/tickets/${id}/request-info`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function provideTicketInfo(id: string, input: TicketNoteRequest): Promise<Ticket> {
  return authedRequest<Ticket>(`/api/v1/tickets/${id}/provide-info`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function completeTicket(id: string, input: CompleteTicketRequest): Promise<Ticket> {
  return authedRequest<Ticket>(`/api/v1/tickets/${id}/complete`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function closeTicket(id: string, input: CloseTicketRequest): Promise<Ticket> {
  return authedRequest<Ticket>(`/api/v1/tickets/${id}/close`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
