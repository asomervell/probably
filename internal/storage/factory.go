package storage

import (
	"context"
	"fmt"
	"os"
)

// StorageType represents the type of storage backend
type StorageType string

const (
	StorageTypeLocal StorageType = "local"
	StorageTypeGCS   StorageType = "gcs"
	StorageTypeS3    StorageType = "s3"
)

// NewStorage creates a storage instance based on configuration
func NewStorage(ctx context.Context, storageType, bucketName, region, endpoint, accessKeyID, secretAccessKey, baseURL, cdnDomain string, gcsCredentialsJSON []byte) (Storage, error) {
	switch StorageType(storageType) {
	case StorageTypeGCS:
		if bucketName == "" {
			return nil, fmt.Errorf("GCS bucket name is required")
		}
		return NewGCSStorage(ctx, bucketName, cdnDomain, gcsCredentialsJSON)

	case StorageTypeS3:
		if bucketName == "" {
			return nil, fmt.Errorf("S3 bucket name is required")
		}
		if accessKeyID == "" || secretAccessKey == "" {
			return nil, fmt.Errorf("S3 access key ID and secret access key are required")
		}
		return NewS3Storage(ctx, bucketName, region, endpoint, accessKeyID, secretAccessKey, cdnDomain)
	
	case StorageTypeLocal:
		// Local storage only for development
		storageDir := "static/logos"
		return NewLocalStorage(storageDir, baseURL), nil
	case "":
		// No storage type specified - require bucket configuration
		if bucketName == "" {
			return nil, fmt.Errorf("storage configuration required: set STORAGE_TYPE and STORAGE_BUCKET, or STORAGE_BUCKET (defaults to GCS)")
		}
		// Default to GCS if bucket is set
		return NewGCSStorage(ctx, bucketName, cdnDomain, gcsCredentialsJSON)
	
	default:
		return nil, fmt.Errorf("unknown storage type: %s", storageType)
	}
}

// NewStorageFromEnv creates a storage instance from environment variables
func NewStorageFromEnv(ctx context.Context, baseURL string) (Storage, error) {
	storageType := os.Getenv("STORAGE_TYPE")
	bucketName := os.Getenv("STORAGE_BUCKET")
	region := os.Getenv("STORAGE_REGION")
	endpoint := os.Getenv("STORAGE_ENDPOINT") // For S3-compatible storage
	accessKeyID := os.Getenv("STORAGE_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("STORAGE_SECRET_ACCESS_KEY")
	cdnDomain := os.Getenv("CDN_DOMAIN")
	
	var gcsCredentialsJSON []byte
	if gcsCreds := os.Getenv("GCS_CREDENTIALS_JSON"); gcsCreds != "" {
		gcsCredentialsJSON = []byte(gcsCreds)
	}

	return NewStorage(ctx, storageType, bucketName, region, endpoint, accessKeyID, secretAccessKey, baseURL, cdnDomain, gcsCredentialsJSON)
}
