package models_test

import (
	"context"
	"testing"
	"time"

	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TestEnv struct {
	T            *testing.T
	Pool         *pgxpool.Pool
	DB           *db.DB
	Users        *models.UserStore
	Ledgers      *models.LedgerStore
	Accounts     *models.AccountStore
	Transactions *models.TransactionStore
	Tags         *models.TagStore
	Rules        *models.RuleStore
}

func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	database := testutil.ConnectTestDB(t)
	return &TestEnv{
		T:            t,
		Pool:         database.Pool,
		DB:           database,
		Users:        models.NewUserStore(database.Pool),
		Ledgers:      models.NewLedgerStore(database.Pool),
		Accounts:     models.NewAccountStore(database.Pool),
		Transactions: models.NewTransactionStore(database.Pool),
		Tags:         models.NewTagStore(database.Pool),
		Rules:        models.NewRuleStore(database.Pool),
	}
}

func (env *TestEnv) Cleanup() { env.DB.Close() }

type TestUser = testutil.TestUser

func (env *TestEnv) CreateTestUser(suffix string) *TestUser {
	return testutil.CreateUserAndLedger(env.T, env.Users, env.Ledgers, suffix)
}

func (env *TestEnv) CleanupUser(tu *TestUser) {
	_, _ = env.Pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", tu.User.ID)
}

func (env *TestEnv) CreateAccount(ledgerID uuid.UUID, name string, accType models.AccountType) *models.Account {
	env.T.Helper()
	acc := &models.Account{
		ID:       uuid.New(),
		LedgerID: ledgerID,
		Name:     name,
		Type:     accType,
		IsActive: true,
	}
	if err := env.Accounts.Create(context.Background(), acc); err != nil {
		env.T.Fatalf("Failed to create account: %v", err)
	}
	return acc
}

func (env *TestEnv) CreateTransaction(ledgerID uuid.UUID, date time.Time, description string, entries []*models.Entry) *models.Transaction {
	env.T.Helper()
	txn := &models.Transaction{
		ID:          uuid.New(),
		LedgerID:    ledgerID,
		Date:        date,
		Description: description,
	}
	if err := env.Transactions.CreateWithEntries(context.Background(), txn, entries); err != nil {
		env.T.Fatalf("Failed to create transaction: %v", err)
	}
	return txn
}

func (env *TestEnv) CreateTag(ledgerID uuid.UUID, name, color string) *models.Tag {
	env.T.Helper()
	tag := &models.Tag{
		ID:       uuid.New(),
		LedgerID: ledgerID,
		Name:     name,
		Color:    color,
	}
	if err := env.Tags.Create(context.Background(), tag); err != nil {
		env.T.Fatalf("Failed to create tag: %v", err)
	}
	return tag
}

// =============================================
// AccountStore Tests
// =============================================

func TestAccountStore_Create(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("account-create")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	acc := &models.Account{
		LedgerID:        tu.Ledger.ID,
		Name:            "Checking Account",
		Type:            models.AccountTypeAsset,
		InstitutionName: "Big Bank",
		IsActive:        true,
	}

	err := env.Accounts.Create(ctx, acc)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if acc.ID == uuid.Nil {
		t.Error("Expected ID to be set")
	}

	// Retrieve and verify
	retrieved, err := env.Accounts.GetByID(ctx, acc.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Name != "Checking Account" {
		t.Errorf("Name: expected 'Checking Account', got '%s'", retrieved.Name)
	}
	if retrieved.Type != models.AccountTypeAsset {
		t.Errorf("Type: expected asset, got %s", retrieved.Type)
	}
}

func TestAccountStore_GetByLedgerID(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("account-list")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	// Create multiple accounts
	env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	env.CreateAccount(tu.Ledger.ID, "Credit Card", models.AccountTypeLiability)

	accounts, err := env.Accounts.GetByLedgerID(ctx, tu.Ledger.ID)
	if err != nil {
		t.Fatalf("GetByLedgerID failed: %v", err)
	}

	if len(accounts) != 3 {
		t.Errorf("Expected 3 accounts, got %d", len(accounts))
	}
}

func TestAccountStore_GetWithBalances(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("account-balances")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)

	// Add a deposit
	env.CreateTransaction(tu.Ledger.ID, time.Now(), "Deposit", []*models.Entry{
		{AccountID: checking.ID, AmountCents: 50000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -50000, Currency: "USD"},
	})

	accounts, err := env.Accounts.GetWithBalances(ctx, tu.Ledger.ID)
	if err != nil {
		t.Fatalf("GetWithBalances failed: %v", err)
	}

	// Find checking and verify balance
	var checkingBalance int64
	for _, acc := range accounts {
		if acc.ID == checking.ID {
			checkingBalance = acc.Balance
			break
		}
	}

	if checkingBalance != 50000 {
		t.Errorf("Checking balance: expected 50000, got %d", checkingBalance)
	}
}

func TestAccountStore_Update(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("account-update")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	acc := env.CreateAccount(tu.Ledger.ID, "Original Name", models.AccountTypeAsset)

	// Update the account
	acc.Name = "Updated Name"
	acc.InstitutionName = "New Bank"

	err := env.Accounts.Update(ctx, acc)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify
	retrieved, _ := env.Accounts.GetByID(ctx, acc.ID)
	if retrieved.Name != "Updated Name" {
		t.Errorf("Name not updated: got '%s'", retrieved.Name)
	}
	if retrieved.InstitutionName != "New Bank" {
		t.Errorf("InstitutionName not updated: got '%s'", retrieved.InstitutionName)
	}
}

func TestAccountStore_Delete(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("account-delete")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	acc := env.CreateAccount(tu.Ledger.ID, "To Delete", models.AccountTypeAsset)

	err := env.Accounts.Delete(ctx, acc.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, err = env.Accounts.GetByID(ctx, acc.ID)
	if err == nil {
		t.Error("Expected error when getting deleted account")
	}
}

// =============================================
// TransactionStore Tests
// =============================================

func TestTransactionStore_CreateWithEntries(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-create")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	txn := &models.Transaction{
		LedgerID:    tu.Ledger.ID,
		Date:        time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Description: "Test Transaction",
	}
	entries := []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	}

	err := env.Transactions.CreateWithEntries(ctx, txn, entries)
	if err != nil {
		t.Fatalf("CreateWithEntries failed: %v", err)
	}

	if txn.ID == uuid.Nil {
		t.Error("Expected transaction ID to be set")
	}

	// Retrieve and verify entries
	retrieved, _ := env.Transactions.GetByID(ctx, txn.ID)
	if err := env.Transactions.LoadEntries(ctx, retrieved); err != nil {
		t.Fatalf("LoadEntries failed: %v", err)
	}

	if len(retrieved.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(retrieved.Entries))
	}
}

func TestTransactionStore_List_WithFilters(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-list-filters")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)
	gas := env.CreateAccount(tu.Ledger.ID, "Gas", models.AccountTypeExpense)

	// Create transactions on different dates
	env.CreateTransaction(tu.Ledger.ID, time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), "Grocery Store", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})
	env.CreateTransaction(tu.Ledger.ID, time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC), "Gas Station", []*models.Entry{
		{AccountID: gas.ID, AmountCents: 4000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -4000, Currency: "USD"},
	})
	env.CreateTransaction(tu.Ledger.ID, time.Date(2024, 2, 5, 0, 0, 0, 0, time.UTC), "Another Grocery", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 6000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -6000, Currency: "USD"},
	})

	// Test date filter
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
	_, total, err := env.Transactions.List(ctx, models.TransactionFilter{
		LedgerID:  tu.Ledger.ID,
		StartDate: &startDate,
		EndDate:   &endDate,
	})
	if err != nil {
		t.Fatalf("List with date filter failed: %v", err)
	}

	if total != 2 {
		t.Errorf("Expected 2 transactions in January, got %d", total)
	}

	// Test search filter
	_, total, _ = env.Transactions.List(ctx, models.TransactionFilter{
		LedgerID: tu.Ledger.ID,
		Search:   "Grocery",
	})

	if total != 2 {
		t.Errorf("Expected 2 transactions matching 'Grocery', got %d", total)
	}

	// Test account filter
	_, total, _ = env.Transactions.List(ctx, models.TransactionFilter{
		LedgerID:  tu.Ledger.ID,
		AccountID: &gas.ID,
	})

	if total != 1 {
		t.Errorf("Expected 1 transaction for gas account, got %d", total)
	}
}

func TestTransactionStore_List_Pagination(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-pagination")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	expenses := env.CreateAccount(tu.Ledger.ID, "Expenses", models.AccountTypeExpense)

	// Create 15 transactions
	for i := 0; i < 15; i++ {
		env.CreateTransaction(tu.Ledger.ID, time.Now().AddDate(0, 0, -i), "Transaction "+string(rune('A'+i)), []*models.Entry{
			{AccountID: expenses.ID, AmountCents: int64((i + 1) * 100), Currency: "USD"},
			{AccountID: checking.ID, AmountCents: int64(-(i + 1) * 100), Currency: "USD"},
		})
	}

	// Test pagination
	txns, total, _ := env.Transactions.List(ctx, models.TransactionFilter{
		LedgerID: tu.Ledger.ID,
		Limit:    10,
		Offset:   0,
	})

	if total != 15 {
		t.Errorf("Expected total 15, got %d", total)
	}
	if len(txns) != 10 {
		t.Errorf("Expected 10 transactions on page 1, got %d", len(txns))
	}

	// Get page 2
	txns, _, _ = env.Transactions.List(ctx, models.TransactionFilter{
		LedgerID: tu.Ledger.ID,
		Limit:    10,
		Offset:   10,
	})

	if len(txns) != 5 {
		t.Errorf("Expected 5 transactions on page 2, got %d", len(txns))
	}
}

func TestTransactionStore_SetTransferPair(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-transfer-pair")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)
	expense := env.CreateAccount(tu.Ledger.ID, "Expense", models.AccountTypeExpense)

	// Create two transactions that should be matched
	txn1 := env.CreateTransaction(tu.Ledger.ID, time.Now(), "Transfer Out", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 10000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -10000, Currency: "USD"},
	})

	txn2 := env.CreateTransaction(tu.Ledger.ID, time.Now(), "Transfer In", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 10000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -10000, Currency: "USD"},
	})

	// Link them as transfer pair
	err := env.Transactions.SetTransferPair(ctx, txn1.ID, txn2.ID)
	if err != nil {
		t.Fatalf("SetTransferPair failed: %v", err)
	}

	// Verify
	retrieved1, _ := env.Transactions.GetByID(ctx, txn1.ID)
	retrieved2, _ := env.Transactions.GetByID(ctx, txn2.ID)

	if !retrieved1.IsTransfer {
		t.Error("Transaction 1 should be marked as transfer")
	}
	if !retrieved2.IsTransfer {
		t.Error("Transaction 2 should be marked as transfer")
	}
	if retrieved1.TransferPairID == nil || *retrieved1.TransferPairID != txn2.ID {
		t.Error("Transaction 1 should be paired with transaction 2")
	}
	if retrieved2.TransferPairID == nil || *retrieved2.TransferPairID != txn1.ID {
		t.Error("Transaction 2 should be paired with transaction 1")
	}
}

func TestTransactionStore_UnlinkTransferPair(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("txn-unlink-transfer")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)
	expense := env.CreateAccount(tu.Ledger.ID, "Expense", models.AccountTypeExpense)

	txn1 := env.CreateTransaction(tu.Ledger.ID, time.Now(), "Transfer Out", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 10000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -10000, Currency: "USD"},
	})

	txn2 := env.CreateTransaction(tu.Ledger.ID, time.Now(), "Transfer In", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 10000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -10000, Currency: "USD"},
	})

	// Link then unlink
	_ = env.Transactions.SetTransferPair(ctx, txn1.ID, txn2.ID)
	err := env.Transactions.UnlinkTransferPair(ctx, txn1.ID)
	if err != nil {
		t.Fatalf("UnlinkTransferPair failed: %v", err)
	}

	// Verify
	retrieved1, _ := env.Transactions.GetByID(ctx, txn1.ID)
	retrieved2, _ := env.Transactions.GetByID(ctx, txn2.ID)

	if retrieved1.IsTransfer {
		t.Error("Transaction 1 should not be marked as transfer")
	}
	if retrieved2.IsTransfer {
		t.Error("Transaction 2 should not be marked as transfer")
	}
}

// =============================================
// TagStore Tests
// =============================================

func TestTagStore_Create(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tag-create")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	tag := &models.Tag{
		LedgerID: tu.Ledger.ID,
		Name:     "Groceries",
		Color:    "#ff5722",
	}

	err := env.Tags.Create(ctx, tag)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if tag.ID == uuid.Nil {
		t.Error("Expected ID to be set")
	}

	// Retrieve and verify
	retrieved, err := env.Tags.GetByID(ctx, tag.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Name != "Groceries" {
		t.Errorf("Name: expected 'Groceries', got '%s'", retrieved.Name)
	}
	if retrieved.Color != "#ff5722" {
		t.Errorf("Color: expected '#ff5722', got '%s'", retrieved.Color)
	}
}

func TestTagStore_CreateHierarchy(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tag-hierarchy")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	// Create parent tag
	parent := env.CreateTag(tu.Ledger.ID, "Food & Drink", "#ff5722")

	// Create child tag
	child := &models.Tag{
		LedgerID: tu.Ledger.ID,
		ParentID: &parent.ID,
		Name:     "Groceries",
		Color:    "#4caf50",
	}
	err := env.Tags.Create(ctx, child)
	if err != nil {
		t.Fatalf("Create child tag failed: %v", err)
	}

	// Retrieve child and verify parent
	retrieved, _ := env.Tags.GetByID(ctx, child.ID)
	if retrieved.ParentID == nil || *retrieved.ParentID != parent.ID {
		t.Error("Child tag should have parent ID set")
	}

	// Get tags with hierarchy
	tags, _ := env.Tags.GetByLedgerID(ctx, tu.Ledger.ID)
	if len(tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(tags))
	}
}

func TestTagStore_AddTagToTransaction(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tag-txn")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)
	tag := env.CreateTag(tu.Ledger.ID, "Food", "#ff5722")

	txn := env.CreateTransaction(tu.Ledger.ID, time.Now(), "Grocery Store", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})

	// Add tag to transaction
	err := env.Tags.AddTagToTransaction(ctx, txn.ID, tag.ID)
	if err != nil {
		t.Fatalf("AddTagToTransaction failed: %v", err)
	}

	// Verify
	if err := env.Transactions.LoadTags(ctx, txn); err != nil {
		t.Fatalf("LoadTags failed: %v", err)
	}

	if len(txn.Tags) != 1 {
		t.Errorf("Expected 1 tag, got %d", len(txn.Tags))
	}
	if txn.Tags[0].Name != "Food" {
		t.Errorf("Expected tag 'Food', got '%s'", txn.Tags[0].Name)
	}
}

func TestTagStore_RemoveTagFromTransaction(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tag-remove")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)
	tag := env.CreateTag(tu.Ledger.ID, "Food", "#ff5722")

	txn := env.CreateTransaction(tu.Ledger.ID, time.Now(), "Grocery Store", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})

	// Add then remove
	_ = env.Tags.AddTagToTransaction(ctx, txn.ID, tag.ID)
	err := env.Tags.RemoveTagFromTransaction(ctx, txn.ID, tag.ID)
	if err != nil {
		t.Fatalf("RemoveTagFromTransaction failed: %v", err)
	}

	// Verify
	_ = env.Transactions.LoadTags(ctx, txn)
	if len(txn.Tags) != 0 {
		t.Errorf("Expected 0 tags, got %d", len(txn.Tags))
	}
}

// =============================================
// RuleStore Tests
// =============================================

func TestRuleStore_Create(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rule-create")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	tag := env.CreateTag(tu.Ledger.ID, "Groceries", "#ff5722")

	rule := &models.CategorizationRule{
		LedgerID:     tu.Ledger.ID,
		Name:         "Grocery Stores",
		MatchPattern: "walmart|costco|trader joe",
		IsRegex:      true,
		TagID:        tag.ID,
		Priority:     10,
		IsActive:     true,
	}

	err := env.Rules.Create(ctx, rule)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if rule.ID == uuid.Nil {
		t.Error("Expected ID to be set")
	}

	// Retrieve and verify
	retrieved, err := env.Rules.GetByID(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Name != "Grocery Stores" {
		t.Errorf("Name: expected 'Grocery Stores', got '%s'", retrieved.Name)
	}
	if retrieved.Priority != 10 {
		t.Errorf("Priority: expected 10, got %d", retrieved.Priority)
	}
}

func TestRuleStore_GetByLedgerID_OrderedByPriority(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rule-priority")
	defer env.CleanupUser(tu)

	ctx := context.Background()

	tag := env.CreateTag(tu.Ledger.ID, "Test", "#ff5722")

	// Create rules with different priorities
	rule1 := &models.CategorizationRule{
		LedgerID:     tu.Ledger.ID,
		Name:         "Low Priority",
		MatchPattern: "low",
		TagID:        tag.ID,
		Priority:     5,
		IsActive:     true,
	}
	_ = env.Rules.Create(ctx, rule1)

	rule2 := &models.CategorizationRule{
		LedgerID:     tu.Ledger.ID,
		Name:         "High Priority",
		MatchPattern: "high",
		TagID:        tag.ID,
		Priority:     20,
		IsActive:     true,
	}
	_ = env.Rules.Create(ctx, rule2)

	rule3 := &models.CategorizationRule{
		LedgerID:     tu.Ledger.ID,
		Name:         "Medium Priority",
		MatchPattern: "medium",
		TagID:        tag.ID,
		Priority:     10,
		IsActive:     true,
	}
	_ = env.Rules.Create(ctx, rule3)

	// Get rules - should be ordered by priority descending
	rules, err := env.Rules.GetByLedgerID(ctx, tu.Ledger.ID)
	if err != nil {
		t.Fatalf("GetByLedgerID failed: %v", err)
	}

	if len(rules) != 3 {
		t.Errorf("Expected 3 rules, got %d", len(rules))
	}

	if len(rules) >= 3 {
		if rules[0].Name != "High Priority" {
			t.Errorf("First rule should be 'High Priority', got '%s'", rules[0].Name)
		}
		if rules[1].Name != "Medium Priority" {
			t.Errorf("Second rule should be 'Medium Priority', got '%s'", rules[1].Name)
		}
		if rules[2].Name != "Low Priority" {
			t.Errorf("Third rule should be 'Low Priority', got '%s'", rules[2].Name)
		}
	}
}

// =============================================
// Cross-User Isolation Tests
// =============================================

func TestModelStores_CrossUserIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	user1 := env.CreateTestUser("isolation-1")
	defer env.CleanupUser(user1)
	user2 := env.CreateTestUser("isolation-2")
	defer env.CleanupUser(user2)

	ctx := context.Background()

	// User 1 creates data
	acc1 := env.CreateAccount(user1.Ledger.ID, "User1 Checking", models.AccountTypeAsset)
	tag1 := env.CreateTag(user1.Ledger.ID, "User1 Tag", "#ff0000")
	_ = acc1
	_ = tag1

	// User 2 creates data
	acc2 := env.CreateAccount(user2.Ledger.ID, "User2 Checking", models.AccountTypeAsset)
	tag2 := env.CreateTag(user2.Ledger.ID, "User2 Tag", "#00ff00")
	_ = acc2
	_ = tag2

	// User 1 should only see their own accounts
	accounts1, _ := env.Accounts.GetByLedgerID(ctx, user1.Ledger.ID)
	if len(accounts1) != 1 {
		t.Errorf("User 1 should have 1 account, got %d", len(accounts1))
	}
	if len(accounts1) > 0 && accounts1[0].Name != "User1 Checking" {
		t.Errorf("User 1 account name mismatch: %s", accounts1[0].Name)
	}

	// User 2 should only see their own accounts
	accounts2, _ := env.Accounts.GetByLedgerID(ctx, user2.Ledger.ID)
	if len(accounts2) != 1 {
		t.Errorf("User 2 should have 1 account, got %d", len(accounts2))
	}
	if len(accounts2) > 0 && accounts2[0].Name != "User2 Checking" {
		t.Errorf("User 2 account name mismatch: %s", accounts2[0].Name)
	}

	// User 1 should only see their own tags
	tags1, _ := env.Tags.GetByLedgerID(ctx, user1.Ledger.ID)
	if len(tags1) != 1 {
		t.Errorf("User 1 should have 1 tag, got %d", len(tags1))
	}

	// User 2 should only see their own tags
	tags2, _ := env.Tags.GetByLedgerID(ctx, user2.Ledger.ID)
	if len(tags2) != 1 {
		t.Errorf("User 2 should have 1 tag, got %d", len(tags2))
	}
}
