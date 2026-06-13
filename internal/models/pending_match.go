package models

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MatchStatus string

const (
	MatchStatusPending   MatchStatus = "pending"
	MatchStatusConfirmed MatchStatus = "confirmed"
	MatchStatusRejected  MatchStatus = "rejected"
)

// PendingTransferMatch represents a potential transfer match awaiting user review
type PendingTransferMatch struct {
	ID                     uuid.UUID   `json:"id"`
	TransactionID          uuid.UUID   `json:"transaction_id"`
	CandidateTransactionID uuid.UUID   `json:"candidate_transaction_id"`
	ConfidenceScore        float64     `json:"confidence_score"` // 0.00 to 1.00
	MatchReasons           []string    `json:"match_reasons"`
	Status                 MatchStatus `json:"status"`
	CreatedAt              time.Time   `json:"created_at"`
	ReviewedAt             *time.Time  `json:"reviewed_at,omitempty"`

	// Loaded separately for display
	Transaction          *Transaction `json:"transaction,omitempty"`
	CandidateTransaction *Transaction `json:"candidate_transaction,omitempty"`
}

type PendingMatchStore struct {
	pool *pgxpool.Pool
}

func NewPendingMatchStore(pool *pgxpool.Pool) *PendingMatchStore {
	return &PendingMatchStore{pool: pool}
}

// Create creates a new pending transfer match
func (s *PendingMatchStore) Create(ctx context.Context, match *PendingTransferMatch) error {
	if match.ID == uuid.Nil {
		match.ID = uuid.New()
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO pending_transfer_matches (id, transaction_id, candidate_transaction_id,
			confidence_score, match_reasons, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (transaction_id, candidate_transaction_id) DO UPDATE SET
			confidence_score = EXCLUDED.confidence_score,
			match_reasons = EXCLUDED.match_reasons
	`, match.ID, match.TransactionID, match.CandidateTransactionID,
		match.ConfidenceScore, match.MatchReasons, match.Status, time.Now())

	return err
}

// UpdateStatus updates the status of a pending match
func (s *PendingMatchStore) UpdateStatus(ctx context.Context, id uuid.UUID, status MatchStatus) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE pending_transfer_matches SET status = $2, reviewed_at = $3 WHERE id = $1
	`, id, status, now)
	return err
}

// GetByID retrieves a pending match by ID
func (s *PendingMatchStore) GetByID(ctx context.Context, id uuid.UUID) (*PendingTransferMatch, error) {
	var m PendingTransferMatch
	var reviewedAt sql.NullTime

	err := s.pool.QueryRow(ctx, `
		SELECT id, transaction_id, candidate_transaction_id, confidence_score, match_reasons,
			status, created_at, reviewed_at
		FROM pending_transfer_matches WHERE id = $1
	`, id).Scan(&m.ID, &m.TransactionID, &m.CandidateTransactionID, &m.ConfidenceScore,
		&m.MatchReasons, &m.Status, &m.CreatedAt, &reviewedAt)

	if err != nil {
		return nil, err
	}

	if reviewedAt.Valid {
		m.ReviewedAt = &reviewedAt.Time
	}

	return &m, nil
}

// GetPendingByLedgerID retrieves all pending matches for a ledger
func (s *PendingMatchStore) GetPendingByLedgerID(ctx context.Context, ledgerID uuid.UUID) ([]*PendingTransferMatch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT pm.id, pm.transaction_id, pm.candidate_transaction_id, pm.confidence_score,
			pm.match_reasons, pm.status, pm.created_at, pm.reviewed_at
		FROM pending_transfer_matches pm
		JOIN transactions t ON pm.transaction_id = t.id
		WHERE t.ledger_id = $1 AND pm.status = 'pending'
		ORDER BY pm.confidence_score DESC, pm.created_at DESC
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanMatches(rows)
}

// GetByTransactionID retrieves pending matches involving a specific transaction
func (s *PendingMatchStore) GetByTransactionID(ctx context.Context, txnID uuid.UUID) ([]*PendingTransferMatch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, transaction_id, candidate_transaction_id, confidence_score, match_reasons,
			status, created_at, reviewed_at
		FROM pending_transfer_matches
		WHERE (transaction_id = $1 OR candidate_transaction_id = $1) AND status = 'pending'
		ORDER BY confidence_score DESC
	`, txnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanMatches(rows)
}

// Delete removes a pending match
func (s *PendingMatchStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM pending_transfer_matches WHERE id = $1`, id)
	return err
}

// DeleteByTransactionID removes all pending matches involving a transaction
func (s *PendingMatchStore) DeleteByTransactionID(ctx context.Context, txnID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM pending_transfer_matches 
		WHERE transaction_id = $1 OR candidate_transaction_id = $1
	`, txnID)
	return err
}

// CountPending returns the count of pending matches for a ledger
func (s *PendingMatchStore) CountPending(ctx context.Context, ledgerID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pending_transfer_matches pm
		JOIN transactions t ON pm.transaction_id = t.id
		WHERE t.ledger_id = $1 AND pm.status = 'pending'
	`, ledgerID).Scan(&count)
	return count, err
}

// Exists checks if a match already exists between two transactions
func (s *PendingMatchStore) Exists(ctx context.Context, txn1ID, txn2ID uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM pending_transfer_matches 
			WHERE (transaction_id = $1 AND candidate_transaction_id = $2)
			   OR (transaction_id = $2 AND candidate_transaction_id = $1)
		)
	`, txn1ID, txn2ID).Scan(&exists)
	return exists, err
}

func (s *PendingMatchStore) scanMatches(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]*PendingTransferMatch, error) {
	var matches []*PendingTransferMatch

	for rows.Next() {
		var m PendingTransferMatch
		var reviewedAt sql.NullTime

		if err := rows.Scan(&m.ID, &m.TransactionID, &m.CandidateTransactionID, &m.ConfidenceScore,
			&m.MatchReasons, &m.Status, &m.CreatedAt, &reviewedAt); err != nil {
			return nil, err
		}

		if reviewedAt.Valid {
			m.ReviewedAt = &reviewedAt.Time
		}

		matches = append(matches, &m)
	}

	return matches, rows.Err()
}

// LoadTransactions loads the full transaction data for a pending match
func (s *PendingMatchStore) LoadTransactions(ctx context.Context, match *PendingTransferMatch, txnStore *TransactionStore) error {
	txn, err := txnStore.GetByID(ctx, match.TransactionID)
	if err != nil {
		return err
	}
	match.Transaction = txn

	candidate, err := txnStore.GetByID(ctx, match.CandidateTransactionID)
	if err != nil {
		return err
	}
	match.CandidateTransaction = candidate

	return nil
}
