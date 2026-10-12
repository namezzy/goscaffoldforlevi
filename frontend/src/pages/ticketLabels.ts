// Presentation-only mirrors of internal/ticket's fixed status/priority sets
// and their labels — kept here, once, so TicketsPage, TicketDetailPage and
// DashboardPage cannot drift into showing different Chinese labels or
// colors for the same status.
import type { Tone } from '../components/ui'
import type { TicketPriority, TicketStatus } from '../api/types'

export const STATUS_LABELS: Record<TicketStatus, string> = {
  pending: '待受理',
  in_progress: '处理中',
  needs_info: '待补充',
  completed: '已完成',
  closed: '已关闭',
}

export const STATUS_TONE: Record<TicketStatus, Tone> = {
  pending: 'cyan',
  in_progress: 'blue',
  needs_info: 'amber',
  completed: 'green',
  closed: 'neutral',
}

export const PRIORITY_LABELS: Record<TicketPriority, string> = {
  low: '低',
  normal: '普通',
  high: '高',
  urgent: '紧急',
}

export const PRIORITY_TONE: Record<TicketPriority, Tone> = {
  low: 'neutral',
  normal: 'cyan',
  high: 'amber',
  urgent: 'red',
}
