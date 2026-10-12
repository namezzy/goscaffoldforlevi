import { API_BASE_URL } from './config'
import { rawRequest } from './http'
import type { HealthStatus, ReadinessStatus } from './types'

export function getHealth(): Promise<HealthStatus> {
  return rawRequest<HealthStatus>('/health')
}

export function getLiveness(): Promise<HealthStatus> {
  return rawRequest<HealthStatus>('/live')
}

/**
 * /ready answers 503 while a dependency is down or the process is draining,
 * but the body is still a well-formed ReadinessStatus — not the Error
 * envelope every other non-2xx response uses. rawRequest assumes the Error
 * shape for anything that isn't res.ok, so this endpoint fetches directly and
 * treats both 200 and 503 as a valid, readable status.
 */
export async function getReadiness(): Promise<ReadinessStatus> {
  const res = await fetch(`${API_BASE_URL}/ready`)
  return (await res.json()) as ReadinessStatus
}
