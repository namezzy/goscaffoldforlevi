import { useEffect, useState, type FormEvent } from 'react'
import { ErrorBanner } from '../components/ErrorBanner'
import { Badge, Button, Card, DescriptionRow, Field, Loading, PageHeader, inputClass } from '../components/ui'
import { changePassword, getUser, updateUser } from '../api/users'
import { roleLabel } from '../auth/permissions'
import { useAuth } from '../auth/useAuth'
import type { User } from '../api/types'

export function ProfilePage() {
  const { user } = useAuth()
  const [profile, setProfile] = useState<User | null>(null)
  const [loadError, setLoadError] = useState<unknown>(null)

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

  return (
    <div className="space-y-6">
      <div>
        <PageHeader eyebrow="Profile" title="我的资料" />
        <dl className="mt-4 max-w-sm">
          <DescriptionRow label="角色" value={roleLabel(profile.role)} />
          <DescriptionRow
            label="状态"
            value={
              <Badge tone={profile.status === 'active' ? 'green' : 'neutral'} dot>
                {profile.status === 'active' ? '已启用' : '已禁用'}
              </Badge>
            }
          />
        </dl>
      </div>

      <Card title="修改资料" className="max-w-sm">
        <form onSubmit={handleSaveProfile} className="space-y-4">
          {profileError ? <ErrorBanner error={profileError} /> : null}

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

          <div className="flex items-center gap-3">
            <Button type="submit" variant="primary" disabled={profileSaving || profileUnchanged || clearingUsername}>
              {profileSaving ? '保存中…' : '保存'}
            </Button>
            {profileSaved && <span className="text-sm text-emerald-300">已保存</span>}
          </div>
        </form>
      </Card>

      <Card title="修改密码" className="max-w-sm">
        <form onSubmit={handleChangePassword} className="space-y-4">
          {passwordError ? <ErrorBanner error={passwordError} /> : null}
          <input
            type="password"
            placeholder="新密码（12-128 位）"
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
          <div className="flex items-center gap-3">
            <Button type="submit" variant="primary" disabled={passwordSaving}>
              {passwordSaving ? '保存中…' : '修改密码'}
            </Button>
            {passwordSaved && <span className="text-sm text-emerald-300">已修改</span>}
          </div>
        </form>
      </Card>
    </div>
  )
}
