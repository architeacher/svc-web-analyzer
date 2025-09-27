-- UUID extension is enabled in 000_create_database.up.sql

-- Create enum types for better type safety
CREATE TYPE analysis_status AS ENUM ('requested', 'in_progress', 'completed', 'failed');
CREATE TYPE analysis_priority AS ENUM ('low', 'normal', 'high', 'urgent');

-- Main analysis table with versioning support
CREATE TABLE analysis (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    url TEXT NOT NULL,
    url_normalized TEXT NOT NULL,
    content_hash VARCHAR(64), -- SHA-256 hash of page content for deduplication
    content_size BIGINT,
    status analysis_status NOT NULL DEFAULT 'requested',
    priority analysis_priority NOT NULL DEFAULT 'normal',
    retry_count INTEGER NOT NULL DEFAULT 0,
    duration BIGINT, -- Duration in milliseconds
    version INTEGER NOT NULL DEFAULT 1,
    lock_version INTEGER NOT NULL DEFAULT 1,

    -- Analysis options and configuration metadata as JSON
    options JSONB,

    -- Analysis results as JSON (PostgreSQL has excellent JSON support)
    results JSONB,

    -- Error information
    error_code TEXT,
    error_message TEXT,
    error_status_code INTEGER,
    error_details TEXT,

    -- Metadata
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure unique URL + version combination
    CONSTRAINT uk_analysis_url_version UNIQUE (url, version)
);

-- Create indexes for performance
-- Composite index for common status + timestamp queries
CREATE INDEX idx_analysis_status_created ON analysis(status, created_at);
CREATE INDEX idx_analysis_status_started ON analysis(status, started_at) WHERE started_at IS NOT NULL;

-- Index for finding the latest version of a URL (normalized)
CREATE INDEX idx_analysis_url_norm_version_desc ON analysis(url_normalized, version DESC);

-- Index for content hash deduplication
CREATE INDEX idx_analysis_content_hash ON analysis(content_hash) WHERE content_hash IS NOT NULL;

-- Index for normalized URL lookups
CREATE INDEX idx_analysis_url_normalized ON analysis(url_normalized);

-- Index for retry logic
CREATE INDEX idx_analysis_retry_status ON analysis(retry_count, status) WHERE retry_count > 0;

-- Index for priority-based queue management
CREATE INDEX idx_analysis_priority_status ON analysis(priority, status, created_at) WHERE status IN ('requested', 'in_progress');

-- JSON indexes for common queries on results and options
CREATE INDEX idx_analysis_results_gin ON analysis USING GIN(results);
CREATE INDEX idx_analysis_options_gin ON analysis USING GIN(options);
CREATE INDEX idx_analysis_html_version ON analysis((results->>'html_version')) WHERE results IS NOT NULL;

-- Partial indexes for performance and cleanup
CREATE INDEX idx_analysis_completed ON analysis(completed_at) WHERE status = 'completed';
CREATE INDEX idx_analysis_failed ON analysis(created_at) WHERE status = 'failed';
CREATE INDEX idx_analysis_expired ON analysis(expires_at) WHERE expires_at IS NOT NULL AND archived_at IS NULL;
CREATE INDEX idx_analysis_archived ON analysis(archived_at) WHERE archived_at IS NOT NULL;

-- Function to automatically update updated_at timestamp and increment lock_version
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    NEW.lock_version = OLD.lock_version + 1;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Function to automatically set the version for new URL analyses
CREATE OR REPLACE FUNCTION set_analysis_version()
RETURNS TRIGGER AS $$
BEGIN
    -- If no version specified, get the next version for this URL
    IF NEW.version IS NULL OR NEW.version = 1 THEN
        SELECT COALESCE(MAX(version), 0) + 1
        INTO NEW.version
        FROM analysis
        WHERE url = NEW.url;
    END IF;

    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to automatically update updated_at
CREATE TRIGGER update_analysis_updated_at
    BEFORE UPDATE ON analysis
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Trigger to automatically set version
CREATE TRIGGER set_analysis_version
    BEFORE INSERT ON analysis
    FOR EACH ROW
    EXECUTE FUNCTION set_analysis_version();

-- Add comments for documentation
COMMENT ON TABLE analysis IS 'Web page analysis results storage with versioning support';
COMMENT ON COLUMN analysis.url IS 'The URL being analyzed';
COMMENT ON COLUMN analysis.url_normalized IS 'Normalized URL for efficient lookups and deduplication';
COMMENT ON COLUMN analysis.priority IS 'Priority level for analysis queue management (low, normal, high, urgent)';
COMMENT ON COLUMN analysis.content_hash IS 'SHA-256 hash of page content for deduplication';
COMMENT ON COLUMN analysis.content_size IS 'Size of analyzed content in bytes';
COMMENT ON COLUMN analysis.retry_count IS 'Number of retry attempts for failed analyses';
COMMENT ON COLUMN analysis.duration IS 'Analysis duration in milliseconds';
COMMENT ON COLUMN analysis.version IS 'Version number for the same URL (auto-incremented)';
COMMENT ON COLUMN analysis.lock_version IS 'Version for optimistic concurrency control';
COMMENT ON COLUMN analysis.options IS 'JSON structure containing analysis options and configuration metadata';
COMMENT ON COLUMN analysis.results IS 'JSON structure containing analysis data including HTML version, title, heading counts, links, and forms';
COMMENT ON COLUMN analysis.started_at IS 'Timestamp when analysis processing started';
COMMENT ON COLUMN analysis.archived_at IS 'Timestamp when record was archived (soft deletion)';
COMMENT ON COLUMN analysis.expires_at IS 'Timestamp when record should be automatically cleaned up';
COMMENT ON CONSTRAINT uk_analysis_url_version ON analysis IS 'Ensures unique URL + version combinations';
COMMENT ON INDEX idx_analysis_url_normalized IS 'Index for fast normalized URL lookups';
COMMENT ON INDEX idx_analysis_content_hash IS 'Index for content hash-based deduplication queries';
COMMENT ON INDEX idx_analysis_retry_status IS 'Index for retry logic queries';
COMMENT ON INDEX idx_analysis_url_norm_version_desc IS 'Index for finding latest version of a normalized URL efficiently';
COMMENT ON INDEX idx_analysis_options_gin IS 'GIN index for efficient JSON queries on options column';
COMMENT ON INDEX idx_analysis_results_gin IS 'GIN index for efficient JSON queries on results column';
COMMENT ON INDEX idx_analysis_expired IS 'Index for finding expired records for cleanup';
COMMENT ON INDEX idx_analysis_archived IS 'Index for archived record queries';
