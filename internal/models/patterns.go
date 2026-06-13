package models

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AggregatedPattern represents a pattern aggregated from transactions
// This is computed directly from the transactions table, not stored separately
type AggregatedPattern struct {
	// Identity
	PatternType string     `json:"pattern_type"`           // salary, recurring_bill, account_transfer, etc.
	PatternName string     `json:"pattern_name,omitempty"` // Specific subscription name (e.g., "Google Workspace", "iCloud Storage")
	EntityID    *uuid.UUID `json:"entity_id,omitempty"`
	EntityName  string     `json:"entity_name,omitempty"`
	EntityLogo  string     `json:"entity_logo,omitempty"`

	// Pattern details
	Frequency        string `json:"frequency,omitempty"` // weekly, biweekly, monthly, quarterly, annual
	AvgAmountCents   int64  `json:"avg_amount_cents"`
	TotalAmountCents int64  `json:"total_amount_cents"`
	OccurrenceCount  int    `json:"occurrence_count"`
	AvgConfidence    int    `json:"avg_confidence"`

	// Dates
	FirstOccurrence time.Time  `json:"first_occurrence"`
	LastOccurrence  time.Time  `json:"last_occurrence"`
	NextExpected    *time.Time `json:"next_expected,omitempty"`

	// For display
	IsSubscription bool `json:"is_subscription"`
}

// EntityPatternGroup represents an entity with multiple subscriptions/patterns
type EntityPatternGroup struct {
	EntityID   *uuid.UUID
	EntityName string
	EntityLogo string
	Patterns   []*AggregatedPattern
	// Computed totals for the entity
	TotalMonthlyCents int64
	TotalOccurrences  int
}

// PatternStore provides access to aggregated pattern data from transactions
type PatternStore struct {
	pool *pgxpool.Pool
}

// NewPatternStore creates a new pattern store
func NewPatternStore(pool *pgxpool.Pool) *PatternStore {
	return &PatternStore{pool: pool}
}

// GetAggregatedPatterns returns patterns aggregated from transactions
// Groups by pattern_type + entity_id + pattern_name to distinguish different subscriptions
func (s *PatternStore) GetAggregatedPatterns(ctx context.Context, ledgerID uuid.UUID) ([]*AggregatedPattern, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT 
			t.pattern_type,
			COALESCE(NULLIF(t.pattern_metadata->>'pattern_name', ''), '') as pattern_name,
			t.entity_id,
			COALESCE(e.name, t.counterparty_name, t.description, 'Unknown') as entity_name,
			COALESCE(e.logo_url, '') as entity_logo,
			COALESCE(NULLIF(t.pattern_metadata->>'frequency', ''), 'monthly') as frequency,
			AVG(ABS(en.amount_cents))::BIGINT as avg_amount_cents,
			SUM(ABS(en.amount_cents))::BIGINT as total_amount_cents,
			COUNT(DISTINCT t.id) as occurrence_count,
			AVG(COALESCE(NULLIF(t.pattern_metadata->>'confidence', '')::INT, 0))::INT as avg_confidence,
			MIN(t.date) as first_occurrence,
			MAX(t.date) as last_occurrence,
			BOOL_OR(COALESCE((t.pattern_metadata->>'is_subscription')::BOOLEAN, false)) as is_subscription
		FROM transactions t
		JOIN entries en ON t.id = en.transaction_id
		JOIN accounts a ON en.account_id = a.id
		LEFT JOIN entities e ON t.entity_id = e.id
		WHERE t.ledger_id = $1
			AND t.pattern_type IS NOT NULL 
			AND t.pattern_type NOT IN ('none', '', 'dismissed')
			AND t.pattern_detection_status = 'done'
			AND a.type IN ('asset', 'liability')
			-- Salary patterns: inflows (positive to assets)
			-- Other patterns: outflows (negative from assets, positive to liabilities)
			AND (
				(t.pattern_type = 'salary' AND a.type = 'asset' AND en.amount_cents > 0)
				OR (t.pattern_type != 'salary' AND ((a.type = 'asset' AND en.amount_cents < 0) OR (a.type = 'liability' AND en.amount_cents > 0)))
			)
		GROUP BY 
			t.pattern_type,
			COALESCE(NULLIF(t.pattern_metadata->>'pattern_name', ''), ''),
			t.entity_id,
			COALESCE(e.name, t.counterparty_name, t.description, 'Unknown'),
			e.logo_url,
			COALESCE(NULLIF(t.pattern_metadata->>'frequency', ''), 'monthly')
		ORDER BY 
			COALESCE(e.name, t.counterparty_name, t.description, 'Unknown'),
			AVG(ABS(en.amount_cents)) DESC
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patterns []*AggregatedPattern
	for rows.Next() {
		p := &AggregatedPattern{}
		var entityID *uuid.UUID
		err := rows.Scan(
			&p.PatternType,
			&p.PatternName,
			&entityID,
			&p.EntityName,
			&p.EntityLogo,
			&p.Frequency,
			&p.AvgAmountCents,
			&p.TotalAmountCents,
			&p.OccurrenceCount,
			&p.AvgConfidence,
			&p.FirstOccurrence,
			&p.LastOccurrence,
			&p.IsSubscription,
		)
		if err != nil {
			return nil, err
		}
		p.EntityID = entityID

		// Calculate next expected date based on frequency and last occurrence
		p.NextExpected = calculateNextExpected(p.LastOccurrence, p.Frequency)

		patterns = append(patterns, p)
	}

	return patterns, rows.Err()
}

// GetPatternsByType returns patterns of a specific type
func (s *PatternStore) GetPatternsByType(ctx context.Context, ledgerID uuid.UUID, patternType string) ([]*AggregatedPattern, error) {
	all, err := s.GetAggregatedPatterns(ctx, ledgerID)
	if err != nil {
		return nil, err
	}

	var filtered []*AggregatedPattern
	for _, p := range all {
		if p.PatternType == patternType {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

// GetRecurringBills returns recurring bill patterns (subscriptions)
func (s *PatternStore) GetRecurringBills(ctx context.Context, ledgerID uuid.UUID) ([]*AggregatedPattern, error) {
	return s.GetPatternsByType(ctx, ledgerID, "recurring_bill")
}

// GetSalaryPatterns returns salary/income patterns
func (s *PatternStore) GetSalaryPatterns(ctx context.Context, ledgerID uuid.UUID) ([]*AggregatedPattern, error) {
	return s.GetPatternsByType(ctx, ledgerID, "salary")
}


// calculateNextExpected calculates the next expected date based on frequency
// Note: Transaction dates from Teller are stored as UTC midnight, so this returns UTC as well.
// All date comparisons should use UTC to avoid timezone issues.
func calculateNextExpected(lastOccurrence time.Time, frequency string) *time.Time {
	var next time.Time
	switch frequency {
	case "weekly":
		next = lastOccurrence.AddDate(0, 0, 7)
	case "biweekly":
		next = lastOccurrence.AddDate(0, 0, 14)
	case "monthly":
		next = lastOccurrence.AddDate(0, 1, 0)
	case "quarterly":
		next = lastOccurrence.AddDate(0, 3, 0)
	case "annual":
		next = lastOccurrence.AddDate(1, 0, 0)
	default:
		next = lastOccurrence.AddDate(0, 1, 0) // Default to monthly
	}
	return &next
}

// PatternDetail contains full details about a pattern including transactions
type PatternDetail struct {
	Pattern      *AggregatedPattern
	Reasoning    string // Representative reasoning from the LLM
	Transactions []*PatternTransaction
}

// PatternTransaction represents a transaction that is part of a pattern
type PatternTransaction struct {
	ID          uuid.UUID `json:"id"`
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
	AmountCents int64     `json:"amount_cents"`
	AccountName string    `json:"account_name"`
	Reasoning   string    `json:"reasoning,omitempty"`    // Individual reasoning from pattern_metadata
	PatternName string    `json:"pattern_name,omitempty"` // Pattern name from metadata
	Confidence  int       `json:"confidence,omitempty"`
}

// GetPatternDetail returns detailed information about a specific pattern
// including all transactions that make up the pattern
func (s *PatternStore) GetPatternDetail(ctx context.Context, ledgerID uuid.UUID, entityID *uuid.UUID, patternType, patternName string) (*PatternDetail, error) {
	// First get the aggregated pattern info
	var pattern AggregatedPattern
	var entityIDValue interface{}
	if entityID != nil {
		entityIDValue = *entityID
	}

	err := s.pool.QueryRow(ctx, `
		SELECT 
			t.pattern_type,
			COALESCE(NULLIF(t.pattern_metadata->>'pattern_name', ''), '') as pattern_name,
			t.entity_id,
			COALESCE(e.name, t.counterparty_name, t.description, 'Unknown') as entity_name,
			COALESCE(e.logo_url, '') as entity_logo,
			COALESCE(NULLIF(t.pattern_metadata->>'frequency', ''), 'monthly') as frequency,
			AVG(ABS(en.amount_cents))::BIGINT as avg_amount_cents,
			SUM(ABS(en.amount_cents))::BIGINT as total_amount_cents,
			COUNT(DISTINCT t.id) as occurrence_count,
			AVG(COALESCE(NULLIF(t.pattern_metadata->>'confidence', '')::INT, 0))::INT as avg_confidence,
			MIN(t.date) as first_occurrence,
			MAX(t.date) as last_occurrence,
			BOOL_OR(COALESCE((t.pattern_metadata->>'is_subscription')::BOOLEAN, false)) as is_subscription
		FROM transactions t
		JOIN entries en ON t.id = en.transaction_id
		JOIN accounts a ON en.account_id = a.id
		LEFT JOIN entities e ON t.entity_id = e.id
		WHERE t.ledger_id = $1
			AND t.pattern_type = $2
			AND (($3::UUID IS NULL AND t.entity_id IS NULL) OR t.entity_id = $3)
			AND COALESCE(NULLIF(t.pattern_metadata->>'pattern_name', ''), '') = $4
			AND t.pattern_detection_status = 'done'
			AND a.type IN ('asset', 'liability')
			-- Salary patterns: inflows (positive to assets)
			-- Other patterns: outflows (negative from assets, positive to liabilities)
			AND (
				(t.pattern_type = 'salary' AND a.type = 'asset' AND en.amount_cents > 0)
				OR (t.pattern_type != 'salary' AND ((a.type = 'asset' AND en.amount_cents < 0) OR (a.type = 'liability' AND en.amount_cents > 0)))
			)
		GROUP BY 
			t.pattern_type,
			COALESCE(NULLIF(t.pattern_metadata->>'pattern_name', ''), ''),
			t.entity_id,
			COALESCE(e.name, t.counterparty_name, t.description, 'Unknown'),
			e.logo_url,
			COALESCE(NULLIF(t.pattern_metadata->>'frequency', ''), 'monthly')
	`, ledgerID, patternType, entityIDValue, patternName).Scan(
		&pattern.PatternType,
		&pattern.PatternName,
		&pattern.EntityID,
		&pattern.EntityName,
		&pattern.EntityLogo,
		&pattern.Frequency,
		&pattern.AvgAmountCents,
		&pattern.TotalAmountCents,
		&pattern.OccurrenceCount,
		&pattern.AvgConfidence,
		&pattern.FirstOccurrence,
		&pattern.LastOccurrence,
		&pattern.IsSubscription,
	)
	if err != nil {
		return nil, err
	}

	pattern.NextExpected = calculateNextExpected(pattern.LastOccurrence, pattern.Frequency)

	// Get all transactions for this pattern
	rows, err := s.pool.Query(ctx, `
		SELECT 
			t.id,
			t.date,
			t.description,
			ABS(en.amount_cents) as amount_cents,
			a.name as account_name,
			COALESCE(t.pattern_metadata->>'reasoning', '') as reasoning,
			COALESCE(t.pattern_metadata->>'pattern_name', '') as pattern_name,
			COALESCE(NULLIF(t.pattern_metadata->>'confidence', '')::INT, 0) as confidence
		FROM transactions t
		JOIN entries en ON t.id = en.transaction_id
		JOIN accounts a ON en.account_id = a.id
		WHERE t.ledger_id = $1
			AND t.pattern_type = $2
			AND (($3::UUID IS NULL AND t.entity_id IS NULL) OR t.entity_id = $3)
			AND COALESCE(NULLIF(t.pattern_metadata->>'pattern_name', ''), '') = $4
			AND t.pattern_detection_status = 'done'
			AND a.type IN ('asset', 'liability')
			-- Salary patterns: inflows (positive to assets)
			-- Other patterns: outflows (negative from assets, positive to liabilities)
			AND (
				(t.pattern_type = 'salary' AND a.type = 'asset' AND en.amount_cents > 0)
				OR (t.pattern_type != 'salary' AND ((a.type = 'asset' AND en.amount_cents < 0) OR (a.type = 'liability' AND en.amount_cents > 0)))
			)
		ORDER BY t.date DESC
	`, ledgerID, patternType, entityIDValue, patternName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*PatternTransaction
	var sampleReasoning string

	for rows.Next() {
		var txn PatternTransaction
		err := rows.Scan(
			&txn.ID,
			&txn.Date,
			&txn.Description,
			&txn.AmountCents,
			&txn.AccountName,
			&txn.Reasoning,
			&txn.PatternName,
			&txn.Confidence,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, &txn)

		// Use the first non-empty reasoning as the representative one
		if sampleReasoning == "" && txn.Reasoning != "" {
			sampleReasoning = txn.Reasoning
		}
	}

	return &PatternDetail{
		Pattern:      &pattern,
		Reasoning:    sampleReasoning,
		Transactions: transactions,
	}, rows.Err()
}

