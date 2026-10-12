import { useState, type FormEvent } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { ErrorBanner } from '../components/ErrorBanner'
import { login } from '../auth/store'
import { useAuth } from '../auth/useAuth'

// The default landing page once there is nowhere else to send someone —
// no prior page, no query string. Same route for every role: the
// workbench itself is what varies by role (see DashboardPage's admin
// section), not the URL someone lands on after logging in.
const DEFAULT_DESTINATION = '/dashboard'

interface LoginLocationState {
  from?: { pathname: string; search: string }
}

/** Reduces the location react-router redirected from into "path + query" — never a bare pathname, or a link into a filtered list loses its filters on the round trip through login. */
function redirectTarget(state: unknown): string {
  const from = (state as LoginLocationState | null)?.from
  if (!from) return DEFAULT_DESTINATION
  return from.pathname + from.search
}

export function LoginPage() {
  const { status } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  if (status === 'authenticated') {
    return <Navigate to={redirectTarget(location.state)} replace />
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      await login(identifier, password)
      navigate(redirectTarget(location.state), { replace: true })
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-neutral-50 px-4 dark:bg-neutral-950">
      <form
        onSubmit={handleSubmit}
        className="w-full max-w-sm rounded-lg border border-neutral-200 bg-white p-8 shadow-sm dark:border-neutral-800 dark:bg-neutral-900"
      >
        <h1 className="mb-1 text-xl font-semibold text-neutral-900 dark:text-neutral-100">
          goscaffoldforlevi
        </h1>
        <p className="mb-6 text-sm text-neutral-500 dark:text-neutral-400">登录以继续</p>

        {error ? (
          <div className="mb-4">
            <ErrorBanner error={error} />
          </div>
        ) : null}

        <label className="mb-3 block">
          <span className="mb-1 block text-sm font-medium text-neutral-700 dark:text-neutral-300">
            邮箱或用户名
          </span>
          <input
            type="text"
            required
            autoFocus
            autoComplete="username"
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            className="w-full rounded-md border border-neutral-300 px-3 py-2 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
          />
        </label>

        <label className="mb-2 block">
          <span className="mb-1 block text-sm font-medium text-neutral-700 dark:text-neutral-300">
            密码
          </span>
          <input
            type="password"
            required
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="w-full rounded-md border border-neutral-300 px-3 py-2 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
          />
        </label>
        <div className="mb-6 text-right">
          <Link to="/forgot-password" className="text-xs text-neutral-500 hover:underline dark:text-neutral-400">
            忘记密码？
          </Link>
        </div>

        <button
          type="submit"
          disabled={busy}
          className="w-full rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
        >
          {busy ? '登录中…' : '登录'}
        </button>
      </form>
    </div>
  )
}
