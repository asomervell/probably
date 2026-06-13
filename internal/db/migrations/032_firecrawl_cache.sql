-- +goose Up
-- Create table for caching Firecrawl API responses (global cache, not user-specific)
CREATE TABLE firecrawl_cache (
    cache_key TEXT PRIMARY KEY,
    cache_type TEXT NOT NULL, -- 'search' or 'scrape'
    response_data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- Index for efficient cleanup of expired entries
CREATE INDEX idx_firecrawl_cache_expires_at ON firecrawl_cache (expires_at);

-- Index for cache type lookups
CREATE INDEX idx_firecrawl_cache_type ON firecrawl_cache (cache_type);

-- +goose Down
DROP INDEX IF EXISTS idx_firecrawl_cache_type;
DROP INDEX IF EXISTS idx_firecrawl_cache_expires_at;
DROP TABLE IF EXISTS firecrawl_cache;
