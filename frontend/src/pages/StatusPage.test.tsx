// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as requestsApi from '../api/requests'
import * as systemApi from '../api/system'
import type { RequestEvent, Role } from '../api/types'
import { StatusPage } from './StatusPage'

vi.mock('../api/requests')
vi.mock('../api/system')

let myRole: Role = 'system_admin'
vi.mock('../auth/useAuth', () => ({
  useAuth: () => ({ status: 'authenticated', user: { id: 'u1', role: myRole } }),
}))

const requests = vi.mocked(requestsApi)
const system = vi.mocked(systemApi)

let seq = 0
function event(path: string, overrides: Partial<RequestEvent> = {}): RequestEvent {
  seq += 1
  return {
    time: new Date(Date.UTC(2026, 9, 26, 0, 0, seq % 60)).toISOString(),
    request_id: `req-${seq}`,
    method: 'GET',
    path,
    status: 200,
    bytes: 10,
    duration_ms: 1,
    ...overrides,
  }
}

beforeEach(() => {
  vi.resetAllMocks()
  myRole = 'system_admin'
  system.getHealth.mockResolvedValue({ status: 'ok', version: 'v1', commit: 'abc' })
  system.getReadiness.mockResolvedValue({ status: 'ok', components: { postgres: 'ok' } })
  requests.getRecentRequests.mockResolvedValue({
    items: [event('/api/v1/users'), event('/api/v1/tickets'), event('/api/v1/tickets')],
  })
  // Never resolves: the page keeps the live stream open, and a stream that
  // ends immediately would make the component loop on reconnect inside the
  // test.
  requests.streamRequests.mockReturnValue(new Promise(() => {}))
})

afterEach(cleanup)

describe('StatusPage request traffic', () => {
  it('draws the path map and the feed from one subscription', async () => {
    render(<StatusPage />)

    await screen.findByText('请求路径地图')
    expect(screen.getByText('实时请求流')).toBeTruthy()
    // One stream, not one per child — two subscriptions would double every
    // event's effect on the rate and the map.
    await waitFor(() => expect(requests.streamRequests).toHaveBeenCalledTimes(1))
  })

  it('filters the feed to the route clicked on the map', async () => {
    render(<StatusPage />)

    const tickets = await screen.findByRole('button', { name: '/api/v1/tickets' })
    expect(screen.getAllByTitle('/api/v1/users').length).toBeGreaterThan(0)

    fireEvent.click(tickets)

    await waitFor(() => expect(screen.queryAllByTitle('/api/v1/users')).toHaveLength(0))
    expect(screen.getAllByTitle('/api/v1/tickets').length).toBeGreaterThan(0)
  })

  it('clears the filter from the chip above the feed', async () => {
    render(<StatusPage />)

    fireEvent.click(await screen.findByRole('button', { name: '/api/v1/tickets' }))
    await waitFor(() => expect(screen.queryAllByTitle('/api/v1/users')).toHaveLength(0))

    fireEvent.click(screen.getByTitle('点击取消筛选'))

    await waitFor(() => expect(screen.getAllByTitle('/api/v1/users').length).toBeGreaterThan(0))
  })

  it('does not open the stream for a role that cannot read requests', async () => {
    myRole = 'read_only'
    render(<StatusPage />)

    await screen.findByText('监控台')
    expect(screen.queryByText('请求路径地图')).toBeNull()
    expect(requests.streamRequests).not.toHaveBeenCalled()
  })
})
