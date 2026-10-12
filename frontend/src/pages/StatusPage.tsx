import { useCallback, useEffect, useState } from 'react'
import { getHealth, getReadiness } from '../api/system'
import { ErrorBanner } from '../components/ErrorBanner'
import { LiveRequestFeed } from '../components/LiveRequestFeed'
import { MessagingStatus } from '../components/MessagingStatus'
import { can } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import type { HealthStatus, ReadinessStatus } from '../api/types'

const POLL_INTERVAL_MS = 10_000

// StatusPage is the one screen in this app that commits to a dark "control
// room" look instead of following the light/dark toggle. An operations
// dashboard read at a glance, often on a second monitor, benefits from a
// fixed high-contrast theme more than it benefits from matching whatever
// else is open — Layout switches its own chrome to match, see there.
export function StatusPage() {
  const { user } = useAuth()
  const [health, setHealth] = useState<HealthStatus | null>(null)
  const [readiness, setReadiness] = useState<ReadinessStatus | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [lastChecked, setLastChecked] = useState<Date | null>(null)
  const [checking, setChecking] = useState(false)

  const poll = useCallback(async () => {
    setChecking(true)
    try {
      const [h, r] = await Promise.all([getHealth(), getReadiness()])
      setHealth(h)
      setReadiness(r)
      setError(null)
      setLastChecked(new Date())
    } catch (err) {
      // health/readiness are deliberately left as they were: this page's
      // job is to say the connection is the thing that broke, not to erase
      // the last known state and make a dead poller look identical to a
      // healthy service that has nothing wrong to report.
      setError(err)
    } finally {
      setChecking(false)
    }
  }, [])

  useEffect(() => {
    void poll()
    const timer = setInterval(() => void poll(), POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [poll])

  const ready = readiness?.status === 'ok'
  // Stale, not just "errored": there is a previous reading on screen that
  // the most recent check could not refresh, so it may no longer be true.
  const stale = error !== null && health !== null

  return (
    <div
      className="min-h-[calc(100vh-57px)] px-6 py-8 text-neutral-200"
      style={{
        backgroundImage:
          'linear-gradient(rgba(34,211,238,0.05) 1px, transparent 1px), linear-gradient(90deg, rgba(34,211,238,0.05) 1px, transparent 1px)',
        backgroundSize: '36px 36px',
      }}
    >
      <div className="mx-auto max-w-6xl space-y-6">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h1 className="font-mono text-lg font-semibold tracking-wide text-cyan-300">监控台</h1>
          <div className="flex items-center gap-3 font-mono text-xs text-neutral-500">
            {lastChecked && (
              <span>
                最后成功检查 {lastChecked.toLocaleTimeString()} · 每 {POLL_INTERVAL_MS / 1000}s 自动刷新
              </span>
            )}
            <button
              onClick={() => void poll()}
              disabled={checking}
              className="rounded border border-cyan-500/30 px-2 py-1 text-cyan-300 hover:bg-cyan-500/10 disabled:opacity-50"
            >
              {checking ? '检查中…' : '手动重试'}
            </button>
          </div>
        </div>

        {stale ? (
          <div className="flex items-center justify-between rounded-lg border border-amber-500/40 bg-amber-500/[0.08] px-4 py-3">
            <p className="font-mono text-sm text-amber-300">
              连接中断，以下数据可能已过期 —— 最后一次成功检查于 {lastChecked?.toLocaleTimeString()}
            </p>
          </div>
        ) : error ? (
          <ErrorBanner error={error} />
        ) : null}

        {health && readiness && (
          <div className={`grid grid-cols-2 gap-3 sm:grid-cols-4 ${stale ? 'opacity-50' : ''}`}>
            <StatCard
              label="服务状态"
              value={ready ? 'READY' : (readiness.status || 'DOWN').toUpperCase()}
              tone={stale ? 'stale' : ready ? 'good' : 'bad'}
            />
            <StatCard
              label="进程"
              value={health.status === 'ok' ? 'ALIVE' : health.status.toUpperCase()}
              tone={stale ? 'stale' : health.status === 'ok' ? 'good' : 'bad'}
            />
            {readiness.components &&
              Object.entries(readiness.components).map(([name, status]) => (
                <StatCard
                  key={name}
                  label={name}
                  value={status.toUpperCase()}
                  tone={stale ? 'stale' : status === 'ok' ? 'good' : 'bad'}
                />
              ))}
          </div>
        )}

        {health && (
          <p className="font-mono text-xs text-neutral-600">
            version <span className="text-neutral-400">{health.version || '—'}</span> · commit{' '}
            <span className="text-neutral-400">{health.commit || '—'}</span>
          </p>
        )}

        {!health && !error && <p className="font-mono text-sm text-neutral-500">检查中…</p>}

        {can(user?.role, 'messaging:read') && <MessagingStatus />}
        {can(user?.role, 'requests:read') && <LiveRequestFeed />}
      </div>
    </div>
  )
}

function StatCard({
  label,
  value,
  tone,
}: {
  label: string
  value: string
  tone: 'good' | 'bad' | 'stale'
}) {
  const styles = {
    good: { border: 'border-emerald-500/20 bg-emerald-500/[0.04]', text: 'text-emerald-400' },
    bad: { border: 'border-rose-500/30 bg-rose-500/[0.06]', text: 'text-rose-400' },
    stale: { border: 'border-neutral-600/30 bg-white/[0.02]', text: 'text-neutral-400' },
  }[tone]

  return (
    <div className={`rounded-lg border px-4 py-3 ${styles.border}`}>
      <p className="font-mono text-[11px] uppercase tracking-wider text-neutral-500">{label}</p>
      <p className={`mt-1 font-mono text-lg font-semibold ${styles.text}`}>{value}</p>
    </div>
  )
}
