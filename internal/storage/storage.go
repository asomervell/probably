package storage

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// Storage is an interface for cloud storage backends
type Storage interface {
	// Upload uploads a file to cloud storage and returns the public URL
	Upload(ctx context.Context, key string, data []byte, contentType string) (string, error)

	// UploadStatement uploads a statement file with RBAC path structure
	// Returns the GCS path (not a public URL, as statements are private)
	UploadStatement(ctx context.Context, ledgerID, accountID, uploadID uuid.UUID, filename string, data []byte, contentType string) (string, error)

	// Read reads a file from storage
	Read(ctx context.Context, key string) ([]byte, error)

	// Exists checks if a file exists in storage
	Exists(ctx context.Context, key string) (bool, error)
}

// sanitizeFilename removes dangerous characters from filenames
func sanitizeFilename(filename string) string {
	// Remove path separators and other dangerous characters
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = strings.ReplaceAll(filename, "..", "_")
	filename = strings.ReplaceAll(filename, "\x00", "_")
	
	// Limit length
	if len(filename) > 255 {
		filename = filename[:255]
	}
	
	return filename
}
