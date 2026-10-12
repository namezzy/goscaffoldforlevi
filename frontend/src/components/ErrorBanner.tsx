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
    <div className="rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-950 dark:text-red-200">
      {describeError(error)}
    </div>
  )
}
