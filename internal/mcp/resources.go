package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResourceRegistry manages UI resources (HTML templates)
type ResourceRegistry struct {
	resources map[string]*Resource
	cdnDomain string // CDN domain for template replacement
}

// Resource represents a UI resource (HTML template)
type Resource struct {
	URI      string
	MimeType string
	Content  string
	Meta     map[string]interface{}
}

// NewResourceRegistry creates a new resource registry and loads HTML templates
// Requires baseURL and cdnDomain to be set via environment variables
func NewResourceRegistry(baseURL string, cdnDomain string) (*ResourceRegistry, error) {
	// Require baseURL to be set
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required: set MCP_BASE_URL or BASE_URL environment variable")
	}

	// Require cdnDomain to be set
	if cdnDomain == "" {
		return nil, fmt.Errorf("cdnDomain is required: set MCP_UI_CDN_URL or CDN_DOMAIN environment variable")
	}

	// Ensure CDN domain has protocol
	if !strings.HasPrefix(cdnDomain, "http") {
		cdnDomain = "https://" + cdnDomain
	}

	registry := &ResourceRegistry{
		resources: make(map[string]*Resource),
		cdnDomain: cdnDomain,
	}

	// Default CSP policy for widgets
	// Allow connections to ChatGPT and our API
	// Allow resources from OpenAI and our CDN
	resourceDomains := []string{"https://*.oaistatic.com", cdnDomain}
	defaultCSP := map[string]interface{}{
		"connect_domains":  []string{"https://chatgpt.com", baseURL},
		"resource_domains": resourceDomains,
	}

	// Widget domain must be set via environment variable
	defaultDomain := baseURL

	// Register UI resources
	// For now, we'll create placeholder resources
	// These will be replaced with actual HTML templates later
	resources := []*Resource{
		{
			URI:      "ui://widget/spending-summary.html",
			MimeType: "text/html+skybridge",
			Content:  placeholderHTML("Spending Summary"),
			Meta: map[string]interface{}{
				"openai/widgetDescription": "Spending summary by category",
				"openai/widgetCSP":         defaultCSP,
				"openai/widgetDomain":      defaultDomain,
			},
		},
		{
			URI:      "ui://widget/account-balances.html",
			MimeType: "text/html+skybridge",
			Content:  placeholderHTML("Account Balances"),
			Meta: map[string]interface{}{
				"openai/widgetDescription": "Account balances and net worth",
				"openai/widgetCSP":         defaultCSP,
				"openai/widgetDomain":      defaultDomain,
			},
		},
		{
			URI:      "ui://widget/ask-question.html",
			MimeType: "text/html+skybridge",
			Content:  placeholderHTML("Financial Insights"),
			Meta: map[string]interface{}{
				"openai/widgetDescription": "AI-powered financial insights",
				"openai/widgetAccessible":  true,
				"openai/widgetCSP":         defaultCSP,
				"openai/widgetDomain":      defaultDomain,
			},
		},
		{
			URI:      "ui://widget/spending-trends.html",
			MimeType: "text/html+skybridge",
			Content:  placeholderHTML("Spending Trends"),
			Meta: map[string]interface{}{
				"openai/widgetDescription": "Spending trends over time",
				"openai/widgetCSP":         defaultCSP,
				"openai/widgetDomain":      defaultDomain,
			},
		},
		{
			URI:      "ui://widget/recurring-patterns.html",
			MimeType: "text/html+skybridge",
			Content:  placeholderHTML("Recurring Patterns"),
			Meta: map[string]interface{}{
				"openai/widgetDescription": "Recurring subscriptions and bills",
				"openai/widgetCSP":         defaultCSP,
				"openai/widgetDomain":      defaultDomain,
			},
		},
		{
			URI:      "ui://widget/search-transactions.html",
			MimeType: "text/html+skybridge",
			Content:  placeholderHTML("Transaction Search"),
			Meta: map[string]interface{}{
				"openai/widgetDescription": "Transaction search results",
				"openai/widgetAccessible":  true,
				"openai/widgetCSP":         defaultCSP,
				"openai/widgetDomain":      defaultDomain,
			},
		},
		{
			URI:      "ui://widget/financial-overview.html",
			MimeType: "text/html+skybridge",
			Content:  placeholderHTML("Financial Overview"),
			Meta: map[string]interface{}{
				"openai/widgetDescription": "Financial dashboard overview",
				"openai/widgetCSP":         defaultCSP,
				"openai/widgetDomain":      defaultDomain,
			},
		},
	}

	for _, resource := range resources {
		registry.resources[resource.URI] = resource
	}

	// Try to load HTML templates from static directory
	registry.loadTemplatesFromDisk()

	return registry, nil
}

// GetResource returns a resource by URI
func (r *ResourceRegistry) GetResource(uri string) (*Resource, error) {
	resource, exists := r.resources[uri]
	if !exists {
		return nil, fmt.Errorf("resource not found: %s", uri)
	}
	return resource, nil
}

// ListResources returns all registered resources
func (r *ResourceRegistry) ListResources() []map[string]interface{} {
	resources := make([]map[string]interface{}, 0, len(r.resources))
	for _, resource := range r.resources {
		resourceMap := map[string]interface{}{
			"uri":      resource.URI,
			"mimeType": resource.MimeType,
			"name":     resource.URI,
		}
		// Include metadata if present
		if len(resource.Meta) > 0 {
			resourceMap["_meta"] = resource.Meta
		}
		resources = append(resources, resourceMap)
	}
	return resources
}

// loadTemplatesFromDisk attempts to load HTML templates from static/mcp-ui/
func (r *ResourceRegistry) loadTemplatesFromDisk() {
	baseDir := "static/mcp-ui"

	// Map of URI to filename
	templateMap := map[string]string{
		"ui://widget/spending-summary.html":    "spending-summary.html",
		"ui://widget/account-balances.html":    "account-balances.html",
		"ui://widget/ask-question.html":        "ask-question.html",
		"ui://widget/spending-trends.html":     "spending-trends.html",
		"ui://widget/recurring-patterns.html":  "recurring-patterns.html",
		"ui://widget/search-transactions.html": "search-transactions.html",
		"ui://widget/financial-overview.html":  "financial-overview.html",
	}

	for uri, filename := range templateMap {
		filePath := filepath.Join(baseDir, filename)
		content, err := os.ReadFile(filePath)
		if err == nil {
			// Replace CDN domain in template content
			contentStr := string(content)
			if r.cdnDomain != "" {
				// Replace any hardcoded CDN domain references with the configured CDN domain
				// This ensures templates use the correct domain from CDN_DOMAIN env var
				// Handles both https:// and protocol-less formats
				contentStr = strings.ReplaceAll(contentStr, "https://cdn.probably.money", r.cdnDomain)
				contentStr = strings.ReplaceAll(contentStr, "cdn.probably.money", strings.TrimPrefix(r.cdnDomain, "https://"))
			}
			// Update resource with file content
			if resource, exists := r.resources[uri]; exists {
				resource.Content = contentStr
			}
		}
		// Ignore errors - use default placeholder if file doesn't exist
	}
}

func placeholderHTML(title string) string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>` + title + `</title>
</head>
<body>
	<div id="root"></div>
	<script type="module">
		const data = window.openai?.toolOutput || {};
		document.getElementById('root').innerHTML = '<pre>' + JSON.stringify(data, null, 2) + '</pre>';
	</script>
</body>
</html>`
}
