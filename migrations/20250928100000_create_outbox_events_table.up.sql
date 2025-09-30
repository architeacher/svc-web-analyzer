-- Create an outbox status enum for message processing states
CREATE TYPE outbox_status AS ENUM ('pending', 'processing', 'published', 'failed');

-- Create the outbox events table for reliable message delivery using an outbox pattern
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL, -- references analysis.id or other aggregates
    aggregate_type VARCHAR(50) NOT NULL DEFAULT 'analysis',
    event_type VARCHAR(100) NOT NULL, -- e.g. 'analysis.requested', 'analysis.retry'
    priority analysis_priority NOT NULL DEFAULT 'normal',
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    status outbox_status NOT NULL DEFAULT 'pending',
    payload JSONB NOT NULL,
    error_details TEXT,

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ
);

-- Create indexes for efficient outbox processing
-- Primary index for finding pending events to process
CREATE INDEX idx_outbox_pending ON outbox_events(status, priority, created_at)
    WHERE status = 'pending';

-- Index for finding failed events ready for retry
CREATE INDEX idx_outbox_retry ON outbox_events(status, next_retry_at)
    WHERE status = 'failed' AND next_retry_at IS NOT NULL;

-- Index for finding events by aggregate for debugging/monitoring
CREATE INDEX idx_outbox_aggregate ON outbox_events(aggregate_id, aggregate_type);

-- Index for event type queries and monitoring
CREATE INDEX idx_outbox_event_type ON outbox_events(event_type, created_at);

-- Index for published events (for cleanup/archival)
CREATE INDEX idx_outbox_published ON outbox_events(published_at)
    WHERE status = 'published';

-- GIN index for efficient JSON queries on payload
CREATE INDEX idx_outbox_payload_gin ON outbox_events USING GIN(payload);

-- Partial indexes for archival and cleanup operations
CREATE INDEX idx_outbox_expired ON outbox_events(expires_at)
    WHERE expires_at IS NOT NULL AND archived_at IS NULL;
CREATE INDEX idx_outbox_archived ON outbox_events(archived_at)
    WHERE archived_at IS NOT NULL;

-- Add table and column comments for documentation
COMMENT ON TABLE outbox_events IS 'Outbox pattern table for reliable message delivery to message queues. Timeline: created_at → started_at (Publisher) → published_at → processed_at (Subscriber) → completed_at';
COMMENT ON COLUMN outbox_events.id IS 'Primary key UUID identifier (application generates UUID V5 from aggregate_id + event_type + created_at)';
COMMENT ON COLUMN outbox_events.aggregate_id IS 'ID of the domain aggregate this event relates to';
COMMENT ON COLUMN outbox_events.aggregate_type IS 'Type of domain aggregate (analysis, user, etc.)';
COMMENT ON COLUMN outbox_events.event_type IS 'Type of event for message routing (e.g., analysis.requested)';
COMMENT ON COLUMN outbox_events.priority IS 'Processing priority for message queue ordering';
COMMENT ON COLUMN outbox_events.retry_count IS 'Number of retry attempts for failed message publishing';
COMMENT ON COLUMN outbox_events.max_retries IS 'Maximum number of retry attempts before permanent failure';
COMMENT ON COLUMN outbox_events.status IS 'Current processing status of the outbox event';
COMMENT ON COLUMN outbox_events.payload IS 'JSON payload containing event data for message queue';
COMMENT ON COLUMN outbox_events.error_details IS 'Infrastructure error information from failed message publishing attempts (connection errors, serialization failures, queue unavailable)';
COMMENT ON COLUMN outbox_events.created_at IS 'Timestamp when outbox event is created (timeline step 1)';
COMMENT ON COLUMN outbox_events.started_at IS 'Timestamp when Publisher starts publishing to message queue (timeline step 2)';
COMMENT ON COLUMN outbox_events.published_at IS 'Timestamp when event was successfully published to queue (timeline step 3)';
COMMENT ON COLUMN outbox_events.processed_at IS 'Timestamp when Subscriber starts processing the analysis (timeline step 4)';
COMMENT ON COLUMN outbox_events.completed_at IS 'Timestamp when Subscriber completes the analysis (timeline step 5 - application layer calculates and updates analysis.duration)';
COMMENT ON COLUMN outbox_events.next_retry_at IS 'Timestamp when failed event should be retried';
COMMENT ON COLUMN outbox_events.archived_at IS 'Timestamp when event was archived (soft deletion)';
COMMENT ON COLUMN outbox_events.expires_at IS 'Timestamp when event should be automatically cleaned up';

-- Index comments for maintenance
COMMENT ON INDEX idx_outbox_pending IS 'Index for efficient polling of pending events by priority';
COMMENT ON INDEX idx_outbox_retry IS 'Index for finding failed events ready for retry processing';
COMMENT ON INDEX idx_outbox_aggregate IS 'Index for aggregate-based queries and debugging';
COMMENT ON INDEX idx_outbox_event_type IS 'Index for event type monitoring and routing queries';
COMMENT ON INDEX idx_outbox_published IS 'Index for published event cleanup and archival operations';
COMMENT ON INDEX idx_outbox_payload_gin IS 'GIN index for efficient JSON payload queries';
COMMENT ON INDEX idx_outbox_expired IS 'Index for finding expired events for automatic cleanup';
COMMENT ON INDEX idx_outbox_archived IS 'Index for archived event queries and maintenance';
