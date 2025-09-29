-- Drop outbox events table and related objects
DROP TABLE IF EXISTS outbox_events;

-- Drop outbox status enum
DROP TYPE IF EXISTS outbox_status;