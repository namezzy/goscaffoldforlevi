-- System queries: infrastructure, not business data.
--
-- Feature queries live in their own files (queries/user.sql, queries/auth.sql)
-- from Phase 3 onward.

-- ReadinessProbe verifies that the connection can actually execute SQL.
-- pgxpool.Ping only proves a connection exists; it does not prove the session
-- is usable, which is the state a failing-over database leaves behind.
-- name: ReadinessProbe :one
SELECT 1::int AS ok;
