import type { ReactNode } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { can, type Permission } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import { Loading } from './ui'

export function ProtectedRoute() {
  const { status } = useAuth()
  const location = useLocation()

  if (status === 'loading') {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Loading />
      </div>
    )
  }

  if (status === 'anonymous') {
    return <Navigate to="/login" replace state={{ from: location }} />
  }

  return <Outlet />
}

// RequirePermission gates a page on a backend permission rather than a
// specific role, so this stays correct as roles are added or a role's
// grants change — see auth/permissions.ts for why this is presentation
// only, never the real authorization.
export function RequirePermission({
  permission,
  children,
}: {
  permission: Permission
  children: ReactNode
}) {
  const { user } = useAuth()
  if (!can(user?.role, permission)) {
    return (
      <div className="rounded-lg border border-amber-400/30 bg-amber-400/10 px-4 py-3 text-sm text-amber-200">
        没有权限访问这个页面。
      </div>
    )
  }
  return <>{children}</>
}
