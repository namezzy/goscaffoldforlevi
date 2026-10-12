import { useEffect, useState } from 'react'
import { listSessions, revokeOtherSessions, revokeSession } from '../api/auth'
import { ErrorBanner } from '../components/ErrorBanner'
import { ConfirmDialog } from '../components/ConfirmDialog'
import type { Session } from '../api/types'

/** The security center: the caller's own active sessions and login history,
 * with self-service revocation. Every request here is self-scoped by the
 * backend (no admin variant exists — see api/openapi.yaml's `auth` tag),
 * so there is nothing to gate this page on beyond being signed in. */
export function SecurityPage() {
  const [sessions, setSessions] = useState<Session[] | null>(null)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [loading, setLoading] = useState(true)
  const [reloadTick, setReloadTick] = useState(0)

  const [revokingId, setRevokingId] = useState<string | null>(null)
  const [revokeError, setRevokeError] = useState<unknown>(null)
  const [revokingOthers, setRevokingOthers] = useState(false)
  const [revokeOthersError, setRevokeOthersError] = useState<unknown>(null)
  const [revokeOthersResult, setRevokeOthersResult] = useState<number | null>(null)
  const [confirmingOthers, setConfirmingOthers] = useState(false)

  useEffect(() => {
    setLoading(true)
    setLoadError(null)
    listSessions()
      .then((data) => setSessions(data.items))
      .catch(setLoadError)
      .finally(() => setLoading(false))
  }, [reloadTick])

  function reload() {
    setReloadTick((t) => t + 1)
  }

  async function handleRevoke(id: string) {
    setRevokingId(id)
    setRevokeError(null)
    try {
      await revokeSession(id)
      reload()
    } catch (err) {
      setRevokeError(err)
    } finally {
      setRevokingId(null)
    }
  }

  async function handleRevokeOthers() {
    setRevokingOthers(true)
    setRevokeOthersError(null)
    try {
      const result = await revokeOtherSessions()
      setRevokeOthersResult(result.sessions_revoked)
      setConfirmingOthers(false)
      reload()
    } catch (err) {
      setRevokeOthersError(err)
    } finally {
      setRevokingOthers(false)
    }
  }

  const activeCount = sessions?.filter((s) => !s.revoked_at).length ?? 0

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">安全中心</h1>
        {activeCount > 1 && (
          <button
            onClick={() => setConfirmingOthers(true)}
            className="rounded-md border border-red-300 px-3 py-1.5 text-sm font-medium text-red-700 hover:bg-red-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-950"
          >
            退出其他所有设备
          </button>
        )}
      </div>
      <p className="text-sm text-neutral-500 dark:text-neutral-400">
        这里列出你账户最近的登录会话。撤销一个会话会让对应设备重新要求登录；已签发但尚未过期的短期访问令牌最多还会保持有效
        1 小时,这是当前设计的已知限制,而不是撤销失败。
      </p>

      {revokeOthersResult !== null && (
        <p className="text-sm text-green-600 dark:text-green-400">
          已退出 {revokeOthersResult} 个其他会话。
        </p>
      )}
      {revokeOthersError ? <ErrorBanner error={revokeOthersError} /> : null}
      {loadError ? (
        <div className="space-y-2">
          <ErrorBanner error={loadError} />
          <button
            onClick={reload}
            className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
          >
            重试
          </button>
        </div>
      ) : null}

      {loading ? (
        <p className="text-neutral-400">加载中…</p>
      ) : sessions && sessions.length === 0 ? (
        <p className="text-sm text-neutral-400">没有会话记录。</p>
      ) : (
        <div className="overflow-hidden rounded-lg border border-neutral-200 dark:border-neutral-800">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[42rem] text-left text-sm">
              <thead className="bg-neutral-50 text-neutral-500 dark:bg-neutral-900 dark:text-neutral-400">
                <tr>
                  <th className="px-4 py-2 font-medium">设备 / User-Agent</th>
                  <th className="px-4 py-2 font-medium">IP</th>
                  <th className="px-4 py-2 font-medium">最近活跃</th>
                  <th className="px-4 py-2 font-medium">状态</th>
                  <th className="px-4 py-2 font-medium">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
                {sessions?.map((s) => (
                  <tr key={s.id}>
                    <td className="max-w-[18rem] truncate px-4 py-2 text-neutral-700 dark:text-neutral-300" title={s.user_agent}>
                      {s.user_agent ?? '未知设备'}
                      {s.current && (
                        <span className="ml-2 rounded bg-neutral-900 px-1.5 py-0.5 text-xs font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">
                          当前会话
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-2 text-neutral-500 dark:text-neutral-400">{s.ip ?? '—'}</td>
                    <td className="whitespace-nowrap px-4 py-2 text-neutral-500 dark:text-neutral-400">
                      {new Date(s.created_at).toLocaleString('zh-CN')}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2">
                      {s.revoked_at ? (
                        <span className="rounded bg-neutral-200 px-1.5 py-0.5 text-xs font-medium text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400">
                          已撤销
                        </span>
                      ) : new Date(s.expires_at) < new Date() ? (
                        <span className="rounded bg-neutral-200 px-1.5 py-0.5 text-xs font-medium text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400">
                          已过期
                        </span>
                      ) : (
                        <span className="rounded bg-green-100 px-1.5 py-0.5 text-xs font-medium text-green-800 dark:bg-green-950 dark:text-green-300">
                          有效
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-2">
                      {!s.revoked_at && (
                        <button
                          onClick={() => handleRevoke(s.id)}
                          disabled={revokingId === s.id}
                          className="text-xs text-red-600 hover:underline disabled:opacity-50"
                        >
                          {revokingId === s.id ? '撤销中…' : '撤销'}
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
      {revokeError ? <ErrorBanner error={revokeError} /> : null}

      {confirmingOthers && (
        <ConfirmDialog
          title="退出其他所有设备"
          message="将撤销除当前会话外的全部会话，对应设备需要重新登录。此操作不可撤销。"
          confirmLabel="退出其他设备"
          danger
          busy={revokingOthers}
          onConfirm={handleRevokeOthers}
          onCancel={() => setConfirmingOthers(false)}
        />
      )}
    </div>
  )
}
