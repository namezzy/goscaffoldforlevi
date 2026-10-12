import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ErrorBanner } from '../components/ErrorBanner'
import { Modal } from '../components/Modal'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { createUser, deleteUser, listUsers } from '../api/users'
import { ROLE_INFO, can, roleLabel } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import type { Role, User, UserStatus } from '../api/types'

const PAGE_SIZE = 20

type SortKey = 'created_at' | 'name' | 'email'

export function MembersPage() {
  const { user: me } = useAuth()
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [searchInput, setSearchInput] = useState('')
  const [roleFilter, setRoleFilter] = useState<Role | ''>('')
  const [statusFilter, setStatusFilter] = useState<UserStatus | ''>('')
  const [sort, setSort] = useState<SortKey>('created_at')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')

  const [members, setMembers] = useState<User[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [reloadTick, setReloadTick] = useState(0)

  const [creating, setCreating] = useState(false)
  const [deletingUser, setDeletingUser] = useState<User | null>(null)

  // The search box debounces locally; role/status/sort take effect
  // immediately since they are discrete choices, not something a user
  // types into character by character.
  useEffect(() => {
    const t = setTimeout(() => setSearch(searchInput), 300)
    return () => clearTimeout(t)
  }, [searchInput])

  useEffect(() => {
    setPage(1)
  }, [search, roleFilter, statusFilter, sort, sortDir])

  useEffect(() => {
    setLoading(true)
    setLoadError(null)
    listUsers({
      page,
      page_size: PAGE_SIZE,
      search: search || undefined,
      role: roleFilter || undefined,
      status: statusFilter || undefined,
      sort,
      sort_dir: sortDir,
    })
      .then((data) => {
        setMembers(data.items)
        setTotalPages(Math.max(data.page_info.total_pages, 1))
        setTotal(data.page_info.total)
      })
      .catch(setLoadError)
      .finally(() => setLoading(false))
  }, [page, search, roleFilter, statusFilter, sort, sortDir, reloadTick])

  function reload() {
    setReloadTick((t) => t + 1)
  }

  function toggleSort(key: SortKey) {
    if (sort === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSort(key)
      setSortDir('asc')
    }
  }

  const canCreate = can(me?.role, 'user:create')
  const canDelete = can(me?.role, 'user:delete')

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">
          成员管理 {total ? <span className="text-sm font-normal text-neutral-500">（共 {total} 个）</span> : null}
        </h1>
        {canCreate && (
          <button
            onClick={() => setCreating(true)}
            className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
          >
            新建成员
          </button>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <input
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="搜索邮箱、用户名或姓名"
          className={`${inputClass} max-w-xs`}
        />
        <select value={roleFilter} onChange={(e) => setRoleFilter(e.target.value as Role | '')} className={inputClass}>
          <option value="">全部角色</option>
          {ROLE_INFO.map((r) => (
            <option key={r.role} value={r.role}>
              {r.label}
            </option>
          ))}
        </select>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value as UserStatus | '')}
          className={inputClass}
        >
          <option value="">全部状态</option>
          <option value="active">已启用</option>
          <option value="disabled">已禁用</option>
        </select>
      </div>

      {loadError ? <ErrorBanner error={loadError} /> : null}

      <div className="overflow-hidden rounded-lg border border-neutral-200 dark:border-neutral-800">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[48rem] table-fixed text-left text-sm">
            <colgroup>
              <col className="w-[24%]" />
              <col className="w-[14%]" />
              <col className="w-[16%]" />
              <col className="w-[14%]" />
              <col className="w-[10%]" />
              <col className="w-[10%]" />
              <col className="w-[12%]" />
            </colgroup>
            <thead className="bg-neutral-50 text-neutral-500 dark:bg-neutral-900 dark:text-neutral-400">
              <tr>
                <SortableHeader label="邮箱" sortKey="email" active={sort} dir={sortDir} onToggle={toggleSort} />
                <th className="px-4 py-2 font-medium">用户名</th>
                <SortableHeader label="姓名" sortKey="name" active={sort} dir={sortDir} onToggle={toggleSort} />
                <th className="whitespace-nowrap px-4 py-2 font-medium">角色</th>
                <th className="whitespace-nowrap px-4 py-2 font-medium">状态</th>
                <SortableHeader label="加入时间" sortKey="created_at" active={sort} dir={sortDir} onToggle={toggleSort} />
                <th className="whitespace-nowrap px-4 py-2 font-medium">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
              {loading ? (
                <tr>
                  <td colSpan={7} className="px-4 py-6 text-center text-neutral-400">
                    加载中…
                  </td>
                </tr>
              ) : members.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-6 text-center text-neutral-400">
                    {search || roleFilter || statusFilter ? '没有匹配的成员' : '没有成员'}
                  </td>
                </tr>
              ) : (
                members.map((u) => (
                  <tr key={u.id}>
                    <td className="truncate px-4 py-2 text-neutral-900 dark:text-neutral-100" title={u.email}>
                      <Link to={`/members/${u.id}`} className="hover:underline">
                        {u.email}
                      </Link>
                    </td>
                    <td className="truncate px-4 py-2 text-neutral-500 dark:text-neutral-400" title={u.username}>
                      {u.username ?? '—'}
                    </td>
                    <td className="truncate px-4 py-2 text-neutral-700 dark:text-neutral-300" title={u.name}>
                      {u.name}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-neutral-700 dark:text-neutral-300">
                      {roleLabel(u.role)}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2">
                      <span
                        className={`rounded px-1.5 py-0.5 text-xs font-medium ${
                          u.status === 'active'
                            ? 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300'
                            : 'bg-neutral-200 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400'
                        }`}
                      >
                        {u.status === 'active' ? '已启用' : '已禁用'}
                      </span>
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-neutral-500 dark:text-neutral-400">
                      {new Date(u.created_at).toLocaleDateString('zh-CN')}
                    </td>
                    <td className="px-4 py-2">
                      <div className="flex flex-nowrap items-center gap-3 whitespace-nowrap text-xs">
                        <Link to={`/members/${u.id}`} className="text-neutral-600 hover:underline dark:text-neutral-300">
                          详情
                        </Link>
                        {canDelete && (
                          <button onClick={() => setDeletingUser(u)} className="text-red-600 hover:underline">
                            删除
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
        <CreateMemberModal
          onClose={() => setCreating(false)}
          onCreated={() => {
            setCreating(false)
            setPage(1)
            reload()
          }}
        />
      )}

      {deletingUser && (
        <DeleteMemberDialog
          user={deletingUser}
          onCancel={() => setDeletingUser(null)}
          onDeleted={() => {
            setDeletingUser(null)
            reload()
          }}
        />
      )}
    </div>
  )
}

function SortableHeader({
  label,
  sortKey,
  active,
  dir,
  onToggle,
}: {
  label: string
  sortKey: SortKey
  active: SortKey
  dir: 'asc' | 'desc'
  onToggle: (key: SortKey) => void
}) {
  const isActive = active === sortKey
  return (
    <th className="px-4 py-2 font-medium">
      <button
        onClick={() => onToggle(sortKey)}
        className={`flex items-center gap-1 hover:text-neutral-900 dark:hover:text-neutral-100 ${
          isActive ? 'text-neutral-900 dark:text-neutral-100' : ''
        }`}
      >
        {label}
        {isActive ? <span aria-hidden>{dir === 'asc' ? '↑' : '↓'}</span> : null}
      </button>
    </th>
  )
}

function CreateMemberModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('read_only')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await createUser({ email, username: username || undefined, name, password, role })
      onCreated()
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal title="新建成员" onClose={onClose}>
      <form onSubmit={handleSubmit} className="space-y-3">
        {error ? <ErrorBanner error={error} /> : null}
        <p className="text-xs text-neutral-500 dark:text-neutral-400">
          直接创建账户会立即生效。更常见的做法是在「邀请管理」中发送邀请，让成员自行设置密码。
        </p>
        <Field label="邮箱">
          <input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} className={inputClass} />
        </Field>
        <Field label="用户名（可选，作为邮箱之外的第二种登录方式）">
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="3-32 位，字母开头"
            minLength={3}
            maxLength={32}
            pattern="[a-zA-Z][a-zA-Z0-9_-]{2,31}"
            className={inputClass}
          />
        </Field>
        <Field label="姓名">
          <input required minLength={1} maxLength={100} value={name} onChange={(e) => setName(e.target.value)} className={inputClass} />
        </Field>
        <Field label="密码（12-128 位）">
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
        <Field label="角色">
          <select value={role} onChange={(e) => setRole(e.target.value as Role)} className={inputClass}>
            {ROLE_INFO.map((r) => (
              <option key={r.role} value={r.role}>
                {r.label}
              </option>
            ))}
          </select>
        </Field>
        <div className="flex justify-end gap-2 pt-2">
          <button type="button" onClick={onClose} className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm dark:border-neutral-700">
            取消
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
          >
            {busy ? '创建中…' : '创建'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

function DeleteMemberDialog({ user, onCancel, onDeleted }: { user: User; onCancel: () => void; onDeleted: () => void }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)

  async function handleConfirm() {
    setBusy(true)
    setError(null)
    try {
      await deleteUser(user.id)
      onDeleted()
    } catch (err) {
      setError(err)
      setBusy(false)
    }
  }

  return (
    <>
      <ConfirmDialog
        title="删除成员"
        message={`确定要删除 ${user.email} 吗？此操作不可撤销。`}
        confirmLabel="删除"
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

export const inputClass =
  'w-full rounded-md border border-neutral-300 px-3 py-2 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100'

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm font-medium text-neutral-700 dark:text-neutral-300">{label}</span>
      {children}
    </label>
  )
}
