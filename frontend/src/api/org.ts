import { authedRequest } from './client'
import type {
  CreateDepartmentRequest,
  Department,
  DepartmentPage,
  NotificationChannel,
  Organization,
  UpdateDepartmentRequest,
  UpdateOrganizationRequest,
} from './types'

export function getOrganization(): Promise<Organization> {
  return authedRequest<Organization>('/api/v1/org')
}

export function updateOrganization(input: UpdateOrganizationRequest): Promise<Organization> {
  return authedRequest<Organization>('/api/v1/org', {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function getNotificationChannels(): Promise<NotificationChannel[]> {
  return authedRequest<NotificationChannel[]>('/api/v1/org/notification-channels')
}

export function listDepartments(): Promise<DepartmentPage> {
  return authedRequest<DepartmentPage>('/api/v1/org/departments')
}

export function createDepartment(input: CreateDepartmentRequest): Promise<Department> {
  return authedRequest<Department>('/api/v1/org/departments', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateDepartment(id: string, input: UpdateDepartmentRequest): Promise<Department> {
  return authedRequest<Department>(`/api/v1/org/departments/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteDepartment(id: string): Promise<void> {
  return authedRequest<void>(`/api/v1/org/departments/${id}`, { method: 'DELETE' })
}
