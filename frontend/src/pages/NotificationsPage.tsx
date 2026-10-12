import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { listNotifications, markAllNotificationsRead, markNotificationRead } from '../api/notifications'
import { Button, Card, EmptyState, ErrorState, Loading, PageHeader, Pagination } from '../components/ui'
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
      <PageHeader
        eyebrow="Notifications"
        title={
          <>
            通知{' '}
            {total ? <span className="font-mono text-sm font-normal tabular-nums text-slate-400">（共 {total} 条）</span> : null}
          </>
        }
        actions={
          <Button onClick={handleMarkAllRead} disabled={markingAll}>
            全部标为已读
          </Button>
        }
      />

      <label className="inline-flex cursor-pointer items-center gap-2 text-sm text-slate-300">
        <input
          type="checkbox"
          checked={unreadOnly}
          onChange={(e) => setUnreadOnly(e.target.checked)}
          className="h-4 w-4 accent-cyan-400"
        />
        只看未读
      </label>

      {loadError ? <ErrorState error={loadError} onRetry={reload} /> : null}

      {loading ? (
        <Loading />
      ) : items.length === 0 ? (
        <EmptyState>{unreadOnly ? '没有未读通知' : '还没有通知'}</EmptyState>
      ) : (
        <Card padded={false} className="overflow-hidden">
          <ul className="divide-y divide-white/[0.04]">
            {items.map((n) => (
              <li key={n.id}>
                <button
                  onClick={() => handleOpen(n)}
                  className={`flex w-full items-start justify-between gap-4 px-4 py-3 text-left transition hover:bg-cyan-400/[0.04] ${
                    !n.read_at ? 'bg-cyan-400/[0.03]' : ''
                  }`}
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      {!n.read_at && (
                        <span
                          className="h-1.5 w-1.5 shrink-0 rounded-full bg-cyan-400 shadow-[0_0_6px_rgba(34,211,238,0.8)]"
                          aria-hidden
                        />
                      )}
                      <span
                        className={`truncate text-sm font-medium ${n.read_at ? 'text-slate-300' : 'text-slate-50'}`}
                      >
                        {n.title}
                      </span>
                    </div>
                    <p className="mt-0.5 truncate text-sm text-slate-400">{n.body}</p>
                  </div>
                  <span className="shrink-0 whitespace-nowrap font-mono text-xs tabular-nums text-slate-500">
                    {new Date(n.created_at).toLocaleString('zh-CN')}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </Card>
      )}

      <Pagination page={page} totalPages={totalPages} onChange={setPage} />
    </div>
  )
}
