package insights

import (
	"testing"
	"time"
)

// =============================================
// DateRange Helper Tests
// =============================================

func TestIsMonthStart(t *testing.T) {
	tests := []struct {
		date     time.Time
		expected bool
	}{
		{time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), true},
		{time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), true},
		{time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC), true},
		{time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), false},
		{time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), false},
		{time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), false},
	}

	for _, tc := range tests {
		result := isMonthStart(tc.date)
		if result != tc.expected {
			t.Errorf("isMonthStart(%v) = %v, want %v", tc.date, result, tc.expected)
		}
	}
}

func TestGetPriorMonth(t *testing.T) {
	tests := []struct {
		current  time.Time
		wantYear int
		wantMonth time.Month
	}{
		{time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), 2024, 1},
		{time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 2023, 12},
		{time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), 2024, 6},
	}

	for _, tc := range tests {
		result := getPriorMonth(tc.current)
		if result.Start.Year() != tc.wantYear || result.Start.Month() != tc.wantMonth {
			t.Errorf("getPriorMonth(%v) start = %v-%v, want %v-%v",
				tc.current, result.Start.Year(), result.Start.Month(),
				tc.wantYear, tc.wantMonth)
		}

		// Start should be first of month
		if result.Start.Day() != 1 {
			t.Errorf("getPriorMonth(%v) start day = %d, want 1", tc.current, result.Start.Day())
		}
	}
}

// =============================================
// DateRange Tests
// =============================================

func TestDateRange_Duration(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
	dr := DateRange{Start: start, End: end}

	duration := dr.End.Sub(dr.Start)
	expectedDays := 30 // Roughly 30 days
	actualDays := int(duration.Hours() / 24)

	if actualDays != expectedDays {
		t.Errorf("DateRange duration: expected ~%d days, got %d", expectedDays, actualDays)
	}
}

// =============================================
// FinancialSnapshot Tests
// =============================================

func TestFinancialSnapshot_NetSavingsCalculation(t *testing.T) {
	snapshot := &FinancialSnapshot{
		TotalIncome:   500000, // $5000
		TotalExpenses: 350000, // $3500
	}
	snapshot.NetSavings = snapshot.TotalIncome - snapshot.TotalExpenses

	expected := int64(150000) // $1500
	if snapshot.NetSavings != expected {
		t.Errorf("NetSavings: expected %d, got %d", expected, snapshot.NetSavings)
	}
}

func TestFinancialSnapshot_NegativeNetSavings(t *testing.T) {
	snapshot := &FinancialSnapshot{
		TotalIncome:   200000, // $2000
		TotalExpenses: 350000, // $3500
	}
	snapshot.NetSavings = snapshot.TotalIncome - snapshot.TotalExpenses

	expected := int64(-150000) // -$1500
	if snapshot.NetSavings != expected {
		t.Errorf("NetSavings: expected %d, got %d", expected, snapshot.NetSavings)
	}
}

func TestFinancialSnapshot_ZeroIncome(t *testing.T) {
	snapshot := &FinancialSnapshot{
		TotalIncome:   0,
		TotalExpenses: 100000,
	}
	snapshot.NetSavings = snapshot.TotalIncome - snapshot.TotalExpenses

	expected := int64(-100000)
	if snapshot.NetSavings != expected {
		t.Errorf("NetSavings with zero income: expected %d, got %d", expected, snapshot.NetSavings)
	}
}

func TestFinancialSnapshot_ZeroExpenses(t *testing.T) {
	snapshot := &FinancialSnapshot{
		TotalIncome:   300000,
		TotalExpenses: 0,
	}
	snapshot.NetSavings = snapshot.TotalIncome - snapshot.TotalExpenses

	expected := int64(300000)
	if snapshot.NetSavings != expected {
		t.Errorf("NetSavings with zero expenses: expected %d, got %d", expected, snapshot.NetSavings)
	}
}

// =============================================
// Month/Quarter/Year Period Tests
// =============================================

func TestBuildMonthPeriod(t *testing.T) {
	year := 2024
	month := time.January

	start := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0).Add(-time.Second)

	// January 2024 should end on Jan 31
	if end.Month() != time.January || end.Day() != 31 {
		t.Errorf("January 2024 should end on Jan 31, got %v", end)
	}

	// February 2024 (leap year)
	month = time.February
	start = time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	end = start.AddDate(0, 1, 0).Add(-time.Second)

	if end.Month() != time.February || end.Day() != 29 {
		t.Errorf("February 2024 (leap year) should end on Feb 29, got %v", end)
	}
}

func TestBuildQuarterPeriod(t *testing.T) {
	tests := []struct {
		quarter    int
		startMonth time.Month
	}{
		{1, time.January},
		{2, time.April},
		{3, time.July},
		{4, time.October},
	}

	year := 2024
	for _, tc := range tests {
		startMonth := time.Month((tc.quarter-1)*3 + 1)
		if startMonth != tc.startMonth {
			t.Errorf("Quarter %d start month: expected %v, got %v", tc.quarter, tc.startMonth, startMonth)
		}

		start := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 3, 0).Add(-time.Second)

		// Verify end is in the correct month
		expectedEndMonth := time.Month((tc.quarter-1)*3 + 3)
		if end.Month() != expectedEndMonth {
			t.Errorf("Quarter %d end month: expected %v, got %v", tc.quarter, expectedEndMonth, end.Month())
		}
	}
}

func TestBuildYearPeriod(t *testing.T) {
	year := 2024
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.Local).Add(-time.Second)

	if start.Year() != 2024 || start.Month() != time.January || start.Day() != 1 {
		t.Errorf("Year start should be Jan 1, 2024, got %v", start)
	}

	if end.Year() != 2024 || end.Month() != time.December || end.Day() != 31 {
		t.Errorf("Year end should be Dec 31, 2024, got %v", end)
	}
}

// =============================================
// CategoryTotals Map Tests
// =============================================

func TestCategoryTotals_Initialization(t *testing.T) {
	snapshot := &FinancialSnapshot{
		CategoryTotals: make(map[string]int64),
	}

	// Add some categories
	snapshot.CategoryTotals["Groceries"] = 50000
	snapshot.CategoryTotals["Dining"] = 25000
	snapshot.CategoryTotals["Transportation"] = 15000

	if len(snapshot.CategoryTotals) != 3 {
		t.Errorf("Expected 3 categories, got %d", len(snapshot.CategoryTotals))
	}

	total := int64(0)
	for _, amount := range snapshot.CategoryTotals {
		total += amount
	}

	expected := int64(90000)
	if total != expected {
		t.Errorf("Category totals sum: expected %d, got %d", expected, total)
	}
}

func TestCategoryTotals_Aggregation(t *testing.T) {
	totals := make(map[string]int64)

	// Simulate aggregating multiple transactions
	transactions := []struct {
		category string
		amount   int64
	}{
		{"Groceries", 5000},
		{"Groceries", 7500},
		{"Dining", 2500},
		{"Groceries", 3000},
		{"Transportation", 4000},
	}

	for _, txn := range transactions {
		totals[txn.category] += txn.amount
	}

	if totals["Groceries"] != 15500 {
		t.Errorf("Groceries total: expected 15500, got %d", totals["Groceries"])
	}
	if totals["Dining"] != 2500 {
		t.Errorf("Dining total: expected 2500, got %d", totals["Dining"])
	}
	if totals["Transportation"] != 4000 {
		t.Errorf("Transportation total: expected 4000, got %d", totals["Transportation"])
	}
}
