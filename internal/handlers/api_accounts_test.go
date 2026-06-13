package handlers_test

import (
	"net/http"
	"testing"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

func TestAPIAccounts_List(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("accounts-list")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create some accounts
	env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	env.CreateAccount(tu.Ledger.ID, "Credit Card", models.AccountTypeLiability)

	// List accounts
	resp := client.Get("/api/v1/accounts")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Type     string `json:"type"`
			Balance  int64  `json:"balance"`
			IsActive bool   `json:"is_active"`
		} `json:"data"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Data) != 3 {
		t.Errorf("Expected 3 accounts, got %d", len(result.Data))
	}

	// Verify all accounts are active
	for _, acc := range result.Data {
		if !acc.IsActive {
			t.Errorf("Account %s should be active", acc.Name)
		}
	}
}

func TestAPIAccounts_Create(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("accounts-create")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tests := []struct {
		name       string
		body       map[string]any
		wantStatus int
		wantError  bool
	}{
		{
			name: "valid asset account",
			body: map[string]any{
				"name":             "My Checking",
				"type":             "asset",
				"institution_name": "Test Bank",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid liability account",
			body: map[string]any{
				"name": "Credit Card",
				"type": "liability",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid income account",
			body: map[string]any{
				"name": "Salary",
				"type": "income",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid expense account",
			body: map[string]any{
				"name": "Groceries",
				"type": "expense",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid equity account",
			body: map[string]any{
				"name": "Opening Balance",
				"type": "equity",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			body: map[string]any{
				"type": "asset",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "missing type",
			body: map[string]any{
				"name": "Test Account",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "invalid type",
			body: map[string]any{
				"name": "Test Account",
				"type": "invalid",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := client.Post("/api/v1/accounts", tt.body)
			AssertStatus(t, resp, tt.wantStatus)

			if !tt.wantError && tt.wantStatus == http.StatusCreated {
				var result struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					Type     string `json:"type"`
					IsActive bool   `json:"is_active"`
				}
				ParseJSON(t, resp, &result)

				if result.Name != tt.body["name"] {
					t.Errorf("Expected name %s, got %s", tt.body["name"], result.Name)
				}
				if result.Type != tt.body["type"] {
					t.Errorf("Expected type %s, got %s", tt.body["type"], result.Type)
				}
				if !result.IsActive {
					t.Error("Expected account to be active")
				}
			}
		})
	}
}

func TestAPIAccounts_Get(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("accounts-get")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create an account
	acc := env.CreateAccount(tu.Ledger.ID, "Test Account", models.AccountTypeAsset)

	// Get the account
	resp := client.Get("/api/v1/accounts/" + acc.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		Balance  int64  `json:"balance"`
		IsActive bool   `json:"is_active"`
	}
	ParseJSON(t, resp, &result)

	if result.ID != acc.ID.String() {
		t.Errorf("Expected ID %s, got %s", acc.ID.String(), result.ID)
	}
	if result.Name != "Test Account" {
		t.Errorf("Expected name 'Test Account', got %s", result.Name)
	}
	if result.Balance != 0 {
		t.Errorf("Expected balance 0, got %d", result.Balance)
	}
}

func TestAPIAccounts_GetNotFound(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("accounts-get-notfound")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Try to get a non-existent account
	resp := client.Get("/api/v1/accounts/" + uuid.New().String())
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestAPIAccounts_Update(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("accounts-update")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create an account
	acc := env.CreateAccount(tu.Ledger.ID, "Original Name", models.AccountTypeAsset)

	// Update the account
	resp := client.Put("/api/v1/accounts/"+acc.ID.String(), map[string]any{
		"name":             "Updated Name",
		"institution_name": "New Bank",
	})
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		InstitutionName string `json:"institution_name"`
	}
	ParseJSON(t, resp, &result)

	if result.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", result.Name)
	}
	if result.InstitutionName != "New Bank" {
		t.Errorf("Expected institution 'New Bank', got %s", result.InstitutionName)
	}
}

func TestAPIAccounts_UpdateType(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("accounts-update-type")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create an asset account
	acc := env.CreateAccount(tu.Ledger.ID, "Test Account", models.AccountTypeAsset)

	// Change type to liability
	resp := client.Put("/api/v1/accounts/"+acc.ID.String(), map[string]any{
		"type": "liability",
	})
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Type string `json:"type"`
	}
	ParseJSON(t, resp, &result)

	if result.Type != "liability" {
		t.Errorf("Expected type 'liability', got %s", result.Type)
	}
}

func TestAPIAccounts_Delete(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("accounts-delete")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create an account
	acc := env.CreateAccount(tu.Ledger.ID, "To Delete", models.AccountTypeAsset)

	// Delete the account
	resp := client.Delete("/api/v1/accounts/" + acc.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Deleted bool `json:"deleted"`
	}
	ParseJSON(t, resp, &result)

	if !result.Deleted {
		t.Error("Expected deleted to be true")
	}

	// Verify it's gone
	resp = client.Get("/api/v1/accounts/" + acc.ID.String())
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestAPIAccounts_CrossUserIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	// Create two users
	user1 := env.CreateTestUser("accounts-isolation-1")
	defer env.CleanupTestUser(user1)
	user2 := env.CreateTestUser("accounts-isolation-2")
	defer env.CleanupTestUser(user2)

	client1 := env.NewAPIClient(user1)
	client2 := env.NewAPIClient(user2)

	// User 1 creates an account
	acc := env.CreateAccount(user1.Ledger.ID, "User1 Account", models.AccountTypeAsset)

	// User 2 should not be able to access it
	resp := client2.Get("/api/v1/accounts/" + acc.ID.String())
	AssertStatus(t, resp, http.StatusNotFound)

	// User 2 should not be able to update it
	resp = client2.Put("/api/v1/accounts/"+acc.ID.String(), map[string]any{
		"name": "Hacked",
	})
	AssertStatus(t, resp, http.StatusNotFound)

	// User 2 should not be able to delete it
	resp = client2.Delete("/api/v1/accounts/" + acc.ID.String())
	AssertStatus(t, resp, http.StatusNotFound)

	// User 1 should still be able to access it
	resp = client1.Get("/api/v1/accounts/" + acc.ID.String())
	AssertStatus(t, resp, http.StatusOK)
}

func TestAPIAccounts_BalanceCalculation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("accounts-balance")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create checking and expense accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	// Create a transaction: $50 expense from checking
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "Grocery Store", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"}, // Debit expense
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"}, // Credit checking
	})

	// Create another transaction: $100 deposit
	income := env.CreateAccount(tu.Ledger.ID, "Salary", models.AccountTypeIncome)
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "Paycheck", []*models.Entry{
		{AccountID: checking.ID, AmountCents: 10000, Currency: "USD"}, // Debit checking
		{AccountID: income.ID, AmountCents: -10000, Currency: "USD"},  // Credit income
	})

	// Check checking balance
	resp := client.Get("/api/v1/accounts/" + checking.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Balance int64 `json:"balance"`
	}
	ParseJSON(t, resp, &result)

	// Expected balance: -5000 + 10000 = 5000 ($50.00)
	if result.Balance != 5000 {
		t.Errorf("Expected balance 5000, got %d", result.Balance)
	}
}

func TestAPIAccounts_Unauthorized(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	// Create a client without a valid API key
	client := &APIClient{
		BaseURL: env.Server.URL,
		APIKey:  "invalid-key",
		T:       t,
	}

	resp := client.Get("/api/v1/accounts")
	AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestAPIAccounts_MissingAuthHeader(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	// Make request without auth header
	req, _ := http.NewRequest(http.MethodGet, env.Server.URL+"/api/v1/accounts", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}
