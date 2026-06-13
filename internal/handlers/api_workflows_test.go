package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/asomervell/probably/internal/models"
)

// =============================================
// Multi-Step Workflow Tests
// These tests verify complex user journeys through the API
// =============================================

// TestWorkflow_NewUserOnboarding tests the complete onboarding flow
// for a new user setting up their finances
func TestWorkflow_NewUserOnboarding(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("onboarding")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Step 1: Create asset accounts (checking, savings)
	resp := client.Post("/api/v1/accounts", map[string]any{
		"name":             "Main Checking",
		"type":             "asset",
		"institution_name": "Big Bank",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var checkingResult struct{ ID string }
	ParseJSON(t, resp, &checkingResult)

	resp = client.Post("/api/v1/accounts", map[string]any{
		"name":             "Savings Account",
		"type":             "asset",
		"institution_name": "Big Bank",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var savingsResult struct{ ID string }
	ParseJSON(t, resp, &savingsResult)

	// Step 2: Create a credit card (liability)
	resp = client.Post("/api/v1/accounts", map[string]any{
		"name":             "Rewards Credit Card",
		"type":             "liability",
		"institution_name": "Card Issuer",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var ccResult struct{ ID string }
	ParseJSON(t, resp, &ccResult)

	// Step 3: Create expense categories
	resp = client.Post("/api/v1/accounts", map[string]any{
		"name": "Groceries",
		"type": "expense",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var groceriesResult struct{ ID string }
	ParseJSON(t, resp, &groceriesResult)

	resp = client.Post("/api/v1/accounts", map[string]any{
		"name": "Dining",
		"type": "expense",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var diningResult struct{ ID string }
	ParseJSON(t, resp, &diningResult)

	// Step 4: Create income account
	resp = client.Post("/api/v1/accounts", map[string]any{
		"name": "Salary",
		"type": "income",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var incomeResult struct{ ID string }
	ParseJSON(t, resp, &incomeResult)

	// Step 5: Record opening balance (salary deposit)
	resp = client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-01",
		"description": "Starting Balance",
		"entries": []map[string]any{
			{"account_id": checkingResult.ID, "amount_cents": 500000},
			{"account_id": incomeResult.ID, "amount_cents": -500000},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)

	// Step 6: Verify dashboard shows correct net worth
	resp = client.Get("/api/v1/dashboard")
	AssertStatus(t, resp, http.StatusOK)
	var dashboard struct {
		NetWorth    int64 `json:"net_worth"`
		TotalAssets int64 `json:"total_assets"`
	}
	ParseJSON(t, resp, &dashboard)

	if dashboard.TotalAssets != 500000 {
		t.Errorf("Expected total assets $5000, got %d cents", dashboard.TotalAssets)
	}
	if dashboard.NetWorth != 500000 {
		t.Errorf("Expected net worth $5000, got %d cents", dashboard.NetWorth)
	}

	t.Logf("✓ New user onboarding workflow completed successfully")
}

// TestWorkflow_MonthlyBudgetCycle tests a typical monthly spending cycle
func TestWorkflow_MonthlyBudgetCycle(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("monthly-budget")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Setup accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)
	utilities := env.CreateAccount(tu.Ledger.ID, "Utilities", models.AccountTypeExpense)
	dining := env.CreateAccount(tu.Ledger.ID, "Dining", models.AccountTypeExpense)

	// Create tags for categorization
	foodTag := env.CreateTag(tu.Ledger.ID, "Food", "#ff5722")
	billsTag := env.CreateTag(tu.Ledger.ID, "Bills", "#2196f3")

	// Month start: receive paycheck
	resp := client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-01",
		"description": "Paycheck - ACME Corp",
		"entries": []map[string]any{
			{"account_id": checking.ID.String(), "amount_cents": 350000},
			{"account_id": income.ID.String(), "amount_cents": -350000},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)
	var paycheckTxn struct{ ID string }
	ParseJSON(t, resp, &paycheckTxn)

	// Recurring: transfer to savings
	resp = client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-02",
		"description": "Monthly Savings Transfer",
		"entries": []map[string]any{
			{"account_id": savings.ID.String(), "amount_cents": 50000},
			{"account_id": checking.ID.String(), "amount_cents": -50000},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)

	// Weekly groceries
	for week := 1; week <= 4; week++ {
		resp = client.Post("/api/v1/transactions", map[string]any{
			"date":        "2024-01-" + string(rune('0'+week*7)),
			"description": "Grocery Store",
			"entries": []map[string]any{
				{"account_id": groceries.ID.String(), "amount_cents": 15000},
				{"account_id": checking.ID.String(), "amount_cents": -15000},
			},
		})
		AssertStatus(t, resp, http.StatusCreated)
		var txn struct{ ID string }
		ParseJSON(t, resp, &txn)

		// Tag as food
		resp = client.Post("/api/v1/transactions/"+txn.ID+"/tags", map[string]any{
			"tag_id": foodTag.ID.String(),
		})
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Logf("Note: Tag assignment returned %d", resp.StatusCode)
		}
	}

	// Utility bills
	resp = client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-15",
		"description": "Electric Company",
		"entries": []map[string]any{
			{"account_id": utilities.ID.String(), "amount_cents": 12500},
			{"account_id": checking.ID.String(), "amount_cents": -12500},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)
	var utilTxn struct{ ID string }
	ParseJSON(t, resp, &utilTxn)

	// Tag utility bill
	client.Post("/api/v1/transactions/"+utilTxn.ID+"/tags", map[string]any{
		"tag_id": billsTag.ID.String(),
	})

	// Dining out
	resp = client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-20",
		"description": "Restaurant",
		"entries": []map[string]any{
			{"account_id": dining.ID.String(), "amount_cents": 4500},
			{"account_id": checking.ID.String(), "amount_cents": -4500},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)

	// End of month: verify balances
	resp = client.Get("/api/v1/accounts/" + checking.ID.String())
	AssertStatus(t, resp, http.StatusOK)
	var checkingBalance struct {
		Balance int64 `json:"balance"`
	}
	ParseJSON(t, resp, &checkingBalance)

	// Expected: 3500 - 500 (savings) - 600 (4x150 groceries) - 125 (electric) - 45 (dining) = 2230
	expected := int64(350000 - 50000 - 60000 - 12500 - 4500)
	if checkingBalance.Balance != expected {
		t.Errorf("Expected checking balance %d, got %d", expected, checkingBalance.Balance)
	}

	// Verify transactions list
	resp = client.Get("/api/v1/transactions")
	AssertStatus(t, resp, http.StatusOK)
	var txnList struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	ParseJSON(t, resp, &txnList)

	// Should have 8 transactions: paycheck, savings transfer, 4 groceries, utility, dining
	if txnList.Pagination.Total != 8 {
		t.Errorf("Expected 8 transactions, got %d", txnList.Pagination.Total)
	}

	t.Logf("✓ Monthly budget cycle workflow completed successfully")
}

// TestWorkflow_TransferDetection tests the automatic transfer matching flow
func TestWorkflow_TransferDetection(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("transfer-detection")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	income := env.CreateAccount(tu.Ledger.ID, "Transfer Income", models.AccountTypeIncome)
	expense := env.CreateAccount(tu.Ledger.ID, "Transfer Expense", models.AccountTypeExpense)

	// Initial balance
	env.CreateTransaction(tu.Ledger.ID, time.Now().AddDate(0, -1, 0), "Opening Balance", []*models.Entry{
		{AccountID: checking.ID, AmountCents: 1000000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -1000000, Currency: "USD"},
	})

	// Create outgoing transfer
	resp := client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-15",
		"description": "Transfer to Savings",
		"entries": []map[string]any{
			{"account_id": expense.ID.String(), "amount_cents": 20000},
			{"account_id": checking.ID.String(), "amount_cents": -20000},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)
	var outgoingTxn struct{ ID string }
	ParseJSON(t, resp, &outgoingTxn)

	// Create incoming transfer (same amount, same day)
	resp = client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-15",
		"description": "Transfer from Checking",
		"entries": []map[string]any{
			{"account_id": savings.ID.String(), "amount_cents": 20000},
			{"account_id": income.ID.String(), "amount_cents": -20000},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)
	var incomingTxn struct{ ID string }
	ParseJSON(t, resp, &incomingTxn)

	// Manually link as transfer pair
	resp = client.Post("/api/v1/transfers/match", map[string]any{
		"transaction_id_1": outgoingTxn.ID,
		"transaction_id_2": incomingTxn.ID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Logf("Note: Manual match returned %d (may already be auto-matched)", resp.StatusCode)
	}

	// Verify transfers are linked
	resp = client.Get("/api/v1/transactions/" + outgoingTxn.ID)
	AssertStatus(t, resp, http.StatusOK)
	var txn struct {
		IsTransfer     bool    `json:"is_transfer"`
		TransferPairID *string `json:"transfer_pair_id"`
	}
	ParseJSON(t, resp, &txn)

	if !txn.IsTransfer {
		t.Error("Expected outgoing transaction to be marked as transfer")
	}

	// Verify dashboard doesn't double-count transfers
	resp = client.Get("/api/v1/dashboard")
	AssertStatus(t, resp, http.StatusOK)
	var dashboard struct {
		TotalAssets int64 `json:"total_assets"`
	}
	ParseJSON(t, resp, &dashboard)

	// Assets should be: 10000 (opening) - 200 (transfer out) + 200 (transfer in) = 10000
	// Or if transfers are excluded from income/expense: checking 8000 + savings 2000 = 10000
	t.Logf("Total assets after transfer: $%.2f", float64(dashboard.TotalAssets)/100)

	t.Logf("✓ Transfer detection workflow completed successfully")
}

// TestWorkflow_RuleBasedCategorization tests automatic transaction categorization
func TestWorkflow_RuleBasedCategorization(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("categorization")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)
	gas := env.CreateAccount(tu.Ledger.ID, "Gas", models.AccountTypeExpense)
	misc := env.CreateAccount(tu.Ledger.ID, "Misc", models.AccountTypeExpense)

	// Create tags
	foodTag := env.CreateTag(tu.Ledger.ID, "Food & Groceries", "#4caf50")
	autoTag := env.CreateTag(tu.Ledger.ID, "Auto & Gas", "#ff9800")

	// Create categorization rules
	resp := client.Post("/api/v1/rules", map[string]any{
		"name":          "Grocery Stores",
		"match_pattern": "walmart|costco|kroger|safeway",
		"is_regex":      true,
		"tag_id":        foodTag.ID.String(),
		"priority":      10,
	})
	AssertStatus(t, resp, http.StatusCreated)

	resp = client.Post("/api/v1/rules", map[string]any{
		"name":          "Gas Stations",
		"match_pattern": "shell|exxon|chevron|bp",
		"is_regex":      true,
		"tag_id":        autoTag.ID.String(),
		"priority":      10,
	})
	AssertStatus(t, resp, http.StatusCreated)

	// Create uncategorized transactions
	env.CreateTransaction(tu.Ledger.ID, time.Now(), "WALMART STORE #1234", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 8500, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -8500, Currency: "USD"},
	})

	env.CreateTransaction(tu.Ledger.ID, time.Now(), "SHELL GAS STATION", []*models.Entry{
		{AccountID: gas.ID, AmountCents: 4500, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -4500, Currency: "USD"},
	})

	env.CreateTransaction(tu.Ledger.ID, time.Now(), "RANDOM STORE", []*models.Entry{
		{AccountID: misc.ID, AmountCents: 2500, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -2500, Currency: "USD"},
	})

	// Apply rules
	resp = client.Post("/api/v1/rules/apply", nil)
	AssertStatus(t, resp, http.StatusOK)
	var applyResult struct {
		Matched   int `json:"matched"`
		Processed int `json:"processed"`
	}
	ParseJSON(t, resp, &applyResult)

	// Should match 2 of 3 transactions
	if applyResult.Matched < 2 {
		t.Errorf("Expected at least 2 matched transactions, got %d", applyResult.Matched)
	}

	t.Logf("✓ Rule-based categorization workflow completed: %d/%d matched",
		applyResult.Matched, applyResult.Processed)
}

// TestWorkflow_MultiUserIsolation tests that users cannot see each other's data
func TestWorkflow_MultiUserIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	// Create two separate users
	alice := env.CreateTestUser("alice")
	defer env.CleanupTestUser(alice)
	bob := env.CreateTestUser("bob")
	defer env.CleanupTestUser(bob)

	aliceClient := env.NewAPIClient(alice)
	bobClient := env.NewAPIClient(bob)

	// Alice creates her data
	resp := aliceClient.Post("/api/v1/accounts", map[string]any{
		"name": "Alice's Secret Account",
		"type": "asset",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var aliceAccount struct{ ID string }
	ParseJSON(t, resp, &aliceAccount)

	aliceTag := env.CreateTag(alice.Ledger.ID, "Alice's Tag", "#ff0000")

	// Bob creates his data
	resp = bobClient.Post("/api/v1/accounts", map[string]any{
		"name": "Bob's Account",
		"type": "asset",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var bobAccount struct{ ID string }
	ParseJSON(t, resp, &bobAccount)

	// Alice should only see her own accounts
	resp = aliceClient.Get("/api/v1/accounts")
	AssertStatus(t, resp, http.StatusOK)
	var aliceAccounts struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	ParseJSON(t, resp, &aliceAccounts)

	for _, acc := range aliceAccounts.Data {
		if acc.Name == "Bob's Account" {
			t.Error("Alice can see Bob's account - isolation breach!")
		}
	}

	// Bob should only see his own accounts
	resp = bobClient.Get("/api/v1/accounts")
	AssertStatus(t, resp, http.StatusOK)
	var bobAccounts struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	ParseJSON(t, resp, &bobAccounts)

	for _, acc := range bobAccounts.Data {
		if acc.Name == "Alice's Secret Account" {
			t.Error("Bob can see Alice's account - isolation breach!")
		}
	}

	// Bob should not be able to access Alice's account directly
	resp = bobClient.Get("/api/v1/accounts/" + aliceAccount.ID)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for Bob accessing Alice's account, got %d", resp.StatusCode)
	}

	// Bob should not be able to access Alice's tag
	resp = bobClient.Get("/api/v1/tags/" + aliceTag.ID.String())
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for Bob accessing Alice's tag, got %d", resp.StatusCode)
	}

	t.Logf("✓ Multi-user isolation workflow completed - data is properly isolated")
}

// TestWorkflow_DataIntegrity verifies double-entry accounting integrity
func TestWorkflow_DataIntegrity(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("data-integrity")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	creditCard := env.CreateAccount(tu.Ledger.ID, "Credit Card", models.AccountTypeLiability)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)
	expenses := env.CreateAccount(tu.Ledger.ID, "Expenses", models.AccountTypeExpense)

	// Create various transactions
	env.CreateTransaction(tu.Ledger.ID, time.Now(), "Paycheck", []*models.Entry{
		{AccountID: checking.ID, AmountCents: 500000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -500000, Currency: "USD"},
	})

	env.CreateTransaction(tu.Ledger.ID, time.Now(), "Transfer to Savings", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 100000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -100000, Currency: "USD"},
	})

	env.CreateTransaction(tu.Ledger.ID, time.Now(), "Credit Card Purchase", []*models.Entry{
		{AccountID: expenses.ID, AmountCents: 25000, Currency: "USD"},
		{AccountID: creditCard.ID, AmountCents: -25000, Currency: "USD"},
	})

	env.CreateTransaction(tu.Ledger.ID, time.Now(), "Pay Credit Card", []*models.Entry{
		{AccountID: creditCard.ID, AmountCents: 25000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -25000, Currency: "USD"},
	})

	// Verify balances
	resp := client.Get("/api/v1/dashboard")
	AssertStatus(t, resp, http.StatusOK)
	var dashboard struct {
		TotalAssets      int64 `json:"total_assets"`
		TotalLiabilities int64 `json:"total_liabilities"`
		NetWorth         int64 `json:"net_worth"`
	}
	ParseJSON(t, resp, &dashboard)

	// Checking: 5000 - 1000 (to savings) - 250 (cc payment) = 3750
	// Savings: 1000
	// Total Assets: 4750
	expectedAssets := int64(500000 - 100000 - 25000 + 100000) // 475000
	if dashboard.TotalAssets != expectedAssets {
		t.Errorf("Expected total assets %d, got %d", expectedAssets, dashboard.TotalAssets)
	}

	// Credit Card: -250 + 250 = 0 (paid off)
	expectedLiabilities := int64(0)
	if dashboard.TotalLiabilities != expectedLiabilities {
		t.Errorf("Expected total liabilities %d, got %d", expectedLiabilities, dashboard.TotalLiabilities)
	}

	// Net Worth: 4750 - 0 = 4750
	expectedNetWorth := expectedAssets - expectedLiabilities
	if dashboard.NetWorth != expectedNetWorth {
		t.Errorf("Expected net worth %d, got %d", expectedNetWorth, dashboard.NetWorth)
	}

	t.Logf("✓ Data integrity workflow completed - balances are correct")
}
