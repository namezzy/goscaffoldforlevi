import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { listNotifications, markAllNotificationsRead, markNotificationRead } from '../api/notifications'
import { ErrorBanner } from '../components/ErrorBanner'
import type { Notification } from '../api/types'

const PAGE_SIZE = 20

/** target_type/target_id name a ticket, a user or an incident; the
 * destination page re-validates access on its
 * own regardless of what this link says (see App.tsx's route guards),
 * which is what lets this stay a plain lookup table instead of needing its
 * own permission logic. */
function targetHref(n: Notification): string {
  if (n.target_type === 'ticket') return `/tickets/${n.target_id}`
  if (n.target_type === 'user') return `/members/${n.target_id}`
  if (n.target_type === 'incident') return `/incidents/${n.target_id}`
  return '/dashboard'
}

export function NotificationsPage() {
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const unreadOnly = params.get('unread_only') === 'true'
  const page = Number(params.get('page') ?? '1') || 1

  const [items, setItems] = useState<Notification[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [reloadTick, setReloadTick] = useState(0)
  const [markingAll, setMarkingAll] = useState(false)

  useEffect(() => {
    setLoading(true)
    setLoadError(null)
    listNotifications({ unread_only: unreadOnly, page, page_size: PAGE_SIZE })
      .then((data) => {
        setItems(data.items)
        setTotalPages(Math.max(data.page_info.total_pages, 1))
        setTotal(data.page_info.total)
      })
      .catch(setLoadError)
      .finally(() => setLoading(false))
  }, [unreadOnly, page, reloadTick])

  function reload() {
    setReloadTick((t) => t + 1)
  }

  function setUnreadOnly(value: boolean) {
    const next = new URLSearchParams(params)
    if (value) next.set('unread_only', 'true')
    else next.delete('unread_only')
    next.delete('page')
    setParams(next, { replace: true })
  }

  function setPage(p: number) {
    const next = new URLSearchParams(params)
    next.set('page', String(p))
    setParams(next, { replace: true })
  }

  async function handleOpen(n: Notification) {
    if (!n.read_at) {
      markNotificationRead(n.id).catch(() => {})
    }
    navigate(targetHref(n))
  }

  async function handleMarkAllRead() {
    setMarkingAll(true)
    try {
      await markAllNotificationsRead()
      reload()
    } finally {
      setMarkingAll(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">
          通知 {total ? <span className="text-sm font-normal text-neutral-500">（共 {total} 条）</span> : null}
        </h1>
        <button
          onClick={handleMarkAllRead}
          disabled={markingAll}
          className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-50 disabled:opacity-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
        >
          全部标为已读
        </button>
      </div>

      <label className="flex items-center gap-2 text-sm text-neutral-600 dark:text-neutral-300">
        <input type="checkbox" checked={unreadOnly} onChange={(e) => setUnreadOnly(e.target.checked)} />
        只看未读
      </label>

      {loadError ? (
        <div className="space-y-2">
          <ErrorBanner error={loadError} />
          <button
            onClick={reload}
            className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
          >
            重试
          </button>
        </div>
      ) : null}

      {loading ? (
        <p className="text-neutral-400">加载中…</p>
      ) : items.length === 0 ? (
        <p className="text-sm text-neutral-400">{unreadOnly ? '没有未读通知' : '还没有通知'}</p>
      ) : (
        <ul className="divide-y divide-neutral-100 overflow-hidden rounded-lg border border-neutral-200 dark:divide-neutral-800 dark:border-neutral-800">
          {items.map((n) => (
            <li key={n.id}>
              <button
                onClick={() => handleOpen(n)}
                className={`flex w-full items-start justify-between gap-4 px-4 py-3 text-left hover:bg-neutral-50 dark:hover:bg-neutral-900 ${
                  !n.read_at ? 'bg-blue-50/50 dark:bg-blue-950/20' : ''
                }`}
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    {!n.read_at && <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-blue-600" aria-hidden />}
                    <span className="truncate text-sm font-medium text-neutral-900 dark:text-neutral-100">{n.title}</span>
                  </div>
                  <p className="mt-0.5 truncate text-sm text-neutral-500 dark:text-neutral-400">{n.body}</p>
                </div>
                <span className="shrink-0 whitespace-nowrap text-xs text-neutral-400">
                  {new Date(n.created_at).toLocaleString('zh-CN')}
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}

      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-3 text-sm">
          <button
            onClick={() => setPage(Math.max(page - 1, 1))}
            disabled={page <= 1}
            className="rounded-md border border-neutral-300 px-2 py-1 disabled:opacity-40 dark:border-neutral-700"
          >
            上一页
          </button>
          <span className="text-neutral-500">
            第 {page} / {totalPages} 页
          </span>
          <button
            onClick={() => setPage(Math.min(page + 1, totalPages))}
            disabled={page >= totalPages}
            className="rounded-md border border-neutral-300 px-2 py-1 disabled:opacity-40 dark:border-neutral-700"
          >
            下一页
          </button>
        </div>
      )}
    </div>
  )
}
