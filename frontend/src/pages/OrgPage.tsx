import { useEffect, useState, type FormEvent } from 'react'
import { ErrorBanner } from '../components/ErrorBanner'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { Modal } from '../components/Modal'
import {
  createDepartment,
  deleteDepartment,
  getNotificationChannels,
  getOrganization,
  listDepartments,
  updateDepartment,
  updateOrganization,
} from '../api/org'
import { can } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import { Field, inputClass } from './MembersPage'
import type { Department, NotificationChannel, Organization } from '../api/types'

export function OrgPage() {
  const { user: me } = useAuth()
  const canReadDept = can(me?.role, 'department:read')
  const canUpdateOrg = can(me?.role, 'org:update')
  const canWriteDept = can(me?.role, 'department:write')

  const [org, setOrg] = useState<Organization | null>(null)
  const [departments, setDepartments] = useState<Department[]>([])
  const [channels, setChannels] = useState<NotificationChannel[]>([])
  const [channelsError, setChannelsError] = useState<unknown>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [deptError, setDeptError] = useState<unknown>(null)
  const [reloadTick, setReloadTick] = useState(0)

  const [addingParent, setAddingParent] = useState<string | null | undefined>(undefined)
  const [editingDept, setEditingDept] = useState<Department | null>(null)
  const [deletingDept, setDeletingDept] = useState<Department | null>(null)

  // The two sections load independently, not through one Promise.all: a
  // role that holds org:read but not department:read (auditor, in the
  // fixed five-role set) must still see the company name — one section's
  // missing permission must not blank the other, which one shared promise
  // and one shared error state would otherwise do.
  useEffect(() => {
    setLoading(true)
    setLoadError(null)
    getOrganization()
      .then(setOrg)
      .catch(setLoadError)
      .finally(() => setLoading(false))
  }, [reloadTick])

  useEffect(() => {
    if (!canReadDept) return
    setDeptError(null)
    listDepartments()
      .then((deps) => setDepartments(deps.items))
      .catch(setDeptError)
    // canReadDept only changes on login/logout, not worth re-running for.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reloadTick])

  // Gated by the route's own org:read requirement (App.tsx), not by a
  // separate check here — anyone who can see this page at all can read the
  // channel status, and the request itself is read-only.
  useEffect(() => {
    setChannelsError(null)
    getNotificationChannels()
      .then(setChannels)
      .catch(setChannelsError)
  }, [reloadTick])

  function reload() {
    setReloadTick((t) => t + 1)
  }

  const byParent = new Map<string, Department[]>()
  for (const d of departments) {
    const key = d.parent_id ?? ''
    byParent.set(key, [...(byParent.get(key) ?? []), d])
  }
  const childCount = (id: string) => (byParent.get(id) ?? []).length

  function renderTree(parentId: string, depth: number) {
    const children = byParent.get(parentId) ?? []
    if (children.length === 0) return null
    return (
      <ul className={depth > 0 ? 'ml-5 space-y-1 border-l border-neutral-200 pl-3 dark:border-neutral-800' : 'space-y-1'}>
        {children.map((d) => (
          <li key={d.id}>
            <div className="flex items-center justify-between rounded-md px-2 py-1.5 hover:bg-neutral-50 dark:hover:bg-neutral-900">
              <span className="text-sm text-neutral-900 dark:text-neutral-100">{d.name}</span>
              {canWriteDept && (
                <div className="flex items-center gap-3 text-xs">
                  <button onClick={() => setAddingParent(d.id)} className="text-neutral-500 hover:underline dark:text-neutral-400">
                    添加子部门
                  </button>
                  <button onClick={() => setEditingDept(d)} className="text-neutral-500 hover:underline dark:text-neutral-400">
                    重命名
                  </button>
                  <button onClick={() => setDeletingDept(d)} className="text-red-600 hover:underline">
                    删除
                  </button>
                </div>
              )}
            </div>
            {renderTree(d.id, depth + 1)}
          </li>
        ))}
      </ul>
    )
  }

  if (loading) return <p className="text-neutral-400">加载中…</p>
  if (loadError) return <ErrorBanner error={loadError} />

  return (
    <div className="space-y-6">
      <section className="space-y-3 rounded-lg border border-neutral-200 p-4 dark:border-neutral-800">
        <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">企业信息</h2>
        {org && <OrgNameForm org={org} canUpdate={canUpdateOrg} onSaved={setOrg} />}
      </section>

      <section className="space-y-3 rounded-lg border border-neutral-200 p-4 dark:border-neutral-800">
        <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">通知渠道</h2>
        <p className="text-xs text-neutral-500 dark:text-neutral-400">
          由部署环境决定是否启用，此处仅展示当前状态，不支持在线修改。
        </p>
        {channelsError ? (
          <ErrorBanner error={channelsError} />
        ) : channels.length === 0 ? (
          <p className="text-sm text-neutral-400">加载中…</p>
        ) : (
          <ul className="space-y-2">
            {channels.map((c) => (
              <li
                key={c.name}
                className="flex items-start justify-between gap-4 rounded-md border border-neutral-200 px-3 py-2 dark:border-neutral-800"
              >
                <div>
                  <p className="text-sm font-medium text-neutral-900 dark:text-neutral-100">{c.name}</p>
                  <p className="text-xs text-neutral-500 dark:text-neutral-400">{c.detail}</p>
                </div>
                <span
                  className={`shrink-0 rounded px-1.5 py-0.5 text-xs font-medium ${
                    c.configured
                      ? 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300'
                      : 'bg-neutral-200 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400'
                  }`}
                >
                  {c.configured ? '已配置' : '未配置'}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">部门架构</h2>
          {canWriteDept && (
            <button
              onClick={() => setAddingParent(null)}
              className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
            >
              新建顶级部门
            </button>
          )}
        </div>
        <div className="rounded-lg border border-neutral-200 p-4 dark:border-neutral-800">
          {!canReadDept ? (
            <p className="text-sm text-neutral-400">没有权限查看部门架构。</p>
          ) : deptError ? (
            <ErrorBanner error={deptError} />
          ) : departments.length === 0 ? (
            <p className="text-sm text-neutral-400">还没有部门。</p>
          ) : (
            renderTree('', 0)
          )}
        </div>
      </section>

      {addingParent !== undefined && (
        <DepartmentModal
          title={addingParent ? '新建子部门' : '新建顶级部门'}
          initialName=""
          onCancel={() => setAddingParent(undefined)}
          onSubmit={async (name) => {
            await createDepartment({ name, parent_id: addingParent ?? undefined })
          }}
          onDone={() => {
            setAddingParent(undefined)
            reload()
          }}
        />
      )}

      {editingDept && (
        <DepartmentModal
          title={`重命名 ${editingDept.name}`}
          initialName={editingDept.name}
          onCancel={() => setEditingDept(null)}
          onSubmit={async (name) => {
            await updateDepartment(editingDept.id, { name })
          }}
          onDone={() => {
            setEditingDept(null)
            reload()
          }}
        />
      )}

      {deletingDept && (
        <DeleteDepartmentDialog
          department={deletingDept}
          hasChildren={childCount(deletingDept.id) > 0}
          onCancel={() => setDeletingDept(null)}
          onDeleted={() => {
            setDeletingDept(null)
            reload()
          }}
        />
      )}
    </div>
  )
}

function OrgNameForm({
  org,
  canUpdate,
  onSaved,
}: {
  org: Organization
  canUpdate: boolean
  onSaved: (o: Organization) => void
}) {
  const [name, setName] = useState(org.name)
  const [identifier, setIdentifier] = useState(org.identifier)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [saved, setSaved] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    setSaved(false)
    try {
      const patch: Parameters<typeof updateOrganization>[0] = {}
      if (name !== org.name) patch.name = name
      if (identifier !== org.identifier) patch.identifier = identifier
      const updated = await updateOrganization(patch)
      onSaved(updated)
      setSaved(true)
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  if (!canUpdate) {
    return (
      <dl className="space-y-2 text-sm">
        <div className="flex justify-between gap-4">
          <dt className="text-neutral-500 dark:text-neutral-400">企业名称</dt>
          <dd className="text-neutral-900 dark:text-neutral-100">{org.name}</dd>
        </div>
        <div className="flex justify-between gap-4">
          <dt className="text-neutral-500 dark:text-neutral-400">企业标识</dt>
          <dd className="text-neutral-900 dark:text-neutral-100">{org.identifier || '未设置'}</dd>
        </div>
      </dl>
    )
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-wrap items-end gap-3">
      {error ? <ErrorBanner error={error} /> : null}
      <div className="max-w-sm flex-1">
        <Field label="企业名称">
          <input required minLength={1} maxLength={200} value={name} onChange={(e) => setName(e.target.value)} className={inputClass} />
        </Field>
      </div>
      <div className="max-w-sm flex-1">
        <Field label="企业标识（可选）">
          <input maxLength={100} value={identifier} onChange={(e) => setIdentifier(e.target.value)} className={inputClass} />
        </Field>
      </div>
      <button
        type="submit"
        disabled={busy}
        className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
      >
        {busy ? '保存中…' : '保存'}
      </button>
      {saved && !error ? <span className="text-xs text-green-600 dark:text-green-400">已保存</span> : null}
    </form>
  )
}

function DepartmentModal({
  title,
  initialName,
  onCancel,
  onSubmit,
  onDone,
}: {
  title: string
  initialName: string
  onCancel: () => void
  onSubmit: (name: string) => Promise<void>
  onDone: () => void
}) {
  const [name, setName] = useState(initialName)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await onSubmit(name)
      onDone()
    } catch (err) {
      setError(err)
      setBusy(false)
    }
  }

  return (
    <Modal title={title} onClose={onCancel}>
      <form onSubmit={handleSubmit} className="space-y-3">
        {error ? <ErrorBanner error={error} /> : null}
        <Field label="部门名称">
          <input required minLength={1} maxLength={100} value={name} onChange={(e) => setName(e.target.value)} className={inputClass} />
        </Field>
        <div className="flex justify-end gap-2 pt-2">
          <button type="button" onClick={onCancel} className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm dark:border-neutral-700">
            取消
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-md bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
          >
            {busy ? '保存中…' : '保存'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

function DeleteDepartmentDialog({
  department,
  hasChildren,
  onCancel,
  onDeleted,
}: {
  department: Department
  hasChildren: boolean
  onCancel: () => void
  onDeleted: () => void
}) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)

  async function handleConfirm() {
    setBusy(true)
    setError(null)
    try {
      await deleteDepartment(department.id)
      onDeleted()
    } catch (err) {
      setError(err)
      setBusy(false)
    }
  }

  return (
    <>
      <ConfirmDialog
        title="删除部门"
        message={
          hasChildren
            ? `${department.name} 下还有子部门，请先移动或删除它们。`
            : `确定要删除 ${department.name} 吗？如果仍有成员归属此部门，删除会被拒绝。`
        }
        confirmLabel="删除"
        danger
        busy={busy}
        disabled={hasChildren}
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
