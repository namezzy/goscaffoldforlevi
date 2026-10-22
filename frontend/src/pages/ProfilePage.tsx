import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { ErrorBanner } from '../components/ErrorBanner'
import { Badge, Button, ButtonLink, Card, Field, Loading, PageHeader, formatDateTime, inputClass } from '../components/ui'
import { changePassword, getUser, updateUser } from '../api/users'
import { listDepartments } from '../api/org'
import { listSessions } from '../api/auth'
import { can, roleLabel } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import type { User } from '../api/types'

/** One labelled fact in the identity card's footer strip. Not DescriptionRow:
 * that one puts label and value on opposite ends of a narrow column, which
 * reads as a gap when the row is a quarter of the page wide. */
function Fact({ label, value, mono }: { label: string; value: ReactNode; mono?: boolean }) {
  return (
    <div className="px-6 py-4">
      <dt className="font-mono text-[11px] uppercase tracking-wider text-slate-500">{label}</dt>
      <dd
        className={`mt-1 truncate text-slate-200 ${mono ? 'font-mono text-xs' : 'text-sm'}`}
        title={typeof value === 'string' ? value : undefined}
      >
        {value}
      </dd>
    </div>
  )
}

export function ProfilePage() {
  const { user } = useAuth()
  const [profile, setProfile] = useState<User | null>(null)
  const [loadError, setLoadError] = useState<unknown>(null)

  const [departmentName, setDepartmentName] = useState<string | null>(null)
  const [sessionCount, setSessionCount] = useState<number | null>(null)

  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [profileSaving, setProfileSaving] = useState(false)
  const [profileError, setProfileError] = useState<unknown>(null)
  const [profileSaved, setProfileSaved] = useState(false)

  const [newPassword, setNewPassword] = useState('')
  const [passwordSaving, setPasswordSaving] = useState(false)
  const [passwordError, setPasswordError] = useState<unknown>(null)
  const [passwordSaved, setPasswordSaved] = useState(false)

  useEffect(() => {
    if (!user) return
    getUser(user.id)
      .then((u) => {
        setProfile(u)
        setName(u.name)
        setEmail(u.email)
        setUsername(u.username ?? '')
      })
      .catch(setLoadError)
  }, [user])

  // The two decorations below are deliberately not part of the page's load
  // state: they only enrich the identity card, so a failure leaves a dash
  // rather than taking the whole page — the profile itself still loaded.
  useEffect(() => {
    if (!user || !profile?.department_id || !can(user.role, 'department:read')) return
    listDepartments()
      .then((page) => setDepartmentName(page.items.find((d) => d.id === profile.department_id)?.name ?? null))
      .catch(() => setDepartmentName(null))
  }, [user, profile?.department_id])

  useEffect(() => {
    if (!user) return
    listSessions()
      .then((data) => setSessionCount(data.items.length))
      .catch(() => setSessionCount(null))
  }, [user])

  async function handleSaveProfile(e: FormEvent) {
    e.preventDefault()
    if (!user || !profile || clearingUsername) return
    setProfileSaving(true)
    setProfileError(null)
    setProfileSaved(false)
    try {
      // Only fields that actually changed are sent, so a blank username
      // (never set) does not turn into a spurious "clear it" request when
      // the user only meant to rename themselves.
      const patch: { name?: string; email?: string; username?: string } = {}
      if (name !== profile.name) patch.name = name
      if (email !== profile.email) patch.email = email
      if (username !== (profile.username ?? '') && username !== '') patch.username = username
      if (Object.keys(patch).length === 0) {
        setProfileSaving(false)
        return
      }
      const updated = await updateUser(user.id, patch)
      setProfile(updated)
      setUsername(updated.username ?? '')
      setProfileSaved(true)
    } catch (err) {
      setProfileError(err)
    } finally {
      setProfileSaving(false)
    }
  }

  async function handleChangePassword(e: FormEvent) {
    e.preventDefault()
    if (!user) return
    setPasswordSaving(true)
    setPasswordError(null)
    setPasswordSaved(false)
    try {
      await changePassword(user.id, { password: newPassword })
      setNewPassword('')
      setPasswordSaved(true)
    } catch (err) {
      setPasswordError(err)
    } finally {
      setPasswordSaving(false)
    }
  }

  if (loadError) return <ErrorBanner error={loadError} />
  if (!profile) return <Loading />

  const profileUnchanged =
    name === profile.name && email === profile.email && username === (profile.username ?? '')

  // Clearing an existing username is not a supported update — see
  // MembersPage's EditMemberForm for the same rule applied to the admin form.
  const clearingUsername = Boolean(profile.username) && username.trim() === ''

  const active = profile.status === 'active'
  // Falls back to the email so the avatar never renders as an empty circle;
  // Array.from rather than [0] because a CJK name's first character can be
  // a surrogate pair, which indexing would split into a replacement glyph.
  const initial = Array.from(profile.name.trim())[0] ?? Array.from(profile.email)[0]?.toUpperCase() ?? '?'

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Profile"
        title="我的资料"
        description="你的身份信息与登录凭据"
        actions={
          <>
            <ButtonLink to="/security">安全中心</ButtonLink>
            <ButtonLink to="/notifications">我的通知</ButtonLink>
          </>
        }
      />

      <Card padded={false}>
        <div className="flex flex-col gap-5 p-6 sm:flex-row sm:items-center">
          <div
            aria-hidden
            className="flex h-20 w-20 shrink-0 items-center justify-center rounded-2xl border border-cyan-400/30 bg-gradient-to-br from-cyan-500/20 to-sky-500/10 text-3xl font-semibold text-cyan-200 shadow-[0_0_28px_-10px_rgba(34,211,238,0.8)]"
          >
            {initial}
          </div>
          <div className="min-w-0">
            <h2 className="truncate text-2xl font-semibold tracking-tight text-slate-50">{profile.name}</h2>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <Badge tone="cyan">{roleLabel(profile.role)}</Badge>
              <Badge tone={active ? 'green' : 'neutral'} dot>
                {active ? '已启用' : '已禁用'}
              </Badge>
            </div>
          </div>
        </div>

        <dl className="grid grid-cols-1 divide-y divide-white/[0.06] border-t border-white/[0.06] sm:grid-cols-2 sm:divide-y-0 lg:grid-cols-4 lg:divide-x">
          <Fact label="登录邮箱" value={profile.email} />
          <Fact label="用户名" value={profile.username ?? '未设置'} />
          <Fact label="部门" value={profile.department_id ? (departmentName ?? '—') : '未分配'} />
          <Fact label="加入时间" value={formatDateTime(profile.created_at)} mono />
        </dl>
      </Card>

      <div className="grid items-start gap-6 lg:grid-cols-3">
        <Card title="修改资料" className="lg:col-span-2">
          <form onSubmit={handleSaveProfile} className="space-y-4">
            {profileError ? <ErrorBanner error={profileError} /> : null}

            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="姓名">
                <input
                  value={name}
                  onChange={(e) => {
                    setName(e.target.value)
                    setProfileSaved(false)
                  }}
                  required
                  minLength={1}
                  maxLength={100}
                  className={inputClass}
                />
              </Field>

              <Field label="登录邮箱">
                <input
                  type="email"
                  value={email}
                  onChange={(e) => {
                    setEmail(e.target.value)
                    setProfileSaved(false)
                  }}
                  required
                  className={inputClass}
                />
              </Field>
            </div>

            <Field label="用户名（可选，作为登录邮箱之外的第二种登录方式）">
              <input
                value={username}
                onChange={(e) => {
                  setUsername(e.target.value)
                  setProfileSaved(false)
                }}
                placeholder="3-32 位，字母开头，可含数字/下划线/连字符"
                minLength={3}
                maxLength={32}
                pattern="[a-zA-Z][a-zA-Z0-9_-]{2,31}"
                aria-invalid={clearingUsername}
                className={inputClass}
              />
              {clearingUsername && (
                <p className="mt-1 text-xs text-amber-300">
                  暂不支持清空用户名——请填写新用户名，或恢复为 {profile.username}。
                </p>
              )}
            </Field>

            <div className="flex items-center gap-3 border-t border-white/[0.06] pt-4">
              <Button type="submit" variant="primary" disabled={profileSaving || profileUnchanged || clearingUsername}>
                {profileSaving ? '保存中…' : '保存'}
              </Button>
              {profileSaved && <span className="text-sm text-emerald-300">已保存</span>}
              <span className="ml-auto text-xs text-slate-500">上次更新 {formatDateTime(profile.updated_at)}</span>
            </div>
          </form>
        </Card>

        <div className="space-y-6">
          <Card title="修改密码">
            <form onSubmit={handleChangePassword} className="space-y-4">
              {passwordError ? <ErrorBanner error={passwordError} /> : null}
              <Field label="新密码" hint="12-128 位。修改后其它设备上的登录态不会被自动注销，可在安全中心逐个下线。">
                <input
                  type="password"
                  autoComplete="new-password"
                  value={newPassword}
                  onChange={(e) => {
                    setNewPassword(e.target.value)
                    setPasswordSaved(false)
                  }}
                  required
                  minLength={12}
                  maxLength={128}
                  className={inputClass}
                />
              </Field>
              <div className="flex items-center gap-3">
                <Button type="submit" variant="primary" disabled={passwordSaving}>
                  {passwordSaving ? '保存中…' : '修改密码'}
                </Button>
                {passwordSaved && <span className="text-sm text-emerald-300">已修改</span>}
              </div>
            </form>
          </Card>

          <Card title="登录与安全">
            <div className="flex items-baseline gap-2">
              <span className="font-mono text-3xl font-semibold tabular-nums text-slate-50">{sessionCount ?? '—'}</span>
              <span className="text-sm text-slate-400">个活跃会话</span>
            </div>
            <p className="mt-2 text-xs text-slate-500">包含当前这台设备。发现不认识的登录，可以在安全中心把它下线。</p>
            <div className="mt-4">
              <ButtonLink to="/security">前往安全中心</ButtonLink>
            </div>
          </Card>
        </div>
      </div>
    </div>
  )
}
