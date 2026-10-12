import { useEffect, useState } from 'react'
import { getOutboxStatus, listDeadLetters, replayDeadLetter } from '../api/messaging'
import { ErrorBanner } from './ErrorBanner'
import { Button, Card, EmptyState, Loading, StatCard, TBody, THead, Td, Th, Tr } from './ui'
import type { DeadLetter, OutboxStatus } from '../api/types'

const POLL_INTERVAL_MS = 10_000

// MessagingStatus is admin-only (the underlying endpoints require
// messaging:read/messaging:replay, which only the admin role holds) — the
// caller decides whether to render it at all.
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
    <Card title="消息队列（outbox / DLQ）">
      <div className="space-y-5">
        {error ? <ErrorBanner error={error} /> : null}

        {status && (
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <StatCard label="待投递" value={status.pending} tone={status.pending > 0 ? 'amber' : 'cyan'} />
            <StatCard label="已投递" value={status.published} />
            <StatCard label="投递失败中" value={status.failing} tone={status.failing > 0 ? 'amber' : 'cyan'} />
            <StatCard label="最大重试次数" value={status.worst_attempts} tone={status.worst_attempts > 0 ? 'amber' : 'cyan'} />
          </div>
        )}

        <div>
          <h3 className="mb-2 font-mono text-xs text-slate-400">
            死信队列
            {letters &&
              (letters.length < total
                ? `（共 ${total} 条待处理，仅显示前 ${letters.length} 条）`
                : `（共 ${total} 条待处理）`)}
          </h3>
          {letters === null ? (
            <Loading />
          ) : letters.length === 0 ? (
            <EmptyState>没有待处理的死信。</EmptyState>
          ) : (
            <div className="overflow-x-auto rounded-lg border border-white/[0.06]">
              <table className="w-full text-left font-mono text-xs">
                <THead>
                  <tr>
                    <Th>事件类型</Th>
                    <Th>消费者</Th>
                    <Th>重试次数</Th>
                    <Th>失败原因</Th>
                    <Th>失败时间</Th>
                    <Th>操作</Th>
                  </tr>
                </THead>
                <TBody>
                  {letters.map((d) => (
                    <Tr key={d.id}>
                      <Td className="text-slate-100">{d.event_type}</Td>
                      <Td className="text-slate-400">{d.consumer}</Td>
                      <Td className="tabular-nums text-amber-300">{d.attempts}</Td>
                      <td className="max-w-xs truncate px-4 py-2.5 text-slate-400" title={d.last_error}>
                        {d.last_error}
                      </td>
                      <Td className="whitespace-nowrap tabular-nums text-slate-500">{new Date(d.failed_at).toLocaleString()}</Td>
                      <Td>
                        <Button size="sm" onClick={() => handleReplay(d.id)} disabled={replayingID === d.id}>
                          {replayingID === d.id ? '重放中…' : '重放'}
                        </Button>
                      </Td>
                    </Tr>
                  ))}
                </TBody>
              </table>
            </div>
          )}
        </div>
      </div>
    </Card>
  )
}
