import { useEffect, useState } from 'react'
import { ErrorBanner } from '../components/ErrorBanner'
import { Modal } from '../components/Modal'
import { getAuditEvent, listAuditEvents } from '../api/audit'
import {
  Badge,
  Button,
  Field,
  Loading,
  PageHeader,
  Pagination,
  TBody,
  THead,
  Table,
  TableMessage,
  Td,
  Th,
  Tr,
  inputClass,
} from '../components/ui'
import type { AuditEvent, AuditResult } from '../api/types'

const PAGE_SIZE = 20

export function AuditPage() {
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [action, setAction] = useState('')
  const [result, setResult] = useState<AuditResult | ''>('')
  const [page, setPage] = useState(1)

  const [events, setEvents] = useState<AuditEvent[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [selected, setSelected] = useState<AuditEvent | null>(null)

  useEffect(() => {
    setPage(1)
  }, [from, to, action, result])

  useEffect(() => {
    setLoading(true)
    setLoadError(null)
    listAuditEvents({
      from: from ? new Date(from).toISOString() : undefined,
      to: to ? new Date(to).toISOString() : undefined,
      action: action || undefined,
      result: result || undefined,
      page,
      page_size: PAGE_SIZE,
    })
      .then((data) => {
        setEvents(data.items)
        setTotalPages(Math.max(data.page_info.total_pages, 1))
        setTotal(data.page_info.total)
      })
      .catch(setLoadError)
      .finally(() => setLoading(false))
  }, [from, to, action, result, page])

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Audit"
        title={
          <>
            操作审计{' '}
            {total ? <span className="font-mono text-sm font-normal tabular-nums text-slate-400">（共 {total} 条）</span> : null}
          </>
        }
      />

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Field label="起始时间">
          <input type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} className={inputClass} />
        </Field>
        <Field label="结束时间">
          <input type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} className={inputClass} />
        </Field>
        <Field label="动作">
          <input
            value={action}
            onChange={(e) => setAction(e.target.value)}
            placeholder="如 member.role_changed"
            className={inputClass}
          />
        </Field>
        <Field label="结果">
          <select value={result} onChange={(e) => setResult(e.target.value as AuditResult | '')} className={inputClass}>
            <option value="">全部</option>
            <option value="success">成功</option>
            <option value="failure">失败</option>
          </select>
        </Field>
      </div>

      {loadError ? <ErrorBanner error={loadError} /> : null}

      <Table minWidth="42rem">
        <colgroup>
          <col className="w-[20%]" />
          <col className="w-[24%]" />
          <col className="w-[18%]" />
          <col className="w-[18%]" />
          <col className="w-[10%]" />
          <col className="w-[10%]" />
        </colgroup>
        <THead>
          <tr>
            <Th>时间</Th>
            <Th>动作</Th>
            <Th>操作者</Th>
            <Th>目标</Th>
            <Th>结果</Th>
            <Th>详情</Th>
          </tr>
        </THead>
        <TBody>
          {loading ? (
            <TableMessage colSpan={6}>加载中…</TableMessage>
          ) : events.length === 0 ? (
            <TableMessage colSpan={6}>没有匹配的记录</TableMessage>
          ) : (
            events.map((e) => (
              <Tr key={e.id}>
                <Td className="whitespace-nowrap font-mono text-xs tabular-nums text-slate-400">
                  {new Date(e.created_at).toLocaleString('zh-CN')}
                </Td>
                <td className="truncate px-4 py-2.5 font-mono text-xs text-slate-100" title={e.action}>
                  {e.action}
                </td>
                <td className="truncate px-4 py-2.5 font-mono text-xs text-slate-400" title={e.actor_id}>
                  {e.actor_id ?? '系统'}
                </td>
                <td className="truncate px-4 py-2.5 font-mono text-xs text-slate-400" title={`${e.target_type}/${e.target_id}`}>
                  {e.target_type}/{e.target_id.slice(0, 8)}
                </td>
                <Td className="whitespace-nowrap">
                  <Badge tone={e.result === 'success' ? 'green' : 'red'} dot>
                    {e.result === 'success' ? '成功' : '失败'}
                  </Badge>
                </Td>
                <Td>
                  <Button variant="ghost" size="sm" onClick={() => setSelected(e)}>
                    查看
                  </Button>
                </Td>
              </Tr>
            ))
          )}
        </TBody>
      </Table>

      <Pagination page={page} totalPages={totalPages} onChange={setPage} />

      {selected && <AuditDetailModal id={selected.id} onClose={() => setSelected(null)} />}
    </div>
  )
}

function AuditDetailModal({ id, onClose }: { id: string; onClose: () => void }) {
  const [event, setEvent] = useState<AuditEvent | null>(null)
  const [error, setError] = useState<unknown>(null)

  useEffect(() => {
    getAuditEvent(id).then(setEvent).catch(setError)
  }, [id])

  return (
    <Modal title="审计详情" onClose={onClose}>
      {error ? <ErrorBanner error={error} /> : null}
      {!event && !error ? <Loading /> : null}
      {event && (
        <dl className="space-y-3 text-sm">
          <Row label="动作" value={event.action} mono />
          <Row label="操作者" value={event.actor_id ?? '系统'} mono />
          <Row label="目标" value={`${event.target_type} / ${event.target_id}`} mono />
          <Row label="结果" value={event.result === 'success' ? '成功' : '失败'} />
          <Row label="时间" value={new Date(event.created_at).toLocaleString('zh-CN')} mono />
          {event.request_id && <Row label="请求 ID" value={event.request_id} mono />}
          {event.before !== undefined && event.before !== null && (
            <div>
              <dt className="mb-1 font-mono text-[11px] uppercase tracking-wider text-slate-500">变更前</dt>
              <dd>
                <pre className="overflow-x-auto rounded-lg border border-white/[0.06] bg-slate-950/70 p-3 font-mono text-xs text-slate-300">
                  {JSON.stringify(event.before, null, 2)}
                </pre>
              </dd>
            </div>
          )}
          {event.after !== undefined && event.after !== null && (
            <div>
              <dt className="mb-1 font-mono text-[11px] uppercase tracking-wider text-slate-500">变更后</dt>
              <dd>
                <pre className="overflow-x-auto rounded-lg border border-white/[0.06] bg-slate-950/70 p-3 font-mono text-xs text-slate-300">
                  {JSON.stringify(event.after, null, 2)}
                </pre>
              </dd>
            </div>
          )}
        </dl>
      )}
    </Modal>
  )
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="shrink-0 text-slate-500">{label}</dt>
      <dd className={`break-all text-right text-slate-200 ${mono ? 'font-mono text-xs' : ''}`}>{value}</dd>
    </div>
  )
}
