package chat

import (
	"fmt"
	"strings"
)

// BuildChatSystemPromptVoice builds a voice-optimized system prompt for SQL generation
// Voice responses should be more concise, conversational, and avoid markdown formatting
func BuildChatSystemPromptVoice() string {
	return `You are a financial assistant that helps users understand their spending and financial data through voice conversation.

You have access to the user's financial data via SQL queries against a PostgreSQL database.

Your job is to:
1. Understand the user's question in natural language
2. Generate a valid PostgreSQL SQL query that answers the question
3. Provide a concise, conversational explanation suitable for voice output

IMPORTANT RULES FOR VOICE RESPONSES:
- Keep responses SHORT and CONVERSATIONAL (2-3 sentences max)
- Avoid markdown formatting (no **bold**, no lists with bullets)
- Use natural speech patterns (e.g., "You spent about $500" not "You spent $500.00")
- For multiple items, summarize instead of listing (e.g., "You spent $200 across 5 transactions" not "Transaction 1: $50, Transaction 2: $30...")
- Avoid tables and complex data structures - summarize the key points
- Use pauses naturally (e.g., "That's about... $1,200 total")

SQL GENERATION RULES:
- You MUST generate valid PostgreSQL SQL
- You MUST include "ledger_id = $1" in the WHERE clause (this is required for security)
- You MUST use parameterized queries ($1 for ledger_id)
- Always use amount_cents (divide by 100.0 for dollars in SELECT)
- Exclude transfers (is_transfer = false) for spending queries unless specifically asked
- Use ABS() for expense amounts when needed
- Be concise and accurate in your SQL

Return your response in JSON format:
{
  "thought": "your reasoning about what the user wants",
  "sql": "SELECT ... WHERE ledger_id = $1 ...",
  "answer_template": "You spent about ${total} on ${category} this month."
}

The answer_template should be SHORT, CONVERSATIONAL, and use ${variable} placeholders that match column aliases in your SQL.`
}

// BuildSchemaDocumentation builds the schema documentation for the LLM
func BuildSchemaDocumentation() string {
	return `DATABASE SCHEMA:

The database uses double-entry bookkeeping. Each transaction has multiple entries that sum to zero.

CORE TABLES:

1. transactions (transaction header)
   - id (UUID)
   - ledger_id (UUID) - REQUIRED in WHERE clause
   - date (DATE)
   - description (TEXT)
   - display_title (TEXT) - cleaned business name
   - notes (TEXT)
   - is_transfer (BOOLEAN) - true for internal transfers
   - transfer_pair_id (UUID) - links matched transfers
   - created_at, updated_at

2. entries (double-entry lines)
   - id (UUID)
   - transaction_id (UUID) - references transactions.id
   - account_id (UUID) - references accounts.id
   - amount_cents (BIGINT) - positive = debit, negative = credit
   - currency (VARCHAR) - default 'USD'
   - created_at

   IMPORTANT: For expense queries, filter entries where:
   - amount_cents < 0 (credits/outflows)
   - account.type = 'expense' (expense accounts)

3. accounts
   - id (UUID)
   - ledger_id (UUID) - REQUIRED in WHERE clause
   - name (VARCHAR)
   - type (ENUM: 'asset', 'liability', 'income', 'expense', 'equity')
   - institution_name (VARCHAR)
   - is_active (BOOLEAN)
   - created_at, updated_at

4. tags (categories)
   - id (UUID)
   - ledger_id (UUID) - REQUIRED in WHERE clause
   - parent_id (UUID) - for category hierarchy
   - name (VARCHAR)
   - color (VARCHAR)
   - created_at, updated_at

5. transaction_tags (many-to-many)
   - transaction_id (UUID)
   - tag_id (UUID)
   - created_at

6. entities (businesses/people)
   - id (UUID)
   - name (VARCHAR)
   - type (VARCHAR) - 'business', 'person', 'trust', 'partnership', 'government'
   - subtype (VARCHAR) - e.g., 'retailer', 'financial_institution'
   - website (VARCHAR)
   - description (TEXT)
   - created_at, updated_at
   NOTE: Entities are shared across ledgers. Filter via transaction relationships.

7. entity_relationships
   - ledger_id (UUID) - REQUIRED in WHERE clause
   - entity_a_id (UUID)
   - entity_b_id (UUID)
   - relationship_type (VARCHAR) - 'spouse', 'partner', 'family', etc.
   - created_at

NOTE: Transactions have entity_id, counterparty_entity_id, and intermediary_entity_id columns
for linking to entities. Use these to filter by entity while maintaining ledger_id security.

COMMON QUERY PATTERNS:

1. Total spending this month:
   SELECT SUM(ABS(e.amount_cents)) / 100.0 as total_spent
   FROM entries e
   JOIN transactions t ON e.transaction_id = t.id
   JOIN accounts a ON e.account_id = a.id
   WHERE t.ledger_id = $1
     AND t.date >= date_trunc('month', CURRENT_DATE)
     AND t.is_transfer = false
     AND a.type = 'expense'
     AND e.amount_cents < 0;

2. Spending by category:
   SELECT tg.name as category, SUM(ABS(e.amount_cents)) / 100.0 as total
   FROM entries e
   JOIN transactions t ON e.transaction_id = t.id
   JOIN accounts a ON e.account_id = a.id
   LEFT JOIN transaction_tags tt ON t.id = tt.transaction_id
   LEFT JOIN tags tg ON tt.tag_id = tg.id
   WHERE t.ledger_id = $1
     AND t.is_transfer = false
     AND a.type = 'expense'
     AND e.amount_cents < 0
   GROUP BY tg.name
   ORDER BY total DESC;

3. Transactions over amount:
   SELECT t.date, t.display_title, ABS(e.amount_cents) / 100.0 as amount
   FROM transactions t
   JOIN entries e ON e.transaction_id = t.id
   JOIN accounts a ON e.account_id = a.id
   WHERE t.ledger_id = $1
     AND ABS(e.amount_cents) > 10000  -- $100.00
     AND a.type = 'expense'
   ORDER BY t.date DESC;

REMEMBER:
- Always include "WHERE ledger_id = $1" for security
- Use amount_cents / 100.0 to convert to dollars
- Filter by is_transfer = false for spending queries
- Use ABS() for expense amounts
- Join entries to get amounts (transactions don't have amounts directly)`
}

// BuildChatUserPrompt builds the user prompt with the question and schema
// If conversationHistory is provided, it will be included in the prompt
func BuildChatUserPrompt(question string, conversationHistory string) string {
	var sb strings.Builder
	
	if conversationHistory != "" {
		sb.WriteString(conversationHistory)
		sb.WriteString("\n\n")
	}
	
	sb.WriteString("USER QUESTION:\n")
	sb.WriteString(question)
	sb.WriteString("\n\n")
	sb.WriteString(BuildSchemaDocumentation())
	sb.WriteString("\n\n")
	sb.WriteString("Generate a PostgreSQL SQL query to answer this question. ")
	sb.WriteString("Remember to include 'ledger_id = $1' in the WHERE clause.")
	return sb.String()
}

// FormatAnswer formats the query results into a natural language answer
func FormatAnswer(template string, result *QueryResult) string {
	if result == nil || len(result.Rows) == 0 {
		return "No results found."
	}

	// Simple template replacement
	answer := template

	// Replace ${variable} with values from first row
	if len(result.Rows) > 0 && len(result.Columns) > 0 {
		firstRow := result.Rows[0]
		for i, col := range result.Columns {
			if i < len(firstRow) {
				placeholder := fmt.Sprintf("${%s}", strings.ToLower(col))
				value := formatValueForTemplate(firstRow[i], col)
				
				// Check if template has $${var} pattern (LLM added a $ before the placeholder)
				// If so, strip the $ from our formatted value to avoid $$
				dollarPlaceholder := "$" + placeholder
				if strings.Contains(answer, dollarPlaceholder) {
					// Remove leading $ from formatted currency values
					valueWithoutDollar := strings.TrimPrefix(value, "$")
					answer = strings.ReplaceAll(answer, dollarPlaceholder, "$"+valueWithoutDollar)
				}
				answer = strings.ReplaceAll(answer, placeholder, value)
			}
		}
	}

	// Replace common placeholders
	answer = strings.ReplaceAll(answer, "${count}", fmt.Sprintf("%d", result.Count))

	return answer
}

// formatValueForTemplate formats a value for use in answer templates
func formatValueForTemplate(val interface{}, columnName string) string {
	if val == nil {
		return "N/A"
	}

	// Format numeric values nicely
	switch v := val.(type) {
	case float64:
		// Check if column name suggests it's a currency amount
		colLower := strings.ToLower(columnName)
		if strings.Contains(colLower, "total") || strings.Contains(colLower, "amount") || 
		   strings.Contains(colLower, "spent") || strings.Contains(colLower, "spending") ||
		   strings.Contains(colLower, "cost") || strings.Contains(colLower, "price") {
			// Format as currency
			return formatCurrency(v)
		}
		// Format as number with commas
		return formatNumber(v)
	case int, int8, int16, int32, int64:
		// Convert to float64 for consistent formatting
		return formatValueForTemplate(float64(getInt64(v)), columnName)
	case uint, uint8, uint16, uint32, uint64:
		return formatValueForTemplate(float64(getUint64(v)), columnName)
	case float32:
		return formatValueForTemplate(float64(v), columnName)
	case string:
		// Try to parse as number if it looks numeric
		if num, err := parseNumericString(v); err == nil {
			// Recursively format as number
			return formatValueForTemplate(num, columnName)
		}
		return v
	default:
		// For unknown types, try to convert to string and parse as number
		str := fmt.Sprintf("%v", v)
		// If it looks like a struct representation, try to extract numeric value
		// Handle cases like "{250569370000000000 -12 false finite true}" (big.Float)
		if num, err := extractNumericFromStructString(str); err == nil {
			return formatValueForTemplate(num, columnName)
		}
		// Try parsing as regular number string
		if num, err := parseNumericString(str); err == nil {
			return formatValueForTemplate(num, columnName)
		}
		return str
	}
}

// formatCurrency formats a float64 as currency
func formatCurrency(amount float64) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}

	// Format with 2 decimal places and commas
	formatted := fmt.Sprintf("$%.2f", amount)
	
	// Add commas for thousands (simple approach)
	parts := strings.Split(formatted, ".")
	intPart := parts[0][1:] // Remove $
	intPart = addCommasToNumber(intPart)
	
	result := "$" + intPart
	if len(parts) > 1 {
		result += "." + parts[1]
	}
	
	if negative {
		return "-" + result
	}
	return result
}

// formatNumber formats a float64 as a number with commas
func formatNumber(num float64) string {
	// Format with appropriate decimal places
	if num == float64(int64(num)) {
		// Integer value
		return addCommasToNumber(fmt.Sprintf("%.0f", num))
	}
	// Decimal value - use 2 decimal places
	formatted := fmt.Sprintf("%.2f", num)
	parts := strings.Split(formatted, ".")
	return addCommasToNumber(parts[0]) + "." + parts[1]
}

// addCommasToNumber adds thousand separators to a number string
func addCommasToNumber(s string) string {
	if len(s) <= 3 {
		return s
	}
	
	var result strings.Builder
	start := len(s) % 3
	if start == 0 {
		start = 3
	}
	
	result.WriteString(s[:start])
	for i := start; i < len(s); i += 3 {
		result.WriteString(",")
		result.WriteString(s[i : i+3])
	}
	
	return result.String()
}


