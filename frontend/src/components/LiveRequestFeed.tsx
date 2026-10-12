import { useCallback, useEffect, useRef, useState } from 'react'
import { getRecentRequests, streamRequests } from '../api/requests'
import type { RequestEvent } from '../api/types'

const MAX_ITEMS = 300
const RECONNECT_DELAY_MS = 1500
// A request/s rate computed over a window this short is noisy on a quiet
// dashboard; long enough smooths it without making the number feel stale.
const RATE_WINDOW_MS = 10_000

type ConnectionStatus = 'connecting' | 'live' | 'reconnecting'

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function useRequestFeed() {
  const [items, setItems] = useState<RequestEvent[]>([])
  const [status, setStatus] = useState<ConnectionStatus>('connecting')

  // The rate is tracked from its own uncapped list of timestamps, not by
  // counting inside `items`. `items` is capped at MAX_ITEMS for display, so
  // under real traffic (more than MAX_ITEMS arriving inside RATE_WINDOW_MS)
  // counting there silently ceilinged the displayed rate at MAX_ITEMS /
  // (RATE_WINDOW_MS / 1000) — about 30 req/s at these constants — no matter
  // how much traffic was actually arriving.
  const timestampsRef = useRef<number[]>([])
  const [rateCount, setRateCount] = useState(0)

  const pruneAndCountRate = useCallback(() => {
    const cutoff = Date.now() - RATE_WINDOW_MS
    timestampsRef.current = timestampsRef.current.filter((t) => t > cutoff)
    setRateCount(timestampsRef.current.length)
  }, [])

  useEffect(() => {
    let cancelled = false
    const controller = new AbortController()

    async function loop() {
      try {
        const page = await getRecentRequests()
        if (cancelled) return
        setItems(page.items.slice(-MAX_ITEMS).reverse())
        const cutoff = Date.now() - RATE_WINDOW_MS
        timestampsRef.current = page.items
          .map((e) => new Date(e.time).getTime())
          .filter((t) => t > cutoff)
        setRateCount(timestampsRef.current.length)
      } catch {
        // The live stream below still gets a chance to connect.
      }

      while (!cancelled) {
        try {
          await streamRequests((e) => {
            if (cancelled) return
            setStatus('live')
            setItems((prev) => [e, ...prev].slice(0, MAX_ITEMS))
            timestampsRef.current.push(new Date(e.time).getTime())
            pruneAndCountRate()
          }, controller.signal)
        } catch {
          // Aborted (unmount) or a genuine network failure — either way the
          // reconnect loop below is the single place that decides what
          // happens next.
        }
        if (cancelled) return
        setStatus('reconnecting')
        await sleep(RECONNECT_DELAY_MS)
      }
    }

    void loop()
    // The rate must keep decaying even while nothing new arrives — without
    // its own tick, a burst that stopped ten seconds ago would still read
    // as "3.0 req/s" forever, because nothing else re-evaluates the window.
    const rateTimer = setInterval(pruneAndCountRate, 1000)
    return () => {
      cancelled = true
      controller.abort()
      clearInterval(rateTimer)
    }
  }, [pruneAndCountRate])

  return { items, status, rateCount }
}

export function LiveRequestFeed() {
  const { items, status, rateCount } = useRequestFeed()
  const rate = rateCount / (RATE_WINDOW_MS / 1000)

  const buckets = { '2xx': 0, '3xx': 0, '4xx': 0, '5xx': 0 }
  for (const e of items) {
    const key = `${Math.floor(e.status / 100)}xx` as keyof typeof buckets
    if (key in buckets) buckets[key]++
  }

  return (
    <div className="overflow-hidden rounded-lg border border-cyan-500/20 bg-[#070b14] shadow-[0_0_30px_-10px_rgba(34,211,238,0.25)]">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-cyan-500/10 bg-cyan-500/[0.03] px-4 py-3">
        <div className="flex items-center gap-2">
          <ConnectionDot status={status} />
          <h2 className="font-mono text-sm font-semibold tracking-wide text-cyan-300">
            实时请求流
          </h2>
        </div>
        <div className="flex items-center gap-4 font-mono text-xs text-neutral-400">
          <span title={`最近 ${RATE_WINDOW_MS / 1000} 秒内收到的请求数 ÷ ${RATE_WINDOW_MS / 1000}`}>
            <span className="text-cyan-300">{rate.toFixed(1)}</span> req/s
            <span className="text-neutral-600">（近{RATE_WINDOW_MS / 1000}s）</span>
          </span>
          <span
            className="text-neutral-600"
            title={`下方列表最多展示最近 ${MAX_ITEMS} 条，速率统计不受此限制`}
          >
            展示 {items.length}/{MAX_ITEMS}
          </span>
          <span className="text-emerald-400">2xx {buckets['2xx']}</span>
          <span className="text-sky-400">3xx {buckets['3xx']}</span>
          <span className="text-amber-400">4xx {buckets['4xx']}</span>
          <span className="text-rose-400">5xx {buckets['5xx']}</span>
        </div>
      </div>

      <div className="max-h-96 overflow-y-auto font-mono text-xs">
        {items.length === 0 ? (
          <p className="px-4 py-8 text-center text-neutral-500">
            {status === 'connecting' ? '连接中…' : '等待请求…'}
          </p>
        ) : (
          <table className="w-full border-collapse">
            <thead className="sticky top-0 bg-[#070b14] text-neutral-600">
              <tr className="border-b border-white/5">
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
    </div>
  )
}

function ConnectionDot({ status }: { status: ConnectionStatus }) {
  const color =
    status === 'live' ? 'bg-emerald-400' : status === 'reconnecting' ? 'bg-amber-400' : 'bg-neutral-500'
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
      return 'text-neutral-400'
  }
}

function RequestRow({ event, isNew }: { event: RequestEvent; isNew: boolean }) {
  const time = new Date(event.time)
  return (
    <tr
      className={`border-b border-white/[0.03] hover:bg-white/[0.02] ${isNew ? 'animate-[fadeIn_0.4s_ease-out]' : ''}`}
    >
      <td className="whitespace-nowrap px-4 py-1 text-neutral-500">
        {time.toLocaleTimeString(undefined, { hour12: false })}
      </td>
      <td className={`whitespace-nowrap px-2 py-1 font-semibold ${methodColor(event.method)}`}>
        {event.method}
      </td>
      <td className="max-w-xs truncate px-2 py-1 text-neutral-300" title={event.path}>
        {event.path}
      </td>
      <td className="whitespace-nowrap px-2 py-1 text-neutral-500">{event.ip || '—'}</td>
      <td className="whitespace-nowrap px-2 py-1">
        <RegionBadge countryName={event.country_name} />
      </td>
      <td className={`whitespace-nowrap px-2 py-1 text-right font-semibold ${statusColor(event.status)}`}>
        {event.status}
      </td>
      <td className="whitespace-nowrap px-2 py-1 text-right text-neutral-500">
        {event.duration_ms.toFixed(1)}ms
      </td>
      <td className="whitespace-nowrap px-4 py-1 text-right text-neutral-600">{event.bytes}B</td>
    </tr>
  )
}

function RegionBadge({ countryName }: { countryName?: string }) {
  if (!countryName) {
    return <span className="text-neutral-700">—</span>
  }
  const isPrivate = countryName === '内网'
  return (
    <span
      className={`rounded px-1.5 py-0.5 text-[11px] font-medium ${
        isPrivate ? 'bg-white/5 text-neutral-400' : 'bg-cyan-500/10 text-cyan-300'
      }`}
    >
      {countryName}
    </span>
  )
}
