-- Refresh sessions.
--
-- One row per issued refresh token. Tokens are stored as SHA-256 digests: the
-- value is a 256-bit random string, not a human password, so a slow hash would
-- add latency to every refresh without adding security. Storing a digest still
-- means a database leak does not hand the attacker usable tokens.
--
-- family_id and parent_id implement the reuse detection described in ADR-0007:
-- a well-behaved client never replays a rotated token, so a second use of one
-- is evidence the token was copied.

CREATE TABLE refresh_sessions (
    id         uuid        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL,
    family_id  uuid        NOT NULL,
    parent_id  uuid        NULL REFERENCES refresh_sessions (id) ON DELETE SET NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    user_agent text        NULL,
    ip         inet        NULL,

    CONSTRAINT refresh_sessions_token_hash_length_check CHECK (octet_length(token_hash) = 32)
);

-- Lookup is always by token digest, and the digest must be unique or two
-- sessions could claim the same token.
CREATE UNIQUE INDEX refresh_sessions_token_hash_key ON refresh_sessions (token_hash);

-- Revoking a whole family on reuse detection, and listing a user's devices,
-- both scan by these columns.
CREATE INDEX refresh_sessions_family_id_idx ON refresh_sessions (family_id);
CREATE INDEX refresh_sessions_user_id_idx ON refresh_sessions (user_id);

-- Expired rows are deleted by a periodic job (Phase 4); this index keeps that
-- sweep from scanning the table.
CREATE INDEX refresh_sessions_expires_at_idx ON refresh_sessions (expires_at);
