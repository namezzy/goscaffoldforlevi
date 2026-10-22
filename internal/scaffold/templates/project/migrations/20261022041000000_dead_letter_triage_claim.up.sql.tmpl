-- The exception center claims dead letters per row instead of by timestamp.
--
-- incident_poll_state tracked "the failed_at of the newest dead letter I have
-- triaged" and the poller asked for rows after it. Visibility order does not
-- follow timestamp order: a transaction that stamps failed_at, then commits
-- after a later one has already moved the checkpoint, leaves a row that is
-- permanently invisible to triage — silently, since nothing ever looks back.
--
-- The original objection to a per-row flag was that dead_letter_events reuses
-- one row per (event_id, consumer) across retries, so a flag would survive
-- into a failure it never saw. That is answered where the reuse happens:
-- RecordDeadLetter's ON CONFLICT now clears incident_processed_at, so every
-- fresh failure — new row or reused one — is untriaged again.
ALTER TABLE dead_letter_events ADD COLUMN incident_processed_at timestamptz NULL;

-- Everything the checkpoint had already passed counts as triaged. Rows the
-- old gap skipped cannot be told apart from those, and are not resurrected
-- here: they are triaged again if and when they fail again.
UPDATE dead_letter_events
SET incident_processed_at = failed_at
WHERE failed_at <= (SELECT last_failed_at FROM incident_poll_state);

-- The poller's only query: the oldest untriaged row.
CREATE INDEX dead_letter_events_untriaged_idx
    ON dead_letter_events (failed_at)
    WHERE incident_processed_at IS NULL;

DROP TABLE incident_poll_state;
