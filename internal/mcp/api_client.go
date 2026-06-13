package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/config"
)

// APIClient handles HTTP requests to existing /api/v1/* endpoints
type APIClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewAPIClient creates a new API client
func NewAPIClient(cfg *config.Config) *APIClient {
	return &APIClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: cfg.BaseURL,
	}
}

// Get makes a GET request to an API endpoint
// token can be either an OAuth access token or API key (for backward compatibility)
func (c *APIClient) Get(ctx context.Context, path string, token string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add Bearer token authentication (OAuth token or API key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// Post makes a POST request to an API endpoint
// token can be either an OAuth access token or API key (for backward compatibility)
func (c *APIClient) Post(ctx context.Context, path string, token string, body interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add Bearer token authentication (OAuth token or API key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	// For chat/ask endpoint, explicitly request JSON (not SSE)
	if strings.Contains(path, "/chat/ask") {
		req.Header.Set("Accept", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}
