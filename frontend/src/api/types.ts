// Mirrors the schemas in api/openapi.yaml. Keep in sync by hand — there is no
// generator wired up for this frontend.

export type Role = 'system_admin' | 'member_admin' | 'operator' | 'read_only' | 'auditor'
export type UserStatus = 'active' | 'disabled'

export interface User {
  id: string
  email: string
  /** Absent for an account that has never set one — see api/openapi.yaml. */
  username?: string
  name: string
  status: UserStatus
  role: Role
  /** Absent for a member not yet assigned to a department. */
  department_id?: string
  created_at: string
  updated_at: string
}

export interface PageInfo {
  page: number
  page_size: number
  total: number
  total_pages: number
}

export interface UserPage {
  items: User[]
  page_info: PageInfo
}

export interface CreateUserRequest {
  email: string
  /** Optional. A second way to log in, alongside email. */
  username?: string
  name: string
  password: string
  role?: Role
}

export interface UpdateUserRequest {
  email?: string
  username?: string
  name?: string
  status?: UserStatus
  /** Requires role:assign, distinct from user:update. */
  role?: Role
  /** Reassigns the member's department. Requires user:update, including on your own record. */
  department_id?: string
  /** Unassigns the member's department, moving it back to unset. Takes precedence over department_id. */
  clear_department?: boolean
}

export interface ListUsersParams {
  page?: number
  page_size?: number
  search?: string
  role?: Role
  status?: UserStatus
  sort?: 'created_at' | 'name' | 'email'
  sort_dir?: 'asc' | 'desc'
}

export interface ChangePasswordRequest {
  password: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  token_type: 'Bearer'
  expires_in: number
  /** Identifies which of the account's sessions this pair belongs to — stable across a refresh. */
  session_id: string
}

// --- Security center: sessions & password reset ---

export interface Session {
  id: string
  /** Whether this is the session the current request is itself authenticated through. */
  current: boolean
  created_at: string
  expires_at: string
  /** Absent for a still-active session. */
  revoked_at?: string
  user_agent?: string
  ip?: string
}

export interface SessionList {
  items: Session[]
}

export interface RevokeOtherSessionsResponse {
  sessions_revoked: number
}

export interface RequestPasswordResetRequest {
  identifier: string
}

export interface ConfirmPasswordResetRequest {
  token: string
  password: string
}

export interface HealthStatus {
  status: string
  version?: string
  commit?: string
}

export interface ReadinessStatus {
  status: string
  version?: string
  commit?: string
  components?: Record<string, string>
}

export interface OutboxStatus {
  pending: number
  published: number
  failing: number
  worst_attempts: number
}

export interface DeadLetter {
  id: string
  event_id: string
  event_type: string
  payload: unknown
  consumer: string
  attempts: number
  last_error: string
  failed_at: string
  replayed_at?: string
}

export interface DeadLetterPage {
  items: DeadLetter[]
  /** How many match in total — items is bounded by the request's limit. */
  total: number
}

export interface RequestEvent {
  time: string
  request_id: string
  method: string
  path: string
  status: number
  bytes: number
  duration_ms: number
  /** Absent when it could not be determined. */
  ip?: string
  /** ISO 3166-1 alpha-2. Absent when unresolved. */
  country_code?: string
  /** Localised label; "内网" for a private or loopback address. */
  country_name?: string
}

export interface RequestEventPage {
  items: RequestEvent[]
}

// --- Organization & departments ---

export interface Organization {
  name: string
  /** A short label for the organization — a slug or short code. May be empty. */
  identifier: string
  created_at: string
  updated_at: string
}

/** Both fields optional, but at least one must be present. */
export interface UpdateOrganizationRequest {
  name?: string
  identifier?: string
}

export interface NotificationChannel {
  name: string
  configured: boolean
  detail: string
}

export interface Department {
  id: string
  name: string
  /** Absent for a top-level department. */
  parent_id?: string
  created_at: string
  updated_at: string
}

export interface DepartmentPage {
  items: Department[]
}

export interface CreateDepartmentRequest {
  name: string
  parent_id?: string
}

export interface UpdateDepartmentRequest {
  name?: string
  parent_id?: string
  /** Moves the department to the top level. Takes precedence over parent_id. */
  clear_parent?: boolean
}

// --- Invitations ---

export type InvitationStatus = 'pending' | 'accepted' | 'expired' | 'revoked'

export interface Invitation {
  id: string
  email: string
  department_id?: string
  role: Role
  status: InvitationStatus
  invited_by: string
  accepted_user_id?: string
  expires_at: string
  accepted_at?: string
  revoked_at?: string
  created_at: string
  updated_at: string
}

/** Returned only from create and reinvite — see api/openapi.yaml's CreatedInvitation. */
export interface CreatedInvitation extends Invitation {
  token: string
}

export interface InvitationPage {
  items: Invitation[]
  page_info: PageInfo
}

export interface CreateInvitationRequest {
  email: string
  department_id?: string
  role: Role
}

export interface ListInvitationsParams {
  status?: InvitationStatus
  email?: string
  page?: number
  page_size?: number
}

export interface AcceptInvitationRequest {
  token: string
  name: string
  password: string
}

export interface AcceptInvitationResponse {
  member_id: string
}

// --- Audit ---

export type AuditResult = 'success' | 'failure'

export interface AuditEvent {
  id: string
  actor_id?: string
  action: string
  target_type: string
  target_id: string
  result: AuditResult
  request_id?: string
  before?: unknown
  after?: unknown
  created_at: string
}

export interface AuditEventPage {
  items: AuditEvent[]
  page_info: PageInfo
}

export interface ListAuditEventsParams {
  from?: string
  to?: string
  actor_id?: string
  action?: string
  result?: AuditResult
  target_type?: string
  target_id?: string
  page?: number
  page_size?: number
}

export interface ApiErrorBody {
  error: {
    code: string
    message: string
    details?: { field: string; reason: string }[]
  }
  request_id?: string
}

// --- Tickets ---

export type TicketPriority = 'low' | 'normal' | 'high' | 'urgent'
export type TicketStatus = 'pending' | 'in_progress' | 'needs_info' | 'completed' | 'closed'
/** mine/assigned need no permission beyond ticket:create, held by every role; all requires ticket:read_all. */
export type TicketScope = 'mine' | 'assigned' | 'all'

export interface Ticket {
  id: string
  title: string
  description: string
  category: string
  priority: TicketPriority
  status: TicketStatus
  requester_id: string
  /** Absent until dispatched. */
  assignee_id?: string
  /** Optimistic-concurrency token — pass back as expected_version on every mutating action. */
  version: number
  created_at: string
  updated_at: string
  /** Present once status reaches closed. */
  closed_at?: string
}

export interface TicketPage {
  items: Ticket[]
  page_info: PageInfo
}

export interface TicketStatusOverview {
  /** One entry per status currently present; a status with zero tickets is absent, not zero. */
  counts: Partial<Record<TicketStatus, number>>
}

export interface CreateTicketRequest {
  title: string
  description: string
  category: string
  priority: TicketPriority
}

export interface ListTicketsParams {
  scope?: TicketScope
  status?: TicketStatus
  priority?: TicketPriority
  search?: string
  sort?: 'created_at' | 'priority' | 'status'
  sort_dir?: 'asc' | 'desc'
  page?: number
  page_size?: number
}

export interface AssignTicketRequest {
  assignee_id: string
  expected_version: number
}

export interface TicketNoteRequest {
  note: string
  expected_version: number
}

export interface CompleteTicketRequest {
  note?: string
  expected_version: number
}

export interface CloseTicketRequest {
  expected_version: number
}

// --- Notifications ---

export interface Notification {
  id: string
  type: string
  title: string
  body: string
  target_type: string
  target_id: string
  /** Absent while unread. */
  read_at?: string
  created_at: string
}

export interface NotificationPage {
  items: Notification[]
  page_info: PageInfo
}

export interface ListNotificationsParams {
  unread_only?: boolean
  page?: number
  page_size?: number
}

// --- Workbench ---

export interface DashboardOverview {
  my_open_tickets: number
  assigned_to_me: number
  unread_notifications: number
  /** Present only for a caller holding at least one admin-facing permission — see each field. */
  admin?: DashboardAdminOverview
}

export interface DashboardAdminOverview {
  /** Present when the caller holds ticket:read_all. */
  tickets_by_status?: Partial<Record<TicketStatus, number>>
  /** Present when the caller holds audit:read: the 10 most recent audit events, newest first. */
  recent_audit_events?: AuditEvent[]
  /** Present when the caller holds messaging:read. */
  outbox?: OutboxStatus
}

// --- Incidents (exception center) ---

export type IncidentSeverity = 'info' | 'warning' | 'critical'
export type IncidentStatus = 'open' | 'acknowledged' | 'resolved'
/** "pending" means submitted for replay, never "processed successfully". */
export type IncidentReplayStatus = 'pending' | 'succeeded' | 'failed'

export interface Incident {
  id: string
  fingerprint: string
  severity: IncidentSeverity
  status: IncidentStatus
  /** Absent while nobody owns it. */
  owner_id?: string
  dead_letter_id: string
  event_type: string
  consumer: string
  last_error: string
  occurrence_count: number
  first_seen_at: string
  last_seen_at: string
  acknowledged_at?: string
  acknowledged_by?: string
  resolved_at?: string
  resolved_by?: string
  /** Absent until a replay has been requested. */
  replay_status?: IncidentReplayStatus
  replay_requested_at?: string
  replay_result_at?: string
  created_at: string
  updated_at: string
}

export interface IncidentPage {
  items: Incident[]
  page_info: PageInfo
}

export interface IncidentSummary {
  /** Non-resolved incidents per severity; a severity with none may be absent. */
  counts: Partial<Record<IncidentSeverity, number>>
}

export interface ListIncidentsParams {
  status?: IncidentStatus
  severity?: IncidentSeverity
  owner_id?: string
  search?: string
  page?: number
  page_size?: number
}
