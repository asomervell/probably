package chat

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLExecutor executes SQL queries safely and returns formatted results
type SQLExecutor struct {
	pool  *pgxpool.Pool
	cache *QueryCache
}

// NewSQLExecutor creates a new SQL executor
func NewSQLExecutor(pool *pgxpool.Pool) *SQLExecutor {
	return &SQLExecutor{
		pool:  pool,
		cache: NewQueryCache(5 * time.Minute), // 5 minute TTL
	}
}

// QueryResult represents the result of a SQL query
type QueryResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	Count   int             `json:"count"`
}

// Execute executes a SQL query and returns formatted results
// Results are cached to reduce database load for repeated queries
func (e *SQLExecutor) Execute(ctx context.Context, sql string, ledgerID uuid.UUID, maxRows int) (*QueryResult, error) {
	if maxRows == 0 {
		maxRows = 10000 // Default max rows
	}

	// Check cache first
	if e.cache != nil {
		cacheKey := CacheKey(sql, ledgerID)
		if cached, found := e.cache.Get(cacheKey); found {
			return cached, nil
		}
	}

	// Add LIMIT if not present and query might return many rows
	// This is a safety measure
	if !containsLimit(sql) {
		// Remove trailing semicolon and whitespace before appending LIMIT
		sql = strings.TrimRight(sql, " \t\n\r;")
		sql = sql + fmt.Sprintf(" LIMIT %d", maxRows)
	}

	// Execute query with timeout
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := e.pool.Query(queryCtx, sql, ledgerID)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns := rows.FieldDescriptions()
	columnNames := make([]string, len(columns))
	for i, col := range columns {
		columnNames[i] = col.Name
	}

	// Read rows
	resultRows := make([][]interface{}, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert values to JSON-serializable types
		row := make([]interface{}, len(values))
		for i, val := range values {
			row[i] = convertValue(val)
		}

		resultRows = append(resultRows, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	result := &QueryResult{
		Columns: columnNames,
		Rows:    resultRows,
		Count:   len(resultRows),
	}

	// Cache the result
	if e.cache != nil {
		cacheKey := CacheKey(sql, ledgerID)
		e.cache.Set(cacheKey, result)
	}

	return result, nil
}

// convertValue converts database values to JSON-serializable types
func convertValue(val interface{}) interface{} {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case time.Time:
		return v.Format(time.RFC3339)
	case []byte:
		// Try to parse as number if it looks like one
		str := string(v)
		if num, err := parseNumericString(str); err == nil {
			return num
		}
		return str
	case uuid.UUID:
		return v.String()
	case string:
		// PostgreSQL numeric types are often returned as strings
		// Try to parse as number if it looks like one
		if num, err := parseNumericString(v); err == nil {
			return num
		}
		return v
	case int, int8, int16, int32, int64:
		// Convert integers to float64 for consistent handling
		return float64(getInt64(v))
	case uint, uint8, uint16, uint32, uint64:
		// Convert unsigned integers to float64
		return float64(getUint64(v))
	case float32:
		return float64(v)
	case float64:
		return v
	default:
		// For unknown types, try to convert to string to avoid raw struct output
		// This handles cases where pgx returns custom types
		str := fmt.Sprintf("%v", v)
		
		// Handle struct representations like {mantissa exponent ...} (e.g., big.Float)
		if num, err := extractNumericFromStructString(str); err == nil {
			return num
		}
		
		// Try parsing as regular number string
		if num, err := parseNumericString(str); err == nil {
			return num
		}
		return str
	}
}

// extractNumericFromStructString extracts a numeric value from struct string representations
// Handles formats like "{29124650000000000 -12 false finite true}" (big.Float)
func extractNumericFromStructString(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.Contains(s, "}") {
		return 0, fmt.Errorf("not a struct representation")
	}
	
	// Extract content between braces
	content := s[1:strings.Index(s, "}")]
	parts := strings.Fields(content)
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid struct format")
	}
	
	// First part is mantissa, second is exponent
	mantissa, err := parseNumericString(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid mantissa: %w", err)
	}
	
	exponent, err := parseNumericString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid exponent: %w", err)
	}
	
	// Calculate: mantissa * 10^exponent
	result := mantissa * math.Pow(10, exponent)
	return result, nil
}

// parseNumericString attempts to parse a string as a number
func parseNumericString(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	
	// Try parsing as float64
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	if err != nil {
		return 0, err
	}
	return f, nil
}

// getInt64 safely converts various integer types to int64
func getInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	default:
		return 0
	}
}

// getUint64 safely converts various unsigned integer types to uint64
func getUint64(v interface{}) uint64 {
	switch val := v.(type) {
	case uint:
		return uint64(val)
	case uint8:
		return uint64(val)
	case uint16:
		return uint64(val)
	case uint32:
		return uint64(val)
	case uint64:
		return val
	default:
		return 0
	}
}

// containsLimit checks if SQL contains a LIMIT clause
func containsLimit(sql string) bool {
	// Simple check - in production, use proper SQL parsing
	upperSQL := strings.ToUpper(sql)
	return strings.Contains(upperSQL, " LIMIT ") || strings.Contains(upperSQL, "\nLIMIT ")
}
