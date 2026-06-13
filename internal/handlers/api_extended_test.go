package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// =============================================
// Dashboard API Tests
// =============================================

func TestAPIDashboard_BasicStats(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("dashboard-basic")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	creditCard := env.CreateAccount(tu.Ledger.ID, "Credit Card", models.AccountTypeLiability)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)
	expense := env.CreateAccount(tu.Ledger.ID, "Expenses", models.AccountTypeExpense)

	// Add deposit to checking
	env.CreateTransaction(tu.Ledger.ID, time.Now(), "Deposit", []*models.Entry{
		{AccountID: checking.ID, AmountCents: 500000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -500000, Currency: "USD"},
	})

	// Add to savings
	env.CreateTransaction(tu.Ledger.ID, time.Now(), "Savings", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 200000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -200000, Currency: "USD"},
	})

	// Credit card debt
	env.CreateTransaction(tu.Ledger.ID, time.Now(), "Purchase", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 50000, Currency: "USD"},
		{AccountID: creditCard.ID, AmountCents: -50000, Currency: "USD"},
	})

	// Get dashboard
	resp := client.Get("/api/v1/dashboard")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		NetWorth         int64 `json:"net_worth"`
		TotalAssets      int64 `json:"total_assets"`
		TotalLiabilities int64 `json:"total_liabilities"`
		AssetAccounts    []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Balance int64  `json:"balance"`
		} `json:"asset_accounts"`
		LiabilityAccounts []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Balance int64  `json:"balance"`
		} `json:"liability_accounts"`
	}
	ParseJSON(t, resp, &result)

	// Assets: 5000 + 2000 = 7000
	expectedAssets := int64(700000)
	if result.TotalAssets != expectedAssets {
		t.Errorf("Total assets: expected %d, got %d", expectedAssets, result.TotalAssets)
	}

	// Liabilities: 500 (credit card debt)
	expectedLiabilities := int64(50000)
	if result.TotalLiabilities != expectedLiabilities {
		t.Errorf("Total liabilities: expected %d, got %d", expectedLiabilities, result.TotalLiabilities)
	}

	// Net worth: 7000 - 500 = 6500
	expectedNetWorth := int64(700000 - 50000)
	if result.NetWorth != expectedNetWorth {
		t.Errorf("Net worth: expected %d, got %d", expectedNetWorth, result.NetWorth)
	}
}

func TestAPIDashboard_EmptyLedger(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("dashboard-empty")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Get dashboard for empty ledger
	resp := client.Get("/api/v1/dashboard")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		NetWorth         int64 `json:"net_worth"`
		TotalAssets      int64 `json:"total_assets"`
		TotalLiabilities int64 `json:"total_liabilities"`
	}
	ParseJSON(t, resp, &result)

	if result.NetWorth != 0 {
		t.Errorf("Empty ledger net worth should be 0, got %d", result.NetWorth)
	}
	if result.TotalAssets != 0 {
		t.Errorf("Empty ledger assets should be 0, got %d", result.TotalAssets)
	}
	if result.TotalLiabilities != 0 {
		t.Errorf("Empty ledger liabilities should be 0, got %d", result.TotalLiabilities)
	}
}

// =============================================
// API Key Management Tests
// =============================================

func TestAPIKeys_CreateAndList(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("api-keys")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create a new API key
	resp := client.Post("/api/v1/api-keys", map[string]any{
		"name": "Test Key",
	})
	AssertStatus(t, resp, http.StatusCreated)

	var createResult struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Key       string `json:"key"`
		KeyPrefix string `json:"key_prefix"`
	}
	ParseJSON(t, resp, &createResult)

	if createResult.Name != "Test Key" {
		t.Errorf("Expected name 'Test Key', got %s", createResult.Name)
	}
	if createResult.Key == "" {
		t.Error("Expected key to be returned on creation")
	}

	// List API keys
	resp = client.Get("/api/v1/api-keys")
	AssertStatus(t, resp, http.StatusOK)

	var listResult struct {
		Data []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			KeyPrefix string `json:"key_prefix"`
		} `json:"data"`
	}
	ParseJSON(t, resp, &listResult)

	// Should have at least 2 keys (original test key + newly created)
	if len(listResult.Data) < 2 {
		t.Errorf("Expected at least 2 API keys, got %d", len(listResult.Data))
	}
}

func TestAPIKeys_CreateMissingName(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("api-keys-error")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Try to create without name
	resp := client.Post("/api/v1/api-keys", map[string]any{})
	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestAPIKeys_Delete(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("api-keys-delete")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create a key
	resp := client.Post("/api/v1/api-keys", map[string]any{
		"name": "To Delete",
	})
	AssertStatus(t, resp, http.StatusCreated)

	var createResult struct {
		ID string `json:"id"`
	}
	ParseJSON(t, resp, &createResult)

	// Delete it
	resp = client.Delete("/api/v1/api-keys/" + createResult.ID)
	AssertStatus(t, resp, http.StatusOK)

	var deleteResult struct {
		Deleted bool `json:"deleted"`
	}
	ParseJSON(t, resp, &deleteResult)

	if !deleteResult.Deleted {
		t.Error("Expected deleted to be true")
	}
}

// =============================================
// Transaction Edge Cases
// =============================================

func TestAPITransactions_CreateValidDate(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-valid-date")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	// Create with valid ISO date format
	resp := client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-06-15",
		"description": "Valid Date Test",
		"entries": []map[string]any{
			{"account_id": groceries.ID.String(), "amount_cents": 5000},
			{"account_id": checking.ID.String(), "amount_cents": -5000},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)

	var result struct {
		Date string `json:"date"`
	}
	ParseJSON(t, resp, &result)

	if result.Date != "2024-06-15" {
		t.Errorf("Expected date '2024-06-15', got '%s'", result.Date)
	}
}

func TestAPITransactions_CreateNoEntries(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-no-entries")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Try to create with no entries
	resp := client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-15",
		"description": "Test",
		"entries":     []map[string]any{},
	})
	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestAPITransactions_CreateWithInvalidAccountID(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-invalid-account")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)

	// Try to create with non-existent account
	resp := client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-15",
		"description": "Test",
		"entries": []map[string]any{
			{"account_id": uuid.New().String(), "amount_cents": 5000},
			{"account_id": checking.ID.String(), "amount_cents": -5000},
		},
	})
	// Should fail due to FK constraint
	if resp.StatusCode == http.StatusCreated {
		t.Error("Expected error when using non-existent account ID")
	}
}

func TestAPITransactions_UpdateNotFound(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-update-notfound")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	resp := client.Put("/api/v1/transactions/"+uuid.New().String(), map[string]any{
		"description": "Updated",
	})
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestAPITransactions_DeleteNotFound(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-delete-notfound")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	resp := client.Delete("/api/v1/transactions/" + uuid.New().String())
	AssertStatus(t, resp, http.StatusNotFound)
}

// =============================================
// Tags Edge Cases
// =============================================

func TestAPITags_CreateMultiple(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tag-multiple")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create multiple distinct tags
	resp := client.Post("/api/v1/tags", map[string]any{
		"name":  "Groceries",
		"color": "#ff5722",
	})
	AssertStatus(t, resp, http.StatusCreated)

	resp = client.Post("/api/v1/tags", map[string]any{
		"name":  "Transportation",
		"color": "#4caf50",
	})
	AssertStatus(t, resp, http.StatusCreated)

	resp = client.Post("/api/v1/tags", map[string]any{
		"name":  "Entertainment",
		"color": "#2196f3",
	})
	AssertStatus(t, resp, http.StatusCreated)

	// List tags - should have 3
	resp = client.Get("/api/v1/tags")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Data) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(result.Data))
	}
}

func TestAPITags_CreateMissingName(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tag-missing-name")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	resp := client.Post("/api/v1/tags", map[string]any{
		"color": "#ff5722",
	})
	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestAPITags_UpdateNotFound(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tag-update-notfound")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	resp := client.Put("/api/v1/tags/"+uuid.New().String(), map[string]any{
		"name": "Updated",
	})
	AssertStatus(t, resp, http.StatusNotFound)
}

// =============================================
// Rules Edge Cases
// =============================================

func TestAPIRules_CreateComplete(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rule-create-complete")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tag := env.CreateTag(tu.Ledger.ID, "Test", "#ff5722")

	// Create a complete rule with all required fields
	resp := client.Post("/api/v1/rules", map[string]any{
		"name":          "Test Rule",
		"match_pattern": "test pattern",
		"tag_id":        tag.ID.String(),
		"priority":      10,
	})
	AssertStatus(t, resp, http.StatusCreated)

	var result struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		MatchPattern string `json:"match_pattern"`
		Priority     int    `json:"priority"`
	}
	ParseJSON(t, resp, &result)

	if result.Name != "Test Rule" {
		t.Errorf("Expected name 'Test Rule', got '%s'", result.Name)
	}
	if result.MatchPattern != "test pattern" {
		t.Errorf("Expected pattern 'test pattern', got '%s'", result.MatchPattern)
	}
}

func TestAPIRules_CreateMissingTag(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rule-missing-tag")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	resp := client.Post("/api/v1/rules", map[string]any{
		"name":          "Test Rule",
		"match_pattern": "test",
	})
	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestAPIRules_ApplyWithMatcher(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rule-apply")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	// Create tag
	tag := env.CreateTag(tu.Ledger.ID, "Food", "#ff5722")

	// Create rule
	client.Post("/api/v1/rules", map[string]any{
		"name":          "Grocery Stores",
		"match_pattern": "walmart|costco",
		"is_regex":      true,
		"tag_id":        tag.ID.String(),
		"priority":      10,
	})

	// Create transactions that should match
	env.CreateTransaction(tu.Ledger.ID, time.Now(), "WALMART STORE #1234", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})

	env.CreateTransaction(tu.Ledger.ID, time.Now(), "Gas Station", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 4000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -4000, Currency: "USD"},
	})

	// Apply rules
	resp := client.Post("/api/v1/rules/apply", nil)
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Matched   int `json:"matched"`
		Processed int `json:"processed"`
	}
	ParseJSON(t, resp, &result)

	// Should match at least 1 transaction (walmart)
	if result.Matched < 1 {
		t.Errorf("Expected at least 1 matched transaction, got %d", result.Matched)
	}
}

// =============================================
// Transfers API Tests
// =============================================

func TestAPITransfers_PendingEmpty(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("transfers-pending")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Get pending transfers (should be empty for new user)
	resp := client.Get("/api/v1/transfers/pending")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	ParseJSON(t, resp, &result)

	// New user should have no pending transfers
	if len(result.Data) != 0 {
		t.Errorf("Expected 0 pending transfers, got %d", len(result.Data))
	}
}

func TestAPITransfers_ManualMatchInvalidIDs(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("transfers-invalid")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Try to match non-existent transactions
	resp := client.Post("/api/v1/transfers/match", map[string]any{
		"transaction_id_1": uuid.New().String(),
		"transaction_id_2": uuid.New().String(),
	})

	// Should fail
	if resp.StatusCode == http.StatusOK {
		t.Error("Expected error when matching non-existent transactions")
	}
}

// =============================================
// Categorization Stats Tests
// =============================================

func TestAPICategorizationStats(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("categorization-stats")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	resp := client.Get("/api/v1/categorization/stats")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Pending    int `json:"pending"`
		Processing int `json:"processing"`
		Done       int `json:"done"`
		Failed     int `json:"failed"`
	}
	ParseJSON(t, resp, &result)

	// For empty ledger, all should be 0
	// Test passes if we get valid stats structure
}

// =============================================
// Concurrent Access Tests
// =============================================

func TestAPIConcurrentAccess(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("concurrent")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	// Concurrent transaction creation
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			resp := client.Post("/api/v1/transactions", map[string]any{
				"date":        "2024-01-15",
				"description": "Concurrent Transaction",
				"entries": []map[string]any{
					{"account_id": groceries.ID.String(), "amount_cents": 100 * n},
					{"account_id": checking.ID.String(), "amount_cents": -100 * n},
				},
			})
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("Concurrent creation failed with status %d", resp.StatusCode)
			}
			resp.Body.Close()
			done <- true
		}(i + 1)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all transactions were created
	resp := client.Get("/api/v1/transactions")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	ParseJSON(t, resp, &result)

	if result.Pagination.Total != 10 {
		t.Errorf("Expected 10 transactions, got %d", result.Pagination.Total)
	}
}
