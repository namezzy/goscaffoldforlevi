-- security_center (up)
--
-- Phase 3: self-service password reset. Session listing/revocation needs no
-- schema change — refresh_sessions already carries everything the security
-- center's session list shows (id, created_at, user_agent, ip, expires_at,
-- revoked_at) — see internal/auth/account.go's Session type.

-- One-time password reset tokens, following the exact same shape and
-- reasoning as refresh_sessions: only the SHA-256 digest of the token is
-- stored (see auth.HashRefreshToken, reused here), so a database leak does
-- not yield a usable reset link. used_at makes the token single-use —
-- ConfirmPasswordReset's lookup is UPDATE-guarded on it being NULL — and
-- expires_at bounds how long an unused link stays valid.
CREATE TABLE password_reset_tokens (
    id         uuid        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT password_reset_tokens_token_hash_key UNIQUE (token_hash)
);

-- Backs "does this user already have a pending reset" — RequestPasswordReset
-- invalidates any earlier pending token before issuing a new one, so a
-- stale link from an earlier request can never coexist with a fresh one.
-- Partial: only pending (used_at IS NULL) rows are ever queried this way.
CREATE INDEX password_reset_tokens_user_id_pending_idx
    ON password_reset_tokens (user_id)
    WHERE used_at IS NULL;
