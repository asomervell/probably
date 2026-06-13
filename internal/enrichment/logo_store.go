package enrichment

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/storage"
)

// LogoStore handles downloading and storing logos
type LogoStore struct {
	httpClient *http.Client
	storage    storage.Storage
	storageDir string // e.g., "static/logos" (for local fallback)
	urlPrefix  string // e.g., "/static/logos" (for local fallback)
}

// NewLogoStore creates a new logo store with cloud storage
func NewLogoStore(storage storage.Storage, storageDir, urlPrefix string) *LogoStore {
	// No local directory creation - we always use cloud storage (GCS/S3)

	return &LogoStore{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		storage:    storage,
		storageDir: storageDir,
		urlPrefix:  urlPrefix,
	}
}

// DownloadAndStore downloads a logo from a URL and stores it globally (not entity-specific).
// Returns just the filename (e.g., "abc123.png") — the full URL is constructed at display time.
func (s *LogoStore) DownloadAndStore(ctx context.Context, sourceURL string) (string, error) {
	if sourceURL == "" {
		return "", fmt.Errorf("empty source URL")
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set a user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ProbablyBot/1.0)")
	req.Header.Set("Accept", "image/*")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download logo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download logo: status %d", resp.StatusCode)
	}

	// Read the image data
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read logo data: %w", err)
	}

	// Validate it's actually an image
	contentType := resp.Header.Get("Content-Type")
	ext := getExtensionFromContentType(contentType, sourceURL)
	if ext == "" {
		return "", fmt.Errorf("unsupported content type: %s", contentType)
	}

	// Generate filename using hash of content (global, not entity-specific)
	// Use full hash to ensure uniqueness and avoid collisions
	hash := sha256.Sum256(data)
	filename := fmt.Sprintf("%x%s", hash, ext)
	storageKey := "logos/" + filename

	// Check if file already exists in cloud storage
	exists, err := s.storage.Exists(ctx, storageKey)
	if err == nil && exists {
		slog.DebugContext(ctx, "logo already exists", "filename", filename)
		return filename, nil
	}

	// Upload to cloud storage (required - no local fallback)
	slog.DebugContext(ctx, "uploading logo to storage", "storage_key", storageKey, "size", len(data))
	_, err = s.storage.Upload(ctx, storageKey, data, contentType)
	if err != nil {
		// No fallback - cloud storage is required
		return "", fmt.Errorf("failed to upload logo to cloud storage: %w", err)
	}
	slog.DebugContext(ctx, "logo uploaded", "filename", filename)

	// Return just the filename - URL will be constructed when displaying
	return filename, nil
}

// DownloadFromLogoDevAndStore downloads a logo from logo.dev and stores it.
func (s *LogoStore) DownloadFromLogoDevAndStore(ctx context.Context, domain, publishableKey string) (string, error) {
	if domain == "" || publishableKey == "" {
		return "", fmt.Errorf("domain or key is empty")
	}
	logoURL := fmt.Sprintf("https://img.logo.dev/%s?token=%s&format=png", domain, publishableKey)
	return s.DownloadAndStore(ctx, logoURL)
}

// StoreBase64Logo stores a base64-encoded logo (e.g., from Plaid's data URL format).
// Returns just the filename (e.g., "abc123.png").
func (s *LogoStore) StoreBase64Logo(ctx context.Context, base64Data string) (string, error) {
	if base64Data == "" {
		return "", fmt.Errorf("empty base64 data")
	}

	// Parse data URL format: "data:image/png;base64,iVBORw0KGgo..."
	var data []byte
	var contentType string
	var ext string

	if strings.HasPrefix(base64Data, "data:") {
		// Parse data URL
		parts := strings.SplitN(base64Data, ",", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid data URL format")
		}
		
		header := parts[0]
		base64Content := parts[1]
		
		// Extract content type from header (e.g., "data:image/png;base64")
		if strings.Contains(header, "image/png") {
			contentType = "image/png"
			ext = ".png"
		} else if strings.Contains(header, "image/jpeg") || strings.Contains(header, "image/jpg") {
			contentType = "image/jpeg"
			ext = ".jpg"
		} else if strings.Contains(header, "image/gif") {
			contentType = "image/gif"
			ext = ".gif"
		} else if strings.Contains(header, "image/webp") {
			contentType = "image/webp"
			ext = ".webp"
		} else if strings.Contains(header, "image/svg+xml") {
			contentType = "image/svg+xml"
			ext = ".svg"
		} else {
			return "", fmt.Errorf("unsupported image type in data URL: %s", header)
		}

		// Decode base64
		decoded, err := base64.StdEncoding.DecodeString(base64Content)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64: %w", err)
		}
		data = decoded
	} else {
		// Assume it's raw base64 without data URL prefix - default to PNG
		decoded, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64: %w", err)
		}
		data = decoded
		contentType = "image/png"
		ext = ".png"
	}

	if len(data) == 0 {
		return "", fmt.Errorf("decoded data is empty")
	}

	// Generate filename using hash of content
	hash := sha256.Sum256(data)
	filename := fmt.Sprintf("%x%s", hash, ext)
	storageKey := "logos/" + filename

	// Check if file already exists in cloud storage
	exists, err := s.storage.Exists(ctx, storageKey)
	if err == nil && exists {
		slog.DebugContext(ctx, "logo already exists", "filename", filename)
		return filename, nil
	}

	// Upload to cloud storage
	slog.DebugContext(ctx, "uploading base64 logo to storage", "storage_key", storageKey, "size", len(data))
	_, err = s.storage.Upload(ctx, storageKey, data, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to upload logo to cloud storage: %w", err)
	}
	slog.DebugContext(ctx, "base64 logo uploaded", "filename", filename)

	// Return just the filename
	return filename, nil
}

// getExtensionFromContentType returns the file extension for a content type
// Rejects .ico files to ensure only high-quality logos are stored
func getExtensionFromContentType(contentType, url string) string {
	// Normalize content type
	contentType = strings.ToLower(strings.Split(contentType, ";")[0])

	// CRITICAL: Reject non-image content types immediately
	// This prevents saving HTML error pages as images
	if contentType == "text/html" || contentType == "text/plain" || contentType == "application/json" {
		return "" // Not an image
	}

	// CRITICAL: Reject .ico files - we only want high-quality logos
	// Check URL first to catch .ico files even if content-type is generic
	urlLower := strings.ToLower(url)
	if strings.HasSuffix(urlLower, ".ico") || strings.Contains(urlLower, ".ico?") {
		return "" // Reject .ico files
	}

	switch contentType {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "image/x-icon", "image/vnd.microsoft.icon":
		// Reject .ico content types - we only want high-quality logos
		return ""
	}

	// Only use URL hints if Content-Type indicates it's an image
	// This prevents "format=png" query params from tricking us when
	// the server returns an HTML error page
	if !strings.HasPrefix(contentType, "image/") {
		return ""
	}

	// Try to get extension from URL for generic image types
	// Skip .ico files
	if strings.Contains(urlLower, ".png") {
		return ".png"
	}
	if strings.Contains(urlLower, ".jpg") || strings.Contains(urlLower, ".jpeg") {
		return ".jpg"
	}
	if strings.Contains(urlLower, ".svg") {
		return ".svg"
	}
	if strings.Contains(urlLower, ".webp") {
		return ".webp"
	}
	if strings.Contains(urlLower, ".gif") {
		return ".gif"
	}
	// Explicitly skip .ico - we don't want low-quality favicons

	// Default to png for unknown image types
	return ".png"
}

// IsLocalLogo checks if a logo is already stored in our storage
// Now that we store just filenames, this checks if it's a filename (not a full URL)
// Returns true if it's a filename (no http/https, no path separators), false if it's an external URL
func (s *LogoStore) IsLocalLogo(logoURL string) bool {
	if logoURL == "" {
		return false
	}
	
	// If it starts with http:// or https://, it's an external URL
	if strings.HasPrefix(logoURL, "http://") || strings.HasPrefix(logoURL, "https://") {
		return false
	}
	
	// If it contains a path separator (old format like /static/logos/abc.png), extract filename
	if strings.Contains(logoURL, "/") {
		// Extract filename from path
		parts := strings.Split(logoURL, "/")
		filename := parts[len(parts)-1]
		// If it looks like a filename (has extension), it's local
		return strings.Contains(filename, ".")
	}
	
	// If it's just a filename (no path, no protocol), it's local
	return strings.Contains(logoURL, ".")
}

// GetStorage returns the underlying storage instance
func (s *LogoStore) GetStorage() storage.Storage {
	return s.storage
}

