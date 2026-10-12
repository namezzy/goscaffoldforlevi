import { Badge } from '../components/ui'
import { PRIORITY_LABELS, PRIORITY_TONE, STATUS_LABELS, STATUS_TONE } from './ticketLabels'
import type { TicketPriority, TicketStatus } from '../api/types'

export { STATUS_LABELS } from './ticketLabels'

export function StatusBadge({ status }: { status: TicketStatus }) {
  return (
    <Badge tone={STATUS_TONE[status]} dot>
      {STATUS_LABELS[status]}
    </Badge>
  )
}

export function PriorityBadge({ priority }: { priority: TicketPriority }) {
  return <Badge tone={PRIORITY_TONE[priority]}>{PRIORITY_LABELS[priority]}</Badge>
}
