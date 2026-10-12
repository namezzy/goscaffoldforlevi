import { ApiError } from '../api/errors'

export function describeError(err: unknown): string {
  if (err instanceof ApiError) {
    const retry = err.retryAfterSeconds ? `，${err.retryAfterSeconds} 秒后可重试` : ''
    const fields = err.details?.length
      ? '：' + err.details.map((d) => `${d.field} ${d.reason}`).join('，')
      : ''
    return `${err.message}${fields}${retry}`
  }
  if (err instanceof Error) return err.message
  return '发生未知错误'
}

export function ErrorBanner({ error }: { error: unknown }) {
  if (!error) return null
  return (
    <div role="alert"
      className="flex items-start gap-2 rounded-lg border border-rose-400/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-200">
      <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-rose-400 shadow-[0_0_6px_rgba(251,113,133,0.9)]" />
      <span>{describeError(error)}</span>
    </div>
  )
}
