import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getIncidentSummary, listIncidents } from '../api/incidents'
import { ErrorBanner } from '../components/ErrorBanner'
import { useAuth } from '../auth/useAuth'
import { inputClass } from './MembersPage'
import {
  INCIDENT_STATUS_LABELS,
  IncidentStatusBadge,
  ReplayBadge,
  SEVERITIES,
  SEVERITY_LABELS,
  SeverityBadge,
} from './incidentShared'
import type { Incident, IncidentSeverity, IncidentStatus, IncidentSummary } from '../api/types'

const PAGE_SIZE = 20

/** The exception center's list. Every filter lives in the URL, the same as
 * TicketsPage, so a shared link reproduces the exact view. Incidents are
 * only ever created by the worker's watcher, so there is no "create" here. */
export function IncidentsPage() {
  const { user: me } = useAuth()
  const [params, setParams] = useSearchParams()

  const status = (params.get('status') as IncidentStatus | null) ?? ''
  const severity = (params.get('severity') as IncidentSeverity | null) ?? ''
  const mine = params.get('mine') === '1'
  const search = params.get('search') ?? ''
  const page = Number(params.get('page') ?? '1') || 1

  const [searchInput, setSearchInput] = useState(search)
  const [incidents, setIncidents] = useState<Incident[]>([])
  const [summary, setSummary] = useState<IncidentSummary | null>(null)
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [reloadTick, setReloadTick] = useState(0)

  function patchParams(patch: Record<string, string | number | undefined>, resetPage = true) {
    const next = new URLSearchParams(params)
    for (const [key, value] of Object.entries(patch)) {
      if (value === undefined || value === '') next.delete(key)
      else next.set(key, String(value))
    }
    if (resetPage) next.delete('page')
    setParams(next, { replace: true })
  }

  useEffect(() => {
    const t = setTimeout(() => {
      if (searchInput !== search) patchParams({ search: searchInput })
    }, 300)
    return () => clearTimeout(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchInput])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setLoadError(null)
    Promise.all([
      listIncidents({
        page,
        page_size: PAGE_SIZE,
        status: status || undefined,
        severity: severity || undefined,
        owner_id: mine ? me?.id : undefined,
        search: search || undefined,
      }),
      getIncidentSummary(),
    ])
      .then(([data, counts]) => {
        if (cancelled) return
        setIncidents(data.items)
        setTotalPages(Math.max(data.page_info.total_pages, 1))
        setTotal(data.page_info.total)
        setSummary(counts)
      })
      .catch((err) => {
        if (!cancelled) setLoadError(err)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [status, severity, mine, me?.id, search, page, reloadTick])

  const filtered = Boolean(status || severity || mine || search)

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">
          异常中心 {total ? <span className="text-sm font-normal text-neutral-500">（共 {total} 个）</span> : null}
        </h1>
        <button
          onClick={() => setReloadTick((t) => t + 1)}
          className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
        >
          刷新
        </button>
      </div>

      {/* Unresolved counts per severity; clicking one filters the list to it. */}
      <div className="grid grid-cols-3 gap-3">
        {SEVERITIES.map((s) => {
          const active = severity === s
          return (
            <button
              key={s}
              onClick={() => patchParams({ severity: active ? '' : s })}
              aria-pressed={active}
              className={`rounded-lg border px-4 py-3 text-left transition-colors ${
                active
                  ? 'border-neutral-900 dark:border-neutral-100'
                  : 'border-neutral-200 hover:border-neutral-400 dark:border-neutral-800 dark:hover:border-neutral-600'
              }`}
            >
              <div className="text-xs text-neutral-500 dark:text-neutral-400">未解决 · {SEVERITY_LABELS[s]}</div>
              <div className="mt-1 text-2xl font-semibold text-neutral-900 dark:text-neutral-100">
                {summary ? (summary.counts[s] ?? 0) : '—'}
              </div>
            </button>
          )
        })}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <input
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="搜索事件类型或消费者"
          className={`${inputClass} max-w-xs`}
        />
        <select value={status} onChange={(e) => patchParams({ status: e.target.value })} className={inputClass}>
          <option value="">全部状态</option>
          {(Object.keys(INCIDENT_STATUS_LABELS) as IncidentStatus[]).map((s) => (
            <option key={s} value={s}>
              {INCIDENT_STATUS_LABELS[s]}
            </option>
          ))}
        </select>
        <select value={severity} onChange={(e) => patchParams({ severity: e.target.value })} className={inputClass}>
          <option value="">全部级别</option>
          {SEVERITIES.map((s) => (
            <option key={s} value={s}>
              {SEVERITY_LABELS[s]}
            </option>
          ))}
        </select>
        <label className="flex items-center gap-1.5 text-sm text-neutral-600 dark:text-neutral-300">
          <input type="checkbox" checked={mine} onChange={(e) => patchParams({ mine: e.target.checked ? '1' : '' })} />
          只看我负责的
        </label>
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
          <table className="w-full min-w-[48rem] table-fixed text-left text-sm">
            <colgroup>
              <col className="w-[34%]" />
              <col className="w-[10%]" />
              <col className="w-[10%]" />
              <col className="w-[8%]" />
              <col className="w-[18%]" />
              <col className="w-[20%]" />
            </colgroup>
            <thead className="bg-neutral-50 text-neutral-500 dark:bg-neutral-900 dark:text-neutral-400">
              <tr>
                <th className="px-4 py-2 font-medium">异常</th>
                <th className="px-4 py-2 font-medium">级别</th>
                <th className="px-4 py-2 font-medium">状态</th>
                <th className="px-4 py-2 font-medium">次数</th>
                <th className="px-4 py-2 font-medium">重放</th>
                <th className="px-4 py-2 font-medium">最近发生</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
              {loading ? (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-neutral-400">
                    加载中…
                  </td>
                </tr>
              ) : incidents.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-neutral-400">
                    {filtered ? '没有匹配的异常' : '目前没有异常'}
                  </td>
                </tr>
              ) : (
                incidents.map((inc) => (
                  <tr key={inc.id}>
                    <td className="px-4 py-2">
                      <Link
                        to={`/incidents/${inc.id}`}
                        className="block truncate font-medium text-neutral-900 hover:underline dark:text-neutral-100"
                        title={inc.event_type}
                      >
                        {inc.event_type}
                      </Link>
                      <div className="truncate text-xs text-neutral-500 dark:text-neutral-400" title={inc.last_error}>
                        {inc.consumer} · {inc.last_error}
                      </div>
                    </td>
                    <td className="whitespace-nowrap px-4 py-2">
                      <SeverityBadge severity={inc.severity} />
                    </td>
                    <td className="whitespace-nowrap px-4 py-2">
                      <IncidentStatusBadge status={inc.status} />
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-neutral-700 dark:text-neutral-300">{inc.occurrence_count}</td>
                    <td className="truncate px-4 py-2">
                      {inc.replay_status ? <ReplayBadge status={inc.replay_status} /> : <span className="text-neutral-400">—</span>}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-neutral-500 dark:text-neutral-400">
                      {new Date(inc.last_seen_at).toLocaleString('zh-CN')}
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
    </div>
  )
}
