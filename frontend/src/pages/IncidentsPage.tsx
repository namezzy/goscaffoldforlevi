import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getIncidentSummary, listIncidents } from '../api/incidents'
import { useAuth } from '../auth/useAuth'
import {
  Button,
  ErrorState,
  PageHeader,
  Pagination,
  TBody,
  THead,
  Table,
  TableMessage,
  Td,
  Th,
  Tr,
  cardClass,
  controlClass,
  inputClass,
} from '../components/ui'
import { INCIDENT_STATUS_LABELS, SEVERITIES, SEVERITY_LABELS } from './incidentLabels'
import { IncidentStatusBadge, ReplayBadge, SeverityBadge, SeverityDot } from './incidentShared'
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
    <div className="space-y-6">
      <PageHeader
        eyebrow="Incidents"
        title={
          <>
            异常中心{' '}
            {total ? <span className="font-mono text-sm font-normal tabular-nums text-slate-400">（共 {total} 个）</span> : null}
          </>
        }
        actions={<Button onClick={() => setReloadTick((t) => t + 1)}>刷新</Button>}
      />

      {/* Unresolved counts per severity; clicking one filters the list to it. */}
      <div className="grid grid-cols-3 gap-4">
        {SEVERITIES.map((s) => {
          const active = severity === s
          return (
            <button
              key={s}
              onClick={() => patchParams({ severity: active ? '' : s })}
              aria-pressed={active}
              className={`${cardClass} px-4 py-3 text-left transition ${
                active
                  ? 'border-cyan-400/50 bg-cyan-400/[0.06] shadow-[0_0_18px_-6px_rgba(34,211,238,0.6)]'
                  : 'hover:border-cyan-400/30 hover:bg-slate-900/80'
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="font-mono text-[11px] uppercase tracking-wider text-slate-400">未解决 · {SEVERITY_LABELS[s]}</span>
                <SeverityDot severity={s} />
              </div>
              <div className="mt-2 font-mono text-3xl font-semibold tabular-nums text-slate-50">
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
        <select value={status} onChange={(e) => patchParams({ status: e.target.value })} className={controlClass}>
          <option value="">全部状态</option>
          {(Object.keys(INCIDENT_STATUS_LABELS) as IncidentStatus[]).map((s) => (
            <option key={s} value={s}>
              {INCIDENT_STATUS_LABELS[s]}
            </option>
          ))}
        </select>
        <select value={severity} onChange={(e) => patchParams({ severity: e.target.value })} className={controlClass}>
          <option value="">全部级别</option>
          {SEVERITIES.map((s) => (
            <option key={s} value={s}>
              {SEVERITY_LABELS[s]}
            </option>
          ))}
        </select>
        <label className="flex items-center gap-1.5 text-sm text-slate-300">
          <input
            type="checkbox"
            checked={mine}
            onChange={(e) => patchParams({ mine: e.target.checked ? '1' : '' })}
            className="accent-cyan-400"
          />
          只看我负责的
        </label>
      </div>

      {loadError ? <ErrorState error={loadError} onRetry={() => setReloadTick((t) => t + 1)} /> : null}

      <Table minWidth="48rem">
        <colgroup>
          <col className="w-[34%]" />
          <col className="w-[10%]" />
          <col className="w-[10%]" />
          <col className="w-[8%]" />
          <col className="w-[18%]" />
          <col className="w-[20%]" />
        </colgroup>
        <THead>
          <tr>
            <Th>异常</Th>
            <Th>级别</Th>
            <Th>状态</Th>
            <Th>次数</Th>
            <Th>重放</Th>
            <Th>最近发生</Th>
          </tr>
        </THead>
        <TBody>
          {loading ? (
            <TableMessage colSpan={6}>加载中…</TableMessage>
          ) : incidents.length === 0 ? (
            <TableMessage colSpan={6}>{filtered ? '没有匹配的异常' : '目前没有异常'}</TableMessage>
          ) : (
            incidents.map((inc) => (
              <Tr key={inc.id}>
                <Td>
                  <Link
                    to={`/incidents/${inc.id}`}
                    className="block truncate font-mono font-medium text-slate-100 transition hover:text-cyan-300"
                    title={inc.event_type}
                  >
                    {inc.event_type}
                  </Link>
                  <div className="truncate text-xs text-slate-500" title={inc.last_error}>
                    {inc.consumer} · {inc.last_error}
                  </div>
                </Td>
                <Td className="whitespace-nowrap">
                  <SeverityBadge severity={inc.severity} />
                </Td>
                <Td className="whitespace-nowrap">
                  <IncidentStatusBadge status={inc.status} />
                </Td>
                <Td className="whitespace-nowrap font-mono tabular-nums">{inc.occurrence_count}</Td>
                <Td className="truncate">
                  {inc.replay_status ? <ReplayBadge status={inc.replay_status} /> : <span className="text-slate-500">—</span>}
                </Td>
                <Td className="whitespace-nowrap font-mono text-xs tabular-nums text-slate-400">
                  {new Date(inc.last_seen_at).toLocaleString('zh-CN')}
                </Td>
              </Tr>
            ))
          )}
        </TBody>
      </Table>

      <Pagination page={page} totalPages={totalPages} onChange={(p) => patchParams({ page: p }, false)} />
    </div>
  )
}
