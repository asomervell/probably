package enrichment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FirecrawlCache handles caching of Firecrawl API responses
type FirecrawlCache struct {
	pool *pgxpool.Pool
}

// NewFirecrawlCache creates a new Firecrawl cache
func NewFirecrawlCache(pool *pgxpool.Pool) *FirecrawlCache {
	return &FirecrawlCache{pool: pool}
}

// CacheType represents the type of cache entry
type CacheType string

const (
	CacheTypeSearch CacheType = "search"
	CacheTypeScrape CacheType = "scrape"
)

// Get retrieves a cached response
func (c *FirecrawlCache) Get(ctx context.Context, cacheKey string) (json.RawMessage, error) {
	var responseData json.RawMessage
	var expiresAt time.Time

	err := c.pool.QueryRow(ctx, `
		SELECT response_data, expires_at
		FROM firecrawl_cache
		WHERE cache_key = $1 AND expires_at > NOW()
	`, cacheKey).Scan(&responseData, &expiresAt)

	if err != nil {
		return nil, err
	}

	return responseData, nil
}

// Set stores a response in the cache
func (c *FirecrawlCache) Set(ctx context.Context, cacheKey string, cacheType CacheType, responseData json.RawMessage, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)

	_, err := c.pool.Exec(ctx, `
		INSERT INTO firecrawl_cache (cache_key, cache_type, response_data, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (cache_key) 
		DO UPDATE SET 
			response_data = EXCLUDED.response_data,
			expires_at = EXCLUDED.expires_at,
			created_at = NOW()
	`, cacheKey, string(cacheType), responseData, expiresAt)

	return err
}

// CleanupExpired removes expired cache entries
func (c *FirecrawlCache) CleanupExpired(ctx context.Context) error {
	_, err := c.pool.Exec(ctx, `
		DELETE FROM firecrawl_cache
		WHERE expires_at <= NOW()
	`)
	return err
}

// GenerateSearchCacheKey creates a cache key for a search request.
// Note: countryHint is typically already included in the query string by the caller,
// so this parameter is kept for backwards compatibility but rarely used directly.
func GenerateSearchCacheKey(query string, limit int, countryHint string) string {
	key := fmt.Sprintf("search:%s:limit:%d", query, limit)
	if countryHint != "" {
		key += fmt.Sprintf(":country:%s", countryHint)
	}
	return key
}

// GenerateScrapeCacheKey creates a cache key for a scrape request
func GenerateScrapeCacheKey(url string) string {
	return fmt.Sprintf("scrape:%s", url)
}
