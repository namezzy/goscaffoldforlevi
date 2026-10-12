import { useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { ErrorBanner } from '../components/ErrorBanner'
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
  controlClass,
  inputClass,
  linkClass,
} from '../components/ui'
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
    <div className="space-y-6">
      <PageHeader
        eyebrow="Members"
        title={
          <>
            成员管理{' '}
            {total ? <span className="font-mono text-sm font-normal tabular-nums text-slate-400">（共 {total} 个）</span> : null}
          </>
        }
        actions={
          canCreate && (
            <Button variant="primary" onClick={() => setCreating(true)}>
              新建成员
            </Button>
          )
        }
      />

      <div className="flex flex-wrap items-center gap-2">
        <input
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="搜索邮箱、用户名或姓名"
          className={`${inputClass} max-w-xs`}
        />
        <select
          value={roleFilter}
          onChange={(e) => setRoleFilter(e.target.value as Role | '')}
          className={controlClass}
        >
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
          className={controlClass}
        >
          <option value="">全部状态</option>
          <option value="active">已启用</option>
          <option value="disabled">已禁用</option>
        </select>
      </div>

      {loadError ? <ErrorBanner error={loadError} /> : null}

      <Table minWidth="48rem">
        <colgroup>
          <col className="w-[24%]" />
          <col className="w-[14%]" />
          <col className="w-[16%]" />
          <col className="w-[14%]" />
          <col className="w-[10%]" />
          <col className="w-[10%]" />
          <col className="w-[12%]" />
        </colgroup>
        <THead>
          <tr>
            <SortableHeader label="邮箱" sortKey="email" active={sort} dir={sortDir} onToggle={toggleSort} />
            <Th>用户名</Th>
            <SortableHeader label="姓名" sortKey="name" active={sort} dir={sortDir} onToggle={toggleSort} />
            <Th className="whitespace-nowrap">角色</Th>
            <Th className="whitespace-nowrap">状态</Th>
            <SortableHeader label="加入时间" sortKey="created_at" active={sort} dir={sortDir} onToggle={toggleSort} />
            <Th className="whitespace-nowrap">操作</Th>
          </tr>
        </THead>
        <TBody>
          {loading ? (
            <TableMessage colSpan={7}>加载中…</TableMessage>
          ) : members.length === 0 ? (
            <TableMessage colSpan={7}>
              {search || roleFilter || statusFilter ? '没有匹配的成员' : '没有成员'}
            </TableMessage>
          ) : (
            members.map((u) => (
              <Tr key={u.id}>
                <td className="truncate px-4 py-2.5 text-slate-100" title={u.email}>
                  <Link to={`/members/${u.id}`} className={linkClass}>
                    {u.email}
                  </Link>
                </td>
                <td className="truncate px-4 py-2.5 font-mono text-xs text-slate-400" title={u.username}>
                  {u.username ?? '—'}
                </td>
                <td className="truncate px-4 py-2.5 text-slate-200" title={u.name}>
                  {u.name}
                </td>
                <Td className="whitespace-nowrap">{roleLabel(u.role)}</Td>
                <Td className="whitespace-nowrap">
                  <Badge tone={u.status === 'active' ? 'green' : 'neutral'} dot>
                    {u.status === 'active' ? '已启用' : '已禁用'}
                  </Badge>
                </Td>
                <Td className="whitespace-nowrap font-mono text-xs tabular-nums text-slate-400">
                  {new Date(u.created_at).toLocaleDateString('zh-CN')}
                </Td>
                <Td>
                  <div className="flex flex-nowrap items-center gap-3 whitespace-nowrap text-xs">
                    <Link to={`/members/${u.id}`} className={linkClass}>
                      详情
                    </Link>
                    {canDelete && (
                      <button
                        onClick={() => setDeletingUser(u)}
                        className="text-rose-400 transition hover:text-rose-300 hover:underline"
                      >
                        删除
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
    <Th>
      <button
        onClick={() => onToggle(sortKey)}
        className={`flex items-center gap-1 uppercase tracking-wider transition hover:text-cyan-200 ${
          isActive ? 'text-cyan-300' : ''
        }`}
      >
        {label}
        {isActive ? <span aria-hidden>{dir === 'asc' ? '↑' : '↓'}</span> : null}
      </button>
    </Th>
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
        <p className="text-xs text-slate-400">
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
          <Button onClick={onClose}>取消</Button>
          <Button type="submit" variant="primary" disabled={busy}>
            {busy ? '创建中…' : '创建'}
          </Button>
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

