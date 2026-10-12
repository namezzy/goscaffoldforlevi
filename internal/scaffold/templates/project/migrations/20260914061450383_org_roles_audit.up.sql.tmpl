-- org_roles_audit (up)
--
-- Phase 1 of the enterprise-organization feature: a single-tenant
-- organization/department tree, member invitations, and an append-only audit
-- log — plus the role-model change all three lean on.

-- 1. Roles: the old two-role model (admin/user) becomes five fixed roles
-- built around organization membership. Every existing row is remapped so
-- current accounts keep working without any manual fix-up:
--   admin -> system_admin  (same permissions the account already had)
--   user  -> read_only     (the closest existing behaviour: no write access,
--                            plus read access to the member directory, which
--                            is new but not a write capability)
-- The CHECK is dropped before the UPDATE and re-added after, so the widened
-- constraint never has to simultaneously accept both the old and new value
-- sets.
ALTER TABLE users DROP CONSTRAINT users_role_check;

UPDATE users SET role = 'system_admin' WHERE role = 'admin';
UPDATE users SET role = 'read_only' WHERE role = 'user';

ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('system_admin', 'member_admin', 'operator', 'read_only', 'auditor'));

-- 2. Organization: exactly one row. The boolean primary key defaulting to
-- (and constrained to) true is a standard way to enforce a true singleton
-- table in PostgreSQL without a service-layer race between "check" and
-- "insert" — a second INSERT collides on the same key.
CREATE TABLE organizations (
    id         boolean     PRIMARY KEY DEFAULT true CHECK (id),
    name       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT organizations_name_length_check CHECK (char_length(name) BETWEEN 1 AND 200)
);

CREATE TRIGGER organizations_set_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

INSERT INTO organizations (name) VALUES ('My Organization');

-- 3. Departments: a shallow tree via a self-reference. parent_id is checked
-- against forming a cycle at the service layer (a CHECK cannot see other
-- rows); the database's job here is just referential integrity and the
-- length/uniqueness invariants that are checkable per-row.
CREATE TABLE departments (
    id         uuid        PRIMARY KEY,
    name       text        NOT NULL,
    parent_id  uuid        NULL REFERENCES departments (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT departments_name_length_check CHECK (char_length(name) BETWEEN 1 AND 100),
    -- A department cannot be its own parent. Deeper cycles (A -> B -> A) are
    -- not representable as a single-row constraint and are prevented in
    -- org.Service when a department is reparented.
    CONSTRAINT departments_not_self_parent_check CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE INDEX departments_parent_id_idx ON departments (parent_id);

CREATE TRIGGER departments_set_updated_at
    BEFORE UPDATE ON departments
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- 4. Member org assignment. A member belongs to at most one department in
-- this phase — nothing in scope needs many-to-many membership, and adding it
-- later is additive. ON DELETE RESTRICT: deleting a department that still has
-- members would silently orphan them; the service must reassign or empty it
-- first.
ALTER TABLE users ADD COLUMN department_id uuid NULL REFERENCES departments (id) ON DELETE RESTRICT;

CREATE INDEX users_department_id_idx ON users (department_id);

-- 5. Invitations. The token itself is never stored — only its SHA-256
-- digest, the same treatment refresh tokens already get (000003), for the
-- same reason: a database leak must not hand out usable tokens.
CREATE TABLE invitations (
    id                uuid        PRIMARY KEY,
    email             text        NOT NULL,
    department_id     uuid        NULL REFERENCES departments (id) ON DELETE SET NULL,
    role              text        NOT NULL,
    token_hash        bytea       NOT NULL,
    status            text        NOT NULL DEFAULT 'pending',
    invited_by        uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    accepted_user_id  uuid        NULL REFERENCES users (id) ON DELETE SET NULL,
    expires_at        timestamptz NOT NULL,
    accepted_at       timestamptz NULL,
    revoked_at        timestamptz NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT invitations_status_check
        CHECK (status IN ('pending', 'accepted', 'expired', 'revoked')),
    CONSTRAINT invitations_role_check
        CHECK (role IN ('system_admin', 'member_admin', 'operator', 'read_only', 'auditor')),
    CONSTRAINT invitations_email_length_check CHECK (char_length(email) BETWEEN 3 AND 254),
    CONSTRAINT invitations_token_hash_length_check CHECK (octet_length(token_hash) = 32)
);

-- Lookup on accept is always by digest; it must be unique or two pending
-- invitations could resolve to the same token.
CREATE UNIQUE INDEX invitations_token_hash_key ON invitations (token_hash);

-- The management list is almost always "pending invitations for this email"
-- (revoke, reinvite, and the "is there already one outstanding" check before
-- creating a new one) or a status-filtered page ordered by recency.
CREATE INDEX invitations_email_status_idx ON invitations (email, status);
CREATE INDEX invitations_created_at_idx ON invitations (created_at DESC);

CREATE TRIGGER invitations_set_updated_at
    BEFORE UPDATE ON invitations
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- 6. Audit log: append-only by construction, not just by convention. A
-- BEFORE UPDATE/DELETE trigger that raises means "no code path modifies a
-- written record" holds even against a future service method that forgets
-- the rule, a hand-run UPDATE, or a bug — the same defence-in-depth already
-- used for the role and status CHECK constraints that mirror service-layer
-- validation.
CREATE TABLE audit_events (
    id          uuid        PRIMARY KEY,
    actor_id    uuid        NULL REFERENCES users (id) ON DELETE SET NULL,
    action      text        NOT NULL,
    target_type text        NOT NULL,
    target_id   text        NOT NULL,
    result      text        NOT NULL,
    request_id  text        NULL,
    before_data jsonb       NULL,
    after_data  jsonb       NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT audit_events_action_check CHECK (action <> ''),
    CONSTRAINT audit_events_result_check CHECK (result IN ('success', 'failure'))
);

-- The list/detail views filter by any combination of time range, actor,
-- action and result; time-ordered is the common case and the one every other
-- filter narrows from, so it is the leading column.
CREATE INDEX audit_events_created_at_idx ON audit_events (created_at DESC);
CREATE INDEX audit_events_actor_id_idx ON audit_events (actor_id);
CREATE INDEX audit_events_action_idx ON audit_events (action);
CREATE INDEX audit_events_target_idx ON audit_events (target_type, target_id);

CREATE FUNCTION audit_events_immutable() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only: % is not permitted', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_events_no_update
    BEFORE UPDATE ON audit_events
    FOR EACH ROW
    EXECUTE FUNCTION audit_events_immutable();

CREATE TRIGGER audit_events_no_delete
    BEFORE DELETE ON audit_events
    FOR EACH ROW
    EXECUTE FUNCTION audit_events_immutable();
