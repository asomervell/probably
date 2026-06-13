package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/asomervell/probably/internal/models"
)

// TestIntegration_FullWorkflow tests a complete workflow:
// Create accounts → Add transactions → Categorize with tags → Verify balances
func TestIntegration_FullWorkflow(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("integration-workflow")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Step 1: Create accounts
	t.Log("Step 1: Creating accounts...")

	// Checking account
	resp := client.Post("/api/v1/accounts", map[string]any{
		"name":             "Main Checking",
		"type":             "asset",
		"institution_name": "Big Bank",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var checkingResult struct{ ID string }
	ParseJSON(t, resp, &checkingResult)
	checkingID := checkingResult.ID

	// Savings account
	resp = client.Post("/api/v1/accounts", map[string]any{
		"name":             "Emergency Fund",
		"type":             "asset",
		"institution_name": "Big Bank",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var savingsResult struct{ ID string }
	ParseJSON(t, resp, &savingsResult)
	savingsID := savingsResult.ID

	// Credit card (liability)
	resp = client.Post("/api/v1/accounts", map[string]any{
		"name": "Rewards Card",
		"type": "liability",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var creditResult struct{ ID string }
	ParseJSON(t, resp, &creditResult)
	creditID := creditResult.ID

	// Expense account for groceries
	resp = client.Post("/api/v1/accounts", map[string]any{
		"name": "Groceries",
		"type": "expense",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var groceriesExpResult struct{ ID string }
	ParseJSON(t, resp, &groceriesExpResult)
	groceriesExpID := groceriesExpResult.ID

	// Income account
	resp = client.Post("/api/v1/accounts", map[string]any{
		"name": "Salary",
		"type": "income",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var salaryResult struct{ ID string }
	ParseJSON(t, resp, &salaryResult)
	salaryID := salaryResult.ID

	// Step 2: Create tags for categorization
	t.Log("Step 2: Creating tags...")

	resp = client.Post("/api/v1/tags", map[string]any{
		"name":  "Food",
		"color": "#ff5722",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var foodTagResult struct{ ID string }
	ParseJSON(t, resp, &foodTagResult)
	foodTagID := foodTagResult.ID

	resp = client.Post("/api/v1/tags", map[string]any{
		"name":  "Income",
		"color": "#4caf50",
	})
	AssertStatus(t, resp, http.StatusCreated)
	var incomeTagResult struct{ ID string }
	ParseJSON(t, resp, &incomeTagResult)
	incomeTagID := incomeTagResult.ID

	// Step 3: Create transactions
	t.Log("Step 3: Creating transactions...")

	// Paycheck deposit ($5000)
	resp = client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-15",
		"description": "PAYROLL - ACME CORP",
		"entries": []map[string]any{
			{"account_id": checkingID, "amount_cents": 500000},
			{"account_id": salaryID, "amount_cents": -500000},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)
	var paycheckResult struct{ ID string }
	ParseJSON(t, resp, &paycheckResult)
	paycheckID := paycheckResult.ID

	// Grocery purchase on credit card ($150)
	resp = client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-16",
		"description": "WHOLE FOODS MARKET",
		"entries": []map[string]any{
			{"account_id": groceriesExpID, "amount_cents": 15000},
			{"account_id": creditID, "amount_cents": -15000}, // Credit to liability (increases debt)
		},
	})
	AssertStatus(t, resp, http.StatusCreated)
	var groceryResult struct{ ID string }
	ParseJSON(t, resp, &groceryResult)
	groceryTxnID := groceryResult.ID

	// Transfer to savings ($1000)
	resp = client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-17",
		"description": "TRANSFER TO SAVINGS",
		"is_transfer": true,
		"entries": []map[string]any{
			{"account_id": savingsID, "amount_cents": 100000},
			{"account_id": checkingID, "amount_cents": -100000},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)

	// Credit card payment ($150)
	resp = client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-20",
		"description": "PAYMENT - REWARDS CARD",
		"is_transfer": true,
		"entries": []map[string]any{
			{"account_id": creditID, "amount_cents": 15000}, // Debit to liability (decreases debt)
			{"account_id": checkingID, "amount_cents": -15000},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)

	// Step 4: Add tags to transactions
	t.Log("Step 4: Categorizing transactions...")

	resp = client.Post("/api/v1/transactions/"+paycheckID+"/tags", map[string]any{
		"tag_id": incomeTagID,
	})
	AssertStatus(t, resp, http.StatusOK)

	resp = client.Post("/api/v1/transactions/"+groceryTxnID+"/tags", map[string]any{
		"tag_id": foodTagID,
	})
	AssertStatus(t, resp, http.StatusOK)

	// Step 5: Verify account balances
	t.Log("Step 5: Verifying balances...")

	// Checking: +5000 (paycheck) - 1000 (to savings) - 150 (cc payment) = 3850
	resp = client.Get("/api/v1/accounts/" + checkingID)
	AssertStatus(t, resp, http.StatusOK)
	var checkingBalance struct{ Balance int64 }
	ParseJSON(t, resp, &checkingBalance)
	expectedChecking := int64(385000)
	if checkingBalance.Balance != expectedChecking {
		t.Errorf("Checking balance: expected %d, got %d", expectedChecking, checkingBalance.Balance)
	}

	// Savings: +1000 (transfer)
	resp = client.Get("/api/v1/accounts/" + savingsID)
	AssertStatus(t, resp, http.StatusOK)
	var savingsBalance struct{ Balance int64 }
	ParseJSON(t, resp, &savingsBalance)
	expectedSavings := int64(100000)
	if savingsBalance.Balance != expectedSavings {
		t.Errorf("Savings balance: expected %d, got %d", expectedSavings, savingsBalance.Balance)
	}

	// Credit card: -150 (grocery) + 150 (payment) = 0
	resp = client.Get("/api/v1/accounts/" + creditID)
	AssertStatus(t, resp, http.StatusOK)
	var creditBalance struct{ Balance int64 }
	ParseJSON(t, resp, &creditBalance)
	expectedCredit := int64(0)
	if creditBalance.Balance != expectedCredit {
		t.Errorf("Credit card balance: expected %d, got %d", expectedCredit, creditBalance.Balance)
	}

	// Step 6: Verify dashboard
	t.Log("Step 6: Checking dashboard...")

	resp = client.Get("/api/v1/dashboard")
	AssertStatus(t, resp, http.StatusOK)
	var dashboard struct {
		NetWorth         int64 `json:"net_worth"`
		TotalAssets      int64 `json:"total_assets"`
		TotalLiabilities int64 `json:"total_liabilities"`
	}
	ParseJSON(t, resp, &dashboard)

	// Net worth = Assets (3850 + 1000) - Liabilities (0) = 4850
	expectedNetWorth := int64(485000)
	if dashboard.NetWorth != expectedNetWorth {
		t.Errorf("Net worth: expected %d, got %d", expectedNetWorth, dashboard.NetWorth)
	}

	// Step 7: Verify tags can be retrieved
	t.Log("Step 7: Verifying tags...")

	resp = client.Get("/api/v1/tags/" + foodTagID)
	AssertStatus(t, resp, http.StatusOK)
	// Note: Tag usage count verification is done in TestAPITags_GetWithUsageCount

	t.Log("Integration test complete!")
}

// TestIntegration_NetWorthCalculation tests accurate net worth calculation
func TestIntegration_NetWorthCalculation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("integration-networth")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	creditCard := env.CreateAccount(tu.Ledger.ID, "Credit Card", models.AccountTypeLiability)
	mortgage := env.CreateAccount(tu.Ledger.ID, "Mortgage", models.AccountTypeLiability)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)
	expense := env.CreateAccount(tu.Ledger.ID, "Expenses", models.AccountTypeExpense)

	// Add income ($10,000 to checking)
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "Paycheck", []*models.Entry{
		{AccountID: checking.ID, AmountCents: 1000000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -1000000, Currency: "USD"},
	})

	// Transfer to savings ($2,000)
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "Savings Transfer", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 200000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -200000, Currency: "USD"},
	})

	// Credit card spending ($500)
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "Shopping", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 50000, Currency: "USD"},
		{AccountID: creditCard.ID, AmountCents: -50000, Currency: "USD"},
	})

	// Mortgage balance (starting debt of $250,000)
	// In double-entry, we'd have equity offset this
	equity := env.CreateAccount(tu.Ledger.ID, "Opening Equity", models.AccountTypeEquity)
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "Initial Mortgage", []*models.Entry{
		{AccountID: equity.ID, AmountCents: 25000000, Currency: "USD"},
		{AccountID: mortgage.ID, AmountCents: -25000000, Currency: "USD"},
	})

	// Get dashboard
	resp := client.Get("/api/v1/dashboard")
	AssertStatus(t, resp, http.StatusOK)

	var dashboard struct {
		NetWorth         int64 `json:"net_worth"`
		TotalAssets      int64 `json:"total_assets"`
		TotalLiabilities int64 `json:"total_liabilities"`
	}
	ParseJSON(t, resp, &dashboard)

	// Assets: Checking (10000 - 2000 = 8000) + Savings (2000) = 10000
	expectedAssets := int64(1000000)
	if dashboard.TotalAssets != expectedAssets {
		t.Errorf("Total assets: expected %d, got %d", expectedAssets, dashboard.TotalAssets)
	}

	// Liabilities: Credit Card (500) + Mortgage (250000) = 250500
	expectedLiabilities := int64(25050000)
	if dashboard.TotalLiabilities != expectedLiabilities {
		t.Errorf("Total liabilities: expected %d, got %d", expectedLiabilities, dashboard.TotalLiabilities)
	}

	// Net worth: 10000 - 250500 = -240500
	expectedNetWorth := int64(1000000 - 25050000)
	if dashboard.NetWorth != expectedNetWorth {
		t.Errorf("Net worth: expected %d, got %d", expectedNetWorth, dashboard.NetWorth)
	}
}

// TestIntegration_LiabilitySignConventions tests that liability accounts
// follow proper double-entry conventions
func TestIntegration_LiabilitySignConventions(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("integration-liability")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	creditCard := env.CreateAccount(tu.Ledger.ID, "Credit Card", models.AccountTypeLiability)
	expense := env.CreateAccount(tu.Ledger.ID, "Expenses", models.AccountTypeExpense)

	// Make a purchase on credit card ($100)
	// This INCREASES the credit card debt
	// In double-entry: Debit Expense (positive), Credit CC (negative)
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "Purchase", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 10000, Currency: "USD"},
		{AccountID: creditCard.ID, AmountCents: -10000, Currency: "USD"},
	})

	// Check credit card balance
	resp := client.Get("/api/v1/accounts/" + creditCard.ID.String())
	AssertStatus(t, resp, http.StatusOK)
	var result struct{ Balance int64 }
	ParseJSON(t, resp, &result)

	// Balance should be -10000 (we owe $100)
	if result.Balance != -10000 {
		t.Errorf("After purchase, CC balance should be -10000, got %d", result.Balance)
	}

	// Pay off the credit card ($100)
	// This DECREASES the credit card debt
	// In double-entry: Debit CC (positive), Credit Checking (negative)
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "CC Payment", []*models.Entry{
		{AccountID: creditCard.ID, AmountCents: 10000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -10000, Currency: "USD"},
	})

	// Check credit card balance again
	resp = client.Get("/api/v1/accounts/" + creditCard.ID.String())
	AssertStatus(t, resp, http.StatusOK)
	ParseJSON(t, resp, &result)

	// Balance should now be 0
	if result.Balance != 0 {
		t.Errorf("After payment, CC balance should be 0, got %d", result.Balance)
	}
}

// TestIntegration_TransferDetection tests automatic transfer detection workflow
func TestIntegration_TransferDetection(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("integration-transfer")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create two bank accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	income := env.CreateAccount(tu.Ledger.ID, "Uncategorized Income", models.AccountTypeIncome)
	expense := env.CreateAccount(tu.Ledger.ID, "Uncategorized Expenses", models.AccountTypeExpense)

	// Create two transactions that look like a transfer
	// (same amount, same date, opposite accounts)
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Outgoing from checking
	txn1 := env.CreateTransaction(tu.Ledger.ID, date, "TRANSFER TO SAVINGS", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 50000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -50000, Currency: "USD"},
	})

	// Incoming to savings
	txn2 := env.CreateTransaction(tu.Ledger.ID, date, "TRANSFER FROM CHECKING", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 50000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -50000, Currency: "USD"},
	})

	// Manually match them as a transfer
	resp := client.Post("/api/v1/transfers/match", map[string]any{
		"transaction_id_1": txn1.ID.String(),
		"transaction_id_2": txn2.ID.String(),
	})
	AssertStatus(t, resp, http.StatusOK)

	// Verify both are now marked as transfers
	resp = client.Get("/api/v1/transactions/" + txn1.ID.String())
	AssertStatus(t, resp, http.StatusOK)
	var txnResult struct {
		IsTransfer     bool   `json:"is_transfer"`
		TransferPairID string `json:"transfer_pair_id"`
	}
	ParseJSON(t, resp, &txnResult)

	if !txnResult.IsTransfer {
		t.Error("Transaction 1 should be marked as transfer")
	}
	if txnResult.TransferPairID != txn2.ID.String() {
		t.Errorf("Transaction 1 should be paired with %s, got %s", txn2.ID.String(), txnResult.TransferPairID)
	}

	// Filter transactions to exclude transfers
	resp = client.Get("/api/v1/transactions?is_transfer=false")
	AssertStatus(t, resp, http.StatusOK)
	var listResult struct {
		Pagination struct{ Total int } `json:"pagination"`
	}
	ParseJSON(t, resp, &listResult)

	if listResult.Pagination.Total != 0 {
		t.Errorf("Expected 0 non-transfer transactions, got %d", listResult.Pagination.Total)
	}
}

// TestIntegration_DoubleEntryIntegrity ensures all transactions maintain
// double-entry balance (sum of entries = 0)
func TestIntegration_DoubleEntryIntegrity(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("integration-double-entry")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create various accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)
	utilities := env.CreateAccount(tu.Ledger.ID, "Utilities", models.AccountTypeExpense)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)

	// Create a multi-entry transaction (split expense)
	// $100 total: $70 groceries, $30 utilities
	resp := client.Post("/api/v1/transactions", map[string]any{
		"date":        "2024-01-15",
		"description": "Walmart - Split Purchase",
		"entries": []map[string]any{
			{"account_id": groceries.ID.String(), "amount_cents": 7000},
			{"account_id": utilities.ID.String(), "amount_cents": 3000},
			{"account_id": checking.ID.String(), "amount_cents": -10000},
		},
	})
	AssertStatus(t, resp, http.StatusCreated)

	// Create income transaction
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "Paycheck", []*models.Entry{
		{AccountID: checking.ID, AmountCents: 500000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -500000, Currency: "USD"},
	})

	// Verify checking balance
	resp = client.Get("/api/v1/accounts/" + checking.ID.String())
	AssertStatus(t, resp, http.StatusOK)
	var result struct{ Balance int64 }
	ParseJSON(t, resp, &result)

	// Should be: +5000 - 100 = 4900
	expected := int64(490000)
	if result.Balance != expected {
		t.Errorf("Checking balance: expected %d, got %d", expected, result.Balance)
	}

	// Verify groceries expense
	resp = client.Get("/api/v1/accounts/" + groceries.ID.String())
	AssertStatus(t, resp, http.StatusOK)
	ParseJSON(t, resp, &result)
	if result.Balance != 7000 {
		t.Errorf("Groceries balance: expected 7000, got %d", result.Balance)
	}

	// Verify utilities expense
	resp = client.Get("/api/v1/accounts/" + utilities.ID.String())
	AssertStatus(t, resp, http.StatusOK)
	ParseJSON(t, resp, &result)
	if result.Balance != 3000 {
		t.Errorf("Utilities balance: expected 3000, got %d", result.Balance)
	}
}

// TestIntegration_RuleBasedCategorization tests the rule application workflow
func TestIntegration_RuleBasedCategorization(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("integration-rules")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	uncategorized := env.CreateAccount(tu.Ledger.ID, "Uncategorized", models.AccountTypeExpense)

	// Create tags
	groceriesTag := env.CreateTag(tu.Ledger.ID, "Groceries", "#ff5722")
	coffeeTag := env.CreateTag(tu.Ledger.ID, "Coffee", "#795548")

	// Create rules
	client.Post("/api/v1/rules", map[string]any{
		"name":          "Grocery Stores",
		"match_pattern": "walmart|costco|whole foods|trader joe",
		"is_regex":      true,
		"tag_id":        groceriesTag.ID.String(),
		"priority":      10,
	})

	client.Post("/api/v1/rules", map[string]any{
		"name":          "Coffee Shops",
		"match_pattern": "starbucks|dunkin|coffee",
		"is_regex":      true,
		"tag_id":        coffeeTag.ID.String(),
		"priority":      5,
	})

	// Create uncategorized transactions
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "WALMART SUPERCENTER", []*models.Entry{
		{AccountID: uncategorized.ID, AmountCents: 15000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -15000, Currency: "USD"},
	})
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "STARBUCKS #1234", []*models.Entry{
		{AccountID: uncategorized.ID, AmountCents: 550, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -550, Currency: "USD"},
	})
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "GAS STATION", []*models.Entry{
		{AccountID: uncategorized.ID, AmountCents: 4500, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -4500, Currency: "USD"},
	})

	// Apply rules
	resp := client.Post("/api/v1/rules/apply", nil)
	AssertStatus(t, resp, http.StatusOK)

	var applyResult struct {
		Matched   int `json:"matched"`
		Processed int `json:"processed"`
	}
	ParseJSON(t, resp, &applyResult)

	// Should match 2 transactions (walmart, starbucks)
	if applyResult.Matched != 2 {
		t.Errorf("Expected 2 matched transactions, got %d", applyResult.Matched)
	}

	// Verify tags exist and can be retrieved
	resp = client.Get("/api/v1/tags/" + groceriesTag.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	resp = client.Get("/api/v1/tags/" + coffeeTag.ID.String())
	AssertStatus(t, resp, http.StatusOK)
	// Note: Tag usage count verification is handled separately in unit tests
}
