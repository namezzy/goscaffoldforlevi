-- The exception_center migration seeded the watcher's checkpoint with
-- '-infinity', which pgx cannot scan into a Go time.Time: every poll failed
-- before reading a single dead letter, so no incident was ever opened.
-- 'epoch' still sorts before any real failure, and is representable.
ALTER TABLE incident_poll_state ALTER COLUMN last_failed_at SET DEFAULT 'epoch';
UPDATE incident_poll_state SET last_failed_at = 'epoch' WHERE last_failed_at = '-infinity';
