-- Invitation queries.

-- name: CreateInvitation :one
INSERT INTO invitations (id, email, department_id, role, token_hash, expires_at, invited_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, email, department_id, role, status, invited_by, accepted_user_id,
          expires_at, accepted_at, revoked_at, created_at, updated_at;

-- name: GetInvitationByID :one
SELECT id, email, department_id, role, status, invited_by, accepted_user_id,
       expires_at, accepted_at, revoked_at, created_at, updated_at
FROM invitations
WHERE id = $1;

-- GetInvitationByTokenHash takes a row lock: Accept always follows it with a
-- conditional update, and two concurrent accepts of the same token must not
-- both see it as pending.
-- name: GetInvitationByTokenHash :one
SELECT id, email, department_id, role, status, invited_by, accepted_user_id,
       expires_at, accepted_at, revoked_at, created_at, updated_at
FROM invitations
WHERE token_hash = $1
FOR UPDATE;

-- ListInvitationsFiltered pages the invitation management list.
-- name: ListInvitationsFiltered :many
SELECT id, email, department_id, role, status, invited_by, accepted_user_id,
       expires_at, accepted_at, revoked_at, created_at, updated_at
FROM invitations
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('email')::text IS NULL OR email = sqlc.narg('email')::text)
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: CountInvitationsFiltered :one
SELECT count(*)
FROM invitations
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('email')::text IS NULL OR email = sqlc.narg('email')::text);

-- MarkInvitationAccepted is the one-time-use guard: the WHERE clause only
-- matches a still-pending row, so a second concurrent accept (or a replay of
-- an old link) updates zero rows instead of racing the first one to
-- provision a second account.
-- name: MarkInvitationAccepted :execrows
UPDATE invitations
SET status = 'accepted', accepted_at = now(), accepted_user_id = $2
WHERE id = $1 AND status = 'pending';

-- name: MarkInvitationExpired :execrows
UPDATE invitations
SET status = 'expired'
WHERE id = $1 AND status = 'pending';

-- name: RevokeInvitation :execrows
UPDATE invitations
SET status = 'revoked', revoked_at = now()
WHERE id = $1 AND status = 'pending';
