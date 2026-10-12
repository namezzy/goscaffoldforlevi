import type { ApiErrorBody } from './types'

// One class for every non-2xx response. The backend's own convention is that
// every failure uses the same envelope (ADR-0004), so there is exactly one
// shape to unwrap here.
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId?: string
  readonly details?: { field: string; reason: string }[]
  readonly retryAfterSeconds?: number

  constructor(status: number, body: ApiErrorBody, retryAfterSeconds?: number) {
    super(body.error.message)
    this.status = status
    this.code = body.error.code
    this.requestId = body.request_id
    this.details = body.error.details
    this.retryAfterSeconds = retryAfterSeconds
  }
}
