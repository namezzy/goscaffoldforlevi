import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { getUnreadNotificationCount } from '../api/notifications'

const POLL_INTERVAL_MS = 30_000

/** The unread badge in the header — links to /notifications rather than
 * opening its own dropdown, keeping this component to "how many" and
 * leaving "which ones" to the notifications page's own list, filters and
 * pagination. Refreshes on every route change (an action on the current
 * page may have just read or created a notification) and otherwise polls,
 * since nothing pushes this update to the client. */
export function NotificationBell({ monitor }: { monitor?: boolean }) {
  const location = useLocation()
  const [count, setCount] = useState<number | null>(null)

  useEffect(() => {
    let cancelled = false
    function refresh() {
      getUnreadNotificationCount()
        .then((r) => {
          if (!cancelled) setCount(r.count)
        })
        .catch(() => {
          // Header display only — a failed refresh just leaves the last
          // known count in place rather than surfacing an error banner.
        })
    }
    refresh()
    const interval = setInterval(refresh, POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(interval)
    }
  }, [location.pathname])

  return (
    <Link
      to="/notifications"
      aria-label={count ? `通知，${count} 条未读` : '通知'}
      className={`relative rounded-md p-2 ${
        monitor ? 'text-neutral-300 hover:bg-white/5' : 'text-neutral-600 hover:bg-neutral-100 dark:text-neutral-300 dark:hover:bg-neutral-800'
      }`}
    >
      <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="2">
        <path
          d="M18 8a6 6 0 1 0-12 0c0 7-3 9-3 9h18s-3-2-3-9"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path d="M13.73 21a2 2 0 0 1-3.46 0" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
      {!!count && (
        <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-red-600 px-1 text-[10px] font-semibold leading-none text-white">
          {count > 99 ? '99+' : count}
        </span>
      )}
    </Link>
  )
}
