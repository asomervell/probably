package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SuggestedQuestion represents a suggested question for the user
type SuggestedQuestion struct {
	Text        string `json:"text"`
	Category    string `json:"category"` // e.g., "spending", "categories", "merchants", "comparison"
}

// SuggestionsGenerator generates contextual question suggestions
type SuggestionsGenerator struct {
	pool *pgxpool.Pool
}

// NewSuggestionsGenerator creates a new suggestions generator
func NewSuggestionsGenerator(pool *pgxpool.Pool) *SuggestionsGenerator {
	return &SuggestionsGenerator{
		pool: pool,
	}
}

// GenerateSuggestions generates contextual question suggestions for a user
func (sg *SuggestionsGenerator) GenerateSuggestions(ctx context.Context, ledgerID uuid.UUID, limit int) ([]SuggestedQuestion, error) {
	if limit <= 0 {
		limit = 6 // Default: 6 suggestions
	}

	suggestions := make([]SuggestedQuestion, 0)

	// Get current month and last month for time-based suggestions
	now := time.Now()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthStart := currentMonthStart.AddDate(0, -1, 0)
	lastMonthEnd := currentMonthStart.AddDate(0, 0, -1)

	// 1. Spending analysis suggestions
	spendingSuggestions := sg.generateSpendingSuggestions()
	suggestions = append(suggestions, spendingSuggestions...)

	// 2. Category breakdown suggestions
	categorySuggestions, err := sg.generateCategorySuggestions(ctx, ledgerID, currentMonthStart, now)
	if err == nil && len(categorySuggestions) > 0 {
		suggestions = append(suggestions, categorySuggestions...)
	}

	// 3. Time comparison suggestions
	comparisonSuggestions := sg.generateComparisonSuggestions(currentMonthStart, lastMonthStart, lastMonthEnd)
	suggestions = append(suggestions, comparisonSuggestions...)

	// 4. Transaction search suggestions
	searchSuggestions := sg.generateSearchSuggestions()
	suggestions = append(suggestions, searchSuggestions...)

	// Limit to requested number
	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions, nil
}

// generateSpendingSuggestions returns a fixed set of spending-related questions.
func (sg *SuggestionsGenerator) generateSpendingSuggestions() []SuggestedQuestion {
	return []SuggestedQuestion{
		{
			Text:     "How much did I spend this month?",
			Category: "spending",
		},
		{
			Text:     "What's my total spending this year?",
			Category: "spending",
		},
		{
			Text:     "How much did I spend last week?",
			Category: "spending",
		},
	}
}

// generateCategorySuggestions generates category-related questions based on actual data
func (sg *SuggestionsGenerator) generateCategorySuggestions(ctx context.Context, ledgerID uuid.UUID, monthStart, now time.Time) ([]SuggestedQuestion, error) {
	// Get top spending categories for this month
	rows, err := sg.pool.Query(ctx, `
		SELECT tg.name, COUNT(*) as count
		FROM entries e
		JOIN transactions t ON e.transaction_id = t.id
		JOIN accounts a ON e.account_id = a.id
		LEFT JOIN transaction_tags tt ON t.id = tt.transaction_id
		LEFT JOIN tags tg ON tt.tag_id = tg.id
		WHERE t.ledger_id = $1
			AND t.date >= $2 AND t.date <= $3
			AND t.is_transfer = false
			AND a.type = 'expense'
			AND e.amount_cents < 0
			AND tg.name IS NOT NULL
		GROUP BY tg.name
		ORDER BY COUNT(*) DESC
		LIMIT 3
	`, ledgerID, monthStart, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	suggestions := []SuggestedQuestion{
		{
			Text:     "What are my top spending categories?",
			Category: "categories",
		},
	}

	var topCategory string
	if rows.Next() {
		var count int
		if err := rows.Scan(&topCategory, &count); err == nil && topCategory != "" {
			suggestions = append(suggestions, SuggestedQuestion{
				Text:     fmt.Sprintf("How much did I spend on %s this month?", topCategory),
				Category: "categories",
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return suggestions, nil
}

// generateComparisonSuggestions generates time comparison questions
func (sg *SuggestionsGenerator) generateComparisonSuggestions(currentMonth, lastMonthStart, lastMonthEnd time.Time) []SuggestedQuestion {
	currentMonthName := currentMonth.Format("January")
	lastMonthName := lastMonthStart.Format("January")
	
	return []SuggestedQuestion{
		{
			Text:     "Am I spending more than last month?",
			Category: "comparison",
		},
		{
			Text:     fmt.Sprintf("Compare my spending in %s vs %s", currentMonthName, lastMonthName),
			Category: "comparison",
		},
	}
}

// generateSearchSuggestions generates transaction search questions
func (sg *SuggestionsGenerator) generateSearchSuggestions() []SuggestedQuestion {
	return []SuggestedQuestion{
		{
			Text:     "Show me transactions over $100",
			Category: "search",
		},
		{
			Text:     "What are my largest expenses this month?",
			Category: "search",
		},
	}
}
