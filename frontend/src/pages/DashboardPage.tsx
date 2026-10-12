import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getDashboard } from '../api/workbench'
import {
  ButtonLink,
  Card,
  EmptyState,
  ErrorState,
  Loading,
  PageHeader,
  StatCard,
  formatDateTime,
  linkClass,
} from '../components/ui'
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

  if (loading) return <Loading />
  if (loadError) return <ErrorState error={loadError} onRetry={() => setReloadTick((t) => t + 1)} />
  if (!overview) return null

  const canSeeAllTickets = can(me?.role, 'ticket:read_all')

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Workbench"
        title="工作台"
        description="你的待办、申请与通知一览"
        actions={
          <>
            <ButtonLink to="/tickets" variant="primary">
              提交工单
            </ButtonLink>
            <ButtonLink to="/notifications">查看通知</ButtonLink>
            <ButtonLink to="/profile">我的资料</ButtonLink>
          </>
        }
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="我的申请" value={overview.my_open_tickets} hint="仍在处理中的申请" to="/tickets?scope=mine" />
        <StatCard
          label="我的待办"
          value={overview.assigned_to_me}
          hint="分配给我且未完结的工单"
          to="/tickets?scope=assigned"
          tone="violet"
        />
        <StatCard
          label="未读通知"
          value={overview.unread_notifications}
          hint="工单分配、补充信息、完成等提醒"
          to="/notifications?unread_only=true"
          tone="amber"
        />
      </div>

      {overview.admin && (
        <div className="grid gap-4 lg:grid-cols-2">
          {overview.admin.tickets_by_status && (
            <Card
              title="工单概况"
              className="lg:col-span-2"
              actions={
                canSeeAllTickets && (
                  <Link to="/tickets?scope=all" className={`text-xs ${linkClass}`}>
                    查看全部 →
                  </Link>
                )
              }
            >
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
                {STATUS_ORDER.map((status) => {
                  const count = overview.admin?.tickets_by_status?.[status] ?? 0
                  return (
                    <Link
                      key={status}
                      to={`/tickets?scope=all&status=${status}`}
                      className="rounded-lg border border-white/[0.06] bg-slate-950/40 px-3 py-2.5 transition hover:border-cyan-400/30"
                    >
                      <div className="font-mono text-xl font-semibold tabular-nums text-slate-50">{count}</div>
                      <div className="mt-0.5 text-xs text-slate-400">{STATUS_LABELS[status]}</div>
                    </Link>
                  )
                })}
              </div>
            </Card>
          )}

          {overview.admin.outbox && (
            <Card title="系统异常摘要">
              <Link to="/status" className="flex items-center justify-between gap-3 text-sm">
                <span className="font-mono text-slate-200">
                  待投递 <span className="text-cyan-300">{overview.admin.outbox.pending}</span> · 失败{' '}
                  <span className={overview.admin.outbox.failing ? 'text-rose-300' : 'text-emerald-300'}>
                    {overview.admin.outbox.failing}
                  </span>
                </span>
                <span className={`text-xs ${linkClass}`}>前往监控台 →</span>
              </Link>
            </Card>
          )}

          {overview.admin.recent_audit_events && (
            <Card
              title="最近管理操作"
              padded={false}
              actions={
                <Link to="/audit" className={`text-xs ${linkClass}`}>
                  查看完整审计日志 →
                </Link>
              }
            >
              {overview.admin.recent_audit_events.length === 0 ? (
                <EmptyState>暂无记录。</EmptyState>
              ) : (
                <ul className="divide-y divide-white/[0.04]">
                  {overview.admin.recent_audit_events.map((e) => (
                    <li key={e.id} className="flex items-center justify-between gap-4 px-5 py-2.5 text-sm">
                      <span className="truncate font-mono text-xs text-slate-200">{e.action}</span>
                      <span className="shrink-0 whitespace-nowrap font-mono text-xs text-slate-500">
                        {formatDateTime(e.created_at)}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </Card>
          )}
        </div>
      )}
    </div>
  )
}
