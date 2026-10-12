UPDATE incident_poll_state SET last_failed_at = '-infinity' WHERE last_failed_at = 'epoch';
ALTER TABLE incident_poll_state ALTER COLUMN last_failed_at SET DEFAULT '-infinity';
