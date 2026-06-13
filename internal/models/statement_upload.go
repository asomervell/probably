package models

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StatementUploadStatus string

const (
	StatementUploadStatusPending    StatementUploadStatus = "pending"
	StatementUploadStatusProcessing StatementUploadStatus = "processing"
	StatementUploadStatusCompleted  StatementUploadStatus = "completed"
	StatementUploadStatusFailed     StatementUploadStatus = "failed"
)

type StatementUpload struct {
	ID               uuid.UUID             `json:"id"`
	LedgerID         uuid.UUID             `json:"ledger_id"`
	AccountID        *uuid.UUID            `json:"account_id,omitempty"`
	OriginalFilename string                `json:"original_filename"`
	GCSPath          string                `json:"gcs_path"`
	FileSizeBytes    int64                 `json:"file_size_bytes"`
	ContentType      string                `json:"content_type"`
	Status           StatementUploadStatus `json:"status"`
	ExtractedCount   int                   `json:"extracted_count"`
	CreatedCount     int                   `json:"created_count"`
	ErrorMessage     string                `json:"error_message,omitempty"`
	ProcessedAt      *time.Time            `json:"processed_at,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

type StatementUploadStore struct {
	pool *pgxpool.Pool
}

func NewStatementUploadStore(pool *pgxpool.Pool) *StatementUploadStore {
	return &StatementUploadStore{pool: pool}
}

func (s *StatementUploadStore) Create(ctx context.Context, upload *StatementUpload) error {
	if upload.ID == uuid.Nil {
		upload.ID = uuid.New()
	}

	now := time.Now()
	upload.CreatedAt = now
	upload.UpdatedAt = now

	_, err := s.pool.Exec(ctx, `
		INSERT INTO statement_uploads (
			id, ledger_id, account_id, original_filename, gcs_path,
			file_size_bytes, content_type, status, extracted_count, created_count,
			error_message, processed_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, upload.ID, upload.LedgerID, nullUUID(upload.AccountID), upload.OriginalFilename,
		upload.GCSPath, upload.FileSizeBytes, upload.ContentType, string(upload.Status),
		upload.ExtractedCount, upload.CreatedCount, NullString(upload.ErrorMessage),
		upload.ProcessedAt, upload.CreatedAt, upload.UpdatedAt)

	return err
}

func (s *StatementUploadStore) GetByID(ctx context.Context, id uuid.UUID) (*StatementUpload, error) {
	var upload StatementUpload
	var accountID sql.NullString
	var errorMsg sql.NullString
	var processedAt sql.NullTime

	err := s.pool.QueryRow(ctx, `
		SELECT id, ledger_id, account_id, original_filename, gcs_path,
			file_size_bytes, content_type, status, extracted_count, created_count,
			error_message, processed_at, created_at, updated_at
		FROM statement_uploads
		WHERE id = $1
	`, id).Scan(
		&upload.ID, &upload.LedgerID, &accountID, &upload.OriginalFilename, &upload.GCSPath,
		&upload.FileSizeBytes, &upload.ContentType, &upload.Status, &upload.ExtractedCount,
		&upload.CreatedCount, &errorMsg, &processedAt, &upload.CreatedAt, &upload.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if accountID.Valid {
		parsed, err := uuid.Parse(accountID.String)
		if err == nil {
			upload.AccountID = &parsed
		}
	}
	upload.ErrorMessage = errorMsg.String
	if processedAt.Valid {
		upload.ProcessedAt = &processedAt.Time
	}

	return &upload, nil
}

func (s *StatementUploadStore) GetByAccountID(ctx context.Context, accountID uuid.UUID) ([]*StatementUpload, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, account_id, original_filename, gcs_path,
			file_size_bytes, content_type, status, extracted_count, created_count,
			error_message, processed_at, created_at, updated_at
		FROM statement_uploads
		WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var uploads []*StatementUpload
	for rows.Next() {
		var upload StatementUpload
		var accountIDVal sql.NullString
		var errorMsg sql.NullString
		var processedAt sql.NullTime

		if err := rows.Scan(
			&upload.ID, &upload.LedgerID, &accountIDVal, &upload.OriginalFilename, &upload.GCSPath,
			&upload.FileSizeBytes, &upload.ContentType, &upload.Status, &upload.ExtractedCount,
			&upload.CreatedCount, &errorMsg, &processedAt, &upload.CreatedAt, &upload.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if accountIDVal.Valid {
			parsed, err := uuid.Parse(accountIDVal.String)
			if err == nil {
				upload.AccountID = &parsed
			}
		}
		upload.ErrorMessage = errorMsg.String
		if processedAt.Valid {
			upload.ProcessedAt = &processedAt.Time
		}

		uploads = append(uploads, &upload)
	}

	return uploads, rows.Err()
}

func (s *StatementUploadStore) UpdateStatus(ctx context.Context, id uuid.UUID, status StatementUploadStatus, extractedCount, createdCount int, errorMsg string) error {
	now := time.Now()
	var processedAt *time.Time
	if status == StatementUploadStatusCompleted || status == StatementUploadStatusFailed {
		processedAt = &now
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE statement_uploads SET
			status = $2,
			extracted_count = $3,
			created_count = $4,
			error_message = $5,
			processed_at = $6,
			updated_at = $7
		WHERE id = $1
	`, id, string(status), extractedCount, createdCount, NullString(errorMsg), processedAt, now)

	return err
}

func (s *StatementUploadStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM statement_uploads
		WHERE id = $1
	`, id)
	return err
}

func (s *StatementUploadStore) DeleteByStatus(ctx context.Context, accountID uuid.UUID, status StatementUploadStatus) (int64, error) {
	result, err := s.pool.Exec(ctx, `
		DELETE FROM statement_uploads
		WHERE account_id = $1 AND status = $2
	`, accountID, string(status))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func nullUUID(u *uuid.UUID) interface{} {
	if u == nil {
		return nil
	}
	return *u
}
