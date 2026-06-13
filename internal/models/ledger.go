package models

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Ledger struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LedgerStore struct {
	pool *pgxpool.Pool
}

func NewLedgerStore(pool *pgxpool.Pool) *LedgerStore {
	return &LedgerStore{pool: pool}
}

func (s *LedgerStore) Create(ctx context.Context, ledger *Ledger) error {
	if ledger.ID == uuid.Nil {
		ledger.ID = uuid.New()
	}
	if ledger.Currency == "" {
		ledger.Currency = "USD"
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO ledgers (id, user_id, name, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, ledger.ID, ledger.UserID, ledger.Name, ledger.Currency, time.Now(), time.Now())

	return err
}

func (s *LedgerStore) Update(ctx context.Context, ledger *Ledger) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE ledgers SET name = $2, currency = $3, updated_at = $4
		WHERE id = $1
	`, ledger.ID, ledger.Name, ledger.Currency, time.Now())

	return err
}

func (s *LedgerStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM ledgers WHERE id = $1`, id)
	return err
}

func (s *LedgerStore) GetByID(ctx context.Context, id uuid.UUID) (*Ledger, error) {
	var l Ledger
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, name, currency, created_at, updated_at
		FROM ledgers WHERE id = $1
	`, id).Scan(&l.ID, &l.UserID, &l.Name, &l.Currency, &l.CreatedAt, &l.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return &l, nil
}

// GetByUserID gets the first ledger for a user (ordered by creation date)
func (s *LedgerStore) GetByUserID(ctx context.Context, userID uuid.UUID) (*Ledger, error) {
	var l Ledger
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, name, currency, created_at, updated_at
		FROM ledgers WHERE user_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, userID).Scan(&l.ID, &l.UserID, &l.Name, &l.Currency, &l.CreatedAt, &l.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return &l, nil
}

// GetOrCreateDefault gets the user's default ledger, or creates one if it doesn't exist
// Returns the ledger and a boolean indicating if it was newly created
func (s *LedgerStore) GetOrCreateDefault(ctx context.Context, userID uuid.UUID) (*Ledger, bool, error) {
	// Try to get existing ledger
	ledger, err := s.GetByUserID(ctx, userID)
	if err == nil {
		return ledger, false, nil
	}

	// If no ledger exists, create a default one
	ledger = &Ledger{
		UserID:   userID,
		Name:     "Personal",
		Currency: "USD",
	}
	if err := s.Create(ctx, ledger); err != nil {
		return nil, false, err
	}

	return ledger, true, nil
}
