import { useEffect, useState, type FormEvent } from 'react'
import { ErrorBanner } from '../components/ErrorBanner'
import { Modal } from '../components/Modal'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { createInvitation, listInvitations, reinviteMember, revokeInvitation } from '../api/invitations'
import { listDepartments } from '../api/org'
import { ROLE_INFO, can, roleLabel } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import { Field, inputClass } from './MembersPage'
import type { CreatedInvitation, Department, Invitation, InvitationStatus, Role } from '../api/types'

const PAGE_SIZE = 20

const STATUS_LABELS: Record<InvitationStatus, string> = {
  pending: '待接受',
  accepted: '已接受',
  expired: '已过期',
  revoked: '已撤销',
}

export function InvitationsPage() {
  const { user: me } = useAuth()
  const canCreate = can(me?.role, 'invite:create')
  const canRevoke = can(me?.role, 'invite:revoke')

  const [statusFilter, setStatusFilter] = useState<InvitationStatus | ''>('')
  const [page, setPage] = useState(1)
  const [invitations, setInvitations] = useState<Invitation[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [reloadTick, setReloadTick] = useState(0)

  const [creating, setCreating] = useState(false)
  const [revokingInvite, setRevokingInvite] = useState<Invitation | null>(null)
  const [justCreated, setJustCreated] = useState<CreatedInvitation | null>(null)

  useEffect(() => {
    setPage(1)
  }, [statusFilter])

  useEffect(() => {
    setLoading(true)
    setLoadError(null)
    listInvitations({ status: statusFilter || undefined, page, page_size: PAGE_SIZE })
      .then((data) => {
        setInvitations(data.items)
        setTotalPages(Math.max(data.page_info.total_pages, 1))
        setTotal(data.page_info.total)
      })
      .catch(setLoadError)
      .finally(() => setLoading(false))
  }, [statusFilter, page, reloadTick])

  function reload() {
    setReloadTick((t) => t + 1)
  }

  async function handleReinvite(inv: Invitation) {
    try {
      const created = await reinviteMember(inv.id)
      setJustCreated(created)
      reload()
    } catch (err) {
      setLoadError(err)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">
          邀请管理 {total ? <span className="text-sm font-normal text-neutral-500">（共 {total} 个）</span> : null}
        </h1>
        {canCreate && (
          <button
            onClick={() => setCreating(true)}
            className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
          >
            邀请成员
          </button>
        )}
      </div>

      <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value as InvitationStatus | '')} className={inputClass + ' max-w-xs'}>
        <option value="">全部状态</option>
        {Object.entries(STATUS_LABELS).map(([k, label]) => (
          <option key={k} value={k}>
            {label}
          </option>
        ))}
      </select>

      {loadError ? <ErrorBanner error={loadError} /> : null}

      <div className="overflow-hidden rounded-lg border border-neutral-200 dark:border-neutral-800">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[42rem] table-fixed text-left text-sm">
            <colgroup>
              <col className="w-[28%]" />
              <col className="w-[16%]" />
              <col className="w-[14%]" />
              <col className="w-[16%]" />
              <col className="w-[16%]" />
              <col className="w-[10%]" />
            </colgroup>
            <thead className="bg-neutral-50 text-neutral-500 dark:bg-neutral-900 dark:text-neutral-400">
              <tr>
                <th className="px-4 py-2 font-medium">邮箱</th>
                <th className="px-4 py-2 font-medium">角色</th>
                <th className="px-4 py-2 font-medium">状态</th>
                <th className="px-4 py-2 font-medium">邀请时间</th>
                <th className="px-4 py-2 font-medium">过期时间</th>
                <th className="whitespace-nowrap px-4 py-2 font-medium">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
              {loading ? (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-neutral-400">
                    加载中…
                  </td>
                </tr>
              ) : invitations.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-neutral-400">
                    没有邀请记录
                  </td>
                </tr>
              ) : (
                invitations.map((inv) => (
                  <tr key={inv.id}>
                    <td className="truncate px-4 py-2 text-neutral-900 dark:text-neutral-100" title={inv.email}>
                      {inv.email}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-neutral-700 dark:text-neutral-300">{roleLabel(inv.role)}</td>
                    <td className="whitespace-nowrap px-4 py-2">
                      <StatusBadge status={inv.status} />
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-neutral-500 dark:text-neutral-400">
                      {new Date(inv.created_at).toLocaleDateString('zh-CN')}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-neutral-500 dark:text-neutral-400">
                      {new Date(inv.expires_at).toLocaleDateString('zh-CN')}
                    </td>
                    <td className="px-4 py-2">
                      <div className="flex flex-nowrap items-center gap-3 whitespace-nowrap text-xs">
                        {/* Reinvite works against any prior status — the
                            backend always issues a fresh invitation, only
                            revoking the old one first if it was still
                            pending — so this is available whenever the
                            link the recipient already has would no longer
                            work: expired for certain, pending too (to
                            resend after an email that never arrived). */}
                        {canCreate && (inv.status === 'pending' || inv.status === 'expired') && (
                          <button onClick={() => handleReinvite(inv)} className="text-neutral-600 hover:underline dark:text-neutral-300">
                            重新邀请
                          </button>
                        )}
                        {canRevoke && inv.status === 'pending' && (
                          <button onClick={() => setRevokingInvite(inv)} className="text-red-600 hover:underline">
                            撤销
                          </button>
                        )}
                      </div>
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
            onClick={() => setPage((p) => Math.max(p - 1, 1))}
            disabled={page <= 1}
            className="rounded-md border border-neutral-300 px-2 py-1 disabled:opacity-40 dark:border-neutral-700"
          >
            上一页
          </button>
          <span className="text-neutral-500">
            第 {page} / {totalPages} 页
          </span>
          <button
            onClick={() => setPage((p) => Math.min(p + 1, totalPages))}
            disabled={page >= totalPages}
            className="rounded-md border border-neutral-300 px-2 py-1 disabled:opacity-40 dark:border-neutral-700"
          >
            下一页
          </button>
        </div>
      )}

      {creating && (
        <CreateInvitationModal
          onClose={() => setCreating(false)}
          onCreated={(created) => {
            setCreating(false)
            setJustCreated(created)
            setPage(1)
            reload()
          }}
        />
      )}

      {justCreated && <InvitationLinkModal invitation={justCreated} onClose={() => setJustCreated(null)} />}

      {revokingInvite && (
        <RevokeInvitationDialog
          invitation={revokingInvite}
          onCancel={() => setRevokingInvite(null)}
          onRevoked={() => {
            setRevokingInvite(null)
            reload()
          }}
        />
      )}
    </div>
  )
}

function StatusBadge({ status }: { status: InvitationStatus }) {
  const cls =
    status === 'pending'
      ? 'bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300'
      : status === 'accepted'
        ? 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300'
        : 'bg-neutral-200 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400'
  return <span className={`rounded px-1.5 py-0.5 text-xs font-medium ${cls}`}>{STATUS_LABELS[status]}</span>
}

function CreateInvitationModal({
  onClose,
  onCreated,
}: {
  onClose: () => void
  onCreated: (created: CreatedInvitation) => void
}) {
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<Role>('read_only')
  const [departmentId, setDepartmentId] = useState('')
  const [departments, setDepartments] = useState<Department[]>([])
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    listDepartments()
      .then((d) => setDepartments(d.items))
      .catch(() => {
        // Department list is a convenience here; the invite still works
        // without it, just with "未分配" as the only option.
      })
  }, [])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const created = await createInvitation({ email, role, department_id: departmentId || undefined })
      onCreated(created)
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal title="邀请成员" onClose={onClose}>
      <form onSubmit={handleSubmit} className="space-y-3">
        {error ? <ErrorBanner error={error} /> : null}
        <Field label="邮箱">
          <input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} className={inputClass} />
        </Field>
        <Field label="角色">
          <select value={role} onChange={(e) => setRole(e.target.value as Role)} className={inputClass}>
            {ROLE_INFO.map((r) => (
              <option key={r.role} value={r.role}>
                {r.label}
              </option>
            ))}
          </select>
        </Field>
        {departments.length > 0 && (
          <Field label="部门（可选）">
            <select value={departmentId} onChange={(e) => setDepartmentId(e.target.value)} className={inputClass}>
              <option value="">未分配</option>
              {departments.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </select>
          </Field>
        )}
        <div className="flex justify-end gap-2 pt-2">
          <button type="button" onClick={onClose} className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm dark:border-neutral-700">
            取消
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
          >
            {busy ? '发送中…' : '发送邀请'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

function InvitationLinkModal({ invitation, onClose }: { invitation: CreatedInvitation; onClose: () => void }) {
  const acceptUrl = `${window.location.origin}/accept-invite?token=${encodeURIComponent(invitation.token)}`
  const [copied, setCopied] = useState(false)

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(acceptUrl)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard API can be unavailable (insecure context, permission
      // denied); the link is still selectable text in the box below.
    }
  }

  return (
    <Modal title={`已邀请 ${invitation.email}`} onClose={onClose}>
      <div className="space-y-3 text-sm">
        <p className="text-neutral-600 dark:text-neutral-300">
          邀请邮件已交由后台消息队列异步发送。开发环境默认未配置真实邮件服务商，仅会记录到服务器日志——请直接复制下方链接发送给对方，链接仅可使用一次。
        </p>
        <div className="rounded-md border border-neutral-300 bg-neutral-50 px-3 py-2 font-mono text-xs break-all dark:border-neutral-700 dark:bg-neutral-900">
          {acceptUrl}
        </div>
        <div className="flex justify-end gap-2">
          <button onClick={handleCopy} className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm dark:border-neutral-700">
            {copied ? '已复制' : '复制链接'}
          </button>
          <button
            onClick={onClose}
            className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
          >
            完成
          </button>
        </div>
      </div>
    </Modal>
  )
}

function RevokeInvitationDialog({
  invitation,
  onCancel,
  onRevoked,
}: {
  invitation: Invitation
  onCancel: () => void
  onRevoked: () => void
}) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)

  async function handleConfirm() {
    setBusy(true)
    setError(null)
    try {
      await revokeInvitation(invitation.id)
      onRevoked()
    } catch (err) {
      setError(err)
      setBusy(false)
    }
  }

  return (
    <>
      <ConfirmDialog
        title="撤销邀请"
        message={`确定要撤销发给 ${invitation.email} 的邀请吗？该链接将立即失效。`}
        confirmLabel="撤销"
        danger
        busy={busy}
        onConfirm={handleConfirm}
        onCancel={onCancel}
      />
      {error ? (
        <div className="fixed inset-x-0 bottom-4 z-50 mx-auto w-full max-w-md px-4">
          <ErrorBanner error={error} />
        </div>
      ) : null}
    </>
  )
}
