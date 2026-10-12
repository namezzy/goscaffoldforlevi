import { authedRequest } from './client'
import type { ListNotificationsParams, NotificationPage } from './types'

export function listNotifications(params: ListNotificationsParams): Promise<NotificationPage> {
  const query = new URLSearchParams()
  query.set('page', String(params.page ?? 1))
  query.set('page_size', String(params.page_size ?? 20))
  if (params.unread_only) query.set('unread_only', 'true')
  return authedRequest<NotificationPage>(`/api/v1/notifications?${query.toString()}`)
}

export function getUnreadNotificationCount(): Promise<{ count: number }> {
  return authedRequest<{ count: number }>('/api/v1/notifications/unread-count')
}

export function markNotificationRead(id: string): Promise<void> {
  return authedRequest<void>(`/api/v1/notifications/${id}/read`, { method: 'POST' })
}

export function markAllNotificationsRead(): Promise<void> {
  return authedRequest<void>('/api/v1/notifications/read-all', { method: 'POST' })
}
