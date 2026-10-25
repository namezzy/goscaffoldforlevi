import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  acknowledgeIncident,
  addIncidentNote,
  assignIncidentOwner,
  getIncident,
  getIncidentActivity,
  requestIncidentReplay,
  resolveIncident,
  setIncidentSeverity,
} from '../api/incidents'
import { getUser, listUsers } from '../api/users'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { ErrorBanner } from '../components/ErrorBanner'
import { can } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import {
  Button,
  Card,
  DescriptionRow,
  EmptyState,
  ErrorState,
  Field,
  FieldGroup,
  Loading,
  PageHeader,
  inputClass,
  linkClass,
} from '../components/ui'
import { SEVERITIES, SEVERITY_LABELS } from './incidentLabels'
import { IncidentStatusBadge, ReplayBadge, SeverityBadge } from './incidentShared'
import type { AuditEvent, Incident, IncidentSeverity, User } from '../api/types'

type Confirmable = 'acknowledge' | 'resolve' | 'replay'

const CONFIRM_COPY: Record<Confirmable, { title: string; message: string; label: string }> = {
  acknowledge: { title: '确认异常', message: '标记为已确认，表示已有人在跟进，但尚未解决。', label: '确认' },
  resolve: {
    title: '解决异常',
    message: '解决后该异常不可再操作；同类故障再次发生时会开启一个新的异常。',
    label: '解决',
  },
  replay: {
    title: '重放死信',
    message: '将最近一次失败的消息重新投递。提交后状态为"等待结果"，真实结果由后台确认后更新。',
    label: '提交重放',
  },
}

export function IncidentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { user: me } = useAuth()
  const canManage = can(me?.role, 'incident:manage')

  const [incident, setIncident] = useState<Incident | null>(null)
  const [activity, setActivity] = useState<AuditEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [actionError, setActionError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const [confirming, setConfirming] = useState<Confirmable | null>(null)

  const load = useCallback(() => {
    if (!id) return
    setLoading(true)
    setLoadError(null)
    Promise.all([getIncident(id), getIncidentActivity(id)])
      .then(([inc, records]) => {
        setIncident(inc)
        setActivity(records)
      })
      .catch(setLoadError)
      .finally(() => setLoading(false))
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  const run = useCallback(
    async (action: () => Promise<Incident>): Promise<boolean> => {
      if (!id) return false
      setBusy(true)
      setActionError(null)
      try {
        setIncident(await action())
        // Actions return the incident, not its processing record.
        getIncidentActivity(id).then(setActivity).catch(() => {})
        return true
      } catch (err) {
        setActionError(err)
        return false
      } finally {
        setBusy(false)
        setConfirming(null)
      }
    },
    [id],
  )

  if (loading) return <Loading />
  if (loadError) return <ErrorState error={loadError} onRetry={load} />
  if (!incident || !id) return null

  const resolved = incident.status === 'resolved'
  const replayPending = incident.replay_status === 'pending'

  function confirmAction(kind: Confirmable) {
    const incidentId = incident!.id
    switch (kind) {
      case 'acknowledge':
        return run(() => acknowledgeIncident(incidentId))
      case 'resolve':
        return run(() => resolveIncident(incidentId))
      case 'replay':
        return run(() => requestIncidentReplay(incidentId))
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm">
        <Link to="/incidents" className={linkClass}>
          异常中心
        </Link>
        <span className="text-slate-600">/</span>
        <span className="truncate font-mono text-slate-300">{incident.event_type}</span>
      </div>

      <PageHeader
        eyebrow="Incident"
        title={<span className="font-mono">{incident.event_type}</span>}
        description={
          <span className="flex flex-wrap items-center gap-2">
            <SeverityBadge severity={incident.severity} />
            <IncidentStatusBadge status={incident.status} />
            {incident.replay_status && <ReplayBadge status={incident.replay_status} />}
          </span>
        }
      />

      <Card>
        <p className="font-mono text-[11px] uppercase tracking-wider text-slate-500">最近一次错误</p>
        <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-all rounded-lg border border-rose-400/20 bg-slate-950/70 p-3 font-mono text-xs text-rose-200/90">
          {incident.last_error || '—'}
        </pre>

        <dl className="mt-4 grid gap-x-8 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          <DescriptionRow label="消费者" value={incident.consumer} />
          <DescriptionRow label="累计发生" value={`${incident.occurrence_count} 次`} />
          <DescriptionRow label="首次发生" value={fmt(incident.first_seen_at)} />
          <DescriptionRow label="最近发生" value={fmt(incident.last_seen_at)} />
          <UserRow label="负责人" userId={incident.owner_id} placeholder="未指派" />
          <DescriptionRow label="死信 ID" value={incident.dead_letter_id} mono />
          {incident.acknowledged_at && <DescriptionRow label="确认时间" value={fmt(incident.acknowledged_at)} />}
          {incident.acknowledged_by && <UserRow label="确认人" userId={incident.acknowledged_by} />}
          {incident.resolved_at && <DescriptionRow label="解决时间" value={fmt(incident.resolved_at)} />}
          {incident.resolved_by && <UserRow label="解决人" userId={incident.resolved_by} />}
          {incident.replay_requested_at && <DescriptionRow label="重放提交时间" value={fmt(incident.replay_requested_at)} />}
          {incident.replay_result_at && <DescriptionRow label="重放结果时间" value={fmt(incident.replay_result_at)} />}
        </dl>

        {canManage && !resolved && (
          <div className="mt-5 space-y-4 border-t border-white/[0.06] pt-5">
            {actionError ? <ErrorBanner error={actionError} /> : null}

            <div className="flex flex-wrap gap-2">
              {incident.status === 'open' && (
                <Button variant="primary" disabled={busy} onClick={() => setConfirming('acknowledge')}>
                  确认
                </Button>
              )}
              <Button
                disabled={busy || replayPending}
                onClick={() => setConfirming('replay')}
                title={replayPending ? '上一次重放仍在等待结果' : undefined}
              >
                重放死信
              </Button>
              <Button variant="success" disabled={busy} onClick={() => setConfirming('resolve')}>
                解决
              </Button>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <SeverityForm
                current={incident.severity}
                busy={busy}
                onSubmit={(s) => run(() => setIncidentSeverity(incident.id, s))}
              />
              <OwnerForm
                current={incident.owner_id}
                meId={me?.id}
                canListUsers={can(me?.role, 'user:list')}
                busy={busy}
                onSubmit={(ownerId) => run(() => assignIncidentOwner(incident.id, ownerId))}
              />
            </div>

            <NoteForm busy={busy} onSubmit={(note) => run(() => addIncidentNote(incident.id, note))} />
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
                <span className="absolute -left-[21px] top-1.5 h-2 w-2 rounded-full bg-cyan-400 shadow-[0_0_8px_rgba(34,211,238,0.8)]" />
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium text-slate-100">{describeAction(e)}</span>
                  <span className="whitespace-nowrap font-mono text-xs tabular-nums text-slate-500">{fmt(e.created_at)}</span>
                </div>
                {noteFromEvent(e) && <p className="mt-1 whitespace-pre-wrap text-slate-400">{noteFromEvent(e)}</p>}
              </li>
            ))}
          </ol>
        )}
      </Card>

      {confirming && (
        <ConfirmDialog
          title={CONFIRM_COPY[confirming].title}
          message={CONFIRM_COPY[confirming].message}
          confirmLabel={CONFIRM_COPY[confirming].label}
          busy={busy}
          onConfirm={() => confirmAction(confirming)}
          onCancel={() => setConfirming(null)}
        />
      )}
    </div>
  )
}

function fmt(ts: string): string {
  return new Date(ts).toLocaleString('zh-CN')
}

/** Best-effort id → email, falling back to the id. Without user:read the
 * lookup is skipped rather than sent to be refused. */
function UserRow({ label, userId, placeholder }: { label: string; userId?: string; placeholder?: string }) {
  const { user: me } = useAuth()
  const canReadUsers = can(me?.role, 'user:read')
  const [display, setDisplay] = useState<{ id: string; text: string } | null>(null)

  useEffect(() => {
    if (!userId || !canReadUsers) return
    let cancelled = false
    getUser(userId)
      .then((u) => !cancelled && setDisplay({ id: userId, text: u.email }))
      .catch(() => !cancelled && setDisplay({ id: userId, text: userId }))
    return () => {
      cancelled = true
    }
  }, [userId, canReadUsers])

  if (!userId) return <DescriptionRow label={label} value={placeholder ?? '—'} />
  return <DescriptionRow label={label} value={display?.id === userId ? display.text : userId} />
}

function SeverityForm({
  current,
  busy,
  onSubmit,
}: {
  current: IncidentSeverity
  busy: boolean
  onSubmit: (s: IncidentSeverity) => void
}) {
  const [value, setValue] = useState<IncidentSeverity>(current)
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit(value)
      }}
    >
      <Field label="严重级别">
        <div className="flex gap-2">
          <select value={value} onChange={(e) => setValue(e.target.value as IncidentSeverity)} className={inputClass}>
            {SEVERITIES.map((s) => (
              <option key={s} value={s}>
                {SEVERITY_LABELS[s]}
              </option>
            ))}
          </select>
          <Button type="submit" disabled={busy || value === current}>
            保存
          </Button>
        </div>
      </Field>
    </form>
  )
}

function OwnerForm({
  current,
  meId,
  canListUsers,
  busy,
  onSubmit,
}: {
  current?: string
  meId?: string
  canListUsers: boolean
  busy: boolean
  onSubmit: (ownerId: string) => void
}) {
  const [users, setUsers] = useState<User[]>([])
  const [value, setValue] = useState(current ?? '')

  useEffect(() => {
    if (!canListUsers) return
    listUsers({ page: 1, page_size: 100, sort: 'name' })
      .then((p) => setUsers(p.items))
      .catch(() => {})
  }, [canListUsers])

  // A group, not Field's <label>: a label activates its first labelable
  // child, and with no select rendered that is "指派给我" — clicking the
  // caption would assign the incident.
  return (
    <FieldGroup id="incident-owner-label" label="负责人">
      <div className="flex flex-wrap gap-2">
        {canListUsers && (
          <form
            className="flex gap-2"
            onSubmit={(e: FormEvent) => {
              e.preventDefault()
              onSubmit(value)
            }}
          >
            <select
              aria-labelledby="incident-owner-label"
              value={value}
              onChange={(e) => setValue(e.target.value)}
              className={inputClass}
            >
              <option value="">未指派</option>
              {users.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.name}（{u.email}）
                </option>
              ))}
            </select>
            <Button type="submit" disabled={busy || value === (current ?? '')}>
              保存
            </Button>
          </form>
        )}
        {meId && current !== meId && (
          <Button disabled={busy} onClick={() => onSubmit(meId)}>
            指派给我
          </Button>
        )}
        {current && (
          <Button variant="ghost" disabled={busy} onClick={() => onSubmit('')}>
            清除
          </Button>
        )}
      </div>
    </FieldGroup>
  )
}

function NoteForm({ busy, onSubmit }: { busy: boolean; onSubmit: (note: string) => Promise<boolean> }) {
  const [note, setNote] = useState('')
  return (
    <form
      onSubmit={async (e) => {
        e.preventDefault()
        if (await onSubmit(note)) setNote('')
      }}
      className="space-y-2"
    >
      <Field label="处理备注">
        <textarea
          required
          minLength={1}
          maxLength={4000}
          rows={3}
          value={note}
          onChange={(e) => setNote(e.target.value)}
          className={inputClass}
        />
      </Field>
      <div className="flex justify-end">
        <Button type="submit" variant="primary" disabled={busy || !note.trim()}>
          添加备注
        </Button>
      </div>
    </form>
  )
}

function describeAction(e: AuditEvent): string {
  const after = e.after as Record<string, unknown> | undefined
  switch (e.action) {
    case 'incident.opened':
      return '检测到异常'
    case 'incident.severity_set': {
      const s = after?.severity
      return typeof s === 'string' && s in SEVERITY_LABELS
        ? `调整级别为「${SEVERITY_LABELS[s as IncidentSeverity]}」`
        : '调整级别'
    }
    case 'incident.owner_assigned':
      return after?.owner_id ? '指派负责人' : '清除负责人'
    case 'incident.acknowledged':
      return '确认异常'
    case 'incident.note_added':
      return '添加处理备注'
    case 'incident.resolved':
      return '解决异常'
    case 'incident.replay_requested':
      return '提交死信重放'
    case 'incident.replay_resolved':
      return after?.result === 'succeeded' ? '重放成功' : after?.result === 'failed' ? '重放失败' : '重放有结果'
    case 'incident.merged': {
      const n = after?.occurrence_count
      return typeof n === 'number' ? `再次发生（累计 ${n} 次）` : '再次发生'
    }
    default:
      return e.action
  }
}

function noteFromEvent(e: AuditEvent): string | null {
  const note = (e.after as Record<string, unknown> | undefined)?.note
  return typeof note === 'string' ? note : null
}
