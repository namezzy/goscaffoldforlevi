-- A dead-letter replay puts an event back into the outbox under its original
-- ID, which is the broker's deduplication key, so JetStream dropped a replay
-- inside its duplicate window as a copy of the previous delivery. Counting
-- replays per row lets the publisher key each delivery separately while a
-- relay re-sending the same delivery is still deduplicated.
ALTER TABLE outbox_events ADD COLUMN replay_generation integer NOT NULL DEFAULT 0;
