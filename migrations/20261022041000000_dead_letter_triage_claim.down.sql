CREATE TABLE incident_poll_state (
    id             boolean     PRIMARY KEY DEFAULT true CHECK (id),
    last_failed_at timestamptz NOT NULL DEFAULT 'epoch'
);

-- Restore the checkpoint from the rows already triaged, so going back does
-- not re-triage the whole table.
INSERT INTO incident_poll_state (id, last_failed_at)
VALUES (true, COALESCE(
    (SELECT max(failed_at) FROM dead_letter_events WHERE incident_processed_at IS NOT NULL),
    'epoch'));

DROP INDEX dead_letter_events_untriaged_idx;
ALTER TABLE dead_letter_events DROP COLUMN incident_processed_at;
