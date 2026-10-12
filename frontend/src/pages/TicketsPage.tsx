import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { createTicket, listTickets } from '../api/tickets'
import { ErrorBanner } from '../components/ErrorBanner'
import { Modal } from '../components/Modal'
import { can } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import {
  Button,
  ErrorState,
  Field,
  PageHeader,
  Pagination,
  TBody,
  THead,
  Table,
  TableMessage,
  Td,
  Th,
  Tr,
  controlClass,
  inputClass,
  linkClass,
} from '../components/ui'
import { PriorityBadge, STATUS_LABELS, StatusBadge } from './ticketShared'
import type { Ticket, TicketPriority, TicketScope, TicketStatus } from '../api/types'

const PAGE_SIZE = 20

type SortKey = 'created_at' | 'priority' | 'status'

const SCOPE_TABS: { scope: TicketScope; label: string }[] = [
  { scope: 'mine', label: '我的申请' },
  { scope: 'assigned', label: '分配给我' },
  { scope: 'all', label: '全部工单' },
]

/** Reads and writes every list condition through the URL, per the product
 * spec's "列表条件保存在 URL 中" — a bookmarked or shared link, or the
 * browser back button, reproduces the exact same filtered view. */
export function TicketsPage() {
  const { user: me } = useAuth()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const canSeeAll = can(me?.role, 'ticket:read_all')

  const scope = (params.get('scope') as TicketScope | null) ?? 'mine'
  const status = (params.get('status') as TicketStatus | null) ?? ''
  const priority = (params.get('priority') as TicketPriority | null) ?? ''
  const search = params.get('search') ?? ''
  const sort = (params.get('sort') as SortKey | null) ?? 'created_at'
  const sortDir = params.get('sort_dir') === 'asc' ? 'asc' : 'desc'
  const page = Number(params.get('page') ?? '1') || 1

  const [searchInput, setSearchInput] = useState(search)
  const [tickets, setTickets] = useState<Ticket[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [reloadTick, setReloadTick] = useState(0)
  const [creating, setCreating] = useState(false)

  function patchParams(patch: Record<string, string | number | undefined>, resetPage = true) {
    const next = new URLSearchParams(params)
    for (const [key, value] of Object.entries(patch)) {
      if (value === undefined || value === '') next.delete(key)
      else next.set(key, String(value))
    }
    if (resetPage) next.delete('page')
    setParams(next, { replace: true })
  }

  // The search box debounces locally before it ever touches the URL —
  // writing every keystroke would spam the history stack and the API both.
  useEffect(() => {
    const t = setTimeout(() => {
      if (searchInput !== search) patchParams({ search: searchInput })
    }, 300)
    return () => clearTimeout(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchInput])

  useEffect(() => {
    if (scope === 'all' && !canSeeAll) {
      patchParams({ scope: 'mine' })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scope, canSeeAll])

  useEffect(() => {
    setLoading(true)
    setLoadError(null)
    listTickets({
      scope,
      page,
      page_size: PAGE_SIZE,
      status: status || undefined,
      priority: priority || undefined,
      search: search || undefined,
      sort,
      sort_dir: sortDir,
    })
      .then((data) => {
        setTickets(data.items)
        setTotalPages(Math.max(data.page_info.total_pages, 1))
        setTotal(data.page_info.total)
      })
      .catch(setLoadError)
      .finally(() => setLoading(false))
  }, [scope, status, priority, search, sort, sortDir, page, reloadTick])

  function toggleSort(key: SortKey) {
    if (sort === key) {
      patchParams({ sort: key, sort_dir: sortDir === 'asc' ? 'desc' : 'asc' })
    } else {
      patchParams({ sort: key, sort_dir: 'asc' })
    }
  }

  return (
    <div className="space-y-4">
      <PageHeader
        eyebrow="Tickets"
        title={
          <>
            工单{' '}
            {total ? <span className="font-mono text-sm font-normal tabular-nums text-slate-400">（共 {total} 个）</span> : null}
          </>
        }
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            提交工单
          </Button>
        }
      />

      <div className="inline-flex flex-wrap gap-1 rounded-lg border border-white/[0.07] bg-slate-950/50 p-1">
        {SCOPE_TABS.filter((t) => t.scope !== 'all' || canSeeAll).map((t) => (
          <button
            key={t.scope}
            onClick={() => patchParams({ scope: t.scope })}
            className={`rounded-md px-3 py-1 text-sm font-medium transition ${
              scope === t.scope
                ? 'bg-cyan-400/15 text-cyan-200 shadow-[inset_0_0_0_1px_rgba(34,211,238,0.3)]'
                : 'text-slate-400 hover:text-slate-100'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <input
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="搜索标题或分类"
          className={`${inputClass} max-w-xs`}
        />
        <select
          value={status}
          onChange={(e) => patchParams({ status: e.target.value })}
          className={controlClass}
        >
          <option value="">全部状态</option>
          {(Object.keys(STATUS_LABELS) as TicketStatus[]).map((s) => (
            <option key={s} value={s}>
              {STATUS_LABELS[s]}
            </option>
          ))}
        </select>
        <select
          value={priority}
          onChange={(e) => patchParams({ priority: e.target.value })}
          className={controlClass}
        >
          <option value="">全部优先级</option>
          <option value="low">低</option>
          <option value="normal">普通</option>
          <option value="high">高</option>
          <option value="urgent">紧急</option>
        </select>
      </div>

      {loadError ? <ErrorState error={loadError} onRetry={() => setReloadTick((t) => t + 1)} /> : null}

      <Table minWidth="42rem">
        <colgroup>
          <col className="w-[36%]" />
          <col className="w-[16%]" />
          <col className="w-[14%]" />
          <col className="w-[14%]" />
          <col className="w-[20%]" />
        </colgroup>
        <THead>
          <tr>
            <Th>标题</Th>
            <Th>分类</Th>
            <SortableHeader label="优先级" sortKey="priority" active={sort} dir={sortDir} onToggle={toggleSort} />
            <SortableHeader label="状态" sortKey="status" active={sort} dir={sortDir} onToggle={toggleSort} />
            <SortableHeader label="更新时间" sortKey="created_at" active={sort} dir={sortDir} onToggle={toggleSort} />
          </tr>
        </THead>
        <TBody>
          {loading ? (
            <TableMessage colSpan={5}>加载中…</TableMessage>
          ) : tickets.length === 0 ? (
            <TableMessage colSpan={5}>{search || status || priority ? '没有匹配的工单' : '还没有工单'}</TableMessage>
          ) : (
            tickets.map((t) => (
              <Tr key={t.id}>
                <td className="truncate px-4 py-2.5 text-slate-100" title={t.title}>
                  <Link to={`/tickets/${t.id}`} className={linkClass}>
                    {t.title}
                  </Link>
                </td>
                <td className="truncate px-4 py-2.5 text-slate-400" title={t.category}>
                  {t.category}
                </td>
                <Td className="whitespace-nowrap">
                  <PriorityBadge priority={t.priority} />
                </Td>
                <Td className="whitespace-nowrap">
                  <StatusBadge status={t.status} />
                </Td>
                <Td className="whitespace-nowrap font-mono text-xs tabular-nums text-slate-400">
                  {new Date(t.updated_at).toLocaleString('zh-CN')}
                </Td>
              </Tr>
            ))
          )}
        </TBody>
      </Table>

      <Pagination page={page} totalPages={totalPages} onChange={(p) => patchParams({ page: p }, false)} />

      {creating && (
        <CreateTicketModal
          onClose={() => setCreating(false)}
          onCreated={(t) => {
            setCreating(false)
            navigate(`/tickets/${t.id}`)
          }}
        />
      )}
    </div>
  )
}

function SortableHeader({
  label,
  sortKey,
  active,
  dir,
  onToggle,
}: {
  label: string
  sortKey: SortKey
  active: SortKey
  dir: 'asc' | 'desc'
  onToggle: (key: SortKey) => void
}) {
  const isActive = active === sortKey
  return (
    <Th>
      <button
        onClick={() => onToggle(sortKey)}
        className={`flex items-center gap-1 uppercase tracking-wider transition hover:text-cyan-200 ${
          isActive ? 'text-cyan-300' : ''
        }`}
      >
        {label}
        {isActive ? <span aria-hidden>{dir === 'asc' ? '↑' : '↓'}</span> : null}
      </button>
    </Th>
  )
}

function CreateTicketModal({ onClose, onCreated }: { onClose: () => void; onCreated: (t: Ticket) => void }) {
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState('')
  const [priority, setPriority] = useState<TicketPriority>('normal')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const t = await createTicket({ title, description, category, priority })
      onCreated(t)
    } catch (err) {
      setError(err)
      setBusy(false)
    }
  }

  return (
    <Modal title="提交工单" onClose={onClose}>
      <form onSubmit={handleSubmit} className="space-y-3">
        {error ? <ErrorBanner error={error} /> : null}
        <Field label="标题">
          <input required minLength={1} maxLength={200} value={title} onChange={(e) => setTitle(e.target.value)} className={inputClass} />
        </Field>
        <Field label="分类">
          <input
            required
            minLength={1}
            maxLength={100}
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            placeholder="例如：设备维修、账号问题"
            className={inputClass}
          />
        </Field>
        <Field label="优先级">
          <select value={priority} onChange={(e) => setPriority(e.target.value as TicketPriority)} className={inputClass}>
            <option value="low">低</option>
            <option value="normal">普通</option>
            <option value="high">高</option>
            <option value="urgent">紧急</option>
          </select>
        </Field>
        <Field label="描述">
          <textarea
            required
            minLength={1}
            maxLength={8000}
            rows={5}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className={inputClass}
          />
        </Field>
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" onClick={onClose}>
            取消
          </Button>
          <Button type="submit" variant="primary" disabled={busy}>
            {busy ? '提交中…' : '提交'}
          </Button>
        </div>
      </form>
    </Modal>
  )
}
