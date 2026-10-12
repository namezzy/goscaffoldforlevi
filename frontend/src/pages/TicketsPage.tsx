import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { createTicket, listTickets } from '../api/tickets'
import { ErrorBanner } from '../components/ErrorBanner'
import { Modal } from '../components/Modal'
import { can } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import { Field, inputClass } from './MembersPage'
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
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">
          工单 {total ? <span className="text-sm font-normal text-neutral-500">（共 {total} 个）</span> : null}
        </h1>
        <button
          onClick={() => setCreating(true)}
          className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
        >
          提交工单
        </button>
      </div>

      <div className="flex gap-1 border-b border-neutral-200 dark:border-neutral-800">
        {SCOPE_TABS.filter((t) => t.scope !== 'all' || canSeeAll).map((t) => (
          <button
            key={t.scope}
            onClick={() => patchParams({ scope: t.scope })}
            className={`border-b-2 px-3 py-2 text-sm font-medium ${
              scope === t.scope
                ? 'border-neutral-900 text-neutral-900 dark:border-neutral-100 dark:text-neutral-100'
                : 'border-transparent text-neutral-500 hover:text-neutral-700 dark:text-neutral-400 dark:hover:text-neutral-200'
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
          className={inputClass}
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
          className={inputClass}
        >
          <option value="">全部优先级</option>
          <option value="low">低</option>
          <option value="normal">普通</option>
          <option value="high">高</option>
          <option value="urgent">紧急</option>
        </select>
      </div>

      {loadError ? (
        <div className="space-y-2">
          <ErrorBanner error={loadError} />
          <button
            onClick={() => setReloadTick((t) => t + 1)}
            className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
          >
            重试
          </button>
        </div>
      ) : null}

      <div className="overflow-hidden rounded-lg border border-neutral-200 dark:border-neutral-800">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[42rem] table-fixed text-left text-sm">
            <colgroup>
              <col className="w-[36%]" />
              <col className="w-[16%]" />
              <col className="w-[14%]" />
              <col className="w-[14%]" />
              <col className="w-[20%]" />
            </colgroup>
            <thead className="bg-neutral-50 text-neutral-500 dark:bg-neutral-900 dark:text-neutral-400">
              <tr>
                <th className="px-4 py-2 font-medium">标题</th>
                <th className="px-4 py-2 font-medium">分类</th>
                <SortableHeader label="优先级" sortKey="priority" active={sort} dir={sortDir} onToggle={toggleSort} />
                <SortableHeader label="状态" sortKey="status" active={sort} dir={sortDir} onToggle={toggleSort} />
                <SortableHeader label="更新时间" sortKey="created_at" active={sort} dir={sortDir} onToggle={toggleSort} />
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
              {loading ? (
                <tr>
                  <td colSpan={5} className="px-4 py-6 text-center text-neutral-400">
                    加载中…
                  </td>
                </tr>
              ) : tickets.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-4 py-6 text-center text-neutral-400">
                    {search || status || priority ? '没有匹配的工单' : '还没有工单'}
                  </td>
                </tr>
              ) : (
                tickets.map((t) => (
                  <tr key={t.id}>
                    <td className="truncate px-4 py-2 text-neutral-900 dark:text-neutral-100" title={t.title}>
                      <Link to={`/tickets/${t.id}`} className="hover:underline">
                        {t.title}
                      </Link>
                    </td>
                    <td className="truncate px-4 py-2 text-neutral-500 dark:text-neutral-400" title={t.category}>
                      {t.category}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2">
                      <PriorityBadge priority={t.priority} />
                    </td>
                    <td className="whitespace-nowrap px-4 py-2">
                      <StatusBadge status={t.status} />
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-neutral-500 dark:text-neutral-400">
                      {new Date(t.updated_at).toLocaleString('zh-CN')}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-3 text-sm">
          <button
            onClick={() => patchParams({ page: Math.max(page - 1, 1) }, false)}
            disabled={page <= 1}
            className="rounded-md border border-neutral-300 px-2 py-1 disabled:opacity-40 dark:border-neutral-700"
          >
            上一页
          </button>
          <span className="text-neutral-500">
            第 {page} / {totalPages} 页
          </span>
          <button
            onClick={() => patchParams({ page: Math.min(page + 1, totalPages) }, false)}
            disabled={page >= totalPages}
            className="rounded-md border border-neutral-300 px-2 py-1 disabled:opacity-40 dark:border-neutral-700"
          >
            下一页
          </button>
        </div>
      )}

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
    <th className="px-4 py-2 font-medium">
      <button
        onClick={() => onToggle(sortKey)}
        className={`flex items-center gap-1 hover:text-neutral-900 dark:hover:text-neutral-100 ${
          isActive ? 'text-neutral-900 dark:text-neutral-100' : ''
        }`}
      >
        {label}
        {isActive ? <span aria-hidden>{dir === 'asc' ? '↑' : '↓'}</span> : null}
      </button>
    </th>
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
          <button type="button" onClick={onClose} className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm dark:border-neutral-700">
            取消
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
          >
            {busy ? '提交中…' : '提交'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
