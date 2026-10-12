import { useEffect, useState } from 'react'
import { getOutboxStatus, listDeadLetters, replayDeadLetter } from '../api/messaging'
import { ErrorBanner } from './ErrorBanner'
import type { DeadLetter, OutboxStatus } from '../api/types'

const POLL_INTERVAL_MS = 10_000

// MessagingStatus is admin-only (the underlying endpoints require
// messaging:read/messaging:replay, which only the admin role holds) — the
// caller decides whether to render it at all. Styled to match StatusPage's
// fixed dark theme — see the comment there.
export function MessagingStatus() {
  const [status, setStatus] = useState<OutboxStatus | null>(null)
  const [letters, setLetters] = useState<DeadLetter[] | null>(null)
  const [total, setTotal] = useState(0)
  const [error, setError] = useState<unknown>(null)
  const [replayingID, setReplayingID] = useState<string | null>(null)
  const [reloadTick, setReloadTick] = useState(0)

  useEffect(() => {
    let cancelled = false

    async function poll() {
      try {
        const [s, page] = await Promise.all([getOutboxStatus(), listDeadLetters()])
        if (cancelled) return
        setStatus(s)
        setLetters(page.items)
        setTotal(page.total)
        setError(null)
      } catch (err) {
        if (!cancelled) setError(err)
      }
    }

    void poll()
    const timer = setInterval(() => void poll(), POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [reloadTick])

  async function handleReplay(id: string) {
    setReplayingID(id)
    setError(null)
    try {
      await replayDeadLetter(id)
      setReloadTick((t) => t + 1)
    } catch (err) {
      setError(err)
    } finally {
      setReplayingID(null)
    }
  }

  return (
    <div className="space-y-4">
      <h2 className="font-mono text-sm font-semibold tracking-wide text-cyan-300">消息队列（outbox / DLQ）</h2>

      {error ? <ErrorBanner error={error} /> : null}

      {status && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Stat label="待投递" value={status.pending} warn={status.pending > 0} />
          <Stat label="已投递" value={status.published} />
          <Stat label="投递失败中" value={status.failing} warn={status.failing > 0} />
          <Stat label="最大重试次数" value={status.worst_attempts} warn={status.worst_attempts > 0} />
        </div>
      )}

      <div>
        <h3 className="mb-2 font-mono text-xs text-neutral-500">
          死信队列
          {letters &&
            (letters.length < total
              ? `（共 ${total} 条待处理，仅显示前 ${letters.length} 条）`
              : `（共 ${total} 条待处理）`)}
        </h3>
        {letters === null ? (
          <p className="font-mono text-sm text-neutral-500">加载中…</p>
        ) : letters.length === 0 ? (
          <p className="font-mono text-sm text-neutral-600">没有待处理的死信。</p>
        ) : (
          <div className="overflow-hidden rounded-lg border border-cyan-500/20 bg-[#070b14]">
            <table className="w-full text-left font-mono text-xs">
              <thead className="bg-cyan-500/[0.03] text-neutral-500">
                <tr>
                  <th className="px-4 py-2 font-medium">事件类型</th>
                  <th className="px-4 py-2 font-medium">消费者</th>
                  <th className="px-4 py-2 font-medium">重试次数</th>
                  <th className="px-4 py-2 font-medium">失败原因</th>
                  <th className="px-4 py-2 font-medium">失败时间</th>
                  <th className="px-4 py-2 font-medium">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-white/5">
                {letters.map((d) => (
                  <tr key={d.id} className="hover:bg-white/[0.02]">
                    <td className="px-4 py-2 text-neutral-200">{d.event_type}</td>
                    <td className="px-4 py-2 text-neutral-400">{d.consumer}</td>
                    <td className="px-4 py-2 text-amber-400">{d.attempts}</td>
                    <td className="max-w-xs truncate px-4 py-2 text-neutral-400" title={d.last_error}>
                      {d.last_error}
                    </td>
                    <td className="px-4 py-2 text-neutral-500">{new Date(d.failed_at).toLocaleString()}</td>
                    <td className="px-4 py-2">
                      <button
                        onClick={() => handleReplay(d.id)}
                        disabled={replayingID === d.id}
                        className="rounded-md border border-cyan-500/30 px-2 py-1 text-xs font-medium text-cyan-300 hover:bg-cyan-500/10 disabled:opacity-50"
                      >
                        {replayingID === d.id ? '重放中…' : '重放'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}

function Stat({ label, value, warn }: { label: string; value: number; warn?: boolean }) {
  return (
    <div className="rounded-lg border border-cyan-500/20 bg-cyan-500/[0.03] px-4 py-3">
      <p className="font-mono text-[11px] uppercase tracking-wider text-neutral-500">{label}</p>
      <p className={`mt-1 font-mono text-lg font-semibold ${warn ? 'text-amber-400' : 'text-cyan-300'}`}>
        {value}
      </p>
    </div>
  )
}
