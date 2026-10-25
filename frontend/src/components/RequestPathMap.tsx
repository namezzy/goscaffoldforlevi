import { useMemo } from 'react'
import { buildPathTree, flattenTree, type PathNode, type TreeRow } from './requestPathTree'
import type { RequestFeed } from './useRequestFeed'
import { cardClass } from './ui'

const MAX_DEPTH = 6
const ROW_HEIGHT_PX = 26
// Below this share a band is a sliver no label fits in; it is still drawn
// and still clickable, just not labelled.
const LABEL_MIN_WIDTH = 4

/** Error rate decides the colour: traffic volume is already the width. */
function bandTone(node: PathNode): { bg: string; border: string; text: string } {
  const serverRate = node.serverErrors / node.count
  const errorRate = (node.serverErrors + node.clientErrors) / node.count
  if (serverRate > 0) {
    return { bg: 'bg-rose-500/25', border: 'border-rose-400/40', text: 'text-rose-100' }
  }
  if (errorRate > 0.2) {
    return { bg: 'bg-amber-500/20', border: 'border-amber-400/40', text: 'text-amber-100' }
  }
  return { bg: 'bg-cyan-400/15', border: 'border-cyan-300/25', text: 'text-cyan-50' }
}

function describe(node: PathNode): string {
  const avg = node.totalMs / node.count
  const parts = [
    node.path,
    `${node.count} 次`,
    `平均 ${avg.toFixed(1)}ms`,
    `最慢 ${node.maxMs.toFixed(1)}ms`,
  ]
  if (node.clientErrors > 0) parts.push(`4xx ${node.clientErrors}`)
  if (node.serverErrors > 0) parts.push(`5xx ${node.serverErrors}`)
  return parts.join(' · ')
}

export function RequestPathMap({
  feed,
  selected,
  onSelect,
}: {
  feed: RequestFeed
  selected?: string
  onSelect: (path: string | undefined) => void
}) {
  const tree = useMemo(() => buildPathTree(feed.items), [feed.items])
  const rows = useMemo(() => flattenTree(tree, MAX_DEPTH), [tree])

  const selectedNode = useMemo(
    () => (selected ? (rows.find((r) => r.node.path === selected)?.node ?? null) : null),
    [rows, selected],
  )

  const depth = rows.length === 0 ? 0 : Math.max(...rows.map((r) => r.depth)) + 1

  return (
    <section className={`${cardClass} overflow-hidden`}>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-white/[0.06] px-5 py-3">
        <div>
          <h2 className="text-sm font-semibold text-slate-100">请求路径地图</h2>
          <p className="mt-0.5 text-xs text-slate-500">
            宽度是请求量，颜色是错误率；点击任一段可筛选下方的实时请求流
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3 font-mono text-[11px] tabular-nums text-slate-400">
          <span>
            <span className="text-cyan-300">{tree.count}</span> 次请求
          </span>
          <span>
            <span className="text-cyan-300">{rows.length}</span> 个路径节点
          </span>
          <LegendSwatch className="bg-cyan-400/40" label="正常" />
          <LegendSwatch className="bg-amber-400/50" label="4xx 偏多" />
          <LegendSwatch className="bg-rose-400/50" label="有 5xx" />
        </div>
      </div>

      {rows.length === 0 ? (
        <p className="px-4 py-12 text-center text-sm text-slate-500">
          {feed.status === 'connecting' ? '连接中…' : '等待请求…'}
        </p>
      ) : (
        <div className="px-4 py-4">
          <div className="relative w-full" style={{ height: depth * ROW_HEIGHT_PX }}>
            {rows.map((row) => (
              <Band
                key={row.node.path}
                row={row}
                selected={selected}
                onSelect={onSelect}
              />
            ))}
          </div>
        </div>
      )}

      {selectedNode && <PathSummary node={selectedNode} />}
    </section>
  )
}

function LegendSwatch({ className, label }: { className: string; label: string }) {
  return (
    <span className="flex items-center gap-1.5 text-slate-500">
      <span className={`h-2 w-3 rounded-sm ${className}`} />
      {label}
    </span>
  )
}

function Band({
  row,
  selected,
  onSelect,
}: {
  row: TreeRow
  selected?: string
  onSelect: (path: string | undefined) => void
}) {
  const { node, depth, offset, width } = row
  const tone = bandTone(node)
  const isSelected = selected === node.path
  // An ancestor of the selection stays lit so the drill-down path reads as
  // one continuous trail rather than a single highlighted band in a sea of
  // dimmed ones.
  const onSelectedTrail = selected !== undefined && selected.startsWith(`${node.path}/`)
  const dimmed = selected !== undefined && !isSelected && !onSelectedTrail

  return (
    <button
      type="button"
      // Clicking the current selection again clears it: the map is the only
      // place the filter can be removed from, so it has to be able to.
      onClick={() => onSelect(isSelected ? undefined : node.path)}
      title={describe(node)}
      aria-pressed={isSelected}
      aria-label={node.path}
      className={`absolute overflow-hidden rounded-sm border px-1.5 text-left font-mono text-[11px] leading-none transition ${tone.bg} ${tone.border} ${tone.text} ${
        isSelected ? 'ring-1 ring-cyan-300' : ''
      } ${dimmed ? 'opacity-35' : ''} hover:brightness-125`}
      style={{
        left: `${offset}%`,
        width: `${width}%`,
        top: depth * ROW_HEIGHT_PX,
        height: ROW_HEIGHT_PX - 3,
      }}
    >
      <span className="flex h-full items-center truncate">
        {width >= LABEL_MIN_WIDTH ? node.segment : ''}
      </span>
    </button>
  )
}

function PathSummary({ node }: { node: PathNode }) {
  const avg = node.totalMs / node.count
  const methods = Object.entries(node.methods).sort((a, b) => b[1] - a[1])

  return (
    <div
      data-testid="path-map-summary"
      className="flex flex-wrap items-center gap-x-6 gap-y-2 border-t border-white/[0.06] bg-slate-950/40 px-5 py-3 font-mono text-xs tabular-nums"
    >
      <span className="text-cyan-200">{node.path}</span>
      <Metric label="请求" value={`${node.count}`} />
      <Metric label="平均" value={`${avg.toFixed(1)}ms`} />
      <Metric label="最慢" value={`${node.maxMs.toFixed(1)}ms`} />
      <Metric
        label="4xx"
        value={`${node.clientErrors}`}
        tone={node.clientErrors > 0 ? 'text-amber-300' : undefined}
      />
      <Metric
        label="5xx"
        value={`${node.serverErrors}`}
        tone={node.serverErrors > 0 ? 'text-rose-300' : undefined}
      />
      <span className="flex items-center gap-2 text-slate-500">
        方法
        {methods.map(([m, n]) => (
          <span key={m} className="text-slate-300">
            {m}
            <span className="text-slate-500"> {n}</span>
          </span>
        ))}
      </span>
    </div>
  )
}

function Metric({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <span className="text-slate-500">
      {label} <span className={tone ?? 'text-slate-200'}>{value}</span>
    </span>
  )
}
