package handlers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

func TestAPIRules_List(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rules-list")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create tags for rules
	groceriesTag := env.CreateTag(tu.Ledger.ID, "Groceries", "#ff0000")
	utilitiesTag := env.CreateTag(tu.Ledger.ID, "Utilities", "#00ff00")

	// Create rules via store
	_ = env.Rules.Create(context.Background(), &models.CategorizationRule{
		ID:           uuid.New(),
		LedgerID:     tu.Ledger.ID,
		Name:         "Grocery Rule",
		MatchPattern: "grocery|walmart|costco",
		IsRegex:      true,
		TagID:        groceriesTag.ID,
		Priority:     10,
		IsActive:     true,
	})
	_ = env.Rules.Create(context.Background(), &models.CategorizationRule{
		ID:           uuid.New(),
		LedgerID:     tu.Ledger.ID,
		Name:         "Utility Rule",
		MatchPattern: "electric|gas|water",
		IsRegex:      true,
		TagID:        utilitiesTag.ID,
		Priority:     5,
		IsActive:     true,
	})

	// List rules
	resp := client.Get("/api/v1/rules")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			TagName  string `json:"tag_name"`
			Priority int    `json:"priority"`
			IsActive bool   `json:"is_active"`
		} `json:"data"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Data) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(result.Data))
	}

	// Should be sorted by priority (highest first)
	if result.Data[0].Priority < result.Data[1].Priority {
		t.Error("Expected rules to be sorted by priority (highest first)")
	}
}

func TestAPIRules_ListActiveOnly(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rules-list-active")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tag := env.CreateTag(tu.Ledger.ID, "Test", "#ff0000")

	// Create active rule
	_ = env.Rules.Create(context.Background(), &models.CategorizationRule{
		ID:           uuid.New(),
		LedgerID:     tu.Ledger.ID,
		Name:         "Active Rule",
		MatchPattern: "active",
		TagID:        tag.ID,
		IsActive:     true,
	})

	// Create another rule and then deactivate it via update
	inactiveRuleID := uuid.New()
	_ = env.Rules.Create(context.Background(), &models.CategorizationRule{
		ID:           inactiveRuleID,
		LedgerID:     tu.Ledger.ID,
		Name:         "Inactive Rule",
		MatchPattern: "inactive",
		TagID:        tag.ID,
		IsActive:     true, // Create sets this to true anyway
	})
	// Update to make inactive
	_ = env.Rules.Update(context.Background(), &models.CategorizationRule{
		ID:           inactiveRuleID,
		LedgerID:     tu.Ledger.ID,
		Name:         "Inactive Rule",
		MatchPattern: "inactive",
		TagID:        tag.ID,
		IsActive:     false,
	})

	// List active only
	resp := client.Get("/api/v1/rules?active_only=true")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data []struct{ Name string } `json:"data"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Data) != 1 {
		t.Errorf("Expected 1 active rule, got %d", len(result.Data))
	}
	if result.Data[0].Name != "Active Rule" {
		t.Errorf("Expected 'Active Rule', got %s", result.Data[0].Name)
	}
}

func TestAPIRules_Create(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rules-create")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tag := env.CreateTag(tu.Ledger.ID, "Groceries", "#ff0000")

	tests := []struct {
		name       string
		body       map[string]any
		wantStatus int
		wantError  bool
	}{
		{
			name: "valid rule with pattern",
			body: map[string]any{
				"name":          "Grocery Rule",
				"match_pattern": "walmart|costco",
				"is_regex":      true,
				"tag_id":        tag.ID.String(),
				"priority":      10,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid rule with prompt and pattern",
			body: map[string]any{
				"name":          "AI Grocery Rule",
				"prompt":        "Categorize as groceries if it's a food purchase",
				"match_pattern": "grocery",
				"tag_id":        tag.ID.String(),
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			body: map[string]any{
				"tag_id": tag.ID.String(),
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "missing tag_id",
			body: map[string]any{
				"name": "Test Rule",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "invalid tag_id",
			body: map[string]any{
				"name":   "Test Rule",
				"tag_id": uuid.New().String(), // Non-existent tag
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := client.Post("/api/v1/rules", tt.body)
			AssertStatus(t, resp, tt.wantStatus)

			if !tt.wantError && tt.wantStatus == http.StatusCreated {
				var result struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					TagName  string `json:"tag_name"`
					IsActive bool   `json:"is_active"`
				}
				ParseJSON(t, resp, &result)

				if result.Name != tt.body["name"] {
					t.Errorf("Expected name %s, got %s", tt.body["name"], result.Name)
				}
				if !result.IsActive {
					t.Error("Expected rule to be active by default")
				}
			}
		})
	}
}

func TestAPIRules_Get(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rules-get")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tag := env.CreateTag(tu.Ledger.ID, "Test Tag", "#ff0000")

	rule := &models.CategorizationRule{
		ID:           uuid.New(),
		LedgerID:     tu.Ledger.ID,
		Name:         "Test Rule",
		Prompt:       "Test prompt",
		MatchPattern: "test",
		TagID:        tag.ID,
		Priority:     10,
		IsActive:     true,
	}
	_ = env.Rules.Create(context.Background(), rule)

	resp := client.Get("/api/v1/rules/" + rule.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Prompt       string `json:"prompt"`
		MatchPattern string `json:"match_pattern"`
		TagID        string `json:"tag_id"`
		TagName      string `json:"tag_name"`
		Priority     int    `json:"priority"`
		IsActive     bool   `json:"is_active"`
	}
	ParseJSON(t, resp, &result)

	if result.ID != rule.ID.String() {
		t.Errorf("Expected ID %s, got %s", rule.ID.String(), result.ID)
	}
	if result.Name != "Test Rule" {
		t.Errorf("Expected name 'Test Rule', got %s", result.Name)
	}
	if result.TagName != "Test Tag" {
		t.Errorf("Expected tag name 'Test Tag', got %s", result.TagName)
	}
}

func TestAPIRules_GetNotFound(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rules-get-notfound")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	resp := client.Get("/api/v1/rules/" + uuid.New().String())
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestAPIRules_Update(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rules-update")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tag := env.CreateTag(tu.Ledger.ID, "Original Tag", "#ff0000")
	newTag := env.CreateTag(tu.Ledger.ID, "New Tag", "#00ff00")

	rule := &models.CategorizationRule{
		ID:           uuid.New(),
		LedgerID:     tu.Ledger.ID,
		Name:         "Original Name",
		MatchPattern: "original",
		TagID:        tag.ID,
		Priority:     5,
		IsActive:     true,
	}
	_ = env.Rules.Create(context.Background(), rule)

	resp := client.Put("/api/v1/rules/"+rule.ID.String(), map[string]any{
		"name":          "Updated Name",
		"match_pattern": "updated",
		"tag_id":        newTag.ID.String(),
		"priority":      15,
	})
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Name         string `json:"name"`
		MatchPattern string `json:"match_pattern"`
		TagID        string `json:"tag_id"`
		TagName      string `json:"tag_name"`
		Priority     int    `json:"priority"`
	}
	ParseJSON(t, resp, &result)

	if result.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", result.Name)
	}
	if result.MatchPattern != "updated" {
		t.Errorf("Expected match_pattern 'updated', got %s", result.MatchPattern)
	}
	if result.TagID != newTag.ID.String() {
		t.Errorf("Expected tag_id %s, got %s", newTag.ID.String(), result.TagID)
	}
	if result.Priority != 15 {
		t.Errorf("Expected priority 15, got %d", result.Priority)
	}
}

func TestAPIRules_UpdateIsActive(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rules-update-active")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tag := env.CreateTag(tu.Ledger.ID, "Test Tag", "#ff0000")

	rule := &models.CategorizationRule{
		ID:           uuid.New(),
		LedgerID:     tu.Ledger.ID,
		Name:         "Test Rule",
		MatchPattern: "test",
		TagID:        tag.ID,
		IsActive:     true,
	}
	_ = env.Rules.Create(context.Background(), rule)

	// Deactivate
	resp := client.Put("/api/v1/rules/"+rule.ID.String(), map[string]any{
		"is_active": false,
	})
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		IsActive bool `json:"is_active"`
	}
	ParseJSON(t, resp, &result)

	if result.IsActive {
		t.Error("Expected rule to be inactive")
	}
}

func TestAPIRules_Delete(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rules-delete")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tag := env.CreateTag(tu.Ledger.ID, "Test Tag", "#ff0000")

	rule := &models.CategorizationRule{
		ID:           uuid.New(),
		LedgerID:     tu.Ledger.ID,
		Name:         "To Delete",
		MatchPattern: "delete",
		TagID:        tag.ID,
		IsActive:     true,
	}
	_ = env.Rules.Create(context.Background(), rule)

	resp := client.Delete("/api/v1/rules/" + rule.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Deleted bool `json:"deleted"`
	}
	ParseJSON(t, resp, &result)

	if !result.Deleted {
		t.Error("Expected deleted to be true")
	}

	// Verify it's gone
	resp = client.Get("/api/v1/rules/" + rule.ID.String())
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestAPIRules_Apply(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("rules-apply")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	uncategorized := env.CreateAccount(tu.Ledger.ID, "Uncategorized Expenses", models.AccountTypeExpense)

	// Create tag and rule
	groceriesTag := env.CreateTag(tu.Ledger.ID, "Groceries", "#ff0000")
	_ = env.Rules.Create(context.Background(), &models.CategorizationRule{
		ID:           uuid.New(),
		LedgerID:     tu.Ledger.ID,
		Name:         "Grocery Rule",
		MatchPattern: "walmart",
		TagID:        groceriesTag.ID,
		IsActive:     true,
	})

	// Create uncategorized transaction
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "WALMART STORE #1234", []*models.Entry{
		{AccountID: uncategorized.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})
	env.CreateTransaction(tu.Ledger.ID, timeNow(), "TARGET STORE", []*models.Entry{
		{AccountID: uncategorized.ID, AmountCents: 3000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -3000, Currency: "USD"},
	})

	// Apply rules
	resp := client.Post("/api/v1/rules/apply", nil)
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Matched   int `json:"matched"`
		Processed int `json:"processed"`
	}
	ParseJSON(t, resp, &result)

	if result.Matched != 1 {
		t.Errorf("Expected 1 matched transaction, got %d", result.Matched)
	}
	if result.Processed < 1 {
		t.Errorf("Expected at least 1 processed transaction, got %d", result.Processed)
	}
}

func TestAPIRules_CrossUserIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	user1 := env.CreateTestUser("rules-iso-1")
	defer env.CleanupTestUser(user1)
	user2 := env.CreateTestUser("rules-iso-2")
	defer env.CleanupTestUser(user2)

	client1 := env.NewAPIClient(user1)
	client2 := env.NewAPIClient(user2)

	// User 1 creates a rule
	tag := env.CreateTag(user1.Ledger.ID, "User1 Tag", "#ff0000")
	rule := &models.CategorizationRule{
		ID:           uuid.New(),
		LedgerID:     user1.Ledger.ID,
		Name:         "User1 Rule",
		MatchPattern: "user1",
		TagID:        tag.ID,
		IsActive:     true,
	}
	_ = env.Rules.Create(context.Background(), rule)

	// User 2 should not be able to access it
	resp := client2.Get("/api/v1/rules/" + rule.ID.String())
	AssertStatus(t, resp, http.StatusNotFound)

	// User 2 should not be able to update it
	resp = client2.Put("/api/v1/rules/"+rule.ID.String(), map[string]any{
		"name": "Hacked",
	})
	AssertStatus(t, resp, http.StatusNotFound)

	// User 2 should not be able to delete it
	resp = client2.Delete("/api/v1/rules/" + rule.ID.String())
	AssertStatus(t, resp, http.StatusNotFound)

	// User 1 should still be able to access it
	resp = client1.Get("/api/v1/rules/" + rule.ID.String())
	AssertStatus(t, resp, http.StatusOK)
}
