-- Ticket queries.
--
-- There is no query here that writes an activity/timeline row: the
-- timeline is audit_events, written through package audit exactly the way
-- every other feature's audit trail is (see internal/ticket's doc comment).

-- name: CreateTicket :one
INSERT INTO tickets (id, title, description, category, priority, requester_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, title, description, category, priority, status, requester_id, assignee_id,
          version, created_at, updated_at, closed_at;

-- name: GetTicketByID :one
SELECT id, title, description, category, priority, status, requester_id, assignee_id,
       version, created_at, updated_at, closed_at
FROM tickets
WHERE id = $1;

-- ListTicketsFiltered backs all three list views (all / mine-as-requester /
-- assigned-to-me): the service passes requester_id or assignee_id only for
-- the scoped views, leaving both NULL for the "all tickets" view a
-- ticket:read_all holder gets. The `(param IS NULL OR ...)` pattern is
-- ADR-0005's documented way to handle this without a query builder.
-- name: ListTicketsFiltered :many
SELECT id, title, description, category, priority, status, requester_id, assignee_id,
       version, created_at, updated_at, closed_at
FROM tickets
WHERE (sqlc.narg('requester_id')::uuid IS NULL OR requester_id = sqlc.narg('requester_id')::uuid)
  AND (sqlc.narg('assignee_id')::uuid IS NULL OR assignee_id = sqlc.narg('assignee_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('priority')::text IS NULL OR priority = sqlc.narg('priority')::text)
  AND (sqlc.narg('search')::text IS NULL
        OR title ILIKE '%' || sqlc.narg('search')::text || '%'
        OR category ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY
  CASE WHEN sqlc.arg('sort_by')::text = 'priority' AND NOT sqlc.arg('sort_desc')::bool THEN priority END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'priority' AND sqlc.arg('sort_desc')::bool THEN priority END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'status' AND NOT sqlc.arg('sort_desc')::bool THEN status END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'status' AND sqlc.arg('sort_desc')::bool THEN status END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'created_at' AND NOT sqlc.arg('sort_desc')::bool THEN created_at END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'created_at' AND sqlc.arg('sort_desc')::bool THEN created_at END DESC,
  id DESC
LIMIT $1 OFFSET $2;

-- name: CountTicketsFiltered :one
SELECT count(*)
FROM tickets
WHERE (sqlc.narg('requester_id')::uuid IS NULL OR requester_id = sqlc.narg('requester_id')::uuid)
  AND (sqlc.narg('assignee_id')::uuid IS NULL OR assignee_id = sqlc.narg('assignee_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('priority')::text IS NULL OR priority = sqlc.narg('priority')::text)
  AND (sqlc.narg('search')::text IS NULL
        OR title ILIKE '%' || sqlc.narg('search')::text || '%'
        OR category ILIKE '%' || sqlc.narg('search')::text || '%');

-- CountTicketsByStatus backs the workbench's ticket overview: one row per
-- status actually present, which is cheaper and simpler than five separate
-- COUNT(*) FILTER columns for statuses that might all be zero.
-- name: CountTicketsByStatus :many
SELECT status, count(*) AS total FROM tickets GROUP BY status;

-- UpdateTicket is every mutating action (assign, reassign, process note,
-- provide info, complete, close): the service always passes the full set
-- of columns that action changes, coalesced against the current row for
-- the rest, and always narrows WHERE by version — see this table's own
-- doc comment in the migration. Zero rows affected means a concurrent
-- writer already moved the ticket to a different version; the service
-- turns that into TICKET_VERSION_CONFLICT rather than silently doing
-- nothing.
-- name: UpdateTicket :one
UPDATE tickets
SET status      = coalesce(sqlc.narg('status'), status),
    assignee_id = CASE WHEN sqlc.arg('clear_assignee')::bool THEN NULL
                       ELSE coalesce(sqlc.narg('assignee_id'), assignee_id) END,
    closed_at   = CASE WHEN sqlc.narg('status')::text = 'closed' THEN now() ELSE closed_at END,
    version     = version + 1
WHERE id = sqlc.arg('id') AND version = sqlc.arg('expected_version')
RETURNING id, title, description, category, priority, status, requester_id, assignee_id,
          version, created_at, updated_at, closed_at;
