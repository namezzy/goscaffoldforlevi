-- Messaging: the transactional outbox, the consumer dedupe log, and the dead
-- letter table.
--
-- All three exist because delivery is at-least-once and nothing about that is
-- optional (ADR-0009). The outbox makes "the row was written" and "the event
-- was recorded" a single commit; the dedupe log makes a redelivery harmless;
-- the dead letter table keeps a message that can never succeed out of the
-- retry loop while still leaving it visible to a human.

-- The outbox holds events that are committed but not yet delivered.
--
-- Writing here happens inside the caller's transaction, so an event exists if
-- and only if the business change it describes exists. Publishing happens
-- afterwards, from a relay that is free to crash and retry.
CREATE TABLE outbox_events (
    id           uuid        PRIMARY KEY,
    event_type   text        NOT NULL,
    payload      jsonb       NOT NULL,
    traceparent  text        NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz NULL,
    attempts     integer     NOT NULL DEFAULT 0,
    last_error   text        NULL,

    CONSTRAINT outbox_events_event_type_check CHECK (event_type <> '')
);

-- The relay only ever reads unpublished rows, and they are a shrinking
-- minority of the table once delivery is healthy. A partial index keeps the
-- claim query off the published rows entirely, which matters because this
-- table is a write hotspot that is only ever pruned in batches.
CREATE INDEX outbox_events_unpublished_idx
    ON outbox_events (created_at)
    WHERE published_at IS NULL;

-- Pruning delivered rows scans by this column.
CREATE INDEX outbox_events_published_at_idx
    ON outbox_events (published_at)
    WHERE published_at IS NOT NULL;

-- The dedupe log records that a consumer has already handled an event.
--
-- The insert is part of the same transaction as the side effect, which is the
-- only arrangement that actually makes a redelivery a no-op: a separate store,
-- or a separate transaction, can always end up recording one without the
-- other. Redis is deliberately not used here for exactly that reason.
CREATE TABLE processed_events (
    event_id     uuid        NOT NULL,
    consumer     text        NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),

    -- Keyed by both columns: two consumers of the same event have each
    -- processed it independently, and one must not suppress the other.
    PRIMARY KEY (event_id, consumer)
);

-- The retention sweep (30 days, longer than the longest redelivery window)
-- scans by time.
CREATE INDEX processed_events_processed_at_idx ON processed_events (processed_at);

-- Dead letters are messages that exhausted their retries.
--
-- This is a table rather than a broker stream because its purpose is human:
-- counting failures by type, filtering by time, reading the error. A stream
-- can only be replayed in order, which is the wrong shape for triage.
CREATE TABLE dead_letter_events (
    id          uuid        PRIMARY KEY,
    event_id    uuid        NOT NULL,
    event_type  text        NOT NULL,
    payload     jsonb       NOT NULL,
    traceparent text        NULL,
    consumer    text        NOT NULL,
    attempts    integer     NOT NULL,
    last_error  text        NOT NULL,
    failed_at   timestamptz NOT NULL DEFAULT now(),
    replayed_at timestamptz NULL,

    -- One dead letter per event per consumer. A replay that fails again
    -- updates the existing row instead of growing the table, so the count
    -- stays a count of broken messages rather than of attempts.
    CONSTRAINT dead_letter_events_event_consumer_key UNIQUE (event_id, consumer)
);

-- Triage is "what is broken right now", so the outstanding rows get their own
-- index rather than sharing one with the replayed history.
CREATE INDEX dead_letter_events_outstanding_idx
    ON dead_letter_events (failed_at DESC)
    WHERE replayed_at IS NULL;

CREATE INDEX dead_letter_events_event_type_idx ON dead_letter_events (event_type);
