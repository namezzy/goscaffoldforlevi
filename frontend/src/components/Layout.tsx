import { useEffect, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { getUser } from '../api/users'
import { can, type Permission } from '../auth/permissions'
import { logout } from '../auth/store'
import { useAuth } from '../auth/useAuth'
import { NotificationBell } from './NotificationBell'

interface NavItem {
  to: string
  label: string
  /** Omitted for an item every signed-in account sees, regardless of role. */
  permission?: Permission
}

// One list drives the sidebar, the mobile menu and the page title (from the
// active item's label) — three places that would otherwise drift out of
// sync if each kept its own copy.
const NAV_ITEMS: NavItem[] = [
  { to: '/dashboard', label: '工作台' },
  { to: '/tickets', label: '工单' },
  { to: '/profile', label: '我的资料' },
  { to: '/security', label: '安全中心' },
  { to: '/members', label: '成员管理', permission: 'user:list' },
  { to: '/org', label: '组织架构', permission: 'org:read' },
  { to: '/invitations', label: '邀请管理', permission: 'invite:read' },
  { to: '/roles', label: '角色权限' },
  { to: '/audit', label: '操作审计', permission: 'audit:read' },
  { to: '/incidents', label: '异常中心', permission: 'incident:read' },
  { to: '/status', label: '监控台', permission: 'requests:read' },
]

export function Layout() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  // The monitor page commits to a dark "control room" look rather than
  // following the light/dark toggle — see StatusPage. The chrome switches
  // with it so it and the content do not visibly disagree.
  const monitor = location.pathname === '/status'
  const [email, setEmail] = useState<string | null>(null)
  const [menuOpen, setMenuOpen] = useState(false)

  const visibleItems = NAV_ITEMS.filter((item) => !item.permission || can(user?.role, item.permission))
  const activeItem = visibleItems.find(
    (item) => location.pathname === item.to || location.pathname.startsWith(item.to + '/'),
  )

  useEffect(() => {
    if (!user) return
    let cancelled = false
    getUser(user.id)
      .then((u) => {
        if (!cancelled) setEmail(u.email)
      })
      .catch(() => {
        // Header display only — a failure here is not worth surfacing.
      })
    return () => {
      cancelled = true
    }
  }, [user])

  // A route change is the signal that a nav link was actually followed —
  // closing on click alone would also close it when someone taps whitespace
  // inside the panel, and closing on navigate covers the browser back/
  // forward buttons too, which a click handler never sees.
  useEffect(() => {
    setMenuOpen(false)
  }, [location.pathname])

  async function handleLogout() {
    await logout()
    navigate('/login', { replace: true })
  }

  const sidebarLinkClass = ({ isActive }: { isActive: boolean }) =>
    `block rounded-md px-3 py-2 text-sm font-medium transition-colors ${
      monitor
        ? isActive
          ? 'bg-cyan-500/10 text-cyan-300'
          : 'text-neutral-400 hover:bg-white/5 hover:text-cyan-200'
        : isActive
          ? 'bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900'
          : 'text-neutral-600 hover:bg-neutral-100 dark:text-neutral-300 dark:hover:bg-neutral-800'
    }`

  const accountBadgeClass = monitor
    ? 'rounded bg-cyan-500/10 px-1.5 py-0.5 text-xs font-medium text-cyan-300'
    : 'rounded bg-neutral-900 px-1.5 py-0.5 text-xs font-medium text-white dark:bg-neutral-100 dark:text-neutral-900'

  const logoutButtonClass = monitor
    ? 'rounded-md border border-white/10 px-3 py-1.5 text-sm hover:border-cyan-500/30 hover:bg-white/5'
    : 'rounded-md border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-50 dark:border-neutral-700 dark:hover:bg-neutral-800'

  const sidebarClass = monitor
    ? 'border-cyan-500/10 bg-[#070a12]'
    : 'border-neutral-200 bg-white dark:border-neutral-800 dark:bg-neutral-900'

  return (
    <div className={monitor ? 'min-h-screen bg-[#05070d]' : 'min-h-screen bg-neutral-50 dark:bg-neutral-950'}>
      {/* Desktop sidebar: fixed width, full height, hidden below sm so it
          never competes with the mobile menu for the same space. */}
      <aside className={`hidden sm:fixed sm:inset-y-0 sm:left-0 sm:flex sm:w-56 sm:flex-col sm:border-r ${sidebarClass}`}>
        <div
          className={`px-4 py-4 text-sm font-semibold ${monitor ? 'font-mono text-cyan-300' : 'text-neutral-900 dark:text-neutral-100'}`}
        >
          goscaffoldforlevi
        </div>
        <nav className="flex-1 space-y-1 px-3">
          {visibleItems.map((item) => (
            <NavLink key={item.to} to={item.to} className={sidebarLinkClass}>
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>

      <div className="sm:pl-56">
        <header
          className={monitor ? 'border-b border-cyan-500/10 bg-[#070a12]' : 'border-b border-neutral-200 bg-white dark:border-neutral-800 dark:bg-neutral-900'}
        >
          <div className="flex items-center justify-between px-4 py-3 sm:px-6">
            {/* Page title stands in for the sidebar on mobile, where the
                sidebar itself is hidden — see the sm:hidden title below for
                the mobile-only variant that also carries the app name. */}
            <h1
              className={`hidden text-sm font-semibold sm:block ${monitor ? 'font-mono text-cyan-300' : 'text-neutral-900 dark:text-neutral-100'}`}
            >
              {activeItem?.label ?? ''}
            </h1>

            <div
              className={`hidden items-center gap-3 text-sm sm:flex ${monitor ? 'font-mono text-neutral-400' : 'text-neutral-500 dark:text-neutral-400'}`}
            >
              <span className="max-w-[16rem] truncate" title={email ?? undefined}>
                {email ?? '…'}
                {user?.role && <span className={`ml-2 ${accountBadgeClass}`}>{user.role}</span>}
              </span>
              <NotificationBell monitor={monitor} />
              <button onClick={handleLogout} className={logoutButtonClass}>
                退出登录
              </button>
            </div>

            {/* Mobile: a title stands in for the sidebar, and everything
                else — nav, account, logout — collapses behind one button. */}
            <span
              className={`text-sm font-semibold sm:hidden ${monitor ? 'font-mono text-cyan-300' : 'text-neutral-900 dark:text-neutral-100'}`}
            >
              {activeItem?.label ?? 'goscaffoldforlevi'}
            </span>
            <div className="flex items-center gap-1 sm:hidden">
              <NotificationBell monitor={monitor} />
              <button
                type="button"
                onClick={() => setMenuOpen((v) => !v)}
                aria-expanded={menuOpen}
                aria-controls="mobile-nav-panel"
                aria-label={menuOpen ? '收起菜单' : '展开菜单'}
                className={`rounded-md p-2 ${
                  monitor ? 'text-neutral-300 hover:bg-white/5' : 'text-neutral-600 hover:bg-neutral-100 dark:text-neutral-300 dark:hover:bg-neutral-800'
                }`}
              >
                {menuOpen ? (
                  <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2">
                    <path d="M18 6 6 18M6 6l12 12" strokeLinecap="round" />
                  </svg>
                ) : (
                  <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2">
                    <path d="M4 7h16M4 12h16M4 17h16" strokeLinecap="round" />
                  </svg>
                )}
              </button>
            </div>
          </div>

          {menuOpen && (
            <div
              id="mobile-nav-panel"
              className={`space-y-1 border-t px-4 py-3 sm:hidden ${
                monitor ? 'border-cyan-500/10 bg-[#070a12]' : 'border-neutral-200 bg-white dark:border-neutral-800 dark:bg-neutral-900'
              }`}
            >
              {visibleItems.map((item) => (
                <NavLink key={item.to} to={item.to} className={sidebarLinkClass}>
                  {item.label}
                </NavLink>
              ))}
              <div className={`my-2 border-t ${monitor ? 'border-cyan-500/10' : 'border-neutral-200 dark:border-neutral-800'}`} />
              <div
                className={`flex items-center justify-between gap-2 px-3 py-1 text-sm ${monitor ? 'font-mono text-neutral-400' : 'text-neutral-500 dark:text-neutral-400'}`}
              >
                <span className="min-w-0 truncate" title={email ?? undefined}>
                  {email ?? '…'}
                </span>
                {user?.role && <span className={`shrink-0 ${accountBadgeClass}`}>{user.role}</span>}
              </div>
              <button onClick={handleLogout} className={`mx-3 mt-1 w-[calc(100%-1.5rem)] ${logoutButtonClass}`}>
                退出登录
              </button>
            </div>
          )}
        </header>
        <main className={monitor ? '' : 'mx-auto max-w-5xl px-4 py-8 sm:px-6'}>
          <Outlet />
        </main>
      </div>
    </div>
  )
}
