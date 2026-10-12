-- name: CreatePasswordResetToken :one
INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, token_hash, expires_at, used_at, created_at;

-- GetPasswordResetTokenByHash takes a row lock because confirmation reads
-- the row and then writes it back (marks it used), and two concurrent
-- confirmations of the same token must not both succeed — the same
-- reasoning as GetRefreshSessionByTokenHash.
-- name: GetPasswordResetTokenByHash :one
SELECT id, user_id, token_hash, expires_at, used_at, created_at
FROM password_reset_tokens
WHERE token_hash = $1
FOR UPDATE;

-- MarkPasswordResetTokenUsed is guarded on used_at IS NULL for the same
-- reason RevokeRefreshSession is guarded on revoked_at IS NULL: idempotent,
-- and a zero row count is how the caller notices a race already consumed it.
-- name: MarkPasswordResetTokenUsed :execrows
UPDATE password_reset_tokens SET used_at = now() WHERE id = $1 AND used_at IS NULL;

-- InvalidatePendingPasswordResetTokensForUser removes any earlier
-- unconsumed token before a new one is issued, so requesting a reset twice
-- cannot leave two live links — only the most recent email's link works.
-- name: InvalidatePendingPasswordResetTokensForUser :execrows
DELETE FROM password_reset_tokens WHERE user_id = $1 AND used_at IS NULL;
