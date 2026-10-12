import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { ErrorBanner } from '../components/ErrorBanner'
import { requestPasswordReset } from '../api/auth'

/** Public — reached before the caller has any session. The response is
 * identical whether or not the identifier matches an account (see
 * api/openapi.yaml's requestPasswordReset), so this page shows the same
 * success message unconditionally rather than branching on the result. */
export function ForgotPasswordPage() {
  const [identifier, setIdentifier] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const [sent, setSent] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      await requestPasswordReset({ identifier })
      setSent(true)
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-neutral-50 px-4 dark:bg-neutral-950">
      <div className="w-full max-w-sm rounded-lg border border-neutral-200 bg-white p-8 shadow-sm dark:border-neutral-800 dark:bg-neutral-900">
        <h1 className="mb-1 text-xl font-semibold text-neutral-900 dark:text-neutral-100">找回密码</h1>

        {sent ? (
          <>
            <p className="mb-6 text-sm text-neutral-600 dark:text-neutral-300">
              如果该邮箱或用户名对应一个账户，我们已经发送了一封包含重置链接的邮件。链接有效期有限，且只能使用一次。
            </p>
            <Link to="/login" className="text-sm text-neutral-600 hover:underline dark:text-neutral-300">
              返回登录
            </Link>
          </>
        ) : (
          <>
            <p className="mb-6 text-sm text-neutral-500 dark:text-neutral-400">
              输入登录邮箱或用户名，我们会发送一封重置密码的邮件。
            </p>

            {error ? (
              <div className="mb-4">
                <ErrorBanner error={error} />
              </div>
            ) : null}

            <form onSubmit={handleSubmit}>
              <label className="mb-6 block">
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

              <button
                type="submit"
                disabled={busy}
                className="w-full rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
              >
                {busy ? '发送中…' : '发送重置邮件'}
              </button>
            </form>

            <Link to="/login" className="mt-4 block text-center text-sm text-neutral-500 hover:underline dark:text-neutral-400">
              返回登录
            </Link>
          </>
        )}
      </div>
    </div>
  )
}
