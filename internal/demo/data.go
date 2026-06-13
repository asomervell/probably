package demo

import (
	"math/rand"
	"time"

	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/pulse"
	"github.com/google/uuid"
)

// GeneratePulseData generates realistic mock pulse dashboard data
func GeneratePulseData() (*pulse.LeftToSpendResult, []pulse.UpcomingBill, *pulse.SpendingPace) {
	now := time.Now()
	dayOfMonth := now.Day()
	daysInMonth := getDaysInMonth(now)

	// Left to Spend
	availableBalance := int64(rand.Intn(500000) + 200000) // $2,000 - $7,000
	upcomingBillsTotal := int64(rand.Intn(200000) + 50000) // $500 - $2,500
	leftToSpend := availableBalance - upcomingBillsTotal

	leftToSpendResult := &pulse.LeftToSpendResult{
		AvailableBalance: availableBalance,
		UpcomingBills:    upcomingBillsTotal,
		LeftToSpend:      leftToSpend,
	}

	// Upcoming Bills - dates relative to today
	bills := []pulse.UpcomingBill{
		{
			EntityName:     "Netflix",
			EntityLogo:     "netflix.png",
			ExpectedDate:   now.AddDate(0, 0, 5), // 5 days from today
			ExpectedAmount: 1599, // $15.99
			IsCovered:      leftToSpend >= 1599,
			Frequency:      "monthly",
			Confidence:     95,
		},
		{
			EntityName:     "Spotify",
			EntityLogo:     "spotify.png",
			ExpectedDate:   now.AddDate(0, 0, 12), // 12 days from today
			ExpectedAmount: 1099, // $10.99
			IsCovered:      leftToSpend >= 1099,
			Frequency:      "monthly",
			Confidence:     95,
		},
		{
			EntityName:     "Rent",
			EntityLogo:     "",
			ExpectedDate:   now.AddDate(0, 0, 3), // 3 days from today
			ExpectedAmount: 120000, // $1,200
			IsCovered:      leftToSpend >= 120000,
			Frequency:      "monthly",
			Confidence:     100,
		},
		{
			EntityName:     "AWS",
			EntityLogo:     "aws.png",
			ExpectedDate:   now.AddDate(0, 0, 8), // 8 days from today
			ExpectedAmount: 4500, // $45.00
			IsCovered:      leftToSpend >= 4500,
			Frequency:      "monthly",
			Confidence:     85,
		},
	}

	// Spending Pace
	currentMonthSpent := int64(rand.Intn(300000) + 100000) // $1,000 - $4,000
	lastMonthTotal := currentMonthSpent + int64(rand.Intn(200000)-100000) // ±$1,000 variation

	// Build daily cumulative data
	currentMonthDaily := make([]pulse.DailySpending, dayOfMonth)
	var cumulative int64 = 0
	for i := 0; i < dayOfMonth; i++ {
		// Simulate daily spending (more on weekends)
		day := i + 1
		dailyAmount := int64(rand.Intn(5000) + 2000) // $20 - $70 per day
		if day%7 == 0 || day%7 == 6 { // Weekend
			dailyAmount = int64(rand.Intn(10000) + 5000) // $50 - $150 on weekends
		}
		cumulative += dailyAmount
		currentMonthDaily[i] = pulse.DailySpending{
			Day:        day,
			Cumulative: cumulative,
		}
	}
	
	// Normalize cumulative to match currentMonthSpent
	if cumulative > 0 {
		scaleFactor := float64(currentMonthSpent) / float64(cumulative)
		for i := range currentMonthDaily {
			currentMonthDaily[i].Cumulative = int64(float64(currentMonthDaily[i].Cumulative) * scaleFactor)
		}
	}

	// Last month daily data (full month)
	lastMonthDays := getDaysInMonth(now.AddDate(0, -1, 0))
	lastMonthDaily := make([]pulse.DailySpending, lastMonthDays)
	cumulative = 0
	for i := 0; i < lastMonthDays; i++ {
		dailyAmount := int64(rand.Intn(5000) + 2000)
		if (i+1)%7 == 0 || (i+1)%7 == 6 {
			dailyAmount = int64(rand.Intn(10000) + 5000)
		}
		cumulative += dailyAmount
		lastMonthDaily[i] = pulse.DailySpending{
			Day:        i + 1,
			Cumulative: cumulative,
		}
	}
	
	// Normalize last month cumulative to match lastMonthTotal
	if cumulative > 0 {
		scaleFactor := float64(lastMonthTotal) / float64(cumulative)
		for i := range lastMonthDaily {
			lastMonthDaily[i].Cumulative = int64(float64(lastMonthDaily[i].Cumulative) * scaleFactor)
		}
	}

	// Calculate pace status
	pacePercentage := float64(currentMonthSpent) / float64(lastMonthTotal)
	status := "on_track"
	percentChange := 0
	if pacePercentage > 1.10 {
		status = "faster"
		percentChange = int((pacePercentage - 1.0) * 100)
	} else if pacePercentage < 0.90 {
		status = "slower"
		percentChange = int((1.0 - pacePercentage) * 100)
	}

	spendingPace := &pulse.SpendingPace{
		CurrentMonthSpent:     currentMonthSpent,
		DayOfMonth:            dayOfMonth,
		DaysInMonth:           getDaysInMonth(now),
		PercentOfMonthElapsed: float64(dayOfMonth) / float64(daysInMonth),
		CurrentMonthName:      now.Format("January"),
		LastMonthName:         now.AddDate(0, -1, 0).Format("January"),
		LastMonthSamePoint:    int64(float64(lastMonthTotal) * float64(dayOfMonth) / float64(lastMonthDays)),
		LastMonthTotal:        lastMonthTotal,
		PacePercentage:        pacePercentage,
		ProjectedMonthEnd:     int64(float64(currentMonthSpent) / float64(dayOfMonth) * float64(daysInMonth)),
		Status:                status,
		StatusMessage:         getStatusMessage(status, percentChange),
		PercentChange:         percentChange,
		HasEnoughData:         dayOfMonth >= 3,
		HasLastMonth:          true,
		CurrentMonthDaily:     currentMonthDaily,
		LastMonthDaily:        lastMonthDaily,
		DebugTopTransactions: []pulse.DebugTransaction{
			{
				Date:        now.AddDate(0, 0, -2).Format("Jan 2"), // 2 days ago (relative)
				Description: "Whole Foods Market",
				AmountCents: 12500,
				AccountName: "Checking",
				AccountType: "asset",
				IsTransfer:  false,
			},
			{
				Date:        now.AddDate(0, 0, -5).Format("Jan 2"), // 5 days ago (relative)
				Description: "Amazon",
				AmountCents: 8999,
				AccountName: "Checking",
				AccountType: "asset",
				IsTransfer:  false,
			},
		},
	}

	return leftToSpendResult, bills, spendingPace
}

// GenerateTransactions generates realistic mock transactions
func GenerateTransactions() []*models.Transaction {
	now := time.Now()
	transactions := []*models.Transaction{}

	merchants := []struct {
		name     string
		category string
		logo     string
		amounts  []int64
	}{
		{"Austin Roasters", "Coffee & Tea", "austin-roasters.png", []int64{450, 550, 475}},
		{"Amazon", "Shopping", "amazon.png", []int64{2499, 8999, 1599, 3499}},
		{"Uber", "Transportation", "uber.png", []int64{1842, 2234, 1520}},
		{"Spotify", "Entertainment", "spotify.png", []int64{1099}},
		{"Whole Foods", "Groceries", "whole-foods.png", []int64{12500, 9800, 15200}},
		{"Shell", "Gas", "shell.png", []int64{4500, 5200}},
		{"Netflix", "Entertainment", "netflix.png", []int64{1599}},
		{"Starbucks", "Coffee & Tea", "starbucks.png", []int64{650, 750, 550}},
	}

	// Generate transactions over last 30 days (relative to today)
	for i := 0; i < 30; i++ {
		// Use negative offset to go back in time from today
		date := now.AddDate(0, 0, -i)
		// Skip some days to make it more realistic
		if rand.Float32() < 0.3 {
			continue
		}

		// 1-3 transactions per day
		numTxns := rand.Intn(3) + 1
		for j := 0; j < numTxns; j++ {
			merchant := merchants[rand.Intn(len(merchants))]
			amount := merchant.amounts[rand.Intn(len(merchant.amounts))]

			// Create a shared account ID for all transactions (demo account)
			accountID := uuid.New()
			
			txn := &models.Transaction{
				ID:          uuid.New(),
				Date:        date,
				Description: merchant.name,
				DisplayTitle: merchant.name,
				CreatedAt:   date,
				UpdatedAt:   date,
				Entries: []*models.Entry{
					{
						ID:          uuid.New(),
						AccountID:   accountID,
						AmountCents: -amount, // Expense (negative)
						Currency:    "USD",
						AccountName: "Checking",
						AccountType: models.AccountTypeAsset,
					},
				},
				Tags: []*models.Tag{
					{
						ID:   uuid.New(),
						Name: merchant.category,
					},
				},
			}

			// Add entity with logo URL
			entityID := uuid.New()
			txn.EntityID = &entityID
			txn.Entity = &models.Entity{
				ID:      entityID,
				Name:    merchant.name,
				LogoURL: merchant.logo, // This will be used by getLogoURL to construct CDN URL
			}

			transactions = append(transactions, txn)
		}
	}

	return transactions
}

// GenerateChatMessages generates mock chat conversation
func GenerateChatMessages() []struct {
	Role    string
	Content string
} {
	now := time.Now()
	// Use relative date for the example transaction (5 days ago)
	exampleDate := now.AddDate(0, 0, -5)
	exampleDateStr := exampleDate.Format("January 2")
	
	return []struct {
		Role    string
		Content string
	}{
		{
			Role:    "user",
			Content: "How much did I spend on groceries this month?",
		},
		{
			Role:    "assistant",
			Content: "You've spent **$387.50** on groceries this month so far. That's across 12 transactions, with your largest purchase being $125.00 at Whole Foods on " + exampleDateStr + ".\n\nCompared to last month, you're spending about 8% more on groceries. Your average grocery transaction is $32.29.",
		},
		{
			Role:    "user",
			Content: "What are my top spending categories?",
		},
		{
			Role:    "assistant",
			Content: "Here are your top spending categories this month:\n\n1. **Groceries**: $387.50 (28% of total)\n2. **Transportation**: $245.20 (18%)\n3. **Shopping**: $189.99 (14%)\n4. **Coffee & Tea**: $87.50 (6%)\n5. **Entertainment**: $42.97 (3%)\n\nYour total spending this month is $1,382.16.",
		},
	}
}

// Helper functions
func getDaysInMonth(t time.Time) int {
	firstOfNextMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
	lastDay := firstOfNextMonth.AddDate(0, 0, -1)
	return lastDay.Day()
}

func getStatusMessage(status string, percentChange int) string {
	switch status {
	case "faster":
		return "Spending faster than last month"
	case "slower":
		return "Spending slower than last month"
	default:
		return "On pace with last month"
	}
}
