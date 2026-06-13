package extraction

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// =============================================
// ExtractedTransaction JSON Unmarshaling Tests
// =============================================

func TestExtractedTransaction_UnmarshalJSON_ValidDate(t *testing.T) {
	jsonData := `{
		"date": "2024-01-15",
		"description": "Test Transaction",
		"amount_cents": -5000
	}`

	var txn ExtractedTransaction
	err := json.Unmarshal([]byte(jsonData), &txn)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	expectedDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	if !txn.Date.Equal(expectedDate) {
		t.Errorf("Date: expected %v, got %v", expectedDate, txn.Date)
	}
	if txn.Description != "Test Transaction" {
		t.Errorf("Description: expected 'Test Transaction', got %q", txn.Description)
	}
	if txn.AmountCents != -5000 {
		t.Errorf("AmountCents: expected -5000, got %d", txn.AmountCents)
	}
}

func TestExtractedTransaction_UnmarshalJSON_WithMerchant(t *testing.T) {
	jsonData := `{
		"date": "2024-01-15",
		"description": "WALMART STORE #1234",
		"amount_cents": -8500,
		"merchant": "Walmart",
		"category": "Groceries",
		"confidence": 0.95
	}`

	var txn ExtractedTransaction
	err := json.Unmarshal([]byte(jsonData), &txn)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if txn.Merchant != "Walmart" {
		t.Errorf("Merchant: expected 'Walmart', got %q", txn.Merchant)
	}
	if txn.Category != "Groceries" {
		t.Errorf("Category: expected 'Groceries', got %q", txn.Category)
	}
	if txn.Confidence != 0.95 {
		t.Errorf("Confidence: expected 0.95, got %f", txn.Confidence)
	}
}

func TestExtractedTransaction_UnmarshalJSON_NullMerchant(t *testing.T) {
	jsonData := `{
		"date": "2024-01-15",
		"description": "Unknown Transaction",
		"amount_cents": -1000,
		"merchant": null
	}`

	var txn ExtractedTransaction
	err := json.Unmarshal([]byte(jsonData), &txn)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if txn.Merchant != "" {
		t.Errorf("Merchant should be empty for null, got %q", txn.Merchant)
	}
}

func TestExtractedTransaction_UnmarshalJSON_InvalidDate(t *testing.T) {
	jsonData := `{
		"date": "not-a-date",
		"description": "Test",
		"amount_cents": -1000
	}`

	var txn ExtractedTransaction
	err := json.Unmarshal([]byte(jsonData), &txn)
	if err == nil {
		t.Error("Expected error for invalid date format")
	}
}

func TestExtractedTransaction_UnmarshalJSON_MalformedJSON(t *testing.T) {
	jsonData := `{not valid json}`

	var txn ExtractedTransaction
	err := json.Unmarshal([]byte(jsonData), &txn)
	if err == nil {
		t.Error("Expected error for malformed JSON")
	}
}

func TestExtractedTransaction_UnmarshalJSON_PositiveAmount(t *testing.T) {
	// Credits are positive amounts
	jsonData := `{
		"date": "2024-01-01",
		"description": "Paycheck Deposit",
		"amount_cents": 350000
	}`

	var txn ExtractedTransaction
	err := json.Unmarshal([]byte(jsonData), &txn)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if txn.AmountCents != 350000 {
		t.Errorf("AmountCents: expected 350000, got %d", txn.AmountCents)
	}
}

// =============================================
// ValidateTransactions Tests
// =============================================

func TestValidateTransactions_AllValid(t *testing.T) {
	txns := []ExtractedTransaction{
		{Date: time.Now(), Description: "Transaction 1", AmountCents: -1000},
		{Date: time.Now(), Description: "Transaction 2", AmountCents: 2000},
		{Date: time.Now(), Description: "Transaction 3", AmountCents: -500},
	}

	result := ValidateTransactions(context.Background(), txns)
	if len(result) != 3 {
		t.Errorf("Expected 3 valid transactions, got %d", len(result))
	}
}

func TestValidateTransactions_EmptyDescription(t *testing.T) {
	txns := []ExtractedTransaction{
		{Date: time.Now(), Description: "", AmountCents: -1000},
		{Date: time.Now(), Description: "Valid", AmountCents: -2000},
	}

	result := ValidateTransactions(context.Background(), txns)
	if len(result) != 1 {
		t.Errorf("Expected 1 valid transaction (empty description filtered), got %d", len(result))
	}
	if result[0].Description != "Valid" {
		t.Errorf("Expected 'Valid' transaction to remain, got %q", result[0].Description)
	}
}

func TestValidateTransactions_ZeroAmount(t *testing.T) {
	txns := []ExtractedTransaction{
		{Date: time.Now(), Description: "Zero Amount", AmountCents: 0},
		{Date: time.Now(), Description: "Has Amount", AmountCents: -500},
	}

	result := ValidateTransactions(context.Background(), txns)
	if len(result) != 1 {
		t.Errorf("Expected 1 valid transaction (zero amount filtered), got %d", len(result))
	}
}

func TestValidateTransactions_ZeroDate(t *testing.T) {
	var zeroTime time.Time
	txns := []ExtractedTransaction{
		{Date: zeroTime, Description: "No Date", AmountCents: -1000},
		{Date: time.Now(), Description: "Has Date", AmountCents: -2000},
	}

	result := ValidateTransactions(context.Background(), txns)
	if len(result) != 1 {
		t.Errorf("Expected 1 valid transaction (zero date filtered), got %d", len(result))
	}
}

func TestValidateTransactions_MultipleInvalid(t *testing.T) {
	var zeroTime time.Time
	txns := []ExtractedTransaction{
		{Date: zeroTime, Description: "", AmountCents: 0},           // All invalid
		{Date: time.Now(), Description: "Valid", AmountCents: -500}, // Valid
		{Date: time.Now(), Description: "", AmountCents: -1000},     // Invalid: no description
		{Date: zeroTime, Description: "Test", AmountCents: -200},    // Invalid: no date
	}

	result := ValidateTransactions(context.Background(), txns)
	if len(result) != 1 {
		t.Errorf("Expected 1 valid transaction, got %d", len(result))
	}
}

func TestValidateTransactions_EmptyInput(t *testing.T) {
	ctx := context.Background()

	result := ValidateTransactions(ctx, nil)
	if len(result) != 0 {
		t.Errorf("Expected nil or empty result for nil input, got %d", len(result))
	}

	result = ValidateTransactions(ctx, []ExtractedTransaction{})
	if len(result) != 0 {
		t.Errorf("Expected empty result for empty input, got %d", len(result))
	}
}

// =============================================
// Batch Unmarshal Tests
// =============================================

func TestExtractedTransaction_BatchUnmarshal(t *testing.T) {
	jsonData := `[
		{"date": "2024-01-01", "description": "Paycheck", "amount_cents": 350000},
		{"date": "2024-01-05", "description": "Groceries", "amount_cents": -8500},
		{"date": "2024-01-10", "description": "Electric Bill", "amount_cents": -12000},
		{"date": "2024-01-15", "description": "Coffee Shop", "amount_cents": -550}
	]`

	var txns []ExtractedTransaction
	err := json.Unmarshal([]byte(jsonData), &txns)
	if err != nil {
		t.Fatalf("Batch unmarshal failed: %v", err)
	}

	if len(txns) != 4 {
		t.Errorf("Expected 4 transactions, got %d", len(txns))
	}

	// Verify first transaction
	if txns[0].Description != "Paycheck" {
		t.Errorf("First transaction description: expected 'Paycheck', got %q", txns[0].Description)
	}
	if txns[0].AmountCents != 350000 {
		t.Errorf("First transaction amount: expected 350000, got %d", txns[0].AmountCents)
	}

	// Verify a debit transaction
	if txns[1].AmountCents != -8500 {
		t.Errorf("Second transaction amount: expected -8500, got %d", txns[1].AmountCents)
	}
}
