import { ROLE_INFO } from '../auth/permissions'
import { PageHeader, cardClass } from '../components/ui'

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
    <div className="space-y-6">
      <PageHeader
        eyebrow="Roles"
        title="角色权限"
        description="第一阶段提供 5 个固定角色，权限点在后端独立校验；下方仅供查阅，不支持自定义角色或可视化编辑权限规则。"
      />
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {ROLE_INFO.map((r) => (
          <section key={r.role} className={`${cardClass} space-y-3 p-5`}>
            <div>
              <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-100">
                <span className="h-3.5 w-0.5 rounded-full bg-cyan-400 shadow-[0_0_8px_rgba(34,211,238,0.8)]" />
                {r.label}
              </h2>
              <p className="mt-1 text-xs text-slate-400">{r.description}</p>
            </div>
            <ul className="flex flex-wrap gap-1.5">
              {r.permissions.map((p) => (
                <li
                  key={p}
                  className="rounded-md border border-cyan-400/20 bg-cyan-400/[0.06] px-1.5 py-0.5 text-xs text-cyan-200"
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
