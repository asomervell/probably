package recurring

import (
	"fmt"
	"strings"

	"github.com/asomervell/probably/internal/models"
)

func formatAmount(txn *models.Transaction) string {
	// Calculate amount from entries if available
	if len(txn.Entries) > 0 {
		var amount int64
		for _, e := range txn.Entries {
			if e.AccountType == models.AccountTypeAsset || e.AccountType == models.AccountTypeLiability {
				amount = e.AmountCents
				if e.AccountType == models.AccountTypeLiability {
					amount = -amount
				}
				break
			}
		}
		if amount < 0 {
			return fmt.Sprintf("%.2f", float64(-amount)/100.0)
		}
		return fmt.Sprintf("%.2f", float64(amount)/100.0)
	}
	return "0.00"
}

// EntityPatternContext contains all transactions for a single entity
type EntityPatternContext struct {
	Entity       *models.Entity
	EntityName   string // Fallback if no entity
	Transactions []*models.Transaction
	LedgerID     string

	// Business context from Teller (bank data) - strong signal for business type
	TellerCategory string // e.g., "dining", "groceries", "subscription"
	EntitySubtype  string // e.g., "cafe", "supermarket", "software"

	// Accumulated pattern knowledge (to help LLM make better decisions)
	ExistingPatternHints []models.EntityPatternHint // Patterns already detected for this entity
	SimilarPatterns      []PatternExample           // Examples of similar patterns from other entities
}

// PatternExample represents a known pattern that can provide context
type PatternExample struct {
	EntityName    string `json:"entity_name"`
	PatternType   string `json:"pattern_type"`
	Frequency     string `json:"frequency"`
	AvgAmountCents int64  `json:"avg_amount_cents"`
	Confidence    int    `json:"confidence"`
	OccurrenceCount int  `json:"occurrence_count"`
}

// EntityPatternResult represents patterns detected for an entity
// An entity may have MULTIPLE distinct patterns (e.g., Apple has iCloud, Apple One, App Store)
type EntityPatternResult struct {
	Patterns []DetectedPattern `json:"patterns"`
}

// DetectedPattern represents a single recurring pattern within an entity
type DetectedPattern struct {
	Name           string   `json:"name"`            // e.g., "iCloud Storage", "Monthly Subscription"
	PatternType    string   `json:"pattern_type"`    // recurring_bill, salary, etc.
	AmountCents    int64    `json:"amount_cents"`    // Typical amount in cents
	Frequency      string   `json:"frequency"`       // weekly, biweekly, monthly, quarterly, annual
	Confidence     int      `json:"confidence"`      // 0-100
	TransactionIDs []string `json:"transaction_ids"` // UUIDs of transactions matching this pattern
	Reasoning      string   `json:"reasoning"`       // Why this pattern was detected
}

// BuildEntityPatternPrompt builds a prompt that shows ALL transactions for one entity
// This allows the LLM to see the full temporal context and detect patterns
// Now enhanced with accumulated pattern knowledge to make better predictions!
func BuildEntityPatternPrompt(ctx *EntityPatternContext) string {
	var sb strings.Builder

	entityName := ctx.EntityName
	entitySubtype := ctx.EntitySubtype
	if ctx.Entity != nil {
		entityName = ctx.Entity.Name
		if ctx.Entity.Subtype != "" {
			entitySubtype = ctx.Entity.Subtype
		}
	}

	sb.WriteString(fmt.Sprintf(`Analyze ALL transactions from "%s" and identify recurring patterns.

Entity: %s
Total Transactions: %d
`, entityName, entityName, len(ctx.Transactions)))

	// Add business type context - this is critical for proper pattern detection
	if entitySubtype != "" {
		sb.WriteString(fmt.Sprintf("Business Type: %s\n", entitySubtype))
	}
	if ctx.TellerCategory != "" {
		sb.WriteString(fmt.Sprintf("Bank Category: %s\n", ctx.TellerCategory))
	}

	// Add important guidance about business types
	if models.IsOneTimeBusinessType(entitySubtype) {
		sb.WriteString(fmt.Sprintf(`
⚠️ IMPORTANT: This is a %s - typically NOT a subscription business.
Cafes, restaurants, supermarkets, and retailers usually have irregular purchases, NOT recurring patterns.
Be VERY conservative - only detect patterns with clear evidence of intentional recurring charges.
Random coffee purchases at a cafe are NOT a subscription pattern!
`, entitySubtype))
	}

	sb.WriteString("\n")

	// Show existing pattern hints for this entity (accumulated knowledge!)
	if len(ctx.ExistingPatternHints) > 0 {
		sb.WriteString("📊 EXISTING PATTERN KNOWLEDGE for this entity:\n")
		sb.WriteString("(We've already detected these patterns - use this to boost confidence!)\n")
		for _, hint := range ctx.ExistingPatternHints {
			sb.WriteString(fmt.Sprintf("  • %s (%s): %d%% confidence, %d occurrences\n",
				hint.PatternType, hint.Frequency, hint.Confidence, hint.OccurrenceCount))
		}
		sb.WriteString("\n")
	}

	// Show similar patterns from other entities (to help recognize new patterns)
	if len(ctx.SimilarPatterns) > 0 {
		sb.WriteString("🔍 SIMILAR PATTERNS from other entities (for reference):\n")
		for _, pattern := range ctx.SimilarPatterns {
			amountStr := fmt.Sprintf("$%.2f", float64(pattern.AvgAmountCents)/100.0)
			sb.WriteString(fmt.Sprintf("  • %s: %s %s (%s) - %d%% confidence\n",
				pattern.EntityName, pattern.PatternType, pattern.Frequency, amountStr, pattern.Confidence))
		}
		sb.WriteString("\n")
	}

	// Show transaction history chronologically
	sb.WriteString("Transaction History (chronological):\n")
	for _, txn := range ctx.Transactions {
		amount := formatAmount(txn)
		sb.WriteString(fmt.Sprintf("- %s: $%s [ID: %s]\n", 
			txn.Date.Format("Jan 2, 2006"), amount, txn.ID.String()))
	}

	sb.WriteString(`

IMPORTANT: This merchant may have MULTIPLE distinct recurring patterns.
For example, Apple might have separate subscriptions for iCloud ($0.99/mo), Apple One ($19.95/mo), and App Store purchases.

Look for:
1. Amounts that repeat at regular intervals (same amount, similar dates each month)
2. Multiple distinct subscription tiers from the same merchant
3. Consistent timing patterns (e.g., always around the 15th of the month)

`)

	// Add guidance based on existing knowledge
	if len(ctx.ExistingPatternHints) > 0 {
		sb.WriteString(`⚡ Since we already have pattern hints for this entity:
- If the transactions CONFIRM an existing pattern, INCREASE confidence
- If transactions match the existing frequency, this is likely the same pattern
- Look for NEW patterns that might not have been detected before

`)
	}

	sb.WriteString(`Pattern Types:
- recurring_bill: Subscriptions, utilities, insurance, recurring expenses
- salary: Regular income (consistent amount, regular interval)
- account_transfer: Transfers between user's own accounts
- investment_contribution: Contributions to investment accounts  
- household_transfer: Transfers to/from household members

Return a JSON object with ALL detected patterns:
{
  "patterns": [
    {
      "name": "descriptive name like 'iCloud Storage' or 'Monthly Premium'",
      "pattern_type": "recurring_bill",
      "amount_cents": 999,
      "frequency": "monthly",
      "confidence": 95,
      "transaction_ids": ["uuid1", "uuid2", "uuid3"],
      "reasoning": "Same $9.99 charge on the 7th of each month for 6 months"
    },
    {
      "name": "another pattern if applicable",
      ...
    }
  ]
}

Rules:
- Only include patterns with 2+ occurrences and confidence >= 70
- Group transactions by similar amounts (within 10% variance)
- If no clear patterns exist, return {"patterns": []}
- Each transaction should only appear in ONE pattern's transaction_ids
`)

	return sb.String()
}
