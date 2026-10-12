-- audit_actor_id_no_fk (down)
--
-- Restores the foreign key. Note what this re-enables: deleting a user who
-- was ever an audit actor will once again fail against the append-only
-- trigger, which is the bug this migration fixed.
ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_actor_id_fkey
    FOREIGN KEY (actor_id) REFERENCES users (id) ON DELETE SET NULL;
