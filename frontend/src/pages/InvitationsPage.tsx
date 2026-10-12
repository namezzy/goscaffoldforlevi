import { useEffect, useState, type FormEvent } from 'react'
import { ErrorBanner } from '../components/ErrorBanner'
import { Modal } from '../components/Modal'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { createInvitation, listInvitations, reinviteMember, revokeInvitation } from '../api/invitations'
import { listDepartments } from '../api/org'
import { ROLE_INFO, can, roleLabel } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import {
  Badge,
  Button,
  Field,
  PageHeader,
  Pagination,
  TBody,
  THead,
  Table,
  TableMessage,
  Td,
  Th,
  Tr,
  inputClass,
  linkClass,
  type Tone,
} from '../components/ui'
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
    <div className="space-y-6">
      <PageHeader
        eyebrow="Invitations"
        title={
          <>
            邀请管理{' '}
            {total ? <span className="font-mono text-sm font-normal tabular-nums text-slate-400">（共 {total} 个）</span> : null}
          </>
        }
        actions={
          canCreate && (
            <Button variant="primary" onClick={() => setCreating(true)}>
              邀请成员
            </Button>
          )
        }
      />

      <select
        value={statusFilter}
        onChange={(e) => setStatusFilter(e.target.value as InvitationStatus | '')}
        className={`${inputClass} max-w-xs`}
      >
        <option value="">全部状态</option>
        {Object.entries(STATUS_LABELS).map(([k, label]) => (
          <option key={k} value={k}>
            {label}
          </option>
        ))}
      </select>

      {loadError ? <ErrorBanner error={loadError} /> : null}

      <Table minWidth="42rem">
        <colgroup>
          <col className="w-[28%]" />
          <col className="w-[16%]" />
          <col className="w-[14%]" />
          <col className="w-[16%]" />
          <col className="w-[16%]" />
          <col className="w-[10%]" />
        </colgroup>
        <THead>
          <tr>
            <Th>邮箱</Th>
            <Th>角色</Th>
            <Th>状态</Th>
            <Th>邀请时间</Th>
            <Th>过期时间</Th>
            <Th className="whitespace-nowrap">操作</Th>
          </tr>
        </THead>
        <TBody>
          {loading ? (
            <TableMessage colSpan={6}>加载中…</TableMessage>
          ) : invitations.length === 0 ? (
            <TableMessage colSpan={6}>没有邀请记录</TableMessage>
          ) : (
            invitations.map((inv) => (
              <Tr key={inv.id}>
                <td className="truncate px-4 py-2.5 text-slate-100" title={inv.email}>
                  {inv.email}
                </td>
                <Td className="whitespace-nowrap">{roleLabel(inv.role)}</Td>
                <Td className="whitespace-nowrap">
                  <StatusBadge status={inv.status} />
                </Td>
                <Td className="whitespace-nowrap font-mono text-xs tabular-nums text-slate-400">
                  {new Date(inv.created_at).toLocaleDateString('zh-CN')}
                </Td>
                <Td className="whitespace-nowrap font-mono text-xs tabular-nums text-slate-400">
                  {new Date(inv.expires_at).toLocaleDateString('zh-CN')}
                </Td>
                <Td>
                  <div className="flex flex-nowrap items-center gap-3 whitespace-nowrap text-xs">
                    {/* Reinvite works against any prior status — the
                        backend always issues a fresh invitation, only
                        revoking the old one first if it was still
                        pending — so this is available whenever the
                        link the recipient already has would no longer
                        work: expired for certain, pending too (to
                        resend after an email that never arrived). */}
                    {canCreate && (inv.status === 'pending' || inv.status === 'expired') && (
                      <button onClick={() => handleReinvite(inv)} className={linkClass}>
                        重新邀请
                      </button>
                    )}
                    {canRevoke && inv.status === 'pending' && (
                      <button
                        onClick={() => setRevokingInvite(inv)}
                        className="text-rose-400 transition hover:text-rose-300 hover:underline"
                      >
                        撤销
                      </button>
                    )}
                  </div>
                </Td>
              </Tr>
            ))
          )}
        </TBody>
      </Table>

      <Pagination page={page} totalPages={totalPages} onChange={setPage} />

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
  const tone: Tone = status === 'pending' ? 'blue' : status === 'accepted' ? 'green' : 'neutral'
  return (
    <Badge tone={tone} dot>
      {STATUS_LABELS[status]}
    </Badge>
  )
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
          <Button onClick={onClose}>取消</Button>
          <Button type="submit" variant="primary" disabled={busy}>
            {busy ? '发送中…' : '发送邀请'}
          </Button>
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
        <p className="text-slate-300">
          邀请邮件已交由后台消息队列异步发送。开发环境默认未配置真实邮件服务商，仅会记录到服务器日志——请直接复制下方链接发送给对方，链接仅可使用一次。
        </p>
        <div className="rounded-lg border border-cyan-400/20 bg-slate-950/60 px-3 py-2 font-mono text-xs break-all text-cyan-200">
          {acceptUrl}
        </div>
        <div className="flex justify-end gap-2">
          <Button onClick={handleCopy}>{copied ? '已复制' : '复制链接'}</Button>
          <Button variant="primary" onClick={onClose}>
            完成
          </Button>
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
