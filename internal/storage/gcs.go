package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"google.golang.org/api/option"
)

// GCSStorage implements Storage using Google Cloud Storage
type GCSStorage struct {
	client     *storage.Client
	bucketName string
	cdnDomain  string
}

// NewGCSStorage creates a new GCS storage instance
func NewGCSStorage(ctx context.Context, bucketName, cdnDomain string, credentialsJSON []byte) (*GCSStorage, error) {
	var opts []option.ClientOption
	if len(credentialsJSON) > 0 {
		opts = append(opts, option.WithCredentialsJSON(credentialsJSON))
	}

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCSStorage{
		client:     client,
		bucketName: bucketName,
		cdnDomain:  cdnDomain,
	}, nil
}

// Upload uploads a file to GCS
func (s *GCSStorage) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(key)

	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType
	writer.CacheControl = "public, max-age=31536000" // 1 year cache

	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to write to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close GCS writer: %w", err)
	}

	return s.getURL(key), nil
}

// Exists checks if a file exists
func (s *GCSStorage) Exists(ctx context.Context, key string) (bool, error) {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(key)

	_, err := obj.Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *GCSStorage) getURL(key string) string {
	// Always use CDN domain if configured (preferred)
	if s.cdnDomain != "" {
		cdnDomain := strings.TrimSuffix(s.cdnDomain, "/")
		// Extract just the filename from the key (remove "logos/" prefix if present)
		filename := key
		if strings.HasPrefix(key, "logos/") {
			filename = strings.TrimPrefix(key, "logos/")
		}
		return fmt.Sprintf("%s/logos/%s", cdnDomain, filename)
	}

	// Fallback to GCS public URL (shouldn't happen if CDN is configured)
	// Extract just the filename from the key
	filename := key
	if strings.HasPrefix(key, "logos/") {
		filename = strings.TrimPrefix(key, "logos/")
	}
	return fmt.Sprintf("https://storage.googleapis.com/%s/logos/%s", s.bucketName, filename)
}

// Read reads a file from GCS
func (s *GCSStorage) Read(ctx context.Context, key string) ([]byte, error) {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(key)

	// Check if object exists first for better error message
	_, err := obj.Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil, fmt.Errorf("file does not exist in GCS (bucket: %s, path: %s)", s.bucketName, key)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check if file exists in GCS (bucket: %s, path: %s): %w", s.bucketName, key, err)
	}

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS reader (bucket: %s, path: %s): %w", s.bucketName, key, err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read from GCS (bucket: %s, path: %s): %w", s.bucketName, key, err)
	}

	return data, nil
}

// UploadStatement uploads a statement file to GCS with RBAC path structure
// Path format: statements/{ledger_id}/{account_id}/{upload_id}/{filename}
func (s *GCSStorage) UploadStatement(ctx context.Context, ledgerID, accountID, uploadID uuid.UUID, filename string, data []byte, contentType string) (string, error) {
	// Sanitize filename to prevent path traversal
	filename = sanitizeFilename(filename)
	
	// Build path with RBAC structure
	path := fmt.Sprintf("statements/%s/%s/%s/%s", 
		ledgerID.String(), 
		accountID.String(), 
		uploadID.String(), 
		filename)

	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(path)

	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType
	// Statements are private, no public cache
	writer.CacheControl = "private, no-cache"

	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to write statement to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close GCS writer: %w", err)
	}

	// Return the GCS path (not a public URL, since statements are private)
	return path, nil
}
