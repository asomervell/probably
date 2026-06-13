package backup_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/asomervell/probably/internal/backup"
	"github.com/asomervell/probably/internal/categorize"
	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TestEnv struct {
	T    *testing.T
	Pool *pgxpool.Pool
	DB   *db.DB

	Users        *models.UserStore
	Ledgers      *models.LedgerStore
	Accounts     *models.AccountStore
	Transactions *models.TransactionStore
	Tags         *models.TagStore
	Rules        *models.RuleStore
	Entities     *models.EntityStore
	Permissions  *models.PermissionStore
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
		Entities:     models.NewEntityStore(database.Pool),
		Permissions:  models.NewPermissionStore(database.Pool),
	}
}

func (env *TestEnv) Cleanup() { env.DB.Close() }

// TestUser represents a test user with their ledger
type TestUser struct {
	User   *models.User
	Ledger *models.Ledger
}

func (env *TestEnv) CreateTestUser(suffix string) *TestUser {
	env.T.Helper()
	ctx := context.Background()

	user := &models.User{
		ID:        uuid.New(),
		Email:     suffix + "-" + uuid.New().String()[:8] + "@test.com",
		Password:  "test-password",
		Confirmed: true,
	}
	if err := env.Users.Create(ctx, user); err != nil {
		env.T.Fatalf("Failed to create user: %v", err)
	}

	// Create person entity for user
	personEntity := &models.Entity{
		ID:        uuid.New(),
		Type:      models.EntityTypePerson,
		Subtype:   "individual",
		Name:      user.Email,
		UserVerified: true,
	}
	if err := env.Entities.Create(ctx, personEntity); err != nil {
		env.T.Fatalf("Failed to create person entity: %v", err)
	}

	// Grant owner permission to user for their entity
	perm := &models.UserEntityPermission{
		UserID:         user.ID,
		EntityID:       personEntity.ID,
		PermissionLevel: models.PermissionLevelOwner,
		GrantedBy:      &user.ID,
	}
	if err := env.Permissions.CreateUserEntityPermission(ctx, perm); err != nil {
		env.T.Fatalf("Failed to create permission: %v", err)
	}

	// Create ledger
	ledger := &models.Ledger{
		ID:       uuid.New(),
		UserID:   user.ID, // Keep for backward compatibility in schema
		Name:     "Test Ledger for " + suffix,
		Currency: "USD",
	}
	if err := env.Ledgers.Create(ctx, ledger); err != nil {
		env.T.Fatalf("Failed to create ledger: %v", err)
	}

	// Link ledger to entity
	entityLedger := &models.EntityLedger{
		EntityID: personEntity.ID,
		LedgerID: ledger.ID,
		Role:     "owner",
	}
	if err := env.Permissions.CreateEntityLedger(ctx, entityLedger); err != nil {
		env.T.Fatalf("Failed to link ledger to entity: %v", err)
	}

	return &TestUser{User: user, Ledger: ledger}
}

func (env *TestEnv) CleanupUser(tu *TestUser) {
	ctx := context.Background()
	_, _ = env.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", tu.User.ID)
}

// getLedgersByUserID gets all ledgers a user has access to via entity permissions
func (env *TestEnv) getLedgersByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Ledger, error) {
	userPerms, err := env.Permissions.GetUserEntityPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	var ledgers []*models.Ledger
	seen := make(map[uuid.UUID]bool)
	
	for _, perm := range userPerms {
		entityLedgers, err := env.Permissions.GetEntityLedgers(ctx, perm.EntityID)
		if err != nil {
			continue
		}
		for _, el := range entityLedgers {
			if !seen[el.LedgerID] {
				ledger, err := env.Ledgers.GetByID(ctx, el.LedgerID)
				if err == nil {
					ledgers = append(ledgers, ledger)
					seen[el.LedgerID] = true
				}
			}
		}
	}
	
	return ledgers, nil
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

func (env *TestEnv) CreateTag(ledgerID uuid.UUID, name, color string, parentID *uuid.UUID) *models.Tag {
	env.T.Helper()
	tag := &models.Tag{
		ID:       uuid.New(),
		LedgerID: ledgerID,
		ParentID: parentID,
		Name:     name,
		Color:    color,
	}
	if err := env.Tags.Create(context.Background(), tag); err != nil {
		env.T.Fatalf("Failed to create tag: %v", err)
	}
	return tag
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

func (env *TestEnv) CreateRule(ledgerID, tagID uuid.UUID, name, prompt, matchPattern string, priority int) *models.CategorizationRule {
	env.T.Helper()
	rule := &models.CategorizationRule{
		ID:           uuid.New(),
		LedgerID:     ledgerID,
		Name:         name,
		Prompt:       prompt,
		MatchPattern: matchPattern,
		TagID:        tagID,
		Priority:     priority,
		IsActive:     true,
	}
	if err := env.Rules.Create(context.Background(), rule); err != nil {
		env.T.Fatalf("Failed to create rule: %v", err)
	}
	return rule
}

func TestExportImport_FullE2E(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// === Create User A with data ===
	userA := env.CreateTestUser("user-a")
	defer env.CleanupUser(userA)

	// Create accounts for User A
	checkingA := env.CreateAccount(userA.Ledger.ID, "Checking Account", models.AccountTypeAsset)
	savingsA := env.CreateAccount(userA.Ledger.ID, "Savings Account", models.AccountTypeAsset)
	groceriesA := env.CreateAccount(userA.Ledger.ID, "Groceries", models.AccountTypeExpense)
	incomeA := env.CreateAccount(userA.Ledger.ID, "Salary", models.AccountTypeIncome)

	// Create tags for User A (with hierarchy)
	foodTag := env.CreateTag(userA.Ledger.ID, "Food & Drink", "#ff5733", nil)
	groceryTag := env.CreateTag(userA.Ledger.ID, "Groceries", "#33ff57", &foodTag.ID)
	transportTag := env.CreateTag(userA.Ledger.ID, "Transportation", "#3357ff", nil)

	// Create transactions for User A
	txn1 := env.CreateTransaction(userA.Ledger.ID, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "Whole Foods Market", []*models.Entry{
		{AccountID: groceriesA.ID, AmountCents: 8500, Currency: "USD"},
		{AccountID: checkingA.ID, AmountCents: -8500, Currency: "USD"},
	})

	txn2 := env.CreateTransaction(userA.Ledger.ID, time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC), "Transfer to Savings", []*models.Entry{
		{AccountID: savingsA.ID, AmountCents: 50000, Currency: "USD"},
		{AccountID: checkingA.ID, AmountCents: -50000, Currency: "USD"},
	})

	_ = env.CreateTransaction(userA.Ledger.ID, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "January Salary", []*models.Entry{
		{AccountID: checkingA.ID, AmountCents: 500000, Currency: "USD"},
		{AccountID: incomeA.ID, AmountCents: -500000, Currency: "USD"},
	})

	// Add tags to transactions
	if err := env.Tags.AddTagToTransaction(ctx, txn1.ID, groceryTag.ID); err != nil {
		t.Fatalf("Failed to add tag to transaction: %v", err)
	}
	if err := env.Tags.AddTagToTransaction(ctx, txn1.ID, foodTag.ID); err != nil {
		t.Fatalf("Failed to add tag to transaction: %v", err)
	}

	// Create categorization rules
	_ = env.CreateRule(userA.Ledger.ID, groceryTag.ID, "Grocery Stores", "Categorize supermarket and grocery store purchases", "grocery|supermarket|whole foods", 10)
	_ = env.CreateRule(userA.Ledger.ID, transportTag.ID, "Gas Stations", "Categorize gas station and fuel purchases", "shell|chevron|gas", 5)

	// Mark txn2 as a transfer (simulate paired transfer)
	_, _ = env.Pool.Exec(ctx, "UPDATE transactions SET is_transfer = true WHERE id = $1", txn2.ID)

	t.Logf("User A created with:")
	t.Logf("  - 4 accounts")
	t.Logf("  - 3 tags (with hierarchy)")
	t.Logf("  - 3 transactions")
	t.Logf("  - 2 rules")
	t.Logf("  - 2 transaction tags")

	// === Export User A's data ===
	zipData, exportStats, err := backup.Export(ctx, env.Pool, userA.Ledger.ID)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	t.Logf("Exported %d bytes of data (tags: %d)", zipData.Len(), exportStats.Tags)

	// === Create User B (empty) ===
	userB := env.CreateTestUser("user-b")
	defer env.CleanupUser(userB)

	// Verify User B starts empty (just the default ledger)
	accountsB, _ := env.Accounts.GetByLedgerID(ctx, userB.Ledger.ID)
	if len(accountsB) != 0 {
		t.Errorf("User B should start with 0 accounts, got %d", len(accountsB))
	}

	// === Import the backup into User B ===
	reader := bytes.NewReader(zipData.Bytes())
	stats, err := backup.Import(ctx, env.Pool, userB.User.ID, reader, int64(zipData.Len()))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	t.Logf("Import completed successfully: %d accounts, %d tags, %d transactions", stats.Accounts, stats.Tags, stats.Transactions)

	// === Verify User B's data matches User A's ===

	// Get User B's new ledger (import replaces the ledger)
	ledgersB, err := env.getLedgersByUserID(ctx, userB.User.ID)
	if err != nil {
		t.Fatalf("Failed to get User B ledgers: %v", err)
	}
	if len(ledgersB) != 1 {
		t.Fatalf("Expected 1 ledger for User B, got %d", len(ledgersB))
	}
	newLedgerB := ledgersB[0]

	// Verify ledger name and currency were imported
	if newLedgerB.Name != userA.Ledger.Name {
		t.Errorf("Ledger name mismatch: expected %q, got %q", userA.Ledger.Name, newLedgerB.Name)
	}
	if newLedgerB.Currency != userA.Ledger.Currency {
		t.Errorf("Ledger currency mismatch: expected %q, got %q", userA.Ledger.Currency, newLedgerB.Currency)
	}

	// Verify accounts
	accountsBNew, err := env.Accounts.GetByLedgerID(ctx, newLedgerB.ID)
	if err != nil {
		t.Fatalf("Failed to get User B accounts: %v", err)
	}
	if len(accountsBNew) != 4 {
		t.Errorf("Expected 4 accounts, got %d", len(accountsBNew))
	}

	// Check account names exist
	accountNames := make(map[string]bool)
	for _, acc := range accountsBNew {
		accountNames[acc.Name] = true
	}
	expectedAccounts := []string{"Checking Account", "Savings Account", "Groceries", "Salary"}
	for _, name := range expectedAccounts {
		if !accountNames[name] {
			t.Errorf("Missing account: %s", name)
		}
	}

	// Verify tags
	tagsB, err := env.Tags.GetByLedgerID(ctx, newLedgerB.ID)
	if err != nil {
		t.Fatalf("Failed to get User B tags: %v", err)
	}
	if len(tagsB) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(tagsB))
	}

	// Check tag hierarchy (Groceries should have parent Food & Drink)
	tagMap := make(map[string]*models.Tag)
	for _, tag := range tagsB {
		tagMap[tag.Name] = tag
	}
	if groceryTagB, ok := tagMap["Groceries"]; ok {
		if groceryTagB.ParentID == nil {
			t.Error("Groceries tag should have a parent")
		} else if foodTagB, ok := tagMap["Food & Drink"]; ok {
			if *groceryTagB.ParentID != foodTagB.ID {
				t.Error("Groceries parent should be Food & Drink")
			}
		}
	} else {
		t.Error("Missing Groceries tag")
	}

	// Verify transactions
	txnsB, totalB, err := env.Transactions.List(ctx, models.TransactionFilter{LedgerID: newLedgerB.ID})
	if err != nil {
		t.Fatalf("Failed to get User B transactions: %v", err)
	}
	if totalB != 3 {
		t.Errorf("Expected 3 transactions, got %d", totalB)
	}

	// Check transaction descriptions exist
	txnDescriptions := make(map[string]bool)
	var transferTxn *models.Transaction
	for _, txn := range txnsB {
		txnDescriptions[txn.Description] = true
		if txn.Description == "Transfer to Savings" {
			transferTxn = txn
		}
	}
	expectedTxns := []string{"Whole Foods Market", "Transfer to Savings", "January Salary"}
	for _, desc := range expectedTxns {
		if !txnDescriptions[desc] {
			t.Errorf("Missing transaction: %s", desc)
		}
	}

	// Verify transfer flag was preserved
	if transferTxn != nil && !transferTxn.IsTransfer {
		t.Error("Transfer to Savings should have is_transfer=true")
	}

	// Verify entries
	for _, txn := range txnsB {
		if err := env.Transactions.LoadEntries(ctx, txn); err != nil {
			t.Fatalf("Failed to load entries: %v", err)
		}
		if len(txn.Entries) != 2 {
			t.Errorf("Transaction %q should have 2 entries, got %d", txn.Description, len(txn.Entries))
		}

		// Verify entries balance
		var sum int64
		for _, e := range txn.Entries {
			sum += e.AmountCents
		}
		if sum != 0 {
			t.Errorf("Transaction %q entries don't balance: sum = %d", txn.Description, sum)
		}
	}

	// Verify transaction tags
	var wholeFoodsTxn *models.Transaction
	for _, txn := range txnsB {
		if txn.Description == "Whole Foods Market" {
			wholeFoodsTxn = txn
			break
		}
	}
	if wholeFoodsTxn != nil {
		if err := env.Transactions.LoadTags(ctx, wholeFoodsTxn); err != nil {
			t.Fatalf("Failed to load tags: %v", err)
		}
		if len(wholeFoodsTxn.Tags) != 2 {
			t.Errorf("Whole Foods transaction should have 2 tags, got %d", len(wholeFoodsTxn.Tags))
		}
	}

	// Verify rules
	rulesB, err := env.Rules.GetByLedgerID(ctx, newLedgerB.ID)
	if err != nil {
		t.Fatalf("Failed to get User B rules: %v", err)
	}
	if len(rulesB) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rulesB))
	}

	// Check rule details
	ruleNames := make(map[string]*models.CategorizationRule)
	for _, r := range rulesB {
		ruleNames[r.Name] = r
	}
	if groceryRule, ok := ruleNames["Grocery Stores"]; ok {
		if groceryRule.Priority != 10 {
			t.Errorf("Grocery Stores rule priority should be 10, got %d", groceryRule.Priority)
		}
		if groceryRule.Prompt != "Categorize supermarket and grocery store purchases" {
			t.Errorf("Grocery Stores rule prompt mismatch")
		}
	} else {
		t.Error("Missing Grocery Stores rule")
	}

	// === Verify User A's data is still intact ===
	accountsA, _ := env.Accounts.GetByLedgerID(ctx, userA.Ledger.ID)
	if len(accountsA) != 4 {
		t.Errorf("User A should still have 4 accounts, got %d", len(accountsA))
	}

	t.Log("E2E test passed: All data exported and imported correctly")
}

func TestExportImport_EmptyLedger(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// Create a user with empty ledger
	userA := env.CreateTestUser("empty-user")
	defer env.CleanupUser(userA)

	// Export empty ledger
	zipData, _, err := backup.Export(ctx, env.Pool, userA.Ledger.ID)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Create another user and import
	userB := env.CreateTestUser("import-empty")
	defer env.CleanupUser(userB)

	reader := bytes.NewReader(zipData.Bytes())
	if _, err := backup.Import(ctx, env.Pool, userB.User.ID, reader, int64(zipData.Len())); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify the ledger was imported
	ledgers, _ := env.getLedgersByUserID(ctx, userB.User.ID)
	if len(ledgers) != 1 {
		t.Errorf("Expected 1 ledger, got %d", len(ledgers))
	}

	t.Log("Empty ledger export/import passed")
}

func TestExportImport_LargeDataset(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	userA := env.CreateTestUser("large-dataset")
	defer env.CleanupUser(userA)

	// Create accounts
	checking := env.CreateAccount(userA.Ledger.ID, "Checking", models.AccountTypeAsset)
	expense := env.CreateAccount(userA.Ledger.ID, "Expenses", models.AccountTypeExpense)

	// Create many tags
	for i := 0; i < 50; i++ {
		env.CreateTag(userA.Ledger.ID, "Tag "+string(rune('A'+i%26))+"-"+uuid.New().String()[:4], "#"+uuid.New().String()[:6], nil)
	}

	// Create many transactions
	for i := 0; i < 100; i++ {
		env.CreateTransaction(userA.Ledger.ID, time.Now().AddDate(0, 0, -i), "Transaction #"+uuid.New().String()[:8], []*models.Entry{
			{AccountID: expense.ID, AmountCents: int64((i + 1) * 100), Currency: "USD"},
			{AccountID: checking.ID, AmountCents: int64(-(i + 1) * 100), Currency: "USD"},
		})
	}

	t.Logf("Created 2 accounts, 50 tags, 100 transactions")

	// Export
	zipData, _, err := backup.Export(ctx, env.Pool, userA.Ledger.ID)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	t.Logf("Exported %d bytes", zipData.Len())

	// Import to new user
	userB := env.CreateTestUser("import-large")
	defer env.CleanupUser(userB)

	reader := bytes.NewReader(zipData.Bytes())
	if _, err := backup.Import(ctx, env.Pool, userB.User.ID, reader, int64(zipData.Len())); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify counts
	ledgers, _ := env.getLedgersByUserID(ctx, userB.User.ID)
	newLedger := ledgers[0]

	accounts, _ := env.Accounts.GetByLedgerID(ctx, newLedger.ID)
	if len(accounts) != 2 {
		t.Errorf("Expected 2 accounts, got %d", len(accounts))
	}

	tags, _ := env.Tags.GetByLedgerID(ctx, newLedger.ID)
	if len(tags) != 50 {
		t.Errorf("Expected 50 tags, got %d", len(tags))
	}

	_, total, _ := env.Transactions.List(ctx, models.TransactionFilter{LedgerID: newLedger.ID, Limit: 1})
	if total != 100 {
		t.Errorf("Expected 100 transactions, got %d", total)
	}

	t.Log("Large dataset export/import passed")
}

func TestExportImport_TransferPairs(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	userA := env.CreateTestUser("transfer-pairs")
	defer env.CleanupUser(userA)

	checking := env.CreateAccount(userA.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(userA.Ledger.ID, "Savings", models.AccountTypeAsset)
	transferIn := env.CreateAccount(userA.Ledger.ID, "Internal Transfer", models.AccountTypeIncome)
	transferOut := env.CreateAccount(userA.Ledger.ID, "Internal Transfer", models.AccountTypeExpense)

	// Create a transfer pair
	txn1 := env.CreateTransaction(userA.Ledger.ID, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "Transfer Out", []*models.Entry{
		{AccountID: transferOut.ID, AmountCents: 10000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -10000, Currency: "USD"},
	})

	txn2 := env.CreateTransaction(userA.Ledger.ID, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "Transfer In", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 10000, Currency: "USD"},
		{AccountID: transferIn.ID, AmountCents: -10000, Currency: "USD"},
	})

	// Link them as a transfer pair
	_, _ = env.Pool.Exec(ctx, "UPDATE transactions SET is_transfer = true, transfer_pair_id = $2 WHERE id = $1", txn1.ID, txn2.ID)
	_, _ = env.Pool.Exec(ctx, "UPDATE transactions SET is_transfer = true, transfer_pair_id = $2 WHERE id = $1", txn2.ID, txn1.ID)

	// Export
	zipData, _, err := backup.Export(ctx, env.Pool, userA.Ledger.ID)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Import to new user
	userB := env.CreateTestUser("import-transfers")
	defer env.CleanupUser(userB)

	reader := bytes.NewReader(zipData.Bytes())
	if _, err := backup.Import(ctx, env.Pool, userB.User.ID, reader, int64(zipData.Len())); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify transfer pairs are maintained
	ledgers, _ := env.getLedgersByUserID(ctx, userB.User.ID)
	newLedger := ledgers[0]

	txns, _, _ := env.Transactions.List(ctx, models.TransactionFilter{LedgerID: newLedger.ID})

	var transferOut2, transferIn2 *models.Transaction
	for _, txn := range txns {
		if txn.Description == "Transfer Out" {
			transferOut2 = txn
		}
		if txn.Description == "Transfer In" {
			transferIn2 = txn
		}
	}

	if transferOut2 == nil || transferIn2 == nil {
		t.Fatal("Missing transfer transactions after import")
	}

	if !transferOut2.IsTransfer || !transferIn2.IsTransfer {
		t.Error("is_transfer flag not preserved")
	}

	if transferOut2.TransferPairID == nil || transferIn2.TransferPairID == nil {
		t.Error("transfer_pair_id not preserved")
	}

	if *transferOut2.TransferPairID != transferIn2.ID {
		t.Error("Transfer pair link broken after import")
	}

	if *transferIn2.TransferPairID != transferOut2.ID {
		t.Error("Reverse transfer pair link broken after import")
	}

	t.Log("Transfer pairs export/import passed")
}

func TestExportImport_SeededTaxonomyTags(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// Create user and seed with the real taxonomy (like the app does)
	userA := env.CreateTestUser("taxonomy-user")
	defer env.CleanupUser(userA)

	// Seed the default taxonomy tags (this is what the app does)
	taxonomyService := categorize.NewTaxonomyService(env.Pool)
	if err := taxonomyService.SeedDefaultTags(ctx, userA.Ledger.ID); err != nil {
		t.Fatalf("Failed to seed tags: %v", err)
	}

	// Verify tags were seeded
	tagsA, err := env.Tags.GetByLedgerID(ctx, userA.Ledger.ID)
	if err != nil {
		t.Fatalf("Failed to get tags: %v", err)
	}
	t.Logf("Seeded %d taxonomy tags", len(tagsA))

	// Should have many tags (the default taxonomy has 60+ categories)
	if len(tagsA) < 50 {
		t.Errorf("Expected at least 50 seeded tags, got %d", len(tagsA))
	}

	// Count parent tags (those without parent_id)
	parentCount := 0
	childCount := 0
	for _, tag := range tagsA {
		if tag.ParentID == nil {
			parentCount++
		} else {
			childCount++
		}
	}
	t.Logf("Parent categories: %d, Subcategories: %d", parentCount, childCount)

	// Export
	zipData, _, err := backup.Export(ctx, env.Pool, userA.Ledger.ID)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	t.Logf("Exported %d bytes", zipData.Len())

	// Create new user and import
	userB := env.CreateTestUser("import-taxonomy")
	defer env.CleanupUser(userB)

	reader := bytes.NewReader(zipData.Bytes())
	if _, err := backup.Import(ctx, env.Pool, userB.User.ID, reader, int64(zipData.Len())); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Get new ledger
	ledgers, _ := env.getLedgersByUserID(ctx, userB.User.ID)
	newLedger := ledgers[0]

	// Verify ALL tags were imported
	tagsB, err := env.Tags.GetByLedgerID(ctx, newLedger.ID)
	if err != nil {
		t.Fatalf("Failed to get imported tags: %v", err)
	}
	t.Logf("Imported %d taxonomy tags", len(tagsB))

	if len(tagsB) != len(tagsA) {
		t.Errorf("Tag count mismatch: exported %d, imported %d", len(tagsA), len(tagsB))
	}

	// Verify parent-child relationships are intact
	parentCountB := 0
	childCountB := 0
	for _, tag := range tagsB {
		if tag.ParentID == nil {
			parentCountB++
		} else {
			childCountB++
		}
	}

	if parentCountB != parentCount {
		t.Errorf("Parent tag count mismatch: %d vs %d", parentCount, parentCountB)
	}
	if childCountB != childCount {
		t.Errorf("Child tag count mismatch: %d vs %d", childCount, childCountB)
	}

	// Verify specific category names exist
	tagNames := make(map[string]bool)
	for _, tag := range tagsB {
		tagNames[tag.Name] = true
	}

	expectedCategories := []string{
		"Income", "Salary & Wages",
		"Food & Drink", "Groceries", "Restaurants",
		"Transportation", "Gas & Fuel",
		"Shopping", "Clothing & Apparel",
		"Entertainment", "Streaming Services",
		"Healthcare", "Pharmacy",
	}

	for _, name := range expectedCategories {
		if !tagNames[name] {
			t.Errorf("Missing expected category tag: %s", name)
		}
	}

	t.Log("Taxonomy tags export/import passed")
}

func TestExportImport_UserIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// Create two users with data
	userA := env.CreateTestUser("isolation-a")
	defer env.CleanupUser(userA)

	userC := env.CreateTestUser("isolation-c")
	defer env.CleanupUser(userC)

	// Add data to User A
	env.CreateAccount(userA.Ledger.ID, "User A Checking", models.AccountTypeAsset)
	env.CreateTag(userA.Ledger.ID, "User A Tag", "#ff0000", nil)

	// Add data to User C
	env.CreateAccount(userC.Ledger.ID, "User C Checking", models.AccountTypeAsset)
	env.CreateTag(userC.Ledger.ID, "User C Tag", "#00ff00", nil)

	// Export User A's data
	zipData, _, err := backup.Export(ctx, env.Pool, userA.Ledger.ID)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Create User B and import User A's data
	userB := env.CreateTestUser("isolation-b")
	defer env.CleanupUser(userB)

	reader := bytes.NewReader(zipData.Bytes())
	if _, err := backup.Import(ctx, env.Pool, userB.User.ID, reader, int64(zipData.Len())); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify User C's data is NOT affected
	accountsC, _ := env.Accounts.GetByLedgerID(ctx, userC.Ledger.ID)
	if len(accountsC) != 1 {
		t.Errorf("User C should still have 1 account, got %d", len(accountsC))
	}
	if accountsC[0].Name != "User C Checking" {
		t.Errorf("User C account name changed unexpectedly")
	}

	tagsC, _ := env.Tags.GetByLedgerID(ctx, userC.Ledger.ID)
	if len(tagsC) != 1 {
		t.Errorf("User C should still have 1 tag, got %d", len(tagsC))
	}
	if tagsC[0].Name != "User C Tag" {
		t.Errorf("User C tag name changed unexpectedly")
	}

	// Verify User A's data is still intact
	accountsA, _ := env.Accounts.GetByLedgerID(ctx, userA.Ledger.ID)
	if len(accountsA) != 1 {
		t.Errorf("User A should still have 1 account, got %d", len(accountsA))
	}

	t.Log("User isolation test passed")
}
