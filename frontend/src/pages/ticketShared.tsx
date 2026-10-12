// Presentation-only mirrors of internal/ticket's fixed status/priority sets
// and their labels — kept here, once, so TicketsPage, TicketDetailPage and
// DashboardPage cannot drift into showing different Chinese labels or
// colors for the same status.
import type { TicketPriority, TicketStatus } from '../api/types'

export const STATUS_LABELS: Record<TicketStatus, string> = {
  pending: '待受理',
  in_progress: '处理中',
  needs_info: '待补充',
  completed: '已完成',
  closed: '已关闭',
}

export const STATUS_BADGE_CLASS: Record<TicketStatus, string> = {
  pending: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300',
  in_progress: 'bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300',
  needs_info: 'bg-orange-100 text-orange-800 dark:bg-orange-950 dark:text-orange-300',
  completed: 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300',
  closed: 'bg-neutral-200 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400',
}

export const PRIORITY_LABELS: Record<TicketPriority, string> = {
  low: '低',
  normal: '普通',
  high: '高',
  urgent: '紧急',
}

export const PRIORITY_BADGE_CLASS: Record<TicketPriority, string> = {
  low: 'bg-neutral-100 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400',
  normal: 'bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300',
  high: 'bg-orange-100 text-orange-800 dark:bg-orange-950 dark:text-orange-300',
  urgent: 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300',
}

export function StatusBadge({ status }: { status: TicketStatus }) {
  return (
    <span className={`whitespace-nowrap rounded px-1.5 py-0.5 text-xs font-medium ${STATUS_BADGE_CLASS[status]}`}>
      {STATUS_LABELS[status]}
    </span>
  )
}

export function PriorityBadge({ priority }: { priority: TicketPriority }) {
  return (
    <span className={`whitespace-nowrap rounded px-1.5 py-0.5 text-xs font-medium ${PRIORITY_BADGE_CLASS[priority]}`}>
      {PRIORITY_LABELS[priority]}
    </span>
  )
}
