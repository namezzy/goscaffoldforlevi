-- Adds an optional username as a second way to identify a user, alongside
-- email. Nullable rather than backfilled: there is no non-arbitrary username
-- to assign an existing account, and PostgreSQL's unique index already
-- treats every NULL as distinct, so any number of accounts can go without
-- one without colliding.
--
-- The index is created inline rather than CONCURRENTLY. That guidance is for
-- a table taking live writes at a size where a SHARE lock would be felt; this
-- one is small enough, and the same table's users_email_key was created the
-- same way in the original migration.

ALTER TABLE users ADD COLUMN username text;

ALTER TABLE users ADD CONSTRAINT users_username_length_check
    CHECK (username IS NULL OR char_length(username) BETWEEN 3 AND 32);

-- Mirrors validate.Username: starts with a letter, then letters, digits,
-- underscore or hyphen. Enforced twice on purpose — the service rejects a
-- malformed value with a field-level 400 before it ever reaches SQL, and this
-- CHECK is the backstop for anything that writes to the table outside the
-- service.
ALTER TABLE users ADD CONSTRAINT users_username_format_check
    CHECK (username IS NULL OR username ~ '^[a-zA-Z][a-zA-Z0-9_-]{2,31}$');

-- Usernames are normalised to lower case before insertion (validate.
-- NormalizeUsername), the same way email already is, so a plain unique index
-- is enough and stays usable for equality lookups.
CREATE UNIQUE INDEX users_username_key ON users (username);
