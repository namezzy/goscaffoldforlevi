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
