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

export const INCIDENT_STATUS_LABELS: Record<IncidentStatus, string> = {
  open: '待处理',
  acknowledged: '已确认',
  resolved: '已解决',
}

// "pending" is deliberately worded as "submitted", never as success — see
// internal/incident.ReplayStatus.
export const REPLAY_LABELS: Record<IncidentReplayStatus, string> = {
  pending: '已提交重放，等待结果',
  succeeded: '重放成功',
  failed: '重放失败',
}
