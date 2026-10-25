import { useMemo } from 'react'
import type { RequestEvent } from '../api/types'
import { normalizePath } from './requestPathTree'
import { MAX_ITEMS, RATE_WINDOW_MS, type ConnectionStatus, type RequestFeed } from './useRequestFeed'
import { Badge, cardClass } from './ui'

export function LiveRequestFeed({
  feed,
  pathFilter,
  onClearFilter,
}: {
  feed: RequestFeed
  /** Normalized path prefix selected on the map; undefined shows everything. */
  pathFilter?: string
  onClearFilter?: () => void
}) {
  const { status, rateCount } = feed
  const rate = rateCount / (RATE_WINDOW_MS / 1000)

  const items = useMemo(
    () =>
      pathFilter
        ? feed.items.filter((e) => {
            const p = normalizePath(e.path)
            return p === pathFilter || p.startsWith(`${pathFilter}/`)
          })
        : feed.items,
    [feed.items, pathFilter],
  )

  const buckets = { '2xx': 0, '3xx': 0, '4xx': 0, '5xx': 0 }
  for (const e of items) {
    const key = `${Math.floor(e.status / 100)}xx` as keyof typeof buckets
    if (key in buckets) buckets[key]++
  }

  return (
    <section className={`${cardClass} overflow-hidden`}>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-white/[0.06] px-5 py-3">
        <div className="flex items-center gap-2">
          <ConnectionDot status={status} />
          <h2 className="text-sm font-semibold text-slate-100">实时请求流</h2>
          {pathFilter && (
            <button
              type="button"
              onClick={onClearFilter}
              className="flex items-center gap-1 rounded-full border border-cyan-400/30 bg-cyan-400/10 px-2 py-0.5 font-mono text-[11px] text-cyan-200 transition hover:border-cyan-300/50 hover:bg-cyan-400/20"
              title="点击取消筛选"
            >
              {pathFilter}
              <span aria-hidden className="text-cyan-400/70">
                ✕
              </span>
            </button>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-4 font-mono text-xs tabular-nums text-slate-400">
          <span title={`最近 ${RATE_WINDOW_MS / 1000} 秒内收到的请求数 ÷ ${RATE_WINDOW_MS / 1000}`}>
            <span className="text-cyan-300">{rate.toFixed(1)}</span> req/s
            <span className="text-slate-500">（近{RATE_WINDOW_MS / 1000}s）</span>
          </span>
          <span
            className="text-slate-500"
            title={`下方列表最多展示最近 ${MAX_ITEMS} 条，速率统计不受此限制`}
          >
            展示 {items.length}/{MAX_ITEMS}
          </span>
          <span className="text-emerald-300">2xx {buckets['2xx']}</span>
          <span className="text-sky-300">3xx {buckets['3xx']}</span>
          <span className="text-amber-300">4xx {buckets['4xx']}</span>
          <span className="text-rose-300">5xx {buckets['5xx']}</span>
        </div>
      </div>

      <div className="max-h-96 overflow-y-auto font-mono text-xs tabular-nums">
        {items.length === 0 ? (
          <p className="px-4 py-8 text-center text-slate-500">
            {pathFilter ? '该路径下还没有请求' : status === 'connecting' ? '连接中…' : '等待请求…'}
          </p>
        ) : (
          <table className="w-full border-collapse">
            <thead className="sticky top-0 bg-slate-900/95 text-[11px] uppercase tracking-wider text-slate-500 backdrop-blur">
              <tr className="border-b border-white/[0.06]">
                <th className="px-4 py-1.5 text-left font-medium">时间</th>
                <th className="px-2 py-1.5 text-left font-medium">方法</th>
                <th className="px-2 py-1.5 text-left font-medium">路径</th>
                <th className="px-2 py-1.5 text-left font-medium">IP</th>
                <th className="px-2 py-1.5 text-left font-medium">地区</th>
                <th className="px-2 py-1.5 text-right font-medium">状态</th>
                <th className="px-2 py-1.5 text-right font-medium">耗时</th>
                <th className="px-4 py-1.5 text-right font-medium">大小</th>
              </tr>
            </thead>
            <tbody>
              {items.map((e, i) => (
                <RequestRow key={e.request_id} event={e} isNew={i === 0 && status === 'live'} />
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  )
}

function ConnectionDot({ status }: { status: ConnectionStatus }) {
  const color =
    status === 'live' ? 'bg-emerald-400' : status === 'reconnecting' ? 'bg-amber-400' : 'bg-slate-500'
  const label = status === 'live' ? '已连接' : status === 'reconnecting' ? '重连中' : '连接中'
  return (
    <span className="flex items-center gap-1.5" title={label}>
      <span className={`relative flex h-2 w-2`}>
        {status !== 'connecting' && (
          <span
            className={`absolute inline-flex h-full w-full animate-ping rounded-full ${color} opacity-60`}
          />
        )}
        <span className={`relative inline-flex h-2 w-2 rounded-full ${color}`} />
      </span>
    </span>
  )
}

function statusColor(status: number): string {
  if (status >= 500) return 'text-rose-400'
  if (status >= 400) return 'text-amber-400'
  if (status >= 300) return 'text-sky-400'
  return 'text-emerald-400'
}

function methodColor(method: string): string {
  switch (method) {
    case 'GET':
      return 'text-cyan-400'
    case 'POST':
      return 'text-emerald-400'
    case 'PATCH':
    case 'PUT':
      return 'text-amber-400'
    case 'DELETE':
      return 'text-rose-400'
    default:
      return 'text-slate-400'
  }
}

function RequestRow({ event, isNew }: { event: RequestEvent; isNew: boolean }) {
  const time = new Date(event.time)
  return (
    <tr
      className={`border-b border-white/[0.03] transition hover:bg-cyan-400/[0.03] ${isNew ? 'animate-[fadeIn_0.4s_ease-out]' : ''}`}
    >
      <td className="whitespace-nowrap px-4 py-1 text-slate-500">
        {time.toLocaleTimeString(undefined, { hour12: false })}
      </td>
      <td className={`whitespace-nowrap px-2 py-1 font-semibold ${methodColor(event.method)}`}>
        {event.method}
      </td>
      <td className="max-w-xs truncate px-2 py-1 text-slate-300" title={event.path}>
        {event.path}
      </td>
      <td className="whitespace-nowrap px-2 py-1 text-slate-500">{event.ip || '—'}</td>
      <td className="whitespace-nowrap px-2 py-1">
        <RegionBadge countryName={event.country_name} />
      </td>
      <td className={`whitespace-nowrap px-2 py-1 text-right font-semibold ${statusColor(event.status)}`}>
        {event.status}
      </td>
      <td className="whitespace-nowrap px-2 py-1 text-right text-slate-500">
        {event.duration_ms.toFixed(1)}ms
      </td>
      <td className="whitespace-nowrap px-4 py-1 text-right text-slate-500">{event.bytes}B</td>
    </tr>
  )
}

function RegionBadge({ countryName }: { countryName?: string }) {
  if (!countryName) {
    return <span className="text-slate-600">—</span>
  }
  const isPrivate = countryName === '内网'
  return <Badge tone={isPrivate ? 'neutral' : 'cyan'}>{countryName}</Badge>
}
