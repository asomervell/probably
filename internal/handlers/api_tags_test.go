package handlers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

func TestAPITags_List(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tags-list")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create some tags
	env.CreateTag(tu.Ledger.ID, "Groceries", "#ff0000")
	env.CreateTag(tu.Ledger.ID, "Entertainment", "#00ff00")
	env.CreateTag(tu.Ledger.ID, "Utilities", "#0000ff")

	// List tags
	resp := client.Get("/api/v1/tags")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Color      string `json:"color"`
			UsageCount int    `json:"usage_count"`
		} `json:"data"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Data) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(result.Data))
	}
}

func TestAPITags_ListHierarchy(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tags-hierarchy")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create parent tag
	parent := env.CreateTag(tu.Ledger.ID, "Food", "#ff0000")

	// Create child tags (directly via store since we need parent_id)
	child1 := &models.Tag{
		ID:       uuid.New(),
		LedgerID: tu.Ledger.ID,
		ParentID: &parent.ID,
		Name:     "Groceries",
		Color:    "#ff5555",
	}
	_ = env.Tags.Create(context.Background(), child1)

	child2 := &models.Tag{
		ID:       uuid.New(),
		LedgerID: tu.Ledger.ID,
		ParentID: &parent.ID,
		Name:     "Restaurants",
		Color:    "#ff7777",
	}
	_ = env.Tags.Create(context.Background(), child2)

	// List with hierarchy
	resp := client.Get("/api/v1/tags?hierarchy=true")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Children []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"children"`
		} `json:"data"`
	}
	ParseJSON(t, resp, &result)

	// Should only return root tags
	if len(result.Data) != 1 {
		t.Errorf("Expected 1 root tag, got %d", len(result.Data))
	}
	if result.Data[0].Name != "Food" {
		t.Errorf("Expected root tag 'Food', got %s", result.Data[0].Name)
	}
	if len(result.Data[0].Children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(result.Data[0].Children))
	}
}

func TestAPITags_Create(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tags-create")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tests := []struct {
		name       string
		body       map[string]any
		wantStatus int
		wantError  bool
	}{
		{
			name: "valid tag",
			body: map[string]any{
				"name":  "Test Tag",
				"color": "#ff0000",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "tag with default color",
			body: map[string]any{
				"name": "Default Color Tag",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			body: map[string]any{
				"color": "#ff0000",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := client.Post("/api/v1/tags", tt.body)
			AssertStatus(t, resp, tt.wantStatus)

			if !tt.wantError && tt.wantStatus == http.StatusCreated {
				var result struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Color string `json:"color"`
				}
				ParseJSON(t, resp, &result)

				if result.Name != tt.body["name"] {
					t.Errorf("Expected name %s, got %s", tt.body["name"], result.Name)
				}
				// Check default color is set
				if result.Color == "" {
					t.Error("Expected color to be set")
				}
			}
		})
	}
}

func TestAPITags_CreateWithParent(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tags-create-parent")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create parent tag
	parent := env.CreateTag(tu.Ledger.ID, "Parent", "#ff0000")

	// Create child tag
	resp := client.Post("/api/v1/tags", map[string]any{
		"name":      "Child",
		"color":     "#00ff00",
		"parent_id": parent.ID.String(),
	})
	AssertStatus(t, resp, http.StatusCreated)

	var result struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		ParentID *string `json:"parent_id"`
	}
	ParseJSON(t, resp, &result)

	if result.ParentID == nil || *result.ParentID != parent.ID.String() {
		t.Errorf("Expected parent_id %s, got %v", parent.ID.String(), result.ParentID)
	}
}

func TestAPITags_Get(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tags-get")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tag := env.CreateTag(tu.Ledger.ID, "Test Tag", "#ff0000")

	resp := client.Get("/api/v1/tags/" + tag.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Color      string `json:"color"`
		UsageCount int    `json:"usage_count"`
	}
	ParseJSON(t, resp, &result)

	if result.ID != tag.ID.String() {
		t.Errorf("Expected ID %s, got %s", tag.ID.String(), result.ID)
	}
	if result.Name != "Test Tag" {
		t.Errorf("Expected name 'Test Tag', got %s", result.Name)
	}
	if result.UsageCount != 0 {
		t.Errorf("Expected usage count 0, got %d", result.UsageCount)
	}
}

func TestAPITags_GetWithUsageCount(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tags-usage")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	groceries := env.CreateAccount(tu.Ledger.ID, "Groceries", models.AccountTypeExpense)

	// Create tag
	tag := env.CreateTag(tu.Ledger.ID, "Food", "#ff0000")

	// Create transaction
	txn := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Grocery Store", []*models.Entry{
		{AccountID: groceries.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})

	// Add tag to transaction
	_ = env.Tags.AddTagToTransaction(env.T.Context(), txn.ID, tag.ID)

	// Get tag with usage count
	resp := client.Get("/api/v1/tags/" + tag.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		UsageCount int `json:"usage_count"`
	}
	ParseJSON(t, resp, &result)

	if result.UsageCount != 1 {
		t.Errorf("Expected usage count 1, got %d", result.UsageCount)
	}
}

func TestAPITags_GetNotFound(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tags-get-notfound")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	resp := client.Get("/api/v1/tags/" + uuid.New().String())
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestAPITags_Update(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tags-update")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tag := env.CreateTag(tu.Ledger.ID, "Original Name", "#ff0000")

	resp := client.Put("/api/v1/tags/"+tag.ID.String(), map[string]any{
		"name":  "Updated Name",
		"color": "#00ff00",
	})
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	ParseJSON(t, resp, &result)

	if result.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", result.Name)
	}
	if result.Color != "#00ff00" {
		t.Errorf("Expected color '#00ff00', got %s", result.Color)
	}
}

func TestAPITags_Delete(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tags-delete")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tag := env.CreateTag(tu.Ledger.ID, "To Delete", "#ff0000")

	resp := client.Delete("/api/v1/tags/" + tag.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Deleted bool `json:"deleted"`
	}
	ParseJSON(t, resp, &result)

	if !result.Deleted {
		t.Error("Expected deleted to be true")
	}

	// Verify it's gone
	resp = client.Get("/api/v1/tags/" + tag.ID.String())
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestAPITags_CrossUserIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	user1 := env.CreateTestUser("tags-iso-1")
	defer env.CleanupTestUser(user1)
	user2 := env.CreateTestUser("tags-iso-2")
	defer env.CleanupTestUser(user2)

	client1 := env.NewAPIClient(user1)
	client2 := env.NewAPIClient(user2)

	// User 1 creates a tag
	tag := env.CreateTag(user1.Ledger.ID, "User1 Tag", "#ff0000")

	// User 2 should not be able to access it
	resp := client2.Get("/api/v1/tags/" + tag.ID.String())
	AssertStatus(t, resp, http.StatusNotFound)

	// User 2 should not be able to update it
	resp = client2.Put("/api/v1/tags/"+tag.ID.String(), map[string]any{
		"name": "Hacked",
	})
	AssertStatus(t, resp, http.StatusNotFound)

	// User 2 should not be able to delete it
	resp = client2.Delete("/api/v1/tags/" + tag.ID.String())
	AssertStatus(t, resp, http.StatusNotFound)

	// User 1 should still be able to access it
	resp = client1.Get("/api/v1/tags/" + tag.ID.String())
	AssertStatus(t, resp, http.StatusOK)
}

func TestAPITags_UniqueNamePerLedger(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tags-unique")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create first tag
	resp := client.Post("/api/v1/tags", map[string]any{
		"name":  "Duplicate",
		"color": "#ff0000",
	})
	AssertStatus(t, resp, http.StatusCreated)

	// Try to create duplicate (should fail due to unique constraint)
	resp = client.Post("/api/v1/tags", map[string]any{
		"name":  "Duplicate",
		"color": "#00ff00",
	})
	// Should fail with server error due to unique constraint
	if resp.StatusCode != http.StatusInternalServerError && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected error for duplicate tag name, got status %d", resp.StatusCode)
	}
}

func TestAPITags_PreventCircularParent(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("tags-circular")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	tag := env.CreateTag(tu.Ledger.ID, "Self Reference", "#ff0000")

	// Try to set parent to itself
	resp := client.Put("/api/v1/tags/"+tag.ID.String(), map[string]any{
		"parent_id": tag.ID.String(),
	})
	AssertStatus(t, resp, http.StatusBadRequest)
}
