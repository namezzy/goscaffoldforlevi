-- Baseline migration: infrastructure only, no business tables.
--
-- Business schema arrives in Phase 3. What lives here is the machinery every
-- later table depends on, so that each of those migrations does not repeat it.

-- Keeping updated_at correct in the database rather than in application code
-- means it stays correct for migrations, admin fixes and any other writer.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION set_updated_at() IS
    'BEFORE UPDATE trigger function that maintains an updated_at column.';
