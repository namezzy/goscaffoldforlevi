-- Organization and department queries.

-- Column order matches the tables' physical column order (identifier was
-- added last, by the tickets_notifications_settings migration) so sqlc
-- reuses the shared Organization model instead of minting a per-query row
-- type — see queries/user.sql's header comment for the same rule.
-- GetOrganization reads the one row the singleton table ever holds.
-- name: GetOrganization :one
SELECT id, name, created_at, updated_at, identifier FROM organizations WHERE id = true;

-- name: UpdateOrganization :one
UPDATE organizations
SET name       = coalesce(sqlc.narg('name'), name),
    identifier = coalesce(sqlc.narg('identifier'), identifier)
WHERE id = true
RETURNING id, name, created_at, updated_at, identifier;

-- name: CreateDepartment :one
INSERT INTO departments (id, name, parent_id)
VALUES ($1, $2, $3)
RETURNING id, name, parent_id, created_at, updated_at;

-- name: GetDepartmentByID :one
SELECT id, name, parent_id, created_at, updated_at FROM departments WHERE id = $1;

-- ListDepartments returns the whole tree. The department count in a
-- single-tenant deployment is small enough (dozens, not thousands) that
-- paging it would be solving a problem this phase does not have; the client
-- builds the tree from the flat list.
-- name: ListDepartments :many
SELECT id, name, parent_id, created_at, updated_at
FROM departments
ORDER BY name, id;

-- LockDepartmentsForReparenting locks every department row for the duration
-- of the caller's transaction, so two concurrent reparent operations
-- serialize instead of each checking the tree against a snapshot the other
-- is simultaneously invalidating — the same class of race
-- LockActiveUsersByRole (queries/user.sql) closes for the last-system-admin
-- guard, applied here to department A being reparented under B at the same
-- moment B is reparented under A: each transaction's own cycle check would
-- pass against the pre-move tree, and both would commit, leaving a real
-- cycle neither check ever saw. Locking the whole table is coarse, but
-- appropriate at this scale — see ListDepartments' own reasoning for why
-- this table is never expected to hold more than a few dozen rows.
-- name: LockDepartmentsForReparenting :many
SELECT id, name, parent_id, created_at, updated_at
FROM departments
FOR UPDATE;

-- name: UpdateDepartment :one
UPDATE departments
SET name      = coalesce(sqlc.narg('name'), name),
    parent_id = CASE WHEN sqlc.arg('clear_parent')::bool THEN NULL ELSE coalesce(sqlc.narg('parent_id'), parent_id) END
WHERE id = sqlc.arg('id')
RETURNING id, name, parent_id, created_at, updated_at;

-- name: DeleteDepartment :execrows
DELETE FROM departments WHERE id = $1;

-- CountChildDepartments and CountDepartmentMembers back the two guards a
-- delete (and, for the child count, a reparent) must pass: a department that
-- still has sub-departments or assigned members cannot be removed, since
-- either would either violate the FK (ON DELETE RESTRICT already refuses it
-- at the database level) or, worse, silently orphan members if that
-- constraint were ever loosened. The service checks these first so the
-- caller gets a clear conflict instead of a raw constraint-violation
-- message.
-- name: CountChildDepartments :one
SELECT count(*) FROM departments WHERE parent_id = $1;

-- name: CountDepartmentMembers :one
SELECT count(*) FROM users WHERE department_id = $1;
