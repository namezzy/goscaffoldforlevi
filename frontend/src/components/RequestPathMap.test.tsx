// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { RequestEvent } from '../api/types'
import { RequestPathMap } from './RequestPathMap'
import type { RequestFeed } from './useRequestFeed'

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

function feedOf(items: RequestEvent[]): RequestFeed {
  return { items, status: 'live', rateCount: items.length }
}

afterEach(cleanup)

describe('RequestPathMap', () => {
  it('draws one band per route segment', () => {
    render(
      <RequestPathMap feed={feedOf([event('/api/v1/users'), event('/api/v1/tickets')])} onSelect={vi.fn()} />,
    )

    expect(screen.getByRole('button', { name: /\/api$/ })).toBeTruthy()
    expect(screen.getByRole('button', { name: /\/api\/v1\/users$/ })).toBeTruthy()
    expect(screen.getByRole('button', { name: /\/api\/v1\/tickets$/ })).toBeTruthy()
  })

  it('sizes a band by its share of traffic, so the busiest route is the widest', () => {
    render(
      <RequestPathMap
        feed={feedOf([event('/a'), event('/a'), event('/a'), event('/b')])}
        onSelect={vi.fn()}
      />,
    )

    const a = screen.getByRole('button', { name: /\/a$/ })
    const b = screen.getByRole('button', { name: /\/b$/ })
    expect(a.style.width).toBe('75%')
    expect(b.style.width).toBe('25%')
    expect(b.style.left).toBe('75%')
  })

  it('collapses entity ids so one endpoint stays one band', () => {
    render(
      <RequestPathMap
        feed={feedOf([
          event('/api/users/3f2a7c1e-0b4d-4f8a-9c6e-1d2b3a4c5d6e'),
          event('/api/users/7c1e3f2a-4f8a-0b4d-1d2b-9c6e5d6e3a4c'),
        ])}
        onSelect={vi.fn()}
      />,
    )

    expect(screen.getByRole('button', { name: /\/api\/users\/\{id\}$/ })).toBeTruthy()
  })

  it('reports the request count and error count it is drawn from', () => {
    render(
      <RequestPathMap
        feed={feedOf([event('/api/x'), event('/api/x', { status: 500 })])}
        onSelect={vi.fn()}
      />,
    )

    const api = screen.getByRole('button', { name: /\/api$/ })
    expect(api.getAttribute('title')).toContain('2 次')
    expect(api.getAttribute('title')).toContain('5xx 1')
  })

  it('hands the clicked route up so the feed below can filter to it', () => {
    const onSelect = vi.fn()
    render(<RequestPathMap feed={feedOf([event('/api/v1/users')])} onSelect={onSelect} />)

    fireEvent.click(screen.getByRole('button', { name: /\/api\/v1$/ }))
    expect(onSelect).toHaveBeenCalledWith('/api/v1')
  })

  it('deselects when the already-selected route is clicked again', () => {
    const onSelect = vi.fn()
    render(
      <RequestPathMap feed={feedOf([event('/api/v1/users')])} selected="/api/v1" onSelect={onSelect} />,
    )

    fireEvent.click(screen.getByRole('button', { name: /\/api\/v1$/ }))
    expect(onSelect).toHaveBeenCalledWith(undefined)
  })

  it('summarises the selected route instead of leaving the reader to hover', () => {
    render(
      <RequestPathMap
        feed={feedOf([
          event('/api/users', { duration_ms: 10 }),
          event('/api/users', { duration_ms: 30, method: 'POST', status: 503 }),
        ])}
        selected="/api/users"
        onSelect={vi.fn()}
      />,
    )

    const summary = screen.getByTestId('path-map-summary')
    expect(summary.textContent).toContain('/api/users')
    expect(summary.textContent).toContain('GET')
    expect(summary.textContent).toContain('POST')
    expect(summary.textContent).toContain('30.0ms')
  })

  it('says it is waiting rather than drawing an empty frame', () => {
    render(<RequestPathMap feed={feedOf([])} onSelect={vi.fn()} />)
    expect(screen.getByText(/等待请求/)).toBeTruthy()
  })
})
