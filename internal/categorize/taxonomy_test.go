package categorize

import (
	"strings"
	"testing"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// =============================================
// GetCategoryTreeForPrompt Tests
// =============================================

func TestGetCategoryTreeForPrompt_Empty(t *testing.T) {
	result := GetCategoryTreeForPrompt(nil)
	if result != "" {
		t.Errorf("Expected empty string for nil tags, got %q", result)
	}

	result = GetCategoryTreeForPrompt([]*models.Tag{})
	if result != "" {
		t.Errorf("Expected empty string for empty tags, got %q", result)
	}
}

func TestGetCategoryTreeForPrompt_RootOnly(t *testing.T) {
	tags := []*models.Tag{
		{ID: uuid.New(), Name: "Food & Drink", ParentID: nil},
		{ID: uuid.New(), Name: "Transportation", ParentID: nil},
	}

	result := GetCategoryTreeForPrompt(tags)

	if result == "" {
		t.Error("Expected non-empty result")
	}

	lower := strings.ToLower(result)
	if !strings.Contains(lower, "food") {
		t.Errorf("Result should contain 'Food & Drink': %q", result)
	}
	if !strings.Contains(lower, "transportation") {
		t.Errorf("Result should contain 'Transportation': %q", result)
	}
}

func TestGetCategoryTreeForPrompt_WithChildren(t *testing.T) {
	parentID := uuid.New()
	tags := []*models.Tag{
		{ID: parentID, Name: "Food & Drink", ParentID: nil},
		{ID: uuid.New(), Name: "Groceries", ParentID: &parentID},
		{ID: uuid.New(), Name: "Restaurants", ParentID: &parentID},
	}

	result := GetCategoryTreeForPrompt(tags)

	if result == "" {
		t.Error("Expected non-empty result")
	}

	t.Logf("Category tree output:\n%s", result)
}

// =============================================
// DefaultTaxonomy Validation Tests
// =============================================

func TestDefaultTaxonomy_HasExpectedCategories(t *testing.T) {
	expectedCategories := []string{
		"Income",
		"Food & Drink",
		"Transportation",
		"Shopping",
		"Entertainment",
		"Travel",
		"Healthcare",
		"Home & Utilities",
		"Bank Fees",
		"Transfers",
	}

	for _, expected := range expectedCategories {
		found := false
		for _, cat := range DefaultTaxonomy {
			if cat.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultTaxonomy missing expected category: %q", expected)
		}
	}
}

func TestDefaultTaxonomy_AllHaveColors(t *testing.T) {
	for _, cat := range DefaultTaxonomy {
		if cat.Color == "" {
			t.Errorf("Category %q has no color", cat.Name)
		}
		if cat.Color[0] != '#' {
			t.Errorf("Category %q color should start with #: %q", cat.Name, cat.Color)
		}
	}
}

func TestDefaultTaxonomy_AllSubcategoriesHaveAliases(t *testing.T) {
	for _, cat := range DefaultTaxonomy {
		for _, sub := range cat.Subcategories {
			if len(sub.Aliases) == 0 {
				t.Errorf("Subcategory %q in %q has no aliases", sub.Name, cat.Name)
			}
		}
	}
}
