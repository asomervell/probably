package pulse

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SpendingPace contains spending pace comparison data
type SpendingPace struct {
	// Current month data
	CurrentMonthSpent     int64   `json:"current_month_spent"`      // Total expenses so far this month
	DayOfMonth            int     `json:"day_of_month"`             // e.g., 15
	DaysInMonth           int     `json:"days_in_month"`            // e.g., 31
	PercentOfMonthElapsed float64 `json:"percent_of_month_elapsed"` // e.g., 0.48

	// Month names for display (calculated at query time, not render time)
	CurrentMonthName string `json:"current_month_name"` // e.g., "December"
	LastMonthName    string `json:"last_month_name"`    // e.g., "November"

	// Last month comparison
	LastMonthSamePoint int64 `json:"last_month_same_point"` // Expenses through same day of last month
	LastMonthTotal     int64 `json:"last_month_total"`      // Full last month expenses

	// Derived metrics
	PacePercentage    float64 `json:"pace_percentage"`     // e.g., 1.21 = 21% faster
	ProjectedMonthEnd int64   `json:"projected_month_end"` // Extrapolated total

	// Status for UI
	Status         string `json:"status"` // "on_track", "faster", "slower"
	StatusMessage  string `json:"status_message"`
	PercentChange  int    `json:"percent_change"` // e.g., 21 (for "21% faster")
	HasEnoughData  bool   `json:"has_enough_data"`
	HasLastMonth   bool   `json:"has_last_month"`

	// Daily cumulative data for line chart
	CurrentMonthDaily []DailySpending `json:"current_month_daily"`
	LastMonthDaily    []DailySpending `json:"last_month_daily"`

	// Debug: top transactions being counted (temporary)
	DebugTopTransactions []DebugTransaction `json:"debug_top_transactions,omitempty"`
}

// DebugTransaction shows what's being counted as spending
type DebugTransaction struct {
	Date        string `json:"date"`
	Description string `json:"description"`
	AmountCents int64  `json:"amount_cents"`
	AccountName string `json:"account_name"`
	AccountType string `json:"account_type"`
	IsTransfer  bool   `json:"is_transfer"`
}

// DailySpending represents cumulative spending up to a day
type DailySpending struct {
	Day        int   `json:"day"`        // Day of month (1-31)
	Cumulative int64 `json:"cumulative"` // Cumulative spending in cents
}

// CalculateSpendingPace computes the spending pace comparison
func (c *Calculator) CalculateSpendingPace(ctx context.Context, ledgerID uuid.UUID) (*SpendingPace, error) {
	now := time.Now()
	dayOfMonth := now.Day()
	totalDaysInMonth := daysInMonth(now)

	// Current month boundaries
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	currentMonthEnd := now

	// Last month boundaries
	lastMonthStart := currentMonthStart.AddDate(0, -1, 0)

	result := &SpendingPace{
		DayOfMonth:            dayOfMonth,
		DaysInMonth:           totalDaysInMonth,
		PercentOfMonthElapsed: float64(dayOfMonth) / float64(totalDaysInMonth),
		HasEnoughData:         dayOfMonth >= 3, // Need at least 3 days of data
		CurrentMonthName:      currentMonthStart.Format("January"),
		LastMonthName:         lastMonthStart.Format("January"),
	}
	lastMonthEnd := currentMonthStart.AddDate(0, 0, -1) // Last day of previous month

	// Get current month spending
	currentMonthSpent, err := c.getExpenses(ctx, ledgerID, currentMonthStart, currentMonthEnd)
	if err != nil {
		return nil, err
	}
	result.CurrentMonthSpent = currentMonthSpent

	// Get last month same point (same day number)
	// Handle edge case: if today is Jan 31 but Feb only has 28 days
	lastMonthDayOfMonth := dayOfMonth
	lastMonthDaysTotal := daysInMonth(lastMonthStart)
	if dayOfMonth > lastMonthDaysTotal {
		lastMonthDayOfMonth = lastMonthDaysTotal
	}
	lastMonthSameDay := time.Date(lastMonthStart.Year(), lastMonthStart.Month(), lastMonthDayOfMonth, 23, 59, 59, 0, now.Location())

	lastMonthSamePoint, err := c.getExpenses(ctx, ledgerID, lastMonthStart, lastMonthSameDay)
	if err != nil {
		return nil, err
	}
	result.LastMonthSamePoint = lastMonthSamePoint

	// Get last month total
	lastMonthTotal, err := c.getExpenses(ctx, ledgerID, lastMonthStart, lastMonthEnd)
	if err != nil {
		return nil, err
	}
	result.LastMonthTotal = lastMonthTotal
	result.HasLastMonth = lastMonthTotal > 0

	// Calculate pace
	if lastMonthSamePoint > 0 {
		result.PacePercentage = float64(currentMonthSpent) / float64(lastMonthSamePoint)
	} else if currentMonthSpent > 0 {
		result.PacePercentage = 2.0 // Arbitrary "much faster" if no last month data
	} else {
		result.PacePercentage = 1.0
	}

	// Project month end (simple linear extrapolation)
	if dayOfMonth > 0 {
		result.ProjectedMonthEnd = int64(float64(currentMonthSpent) / float64(dayOfMonth) * float64(totalDaysInMonth))
	}

	// Determine status
	percentChange := int((result.PacePercentage - 1.0) * 100)
	result.PercentChange = abs(percentChange)

	switch {
	case !result.HasEnoughData:
		result.Status = "insufficient_data"
		result.StatusMessage = "" // Only shows first few days of month
	case !result.HasLastMonth:
		result.Status = "no_last_month"
		result.StatusMessage = "" // UI handles this case with actual chart
	case result.PacePercentage > 1.10:
		result.Status = "faster"
		result.StatusMessage = "Spending faster than last month"
	case result.PacePercentage < 0.90:
		result.Status = "slower"
		result.StatusMessage = "Spending slower than last month"
	default:
		result.Status = "on_track"
		result.StatusMessage = "On pace with last month"
	}

	// Get daily cumulative data for the line chart
	currentDaily, err := c.getDailyCumulativeSpending(ctx, ledgerID, currentMonthStart, dayOfMonth)
	if err != nil {
		return nil, err
	}
	result.CurrentMonthDaily = currentDaily

	lastDaily, err := c.getDailyCumulativeSpending(ctx, ledgerID, lastMonthStart, lastMonthDaysTotal)
	if err != nil {
		return nil, err
	}
	result.LastMonthDaily = lastDaily

	// Get top transactions for display
	debugTxns, err := c.getDebugTransactions(ctx, ledgerID, currentMonthStart, currentMonthEnd, 5)
	if err != nil {
		// Don't fail on debug error, just skip
		debugTxns = nil
	}
	result.DebugTopTransactions = debugTxns

	return result, nil
}

// getExpenses returns total spending (credits to expense accounts) for a date range
// This measures actual spending on goods/services, not equity movements like
// credit card payments, investment contributions, or debt repayment.
func (c *Calculator) getExpenses(ctx context.Context, ledgerID uuid.UUID, start, end time.Time) (int64, error) {
	var total int64

	// Spending = negative amounts (credits) on expense accounts, converted to positive
	// Expense entries are stored as negative values in double-entry accounting
	err := c.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(ABS(e.amount_cents)), 0)
		FROM entries e
		JOIN transactions t ON e.transaction_id = t.id
		JOIN accounts a ON e.account_id = a.id
		WHERE t.ledger_id = $1
			AND t.date >= $2 AND t.date <= $3
			AND a.type = 'expense'
			AND e.amount_cents < 0
			AND COALESCE(t.is_transfer, false) = false
	`, ledgerID, start, end).Scan(&total)

	return total, err
}

// getDebugTransactions returns the top transactions being counted as spending (for debugging)
func (c *Calculator) getDebugTransactions(ctx context.Context, ledgerID uuid.UUID, start, end time.Time, limit int) ([]DebugTransaction, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT 
			t.date,
			t.description,
			ABS(e.amount_cents) as amount_cents,
			a.name as account_name,
			a.type as account_type,
			COALESCE(t.is_transfer, false) as is_transfer
		FROM entries e
		JOIN transactions t ON e.transaction_id = t.id
		JOIN accounts a ON e.account_id = a.id
		WHERE t.ledger_id = $1
			AND t.date >= $2 AND t.date <= $3
			AND a.type = 'expense'
			AND e.amount_cents < 0
			AND COALESCE(t.is_transfer, false) = false
		ORDER BY ABS(e.amount_cents) DESC
		LIMIT $4
	`, ledgerID, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DebugTransaction
	for rows.Next() {
		var dt DebugTransaction
		var date time.Time
		if err := rows.Scan(&date, &dt.Description, &dt.AmountCents, &dt.AccountName, &dt.AccountType, &dt.IsTransfer); err != nil {
			return nil, err
		}
		dt.Date = date.Format("Jan 2")
		results = append(results, dt)
	}

	return results, rows.Err()
}

// getDailyCumulativeSpending returns cumulative spending for each day of the month
func (c *Calculator) getDailyCumulativeSpending(ctx context.Context, ledgerID uuid.UUID, monthStart time.Time, maxDays int) ([]DailySpending, error) {
	// Spending = credits to expense accounts (stored as negative, convert to positive)
	rows, err := c.pool.Query(ctx, `
		WITH daily_expenses AS (
			SELECT 
				EXTRACT(DAY FROM t.date)::int as day_num,
				SUM(ABS(e.amount_cents)) as daily_total
			FROM entries e
			JOIN transactions t ON e.transaction_id = t.id
			JOIN accounts a ON e.account_id = a.id
			WHERE t.ledger_id = $1
				AND t.date >= $2 
				AND t.date < $2 + interval '1 month'
				AND a.type = 'expense'
				AND e.amount_cents < 0
				AND COALESCE(t.is_transfer, false) = false
			GROUP BY EXTRACT(DAY FROM t.date)
		)
		SELECT day_num, daily_total
		FROM daily_expenses
		ORDER BY day_num
	`, ledgerID, monthStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Build a map of day -> daily spending
	dailyMap := make(map[int]int64)
	for rows.Next() {
		var day int
		var amount int64
		if err := rows.Scan(&day, &amount); err != nil {
			return nil, err
		}
		dailyMap[day] = amount
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build cumulative array
	result := make([]DailySpending, maxDays)
	var cumulative int64 = 0
	for day := 1; day <= maxDays; day++ {
		cumulative += dailyMap[day]
		result[day-1] = DailySpending{
			Day:        day,
			Cumulative: cumulative,
		}
	}

	return result, nil
}

// daysInMonth returns the number of days in the month containing t
func daysInMonth(t time.Time) int {
	// Go to the first day of next month, then back one day
	firstOfNextMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
	lastDay := firstOfNextMonth.AddDate(0, 0, -1)
	return lastDay.Day()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
