-- Exception/processing center: incidents merged from dead letters, and the
-- poller's claim on the dead letters it has triaged. See the
-- exception_center migration for the fingerprint merge rule and
-- dead_letter_triage_claim for why triage is a per-row claim.

-- ClaimUntriagedDeadLetter takes the oldest dead letter no incident has been
-- opened or merged for yet, and holds it for the rest of the transaction.
--
-- Per-row rather than against a timestamp checkpoint: a transaction that
-- commits after a later one would leave a row behind a checkpoint that had
-- already moved past its failed_at, skipping it permanently (see the
-- dead_letter_triage_claim migration). FOR UPDATE marks the row as being
-- worked on; SKIP LOCKED lets a second watcher replica take the next one
-- instead of queueing behind the first.
-- name: ClaimUntriagedDeadLetter :one
SELECT id, event_id, event_type, consumer, last_error, failed_at
FROM dead_letter_events
WHERE incident_processed_at IS NULL
ORDER BY failed_at
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- MarkDeadLetterTriaged commits that this dead letter has been turned into
-- or merged into an incident. Runs in the same transaction as that write, so
-- a crash in between re-triages the row rather than dropping it.
-- name: MarkDeadLetterTriaged :exec
UPDATE dead_letter_events SET incident_processed_at = now() WHERE id = $1;

-- GetOpenIncidentByFingerprint finds the one non-resolved incident for this
-- kind of failure, if any — the merge target. FOR UPDATE so a concurrent
-- operator action (ack, resolve, replay) on the same incident serializes
-- with the poller's bump instead of racing it.
-- name: GetOpenIncidentByFingerprint :one
SELECT id, fingerprint, severity, status, owner_id, dead_letter_id, event_id, event_type,
       consumer, last_error, occurrence_count, first_seen_at, last_seen_at,
       acknowledged_at, acknowledged_by, resolved_at, resolved_by,
       replay_status, replay_requested_at, replay_result_at, created_at, updated_at,
       replay_event_id
FROM incidents
WHERE fingerprint = $1 AND status <> 'resolved'
FOR UPDATE;

-- CreateIncident opens a new incident for a fingerprint that has no
-- non-resolved one yet.
-- name: CreateIncident :one
INSERT INTO incidents (
    id, fingerprint, dead_letter_id, event_id, event_type, consumer, last_error,
    first_seen_at, last_seen_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $8
)
RETURNING id, fingerprint, severity, status, owner_id, dead_letter_id, event_id, event_type,
       consumer, last_error, occurrence_count, first_seen_at, last_seen_at,
       acknowledged_at, acknowledged_by, resolved_at, resolved_by,
       replay_status, replay_requested_at, replay_result_at, created_at, updated_at,
       replay_event_id;

-- MergeIncidentOccurrence records a repeat failure onto an existing
-- incident: bump the count, move the pointer to the newest dead letter and
-- its event, and refresh the error text so the list shows what is failing
-- right now rather than what failed first. event_id must move together with
-- dead_letter_id — the replay-result resolver checks processed_events by
-- event_id, and leaving it pointed at a stale occurrence would make that
-- check answer the wrong question.
-- name: MergeIncidentOccurrence :exec
UPDATE incidents
SET occurrence_count = occurrence_count + 1,
    last_seen_at      = $2,
    dead_letter_id    = $3,
    event_id          = $4,
    last_error        = $5,
    updated_at        = now()
WHERE id = $1;

-- GetIncident fetches one incident for detail/actions. FOR UPDATE variant is
-- GetIncidentForUpdate below; this one is for read-only access.
-- name: GetIncident :one
SELECT id, fingerprint, severity, status, owner_id, dead_letter_id, event_id, event_type,
       consumer, last_error, occurrence_count, first_seen_at, last_seen_at,
       acknowledged_at, acknowledged_by, resolved_at, resolved_by,
       replay_status, replay_requested_at, replay_result_at, created_at, updated_at,
       replay_event_id
FROM incidents
WHERE id = $1;

-- GetIncidentForUpdate is GetIncident, row-locked for a mutating action.
-- name: GetIncidentForUpdate :one
SELECT id, fingerprint, severity, status, owner_id, dead_letter_id, event_id, event_type,
       consumer, last_error, occurrence_count, first_seen_at, last_seen_at,
       acknowledged_at, acknowledged_by, resolved_at, resolved_by,
       replay_status, replay_requested_at, replay_result_at, created_at, updated_at,
       replay_event_id
FROM incidents
WHERE id = $1
FOR UPDATE;

-- ListIncidentsFiltered is the exception center's list view, newest activity
-- first. Every filter is optional (NULL = no constraint on that dimension).
-- name: ListIncidentsFiltered :many
SELECT id, fingerprint, severity, status, owner_id, dead_letter_id, event_id, event_type,
       consumer, last_error, occurrence_count, first_seen_at, last_seen_at,
       acknowledged_at, acknowledged_by, resolved_at, resolved_by,
       replay_status, replay_requested_at, replay_result_at, created_at, updated_at,
       replay_event_id
FROM incidents
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity')::text)
  AND (sqlc.narg('owner_id')::uuid IS NULL OR owner_id = sqlc.narg('owner_id')::uuid)
  AND (
    sqlc.narg('search')::text IS NULL
    OR event_type ILIKE '%' || sqlc.narg('search')::text || '%'
    OR consumer ILIKE '%' || sqlc.narg('search')::text || '%'
  )
ORDER BY last_seen_at DESC
LIMIT $1 OFFSET $2;

-- name: CountIncidentsFiltered :one
SELECT count(*) FROM incidents
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity')::text)
  AND (sqlc.narg('owner_id')::uuid IS NULL OR owner_id = sqlc.narg('owner_id')::uuid)
  AND (
    sqlc.narg('search')::text IS NULL
    OR event_type ILIKE '%' || sqlc.narg('search')::text || '%'
    OR consumer ILIKE '%' || sqlc.narg('search')::text || '%'
  );

-- CountOpenIncidentsBySeverity backs the exception center's summary strip.
-- name: CountOpenIncidentsBySeverity :many
SELECT severity, count(*)::bigint AS total
FROM incidents
WHERE status <> 'resolved'
GROUP BY severity;

-- SetIncidentSeverity lets an operator re-triage a merged incident.
-- name: SetIncidentSeverity :exec
UPDATE incidents SET severity = $2, updated_at = now() WHERE id = $1;

-- AssignIncidentOwner sets or clears (via NULL) who is handling this.
-- name: AssignIncidentOwner :exec
UPDATE incidents SET owner_id = $2, updated_at = now() WHERE id = $1;

-- AcknowledgeIncident marks the incident seen without resolving it. Returns
-- 0 rows changed when the incident was not open — the caller's signal that
-- the transition did not apply.
-- name: AcknowledgeIncident :execrows
UPDATE incidents
SET status = 'acknowledged', acknowledged_at = now(), acknowledged_by = $2, updated_at = now()
WHERE id = $1 AND status = 'open';

-- ResolveIncident closes the incident out. Resolving is what lets a later,
-- unrelated recurrence of the same fingerprint open a fresh incident instead
-- of reopening this one's history. Returns 0 rows changed when it was
-- already resolved.
-- name: ResolveIncident :execrows
UPDATE incidents
SET status = 'resolved', resolved_at = now(), resolved_by = $2, updated_at = now()
WHERE id = $1 AND status <> 'resolved';

-- RequestIncidentReplay records that a replay was submitted — not that it
-- succeeded — together with the event it submitted. See ResolveIncidentReplay
-- for how the real result is learned, and the replay_event_id migration for
-- why the event is stamped here rather than read back off event_id later.
-- name: RequestIncidentReplay :exec
UPDATE incidents
SET replay_status = 'pending', replay_requested_at = now(), replay_result_at = NULL,
    replay_event_id = $2, updated_at = now()
WHERE id = $1;

-- ResolveIncidentReplay records the real outcome of the last requested
-- replay.
-- name: ResolveIncidentReplay :exec
UPDATE incidents
SET replay_status = $2, replay_result_at = now(), updated_at = now()
WHERE id = $1 AND replay_status = 'pending';

-- ListPendingReplayIncidents is the replay-result resolver's work queue.
-- name: ListPendingReplayIncidents :many
SELECT id, fingerprint, severity, status, owner_id, dead_letter_id, event_id, event_type,
       consumer, last_error, occurrence_count, first_seen_at, last_seen_at,
       acknowledged_at, acknowledged_by, resolved_at, resolved_by,
       replay_status, replay_requested_at, replay_result_at, created_at, updated_at,
       replay_event_id
FROM incidents
WHERE replay_status = 'pending'
LIMIT $1;

-- ExistsProcessedEventSince is the real "did the replay actually succeed"
-- signal: the dedupe log only gains a row for (event_id, consumer) once the
-- consumer's handler has actually run to completion (see
-- messaging.Consumer.attempt — the claim and the handler share a
-- transaction, so a failed attempt never commits a row here).
-- name: ExistsProcessedEventSince :one
SELECT EXISTS(
    SELECT 1 FROM processed_events
    WHERE event_id = $1 AND consumer = $2 AND processed_at > $3
) AS processed;

-- ExistsDeadLetterSince is the matching "did the replay fail again" signal.
-- dead_letter_events keeps one row per (event_id, consumer) — a repeat
-- failure updates that row's failed_at (see RecordDeadLetter's ON CONFLICT)
-- — so a failed_at past the replay request means this very event was retried
-- and dead-lettered again, rather than some other event sharing the
-- incident's fingerprint having failed.
-- name: ExistsDeadLetterSince :one
SELECT EXISTS(
    SELECT 1 FROM dead_letter_events
    WHERE event_id = $1 AND consumer = $2 AND failed_at > $3
) AS dead_lettered;
