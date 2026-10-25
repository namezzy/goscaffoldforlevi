import type { RequestEvent } from '../api/types'

export interface PathNode {
  /** The single URL segment this node stands for; '' for the synthetic root. */
  segment: string
  /** The normalized path from the root down to and including this node. */
  path: string
  /** Requests whose path passes through this node, including deeper ones. */
  count: number
  clientErrors: number
  serverErrors: number
  /** Slowest request seen anywhere under this node. */
  maxMs: number
  totalMs: number
  methods: Record<string, number>
  children: PathNode[]
}

export interface TreeRow {
  node: PathNode
  depth: number
  /** Percentage of the full width, measured from the left edge. */
  offset: number
  width: number
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
const ALL_DIGITS = /^\d+$/
// A bare run of hex long enough to be a token or a hash — but not a word
// like "v1" or "oauth2", which are route names and must stay distinct.
const OPAQUE_HEX = /^[0-9a-f]{16,}$/i

function isIdentifier(segment: string): boolean {
  return UUID.test(segment) || ALL_DIGITS.test(segment) || OPAQUE_HEX.test(segment)
}

/**
 * Reduces a request path to the route it was served by, so that every call
 * to one endpoint lands on one node. Without this the map degenerates into
 * a fan of thousands of leaves, one per entity id, and shows nothing.
 */
export function normalizePath(path: string): string {
  const withoutQuery = path.split(/[?#]/, 1)[0]
  const segments = withoutQuery
    .split('/')
    .filter((s) => s !== '')
    .map((s) => (isIdentifier(s) ? '{id}' : s))
  return segments.length === 0 ? '/' : `/${segments.join('/')}`
}

function emptyNode(segment: string, path: string): PathNode {
  return {
    segment,
    path,
    count: 0,
    clientErrors: 0,
    serverErrors: 0,
    maxMs: 0,
    totalMs: 0,
    methods: {},
    children: [],
  }
}

function tally(node: PathNode, e: RequestEvent): void {
  node.count += 1
  if (e.status >= 500) node.serverErrors += 1
  else if (e.status >= 400) node.clientErrors += 1
  // Totals roll up the whole subtree: a parent's "slowest" has to be the
  // slowest of anything beneath it, or a slow leaf stays invisible until
  // the reader has already drilled down to it.
  node.maxMs = Math.max(node.maxMs, e.duration_ms)
  node.totalMs += e.duration_ms
  node.methods[e.method] = (node.methods[e.method] ?? 0) + 1
}

function sortByTraffic(node: PathNode): void {
  node.children.sort((a, b) => b.count - a.count || a.segment.localeCompare(b.segment))
  for (const child of node.children) sortByTraffic(child)
}

/**
 * Folds a batch of request events into a prefix tree of their paths. The
 * returned root is synthetic: it carries the totals but is not itself a
 * path anyone requested.
 */
export function buildPathTree(events: RequestEvent[]): PathNode {
  const root = emptyNode('', '')

  for (const e of events) {
    const normalized = normalizePath(e.path)
    tally(root, e)
    if (normalized === '/') continue

    let current = root
    let prefix = ''
    for (const segment of normalized.split('/').slice(1)) {
      prefix += `/${segment}`
      let child = current.children.find((c) => c.segment === segment)
      if (!child) {
        child = emptyNode(segment, prefix)
        current.children.push(child)
      }
      tally(child, e)
      current = child
    }
  }

  sortByTraffic(root)
  return root
}

/**
 * Projects the tree onto an icicle layout: one row per depth, each node a
 * horizontal band whose width is its share of the total traffic. Siblings
 * are laid end to end inside their parent's band so a child never escapes
 * the parent it belongs to.
 */
export function flattenTree(root: PathNode, maxDepth = 6): TreeRow[] {
  const rows: TreeRow[] = []
  if (root.count === 0) return rows

  const walk = (node: PathNode, depth: number, offset: number, width: number): void => {
    if (depth >= maxDepth) return
    let cursor = offset
    for (const child of node.children) {
      // Shares are taken against the parent's own count, not the global
      // total, so a parent whose requests stop at it leaves a visible gap
      // rather than having its children stretched to fill the band.
      const childWidth = (child.count / node.count) * width
      rows.push({ node: child, depth, offset: cursor, width: childWidth })
      walk(child, depth + 1, cursor, childWidth)
      cursor += childWidth
    }
  }

  walk(root, 0, 0, 100)
  return rows
}
