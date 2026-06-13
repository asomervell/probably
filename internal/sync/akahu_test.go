package sync

import (
	"testing"
	"time"

	"github.com/asomervell/probably/internal/models"
)

func TestMapAkahuAccountType(t *testing.T) {
	tests := []struct {
		name      string
		akahuType string
		expected  models.AccountType
	}{
		{"savings", "SAVINGS", models.AccountTypeAsset},
		{"checking", "CHECKING", models.AccountTypeAsset},
		{"transaction", "TRANSACTION", models.AccountTypeAsset},
		{"term deposit", "TERMDEPOSIT", models.AccountTypeAsset},
		{"kiwisaver", "KIWISAVER", models.AccountTypeAsset},
		{"investment", "INVESTMENT", models.AccountTypeAsset},
		{"credit card", "CREDIT_CARD", models.AccountTypeLiability},
		{"loan", "LOAN", models.AccountTypeLiability},
		{"mortgage", "MORTGAGE", models.AccountTypeLiability},
		{"overdraft", "OVERDRAFT", models.AccountTypeLiability},
		{"unknown defaults to asset", "UNKNOWN", models.AccountTypeAsset},
		{"lowercase savings", "savings", models.AccountTypeAsset},
		{"mixed case", "Savings", models.AccountTypeAsset},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAkahuAccountType(tt.akahuType)
			if got != tt.expected {
				t.Errorf("mapAkahuAccountType(%q) = %q, want %q", tt.akahuType, got, tt.expected)
			}
		})
	}
}

func TestAkahuAPIError(t *testing.T) {
	tests := []struct {
		name           string
		err            *AkahuAPIError
		expectedString string
		isRevoked      bool
		isTransient    bool
	}{
		{
			name: "basic error",
			err: &AkahuAPIError{
				StatusCode: 400,
				Status:     "400 Bad Request",
				Message:    "Invalid request",
			},
			expectedString: "akahu API error: 400 Bad Request - Invalid request",
			isRevoked:      false,
			isTransient:    false,
		},
		{
			name: "unauthorized error",
			err: &AkahuAPIError{
				StatusCode: 401,
				Status:     "401 Unauthorized",
				Message:    "Token expired",
			},
			expectedString: "akahu API error: 401 Unauthorized - Token expired",
			isRevoked:      true,
			isTransient:    false,
		},
		{
			name: "forbidden error",
			err: &AkahuAPIError{
				StatusCode: 403,
				Status:     "403 Forbidden",
				Message:    "Access revoked",
			},
			expectedString: "akahu API error: 403 Forbidden - Access revoked",
			isRevoked:      true,
			isTransient:    false,
		},
		{
			name: "not found error",
			err: &AkahuAPIError{
				StatusCode: 404,
				Status:     "404 Not Found",
				Message:    "Account not found",
			},
			expectedString: "akahu API error: 404 Not Found - Account not found",
			isRevoked:      true,
			isTransient:    false,
		},
		{
			name: "server error",
			err: &AkahuAPIError{
				StatusCode: 500,
				Status:     "500 Internal Server Error",
				Message:    "Server error",
			},
			expectedString: "akahu API error: 500 Internal Server Error - Server error",
			isRevoked:      false,
			isTransient:    true,
		},
		{
			name: "gateway timeout",
			err: &AkahuAPIError{
				StatusCode: 503,
				Status:     "503 Service Unavailable",
				Message:    "Service unavailable",
			},
			expectedString: "akahu API error: 503 Service Unavailable - Service unavailable",
			isRevoked:      false,
			isTransient:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expectedString {
				t.Errorf("AkahuAPIError.Error() = %q, want %q", got, tt.expectedString)
			}
			if got := tt.err.IsConnectionDisconnected(); got != tt.isRevoked {
				t.Errorf("AkahuAPIError.IsConnectionDisconnected() = %v, want %v", got, tt.isRevoked)
			}
			if got := IsAkahuTransientError(tt.err); got != tt.isTransient {
				t.Errorf("IsAkahuTransientError() = %v, want %v", got, tt.isTransient)
			}
		})
	}
}

func TestAkahuTransactionParsing(t *testing.T) {
	// Test transaction type mapping
	testCases := []struct {
		txnType   string
		amount    float64
		accType   models.AccountType
		isExpense bool
	}{
		{"DEBIT", -100.00, models.AccountTypeAsset, true},       // Negative from asset = expense
		{"CREDIT", 100.00, models.AccountTypeAsset, false},      // Positive to asset = income
		{"EFTPOS", -50.00, models.AccountTypeAsset, true},       // EFTPOS spending
		{"PAYMENT", -200.00, models.AccountTypeAsset, true},     // Bill payment
		{"DEBIT", 100.00, models.AccountTypeLiability, true},    // Debt increase = expense
		{"CREDIT", -100.00, models.AccountTypeLiability, false}, // Debt decrease = income-like
	}

	for _, tc := range testCases {
		t.Run(tc.txnType, func(t *testing.T) {
			amountCents := int64(tc.amount * 100)

			// Determine if expense (matching logic in SyncTransactions)
			isExpense := amountCents < 0
			if tc.accType == models.AccountTypeLiability {
				isExpense = amountCents > 0
			}

			if isExpense != tc.isExpense {
				t.Errorf("Transaction type %s with amount %.2f and account type %s: isExpense = %v, want %v",
					tc.txnType, tc.amount, tc.accType, isExpense, tc.isExpense)
			}
		})
	}
}

func TestAkahuAccountFormattedNumber(t *testing.T) {
	// Test NZ bank account number parsing
	testCases := []struct {
		formatted string
		lastFour  string
	}{
		{"01-1234-1234567-12", "4567"}, // Standard format
		{"12-3456-7890123-00", "0123"},
		{"", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.formatted, func(t *testing.T) {
			// Simulate the last four extraction logic
			var lastFour string
			if tc.formatted != "" {
				parts := splitAccountNumber(tc.formatted)
				if len(parts) >= 3 {
					accountNum := parts[len(parts)-2]
					if len(accountNum) >= 4 {
						lastFour = accountNum[len(accountNum)-4:]
					}
				}
			}

			if lastFour != tc.lastFour {
				t.Errorf("extractLastFour(%q) = %q, want %q", tc.formatted, lastFour, tc.lastFour)
			}
		})
	}
}

func splitAccountNumber(formatted string) []string {
	var parts []string
	var current string
	for _, c := range formatted {
		if c == '-' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func TestAkahuDateParsing(t *testing.T) {
	// Test date parsing formats that Akahu might return
	testCases := []struct {
		dateStr  string
		expected time.Time
		ok       bool
	}{
		{"2024-01-15", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), true},
		{"2024-12-31", time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), true},
		{"invalid", time.Time{}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.dateStr, func(t *testing.T) {
			date, err := time.Parse("2006-01-02", tc.dateStr)
			if tc.ok {
				if err != nil {
					t.Errorf("Expected to parse %q successfully, got error: %v", tc.dateStr, err)
					return
				}
				if !date.Equal(tc.expected) {
					t.Errorf("Parse(%q) = %v, want %v", tc.dateStr, date, tc.expected)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error parsing %q, got nil", tc.dateStr)
				}
			}
		})
	}
}
