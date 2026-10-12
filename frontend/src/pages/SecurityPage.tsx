import { useEffect, useState } from 'react'
import { listSessions, revokeOtherSessions, revokeSession } from '../api/auth'
import { ErrorBanner } from '../components/ErrorBanner'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { Badge, Button, EmptyState, ErrorState, Loading, PageHeader, TBody, THead, Table, Td, Th, Tr } from '../components/ui'
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
      <PageHeader
        eyebrow="Security"
        title="安全中心"
        description={
          <>
            这里列出你账户最近的登录会话。撤销一个会话会让对应设备重新要求登录；已签发但尚未过期的短期访问令牌最多还会保持有效
            1 小时,这是当前设计的已知限制,而不是撤销失败。
          </>
        }
        actions={
          activeCount > 1 && (
            <Button variant="danger" onClick={() => setConfirmingOthers(true)}>
              退出其他所有设备
            </Button>
          )
        }
      />

      {revokeOthersResult !== null && (
        <p className="text-sm text-emerald-300">
          已退出 <span className="font-mono tabular-nums">{revokeOthersResult}</span> 个其他会话。
        </p>
      )}
      {revokeOthersError ? <ErrorBanner error={revokeOthersError} /> : null}
      {loadError ? <ErrorState error={loadError} onRetry={reload} /> : null}

      {loading ? (
        <Loading />
      ) : sessions && sessions.length === 0 ? (
        <EmptyState>没有会话记录。</EmptyState>
      ) : (
        <Table minWidth="42rem">
          <THead>
            <tr>
              <Th className="w-[40%]">设备 / User-Agent</Th>
              <Th>IP</Th>
              <Th>最近活跃</Th>
              <Th>状态</Th>
              <Th>操作</Th>
            </tr>
          </THead>
          <TBody>
            {sessions?.map((s) => (
              <Tr key={s.id}>
                <Td>
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="truncate text-slate-200" title={s.user_agent}>
                      {s.user_agent ?? '未知设备'}
                    </span>
                    {s.current && <Badge tone="cyan">当前会话</Badge>}
                  </div>
                </Td>
                <Td className="font-mono text-xs text-slate-400">{s.ip ?? '—'}</Td>
                <Td className="whitespace-nowrap font-mono text-xs tabular-nums text-slate-400">
                  {new Date(s.created_at).toLocaleString('zh-CN')}
                </Td>
                <Td className="whitespace-nowrap">
                  {s.revoked_at ? (
                    <Badge tone="neutral">已撤销</Badge>
                  ) : new Date(s.expires_at) < new Date() ? (
                    <Badge tone="neutral">已过期</Badge>
                  ) : (
                    <Badge tone="green" dot>
                      有效
                    </Badge>
                  )}
                </Td>
                <Td>
                  {!s.revoked_at && (
                    <Button
                      variant="danger"
                      size="sm"
                      onClick={() => handleRevoke(s.id)}
                      disabled={revokingId === s.id}
                    >
                      {revokingId === s.id ? '撤销中…' : '撤销'}
                    </Button>
                  )}
                </Td>
              </Tr>
            ))}
          </TBody>
        </Table>
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
