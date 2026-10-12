import { useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { AuthShell } from '../components/AuthShell'
import { ErrorBanner } from '../components/ErrorBanner'
import { ButtonLink, Button, Field, inputClass, linkClass } from '../components/ui'
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
    <AuthShell
      title="设置新密码"
      subtitle={
        done
          ? '密码已更新，此账户此前的所有登录会话都已失效。请用新密码重新登录。'
          : !token
            ? '链接缺少重置令牌，无法继续。请重新从邮件中的链接打开此页面，或重新发起找回密码。'
            : '设置一个新密码。此操作会让该账户此前的所有登录会话失效。'
      }
    >
      {done ? (
        <ButtonLink to="/login" variant="primary" className="w-full">
          前往登录
        </ButtonLink>
      ) : !token ? (
        <div className="text-center">
          <Link to="/forgot-password" className={`text-sm ${linkClass}`}>
            重新找回密码
          </Link>
        </div>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-4">
          {error ? <ErrorBanner error={error} /> : null}

          <Field label="新密码（12-128 位）">
            <input
              type="password"
              required
              autoFocus
              autoComplete="new-password"
              minLength={12}
              maxLength={128}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={inputClass}
            />
          </Field>

          <Field label="确认新密码">
            <input
              type="password"
              required
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              aria-invalid={mismatch}
              className={inputClass}
            />
            {mismatch && <span className="mt-1 block text-xs text-rose-300">两次输入的密码不一致。</span>}
          </Field>

          <Button type="submit" variant="primary" disabled={busy || mismatch} className="w-full">
            {busy ? '提交中…' : '设置新密码'}
          </Button>
        </form>
      )}
    </AuthShell>
  )
}
