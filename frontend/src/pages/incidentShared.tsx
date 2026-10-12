// Presentation-only mirrors of internal/incident's fixed severity, status and
// replay-outcome sets — kept once so IncidentsPage and IncidentDetailPage
// cannot show different labels or colors for the same value.
import { Badge, toneDotClass, type Tone } from '../components/ui'
import { INCIDENT_STATUS_LABELS, REPLAY_LABELS, SEVERITY_LABELS } from './incidentLabels'
import type { IncidentReplayStatus, IncidentSeverity, IncidentStatus } from '../api/types'

const SEVERITY_TONE: Record<IncidentSeverity, Tone> = {
  info: 'cyan',
  warning: 'amber',
  critical: 'red',
}

const STATUS_TONE: Record<IncidentStatus, Tone> = {
  open: 'amber',
  acknowledged: 'blue',
  resolved: 'green',
}

const REPLAY_TONE: Record<IncidentReplayStatus, Tone> = {
  pending: 'amber',
  succeeded: 'green',
  failed: 'red',
}

export function SeverityBadge({ severity }: { severity: IncidentSeverity }) {
  return (
    <Badge tone={SEVERITY_TONE[severity]} dot>
      {SEVERITY_LABELS[severity]}
    </Badge>
  )
}

export function SeverityDot({ severity }: { severity: IncidentSeverity }) {
  return <span className={`h-1.5 w-1.5 rounded-full ${toneDotClass[SEVERITY_TONE[severity]]} shadow-[0_0_8px_currentColor]`} />
}

export function IncidentStatusBadge({ status }: { status: IncidentStatus }) {
  return <Badge tone={STATUS_TONE[status]}>{INCIDENT_STATUS_LABELS[status]}</Badge>
}

export function ReplayBadge({ status }: { status: IncidentReplayStatus }) {
  return <Badge tone={REPLAY_TONE[status]}>{REPLAY_LABELS[status]}</Badge>
}
