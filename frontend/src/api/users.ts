import { authedRequest } from './client'
import type {
  ChangePasswordRequest,
  CreateUserRequest,
  ListUsersParams,
  UpdateUserRequest,
  User,
  UserPage,
} from './types'

export function listUsers(params: ListUsersParams): Promise<UserPage> {
  const query = new URLSearchParams()
  query.set('page', String(params.page ?? 1))
  query.set('page_size', String(params.page_size ?? 20))
  if (params.search) query.set('search', params.search)
  if (params.role) query.set('role', params.role)
  if (params.status) query.set('status', params.status)
  if (params.sort) query.set('sort', params.sort)
  if (params.sort_dir) query.set('sort_dir', params.sort_dir)
  return authedRequest<UserPage>(`/api/v1/users?${query.toString()}`)
}

export function getUser(id: string): Promise<User> {
  return authedRequest<User>(`/api/v1/users/${id}`)
}

export function createUser(input: CreateUserRequest): Promise<User> {
  return authedRequest<User>('/api/v1/users', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateUser(id: string, input: UpdateUserRequest): Promise<User> {
  return authedRequest<User>(`/api/v1/users/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteUser(id: string): Promise<void> {
  return authedRequest<void>(`/api/v1/users/${id}`, { method: 'DELETE' })
}

export function changePassword(id: string, input: ChangePasswordRequest): Promise<void> {
  return authedRequest<void>(`/api/v1/users/${id}/password`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}
