import type { Role } from '../api/types'

// Mirrors internal/auth/policy.go's rolePermissions map. Keep in sync by
// hand, the same way api/types.ts mirrors the OpenAPI schemas — there is no
// generator wired up for this frontend.
//
// This is presentation only: it decides which entry points a role sees, not
// whether an action succeeds. Every operation is independently authorized by
// the backend regardless of what this file says, so a UI bug here can hide a
// button but can never grant access a permission check would refuse.
export type Permission =
  | 'user:create'
  | 'user:read'
  | 'user:list'
  | 'user:update'
  | 'user:delete'
  | 'messaging:read'
  | 'messaging:replay'
  | 'requests:read'
  | 'org:read'
  | 'org:update'
  | 'department:read'
  | 'department:write'
  | 'invite:create'
  | 'invite:read'
  | 'invite:revoke'
  | 'role:assign'
  | 'audit:read'
  | 'ticket:create'
  | 'ticket:read_all'
  | 'ticket:manage'
  | 'incident:read'
  | 'incident:manage'

const ALL_PERMISSIONS: Permission[] = [
  'user:create',
  'user:read',
  'user:list',
  'user:update',
  'user:delete',
  'messaging:read',
  'messaging:replay',
  'requests:read',
  'org:read',
  'org:update',
  'department:read',
  'department:write',
  'invite:create',
  'invite:read',
  'invite:revoke',
  'role:assign',
  'audit:read',
  'ticket:create',
  'ticket:read_all',
  'ticket:manage',
  'incident:read',
  'incident:manage',
]

const rolePermissions: Record<Role, Permission[]> = {
  system_admin: ALL_PERMISSIONS,
  member_admin: [
    'user:create',
    'user:read',
    'user:list',
    'user:update',
    'org:read',
    'department:read',
    'department:write',
    'invite:create',
    'invite:read',
    'invite:revoke',
    'role:assign',
    'ticket:create',
    'ticket:read_all',
    'ticket:manage',
  ],
  operator: ['messaging:read', 'messaging:replay', 'requests:read', 'ticket:create', 'incident:read', 'incident:manage'],
  read_only: ['user:read', 'user:list', 'org:read', 'department:read', 'ticket:create'],
  auditor: ['user:read', 'user:list', 'org:read', 'audit:read', 'ticket:create', 'ticket:read_all', 'incident:read'],
}

export function can(role: Role | undefined, permission: Permission): boolean {
  if (!role) return false
  return rolePermissions[role].includes(permission)
}

export interface RoleInfo {
  role: Role
  label: string
  description: string
  permissions: Permission[]
}

// Display copy for the roles page ("查看角色权限说明") — Chinese labels and
// the plain-language description of what each role is for, not just the
// permission list.
export const ROLE_INFO: RoleInfo[] = [
  {
    role: 'system_admin',
    label: '系统管理员',
    description: '拥有全部权限，包括组织、成员、角色分配、运维数据、审计日志、全部工单的分派与管理，以及异常中心的处理。',
    permissions: rolePermissions.system_admin,
  },
  {
    role: 'member_admin',
    label: '成员管理员',
    description: '管理组织成员、部门、邀请和角色分配，并分派与管理全部工单；无法查看运维数据或审计日志，也不能删除账户。',
    permissions: rolePermissions.member_admin,
  },
  {
    role: 'operator',
    label: '运维人员',
    description: '查看消息队列、死信队列和实时请求监控，处理异常中心的异常；可以提交工单并处理分配给自己的工单，不涉及成员管理。',
    permissions: rolePermissions.operator,
  },
  {
    role: 'read_only',
    label: '只读观察者',
    description: '可以查看成员目录与组织架构、提交工单，没有其他写权限。',
    permissions: rolePermissions.read_only,
  },
  {
    role: 'auditor',
    label: '审计人员',
    description: '查看审计日志与成员目录，可查看全部工单与异常中心（不可分派或处理），用于追溯和合规核查。',
    permissions: rolePermissions.auditor,
  },
]

export function roleLabel(role: Role): string {
  return ROLE_INFO.find((r) => r.role === role)?.label ?? role
}
