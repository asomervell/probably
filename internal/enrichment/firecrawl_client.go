package enrichment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/config"
)

const (
	FirecrawlBaseURL = "https://api.firecrawl.dev/v1"
)

// FirecrawlClient handles communication with the Firecrawl API
type FirecrawlClient struct {
	httpClient *http.Client
	apiKey     string
	cache      *FirecrawlCache
}

// NewFirecrawlClientWithCache creates a new Firecrawl API client with cache
func NewFirecrawlClientWithCache(cfg *config.Config, cache *FirecrawlCache) *FirecrawlClient {
	return &FirecrawlClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     cfg.FirecrawlAPIKey,
		cache:      cache,
	}
}

// IsConfigured returns true if the API key is set
func (c *FirecrawlClient) IsConfigured() bool {
	return c.apiKey != ""
}

// CompanyInfo contains extracted company information
type CompanyInfo struct {
	Name        string `json:"name"`
	LogoURL     string `json:"logo_url"`
	Website     string `json:"website"`
	Description string `json:"description"`
	SourceURL   string `json:"source_url,omitempty"` // The URL that was scraped to get this info
}

// SearchResult represents a search result from Firecrawl
type SearchResult struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// FirecrawlSearchResponse is the response from the search API
type FirecrawlSearchResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		URL         string `json:"url"`
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

// FirecrawlErrorResponse represents an error response from Firecrawl API
type FirecrawlErrorResponse struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

// FirecrawlScrapeRequest is the request body for the scrape API
type FirecrawlScrapeRequest struct {
	URL     string   `json:"url"`
	Formats []string `json:"formats"`
}

// FirecrawlScrapeResponse is the response from the scrape API
type FirecrawlScrapeResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Markdown string `json:"markdown"`
		HTML     string `json:"html"`
		Metadata struct {
			Title          string `json:"title"`
			Description    string `json:"description"`
			OGImage        string `json:"ogImage"`
			OGTitle        string `json:"ogTitle"`
			Icon           string `json:"icon"`
			AppleTouchIcon string `json:"appleTouchIcon"`
			Favicon        string `json:"favicon"`
		} `json:"metadata"`
		Links []string `json:"links"`
	} `json:"data"`
}

// FirecrawlSearchRequest is the request body for the search API
type FirecrawlSearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// Search searches the web for companies matching the query.
// descriptionContext is optional - if provided, it's appended to provide location/context hints
// (e.g., cleaned transaction description with "AUCKLAND").
// countryHint is optional - if provided, it's appended to help localize results
// (e.g., "New Zealand" for NZD transactions).
func (c *FirecrawlClient) Search(ctx context.Context, query string, limit int, descriptionContext string, countryHint string) ([]SearchResult, error) {
	if !c.IsConfigured() || query == "" {
		return nil, nil
	}

	if limit <= 0 {
		limit = 5
	}

	// Build search query with optional description context and country hint
	searchQuery := query
	if descriptionContext != "" {
		searchQuery = searchQuery + " " + descriptionContext
	}
	if countryHint != "" {
		searchQuery = searchQuery + " " + countryHint
	}

	// Check cache first
	if c.cache != nil {
		cacheKey := GenerateSearchCacheKey(searchQuery, limit, "")
		if cachedData, err := c.cache.Get(ctx, cacheKey); err == nil && cachedData != nil {
			var searchResp FirecrawlSearchResponse
			if err := json.Unmarshal(cachedData, &searchResp); err == nil {
				slog.DebugContext(ctx, "firecrawl cache hit", "search_query", searchQuery, "limit", limit)
				// Convert to SearchResult slice
				results := make([]SearchResult, 0, len(searchResp.Data))
				for _, r := range searchResp.Data {
					results = append(results, SearchResult{
						URL:         r.URL,
						Title:       r.Title,
						Description: r.Description,
					})
				}
				return results, nil
			} else {
				slog.WarnContext(ctx, "firecrawl cache unmarshal error", "type", "search", "err", err)
			}
		}
		slog.DebugContext(ctx, "firecrawl cache miss", "search_query", searchQuery, "limit", limit)
	}

	// Build search request body
	searchReq := FirecrawlSearchRequest{
		Query: searchQuery,
		Limit: limit,
	}

	body, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", FirecrawlBaseURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body first to check for errors
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse error response
		var errorResp FirecrawlErrorResponse
		if err := json.Unmarshal(respBody, &errorResp); err == nil {
			errorMsg := errorResp.Error
			if errorMsg == "" {
				errorMsg = errorResp.Message
			}
			if errorMsg == "" {
				errorMsg = fmt.Sprintf("API error (status %d)", resp.StatusCode)
			}
			return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, errorMsg)
		}
		// If not JSON, return raw response (truncated)
		errorMsg := string(respBody)
		if len(errorMsg) > 200 {
			errorMsg = errorMsg[:200] + "..."
		}
		return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, errorMsg)
	}

	// Check if response is valid JSON before attempting to decode
	var searchResp FirecrawlSearchResponse
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		// If response is not JSON, it's likely an error message
		errorMsg := string(respBody)
		if len(errorMsg) > 200 {
			errorMsg = errorMsg[:200] + "..."
		}
		return nil, fmt.Errorf("search API returned invalid JSON response: %s", errorMsg)
	}

	if !searchResp.Success {
		errorMsg := searchResp.Error
		if errorMsg == "" {
			errorMsg = "search was not successful"
		}
		return nil, fmt.Errorf("search failed: %s", errorMsg)
	}

	// Cache the response (30 days TTL for search results)
	if c.cache != nil {
		cacheKey := GenerateSearchCacheKey(searchQuery, limit, "")
		if responseData, err := json.Marshal(searchResp); err == nil {
			if err := c.cache.Set(ctx, cacheKey, CacheTypeSearch, responseData, 30*24*time.Hour); err == nil {
				slog.DebugContext(ctx, "firecrawl cache set", "search_query", searchQuery, "limit", limit)
			} else {
				slog.WarnContext(ctx, "firecrawl cache set failed", "type", "search", "err", err)
			}
		} else {
			slog.WarnContext(ctx, "firecrawl cache marshal failed", "type", "search", "err", err)
		}
	}

	// Convert to SearchResult slice
	results := make([]SearchResult, 0, len(searchResp.Data))
	for _, r := range searchResp.Data {
		results = append(results, SearchResult{
			URL:         r.URL,
			Title:       r.Title,
			Description: r.Description,
		})
	}

	return results, nil
}

// ExtractCompanyInfo scrapes a URL and extracts company information
func (c *FirecrawlClient) ExtractCompanyInfo(ctx context.Context, targetURL string) (*CompanyInfo, error) {
	if !c.IsConfigured() || targetURL == "" {
		return nil, nil
	}

	// Check cache first
	if c.cache != nil {
		cacheKey := GenerateScrapeCacheKey(targetURL)
		if cachedData, err := c.cache.Get(ctx, cacheKey); err == nil && cachedData != nil {
			var scrapeResp FirecrawlScrapeResponse
			if err := json.Unmarshal(cachedData, &scrapeResp); err == nil {
				slog.DebugContext(ctx, "firecrawl scrape cache hit", "url", targetURL)
				// Extract company info from cached response
				return c.extractCompanyInfoFromResponse(targetURL, &scrapeResp), nil
			} else {
				slog.WarnContext(ctx, "firecrawl cache unmarshal error", "type", "scrape", "err", err)
			}
		}
		slog.DebugContext(ctx, "firecrawl scrape cache miss", "url", targetURL)
	}

	// Build scrape request - request markdown and links to get metadata (logos, icons, etc.)
	scrapeReq := FirecrawlScrapeRequest{
		URL:     targetURL,
		Formats: []string{"markdown", "links"},
	}

	body, err := json.Marshal(scrapeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal scrape request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", FirecrawlBaseURL+"/scrape", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create scrape request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scrape request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body first to check for errors
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read scrape response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scrape failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Check if response is valid JSON before attempting to decode
	var scrapeResp FirecrawlScrapeResponse
	if err := json.Unmarshal(respBody, &scrapeResp); err != nil {
		// If response is not JSON, it's likely an error message
		errorMsg := string(respBody)
		if len(errorMsg) > 100 {
			errorMsg = errorMsg[:100] + "..."
		}
		return nil, fmt.Errorf("scrape API returned invalid response: %s", errorMsg)
	}

	if !scrapeResp.Success {
		return nil, fmt.Errorf("scrape was not successful")
	}

	// Cache the response (90 days TTL for scrape results)
	if c.cache != nil {
		cacheKey := GenerateScrapeCacheKey(targetURL)
		if responseData, err := json.Marshal(scrapeResp); err == nil {
			if err := c.cache.Set(ctx, cacheKey, CacheTypeScrape, responseData, 90*24*time.Hour); err == nil {
				slog.DebugContext(ctx, "firecrawl scrape cache set", "url", targetURL)
			} else {
				slog.WarnContext(ctx, "firecrawl cache set failed", "type", "scrape", "err", err)
			}
		} else {
			slog.WarnContext(ctx, "firecrawl cache marshal failed", "type", "scrape", "err", err)
		}
	}

	return c.extractCompanyInfoFromResponse(targetURL, &scrapeResp), nil
}

// extractCompanyInfoFromResponse extracts company info from a scrape response
func (c *FirecrawlClient) extractCompanyInfoFromResponse(targetURL string, scrapeResp *FirecrawlScrapeResponse) *CompanyInfo {
	// Extract company info from metadata
	info := &CompanyInfo{
		Website: extractDomainFromURL(targetURL),
	}

	// Get name from metadata
	if scrapeResp.Data.Metadata.OGTitle != "" {
		info.Name = cleanCompanyName(scrapeResp.Data.Metadata.OGTitle)
	} else if scrapeResp.Data.Metadata.Title != "" {
		info.Name = cleanCompanyName(scrapeResp.Data.Metadata.Title)
	}

	// If name is still empty or looks bad, fall back to domain name
	if info.Name == "" || len(info.Name) > 50 || isLikelyTagline(info.Name) {
		info.Name = domainToCompanyName(info.Website)
	}

	// Try to get logo - STRONGLY prefer apple-touch-icon (usually PNG, best quality, always a logo)
	// Only accept high-quality formats: apple-touch-icon, PNG, SVG, WebP, JPG
	// Reject .ico files (low-quality favicons)
	// Priority: apple-touch-icon (metadata) > apple-touch-icon (links) > favicon > icon > og:image (last resort, often marketing banners)

	// First: Check metadata for apple-touch-icon
	if scrapeResp.Data.Metadata.AppleTouchIcon != "" {
		info.LogoURL = makeAbsoluteURL(scrapeResp.Data.Metadata.AppleTouchIcon, targetURL)
	}

	// Second: Check links for apple-touch-icon (even if we have og:image, prefer apple-touch-icon)
	// This is important because many sites don't put apple-touch-icon in metadata but do in <link> tags
	if info.LogoURL == "" {
		for _, link := range scrapeResp.Data.Links {
			linkLower := strings.ToLower(link)
			if strings.Contains(linkLower, "apple-touch-icon") && !isIcoFile(link) {
				info.LogoURL = makeAbsoluteURL(link, targetURL)
				break
			}
		}
	}

	// Third: Fall back to favicon (if not .ico)
	if info.LogoURL == "" && scrapeResp.Data.Metadata.Favicon != "" && !isIcoFile(scrapeResp.Data.Metadata.Favicon) {
		info.LogoURL = makeAbsoluteURL(scrapeResp.Data.Metadata.Favicon, targetURL)
	}

	// Fourth: Fall back to icon (if not .ico)
	if info.LogoURL == "" && scrapeResp.Data.Metadata.Icon != "" && !isIcoFile(scrapeResp.Data.Metadata.Icon) {
		info.LogoURL = makeAbsoluteURL(scrapeResp.Data.Metadata.Icon, targetURL)
	}

	// Last resort: og:image (often a marketing banner, not a logo, but better than nothing)
	// Only use if we have nothing else
	if info.LogoURL == "" && scrapeResp.Data.Metadata.OGImage != "" && !isIcoFile(scrapeResp.Data.Metadata.OGImage) {
		info.LogoURL = makeAbsoluteURL(scrapeResp.Data.Metadata.OGImage, targetURL)
	}

	// Get description from metadata
	if scrapeResp.Data.Metadata.Description != "" {
		info.Description = scrapeResp.Data.Metadata.Description
	}

	return info
}

// SearchAndExtract searches for a company and extracts info from the top result.
// descriptionContext is optional - if provided, it provides location/context hints from transaction description.
// countryHint is optional - if provided, it helps localize search results
// (e.g., "New Zealand" for NZD transactions to find NZ businesses).
func (c *FirecrawlClient) SearchAndExtract(ctx context.Context, query string, descriptionContext string, countryHint string) (*CompanyInfo, error) {
	if !c.IsConfigured() || query == "" {
		return nil, nil
	}

	// Search for the company with optional description context and country hint
	results, err := c.Search(ctx, query, 3, descriptionContext, countryHint)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Try to extract info from the first result that works
	for _, result := range results {
		// Skip social media and generic sites
		if isGenericSite(result.URL) {
			continue
		}

		info, err := c.ExtractCompanyInfo(ctx, result.URL)
		if err != nil {
			continue // Try next result
		}

		if info != nil && (info.LogoURL != "" || info.Name != "") {
			// Use search result info to fill gaps
			if info.Name == "" && result.Title != "" {
				info.Name = cleanCompanyName(result.Title)
			}
			if info.Description == "" && result.Description != "" {
				info.Description = result.Description
			}
			return info, nil
		}
	}

	// If scraping failed, return basic info from the first non-generic search result
	for _, result := range results {
		if isGenericSite(result.URL) {
			continue
		}
		return &CompanyInfo{
			Name:        cleanCompanyName(result.Title),
			Website:     extractDomainFromURL(result.URL),
			Description: result.Description,
		}, nil
	}

	return nil, nil
}

// SearchWithHint searches for a company with a user-provided hint.
// descriptionContext is optional - if provided, it provides location/context hints from transaction description.
// countryHint is optional - if provided, it helps localize search results
// (e.g., "New Zealand" for NZD transactions).
func (c *FirecrawlClient) SearchWithHint(ctx context.Context, entityName, hint string, descriptionContext string, countryHint string) ([]CompanyInfo, error) {
	if !c.IsConfigured() {
		return nil, nil
	}

	// Build search query combining entity name and hint
	query := entityName
	if hint != "" {
		query = fmt.Sprintf("%s %s", entityName, hint)
	}

	// Search for companies with optional description context and country hint
	results, err := c.Search(ctx, query, 5, descriptionContext, countryHint)
	if err != nil {
		slog.ErrorContext(ctx, "firecrawl search failed", "query", query, "err", err)
		return nil, fmt.Errorf("firecrawl search failed: %w", err)
	}

	if len(results) == 0 {
		// No results is not an error, just return empty list
		return nil, nil
	}

	// Extract info from each result
	companies := make([]CompanyInfo, 0, len(results))
	for _, result := range results {
		// Skip generic sites
		if isGenericSite(result.URL) {
			continue
		}

		info := CompanyInfo{
			Name:        cleanCompanyName(result.Title),
			Website:     extractDomainFromURL(result.URL),
			Description: result.Description,
			SourceURL:   result.URL,
		}

		// Try to extract more detailed info (logo) - this uses ExtractCompanyInfo which properly gets apple-touch-icon, og:image, etc.
		detailed, err := c.ExtractCompanyInfo(ctx, result.URL)
		if err == nil && detailed != nil {
			if detailed.Name != "" {
				info.Name = detailed.Name
			}
			if detailed.LogoURL != "" {
				info.LogoURL = detailed.LogoURL
			}
			if detailed.Description != "" {
				info.Description = detailed.Description
			}
			// Keep the source URL
			info.SourceURL = result.URL
		}

		companies = append(companies, info)
	}

	return companies, nil
}

// AgentSearchRequest represents a request to the Firecrawl Agent API
type AgentSearchRequest struct {
	Prompt string                 `json:"prompt"`
	URLs   []string               `json:"urls,omitempty"`
	Schema map[string]interface{} `json:"schema,omitempty"`
}

// AgentSearchResponse represents the response from the Firecrawl Agent API
type AgentSearchResponse struct {
	Success     bool                   `json:"success"`
	Status      string                 `json:"status"` // processing, completed, failed
	Data        map[string]interface{} `json:"data,omitempty"`
	ExpiresAt   string                 `json:"expiresAt,omitempty"`
	CreditsUsed int                    `json:"creditsUsed,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// AgentSearch uses Firecrawl Agent API for better merchant discovery
// The Agent API can search the web autonomously and extract structured data
func (c *FirecrawlClient) AgentSearch(ctx context.Context, merchantName string, descriptionContext string) (*CompanyInfo, error) {
	if !c.IsConfigured() || merchantName == "" {
		return nil, nil
	}

	// Build prompt for agent
	prompt := fmt.Sprintf("Find company information for %s", merchantName)
	if descriptionContext != "" {
		prompt += ". " + descriptionContext
	}
	prompt += ". Extract: name, logo URL, website, description, business type."

	// Define schema for structured output
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Company name",
			},
			"logo_url": map[string]interface{}{
				"type":        "string",
				"description": "URL to company logo",
			},
			"website": map[string]interface{}{
				"type":        "string",
				"description": "Company website URL",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "What the company does",
			},
			"business_type": map[string]interface{}{
				"type":        "string",
				"description": "Type of business (retailer, restaurant, service, etc.)",
			},
		},
		"required": []string{"name"},
	}

	reqBody := AgentSearchRequest{
		Prompt: prompt,
		Schema: schema,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", FirecrawlBaseURL+"/agent", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create agent request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agent request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp FirecrawlErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("agent API error: %s", errResp.Error)
		}
		return nil, fmt.Errorf("agent API error %d: %s", resp.StatusCode, string(respBody))
	}

	var agentResp AgentSearchResponse
	if err := json.Unmarshal(respBody, &agentResp); err != nil {
		return nil, fmt.Errorf("failed to parse agent response: %w", err)
	}

	if !agentResp.Success {
		return nil, fmt.Errorf("agent API returned error: %s", agentResp.Error)
	}

	// If status is "processing", we need to poll for results
	// For now, we'll return an error and let the caller handle polling
	if agentResp.Status == "processing" {
		return nil, fmt.Errorf("agent job is still processing - polling not yet implemented")
	}

	if agentResp.Status != "completed" {
		return nil, fmt.Errorf("agent job failed with status: %s", agentResp.Status)
	}

	// Extract company info from response data
	if agentResp.Data == nil {
		return nil, nil
	}

	info := &CompanyInfo{}
	if name, ok := agentResp.Data["name"].(string); ok {
		info.Name = name
	}
	if logoURL, ok := agentResp.Data["logo_url"].(string); ok {
		info.LogoURL = logoURL
	}
	if website, ok := agentResp.Data["website"].(string); ok {
		info.Website = website
	}
	if description, ok := agentResp.Data["description"].(string); ok {
		info.Description = description
	}

	// If we got at least a name, return the info
	if info.Name != "" {
		return info, nil
	}

	return nil, nil
}

// extractDomainFromURL extracts the domain from a URL
func extractDomainFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	host = strings.TrimPrefix(host, "www.")
	return host
}

// cleanCompanyName cleans up a company name from a page title
// Handles marketing taglines like "Invest Confidently with Webull" -> "Webull"
func cleanCompanyName(title string) string {
	result := strings.TrimSpace(title)
	if result == "" {
		return ""
	}

	// First, check for "with [Company]" pattern (e.g., "Invest Confidently with Webull")
	lowerResult := strings.ToLower(result)
	if idx := strings.LastIndex(lowerResult, " with "); idx > 0 {
		candidate := strings.TrimSpace(result[idx+6:])
		// Make sure it's not too long (likely just a company name)
		if len(candidate) > 0 && len(candidate) < 30 && (!strings.Contains(candidate, " ") || countWords(candidate) <= 3) {
			return candidate
		}
	}

	// Check for "by [Company]" pattern
	if idx := strings.LastIndex(lowerResult, " by "); idx > 0 {
		candidate := strings.TrimSpace(result[idx+4:])
		if len(candidate) > 0 && len(candidate) < 30 && countWords(candidate) <= 3 {
			return candidate
		}
	}

	// Remove common suffixes
	suffixes := []string{
		" | Official Site",
		" - Official Site",
		" | Home",
		" - Home",
		" | Official Website",
		" - Official Website",
		" | Welcome",
		" - Welcome",
		" - Home Page",
		" | Home Page",
	}

	for _, suffix := range suffixes {
		if idx := strings.Index(lowerResult, strings.ToLower(suffix)); idx > 0 {
			result = result[:idx]
			lowerResult = strings.ToLower(result)
		}
	}

	// Split on common separators and take the first part
	separators := []string{" | ", " - ", " – ", " — ", ": "}
	for _, sep := range separators {
		if parts := strings.Split(result, sep); len(parts) > 1 {
			// Take the shorter part that looks like a company name (usually first, but check both)
			first := strings.TrimSpace(parts[0])
			second := strings.TrimSpace(parts[1])

			// If first part is very long or looks like a tagline, prefer second
			if isLikelyTagline(first) && !isLikelyTagline(second) && len(second) < 30 {
				result = second
			} else {
				result = first
			}
			break
		}
	}

	// Remove trailing punctuation and common words
	result = strings.TrimRight(result, ".,!?")

	// If result is still too long, it might be a tagline - try to extract brand
	if len(result) > 40 || countWords(result) > 5 {
		// Look for capitalized words that might be the brand
		words := strings.Fields(result)
		for _, word := range words {
			if len(word) > 2 && word[0] >= 'A' && word[0] <= 'Z' && !isCommonWord(word) {
				// Could be the brand name
				return word
			}
		}
	}

	return strings.TrimSpace(result)
}

// countWords counts the number of words in a string
func countWords(s string) int {
	return len(strings.Fields(s))
}

// isLikelyTagline checks if a string looks like a marketing tagline
func isLikelyTagline(s string) bool {
	lower := strings.ToLower(s)
	taglineWords := []string{
		"welcome", "discover", "explore", "experience", "your", "the best",
		"official", "home of", "introducing", "meet", "get started",
		"invest", "shop", "buy", "find", "create", "build", "grow",
		"confidently", "easily", "simply", "better", "smarter",
	}
	for _, word := range taglineWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	// Also likely a tagline if it's very long
	return countWords(s) > 5
}

// isCommonWord checks if a word is a common English word (not a brand)
func isCommonWord(word string) bool {
	common := map[string]bool{
		"The": true, "And": true, "For": true, "Your": true, "Our": true,
		"With": true, "From": true, "This": true, "That": true, "Have": true,
		"More": true, "About": true, "Home": true, "Welcome": true, "New": true,
	}
	return common[word]
}

// domainToCompanyName converts a domain to a company name
// e.g., "webull.com" -> "Webull", "blue-bottle.com" -> "Blue Bottle"
func domainToCompanyName(domain string) string {
	if domain == "" {
		return ""
	}

	// Remove TLD
	parts := strings.Split(domain, ".")
	if len(parts) > 1 {
		domain = parts[0]
	}

	// Replace hyphens with spaces
	domain = strings.ReplaceAll(domain, "-", " ")
	domain = strings.ReplaceAll(domain, "_", " ")

	// Title case each word
	words := strings.Fields(domain)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}

// makeAbsoluteURL converts a potentially relative URL to an absolute URL
func makeAbsoluteURL(href, baseURL string) string {
	if href == "" {
		return ""
	}

	// Already absolute
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	// Parse the base URL
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}

	// Handle protocol-relative URLs (//example.com/path)
	if strings.HasPrefix(href, "//") {
		return base.Scheme + ":" + href
	}

	// Parse the relative URL
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}

	// Resolve against the base
	return base.ResolveReference(ref).String()
}

// isIcoFile returns true if the URL points to an .ico file
// We reject .ico files to ensure only high-quality logos are used
func isIcoFile(url string) bool {
	urlLower := strings.ToLower(url)
	// Check if URL ends with .ico or contains .ico? (with query params)
	return strings.HasSuffix(urlLower, ".ico") || strings.Contains(urlLower, ".ico?")
}

// isGenericSite returns true if the URL is a generic site we should skip
func isGenericSite(rawURL string) bool {
	genericDomains := []string{
		"facebook.com",
		"twitter.com",
		"x.com",
		"linkedin.com",
		"instagram.com",
		"youtube.com",
		"wikipedia.org",
		"yelp.com",
		"crunchbase.com",
		"bloomberg.com",
		"reuters.com",
		"glassdoor.com",
	}

	lowerURL := strings.ToLower(rawURL)
	for _, domain := range genericDomains {
		if strings.Contains(lowerURL, domain) {
			return true
		}
	}
	return false
}
