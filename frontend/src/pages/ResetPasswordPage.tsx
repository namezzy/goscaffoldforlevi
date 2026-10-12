import { useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ErrorBanner } from '../components/ErrorBanner'
import { confirmPasswordReset } from '../api/auth'

/** Public — the token proves identity here, not a session. Reached from
 * the link in the reset email: AUTH_PASSWORD_RESET_URL_BASE plus
 * ?token=... (see api/openapi.yaml's confirmPasswordReset). */
export function ResetPasswordPage() {
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''

  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState(false)

  const mismatch = confirmPassword !== '' && password !== confirmPassword

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (mismatch) return
    setError(null)
    setBusy(true)
    try {
      await confirmPasswordReset({ token, password })
      setDone(true)
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-neutral-50 px-4 dark:bg-neutral-950">
      <div className="w-full max-w-sm rounded-lg border border-neutral-200 bg-white p-8 shadow-sm dark:border-neutral-800 dark:bg-neutral-900">
        <h1 className="mb-1 text-xl font-semibold text-neutral-900 dark:text-neutral-100">设置新密码</h1>

        {done ? (
          <>
            <p className="mb-6 text-sm text-neutral-600 dark:text-neutral-300">
              密码已更新，此账户此前的所有登录会话都已失效。请用新密码重新登录。
            </p>
            <Link
              to="/login"
              className="block w-full rounded-md bg-neutral-900 px-4 py-2 text-center text-sm font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
            >
              前往登录
            </Link>
          </>
        ) : !token ? (
          <>
            <p className="mb-6 text-sm text-neutral-600 dark:text-neutral-300">
              链接缺少重置令牌，无法继续。请重新从邮件中的链接打开此页面，或重新发起找回密码。
            </p>
            <Link to="/forgot-password" className="text-sm text-neutral-600 hover:underline dark:text-neutral-300">
              重新找回密码
            </Link>
          </>
        ) : (
          <>
            <p className="mb-6 text-sm text-neutral-500 dark:text-neutral-400">
              设置一个新密码。此操作会让该账户此前的所有登录会话失效。
            </p>

            {error ? (
              <div className="mb-4">
                <ErrorBanner error={error} />
              </div>
            ) : null}

            <form onSubmit={handleSubmit}>
              <label className="mb-3 block">
                <span className="mb-1 block text-sm font-medium text-neutral-700 dark:text-neutral-300">
                  新密码（12-128 位）
                </span>
                <input
                  type="password"
                  required
                  autoFocus
                  autoComplete="new-password"
                  minLength={12}
                  maxLength={128}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full rounded-md border border-neutral-300 px-3 py-2 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
                />
              </label>

              <label className="mb-1 block">
                <span className="mb-1 block text-sm font-medium text-neutral-700 dark:text-neutral-300">确认新密码</span>
                <input
                  type="password"
                  required
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  aria-invalid={mismatch}
                  className="w-full rounded-md border border-neutral-300 px-3 py-2 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
                />
              </label>
              {mismatch && <p className="mb-3 text-xs text-red-600 dark:text-red-400">两次输入的密码不一致。</p>}

              <button
                type="submit"
                disabled={busy || mismatch}
                className="mt-5 w-full rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
              >
                {busy ? '提交中…' : '设置新密码'}
              </button>
            </form>
          </>
        )}
      </div>
    </div>
  )
}
