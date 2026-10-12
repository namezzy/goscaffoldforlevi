import { authedRequest } from './client'
import type { DashboardOverview } from './types'

export function getDashboard(): Promise<DashboardOverview> {
  return authedRequest<DashboardOverview>('/api/v1/dashboard')
}
