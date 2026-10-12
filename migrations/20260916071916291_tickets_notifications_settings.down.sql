-- tickets_notifications_settings (down)

ALTER TABLE organizations DROP CONSTRAINT organizations_identifier_length_check;
ALTER TABLE organizations DROP COLUMN identifier;

DROP TABLE IF EXISTS notifications;

DROP TRIGGER IF EXISTS tickets_set_updated_at ON tickets;
DROP TABLE IF EXISTS tickets;
