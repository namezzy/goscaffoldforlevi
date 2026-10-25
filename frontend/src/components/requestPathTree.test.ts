import { describe, expect, it } from 'vitest'
import { buildPathTree, flattenTree, normalizePath } from './requestPathTree'
import type { RequestEvent } from '../api/types'

let seq = 0

function event(path: string, overrides: Partial<RequestEvent> = {}): RequestEvent {
  seq += 1
  return {
    time: new Date(Date.UTC(2026, 9, 26, 0, 0, seq)).toISOString(),
    request_id: `req-${seq}`,
    method: 'GET',
    path,
    status: 200,
    bytes: 10,
    duration_ms: 1,
    ...overrides,
  }
}

describe('normalizePath', () => {
  it('collapses UUID segments so one endpoint is one node', () => {
    expect(normalizePath('/api/v1/users/3f2a7c1e-0b4d-4f8a-9c6e-1d2b3a4c5d6e/sessions')).toBe(
      '/api/v1/users/{id}/sessions',
    )
  })

  it('collapses numeric segments', () => {
    expect(normalizePath('/api/v1/tickets/41821')).toBe('/api/v1/tickets/{id}')
  })

  it('keeps words that merely contain digits', () => {
    // "v1" and "oauth2" are route names, not identifiers — collapsing them
    // would merge every version of the API into one node.
    expect(normalizePath('/api/v1/oauth2/token')).toBe('/api/v1/oauth2/token')
  })

  it('drops the query string and the trailing slash', () => {
    expect(normalizePath('/api/v1/users/?page=2')).toBe('/api/v1/users')
  })

  it('represents the root as a single slash', () => {
    expect(normalizePath('/')).toBe('/')
  })
})

describe('buildPathTree', () => {
  it('nests children under their shared prefix and rolls counts up', () => {
    const root = buildPathTree([
      event('/api/v1/users'),
      event('/api/v1/users'),
      event('/api/v1/tickets'),
    ])

    expect(root.count).toBe(3)
    const api = root.children[0]
    expect(api.segment).toBe('api')
    expect(api.count).toBe(3)

    const v1 = api.children[0]
    const users = v1.children.find((c) => c.segment === 'users')
    expect(users?.count).toBe(2)
    expect(users?.path).toBe('/api/v1/users')
  })

  it('orders siblings by traffic so the busiest path reads first', () => {
    const root = buildPathTree([
      event('/a/one'),
      event('/b/two'),
      event('/b/two'),
      event('/b/two'),
    ])
    expect(root.children.map((c) => c.segment)).toEqual(['b', 'a'])
  })

  it('counts 4xx and 5xx separately, at every level above them', () => {
    const root = buildPathTree([
      event('/api/users', { status: 200 }),
      event('/api/users', { status: 404 }),
      event('/api/users', { status: 503 }),
    ])

    const api = root.children[0]
    expect(api.clientErrors).toBe(1)
    expect(api.serverErrors).toBe(1)
    expect(root.serverErrors).toBe(1)
  })

  it('reports the slowest request seen under a node, not just at it', () => {
    const root = buildPathTree([
      event('/api/users', { duration_ms: 5 }),
      event('/api/users/{id}', { duration_ms: 120 }),
    ])
    expect(root.children[0].maxMs).toBe(120)
  })

  it('tallies methods so one node can show it is both read and written', () => {
    const root = buildPathTree([
      event('/api/users', { method: 'GET' }),
      event('/api/users', { method: 'GET' }),
      event('/api/users', { method: 'POST' }),
    ])
    const users = root.children[0].children[0]
    expect(users.methods).toEqual({ GET: 2, POST: 1 })
  })

  it('treats differently-shaped ids under one route as the same node', () => {
    const root = buildPathTree([
      event('/api/v1/users/3f2a7c1e-0b4d-4f8a-9c6e-1d2b3a4c5d6e'),
      event('/api/v1/users/7c1e3f2a-4f8a-0b4d-1d2b-9c6e5d6e3a4c'),
    ])
    const users = root.children[0].children[0].children[0]
    expect(users.segment).toBe('users')
    expect(users.children).toHaveLength(1)
    expect(users.children[0].segment).toBe('{id}')
    expect(users.children[0].count).toBe(2)
  })

  it('returns an empty root rather than throwing when nothing has arrived', () => {
    const root = buildPathTree([])
    expect(root.count).toBe(0)
    expect(root.children).toEqual([])
  })
})

describe('flattenTree', () => {
  it('lays out each node with a depth and an offset proportional to traffic', () => {
    // Two top-level branches, 3:1 by volume — the second must start exactly
    // where the first ends, or the map's rectangles will not line up.
    const root = buildPathTree([event('/a'), event('/a'), event('/a'), event('/b')])
    const rows = flattenTree(root)

    const a = rows.find((r) => r.node.segment === 'a')!
    const b = rows.find((r) => r.node.segment === 'b')!
    expect(a.depth).toBe(0)
    expect(a.offset).toBeCloseTo(0)
    expect(a.width).toBeCloseTo(75)
    expect(b.offset).toBeCloseTo(75)
    expect(b.width).toBeCloseTo(25)
  })

  it('stops descending past the requested depth', () => {
    const root = buildPathTree([event('/a/b/c/d/e')])
    const rows = flattenTree(root, 2)
    expect(Math.max(...rows.map((r) => r.depth))).toBe(1)
  })

  it('excludes the synthetic root, which is not a path anyone requested', () => {
    const root = buildPathTree([event('/a')])
    expect(flattenTree(root).every((r) => r.node !== root)).toBe(true)
  })
})
