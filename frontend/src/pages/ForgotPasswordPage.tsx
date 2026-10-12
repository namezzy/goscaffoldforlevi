import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { AuthShell } from '../components/AuthShell'
import { ErrorBanner } from '../components/ErrorBanner'
import { Button, Field, inputClass, linkClass } from '../components/ui'
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
    <AuthShell
      title="找回密码"
      subtitle={
        sent
          ? '如果该邮箱或用户名对应一个账户，我们已经发送了一封包含重置链接的邮件。链接有效期有限，且只能使用一次。'
          : '输入登录邮箱或用户名，我们会发送一封重置密码的邮件。'
      }
    >
      {sent ? (
        <div className="text-center">
          <Link to="/login" className={`text-sm ${linkClass}`}>
            返回登录
          </Link>
        </div>
      ) : (
        <>
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

            <Button type="submit" variant="primary" disabled={busy} className="w-full">
              {busy ? '发送中…' : '发送重置邮件'}
            </Button>
          </form>

          <Link to="/login" className={`mt-4 block text-center text-sm ${linkClass}`}>
            返回登录
          </Link>
        </>
      )}
    </AuthShell>
  )
}
