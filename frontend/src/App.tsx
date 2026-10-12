import { Navigate, Route, BrowserRouter, Routes } from 'react-router-dom'
import { ProtectedRoute, RequirePermission } from './components/ProtectedRoute'
import { Layout } from './components/Layout'
import { AcceptInvitePage } from './pages/AcceptInvitePage'
import { AuditPage } from './pages/AuditPage'
import { DashboardPage } from './pages/DashboardPage'
import { IncidentDetailPage } from './pages/IncidentDetailPage'
import { IncidentsPage } from './pages/IncidentsPage'
import { ForgotPasswordPage } from './pages/ForgotPasswordPage'
import { InvitationsPage } from './pages/InvitationsPage'
import { LoginPage } from './pages/LoginPage'
import { MemberDetailPage } from './pages/MemberDetailPage'
import { MembersPage } from './pages/MembersPage'
import { NotificationsPage } from './pages/NotificationsPage'
import { OrgPage } from './pages/OrgPage'
import { ProfilePage } from './pages/ProfilePage'
import { ResetPasswordPage } from './pages/ResetPasswordPage'
import { RolesPage } from './pages/RolesPage'
import { SecurityPage } from './pages/SecurityPage'
import { StatusPage } from './pages/StatusPage'
import { TicketDetailPage } from './pages/TicketDetailPage'
import { TicketsPage } from './pages/TicketsPage'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/accept-invite" element={<AcceptInvitePage />} />
        {/* Both unauthenticated by necessity — the caller has no session
            yet, which is the whole point of the flow. See
            api/openapi.yaml's requestPasswordReset/confirmPasswordReset. */}
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
        <Route element={<ProtectedRoute />}>
          <Route element={<Layout />}>
            {/* The post-login landing page for every role — see
                DashboardPage's own role-aware sections for how it differs
                per role without needing separate routes. */}
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/profile" element={<ProfilePage />} />
            {/* No RequirePermission: entirely self-scoped, the same as
                /profile — see SecurityPage's own doc comment. */}
            <Route path="/security" element={<SecurityPage />} />
            <Route path="/status" element={<StatusPage />} />
            <Route
              path="/members"
              element={
                <RequirePermission permission="user:list">
                  <MembersPage />
                </RequirePermission>
              }
            />
            {/* Not gated by RequirePermission like the other routes here:
                the backend allows a member to read their own record by
                ownership regardless of role (AuthorizeSelfOrPermission), so
                a blanket user:read requirement would wrongly block
                visiting one's own detail page. A 403 for someone else's
                record still surfaces through the page's own error state. */}
            <Route path="/members/:id" element={<MemberDetailPage />} />
            <Route
              path="/org"
              element={
                <RequirePermission permission="org:read">
                  <OrgPage />
                </RequirePermission>
              }
            />
            <Route
              path="/invitations"
              element={
                <RequirePermission permission="invite:read">
                  <InvitationsPage />
                </RequirePermission>
              }
            />
            <Route path="/roles" element={<RolesPage />} />
            <Route
              path="/audit"
              element={
                <RequirePermission permission="audit:read">
                  <AuditPage />
                </RequirePermission>
              }
            />
            {/* No RequirePermission: every role holds ticket:create, and
                the list/detail pages' own scope and ownership rules — not a
                blanket permission — decide what each caller sees, the same
                way /members/:id works above. */}
            <Route path="/tickets" element={<TicketsPage />} />
            <Route path="/tickets/:id" element={<TicketDetailPage />} />
            <Route
              path="/incidents"
              element={
                <RequirePermission permission="incident:read">
                  <IncidentsPage />
                </RequirePermission>
              }
            />
            <Route
              path="/incidents/:id"
              element={
                <RequirePermission permission="incident:read">
                  <IncidentDetailPage />
                </RequirePermission>
              }
            />
            <Route path="/notifications" element={<NotificationsPage />} />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
