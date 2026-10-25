import { useEffect, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { getUser } from '../api/users'
import { can, type Permission } from '../auth/permissions'
import { logout } from '../auth/store'
import { useAuth } from '../auth/useAuth'
import { NotificationBell } from './NotificationBell'
import { Button } from './ui'

interface NavItem {
  to: string
  label: string
  group: NavGroup
  /** SVG path data for a 24×24, stroke-only icon. */
  icon: string
  /** Omitted for an item every signed-in account sees, regardless of role. */
  permission?: Permission
}

// One list drives the sidebar, the mobile menu and the page title (from the
// active item's label) — three places that would otherwise drift out of
// sync if each kept its own copy.
type NavGroup = '工作' | '管理' | '运维' | '个人'
const NAV_GROUPS: NavGroup[] = ['工作', '管理', '运维', '个人']

const NAV_ITEMS: NavItem[] = [
  { to: '/dashboard', label: '工作台', group: '工作', icon: 'M4 13h6V4H4zM14 20h6v-9h-6zM14 4v4h6V4zM4 20h6v-4H4z' },
  { to: '/tickets', label: '工单', group: '工作', icon: 'M5 4h14v16l-3-2-2 2-2-2-2 2-2-2-3 2zM9 9h6M9 13h6' },
  { to: '/members', label: '成员管理', group: '管理', permission: 'user:list', icon: 'M16 19v-1a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v1M9 10a3 3 0 1 0 0-6 3 3 0 0 0 0 6M22 19v-1a4 4 0 0 0-3-3.9M16 4.1a3 3 0 0 1 0 5.8' },
  { to: '/org', label: '组织架构', group: '管理', permission: 'org:read', icon: 'M9 3h6v5H9zM3 16h6v5H3zM15 16h6v5h-6zM12 8v4M6 16v-4h12v4' },
  { to: '/invitations', label: '邀请管理', group: '管理', permission: 'invite:read', icon: 'M3 6h18v12H3zM3 7l9 6 9-6' },
  { to: '/roles', label: '角色权限', group: '管理', icon: 'M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6zM9 12l2 2 4-4' },
  { to: '/audit', label: '操作审计', group: '管理', permission: 'audit:read', icon: 'M9 5H6v15h12V5h-3M9 3h6v4H9zM9 12h6M9 16h4' },
  { to: '/incidents', label: '异常中心', group: '运维', permission: 'incident:read', icon: 'M12 3l10 17H2zM12 10v4M12 17.5v.5' },
  { to: '/status', label: '监控台', group: '运维', permission: 'requests:read', icon: 'M3 12h4l3-8 4 16 3-8h4' },
  { to: '/profile', label: '我的资料', group: '个人', icon: 'M20 21v-1a5 5 0 0 0-5-5H9a5 5 0 0 0-5 5v1M12 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8' },
  { to: '/security', label: '安全中心', group: '个人', icon: 'M5 11h14v10H5zM8 11V7a4 4 0 0 1 8 0v4' },
]

function NavIcon({ d }: { d: string }) {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d={d} />
    </svg>
  )
}

function Brand() {
  return (
    <div className="flex items-center gap-2.5">
      <span className="relative flex h-7 w-7 items-center justify-center rounded-lg border border-cyan-400/40 bg-cyan-400/10 shadow-[0_0_14px_-2px_rgba(34,211,238,0.6)]">
        <span className="h-2 w-2 rounded-sm bg-cyan-300" />
      </span>
      <span className="font-mono text-sm font-semibold tracking-tight text-slate-100">goscaffold<span className="text-cyan-400">·</span>levi</span>
    </div>
  )
}

const sidebarLinkClass = ({ isActive }: { isActive: boolean }) =>
  `group relative flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition ${
    isActive
      ? 'bg-cyan-400/10 text-cyan-200 before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-cyan-400 before:shadow-[0_0_8px_rgba(34,211,238,0.9)]'
      : 'text-slate-400 hover:bg-white/[0.04] hover:text-slate-100'
  }`

function NavList({ items }: { items: NavItem[] }) {
  return (
    <>
      {NAV_GROUPS.map((group) => {
        const inGroup = items.filter((i) => i.group === group)
        if (inGroup.length === 0) return null
        return (
          <div key={group} className="space-y-0.5">
            <div className="px-3 pb-1 pt-4 font-mono text-[10px] uppercase tracking-[0.2em] text-slate-600">{group}</div>
            {inGroup.map((item) => (
              <NavLink key={item.to} to={item.to} className={sidebarLinkClass}>
                <NavIcon d={item.icon} />
                {item.label}
              </NavLink>
            ))}
          </div>
        )
      })}
    </>
  )
}

export function Layout() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
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

  const roleBadge = user?.role && (
    <span className="rounded-md border border-cyan-400/30 bg-cyan-400/10 px-1.5 py-0.5 font-mono text-[11px] text-cyan-300">
      {user.role}
    </span>
  )

  return (
    <div className="min-h-screen">
      <aside className="hidden border-r border-white/[0.06] bg-slate-950/70 backdrop-blur-md sm:fixed sm:inset-y-0 sm:left-0 sm:flex sm:w-60 sm:flex-col">
        <div className="flex h-14 items-center border-b border-white/[0.06] px-5">
          <Brand />
        </div>
        <nav className="flex-1 overflow-y-auto px-3 pb-6">
          <NavList items={visibleItems} />
        </nav>
        <div className="border-t border-white/[0.06] px-5 py-3 font-mono text-[10px] text-slate-600">
          <span className="mr-1.5 inline-block h-1.5 w-1.5 rounded-full bg-emerald-400 shadow-[0_0_6px_rgba(52,211,153,0.9)]" />
          SYSTEM ONLINE
        </div>
      </aside>

      <div className="sm:pl-60">
        <header className="sticky top-0 z-40 border-b border-white/[0.06] bg-slate-950/70 backdrop-blur-md">
          <div className="flex h-14 items-center justify-between px-4 sm:px-8">
            <div className="hidden items-center gap-2 font-mono text-xs sm:flex">
              <span className="text-slate-600">console</span>
              <span className="text-slate-700">/</span>
              <span className="text-cyan-300">{activeItem?.label ?? ''}</span>
            </div>

            <div className="hidden items-center gap-3 text-sm sm:flex">
              <span className="flex max-w-[18rem] items-center gap-2 truncate text-slate-400" title={email ?? undefined}>
                <span className="truncate">{email ?? '…'}</span>
                {roleBadge}
              </span>
              <NotificationBell />
              <Button size="sm" onClick={handleLogout}>
                退出登录
              </Button>
            </div>

            {/* Mobile: brand stands in for the sidebar, and everything
                else — nav, account, logout — collapses behind one button. */}
            <div className="sm:hidden">
              <Brand />
            </div>
            <div className="flex items-center gap-1 sm:hidden">
              <NotificationBell />
              <button
                type="button"
                onClick={() => setMenuOpen((v) => !v)}
                aria-expanded={menuOpen}
                aria-controls="mobile-nav-panel"
                aria-label={menuOpen ? '收起菜单' : '展开菜单'}
                className="rounded-lg p-2 text-slate-300 hover:bg-white/5"
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
            <div id="mobile-nav-panel" className="border-t border-white/[0.06] px-3 pb-4 sm:hidden">
              <NavList items={visibleItems} />
              <div className="my-3 border-t border-white/[0.06]" />
              <div className="flex items-center justify-between gap-2 px-3 py-1 text-sm text-slate-400">
                <span className="min-w-0 truncate" title={email ?? undefined}>
                  {email ?? '…'}
                </span>
                {roleBadge}
              </div>
              <div className="px-3 pt-2">
                <Button onClick={handleLogout} className="w-full">
                  退出登录
                </Button>
              </div>
            </div>
          )}
        </header>
        <main className="mx-auto w-full max-w-[1800px] px-4 py-8 sm:px-6 lg:px-10 2xl:px-14">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
