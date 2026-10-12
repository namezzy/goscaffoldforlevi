// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as incidentsApi from '../api/incidents'
import type { Incident } from '../api/types'
import { IncidentsPage } from './IncidentsPage'
import { makeIncident, OPERATOR_ID } from './incidentTestFixtures'

vi.mock('../api/incidents')
vi.mock('../auth/useAuth', () => ({
  useAuth: () => ({ status: 'authenticated', user: { id: OPERATOR_ID, role: 'operator' } }),
}))

const api = vi.mocked(incidentsApi)

function page(items: Incident[]) {
  return { items, page_info: { page: 1, page_size: 20, total: items.length, total_pages: items.length ? 1 : 0 } }
}

function renderAt(url = '/incidents') {
  render(
    <MemoryRouter initialEntries={[url]}>
      <IncidentsPage />
    </MemoryRouter>,
  )
}

describe('IncidentsPage', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    api.listIncidents.mockResolvedValue(page([]))
    api.getIncidentSummary.mockResolvedValue({ counts: {} })
  })
  afterEach(cleanup)

  it('lists incidents with the unresolved counts per severity', async () => {
    const inc = makeIncident({ severity: 'critical', replay_status: 'failed' })
    api.listIncidents.mockResolvedValue(page([inc]))
    api.getIncidentSummary.mockResolvedValue({ counts: { critical: 2 } })
    renderAt()

    const link = await screen.findByRole('link', { name: inc.event_type })
    expect(link.getAttribute('href')).toBe(`/incidents/${inc.id}`)
    expect(screen.getByText('重放失败')).toBeTruthy()
    // A severity absent from counts is zero, not missing.
    expect(screen.getByRole('button', { name: /严重/ }).textContent).toContain('2')
    expect(screen.getByRole('button', { name: /提示/ }).textContent).toContain('0')
  })

  it('reads its filters from the URL', async () => {
    renderAt('/incidents?status=resolved&severity=info&page=2')

    await waitFor(() =>
      expect(api.listIncidents).toHaveBeenCalledWith(expect.objectContaining({ status: 'resolved', severity: 'info', page: 2 })),
    )
    expect(await screen.findByText('没有匹配的异常')).toBeTruthy()
  })

  it('filters to my incidents by owner id', async () => {
    renderAt()
    await screen.findByText('目前没有异常')

    fireEvent.click(screen.getByRole('checkbox', { name: '只看我负责的' }))

    await waitFor(() =>
      expect(api.listIncidents).toHaveBeenLastCalledWith(expect.objectContaining({ owner_id: OPERATOR_ID })),
    )
  })

  it('toggles a severity filter from the summary card', async () => {
    renderAt()
    await screen.findByText('目前没有异常')

    const card = screen.getByRole('button', { name: /警告/ })
    fireEvent.click(card)

    await waitFor(() =>
      expect(api.listIncidents).toHaveBeenLastCalledWith(expect.objectContaining({ severity: 'warning' })),
    )
    expect(card.getAttribute('aria-pressed')).toBe('true')
  })
})
