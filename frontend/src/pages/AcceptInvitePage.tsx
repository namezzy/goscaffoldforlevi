import { useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ErrorBanner } from '../components/ErrorBanner'
import { acceptInvitation } from '../api/invitations'

export function AcceptInvitePage() {
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') ?? ''

  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState(false)

  const mismatch = confirmPassword.length > 0 && password !== confirmPassword

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (mismatch) return
    setBusy(true)
    setError(null)
    try {
      await acceptInvitation({ token, name, password })
      setDone(true)
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-neutral-50 px-4 dark:bg-neutral-950">
      <div className="w-full max-w-sm space-y-6 rounded-lg border border-neutral-200 bg-white p-6 dark:border-neutral-800 dark:bg-neutral-900">
        <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">接受邀请</h1>

        {!token ? (
          <p className="text-sm text-red-600 dark:text-red-400">
            链接缺少邀请令牌，请使用邀请邮件（或邀请人提供）中的完整链接打开此页面。
          </p>
        ) : done ? (
          <div className="space-y-4 text-sm">
            <p className="text-green-700 dark:text-green-400">账户已创建，现在可以使用邮箱和刚才设置的密码登录。</p>
            <Link
              to="/login"
              className="block w-full rounded-md bg-neutral-900 px-3 py-2 text-center text-sm font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
            >
              前往登录
            </Link>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-3">
            {error ? <ErrorBanner error={error} /> : null}
            <label className="block">
              <span className="mb-1 block text-sm font-medium text-neutral-700 dark:text-neutral-300">姓名</span>
              <input
                required
                minLength={1}
                maxLength={100}
                value={name}
                onChange={(e) => setName(e.target.value)}
                className={inputClass}
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-sm font-medium text-neutral-700 dark:text-neutral-300">设置密码（12-128 位）</span>
              <input
                type="password"
                autoComplete="new-password"
                required
                minLength={12}
                maxLength={128}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className={inputClass}
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-sm font-medium text-neutral-700 dark:text-neutral-300">确认密码</span>
              <input
                type="password"
                autoComplete="new-password"
                required
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                aria-invalid={mismatch}
                className={inputClass}
              />
              {mismatch && <p className="mt-1 text-xs text-red-600 dark:text-red-400">两次输入的密码不一致。</p>}
            </label>
            <button
              type="submit"
              disabled={busy || mismatch}
              className="w-full rounded-md bg-neutral-900 px-3 py-2 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
            >
              {busy ? '提交中…' : '接受邀请并创建账户'}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}

const inputClass =
  'w-full rounded-md border border-neutral-300 px-3 py-2 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100'
