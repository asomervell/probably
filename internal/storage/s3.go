package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// S3Storage implements Storage using AWS S3 or S3-compatible storage
type S3Storage struct {
	client     *s3.Client
	bucketName string
	cdnDomain  string
	region     string
}

// NewS3Storage creates a new S3 storage instance
func NewS3Storage(ctx context.Context, bucketName, region, endpoint, accessKeyID, secretAccessKey, cdnDomain string) (*S3Storage, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	var client *s3.Client
	if endpoint != "" {
		// S3-compatible storage (e.g., DigitalOcean Spaces, MinIO)
		client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	} else {
		// AWS S3
		client = s3.NewFromConfig(cfg)
	}

	return &S3Storage{
		client:     client,
		bucketName: bucketName,
		cdnDomain:  cdnDomain,
		region:     region,
	}, nil
}

// Upload uploads a file to S3
func (s *S3Storage) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	// Key already includes "logos/" prefix from logo_store
	s3Key := key
	if !strings.HasPrefix(key, "logos/") {
		s3Key = "logos/" + key
	}
	
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(s.bucketName),
		Key:          aws.String(s3Key),
		Body:         bytes.NewReader(data),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=31536000"), // 1 year cache
		ACL:          "public-read",
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return s.getURL(key), nil
}

// Exists checks if a file exists
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String("logos/" + key),
	})

	if err != nil {
		// Check if it's a "not found" error - check error message
		errStr := err.Error()
		if strings.Contains(errStr, "NoSuchKey") || strings.Contains(errStr, "NotFound") || strings.Contains(errStr, "404") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (s *S3Storage) getURL(key string) string {
	// Extract filename from key (remove "logos/" prefix if present)
	filename := key
	if strings.HasPrefix(key, "logos/") {
		filename = strings.TrimPrefix(key, "logos/")
	}
	
	// Always use CDN domain if configured (preferred)
	if s.cdnDomain != "" {
		cdnDomain := strings.TrimSuffix(s.cdnDomain, "/")
		return fmt.Sprintf("%s/logos/%s", cdnDomain, filename)
	}

	// Fallback to S3 public URL
	if s.region == "us-east-1" {
		return fmt.Sprintf("https://%s.s3.amazonaws.com/logos/%s", s.bucketName, filename)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/logos/%s", s.bucketName, s.region, filename)
}

// UploadStatement uploads a statement file to S3 with RBAC path structure
// Path format: statements/{ledger_id}/{account_id}/{upload_id}/{filename}
func (s *S3Storage) UploadStatement(ctx context.Context, ledgerID, accountID, uploadID uuid.UUID, filename string, data []byte, contentType string) (string, error) {
	// Sanitize filename to prevent path traversal
	filename = sanitizeFilename(filename)
	
	// Build path with RBAC structure
	path := fmt.Sprintf("statements/%s/%s/%s/%s", 
		ledgerID.String(), 
		accountID.String(), 
		uploadID.String(), 
		filename)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(s.bucketName),
		Key:          aws.String(path),
		Body:         bytes.NewReader(data),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("private, no-cache"),
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload statement to S3: %w", err)
	}

	// Return the S3 path (not a public URL, since statements are private)
	return path, nil
}

// Read reads a file from S3
func (s *S3Storage) Read(ctx context.Context, key string) ([]byte, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read from S3: %w", err)
	}

	return data, nil
}

