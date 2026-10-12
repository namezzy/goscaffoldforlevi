-- Exception/processing center: turns dead letters into triageable incidents.
--
-- This is deliberately not a second messaging system. An incident stores a
-- reference to the dead letter that caused it (dead_letter_id) plus just
-- enough denormalized context (event_type, consumer, last_error) to render a
-- list without a join — the full payload still lives only in
-- dead_letter_events, read through the existing DLQ endpoints.
--
-- Merge rule: many dead letters can describe the same underlying failure (a
-- broken handler dead-lettering every event of one type from one consumer).
-- fingerprint = consumer + ':' + event_type identifies "this kind of
-- failure," and incidents_fingerprint_active_idx allows at most one
-- non-resolved incident per fingerprint — a repeat failure bumps
-- occurrence_count on the existing incident instead of paging someone again.
-- A fingerprint that failed, got resolved, and later fails again for an
-- unrelated reason opens a new incident: resolution is a claim that the
-- underlying cause was fixed, and a recurrence after that deserves its own
-- record rather than reopening history that was closed.
CREATE TABLE incidents (
    id                   uuid        PRIMARY KEY,
    fingerprint          text        NOT NULL,
    severity             text        NOT NULL DEFAULT 'warning',
    status               text        NOT NULL DEFAULT 'open',
    owner_id             uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    -- The dead letter behind the most recent occurrence. Updated on every
    -- merge, so "open the underlying message" always shows the latest one.
    dead_letter_id       uuid        NOT NULL REFERENCES dead_letter_events(id) ON DELETE RESTRICT,
    event_id             uuid        NOT NULL,
    event_type           text        NOT NULL,
    consumer             text        NOT NULL,
    last_error           text        NOT NULL,
    occurrence_count     integer     NOT NULL DEFAULT 1,
    first_seen_at        timestamptz NOT NULL DEFAULT now(),
    last_seen_at         timestamptz NOT NULL DEFAULT now(),
    acknowledged_at      timestamptz NULL,
    acknowledged_by      uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    resolved_at          timestamptz NULL,
    resolved_by          uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    -- Tracks the REAL outcome of the last replay this incident requested —
    -- see docs/security.md's exception center section. 'pending' must never
    -- be presented as success: "submitted for replay" and "processed
    -- successfully" are different claims.
    replay_status        text        NULL,
    replay_requested_at  timestamptz NULL,
    replay_result_at     timestamptz NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT incidents_severity_check CHECK (severity IN ('info', 'warning', 'critical')),
    CONSTRAINT incidents_status_check CHECK (status IN ('open', 'acknowledged', 'resolved')),
    CONSTRAINT incidents_replay_status_check
        CHECK (replay_status IS NULL OR replay_status IN ('pending', 'succeeded', 'failed'))
);

CREATE UNIQUE INDEX incidents_fingerprint_active_idx ON incidents (fingerprint) WHERE status <> 'resolved';
CREATE INDEX incidents_status_last_seen_idx ON incidents (status, last_seen_at DESC);
CREATE INDEX incidents_dead_letter_id_idx ON incidents (dead_letter_id);

-- incident_poll_state is the background poller's checkpoint: the failed_at
-- of the newest dead letter it has already turned into (or merged into) an
-- incident. A single-row table rather than tracking per-row "have I seen
-- this" flags, because dead_letter_events reuses one row per (event_id,
-- consumer) across retries (see ON CONFLICT in RecordDeadLetter) — a flag on
-- that row would be invalidated the moment the row is reused for a new
-- failure, silently dropping it from triage.
CREATE TABLE incident_poll_state (
    id             boolean     PRIMARY KEY DEFAULT true CHECK (id),
    last_failed_at timestamptz NOT NULL DEFAULT '-infinity'
);

INSERT INTO incident_poll_state (id) VALUES (true);
