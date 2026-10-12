import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { listAuditEvents } from '../api/audit'
import { listDepartments } from '../api/org'
import { changePassword, getUser, updateUser } from '../api/users'
import { ErrorBanner } from '../components/ErrorBanner'
import { can, roleLabel, ROLE_INFO } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import { Field, inputClass } from './MembersPage'
import type { AuditEvent, Department, Role, User, UserStatus } from '../api/types'

export function MemberDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { user: me } = useAuth()

  const [member, setMember] = useState<User | null>(null)
  const [departments, setDepartments] = useState<Department[]>([])
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [loadError, setLoadError] = useState<unknown>(null)
  const [loading, setLoading] = useState(true)
  const [reloadTick, setReloadTick] = useState(0)

  const isSelf = me?.id === id
  const canUpdate = isSelf || can(me?.role, 'user:update')
  const canAssignRole = can(me?.role, 'role:assign')
  const canReadAudit = can(me?.role, 'audit:read')

  useEffect(() => {
    if (!id) return
    setLoading(true)
    setLoadError(null)
    Promise.all([
      getUser(id),
      can(me?.role, 'department:read') ? listDepartments() : Promise.resolve({ items: [] }),
      canReadAudit
        ? listAuditEvents({ target_type: 'user', target_id: id, page_size: 20 })
        : Promise.resolve({ items: [], page_info: { page: 1, page_size: 0, total: 0, total_pages: 1 } }),
    ])
      .then(([u, deps, audit]) => {
        setMember(u)
        setDepartments(deps.items)
        setEvents(audit.items)
      })
      .catch(setLoadError)
      .finally(() => setLoading(false))
    // me?.role only changes on login/logout, not worth re-fetching for.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, reloadTick])

  function reload() {
    setReloadTick((t) => t + 1)
  }

  if (loading) return <p className="text-neutral-400">加载中…</p>
  if (loadError) return <ErrorBanner error={loadError} />
  if (!member) return null

  const departmentName = departments.find((d) => d.id === member.department_id)?.name

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm">
        <Link to="/members" className="text-neutral-500 hover:underline dark:text-neutral-400">
          成员管理
        </Link>
        <span className="text-neutral-400">/</span>
        <span className="text-neutral-900 dark:text-neutral-100">{member.email}</span>
      </div>

      <div className="grid gap-6 sm:grid-cols-2">
        <section className="space-y-4 rounded-lg border border-neutral-200 p-4 dark:border-neutral-800">
          <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">基本信息</h2>
          {canUpdate ? (
            <EditMemberForm
              member={member}
              departments={departments}
              canAssignRole={canAssignRole}
              canEditDepartment={can(me?.role, 'user:update')}
              onSaved={(u) => {
                setMember(u)
                reload()
              }}
            />
          ) : (
            <ReadOnlyInfo member={member} departmentName={departmentName} />
          )}
        </section>

        <section className="space-y-2 rounded-lg border border-neutral-200 p-4 dark:border-neutral-800">
          <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">组织归属与安全</h2>
          <dl className="space-y-2 text-sm">
            <Row label="部门" value={departmentName ?? '未分配'} />
            <Row label="角色" value={roleLabel(member.role)} />
            <Row
              label="状态"
              value={member.status === 'active' ? '已启用' : '已禁用'}
              valueClassName={member.status === 'active' ? 'text-green-700 dark:text-green-400' : 'text-neutral-500'}
            />
            <Row label="加入时间" value={new Date(member.created_at).toLocaleString('zh-CN')} />
          </dl>
          {canUpdate && <ChangePasswordSection memberId={member.id} />}
        </section>
      </div>

      {canReadAudit && (
        <section className="space-y-2">
          <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">相关操作记录</h2>
          {events.length === 0 ? (
            <p className="text-sm text-neutral-400">暂无相关审计记录。</p>
          ) : (
            <div className="overflow-hidden rounded-lg border border-neutral-200 dark:border-neutral-800">
              <table className="w-full text-left text-sm">
                <thead className="bg-neutral-50 text-neutral-500 dark:bg-neutral-900 dark:text-neutral-400">
                  <tr>
                    <th className="px-4 py-2 font-medium">时间</th>
                    <th className="px-4 py-2 font-medium">动作</th>
                    <th className="px-4 py-2 font-medium">结果</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
                  {events.map((e) => (
                    <tr key={e.id}>
                      <td className="whitespace-nowrap px-4 py-2 text-neutral-500 dark:text-neutral-400">
                        {new Date(e.created_at).toLocaleString('zh-CN')}
                      </td>
                      <td className="px-4 py-2 text-neutral-900 dark:text-neutral-100">{e.action}</td>
                      <td className="px-4 py-2">
                        <span
                          className={
                            e.result === 'success'
                              ? 'text-green-700 dark:text-green-400'
                              : 'text-red-600 dark:text-red-400'
                          }
                        >
                          {e.result === 'success' ? '成功' : '失败'}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}

      {!canUpdate && !canReadAudit && (
        <button onClick={() => navigate(-1)} className="text-sm text-neutral-500 hover:underline dark:text-neutral-400">
          返回
        </button>
      )}
    </div>
  )
}

function Row({ label, value, valueClassName }: { label: string; value: string; valueClassName?: string }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-neutral-500 dark:text-neutral-400">{label}</dt>
      <dd className={`text-right text-neutral-900 dark:text-neutral-100 ${valueClassName ?? ''}`}>{value}</dd>
    </div>
  )
}

function ReadOnlyInfo({ member, departmentName }: { member: User; departmentName?: string }) {
  return (
    <dl className="space-y-2 text-sm">
      <Row label="邮箱" value={member.email} />
      <Row label="用户名" value={member.username ?? '—'} />
      <Row label="姓名" value={member.name} />
      <Row label="部门" value={departmentName ?? '未分配'} />
    </dl>
  )
}

function EditMemberForm({
  member,
  departments,
  canAssignRole,
  canEditDepartment,
  onSaved,
}: {
  member: User
  departments: Department[]
  canAssignRole: boolean
  canEditDepartment: boolean
  onSaved: (u: User) => void
}) {
  const [name, setName] = useState(member.name)
  const [email, setEmail] = useState(member.email)
  const [role, setRole] = useState<Role>(member.role)
  const [status, setStatus] = useState<UserStatus>(member.status)
  const [departmentId, setDepartmentId] = useState(member.department_id ?? '')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const [saved, setSaved] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    setSaved(false)
    try {
      const patch: Parameters<typeof updateUser>[1] = { name }
      if (email !== member.email) patch.email = email
      if (canAssignRole && role !== member.role) patch.role = role
      if (status !== member.status) patch.status = status
      // Three states, matching the backend's Patch.DepartmentID/
      // ClearDepartment: unchanged (the select still shows what it loaded
      // with), assigned to a specific department, or explicitly cleared —
      // choosing "未分配" for a member who currently has one. Gating this
      // on `departmentId` being truthy, as an earlier version did, silently
      // dropped that last case: an empty string is falsy, so selecting
      // "未分配" sent no field at all, and the save still reported success.
      if (canEditDepartment && departmentId !== (member.department_id ?? '')) {
        if (departmentId === '') {
          patch.clear_department = true
        } else {
          patch.department_id = departmentId
        }
      }
      const updated = await updateUser(member.id, patch)
      setSaved(true)
      onSaved(updated)
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      {error ? <ErrorBanner error={error} /> : null}
      {saved && !error ? <p className="text-xs text-green-600 dark:text-green-400">已保存。</p> : null}
      <Field label="邮箱">
        <input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} className={inputClass} />
      </Field>
      <Field label="姓名">
        <input required minLength={1} maxLength={100} value={name} onChange={(e) => setName(e.target.value)} className={inputClass} />
      </Field>
      {canEditDepartment && (
        <Field label="部门">
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
      {canAssignRole && (
        <Field label="角色">
          <select value={role} onChange={(e) => setRole(e.target.value as Role)} className={inputClass}>
            {ROLE_INFO.map((r) => (
              <option key={r.role} value={r.role}>
                {r.label}
              </option>
            ))}
          </select>
        </Field>
      )}
      {canAssignRole && (
        <Field label="状态">
          <select value={status} onChange={(e) => setStatus(e.target.value as UserStatus)} className={inputClass}>
            <option value="active">已启用</option>
            <option value="disabled">已禁用（将立即撤销其所有会话）</option>
          </select>
        </Field>
      )}
      <div className="flex justify-end pt-2">
        <button
          type="submit"
          disabled={busy}
          className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
        >
          {busy ? '保存中…' : '保存'}
        </button>
      </div>
    </form>
  )
}

function ChangePasswordSection({ memberId }: { memberId: string }) {
  const [open, setOpen] = useState(false)
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [done, setDone] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await changePassword(memberId, { password })
      setDone(true)
      setPassword('')
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  if (!open) {
    return (
      <button onClick={() => setOpen(true)} className="text-sm text-neutral-600 hover:underline dark:text-neutral-300">
        修改密码
      </button>
    )
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-2 border-t border-neutral-200 pt-3 dark:border-neutral-800">
      {error ? <ErrorBanner error={error} /> : null}
      {done ? <p className="text-xs text-green-600 dark:text-green-400">密码已修改。</p> : null}
      <Field label="新密码（12-128 位）">
        <input
          type="password"
          required
          minLength={12}
          maxLength={128}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className={inputClass}
        />
      </Field>
      <div className="flex justify-end gap-2">
        <button type="button" onClick={() => setOpen(false)} className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm dark:border-neutral-700">
          取消
        </button>
        <button
          type="submit"
          disabled={busy}
          className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
        >
          {busy ? '提交中…' : '修改密码'}
        </button>
      </div>
    </form>
  )
}
