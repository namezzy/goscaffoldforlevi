import { useCallback, useEffect, useState } from 'react'
import { getHealth, getReadiness } from '../api/system'
import { ErrorBanner } from '../components/ErrorBanner'
import { LiveRequestFeed } from '../components/LiveRequestFeed'
import { MessagingStatus } from '../components/MessagingStatus'
import { Button, Loading, PageHeader, cardClass, toneClass, toneDotClass } from '../components/ui'
import { can } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import type { HealthStatus, ReadinessStatus } from '../api/types'

const POLL_INTERVAL_MS = 10_000

// An operations dashboard read at a glance, often on a second monitor:
// every reading is a mono figure with a status-coloured accent.
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
    <div className="space-y-6">
      <PageHeader
        eyebrow="System Status"
        title="监控台"
        actions={
          <>
            {lastChecked && (
              <span className="font-mono text-xs tabular-nums text-slate-500">
                最后成功检查 {lastChecked.toLocaleTimeString()} · 每 {POLL_INTERVAL_MS / 1000}s 自动刷新
              </span>
            )}
            <Button size="sm" onClick={() => void poll()} disabled={checking}>
              {checking ? '检查中…' : '手动重试'}
            </Button>
          </>
        }
      />

      {stale ? (
        <div className={`rounded-lg border px-4 py-3 ${toneClass.amber}`}>
          <p className="font-mono text-sm">
            连接中断，以下数据可能已过期 —— 最后一次成功检查于 {lastChecked?.toLocaleTimeString()}
          </p>
        </div>
      ) : error ? (
        <ErrorBanner error={error} />
      ) : null}

      {health && readiness && (
        <div className={`grid grid-cols-2 gap-4 sm:grid-cols-4 ${stale ? 'opacity-50' : ''}`}>
          <HealthCard
            label="服务状态"
            value={ready ? 'READY' : (readiness.status || 'DOWN').toUpperCase()}
            tone={stale ? 'stale' : ready ? 'good' : 'bad'}
          />
          <HealthCard
            label="进程"
            value={health.status === 'ok' ? 'ALIVE' : health.status.toUpperCase()}
            tone={stale ? 'stale' : health.status === 'ok' ? 'good' : 'bad'}
          />
          {readiness.components &&
            Object.entries(readiness.components).map(([name, status]) => (
              <HealthCard
                key={name}
                label={name}
                value={status.toUpperCase()}
                tone={stale ? 'stale' : status === 'ok' ? 'good' : 'bad'}
              />
            ))}
        </div>
      )}

      {health && (
        <p className="font-mono text-xs text-slate-500">
          version <span className="text-slate-300">{health.version || '—'}</span> · commit{' '}
          <span className="text-slate-300">{health.commit || '—'}</span>
        </p>
      )}

      {!health && !error && <Loading label="检查中…" />}

      {can(user?.role, 'messaging:read') && <MessagingStatus />}
      {can(user?.role, 'requests:read') && <LiveRequestFeed />}
    </div>
  )
}

const HEALTH_TONE: Record<'good' | 'bad' | 'stale', { dot: string; text: string; card: string }> = {
  good: { dot: toneDotClass.green, text: 'text-emerald-300', card: '' },
  bad: { dot: toneDotClass.red, text: 'text-rose-300', card: 'border-rose-400/30 bg-rose-500/[0.06]' },
  stale: { dot: toneDotClass.neutral, text: 'text-slate-400', card: '' },
}

function HealthCard({ label, value, tone }: { label: string; value: string; tone: 'good' | 'bad' | 'stale' }) {
  const styles = HEALTH_TONE[tone]
  return (
    <div className={`${cardClass} p-4 ${styles.card}`}>
      <div className="flex items-center justify-between">
        <span className="font-mono text-[11px] uppercase tracking-wider text-slate-400">{label}</span>
        <span className={`h-1.5 w-1.5 rounded-full ${styles.dot} shadow-[0_0_8px_currentColor]`} />
      </div>
      <p className={`mt-2 font-mono text-xl font-semibold tracking-wide ${styles.text}`}>{value}</p>
    </div>
  )
}
