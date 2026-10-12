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
import { Badge, Button, Card, DescriptionRow, EmptyState, Field, Loading, inputClass } from '../components/ui'
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
      <ul className={depth > 0 ? 'ml-5 space-y-1 border-l border-cyan-400/15 pl-3' : 'space-y-1'}>
        {children.map((d) => (
          <li key={d.id}>
            <div className="flex items-center justify-between rounded-md px-2 py-1.5 transition hover:bg-cyan-400/[0.04]">
              <span className="flex items-center gap-2 text-sm text-slate-100">
                <span className="h-1.5 w-1.5 rounded-full bg-cyan-400/70" />
                {d.name}
              </span>
              {canWriteDept && (
                <div className="flex items-center gap-1">
                  <Button size="sm" variant="ghost" onClick={() => setAddingParent(d.id)}>
                    添加子部门
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => setEditingDept(d)}>
                    重命名
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setDeletingDept(d)}
                    className="text-rose-400 hover:bg-rose-500/10 hover:text-rose-300"
                  >
                    删除
                  </Button>
                </div>
              )}
            </div>
            {renderTree(d.id, depth + 1)}
          </li>
        ))}
      </ul>
    )
  }

  if (loading) return <Loading />
  if (loadError) return <ErrorBanner error={loadError} />

  return (
    <div className="space-y-6">
      <Card title="企业信息">{org && <OrgNameForm org={org} canUpdate={canUpdateOrg} onSaved={setOrg} />}</Card>

      <Card title="通知渠道">
        <div className="space-y-3">
          <p className="text-xs text-slate-400">由部署环境决定是否启用，此处仅展示当前状态，不支持在线修改。</p>
          {channelsError ? (
            <ErrorBanner error={channelsError} />
          ) : channels.length === 0 ? (
            <Loading />
          ) : (
            <ul className="space-y-2">
              {channels.map((c) => (
                <li
                  key={c.name}
                  className="flex items-start justify-between gap-4 rounded-lg border border-white/[0.06] bg-slate-950/40 px-3 py-2.5"
                >
                  <div>
                    <p className="font-mono text-sm font-medium text-slate-100">{c.name}</p>
                    <p className="text-xs text-slate-400">{c.detail}</p>
                  </div>
                  <Badge tone={c.configured ? 'green' : 'neutral'} dot>
                    {c.configured ? '已配置' : '未配置'}
                  </Badge>
                </li>
              ))}
            </ul>
          )}
        </div>
      </Card>

      <Card
        title="部门架构"
        actions={
          canWriteDept && (
            <Button size="sm" variant="primary" onClick={() => setAddingParent(null)}>
              新建顶级部门
            </Button>
          )
        }
      >
        {!canReadDept ? (
          <EmptyState>没有权限查看部门架构。</EmptyState>
        ) : deptError ? (
          <ErrorBanner error={deptError} />
        ) : departments.length === 0 ? (
          <EmptyState>还没有部门。</EmptyState>
        ) : (
          renderTree('', 0)
        )}
      </Card>

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
      <dl>
        <DescriptionRow label="企业名称" value={org.name} />
        <DescriptionRow label="企业标识" value={org.identifier || '未设置'} mono={!!org.identifier} />
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
      <Button type="submit" variant="primary" disabled={busy}>
        {busy ? '保存中…' : '保存'}
      </Button>
      {saved && !error ? <span className="pb-2 text-xs text-emerald-300">已保存</span> : null}
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
          <Button onClick={onCancel}>取消</Button>
          <Button type="submit" variant="primary" disabled={busy}>
            {busy ? '保存中…' : '保存'}
          </Button>
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
