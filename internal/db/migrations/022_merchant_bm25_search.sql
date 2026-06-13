-- +goose Up
-- +goose StatementBegin
-- Enable pg_textsearch extension for BM25 full-text search (if available)
-- This extension is available on self-hosted PostgreSQL 18+ with pg_textsearch installed,
-- but NOT on managed services like Google Cloud SQL.
-- The application code falls back to built-in tsvector/ts_rank if BM25 is unavailable.

DO $$
BEGIN
    -- Try to create the extension (will fail silently on Cloud SQL where it's not available)
    BEGIN
        CREATE EXTENSION IF NOT EXISTS pg_textsearch;
        RAISE NOTICE 'pg_textsearch extension created successfully';
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'pg_textsearch extension not available (this is OK on Cloud SQL) - using built-in FTS fallback';
    END;
    
    -- Only create the BM25 index if the extension is loaded
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_textsearch') THEN
        -- Check if index already exists
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'merchants_bm25_idx') THEN
            CREATE INDEX merchants_bm25_idx ON merchants USING bm25(display_name) WITH (text_config='english');
            RAISE NOTICE 'BM25 index created on merchants.display_name';
        END IF;
    ELSE
        RAISE NOTICE 'Skipping BM25 index creation - pg_textsearch not available';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS merchants_bm25_idx;

DO $$
BEGIN
    DROP EXTENSION IF EXISTS pg_textsearch;
EXCEPTION WHEN OTHERS THEN
    -- Ignore errors if extension doesn't exist
    NULL;
END $$;
-- +goose StatementEnd