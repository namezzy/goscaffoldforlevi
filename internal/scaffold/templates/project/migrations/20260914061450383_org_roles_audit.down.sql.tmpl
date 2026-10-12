-- org_roles_audit (down)
--
-- Reverses the up migration. Running this after any invitation, department
-- or audit row has been created discards it — audit_events especially, whose
-- entire purpose is durability, so this path is for local work and CI only,
-- never production (ADR-0005).

DROP TRIGGER IF EXISTS audit_events_no_delete ON audit_events;
DROP TRIGGER IF EXISTS audit_events_no_update ON audit_events;
DROP FUNCTION IF EXISTS audit_events_immutable();
DROP TABLE IF EXISTS audit_events;

DROP TABLE IF EXISTS invitations;

ALTER TABLE users DROP COLUMN department_id;

DROP TABLE IF EXISTS departments;
DROP TABLE IF EXISTS organizations;

ALTER TABLE users DROP CONSTRAINT users_role_check;

UPDATE users SET role = 'admin' WHERE role = 'system_admin';
UPDATE users SET role = 'user' WHERE role IN ('member_admin', 'operator', 'read_only', 'auditor');

ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'user'));
