import { ROLE_INFO } from '../auth/permissions'

const PERMISSION_LABELS: Record<string, string> = {
  'user:create': '创建成员',
  'user:read': '查看成员',
  'user:list': '列出成员',
  'user:update': '编辑成员',
  'user:delete': '删除成员',
  'messaging:read': '查看消息队列',
  'messaging:replay': '重放死信消息',
  'requests:read': '查看实时请求监控',
  'org:read': '查看组织信息',
  'org:update': '编辑组织信息',
  'department:read': '查看部门',
  'department:write': '编辑部门',
  'invite:create': '发送/重发邀请',
  'invite:read': '查看邀请',
  'invite:revoke': '撤销邀请',
  'role:assign': '分配角色',
  'audit:read': '查看审计日志',
}

export function RolesPage() {
  return (
    <div className="space-y-4">
      <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">角色权限</h1>
      <p className="text-sm text-neutral-500 dark:text-neutral-400">
        第一阶段提供 5 个固定角色，权限点在后端独立校验；下方仅供查阅，不支持自定义角色或可视化编辑权限规则。
      </p>
      <div className="grid gap-4 sm:grid-cols-2">
        {ROLE_INFO.map((r) => (
          <section key={r.role} className="space-y-3 rounded-lg border border-neutral-200 p-4 dark:border-neutral-800">
            <div>
              <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">{r.label}</h2>
              <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-400">{r.description}</p>
            </div>
            <ul className="flex flex-wrap gap-1.5">
              {r.permissions.map((p) => (
                <li
                  key={p}
                  className="rounded bg-neutral-100 px-1.5 py-0.5 text-xs text-neutral-600 dark:bg-neutral-800 dark:text-neutral-300"
                >
                  {PERMISSION_LABELS[p] ?? p}
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </div>
  )
}
