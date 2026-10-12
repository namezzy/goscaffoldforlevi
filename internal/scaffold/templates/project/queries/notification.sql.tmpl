-- Notification queries.

-- CreateNotification is written inside the caller's transaction (the
-- ticket or member action that causes it), and is expected to sometimes
-- conflict on notifications_dedup_key_unique — that is the redelivery/
-- duplicate-trigger guard working as intended, not an error condition the
-- caller should surface, so ON CONFLICT DO NOTHING rather than propagating
-- a unique-violation. :execrows lets the caller tell "created" from
-- "already existed" without treating the latter as a failure.
-- name: CreateNotification :execrows
INSERT INTO notifications (id, user_id, type, title, body, target_type, target_id, dedup_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (user_id, dedup_key) DO NOTHING;

-- name: ListNotificationsFiltered :many
SELECT id, user_id, type, title, body, target_type, target_id, read_at, created_at
FROM notifications
WHERE user_id = sqlc.arg('user_id')
  AND (NOT sqlc.arg('unread_only')::bool OR read_at IS NULL)
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: CountNotificationsFiltered :one
SELECT count(*)
FROM notifications
WHERE user_id = sqlc.arg('user_id')
  AND (NOT sqlc.arg('unread_only')::bool OR read_at IS NULL);

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL;

-- MarkNotificationRead is scoped by user_id, not just id, so one member can
-- never mark another's notification read via a guessed id — the same
-- ownership-in-the-WHERE-clause shape as every other per-user resource in
-- this schema.
-- name: MarkNotificationRead :execrows
UPDATE notifications SET read_at = now() WHERE id = $1 AND user_id = $2 AND read_at IS NULL;

-- name: MarkAllNotificationsRead :execrows
UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL;
