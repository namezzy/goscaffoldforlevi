import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getDashboard } from '../api/workbench'
import { ErrorBanner } from '../components/ErrorBanner'
import { can } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import { STATUS_LABELS } from './ticketShared'
import type { DashboardOverview, TicketStatus } from '../api/types'

// The status order a status breakdown reads most naturally in, following
// the ticket lifecycle rather than however Object.keys happens to return
// them.
const STATUS_ORDER: TicketStatus[] = ['pending', 'in_progress', 'needs_info', 'completed', 'closed']

export function DashboardPage() {
  const { user: me } = useAuth()
  const [overview, setOverview] = useState<DashboardOverview | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [reloadTick, setReloadTick] = useState(0)

  useEffect(() => {
    setLoading(true)
    setLoadError(null)
    getDashboard()
      .then(setOverview)
      .catch(setLoadError)
      .finally(() => setLoading(false))
  }, [reloadTick])

  if (loading) return <p className="text-neutral-400">加载中…</p>
  if (loadError) {
    return (
      <div className="space-y-3">
        <ErrorBanner error={loadError} />
        <button
          onClick={() => setReloadTick((t) => t + 1)}
          className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
        >
          重试
        </button>
      </div>
    )
  }
  if (!overview) return null

  const canSeeAllTickets = can(me?.role, 'ticket:read_all')

  return (
    <div className="space-y-6">
      <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">工作台</h1>

      <div className="grid gap-4 sm:grid-cols-3">
        <MetricCard
          label="我的申请"
          value={overview.my_open_tickets}
          hint="仍在处理中的申请"
          to="/tickets?scope=mine"
        />
        <MetricCard
          label="我的待办"
          value={overview.assigned_to_me}
          hint="分配给我且未完结的工单"
          to="/tickets?scope=assigned"
        />
        <MetricCard
          label="未读通知"
          value={overview.unread_notifications}
          hint="工单分配、补充信息、完成等提醒"
          to="/notifications?unread_only=true"
        />
      </div>

      <div className="flex flex-wrap gap-2">
        <Link
          to="/tickets"
          className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
        >
          提交工单
        </Link>
        <Link
          to="/notifications"
          className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
        >
          查看通知
        </Link>
        <Link
          to="/profile"
          className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
        >
          我的资料
        </Link>
      </div>

      {overview.admin && (
        <div className="space-y-4 border-t border-neutral-200 pt-6 dark:border-neutral-800">
          <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">管理概览</h2>

          {overview.admin.tickets_by_status && (
            <section className="space-y-2">
              <h3 className="text-xs font-medium text-neutral-500 dark:text-neutral-400">工单概况</h3>
              <div className="flex flex-wrap gap-2">
                {STATUS_ORDER.map((status) => {
                  const count = overview.admin?.tickets_by_status?.[status] ?? 0
                  return (
                    <Link
                      key={status}
                      to={`/tickets?scope=all&status=${status}`}
                      className="min-w-[7rem] flex-1 rounded-lg border border-neutral-200 px-3 py-2 text-sm hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-900"
                    >
                      <div className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">{count}</div>
                      <div className="text-neutral-500 dark:text-neutral-400">{STATUS_LABELS[status]}</div>
                    </Link>
                  )
                })}
                {canSeeAllTickets && (
                  <Link
                    to="/tickets?scope=all"
                    className="flex min-w-[7rem] flex-1 items-center justify-center rounded-lg border border-dashed border-neutral-300 px-3 py-2 text-sm text-neutral-500 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-400 dark:hover:bg-neutral-900"
                  >
                    查看全部 →
                  </Link>
                )}
              </div>
            </section>
          )}

          {overview.admin.outbox && (
            <section className="space-y-2">
              <h3 className="text-xs font-medium text-neutral-500 dark:text-neutral-400">系统异常摘要</h3>
              <Link
                to="/status"
                className="block rounded-lg border border-neutral-200 px-3 py-2 text-sm hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-900"
              >
                <span className="text-neutral-900 dark:text-neutral-100">
                  待投递 {overview.admin.outbox.pending} · 失败 {overview.admin.outbox.failing}
                </span>
                <span className="ml-2 text-neutral-500 dark:text-neutral-400">前往监控台 →</span>
              </Link>
            </section>
          )}

          {overview.admin.recent_audit_events && (
            <section className="space-y-2">
              <h3 className="text-xs font-medium text-neutral-500 dark:text-neutral-400">最近管理操作</h3>
              {overview.admin.recent_audit_events.length === 0 ? (
                <p className="text-sm text-neutral-400">暂无记录。</p>
              ) : (
                <ul className="divide-y divide-neutral-100 overflow-hidden rounded-lg border border-neutral-200 dark:divide-neutral-800 dark:border-neutral-800">
                  {overview.admin.recent_audit_events.map((e) => (
                    <li key={e.id} className="flex items-center justify-between gap-4 px-3 py-2 text-sm">
                      <span className="truncate text-neutral-900 dark:text-neutral-100">{e.action}</span>
                      <span className="shrink-0 whitespace-nowrap text-xs text-neutral-500 dark:text-neutral-400">
                        {new Date(e.created_at).toLocaleString('zh-CN')}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
              <Link to="/audit" className="text-xs text-neutral-500 hover:underline dark:text-neutral-400">
                查看完整审计日志 →
              </Link>
            </section>
          )}
        </div>
      )}
    </div>
  )
}

function MetricCard({ label, value, hint, to }: { label: string; value: number; hint: string; to: string }) {
  return (
    <Link
      to={to}
      className="block rounded-lg border border-neutral-200 p-4 hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-900"
    >
      <div className="text-2xl font-semibold text-neutral-900 dark:text-neutral-100">{value}</div>
      <div className="mt-1 text-sm text-neutral-600 dark:text-neutral-300">{label}</div>
      <div className="mt-0.5 text-xs text-neutral-400">{hint}</div>
    </Link>
  )
}
