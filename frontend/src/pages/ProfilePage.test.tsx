// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as usersApi from '../api/users'
import * as orgApi from '../api/org'
import * as authApi from '../api/auth'
import type { Department, Role, Session, User } from '../api/types'
import { ProfilePage } from './ProfilePage'

vi.mock('../api/users')
vi.mock('../api/org')
vi.mock('../api/auth')

const ME_ID = '11111111-1111-4111-8111-111111111111'
const DEPT_ID = '22222222-2222-4222-8222-222222222222'

let myRole: Role = 'member_admin'
vi.mock('../auth/useAuth', () => ({
  useAuth: () => ({ status: 'authenticated', user: { id: ME_ID, role: myRole } }),
}))

const users = vi.mocked(usersApi)
const org = vi.mocked(orgApi)
const auth = vi.mocked(authApi)

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: ME_ID,
    email: 'ada@example.com',
    username: 'ada',
    name: '阿达',
    status: 'active',
    role: myRole,
    created_at: '2026-01-02T03:04:05Z',
    updated_at: '2026-02-03T04:05:06Z',
    ...overrides,
  }
}

function makeDepartment(): Department {
  return { id: DEPT_ID, name: '平台组', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }
}

function makeSession(id: string, current: boolean): Session {
  return { id, current, created_at: '2026-02-01T00:00:00Z', expires_at: '2026-03-01T00:00:00Z' }
}

function renderPage() {
  render(
    <MemoryRouter>
      <ProfilePage />
    </MemoryRouter>,
  )
}

describe('ProfilePage', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    myRole = 'member_admin'
    users.getUser.mockResolvedValue(makeUser())
    org.listDepartments.mockResolvedValue({ items: [makeDepartment()] })
    auth.listSessions.mockResolvedValue({ items: [makeSession('s1', true), makeSession('s2', false)] })
  })
  afterEach(cleanup)

  it('shows who you are: identity, department and active sessions', async () => {
    users.getUser.mockResolvedValue(makeUser({ department_id: DEPT_ID }))
    renderPage()

    expect(await screen.findByRole('heading', { name: '阿达' })).toBeTruthy()
    expect(screen.getByText('已启用')).toBeTruthy()
    expect(screen.getByText('ada@example.com')).toBeTruthy()
    expect(await screen.findByText('平台组')).toBeTruthy()
    expect(await screen.findByText('2')).toBeTruthy()
  })

  it('leaves the department unresolved rather than asking for a list it cannot read', async () => {
    myRole = 'operator'
    users.getUser.mockResolvedValue(makeUser({ department_id: DEPT_ID, role: 'operator' }))
    renderPage()

    await screen.findByRole('heading', { name: '阿达' })
    await waitFor(() => expect(auth.listSessions).toHaveBeenCalled())
    expect(org.listDepartments).not.toHaveBeenCalled()
  })

  it('stays usable when the decorations fail', async () => {
    users.getUser.mockResolvedValue(makeUser({ department_id: DEPT_ID }))
    org.listDepartments.mockRejectedValue(new Error('403'))
    auth.listSessions.mockRejectedValue(new Error('boom'))
    renderPage()

    expect(await screen.findByRole('heading', { name: '阿达' })).toBeTruthy()
    await waitFor(() => expect(screen.getAllByText('—').length).toBeGreaterThan(0))
    expect(screen.getByRole('button', { name: '保存' })).toBeTruthy()
  })

  it('sends only the field that changed', async () => {
    users.updateUser.mockResolvedValue(makeUser({ name: '阿达二号' }))
    renderPage()

    const save = (await screen.findByRole('button', { name: '保存' })) as HTMLButtonElement
    expect(save.disabled).toBe(true)

    fireEvent.change(screen.getByDisplayValue('阿达'), { target: { value: '阿达二号' } })
    expect(save.disabled).toBe(false)
    fireEvent.click(save)

    await waitFor(() => expect(users.updateUser).toHaveBeenCalledWith(ME_ID, { name: '阿达二号' }))
  })

  it('refuses to clear an existing username', async () => {
    renderPage()

    fireEvent.change(await screen.findByDisplayValue('ada'), { target: { value: '' } })

    expect(screen.getByText(/暂不支持清空用户名/)).toBeTruthy()
    expect((screen.getByRole('button', { name: '保存' }) as HTMLButtonElement).disabled).toBe(true)
  })
})
