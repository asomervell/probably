package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// LocalStorage implements Storage using local filesystem
// This is used for development and as a fallback
type LocalStorage struct {
	baseDir  string
	baseURL  string
	urlPrefix string
}

// NewLocalStorage creates a new local storage instance
// NOTE: This should not be used - cloud storage (GCS/S3) is required
func NewLocalStorage(baseDir, baseURL string) *LocalStorage {
	// No directory creation - local storage should not be used

	// Normalize baseURL - remove trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")
	urlPrefix := "/static/logos"

	return &LocalStorage{
		baseDir:   baseDir,
		baseURL:   baseURL,
		urlPrefix: urlPrefix,
	}
}

// Upload uploads a file to local storage
func (s *LocalStorage) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	filePath := filepath.Join(s.baseDir, key)
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return s.getURL(key), nil
}

// Exists checks if a file exists
func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	filePath := filepath.Join(s.baseDir, key)
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *LocalStorage) getURL(key string) string {
	// Use the configured CDN domain if set, otherwise use baseURL
	if s.baseURL != "" {
		return fmt.Sprintf("%s%s/%s", s.baseURL, s.urlPrefix, key)
	}
	return fmt.Sprintf("%s/%s", s.urlPrefix, key)
}

// Read reads a file from local storage
func (s *LocalStorage) Read(ctx context.Context, key string) ([]byte, error) {
	filePath := filepath.Join(s.baseDir, key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return data, nil
}

// UploadStatement uploads a statement file to local storage with RBAC path structure
// Path format: statements/{ledger_id}/{account_id}/{upload_id}/{filename}
// Note: This is for development only - production should use GCS/S3
func (s *LocalStorage) UploadStatement(ctx context.Context, ledgerID, accountID, uploadID uuid.UUID, filename string, data []byte, contentType string) (string, error) {
	// Sanitize filename to prevent path traversal
	filename = sanitizeFilename(filename)
	
	// Build path with RBAC structure
	path := filepath.Join("statements", ledgerID.String(), accountID.String(), uploadID.String(), filename)
	filePath := filepath.Join(s.baseDir, path)
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file (private, 0600 permissions)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Return the path (not a public URL, since statements are private)
	return path, nil
}

