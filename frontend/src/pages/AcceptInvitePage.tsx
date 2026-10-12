import { useState, type FormEvent } from 'react'
import { useSearchParams } from 'react-router-dom'
import { AuthShell } from '../components/AuthShell'
import { ErrorBanner } from '../components/ErrorBanner'
import { ButtonLink, Button, Field, inputClass } from '../components/ui'
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
    <AuthShell title="接受邀请">
      {!token ? (
        <p className="text-sm text-rose-300">
          链接缺少邀请令牌，请使用邀请邮件（或邀请人提供）中的完整链接打开此页面。
        </p>
      ) : done ? (
        <div className="space-y-4 text-sm">
          <p className="text-emerald-300">账户已创建，现在可以使用邮箱和刚才设置的密码登录。</p>
          <ButtonLink to="/login" variant="primary" className="w-full">
            前往登录
          </ButtonLink>
        </div>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-4">
          {error ? <ErrorBanner error={error} /> : null}
          <Field label="姓名">
            <input
              required
              minLength={1}
              maxLength={100}
              value={name}
              onChange={(e) => setName(e.target.value)}
              className={inputClass}
            />
          </Field>
          <Field label="设置密码（12-128 位）">
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
          </Field>
          <Field label="确认密码">
            <input
              type="password"
              autoComplete="new-password"
              required
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              aria-invalid={mismatch}
              className={inputClass}
            />
            {mismatch && <span className="mt-1 block text-xs text-rose-300">两次输入的密码不一致。</span>}
          </Field>
          <Button type="submit" variant="primary" disabled={busy || mismatch} className="w-full">
            {busy ? '提交中…' : '接受邀请并创建账户'}
          </Button>
        </form>
      )}
    </AuthShell>
  )
}
