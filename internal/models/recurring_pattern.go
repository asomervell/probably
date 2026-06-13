package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RecurringPattern struct {
	ID             uuid.UUID  `json:"id"`
	LedgerID       uuid.UUID  `json:"ledger_id"`
	EntityID       *uuid.UUID `json:"entity_id,omitempty"`
	Frequency      string     `json:"frequency,omitempty"` // weekly, biweekly, monthly, etc.
	PredictedDates []string   `json:"predicted_dates,omitempty"`
	AvgAmountCents int64      `json:"avg_amount_cents,omitempty"`
	LastSeenAt     *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Loaded separately
	Entity *Entity `json:"entity,omitempty"`
}

type RecurringPatternStore struct {
	pool *pgxpool.Pool
}

func NewRecurringPatternStore(pool *pgxpool.Pool) *RecurringPatternStore {
	return &RecurringPatternStore{pool: pool}
}

func (s *RecurringPatternStore) Create(ctx context.Context, p *RecurringPattern) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	predictedDatesJSON, _ := json.Marshal(p.PredictedDates)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO recurring_patterns (id, ledger_id, entity_id, frequency,
			predicted_dates, avg_amount_cents, last_seen_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, p.ID, p.LedgerID, p.EntityID, NullString(p.Frequency),
		predictedDatesJSON, NullInt64(p.AvgAmountCents), p.LastSeenAt, p.CreatedAt, p.UpdatedAt)

	return err
}

func (s *RecurringPatternStore) Update(ctx context.Context, p *RecurringPattern) error {
	p.UpdatedAt = time.Now()

	predictedDatesJSON, _ := json.Marshal(p.PredictedDates)

	_, err := s.pool.Exec(ctx, `
		UPDATE recurring_patterns SET 
			entity_id = $2, frequency = $3, predicted_dates = $4,
			avg_amount_cents = $5, last_seen_at = $6, updated_at = $7
		WHERE id = $1
	`, p.ID, p.EntityID, NullString(p.Frequency), predictedDatesJSON,
		NullInt64(p.AvgAmountCents), p.LastSeenAt, p.UpdatedAt)

	return err
}

func (s *RecurringPatternStore) GetByID(ctx context.Context, id uuid.UUID) (*RecurringPattern, error) {
	var p RecurringPattern
	var frequency sql.NullString
	var predictedDatesJSON []byte
	var avgAmount sql.NullInt64
	var lastSeen sql.NullTime

	err := s.pool.QueryRow(ctx, `
		SELECT id, ledger_id, entity_id, frequency,
			predicted_dates, avg_amount_cents, last_seen_at, created_at, updated_at
		FROM recurring_patterns WHERE id = $1
	`, id).Scan(&p.ID, &p.LedgerID, &p.EntityID, &frequency,
		&predictedDatesJSON, &avgAmount, &lastSeen, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return nil, err
	}

	p.Frequency = frequency.String
	p.AvgAmountCents = avgAmount.Int64
	if lastSeen.Valid {
		p.LastSeenAt = &lastSeen.Time
	}
	if len(predictedDatesJSON) > 0 {
		_ = json.Unmarshal(predictedDatesJSON, &p.PredictedDates)
	}

	return &p, nil
}

// Upsert creates or updates a recurring pattern by ledger + entity
func (s *RecurringPatternStore) Upsert(ctx context.Context, p *RecurringPattern) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now()
	p.UpdatedAt = now

	predictedDatesJSON, _ := json.Marshal(p.PredictedDates)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO recurring_patterns (id, ledger_id, entity_id, frequency,
			predicted_dates, avg_amount_cents, last_seen_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			entity_id = EXCLUDED.entity_id,
			frequency = EXCLUDED.frequency,
			predicted_dates = EXCLUDED.predicted_dates,
			avg_amount_cents = EXCLUDED.avg_amount_cents,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = EXCLUDED.updated_at
	`, p.ID, p.LedgerID, p.EntityID, NullString(p.Frequency),
		predictedDatesJSON, NullInt64(p.AvgAmountCents), p.LastSeenAt, now, now)

	return err
}

// GetByLedgerID returns all recurring patterns for a ledger
func (s *RecurringPatternStore) GetByLedgerID(ctx context.Context, ledgerID uuid.UUID) ([]*RecurringPattern, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT rp.id, rp.ledger_id, rp.entity_id, rp.frequency,
			rp.predicted_dates, rp.avg_amount_cents, rp.last_seen_at, rp.created_at, rp.updated_at,
			e.id, e.name, e.logo_url
		FROM recurring_patterns rp
		LEFT JOIN entities e ON rp.entity_id = e.id
		WHERE rp.ledger_id = $1
		ORDER BY rp.frequency, e.name
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patterns []*RecurringPattern
	for rows.Next() {
		var p RecurringPattern
		var frequency sql.NullString
		var predictedDatesJSON []byte
		var avgAmount sql.NullInt64
		var lastSeen sql.NullTime
		var entityID, entityName, entityLogo sql.NullString

		if err := rows.Scan(&p.ID, &p.LedgerID, &p.EntityID, &frequency,
			&predictedDatesJSON, &avgAmount, &lastSeen, &p.CreatedAt, &p.UpdatedAt,
			&entityID, &entityName, &entityLogo); err != nil {
			return nil, err
		}

		p.Frequency = frequency.String
		p.AvgAmountCents = avgAmount.Int64
		if lastSeen.Valid {
			p.LastSeenAt = &lastSeen.Time
		}
		if len(predictedDatesJSON) > 0 {
			_ = json.Unmarshal(predictedDatesJSON, &p.PredictedDates)
		}

		// Include entity info if available
		if entityID.Valid && entityName.Valid {
			eid, _ := uuid.Parse(entityID.String)
			p.Entity = &Entity{
				ID:      eid,
				Name:    entityName.String,
				LogoURL: entityLogo.String,
			}
		}

		patterns = append(patterns, &p)
	}

	return patterns, rows.Err()
}

// Delete removes a recurring pattern
func (s *RecurringPatternStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM recurring_patterns WHERE id = $1`, id)
	return err
}
