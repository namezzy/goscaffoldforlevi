// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as incidentsApi from '../api/incidents'
import * as usersApi from '../api/users'
import { ApiError } from '../api/errors'
import type { Role } from '../api/types'
import { IncidentDetailPage } from './IncidentDetailPage'
import { makeEvent, makeIncident, OPERATOR_ID } from './incidentTestFixtures'

vi.mock('../api/incidents')
vi.mock('../api/users')

const auth = vi.hoisted(() => ({ role: 'operator' as Role }))
vi.mock('../auth/useAuth', () => ({
  useAuth: () => ({ status: 'authenticated', user: { id: OPERATOR_ID, role: auth.role } }),
}))

const api = vi.mocked(incidentsApi)
const users = vi.mocked(usersApi)

function renderPage(incident = makeIncident()) {
  api.getIncident.mockResolvedValue(incident)
  render(
    <MemoryRouter initialEntries={[`/incidents/${incident.id}`]}>
      <Routes>
        <Route path="/incidents/:id" element={<IncidentDetailPage />} />
      </Routes>
    </MemoryRouter>,
  )
  return screen.findByRole('heading', { name: incident.event_type })
}

const button = (name: string) => screen.queryByRole('button', { name })

describe('IncidentDetailPage', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    auth.role = 'operator'
    api.getIncidentActivity.mockResolvedValue([])
    users.listUsers.mockResolvedValue({ items: [], page_info: { page: 1, page_size: 100, total: 0, total_pages: 0 } })
  })
  afterEach(cleanup)

  it('offers every action on an open incident to a manager', async () => {
    await renderPage()

    for (const name of ['确认', '重放死信', '解决', '添加备注', '指派给我']) {
      expect(button(name), name).not.toBeNull()
    }
  })

  // The owner controls used to sit inside a <label>; without the user list
  // its first labelable child was "指派给我", so clicking the "负责人" caption
  // silently assigned the incident to the viewer.
  it('does not assign on a click of the owner caption', async () => {
    await renderPage()

    fireEvent.click(screen.getByText('负责人', { selector: 'span' }))

    expect(api.assignIncidentOwner).not.toHaveBeenCalled()
  })

  it('offers no action on a resolved incident — resolved is terminal', async () => {
    api.getIncidentActivity.mockResolvedValue([makeEvent({ after: { note: '已修复 SMTP 配置' } })])
    await renderPage(makeIncident({ status: 'resolved', resolved_at: '2026-10-01T10:00:00Z' }))

    for (const name of ['确认', '重放死信', '解决', '添加备注', '指派给我']) {
      expect(button(name), name).toBeNull()
    }
    // The processing record stays readable.
    expect(screen.getByText('已修复 SMTP 配置')).toBeTruthy()
  })

  it('offers no action to a read-only holder of incident:read', async () => {
    auth.role = 'auditor'
    await renderPage()

    expect(button('确认')).toBeNull()
    expect(button('重放死信')).toBeNull()
  })

  it('disables replay while the previous one is still pending', async () => {
    await renderPage(makeIncident({ status: 'acknowledged', replay_status: 'pending' }))

    expect(screen.getByText('已提交重放，等待结果')).toBeTruthy()
    expect((button('重放死信') as HTMLButtonElement).disabled).toBe(true)
    // Acknowledge only applies to an open incident.
    expect(button('确认')).toBeNull()
  })

  it('submits a replay after confirmation and shows it as pending, not succeeded', async () => {
    const incident = makeIncident({ status: 'acknowledged' })
    api.requestIncidentReplay.mockResolvedValue({ ...incident, replay_status: 'pending' })
    await renderPage(incident)

    fireEvent.click(button('重放死信')!)
    expect(api.requestIncidentReplay).not.toHaveBeenCalled()
    fireEvent.click(button('提交重放')!)

    await screen.findByText('已提交重放，等待结果')
    expect(api.requestIncidentReplay).toHaveBeenCalledWith(incident.id)
    expect(screen.queryByText('重放成功')).toBeNull()
    expect((button('重放死信') as HTMLButtonElement).disabled).toBe(true)
  })

  it('shows the server refusal when an action conflicts', async () => {
    api.resolveIncident.mockRejectedValue(
      new ApiError(409, { error: { code: 'INCIDENT_INVALID_TRANSITION', message: 'incident is resolved' } }),
    )
    await renderPage()

    fireEvent.click(button('解决')!)
    fireEvent.click(screen.getAllByRole('button', { name: '解决' }).at(-1)!)

    await screen.findByText('incident is resolved')
  })

  it('skips the owner lookup without user:read and shows the raw id', async () => {
    await renderPage(makeIncident({ owner_id: OPERATOR_ID }))

    expect(screen.getByTitle(OPERATOR_ID)).toBeTruthy()
    expect(users.getUser).not.toHaveBeenCalled()
  })

  it('resolves the owner to an email with user:read', async () => {
    auth.role = 'system_admin'
    users.getUser.mockResolvedValue({
      id: OPERATOR_ID, email: 'ops@example.com', name: 'Ops', status: 'active', role: 'operator',
      created_at: '', updated_at: '',
    })
    await renderPage(makeIncident({ owner_id: OPERATOR_ID }))

    await waitFor(() => expect(screen.getAllByText('ops@example.com').length).toBeGreaterThan(0))
    expect(users.getUser).toHaveBeenCalledWith(OPERATOR_ID)
  })
})
