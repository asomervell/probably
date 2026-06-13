package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

func TestAPITransactions_List(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-list")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	// Create some transactions
	for i := 0; i < 5; i++ {
		env.CreateTransaction(tu.Ledger.ID, time.Now().AddDate(0, 0, -i), fmt.Sprintf("Transaction %d", i), []*models.Entry{
			{AccountID: groceries.ID, AmountCents: int64((i + 1) * 1000), Currency: "USD"},
			{AccountID: checking.ID, AmountCents: int64(-(i + 1) * 1000), Currency: "USD"},
		})
	}

	// List transactions
	resp := client.Get("/api/v1/transactions")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Date        string `json:"date"`
			Entries     []struct {
				AccountID   string `json:"account_id"`
				AmountCents int64  `json:"amount_cents"`
			} `json:"entries"`
		} `json:"data"`
		Pagination struct {
			Total      int `json:"total"`
			Page       int `json:"page"`
			PerPage    int `json:"per_page"`
			TotalPages int `json:"total_pages"`
		} `json:"pagination"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Data) != 5 {
		t.Errorf("Expected 5 transactions, got %d", len(result.Data))
	}
	if result.Pagination.Total != 5 {
		t.Errorf("Expected total 5, got %d", result.Pagination.Total)
	}
}

func TestAPITransactions_ListWithFilters(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-list-filter")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	// Create transactions
	env.CreateTransaction(tu.Ledger.ID, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "Grocery Store", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})
	env.CreateTransaction(tu.Ledger.ID, time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC), "Transfer to Savings", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 10000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -10000, Currency: "USD"},
	})
	env.CreateTransaction(tu.Ledger.ID, time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), "More Groceries", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 7500, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -7500, Currency: "USD"},
	})

	// Filter by account
	resp := client.Get("/api/v1/transactions?account_id=" + savings.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data       []struct{ ID string } `json:"data"`
		Pagination struct{ Total int }   `json:"pagination"`
	}
	ParseJSON(t, resp, &result)

	if result.Pagination.Total != 1 {
		t.Errorf("Expected 1 transaction for savings account, got %d", result.Pagination.Total)
	}

	// Filter by date range
	resp = client.Get("/api/v1/transactions?start_date=2024-01-01&end_date=2024-01-31")
	AssertStatus(t, resp, http.StatusOK)
	ParseJSON(t, resp, &result)

	if result.Pagination.Total != 2 {
		t.Errorf("Expected 2 transactions in January, got %d", result.Pagination.Total)
	}

	// Filter by search (search for common substring "Grocer")
	resp = client.Get("/api/v1/transactions?search=Grocer")
	AssertStatus(t, resp, http.StatusOK)
	ParseJSON(t, resp, &result)

	if result.Pagination.Total != 2 {
		t.Errorf("Expected 2 transactions matching 'Grocer', got %d", result.Pagination.Total)
	}
}

func TestAPITransactions_Create(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-create")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	// Create a valid transaction
	resp := client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-15",
		"description": "Test Purchase",
		"notes":       "Some notes",
		"entries": []map[string]any{
			{"account_id": groceries.ID.String(), "amount_cents": 5000, "currency": "USD"},
			{"account_id": checking.ID.String(), "amount_cents": -5000, "currency": "USD"},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)

	var result struct {
		ID          string `json:"id"`
		Description string `json:"description"`
		Date        string `json:"date"`
		Notes       string `json:"notes"`
		Entries     []struct {
			AccountID   string `json:"account_id"`
			AmountCents int64  `json:"amount_cents"`
		} `json:"entries"`
	}
	ParseJSON(t, resp, &result)

	if result.Description != "Test Purchase" {
		t.Errorf("Expected description 'Test Purchase', got %s", result.Description)
	}
	if result.Date != "2024-01-15" {
		t.Errorf("Expected date '2024-01-15', got %s", result.Date)
	}
	if len(result.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(result.Entries))
	}
}

func TestAPITransactions_CreateUnbalanced(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-create-unbalanced")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	// Try to create an unbalanced transaction
	resp := client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-15",
		"description": "Unbalanced",
		"entries": []map[string]any{
			{"account_id": groceries.ID.String(), "amount_cents": 5000},
			{"account_id": checking.ID.String(), "amount_cents": -4000}, // Wrong!
		},
	})
	AssertStatus(t, resp, http.StatusBadRequest)

	var result struct {
		Error string `json:"error"`
	}
	ParseJSON(t, resp, &result)

	if result.Error == "" {
		t.Error("Expected error message for unbalanced transaction")
	}
}

func TestAPITransactions_CreateSingleEntry(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-create-single")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)

	// Try to create with single entry (should fail)
	resp := client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-15",
		"description": "Single Entry",
		"entries": []map[string]any{
			{"account_id": checking.ID.String(), "amount_cents": 5000},
		},
	})
	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestAPITransactions_Get(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-get")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	txn := env.CreateTransaction(tu.Ledger.ID, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "Test Transaction", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})

	resp := client.Get("/api/v1/transactions/" + txn.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		ID          string `json:"id"`
		Description string `json:"description"`
		Entries     []struct {
			AccountID   string `json:"account_id"`
			AccountName string `json:"account_name"`
			AmountCents int64  `json:"amount_cents"`
		} `json:"entries"`
	}
	ParseJSON(t, resp, &result)

	if result.ID != txn.ID.String() {
		t.Errorf("Expected ID %s, got %s", txn.ID.String(), result.ID)
	}
	if len(result.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(result.Entries))
	}
}

func TestAPITransactions_GetNotFound(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-get-notfound")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	resp := client.Get("/api/v1/transactions/" + uuid.New().String())
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestAPITransactions_Update(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-update")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	txn := env.CreateTransaction(tu.Ledger.ID, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "Original", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})

	resp := client.Put("/api/v1/transactions/"+txn.ID.String(), map[string]any{
		"description": "Updated Description",
		"notes":       "New notes",
		"date":        "2024-01-20",
	})
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Description string `json:"description"`
		Notes       string `json:"notes"`
		Date        string `json:"date"`
	}
	ParseJSON(t, resp, &result)

	if result.Description != "Updated Description" {
		t.Errorf("Expected description 'Updated Description', got %s", result.Description)
	}
	if result.Notes != "New notes" {
		t.Errorf("Expected notes 'New notes', got %s", result.Notes)
	}
	if result.Date != "2024-01-20" {
		t.Errorf("Expected date '2024-01-20', got %s", result.Date)
	}
}

func TestAPITransactions_Delete(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-delete")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	txn := env.CreateTransaction(tu.Ledger.ID, timeNow(), "To Delete", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})

	resp := client.Delete("/api/v1/transactions/" + txn.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	// Verify it's gone
	resp = client.Get("/api/v1/transactions/" + txn.ID.String())
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestAPITransactions_AddTag(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-addtag")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)
	tag := env.CreateTag(tu.Ledger.ID, "Food", "#ff0000")

	txn := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Grocery Store", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})

	resp := client.Post("/api/v1/transactions/"+txn.ID.String()+"/tags", map[string]any{
		"tag_id": tag.ID.String(),
	})
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Tags []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"tags"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Tags) != 1 {
		t.Errorf("Expected 1 tag, got %d", len(result.Tags))
	}
	if result.Tags[0].Name != "Food" {
		t.Errorf("Expected tag 'Food', got %s", result.Tags[0].Name)
	}
}

func TestAPITransactions_RemoveTag(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-removetag")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)
	tag := env.CreateTag(tu.Ledger.ID, "Food", "#ff0000")

	txn := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Grocery Store", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})

	// Add tag first
	client.Post("/api/v1/transactions/"+txn.ID.String()+"/tags", map[string]any{
		"tag_id": tag.ID.String(),
	})

	// Remove tag
	resp := client.Delete("/api/v1/transactions/" + txn.ID.String() + "/tags/" + tag.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Tags []struct{ ID string } `json:"tags"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Tags) != 0 {
		t.Errorf("Expected 0 tags, got %d", len(result.Tags))
	}
}

func TestAPITransactions_Pagination(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-pagination")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	// Create 25 transactions
	for i := 0; i < 25; i++ {
		env.CreateTransaction(tu.Ledger.ID, time.Now().AddDate(0, 0, -i), fmt.Sprintf("Transaction %d", i), []*models.Entry{
			{AccountID: groceries.ID, AmountCents: int64((i + 1) * 100), Currency: "USD"},
			{AccountID: checking.ID, AmountCents: int64(-(i + 1) * 100), Currency: "USD"},
		})
	}

	// Get page 1 with 10 per page
	resp := client.Get("/api/v1/transactions?per_page=10&page=1")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data       []struct{ ID string } `json:"data"`
		Pagination struct {
			Total      int `json:"total"`
			Page       int `json:"page"`
			PerPage    int `json:"per_page"`
			TotalPages int `json:"total_pages"`
		} `json:"pagination"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Data) != 10 {
		t.Errorf("Expected 10 transactions on page 1, got %d", len(result.Data))
	}
	if result.Pagination.Total != 25 {
		t.Errorf("Expected total 25, got %d", result.Pagination.Total)
	}
	if result.Pagination.TotalPages != 3 {
		t.Errorf("Expected 3 pages, got %d", result.Pagination.TotalPages)
	}

	// Get page 3
	resp = client.Get("/api/v1/transactions?per_page=10&page=3")
	AssertStatus(t, resp, http.StatusOK)
	ParseJSON(t, resp, &result)

	if len(result.Data) != 5 {
		t.Errorf("Expected 5 transactions on page 3, got %d", len(result.Data))
	}
}

func TestAPITransactions_CrossUserIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	user1 := env.CreateTestUser("txn-iso-1")
	defer env.CleanupTestUser(user1)
	user2 := env.CreateTestUser("txn-iso-2")
	defer env.CleanupTestUser(user2)

	client1 := env.NewAPIClient(user1)
	client2 := env.NewAPIClient(user2)

	// User 1 creates accounts and transaction
	checking1 := env.CreateAccount(user1.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries1 := env.CreateAccount(user1.Ledger.ID, "Groceries", models.AccountTypeExpense)
	txn := env.CreateTransaction(user1.Ledger.ID, timeNow(), "User1 Transaction", []*models.Entry{
		{AccountID: groceries1.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking1.ID, AmountCents: -5000, Currency: "USD"},
	})

	// User 2 should not be able to access
	resp := client2.Get("/api/v1/transactions/" + txn.ID.String())
	AssertStatus(t, resp, http.StatusNotFound)

	// User 2 should not see it in list
	resp = client2.Get("/api/v1/transactions")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Pagination struct{ Total int } `json:"pagination"`
	}
	ParseJSON(t, resp, &result)

	if result.Pagination.Total != 0 {
		t.Errorf("User 2 should see 0 transactions, got %d", result.Pagination.Total)
	}

	// User 1 should be able to access
	resp = client1.Get("/api/v1/transactions/" + txn.ID.String())
	AssertStatus(t, resp, http.StatusOK)
}
