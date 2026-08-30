-- Columns are listed explicitly everywhere. SELECT * makes the generated
-- struct change shape whenever a migration adds a column, which turns an
-- additive schema change into a compile error at best and a silent behaviour
-- change at worst.

-- name: CreateUser :one
INSERT INTO users (id, email, password_hash, name, status, role)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, email, password_hash, name, status, role, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, email, password_hash, name, status, role, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, name, status, role, created_at, updated_at
FROM users
WHERE email = $1;

-- ListUsers pages with a stable order. created_at alone is not stable: rows
-- created in the same millisecond would come back in arbitrary order and a
-- client paging through them would see duplicates and gaps.
-- name: ListUsers :many
SELECT id, email, password_hash, name, status, role, created_at, updated_at
FROM users
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- UpdateUser applies a partial update. A NULL argument means "leave alone",
-- which is what lets one statement serve PATCH without the repository having
-- to build SQL by string concatenation.
-- name: UpdateUser :one
UPDATE users
SET name          = coalesce(sqlc.narg('name'), name),
    status        = coalesce(sqlc.narg('status'), status),
    role          = coalesce(sqlc.narg('role'), role),
    password_hash = coalesce(sqlc.narg('password_hash'), password_hash)
WHERE id = sqlc.arg('id')
RETURNING id, email, password_hash, name, status, role, created_at, updated_at;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = $1;
