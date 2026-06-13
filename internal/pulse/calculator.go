package pulse

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LeftToSpendResult contains the calculated left to spend metrics
type LeftToSpendResult struct {
	AvailableBalance int64 `json:"available_balance"` // Sum of spendable account balances
	UpcomingBills    int64 `json:"upcoming_bills"`    // Sum of upcoming bills in next 60 days
	LeftToSpend      int64 `json:"left_to_spend"`     // available - upcoming
}

// UpcomingBill represents a single upcoming bill
type UpcomingBill struct {
	EntityID       *uuid.UUID `json:"entity_id,omitempty"`
	EntityName     string     `json:"entity_name"`
	EntityLogo     string     `json:"entity_logo,omitempty"`
	PatternName    string     `json:"pattern_name,omitempty"` // Specific subscription (e.g., "iCloud Storage")
	ExpectedDate   time.Time  `json:"expected_date"`
	ExpectedAmount int64      `json:"expected_amount"`
	IsCovered      bool       `json:"is_covered"` // available_at_date >= amount
	Frequency      string     `json:"frequency"`  // monthly, annual, etc.
	Confidence     int        `json:"confidence"` // detection confidence 0-100
}

// Calculator handles left to spend calculations
type Calculator struct {
	pool *pgxpool.Pool
}

// NewCalculator creates a new calculator
func NewCalculator(pool *pgxpool.Pool) *Calculator {
	return &Calculator{pool: pool}
}

// CalculateLeftToSpend calculates the left to spend metric for a ledger
// Uses the new pattern system (transaction metadata) instead of detected_recurring
func (c *Calculator) CalculateLeftToSpend(ctx context.Context, ledgerID uuid.UUID) (*LeftToSpendResult, error) {
	var result LeftToSpendResult

	// Get spendable account total
	err := c.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(e.amount_cents), 0)
		FROM accounts a
		JOIN entries e ON a.id = e.account_id
		WHERE a.ledger_id = $1
			AND a.is_active = true
			AND a.type = 'asset'
			AND (
				a.include_in_left_to_spend = true
				OR (
					a.include_in_left_to_spend IS NULL 
					AND (a.account_subtype = 'checking' OR a.account_subtype IS NULL)
					AND COALESCE(a.account_subtype, '') NOT IN ('investment', 'savings')
					AND LOWER(a.name) NOT LIKE '%investment%'
				)
			)
	`, ledgerID).Scan(&result.AvailableBalance)
	if err != nil {
		return nil, err
	}

	// Get upcoming bills from new pattern system
	upcomingBills, err := c.GetUpcomingBills(ctx, ledgerID, result.AvailableBalance)
	if err != nil {
		return nil, err
	}

	// Sum upcoming bills
	for _, bill := range upcomingBills {
		result.UpcomingBills += bill.ExpectedAmount
	}

	result.LeftToSpend = result.AvailableBalance - result.UpcomingBills
	return &result, nil
}

// GetUpcomingBills returns the list of upcoming bills within the next 60 days
// Uses the new pattern system (aggregated from transaction metadata)
func (c *Calculator) GetUpcomingBills(ctx context.Context, ledgerID uuid.UUID, availableBalance int64) ([]UpcomingBill, error) {
	patternStore := models.NewPatternStore(c.pool)

	// Get recurring bill patterns
	patterns, err := patternStore.GetRecurringBills(ctx, ledgerID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	sixtyDaysLater := today.AddDate(0, 0, 60)

	var bills []UpcomingBill
	runningBalance := availableBalance

	for _, pattern := range patterns {
		// Skip patterns without a next expected date
		if pattern.NextExpected == nil {
			continue
		}

		// Treat as calendar date (year, month, day only - no timezone conversion)
		nextDate := *pattern.NextExpected
		nextDateOnly := time.Date(nextDate.Year(), nextDate.Month(), nextDate.Day(), 0, 0, 0, 0, now.Location())

		// Only include bills due within the next 60 days (and not in the past)
		if nextDateOnly.Before(today) || nextDateOnly.After(sixtyDaysLater) {
			continue
		}

		// Skip low confidence patterns
		if pattern.AvgConfidence < 50 {
			continue
		}

		bill := UpcomingBill{
			EntityID:       pattern.EntityID,
			EntityName:     pattern.EntityName,
			EntityLogo:     pattern.EntityLogo,
			PatternName:    pattern.PatternName,
			ExpectedDate:   nextDateOnly,
			ExpectedAmount: pattern.AvgAmountCents,
			Frequency:      pattern.Frequency,
			Confidence:     pattern.AvgConfidence,
		}

		// Check if this bill is covered by available balance
		bill.IsCovered = runningBalance >= bill.ExpectedAmount
		runningBalance -= bill.ExpectedAmount

		bills = append(bills, bill)
	}

	slices.SortFunc(bills, func(a, b UpcomingBill) int {
		return cmp.Compare(a.ExpectedDate.Unix(), b.ExpectedDate.Unix())
	})

	// Limit to 10 upcoming bills
	if len(bills) > 10 {
		bills = bills[:10]
	}

	return bills, nil
}

