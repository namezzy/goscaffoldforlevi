import { useEffect, useState, type FormEvent } from 'react'
import { ErrorBanner } from '../components/ErrorBanner'
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
  if (!profile) return <p className="text-neutral-500">加载中…</p>

  const profileUnchanged =
    name === profile.name && email === profile.email && username === (profile.username ?? '')

  // Clearing an existing username is not a supported update — see
  // MembersPage's EditMemberForm for the same rule applied to the admin form.
  const clearingUsername = Boolean(profile.username) && username.trim() === ''

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">我的资料</h1>
        <dl className="mt-3 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
          <dt className="text-neutral-500">角色</dt>
          <dd className="text-neutral-900 dark:text-neutral-100">{roleLabel(profile.role)}</dd>
          <dt className="text-neutral-500">状态</dt>
          <dd className="text-neutral-900 dark:text-neutral-100">
            {profile.status === 'active' ? '已启用' : '已禁用'}
          </dd>
        </dl>
      </div>

      <form
        onSubmit={handleSaveProfile}
        className="max-w-sm rounded-lg border border-neutral-200 p-5 dark:border-neutral-800"
      >
        <h2 className="mb-3 text-sm font-semibold text-neutral-900 dark:text-neutral-100">修改资料</h2>
        {profileError ? (
          <div className="mb-3">
            <ErrorBanner error={profileError} />
          </div>
        ) : null}

        <label className="mb-3 block">
          <span className="mb-1 block text-xs font-medium text-neutral-500 dark:text-neutral-400">姓名</span>
          <input
            value={name}
            onChange={(e) => {
              setName(e.target.value)
              setProfileSaved(false)
            }}
            required
            minLength={1}
            maxLength={100}
            className="w-full rounded-md border border-neutral-300 px-3 py-2 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
          />
        </label>

        <label className="mb-3 block">
          <span className="mb-1 block text-xs font-medium text-neutral-500 dark:text-neutral-400">
            登录邮箱
          </span>
          <input
            type="email"
            value={email}
            onChange={(e) => {
              setEmail(e.target.value)
              setProfileSaved(false)
            }}
            required
            className="w-full rounded-md border border-neutral-300 px-3 py-2 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
          />
        </label>

        <label className="mb-3 block">
          <span className="mb-1 block text-xs font-medium text-neutral-500 dark:text-neutral-400">
            用户名（可选，作为登录邮箱之外的第二种登录方式）
          </span>
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
            className="w-full rounded-md border border-neutral-300 px-3 py-2 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
          />
          {clearingUsername && (
            <p className="mt-1 text-xs text-amber-600 dark:text-amber-400">
              暂不支持清空用户名——请填写新用户名，或恢复为 {profile.username}。
            </p>
          )}
        </label>

        <button
          type="submit"
          disabled={profileSaving || profileUnchanged || clearingUsername}
          className="rounded-md bg-neutral-900 px-4 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
        >
          {profileSaving ? '保存中…' : '保存'}
        </button>
        {profileSaved && <span className="ml-3 text-sm text-green-600">已保存</span>}
      </form>

      <form
        onSubmit={handleChangePassword}
        className="max-w-sm rounded-lg border border-neutral-200 p-5 dark:border-neutral-800"
      >
        <h2 className="mb-3 text-sm font-semibold text-neutral-900 dark:text-neutral-100">修改密码</h2>
        {passwordError ? (
          <div className="mb-3">
            <ErrorBanner error={passwordError} />
          </div>
        ) : null}
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
          className="mb-3 w-full rounded-md border border-neutral-300 px-3 py-2 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
        />
        <button
          type="submit"
          disabled={passwordSaving}
          className="rounded-md bg-neutral-900 px-4 py-1.5 text-sm font-medium text-white hover:bg-neutral-800 disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
        >
          {passwordSaving ? '保存中…' : '修改密码'}
        </button>
        {passwordSaved && <span className="ml-3 text-sm text-green-600">已修改</span>}
      </form>
    </div>
  )
}
