-- Database initialization
-- Note: In Docker Compose setup, the database is created automatically
-- via the POSTGRES_DATABASE environment variable. This migration ensures
-- proper database settings and extensions are available.

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
