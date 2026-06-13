package chat

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reSQLLineComment  = regexp.MustCompile(`--.*`)
	reSQLBlockComment = regexp.MustCompile(`/\*.*?\*/`)
	reWhitespace      = regexp.MustCompile(`\s+`)
	reFromTable       = regexp.MustCompile(`FROM\s+(\w+)`)
	reJoinTable       = regexp.MustCompile(`JOIN\s+(\w+)`)
)

// SQLValidator validates SQL queries for safe execution
type SQLValidator struct {
	allowedTables   map[string]bool
	allowedColumns  map[string]map[string]bool // table -> columns
	dangerousFuncs   []string
	systemTables     []string
}

// NewSQLValidator creates a new SQL validator with default whitelists
func NewSQLValidator() *SQLValidator {
	// Define allowed tables for chat queries
	allowedTables := map[string]bool{
		"transactions":          true,
		"entries":               true,
		"accounts":               true,
		"tags":                  true,
		"transaction_tags":      true,
		"entities":              true,
		"entity_relationships":  true,
	}

	// Define allowed columns per table
	allowedColumns := map[string]map[string]bool{
		"transactions": {
			"id":                true,
			"ledger_id":         true,
			"date":              true,
			"description":      true,
			"display_title":     true,
			"notes":             true,
			"is_transfer":       true,
			"transfer_pair_id":  true,
			"created_at":        true,
			"updated_at":        true,
		},
		"entries": {
			"id":             true,
			"transaction_id": true,
			"account_id":     true,
			"amount_cents":   true,
			"currency":       true,
			"created_at":     true,
		},
		"accounts": {
			"id":              true,
			"ledger_id":       true,
			"name":            true,
			"type":            true,
			"institution_name": true,
			"is_active":       true,
			"created_at":      true,
			"updated_at":      true,
		},
		"tags": {
			"id":        true,
			"ledger_id": true,
			"parent_id": true,
			"name":      true,
			"color":     true,
			"created_at": true,
			"updated_at": true,
		},
		"transaction_tags": {
			"transaction_id": true,
			"tag_id":         true,
			"created_at":     true,
		},
		"entities": {
			"id":          true,
			"name":        true,
			"type":        true,
			"subtype":     true,
			"website":     true,
			"description": true,
			"created_at":  true,
			"updated_at":  true,
		},
		"entity_relationships": {
			"ledger_id":        true,
			"entity_a_id":      true,
			"entity_b_id":      true,
			"relationship_type": true,
			"created_at":       true,
		},
	}

	// Dangerous functions to block
	dangerousFuncs := []string{
		"pg_",           // PostgreSQL system functions
		"current_user",  // User context functions
		"session_user",
		"user",
		"version",       // Version info
		"COPY",          // COPY command
		"EXECUTE",       // Dynamic SQL
		"EXEC",          // Dynamic SQL
	}

	// System tables to block
	systemTables := []string{
		"pg_",
		"information_schema",
		"pg_catalog",
		"pg_toast",
		"pg_temp",
	}

	return &SQLValidator{
		allowedTables:  allowedTables,
		allowedColumns: allowedColumns,
		dangerousFuncs:  dangerousFuncs,
		systemTables:    systemTables,
	}
}

// Validate validates a SQL query for safe execution
func (v *SQLValidator) Validate(sql string, ledgerID string) error {
	if sql == "" {
		return fmt.Errorf("empty SQL query")
	}

	// Normalize SQL: remove comments, normalize whitespace
	normalized := v.normalizeSQL(sql)

	// Check 1: Must be a SELECT statement
	if !v.isSelectOnly(normalized) {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	// Check 2: Must filter by ledger_id
	if !v.hasLedgerFilter(normalized, ledgerID) {
		return fmt.Errorf("query must filter by ledger_id = $1 (for security)")
	}

	// Check 3: No dangerous functions
	if v.containsDangerousFunctions(normalized) {
		return fmt.Errorf("query contains disallowed functions")
	}

	// Check 4: No system tables
	if v.accessesSystemTables(normalized) {
		return fmt.Errorf("cannot access system tables")
	}

	// Check 5: Only whitelisted tables
	if !v.onlyWhitelistedTables(normalized) {
		return fmt.Errorf("query references non-whitelisted tables")
	}

	// Check 6: No DML operations (INSERT, UPDATE, DELETE)
	if v.containsDML(normalized) {
		return fmt.Errorf("DML operations (INSERT, UPDATE, DELETE) are not allowed")
	}

	return nil
}

// normalizeSQL normalizes SQL for easier parsing
func (v *SQLValidator) normalizeSQL(sql string) string {
	sql = reSQLLineComment.ReplaceAllString(sql, "")
	sql = reSQLBlockComment.ReplaceAllString(sql, "")
	sql = reWhitespace.ReplaceAllString(sql, " ")
	return strings.ToUpper(strings.TrimSpace(sql))
}

// isSelectOnly checks if the query is a SELECT statement
func (v *SQLValidator) isSelectOnly(sql string) bool {
	// Must start with SELECT
	if !strings.HasPrefix(sql, "SELECT") {
		return false
	}

	// Check for DML keywords
	dmlKeywords := []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE", "GRANT", "REVOKE"}
	for _, keyword := range dmlKeywords {
		if strings.Contains(sql, " "+keyword+" ") || strings.Contains(sql, "\n"+keyword+" ") {
			return false
		}
	}

	return true
}

// hasLedgerFilter checks if the query filters by ledger_id
func (v *SQLValidator) hasLedgerFilter(sql string, ledgerID string) bool {
	// Look for ledger_id filter patterns
	patterns := []string{
		"LEDGER_ID = $1",
		"LEDGER_ID=$1",
		"T.LEDGER_ID = $1",
		"T.LEDGER_ID=$1",
		"TRANSACTIONS.LEDGER_ID = $1",
		"ACCOUNTS.LEDGER_ID = $1",
		"TAGS.LEDGER_ID = $1",
		"ENTITY_RELATIONSHIPS.LEDGER_ID = $1",
	}

	upperSQL := strings.ToUpper(sql)
	for _, pattern := range patterns {
		if strings.Contains(upperSQL, pattern) {
			return true
		}
	}

	// Also check for WHERE clauses that might contain ledger_id
	// This is a basic check - more sophisticated parsing would be better
	if strings.Contains(upperSQL, "WHERE") {
		// Check if ledger_id appears in WHERE clause
		whereIndex := strings.Index(upperSQL, "WHERE")
		if whereIndex >= 0 {
			whereClause := upperSQL[whereIndex:]
			if strings.Contains(whereClause, "LEDGER_ID") {
				// Basic check - in production, use proper SQL parser
				return true
			}
		}
	}

	return false
}

// containsDangerousFunctions checks for dangerous function calls
func (v *SQLValidator) containsDangerousFunctions(sql string) bool {
	upperSQL := strings.ToUpper(sql)
	for _, funcName := range v.dangerousFuncs {
		if strings.Contains(upperSQL, funcName) {
			return true
		}
	}
	return false
}

// accessesSystemTables checks for system table access
func (v *SQLValidator) accessesSystemTables(sql string) bool {
	upperSQL := strings.ToUpper(sql)
	for _, prefix := range v.systemTables {
		if strings.Contains(upperSQL, prefix) {
			return true
		}
	}
	return false
}

// onlyWhitelistedTables checks if only whitelisted tables are referenced
func (v *SQLValidator) onlyWhitelistedTables(sql string) bool {
	upperSQL := strings.ToUpper(sql)

	// Extract table names from FROM and JOIN clauses
	matches := reFromTable.FindAllStringSubmatch(upperSQL, -1)
	matches = append(matches, reJoinTable.FindAllStringSubmatch(upperSQL, -1)...)

	for _, match := range matches {
		if len(match) > 1 {
			tableName := strings.ToLower(match[1])
			// Remove table aliases (e.g., "transactions t" -> "transactions")
			tableName = strings.Fields(tableName)[0]
			if !v.allowedTables[tableName] {
				return false
			}
		}
	}

	return true
}

// containsDML checks for DML operations
func (v *SQLValidator) containsDML(sql string) bool {
	dmlKeywords := []string{"INSERT", "UPDATE", "DELETE", "MERGE", "UPSERT"}
	upperSQL := strings.ToUpper(sql)
	for _, keyword := range dmlKeywords {
		if strings.Contains(upperSQL, " "+keyword+" ") || strings.Contains(upperSQL, "\n"+keyword+" ") {
			return true
		}
	}
	return false
}
