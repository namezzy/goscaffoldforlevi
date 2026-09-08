-- Users.
--
-- The email uniqueness constraint lives here rather than in a service-level
-- check. A check-then-insert in application code is a race with a window the
-- width of a network round trip, and under concurrent registration it loses:
-- two requests both see "not taken" and both insert. The database is the only
-- place that can decide this atomically.

CREATE TABLE users (
    id            uuid        PRIMARY KEY,
    email         text        NOT NULL,
    password_hash text        NOT NULL,
    name          text        NOT NULL,
    status        text        NOT NULL,
    role          text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT users_status_check CHECK (status IN ('active', 'disabled')),
    -- The role set is closed and small, so a CHECK is cheaper and clearer than
    -- a lookup table. Widening it later is a one-line migration.
    CONSTRAINT users_role_check CHECK (role IN ('admin', 'user')),
    CONSTRAINT users_email_length_check CHECK (char_length(email) BETWEEN 3 AND 254),
    CONSTRAINT users_name_length_check CHECK (char_length(name) BETWEEN 1 AND 100)
);

-- Emails are normalised to lower case before insertion, so a plain unique
-- index is enough and stays usable for equality lookups.
CREATE UNIQUE INDEX users_email_key ON users (email);

-- The list endpoint orders by (created_at DESC, id DESC); a matching index
-- keeps that from becoming a sort of the whole table.
CREATE INDEX users_created_at_id_idx ON users (created_at DESC, id DESC);

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

COMMENT ON COLUMN users.password_hash IS
    'Argon2id PHC string. Parameters travel with the hash so they can be upgraded per user.';
