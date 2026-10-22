-- Messaging queries: the outbox, the consumer dedupe log, and dead letters.

-- InsertOutboxEvent records an event inside the caller's transaction.
--
-- It takes no published_at: a row is unpublished by definition when it is
-- written, and letting a caller set it would allow an event to be born
-- already-delivered.
-- name: InsertOutboxEvent :exec
INSERT INTO outbox_events (id, event_type, payload, traceparent)
VALUES ($1, $2, $3, $4);

-- RequeueOutboxEvent puts a dead-lettered event back into the outbox under
-- its original ID. The delivered row is usually still there (it is kept for
-- the published-retention window), so a plain insert collides with it; this
-- resets that row to unpublished instead, or recreates it if it was pruned.
-- Either way replay_generation moves past every earlier delivery, so the
-- broker does not discard the replay as a duplicate.
-- name: RequeueOutboxEvent :exec
INSERT INTO outbox_events (id, event_type, payload, traceparent, replay_generation)
VALUES ($1, $2, $3, $4, 1)
ON CONFLICT (id) DO UPDATE
SET published_at = NULL, attempts = 0, last_error = NULL,
    replay_generation = outbox_events.replay_generation + 1;

-- ClaimOutboxEvents takes the next batch for delivery.
--
-- FOR UPDATE SKIP LOCKED is what allows more than one relay to run: each
-- claims rows the others are not holding instead of queueing behind them. The
-- rows stay locked until the caller's transaction ends, so a relay that
-- crashes releases its batch rather than stranding it.
--
-- The order is by created_at so that events are delivered roughly in the order
-- they happened. Roughly is the honest word: two transactions can commit in a
-- different order than they inserted, and no ordering guarantee is offered to
-- consumers.
-- name: ClaimOutboxEvents :many
SELECT id, event_type, payload, traceparent, created_at, attempts, replay_generation
FROM outbox_events
WHERE published_at IS NULL
ORDER BY created_at
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- MarkOutboxEventPublished records a successful delivery.
-- name: MarkOutboxEventPublished :exec
UPDATE outbox_events
SET published_at = now(), last_error = NULL
WHERE id = $1;

-- RecordOutboxEventFailure counts a failed delivery attempt.
--
-- The row stays unpublished, so the next sweep picks it up again. attempts is
-- kept for visibility rather than control: an event the broker cannot accept
-- is a broker problem, and dropping it would turn an outage into data loss.
-- name: RecordOutboxEventFailure :exec
UPDATE outbox_events
SET attempts = attempts + 1, last_error = $2
WHERE id = $1;

-- CountUnpublishedOutboxEvents backs the queue-depth metric and the readiness
-- of any operator asking "is the relay keeping up".
-- name: CountUnpublishedOutboxEvents :one
SELECT count(*)::bigint AS pending
FROM outbox_events
WHERE published_at IS NULL;

-- OutboxSummary is the admin API's view of the outbox: the same four numbers
-- `make outbox-status` prints, behind an endpoint rather than a raw SQL
-- connection. worst_attempts is coalesced to 0 because max() over an empty
-- FILTER is NULL, and a NULL "how bad is the worst one" reads as a query
-- failure rather than as "nothing is failing".
-- name: OutboxSummary :one
SELECT
    count(*) FILTER (WHERE published_at IS NULL)::bigint AS pending,
    count(*) FILTER (WHERE published_at IS NOT NULL)::bigint AS published,
    count(*) FILTER (WHERE published_at IS NULL AND attempts > 0)::bigint AS failing,
    coalesce(max(attempts) FILTER (WHERE published_at IS NULL), 0)::int AS worst_attempts
FROM outbox_events;

-- DeletePublishedOutboxEventsBefore prunes delivered rows.
-- name: DeletePublishedOutboxEventsBefore :execrows
DELETE FROM outbox_events
WHERE published_at IS NOT NULL AND published_at < sqlc.arg('cutoff')::timestamptz;

-- ClaimEventForConsumer marks an event as handled by one consumer.
--
-- ON CONFLICT DO NOTHING makes the second delivery of the same event return no
-- rows, which is how a redelivery is recognised. The caller must run this in
-- the same transaction as the side effect: separated, the process can die
-- between them and either lose the side effect or suppress it forever.
-- name: ClaimEventForConsumer :execrows
INSERT INTO processed_events (event_id, consumer)
VALUES ($1, $2)
ON CONFLICT (event_id, consumer) DO NOTHING;

-- DeleteProcessedEventsBefore prunes the dedupe log. The cutoff must stay
-- longer than the longest possible redelivery window, or pruning would make
-- old messages processable again.
-- name: DeleteProcessedEventsBefore :execrows
DELETE FROM processed_events
WHERE processed_at < $1;

-- RecordDeadLetter parks a message that exhausted its retries.
--
-- A repeat failure of the same event updates the existing row so the table
-- counts broken messages rather than attempts at them.
-- name: RecordDeadLetter :exec
INSERT INTO dead_letter_events (
    id, event_id, event_type, payload, traceparent, consumer, attempts, last_error
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (event_id, consumer) DO UPDATE
SET attempts    = EXCLUDED.attempts,
    last_error  = EXCLUDED.last_error,
    failed_at   = now(),
    replayed_at = NULL,
    -- The reused row describes a failure the exception center has not seen,
    -- so it goes back into triage. Leaving the old claim would hide every
    -- repeat failure of an event that already dead-lettered once — see
    -- queries/incident.sql's ClaimUntriagedDeadLetter.
    incident_processed_at = NULL;

-- ListDeadLetters is the triage query behind `make dlq-list`.
-- name: ListDeadLetters :many
SELECT id, event_id, event_type, payload, traceparent, consumer,
       attempts, last_error, failed_at, replayed_at
FROM dead_letter_events
WHERE replayed_at IS NULL
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type')::text)
ORDER BY failed_at DESC
LIMIT $1;

-- CountDeadLetters answers "how many, in total" for the same filter
-- ListDeadLetters uses. A LIMIT-ed list has no way to say that on its own,
-- and reporting len(rows) as the total silently under-counts a queue larger
-- than the page it was asked to hand back.
-- name: CountDeadLetters :one
SELECT count(*) FROM dead_letter_events
WHERE replayed_at IS NULL
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type')::text);

-- GetDeadLetter fetches one parked message for replay.
-- name: GetDeadLetter :one
SELECT id, event_id, event_type, payload, traceparent, consumer,
       attempts, last_error, failed_at, replayed_at
FROM dead_letter_events
WHERE id = $1;

-- MarkDeadLetterReplayed records that a parked message was put back.
-- name: MarkDeadLetterReplayed :execrows
UPDATE dead_letter_events
SET replayed_at = now()
WHERE id = $1 AND replayed_at IS NULL;
