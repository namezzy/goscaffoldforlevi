import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getUser, listUsers } from '../api/users'
import {
  addTicketNote,
  assignTicket,
  closeTicket,
  completeTicket,
  getTicket,
  getTicketActivity,
  provideTicketInfo,
  reassignTicket,
  requestTicketInfo,
} from '../api/tickets'
import { ApiError } from '../api/errors'
import { ErrorBanner } from '../components/ErrorBanner'
import { can } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import { Button, Card, EmptyState, ErrorState, Field, Loading, PageHeader, inputClass, linkClass } from '../components/ui'
import { PriorityBadge, StatusBadge } from './ticketShared'
import type { AuditEvent, Ticket, User } from '../api/types'

type ActionKind = 'assign' | 'reassign' | 'note' | 'request-info' | 'provide-info' | 'complete' | 'close'

export function TicketDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { user: me } = useAuth()

  const [ticket, setTicket] = useState<Ticket | null>(null)
  const [activity, setActivity] = useState<AuditEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [activeAction, setActiveAction] = useState<ActionKind | null>(null)

  const load = useCallback(() => {
    if (!id) return
    setLoading(true)
    setLoadError(null)
    Promise.all([getTicket(id), getTicketActivity(id)])
      .then(([t, records]) => {
        setTicket(t)
        setActivity(records)
      })
      .catch(setLoadError)
      .finally(() => setLoading(false))
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  if (loading) return <Loading />
  if (loadError) return <ErrorState error={loadError} onRetry={load} />
  if (!ticket || !id) return null

  const isRequester = me?.id === ticket.requester_id
  const isAssignee = !!ticket.assignee_id && me?.id === ticket.assignee_id
  const canManage = can(me?.role, 'ticket:manage')

  function onDone(updated: Ticket) {
    setTicket(updated)
    setActiveAction(null)
    // The action's own response already carries the new state; the
    // timeline is refetched separately since the action does not return it.
    if (id) getTicketActivity(id).then(setActivity).catch(() => {})
  }

  // What the ticket's own state machine allows right now, for who this
  // caller is — mirrors internal/ticket's Service methods' Authorize
  // closures exactly, so a button never appears only to be refused by the
  // backend a moment later (see each Service method's own doc comment for
  // the rule this reproduces).
  const availableActions: { kind: ActionKind; label: string }[] = []
  if (ticket.status === 'pending' && canManage) {
    availableActions.push({ kind: 'assign', label: '分派' })
  }
  if ((ticket.status === 'in_progress' || ticket.status === 'needs_info') && canManage) {
    availableActions.push({ kind: 'reassign', label: '重新分派' })
  }
  if ((ticket.status === 'in_progress' || ticket.status === 'needs_info') && (isAssignee || canManage)) {
    availableActions.push({ kind: 'note', label: '添加处理备注' })
  }
  if (ticket.status === 'in_progress' && (isAssignee || canManage)) {
    availableActions.push({ kind: 'request-info', label: '要求补充信息' })
  }
  if (ticket.status === 'needs_info' && isRequester) {
    availableActions.push({ kind: 'provide-info', label: '补充信息' })
  }
  if (ticket.status === 'in_progress' && (isAssignee || canManage)) {
    availableActions.push({ kind: 'complete', label: '标记完成' })
  }
  if ((ticket.status === 'completed' && (isRequester || canManage)) || (canManage && ticket.status !== 'closed' && ticket.status !== 'completed')) {
    availableActions.push({ kind: 'close', label: '关闭' })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm">
        <Link to="/tickets" className={linkClass}>
          工单
        </Link>
        <span className="text-slate-600">/</span>
        <span className="truncate text-slate-300">{ticket.title}</span>
      </div>

      <PageHeader
        eyebrow="Ticket"
        title={ticket.title}
        description={
          <span className="flex flex-wrap items-center gap-2">
            <StatusBadge status={ticket.status} />
            <PriorityBadge priority={ticket.priority} />
            <span className="text-xs text-slate-400">{ticket.category}</span>
          </span>
        }
      />

      <Card>
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-slate-300">{ticket.description}</p>

        <dl className="mt-5 grid gap-x-8 border-t border-white/[0.06] pt-3 text-sm sm:grid-cols-2">
          <RequesterRow label="申请人" userId={ticket.requester_id} />
          <RequesterRow label="负责人" userId={ticket.assignee_id} placeholder="未分配" />
          <Row label="创建时间" value={new Date(ticket.created_at).toLocaleString('zh-CN')} mono />
          <Row label="更新时间" value={new Date(ticket.updated_at).toLocaleString('zh-CN')} mono />
          {ticket.closed_at && <Row label="关闭时间" value={new Date(ticket.closed_at).toLocaleString('zh-CN')} mono />}
        </dl>

        {availableActions.length > 0 && (
          <div className="mt-5 border-t border-white/[0.06] pt-4">
            <p className="mb-2 font-mono text-[11px] uppercase tracking-wider text-slate-500">下一步操作</p>
            <div className="flex flex-wrap gap-2">
              {availableActions.map((a) => (
                <Button key={a.kind} variant={a.kind === 'close' ? 'danger' : 'primary'} onClick={() => setActiveAction(a.kind)}>
                  {a.label}
                </Button>
              ))}
            </div>
          </div>
        )}

        {activeAction && (
          <div className="mt-4 border-t border-white/[0.06] pt-4">
            <ActionForm kind={activeAction} ticket={ticket} onCancel={() => setActiveAction(null)} onDone={onDone} />
          </div>
        )}
      </Card>

      <Card title="处理记录">
        {activity.length === 0 ? (
          <EmptyState>暂无记录。</EmptyState>
        ) : (
          <ol className="relative space-y-3 border-l border-cyan-400/20 pl-4">
            {activity.map((e) => (
              <li key={e.id} className="relative text-sm">
                <span className="absolute -left-[21px] top-1.5 h-2 w-2 rounded-full bg-cyan-400 shadow-[0_0_6px_rgba(34,211,238,0.8)]" />
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium text-slate-100">{describeAction(e.action)}</span>
                  <span className="whitespace-nowrap font-mono text-xs tabular-nums text-slate-500">
                    {new Date(e.created_at).toLocaleString('zh-CN')}
                  </span>
                </div>
                {noteFromEvent(e) && <p className="mt-1 whitespace-pre-wrap text-slate-400">{noteFromEvent(e)}</p>}
              </li>
            ))}
          </ol>
        )}
      </Card>
    </div>
  )
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex justify-between gap-4 border-b border-white/[0.04] py-2">
      <dt className="shrink-0 text-slate-500">{label}</dt>
      <dd className={`min-w-0 break-all text-right text-slate-200 ${mono ? 'font-mono text-xs tabular-nums' : ''}`}>{value}</dd>
    </div>
  )
}
/** Resolves a user id to an email, falling back to the raw id if the lookup
 * fails (no permission, or the id is absent) — a cosmetic best effort, not
 * this page's primary content, so it fails silently like Layout's own
 * header email lookup. */
function RequesterRow({ label, userId, placeholder }: { label: string; userId?: string; placeholder?: string }) {
  const [display, setDisplay] = useState<string | null>(null)

  useEffect(() => {
    setDisplay(null)
    if (!userId) return
    let cancelled = false
    getUser(userId)
      .then((u) => {
        if (!cancelled) setDisplay(u.email)
      })
      .catch(() => {
        if (!cancelled) setDisplay(userId)
      })
    return () => {
      cancelled = true
    }
  }, [userId])

  if (!userId) return <Row label={label} value={placeholder ?? '—'} />
  return <Row label={label} value={display ?? userId} />
}

function describeAction(action: string): string {
  const labels: Record<string, string> = {
    'ticket.created': '提交工单',
    'ticket.assigned': '分派工单',
    'ticket.reassigned': '重新分派',
    'ticket.note_added': '添加处理备注',
    'ticket.info_requested': '要求补充信息',
    'ticket.info_provided': '补充信息',
    'ticket.completed': '标记完成',
    'ticket.closed': '关闭工单',
  }
  return labels[action] ?? action
}

function noteFromEvent(e: AuditEvent): string | null {
  const after = e.after as Record<string, unknown> | undefined
  const note = after?.note
  return typeof note === 'string' ? note : null
}

function ActionForm({
  kind,
  ticket,
  onCancel,
  onDone,
}: {
  kind: ActionKind
  ticket: Ticket
  onCancel: () => void
  onDone: (t: Ticket) => void
}) {
  const [assigneeId, setAssigneeId] = useState('')
  const [users, setUsers] = useState<User[]>([])
  const [note, setNote] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const needsAssignee = kind === 'assign' || kind === 'reassign'
  const needsNote = kind === 'note' || kind === 'request-info' || kind === 'provide-info'
  const noteOptional = kind === 'complete'

  useEffect(() => {
    if (!needsAssignee) return
    listUsers({ page: 1, page_size: 100, sort: 'name' })
      .then((page) => setUsers(page.items))
      .catch(() => {})
  }, [needsAssignee])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const expected_version = ticket.version
      let updated: Ticket
      switch (kind) {
        case 'assign':
          updated = await assignTicket(ticket.id, { assignee_id: assigneeId, expected_version })
          break
        case 'reassign':
          updated = await reassignTicket(ticket.id, { assignee_id: assigneeId, expected_version })
          break
        case 'note':
          updated = await addTicketNote(ticket.id, { note, expected_version })
          break
        case 'request-info':
          updated = await requestTicketInfo(ticket.id, { note, expected_version })
          break
        case 'provide-info':
          updated = await provideTicketInfo(ticket.id, { note, expected_version })
          break
        case 'complete':
          updated = await completeTicket(ticket.id, { note: note || undefined, expected_version })
          break
        case 'close':
          updated = await closeTicket(ticket.id, { expected_version })
          break
      }
      onDone(updated)
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  const isConflict = error instanceof ApiError && error.code === 'TICKET_VERSION_CONFLICT'

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      {error ? (
        <div>
          <ErrorBanner error={error} />
          {isConflict && (
            <p className="mt-1 text-xs text-slate-400">
              该工单已被他人更新，请刷新页面后重试。
            </p>
          )}
        </div>
      ) : null}
      {needsAssignee && (
        <Field label="负责人">
          <select required value={assigneeId} onChange={(e) => setAssigneeId(e.target.value)} className={inputClass}>
            <option value="" disabled>
              选择成员
            </option>
            {users.map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}（{u.email}）
              </option>
            ))}
          </select>
        </Field>
      )}
      {(needsNote || noteOptional) && (
        <Field label={noteOptional ? '完成说明（可选）' : '说明'}>
          <textarea
            required={needsNote}
            minLength={needsNote ? 1 : undefined}
            maxLength={4000}
            rows={3}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            className={inputClass}
          />
        </Field>
      )}
      {kind === 'close' && !needsNote && (
        <p className="text-sm text-slate-300">确定要关闭这个工单吗？关闭后无法再次操作。</p>
      )}
      <div className="flex justify-end gap-2 pt-1">
        <Button type="button" onClick={onCancel}>
          取消
        </Button>
        <Button type="submit" variant={kind === 'close' ? 'danger' : 'primary'} disabled={busy}>
          {busy ? '提交中…' : '确认'}
        </Button>
      </div>
    </form>
  )
}
