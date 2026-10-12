import { useEffect, useState } from 'react'
import { ErrorBanner } from '../components/ErrorBanner'
import { Modal } from '../components/Modal'
import { getAuditEvent, listAuditEvents } from '../api/audit'
import { inputClass } from './MembersPage'
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
    <div className="space-y-4">
      <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">
        操作审计 {total ? <span className="text-sm font-normal text-neutral-500">（共 {total} 条）</span> : null}
      </h1>

      <div className="flex flex-wrap items-end gap-2">
        <label className="block text-xs text-neutral-500 dark:text-neutral-400">
          起始时间
          <input type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} className={inputClass} />
        </label>
        <label className="block text-xs text-neutral-500 dark:text-neutral-400">
          结束时间
          <input type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} className={inputClass} />
        </label>
        <label className="block text-xs text-neutral-500 dark:text-neutral-400">
          动作
          <input
            value={action}
            onChange={(e) => setAction(e.target.value)}
            placeholder="如 member.role_changed"
            className={inputClass}
          />
        </label>
        <label className="block text-xs text-neutral-500 dark:text-neutral-400">
          结果
          <select value={result} onChange={(e) => setResult(e.target.value as AuditResult | '')} className={inputClass}>
            <option value="">全部</option>
            <option value="success">成功</option>
            <option value="failure">失败</option>
          </select>
        </label>
      </div>

      {loadError ? <ErrorBanner error={loadError} /> : null}

      <div className="overflow-hidden rounded-lg border border-neutral-200 dark:border-neutral-800">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[42rem] table-fixed text-left text-sm">
            <colgroup>
              <col className="w-[20%]" />
              <col className="w-[24%]" />
              <col className="w-[18%]" />
              <col className="w-[18%]" />
              <col className="w-[10%]" />
              <col className="w-[10%]" />
            </colgroup>
            <thead className="bg-neutral-50 text-neutral-500 dark:bg-neutral-900 dark:text-neutral-400">
              <tr>
                <th className="px-4 py-2 font-medium">时间</th>
                <th className="px-4 py-2 font-medium">动作</th>
                <th className="px-4 py-2 font-medium">操作者</th>
                <th className="px-4 py-2 font-medium">目标</th>
                <th className="px-4 py-2 font-medium">结果</th>
                <th className="px-4 py-2 font-medium">详情</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
              {loading ? (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-neutral-400">
                    加载中…
                  </td>
                </tr>
              ) : events.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-neutral-400">
                    没有匹配的记录
                  </td>
                </tr>
              ) : (
                events.map((e) => (
                  <tr key={e.id}>
                    <td className="whitespace-nowrap px-4 py-2 text-neutral-500 dark:text-neutral-400">
                      {new Date(e.created_at).toLocaleString('zh-CN')}
                    </td>
                    <td className="truncate px-4 py-2 text-neutral-900 dark:text-neutral-100" title={e.action}>
                      {e.action}
                    </td>
                    <td className="truncate px-4 py-2 text-neutral-500 dark:text-neutral-400" title={e.actor_id}>
                      {e.actor_id ?? '系统'}
                    </td>
                    <td className="truncate px-4 py-2 text-neutral-500 dark:text-neutral-400" title={`${e.target_type}/${e.target_id}`}>
                      {e.target_type}/{e.target_id.slice(0, 8)}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2">
                      <span className={e.result === 'success' ? 'text-green-700 dark:text-green-400' : 'text-red-600 dark:text-red-400'}>
                        {e.result === 'success' ? '成功' : '失败'}
                      </span>
                    </td>
                    <td className="px-4 py-2">
                      <button onClick={() => setSelected(e)} className="text-xs text-neutral-600 hover:underline dark:text-neutral-300">
                        查看
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-3 text-sm">
          <button
            onClick={() => setPage((p) => Math.max(p - 1, 1))}
            disabled={page <= 1}
            className="rounded-md border border-neutral-300 px-2 py-1 disabled:opacity-40 dark:border-neutral-700"
          >
            上一页
          </button>
          <span className="text-neutral-500">
            第 {page} / {totalPages} 页
          </span>
          <button
            onClick={() => setPage((p) => Math.min(p + 1, totalPages))}
            disabled={page >= totalPages}
            className="rounded-md border border-neutral-300 px-2 py-1 disabled:opacity-40 dark:border-neutral-700"
          >
            下一页
          </button>
        </div>
      )}

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
      {!event && !error ? <p className="text-sm text-neutral-400">加载中…</p> : null}
      {event && (
        <dl className="space-y-3 text-sm">
          <Row label="动作" value={event.action} />
          <Row label="操作者" value={event.actor_id ?? '系统'} />
          <Row label="目标" value={`${event.target_type} / ${event.target_id}`} />
          <Row label="结果" value={event.result === 'success' ? '成功' : '失败'} />
          <Row label="时间" value={new Date(event.created_at).toLocaleString('zh-CN')} />
          {event.request_id && <Row label="请求 ID" value={event.request_id} />}
          {event.before !== undefined && event.before !== null && (
            <div>
              <dt className="mb-1 text-neutral-500 dark:text-neutral-400">变更前</dt>
              <dd>
                <pre className="overflow-x-auto rounded-md bg-neutral-50 p-2 text-xs dark:bg-neutral-900">
                  {JSON.stringify(event.before, null, 2)}
                </pre>
              </dd>
            </div>
          )}
          {event.after !== undefined && event.after !== null && (
            <div>
              <dt className="mb-1 text-neutral-500 dark:text-neutral-400">变更后</dt>
              <dd>
                <pre className="overflow-x-auto rounded-md bg-neutral-50 p-2 text-xs dark:bg-neutral-900">
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

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-neutral-500 dark:text-neutral-400">{label}</dt>
      <dd className="text-right break-all text-neutral-900 dark:text-neutral-100">{value}</dd>
    </div>
  )
}
