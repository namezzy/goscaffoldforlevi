-- Audit log queries.
--
-- There is deliberately no update or delete query here: the table's own
-- triggers refuse both (see the org_roles_audit migration), and not writing
-- the queries in the first place means there is no generated method that
-- could tempt a caller into trying.

-- name: InsertAuditEvent :one
INSERT INTO audit_events (id, actor_id, action, target_type, target_id, result, request_id, before_data, after_data)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, actor_id, action, target_type, target_id, result, request_id, before_data, after_data, created_at;

-- name: GetAuditEventByID :one
SELECT id, actor_id, action, target_type, target_id, result, request_id, before_data, after_data, created_at
FROM audit_events
WHERE id = $1;

-- ListAuditEventsFiltered backs the audit list view: any combination of a
-- time range, actor, action, result and target. Target (type + id together,
-- since an id is only unambiguous within its type) is what the member
-- detail page uses to show "operations related to this person" — events
-- where they were acted on, not just ones they performed. These six filters
-- use the `(param IS NULL OR ...)` pattern (ADR-0005); six dynamic
-- dimensions is above the point where that pattern is guaranteed to pick a
-- good plan, but audit_events is written far more than it is queried and
-- its indexes cover each filter column individually, so a suboptimal
-- combined plan costs milliseconds, not correctness — this is the "revisit
-- if EXPLAIN shows a full scan" case the ADR calls out, not yet the
-- "hand-write it" case.
-- name: ListAuditEventsFiltered :many
SELECT id, actor_id, action, target_type, target_id, result, request_id, before_data, after_data, created_at
FROM audit_events
WHERE (sqlc.narg('from_time')::timestamptz IS NULL OR created_at >= sqlc.narg('from_time')::timestamptz)
  AND (sqlc.narg('to_time')::timestamptz IS NULL OR created_at <= sqlc.narg('to_time')::timestamptz)
  AND (sqlc.narg('actor_id')::uuid IS NULL OR actor_id = sqlc.narg('actor_id')::uuid)
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action')::text)
  AND (sqlc.narg('result')::text IS NULL OR result = sqlc.narg('result')::text)
  AND (sqlc.narg('target_type')::text IS NULL OR target_type = sqlc.narg('target_type')::text)
  AND (sqlc.narg('target_id')::text IS NULL OR target_id = sqlc.narg('target_id')::text)
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: CountAuditEventsFiltered :one
SELECT count(*)
FROM audit_events
WHERE (sqlc.narg('from_time')::timestamptz IS NULL OR created_at >= sqlc.narg('from_time')::timestamptz)
  AND (sqlc.narg('to_time')::timestamptz IS NULL OR created_at <= sqlc.narg('to_time')::timestamptz)
  AND (sqlc.narg('actor_id')::uuid IS NULL OR actor_id = sqlc.narg('actor_id')::uuid)
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action')::text)
  AND (sqlc.narg('result')::text IS NULL OR result = sqlc.narg('result')::text)
  AND (sqlc.narg('target_type')::text IS NULL OR target_type = sqlc.narg('target_type')::text)
  AND (sqlc.narg('target_id')::text IS NULL OR target_id = sqlc.narg('target_id')::text);
