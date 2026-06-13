package enrichment

import (
	"strings"
)

// GetLogoURL constructs the full CDN URL from a filename
// If logoURL is already a full URL (http/https), returns it as-is
// Otherwise treats it as a filename and constructs the CDN URL
func GetLogoURL(logoURL string, cdnDomain string) string {
	if logoURL == "" {
		return ""
	}

	// If it's already a full URL (external or CDN), return as-is
	if strings.HasPrefix(logoURL, "http://") || strings.HasPrefix(logoURL, "https://") {
		return logoURL
	}

	// Extract filename from path if needed (handles old /static/logos/abc.png format)
	filename := logoURL
	if strings.Contains(logoURL, "/") {
		parts := strings.Split(logoURL, "/")
		filename = parts[len(parts)-1]
	}

	// Construct CDN URL from filename
	if cdnDomain != "" {
		cdnDomain = strings.TrimSuffix(cdnDomain, "/")
		return cdnDomain + "/logos/" + filename
	}

	// Fallback: return as relative path (shouldn't happen if CDN is configured)
	return "/logos/" + filename
}
