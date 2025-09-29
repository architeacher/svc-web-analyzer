-- Drop triggers
DROP TRIGGER IF EXISTS update_analysis_updated_at ON analysis;
DROP TRIGGER IF EXISTS set_analysis_version ON analysis;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS set_analysis_version();

-- Drop indexes (they'll be dropped with the table, but explicit for clarity)
DROP INDEX IF EXISTS idx_analysis_status_created;
DROP INDEX IF EXISTS idx_analysis_status_started;
DROP INDEX IF EXISTS idx_analysis_url_norm_version_desc;
DROP INDEX IF EXISTS idx_analysis_content_hash;
DROP INDEX IF EXISTS idx_analysis_url_normalized;
DROP INDEX IF EXISTS idx_analysis_retry_status;
DROP INDEX IF EXISTS idx_analysis_results_gin;
DROP INDEX IF EXISTS idx_analysis_options_gin;
DROP INDEX IF EXISTS idx_analysis_html_version;
DROP INDEX IF EXISTS idx_analysis_completed;
DROP INDEX IF EXISTS idx_analysis_failed;
DROP INDEX IF EXISTS idx_analysis_expired;
DROP INDEX IF EXISTS idx_analysis_archived;

-- Drop main table
DROP TABLE IF EXISTS analysis;

-- Drop enum types
DROP TYPE IF EXISTS analysis_status;