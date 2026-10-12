-- name: CreateRefreshSession :one
INSERT INTO refresh_sessions (id, user_id, token_hash, family_id, parent_id, expires_at, user_agent, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, user_id, token_hash, family_id, parent_id, expires_at, revoked_at, created_at, user_agent, ip;

-- GetRefreshSessionByTokenHash takes a row lock because every caller reads the
-- session in order to immediately rotate or revoke it. Without the lock, two
-- concurrent refreshes with the same token both read it as valid and both
-- succeed, which is exactly the replay this table exists to detect.
-- name: GetRefreshSessionByTokenHash :one
SELECT id, user_id, token_hash, family_id, parent_id, expires_at, revoked_at, created_at, user_agent, ip
FROM refresh_sessions
WHERE token_hash = $1
FOR UPDATE;

-- RevokeRefreshSession is idempotent: revoked_at is only set the first time,
-- so re-revoking never moves the timestamp forward.
-- name: RevokeRefreshSession :execrows
UPDATE refresh_sessions
SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshFamily :execrows
UPDATE refresh_sessions
SET revoked_at = now()
WHERE family_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshSessions :execrows
DELETE FROM refresh_sessions WHERE expires_at < now();

-- RevokeAllRefreshSessionsForUser backs member deactivation: every family a
-- user holds is revoked, not just the one behind the current request, so a
-- disabled account cannot keep refreshing a session opened on another
-- device before it was deactivated. It also backs password reset
-- confirmation — see auth.Service.ConfirmPasswordReset.
-- name: RevokeAllRefreshSessionsForUser :execrows
UPDATE refresh_sessions
SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- ListRecentRefreshSessionsByUser backs the security center's session list.
--
-- One row per session family, not per refresh-token rotation: a family is
-- what a user perceives as "one login on one device", but every rotation
-- (internal/auth/service.go's Refresh) inserts a new refresh_sessions row
-- sharing that family_id and revokes the old one. Showing one row per
-- rotation would list the same login several times over, growing with
-- however many times the client happened to refresh. DISTINCT ON collapses
-- each family to its single most recent row (current expiry/revocation
-- state), and the outer ORDER BY/LIMIT then rank and cap the resulting
-- families by recency — a plain ORDER BY/LIMIT before collapsing would rank
-- rotations, not families, and could cap the page at N rows all belonging
-- to the one most-refreshed family.
-- name: ListRecentRefreshSessionsByUser :many
SELECT id, user_id, token_hash, family_id, parent_id, expires_at, revoked_at, created_at, user_agent, ip
FROM (
    SELECT DISTINCT ON (family_id) *
    FROM refresh_sessions
    WHERE user_id = $1
    ORDER BY family_id, created_at DESC
) AS latest_per_family
ORDER BY created_at DESC
LIMIT $2;

-- RevokeRefreshFamilyOwned is the self-service revoke-one-session path — one
-- session meaning one family, per ListRecentRefreshSessionsByUser's doc
-- comment, so this revokes every row in it, not just the latest rotation
-- (RevokeRefreshFamily does the same for the unauthenticated reuse-detection
-- path; this is that operation's ownership-scoped sibling). The WHERE
-- clause scopes by user_id as well as family_id, so a member can never
-- revoke someone else's session by guessing its id, and the row count
-- tells the caller whether anything actually happened (already revoked, or
-- not theirs — the service treats both as "not found", see
-- ConfirmPasswordReset's sibling reasoning on not distinguishing failure
-- causes).
-- name: RevokeRefreshFamilyOwned :execrows
UPDATE refresh_sessions
SET revoked_at = now()
WHERE family_id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- RevokeRefreshSessionsForUserExceptFamily backs "revoke my other sessions":
-- every active session the user holds except the one the current request is
-- authenticated through.
-- name: RevokeRefreshSessionsForUserExceptFamily :execrows
UPDATE refresh_sessions
SET revoked_at = now()
WHERE user_id = $1 AND family_id != $2 AND revoked_at IS NULL;
