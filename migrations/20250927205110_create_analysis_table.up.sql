-- UUID extension is enabled in 000_create_database.up.sql

-- Create enum types for better type safety
CREATE TYPE analysis_status AS ENUM ('requested', 'in_progress', 'completed', 'failed');
CREATE TYPE analysis_priority AS ENUM ('low', 'normal', 'high', 'urgent');

-- Main analysis table with versioning support
CREATE TABLE analysis (
    id UUID PRIMARY KEY,
    url TEXT NOT NULL,
    url_normalized TEXT NOT NULL,
    content_hash VARCHAR(64), -- SHA-256 hash of page content for deduplication
    content_size BIGINT,
    status analysis_status NOT NULL DEFAULT 'requested',
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
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,

    -- Ensure unique URL + version combination
    CONSTRAINT uk_analysis_url_version UNIQUE (url, version)
);

-- ============================================================================
-- INDEXING STRATEGY
-- ============================================================================
--
-- This migration follows a balanced indexing strategy prioritizing:
-- 1. Query performance for common access patterns
-- 2. Minimal maintenance overhead (fewer indexes = faster writes)
-- 3. Flexibility for future query requirements
--
-- COMPOSITE vs. PARTIAL INDEXES:
-- - Composite indexes (e.g., idx_analysis_status_created) cover multiple query patterns
--   and are preferred over multiple partial indexes unless proven otherwise
-- - Partial indexes are ONLY added when:
--   a) A subset is queried extremely frequently (thousands of QPS)
--   b) The subset is very small (<5% of total rows)
--   c) The performance gain justifies the maintenance cost
--
-- CURRENT QUERY PATTERNS:
-- - Find analysis by ID: Primary key lookup (no index needed)
-- - Find by content hash: idx_analysis_content_hash (partial, WHERE content_hash IS NOT NULL)
-- - Find by normalized URL: idx_analysis_url_normalized
-- - Find latest version: idx_analysis_url_norm_version_desc
-- - Status queries: idx_analysis_status_created (covers ALL status values)
-- - Cleanup queries: idx_analysis_expired, idx_analysis_archived
--
-- FUTURE CONSIDERATIONS:
-- If monitoring dashboards or cleanup jobs frequently query specific statuses
-- (e.g., WHERE status = 'failed' ORDER BY created_at), consider partial indexes.
-- However, the FIRST measure if idx_analysis_status_created is sufficient.
--
-- MAINTENANCE COST:
-- Each index adds overhead to INSERT, UPDATE, DELETE operations.
-- Analysis records are written frequently (every analysis request).
-- Avoid premature optimization - add indexes only when proven necessary.
-- ============================================================================

-- Create indexes for performance
-- Composite index for common status + timestamp queries
-- Covers: WHERE status = ? ORDER BY created_at (for ANY status value)
CREATE INDEX idx_analysis_status_created ON analysis(status, created_at);

-- Index for finding the latest version of a URL (normalized)
CREATE INDEX idx_analysis_url_norm_version_desc ON analysis(url_normalized, version DESC);

-- Index for content hash deduplication
CREATE INDEX idx_analysis_content_hash ON analysis(content_hash) WHERE content_hash IS NOT NULL;

-- Index for normalized URL lookups
CREATE INDEX idx_analysis_url_normalized ON analysis(url_normalized);

-- JSON indexes for common queries on results and options
CREATE INDEX idx_analysis_results_gin ON analysis USING GIN(results);
CREATE INDEX idx_analysis_options_gin ON analysis USING GIN(options);
CREATE INDEX idx_analysis_html_version ON analysis((results->>'html_version')) WHERE results IS NOT NULL;

-- Partial indexes for performance and cleanup
-- Note: No partial index for 'failed' status - idx_analysis_status_created covers it efficiently
CREATE INDEX idx_analysis_completed ON analysis(completed_at) WHERE status = 'completed';
CREATE INDEX idx_analysis_expired ON analysis(expires_at) WHERE expires_at IS NOT NULL AND archived_at IS NULL;
CREATE INDEX idx_analysis_archived ON analysis(archived_at) WHERE archived_at IS NOT NULL;

-- Add comments for documentation
COMMENT ON TABLE analysis IS 'Web page analysis results storage with versioning support';
COMMENT ON COLUMN analysis.id IS 'Primary key UUID identifier (application generates UUID V5 from url_normalized + version)';
COMMENT ON COLUMN analysis.url IS 'The URL being analyzed';
COMMENT ON COLUMN analysis.url_normalized IS 'Normalized URL for efficient lookups and deduplication';
COMMENT ON COLUMN analysis.content_hash IS 'SHA-256 hash of page content for deduplication';
COMMENT ON COLUMN analysis.content_size IS 'Size of analyzed content in bytes';
COMMENT ON COLUMN analysis.duration IS 'Total processing duration in milliseconds from request creation to completion (calculated by application layer when analysis completes)';
COMMENT ON COLUMN analysis.version IS 'Version number for the same URL (auto-incremented)';
COMMENT ON COLUMN analysis.lock_version IS 'Version for optimistic concurrency control';
COMMENT ON COLUMN analysis.options IS 'JSON structure containing analysis options and configuration metadata';
COMMENT ON COLUMN analysis.results IS 'JSON structure containing analysis data including HTML version, title, heading counts, links, and forms';
COMMENT ON COLUMN analysis.error_code IS 'Business logic error identifier when webpage analysis fails (e.g., FETCH_ERROR, PARSE_ERROR)';
COMMENT ON COLUMN analysis.error_message IS 'Human-readable description of business logic failure during webpage analysis';
COMMENT ON COLUMN analysis.error_status_code IS 'HTTP status code from failed webpage fetch attempt (e.g., 404, 500, timeout)';
COMMENT ON COLUMN analysis.error_details IS 'Additional context for business logic failures (stack traces, parsing errors, network issues)';
COMMENT ON COLUMN analysis.archived_at IS 'Timestamp when record was archived (soft deletion)';
COMMENT ON COLUMN analysis.expires_at IS 'Timestamp when record should be automatically cleaned up';
COMMENT ON CONSTRAINT uk_analysis_url_version ON analysis IS 'Ensures unique URL + version combinations';
COMMENT ON INDEX idx_analysis_url_normalized IS 'Index for fast normalized URL lookups';
COMMENT ON INDEX idx_analysis_content_hash IS 'Index for content hash-based deduplication queries';
COMMENT ON INDEX idx_analysis_url_norm_version_desc IS 'Index for finding latest version of a normalized URL efficiently';
COMMENT ON INDEX idx_analysis_options_gin IS 'GIN index for efficient JSON queries on options column';
COMMENT ON INDEX idx_analysis_results_gin IS 'GIN index for efficient JSON queries on results column';
COMMENT ON INDEX idx_analysis_expired IS 'Index for finding expired records for cleanup';
COMMENT ON INDEX idx_analysis_archived IS 'Index for archived record queries';
