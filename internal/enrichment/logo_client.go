package enrichment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/storage"
)

// LogoClient fetches company logos from logo.dev
type LogoClient struct {
	httpClient     *http.Client
	publishableKey string
	secretKey      string
	logoStore      *LogoStore
	cdnDomain      string // CDN domain for URL transformation

	// Cache domain lookups to avoid repeated API calls
	domainCache map[string]string
	cacheMu     sync.RWMutex
}

// NewLogoClient creates a new logo.dev client
func NewLogoClient(cfg *config.Config) (*LogoClient, error) {
	// Always use cloud storage if bucket is configured, otherwise fail
	ctx := context.Background()
	var storageInstance storage.Storage
	var err error

	// Determine storage type: prefer explicit type, or default to GCS if bucket is set
	storageType := cfg.StorageType
	if storageType == "" && cfg.StorageBucket != "" {
		storageType = "gcs"
	}

	if storageType == "gcs" || storageType == "s3" {
		var gcsCreds []byte
		if cfg.GCSCredentialsJSON != "" {
			gcsCreds = []byte(cfg.GCSCredentialsJSON)
		}
		storageInstance, err = storage.NewStorage(
			ctx,
			storageType,
			cfg.StorageBucket,
			cfg.StorageRegion,
			cfg.StorageEndpoint,
			cfg.StorageAccessKeyID,
			cfg.StorageSecretAccessKey,
			cfg.BaseURL,
			cfg.CDNDomain,
			gcsCreds,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize storage: %w", err)
		}
	} else {
		// Cloud storage is required - no local fallback
		return nil, fmt.Errorf("cloud storage is required (set STORAGE_BUCKET or STORAGE_TYPE=gcs/s3)")
	}

	return &LogoClient{
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		publishableKey: cfg.LogoDevPublishableKey,
		secretKey:      cfg.LogoDevSecretKey,
		logoStore:      NewLogoStore(storageInstance, "static/logos", "/static/logos"),
		cdnDomain:      cfg.CDNDomain,
		domainCache:    make(map[string]string),
	}, nil
}

// GetLogoStore returns the logo store for downloading logos
func (c *LogoClient) GetLogoStore() *LogoStore {
	return c.logoStore
}

// GetLogoURL constructs the full CDN URL from a filename stored in the database
func (c *LogoClient) GetLogoURL(logoURL string) string {
	return GetLogoURL(logoURL, c.cdnDomain)
}

// IsConfigured returns true if both keys are set
func (c *LogoClient) IsConfigured() bool {
	return c.publishableKey != "" && c.secretKey != ""
}

type brandSearchResult struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

type brandInfo struct {
	Name   string
	Domain string
}

func (c *LogoClient) searchBrand(ctx context.Context, searchTerm string) (*brandInfo, error) {
	if !c.IsConfigured() || searchTerm == "" {
		return nil, nil
	}

	cacheKey := strings.ToLower(searchTerm)

	// Check cache first
	c.cacheMu.RLock()
	if cached, ok := c.domainCache[cacheKey]; ok {
		c.cacheMu.RUnlock()
		if cached == "" {
			return nil, nil // Cached negative result
		}
		// Parse cached value (format: "name|domain")
		parts := strings.SplitN(cached, "|", 2)
		if len(parts) == 2 {
			return &brandInfo{Name: parts[0], Domain: parts[1]}, nil
		}
		return nil, nil
	}
	c.cacheMu.RUnlock()

	// Call the Brand Search API
	searchURL := fmt.Sprintf("https://api.logo.dev/search?q=%s&strategy=match", url.QueryEscape(searchTerm))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.secretKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	var results []brandSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	// Find the best match - prefer exact matches, then longest matching name
	var best *brandSearchResult
	var bestMatchLen int
	searchLower := strings.ToLower(searchTerm)

	for i := range results {
		r := &results[i]
		nameLower := strings.ToLower(r.Name)

		// Exact match is always best
		if nameLower == searchLower {
			best = r
			break
		}

		// Check if search term contains the brand name
		// Prefer longer matches (e.g., "Bluestone Lane" over "Bluestone")
		if strings.Contains(searchLower, nameLower) {
			if len(nameLower) > bestMatchLen {
				best = r
				bestMatchLen = len(nameLower)
			}
		}
	}

	// Fall back to first result if no better match
	if best == nil && len(results) > 0 {
		best = &results[0]
	}

	// Cache the result
	c.cacheMu.Lock()
	if best != nil {
		c.domainCache[cacheKey] = best.Name + "|" + best.Domain
	} else {
		c.domainCache[cacheKey] = "" // Cache negative result
	}
	c.cacheMu.Unlock()

	if best == nil {
		return nil, nil
	}

	return &brandInfo{Name: best.Name, Domain: best.Domain}, nil
}

// GetLogoDevURL returns a logo.dev image URL for the given domain (without downloading)
func (c *LogoClient) GetLogoDevURL(domain string) string {
	if c.publishableKey == "" || domain == "" {
		return ""
	}
	return fmt.Sprintf("https://img.logo.dev/%s?token=%s", url.PathEscape(domain), c.publishableKey)
}

// DownloadAndStoreLogo downloads a logo from logo.dev and stores it.
// Returns just the filename (e.g., "abc123.png").
func (c *LogoClient) DownloadAndStoreLogo(ctx context.Context, domain string) (string, error) {
	if !c.IsConfigured() || domain == "" {
		return "", fmt.Errorf("not configured or empty domain")
	}
	return c.logoStore.DownloadFromLogoDevAndStore(ctx, domain, c.publishableKey)
}

// GetLogoURLForEntity searches for a company's domain and returns a logo URL
// It first tries the website domain, then falls back to searching by name
func (c *LogoClient) GetLogoURLForEntity(ctx context.Context, website, entityName string) string {
	if !c.IsConfigured() {
		return ""
	}

	// Try website domain first
	if website != "" {
		domain := extractDomain(website)
		if domain != "" {
			return c.GetLogoDevURL(domain)
		}
	}

	// Search for brand by company name
	brand, err := c.searchBrand(ctx, entityName)
	if err != nil || brand == nil {
		return ""
	}

	return c.GetLogoDevURL(brand.Domain)
}

// DownloadLogoForEntity searches for a company and downloads+stores the logo.
// Returns just the filename (e.g., "abc123.png").
func (c *LogoClient) DownloadLogoForEntity(ctx context.Context, website, entityName string) (string, error) {
	if !c.IsConfigured() {
		return "", fmt.Errorf("logo client not configured")
	}

	var domain string

	// Try website domain first
	if website != "" {
		domain = extractDomain(website)
	}

	// Fall back to searching by name
	if domain == "" {
		brand, err := c.searchBrand(ctx, entityName)
		if err != nil || brand == nil {
			return "", fmt.Errorf("could not find brand for %s", entityName)
		}
		domain = brand.Domain
	}

	if domain == "" {
		return "", fmt.Errorf("no domain found for entity")
	}

	return c.DownloadAndStoreLogo(ctx, domain)
}

// extractDomain extracts the domain from a URL or returns as-is if already a domain
func extractDomain(website string) string {
	website = strings.TrimSpace(website)
	if website == "" {
		return ""
	}

	// Add scheme if missing for URL parsing
	if !strings.HasPrefix(website, "http://") && !strings.HasPrefix(website, "https://") {
		website = "https://" + website
	}

	parsed, err := url.Parse(website)
	if err != nil {
		return ""
	}

	host := parsed.Hostname()
	// Remove www. prefix
	host = strings.TrimPrefix(host, "www.")
	return host
}
