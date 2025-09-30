-- Drop indexes (they'll be dropped with the table, but explicit for clarity)
DROP INDEX IF EXISTS idx_analysis_status_created;
DROP INDEX IF EXISTS idx_analysis_url_norm_version_desc;
DROP INDEX IF EXISTS idx_analysis_content_hash;
DROP INDEX IF EXISTS idx_analysis_url_normalized;
DROP INDEX IF EXISTS idx_analysis_results_gin;
DROP INDEX IF EXISTS idx_analysis_options_gin;
DROP INDEX IF EXISTS idx_analysis_html_version;
DROP INDEX IF EXISTS idx_analysis_completed;
DROP INDEX IF EXISTS idx_analysis_expired;
DROP INDEX IF EXISTS idx_analysis_archived;

-- Drop the main table
DROP TABLE IF EXISTS analysis;

-- Drop enum types
DROP TYPE IF EXISTS analysis_status;
