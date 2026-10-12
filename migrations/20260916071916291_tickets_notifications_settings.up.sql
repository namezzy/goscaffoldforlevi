-- tickets_notifications_settings (up)
--
-- Phase 2: a reusable "service request" business (tickets), the in-app
-- notifications that keep people from having to poll for them, and the two
-- new organization-settings fields the workbench's settings page edits.

-- 1. Tickets. There is deliberately no separate activity/timeline table —
-- see ticket.Service's doc comment: the timeline is audit_events rows with
-- target_type = 'ticket', reusing Phase 1's append-only log instead of
-- building a parallel one. What lives here is exactly the state a list/
-- filter/sort query needs indexed columns for.
CREATE TABLE tickets (
    id           uuid        PRIMARY KEY,
    title        text        NOT NULL,
    description  text        NOT NULL,
    category     text        NOT NULL,
    priority     text        NOT NULL,
    status       text        NOT NULL DEFAULT 'pending',
    requester_id uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    -- Unassigned until Assign. ON DELETE SET NULL rather than RESTRICT:
    -- deleting the assignee's account must not block deleting them just
    -- because a ticket still points at them — the ticket falls back to
    -- unassigned, same as a fresh one.
    assignee_id  uuid        NULL REFERENCES users (id) ON DELETE SET NULL,
    -- Optimistic concurrency: every mutating action requires the caller's
    -- last-known version and increments it. A concurrent second writer's
    -- stale version fails the conditional UPDATE instead of silently
    -- overwriting whichever action landed first — see queries/ticket.sql.
    version      integer     NOT NULL DEFAULT 1,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    closed_at    timestamptz NULL,

    CONSTRAINT tickets_title_length_check CHECK (char_length(title) BETWEEN 1 AND 200),
    CONSTRAINT tickets_description_length_check CHECK (char_length(description) BETWEEN 1 AND 8000),
    CONSTRAINT tickets_category_length_check CHECK (char_length(category) BETWEEN 1 AND 100),
    CONSTRAINT tickets_priority_check CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    CONSTRAINT tickets_status_check
        CHECK (status IN ('pending', 'in_progress', 'needs_info', 'completed', 'closed'))
);

-- The three list views (all/mine-as-requester/assigned-to-me) all filter by
-- one of these plus status, and all order by created_at DESC.
CREATE INDEX tickets_requester_id_idx ON tickets (requester_id);
CREATE INDEX tickets_assignee_id_idx ON tickets (assignee_id);
CREATE INDEX tickets_status_idx ON tickets (status);
CREATE INDEX tickets_created_at_id_idx ON tickets (created_at DESC, id DESC);

CREATE TRIGGER tickets_set_updated_at
    BEFORE UPDATE ON tickets
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- 2. Notifications. Written synchronously, in the same transaction as the
-- ticket (or member) action that causes them — not through the outbox —
-- because an in-app unread badge must work in the default deployment,
-- where MESSAGING_ENABLED is off and no worker is running (ADR-0009). The
-- outbox is still what email delivery for the same events rides on; see
-- internal/notify's ticket handler.
CREATE TABLE notifications (
    id          uuid        PRIMARY KEY,
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type        text        NOT NULL,
    title       text        NOT NULL,
    body        text        NOT NULL,
    target_type text        NOT NULL,
    target_id   text        NOT NULL,
    read_at     timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    -- Redelivery/duplicate-trigger protection: the same (recipient, cause)
    -- pair can only ever produce one row. dedup_key is built from the
    -- action that caused the notification (e.g. "ticket:<id>:assigned:v3",
    -- the ticket's post-action version), so a retried request or a race
    -- that ran the same handler twice cannot double-notify.
    dedup_key   text        NOT NULL,

    CONSTRAINT notifications_dedup_key_unique UNIQUE (user_id, dedup_key)
);

CREATE INDEX notifications_user_id_created_at_idx ON notifications (user_id, created_at DESC);

-- The unread badge and the unread-only list both scan just this slice;
-- once read, a notification leaves this index for good.
CREATE INDEX notifications_user_id_unread_idx ON notifications (user_id) WHERE read_at IS NULL;

-- 3. Organization settings additions. name already exists (org_roles_audit
-- migration); identifier is the short code/slug the workbench's settings
-- page adds alongside it.
ALTER TABLE organizations ADD COLUMN identifier text NOT NULL DEFAULT '';
ALTER TABLE organizations ALTER COLUMN identifier DROP DEFAULT;
ALTER TABLE organizations ADD CONSTRAINT organizations_identifier_length_check
    CHECK (char_length(identifier) <= 100);
