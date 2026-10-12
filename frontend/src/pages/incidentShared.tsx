// Presentation-only mirrors of internal/incident's fixed severity, status and
// replay-outcome sets — kept once so IncidentsPage and IncidentDetailPage
// cannot show different labels or colors for the same value.
import type { IncidentReplayStatus, IncidentSeverity, IncidentStatus } from '../api/types'

export const SEVERITIES: IncidentSeverity[] = ['critical', 'warning', 'info']

export const SEVERITY_LABELS: Record<IncidentSeverity, string> = {
  info: '提示',
  warning: '警告',
  critical: '严重',
}

const SEVERITY_BADGE_CLASS: Record<IncidentSeverity, string> = {
  info: 'bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300',
  warning: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300',
  critical: 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300',
}

export const INCIDENT_STATUS_LABELS: Record<IncidentStatus, string> = {
  open: '待处理',
  acknowledged: '已确认',
  resolved: '已解决',
}

const STATUS_BADGE_CLASS: Record<IncidentStatus, string> = {
  open: 'bg-orange-100 text-orange-800 dark:bg-orange-950 dark:text-orange-300',
  acknowledged: 'bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300',
  resolved: 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300',
}

// "pending" is deliberately worded as "submitted", never as success — see
// internal/incident.ReplayStatus.
export const REPLAY_LABELS: Record<IncidentReplayStatus, string> = {
  pending: '已提交重放，等待结果',
  succeeded: '重放成功',
  failed: '重放失败',
}

const REPLAY_BADGE_CLASS: Record<IncidentReplayStatus, string> = {
  pending: 'bg-neutral-200 text-neutral-700 dark:bg-neutral-800 dark:text-neutral-300',
  succeeded: 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300',
  failed: 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300',
}

const badge = 'whitespace-nowrap rounded px-1.5 py-0.5 text-xs font-medium'

export function SeverityBadge({ severity }: { severity: IncidentSeverity }) {
  return <span className={`${badge} ${SEVERITY_BADGE_CLASS[severity]}`}>{SEVERITY_LABELS[severity]}</span>
}

export function IncidentStatusBadge({ status }: { status: IncidentStatus }) {
  return <span className={`${badge} ${STATUS_BADGE_CLASS[status]}`}>{INCIDENT_STATUS_LABELS[status]}</span>
}

export function ReplayBadge({ status }: { status: IncidentReplayStatus }) {
  return <span className={`${badge} ${REPLAY_BADGE_CLASS[status]}`}>{REPLAY_LABELS[status]}</span>
}
