-- Columns are listed explicitly everywhere. SELECT * makes the generated
-- struct change shape whenever a migration adds a column, which turns an
-- additive schema change into a compile error at best and a silent behaviour
-- change at worst.
--
-- The lists below put username, then department_id, last, matching their
-- physical position (each was added by a later migration, after
-- created_at/updated_at). sqlc only reuses the generated User model for a
-- query whose column list matches the model's field order; out of order, it
-- would mint a near-duplicate row type per query.

-- name: CreateUser :one
INSERT INTO users (id, email, username, password_hash, name, status, role, department_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, email, password_hash, name, status, role, created_at, updated_at, username, department_id;

-- name: GetUserByID :one
SELECT id, email, password_hash, name, status, role, created_at, updated_at, username, department_id
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, name, status, role, created_at, updated_at, username, department_id
FROM users
WHERE email = $1;

-- GetUserByIdentifier backs login, which accepts either an email address or a
-- username in the same field. Both are normalised to lower case before this
-- runs, so one equality check against each column is enough — there is no
-- need for the caller to guess which kind of identifier it was given.
-- name: GetUserByIdentifier :one
SELECT id, email, password_hash, name, status, role, created_at, updated_at, username, department_id
FROM users
WHERE email = sqlc.arg('identifier') OR username = sqlc.arg('identifier');

-- ListUsers pages with a stable order. created_at alone is not stable: rows
-- created in the same millisecond would come back in arbitrary order and a
-- client paging through them would see duplicates and gaps.
-- name: ListUsers :many
SELECT id, email, password_hash, name, status, role, created_at, updated_at, username, department_id
FROM users
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- ListUsersFiltered is the member-directory search: a substring match across
-- email/username/name, an optional role and status filter, and a choice of
-- sort key and direction.
--
-- The `(param IS NULL OR ...)` shape and the CASE-based ORDER BY are the
-- documented way this project handles dynamic filtering/sorting without a
-- query builder (ADR-0005): sqlc cannot parameterise a column name or a sort
-- direction directly, but exactly one of the six CASE branches is non-NULL
-- for any given call, since sort_by/sort_desc are fixed for the whole query —
-- the other five are NULL for every row and so cannot affect the comparison.
-- id DESC is the final tiebreaker so paging stays stable regardless of which
-- column is sorted on.
-- name: ListUsersFiltered :many
SELECT id, email, password_hash, name, status, role, created_at, updated_at, username, department_id
FROM users
WHERE (sqlc.narg('search')::text IS NULL
        OR email ILIKE '%' || sqlc.narg('search')::text || '%'
        OR username ILIKE '%' || sqlc.narg('search')::text || '%'
        OR name ILIKE '%' || sqlc.narg('search')::text || '%')
  AND (sqlc.narg('role')::text IS NULL OR role = sqlc.narg('role')::text)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY
  CASE WHEN sqlc.arg('sort_by')::text = 'name' AND NOT sqlc.arg('sort_desc')::bool THEN name END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'name' AND sqlc.arg('sort_desc')::bool THEN name END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'email' AND NOT sqlc.arg('sort_desc')::bool THEN email END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'email' AND sqlc.arg('sort_desc')::bool THEN email END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'created_at' AND NOT sqlc.arg('sort_desc')::bool THEN created_at END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'created_at' AND sqlc.arg('sort_desc')::bool THEN created_at END DESC,
  id DESC
LIMIT $1 OFFSET $2;

-- name: CountUsersFiltered :one
SELECT count(*)
FROM users
WHERE (sqlc.narg('search')::text IS NULL
        OR email ILIKE '%' || sqlc.narg('search')::text || '%'
        OR username ILIKE '%' || sqlc.narg('search')::text || '%'
        OR name ILIKE '%' || sqlc.narg('search')::text || '%')
  AND (sqlc.narg('role')::text IS NULL OR role = sqlc.narg('role')::text)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text);

-- UpdateUser applies a partial update. A NULL argument means "leave alone",
-- which is what lets one statement serve PATCH without the repository having
-- to build SQL by string concatenation.
--
-- department_id has a third state beyond "leave alone" / "set to this
-- value": clear_department explicitly unassigns it, matching how
-- departments.parent_id's clear_parent already works (queries/org.sql).
-- coalesce alone cannot express that: a NULL argument is indistinguishable
-- from "not provided".
-- name: UpdateUser :one
UPDATE users
SET email         = coalesce(sqlc.narg('email'), email),
    username      = coalesce(sqlc.narg('username'), username),
    name          = coalesce(sqlc.narg('name'), name),
    status        = coalesce(sqlc.narg('status'), status),
    role          = coalesce(sqlc.narg('role'), role),
    password_hash = coalesce(sqlc.narg('password_hash'), password_hash),
    department_id = CASE WHEN sqlc.arg('clear_department')::bool THEN NULL
                         ELSE coalesce(sqlc.narg('department_id'), department_id) END
WHERE id = sqlc.arg('id')
RETURNING id, email, password_hash, name, status, role, created_at, updated_at, username, department_id;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = $1;

-- CountUsersByRole exists for the bootstrap command, which refuses to run once
-- an administrator already exists. Without that check the command would be a
-- permanent way to mint privileged accounts outside the API, bypassing both
-- authorization and the audit trail.
-- name: CountUsersByRole :one
SELECT count(*) FROM users WHERE role = $1;

-- CountActiveUsersByRole backs the "protect the last system admin" rule: a
-- role change or deactivation that would leave zero active holders of
-- system_admin is refused. Unlike CountUsersByRole it excludes disabled
-- accounts, since a disabled admin cannot act and so does not count as
-- protection against lockout.
--
-- This is the read used outside a transaction (e.g. by cmd/adminctl); it
-- gives no concurrency guarantee by itself. LockActiveUsersByRole below is
-- what the last-system-admin guard actually serializes on.
-- name: CountActiveUsersByRole :one
SELECT count(*) FROM users WHERE role = $1 AND status = 'active';

-- LockActiveUsersByRole is CountActiveUsersByRole's transactional sibling:
-- it must be called from inside the same transaction as the write it
-- guards, and its FOR UPDATE is what makes the last-system-admin check
-- correct under concurrency rather than merely usually-correct.
--
-- FOR UPDATE cannot be combined with an aggregate (count(*)), so this
-- returns the matching rows and the caller counts len(rows) instead. The
-- guarantee comes from PostgreSQL re-evaluating a FOR UPDATE query's WHERE
-- clause against each row's latest committed version once its lock is
-- granted: a second transaction's call to this query blocks on the first's
-- locked rows, and when it resumes it sees whichever of those rows the
-- first transaction's write left still matching role/status — not the
-- stale set it would have seen at the query's start. Two concurrent
-- requests each demoting one of the two remaining system admins therefore
-- serialize correctly instead of both reading "2 remaining" and both
-- proceeding.
-- name: LockActiveUsersByRole :many
SELECT id FROM users WHERE role = $1 AND status = 'active' FOR UPDATE;

-- ListActiveUserIDsByRole is LockActiveUsersByRole without the lock, for
-- callers that only need to know who to notify (the exception center's
-- incident alerts) rather than to guard a concurrent write.
-- name: ListActiveUserIDsByRole :many
SELECT id FROM users WHERE role = $1 AND status = 'active';
