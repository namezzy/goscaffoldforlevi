import { useState, type FormEvent } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { AuthShell } from '../components/AuthShell'
import { ErrorBanner } from '../components/ErrorBanner'
import { Button, Field, inputClass, linkClass } from '../components/ui'
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
    <AuthShell title="控制台登录" subtitle="登录以继续">
      <form onSubmit={handleSubmit} className="space-y-4">
        {error ? <ErrorBanner error={error} /> : null}

        <Field label="邮箱或用户名">
          <input
            type="text"
            required
            autoFocus
            autoComplete="username"
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            className={inputClass}
          />
        </Field>

        <div>
          <Field label="密码">
            <input
              type="password"
              required
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={inputClass}
            />
          </Field>
          <div className="mt-2 text-right">
            <Link to="/forgot-password" className={`text-xs ${linkClass}`}>
              忘记密码？
            </Link>
          </div>
        </div>

        <Button type="submit" variant="primary" disabled={busy} className="w-full">
          {busy ? '登录中…' : '登录'}
        </Button>
      </form>
    </AuthShell>
  )
}
