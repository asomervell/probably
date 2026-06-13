package providers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/asomervell/probably/internal/models"
)

func buildReportSystemPrompt() string {
	return `You are a personal finance analyst providing insights on financial data. Your task is to analyze 
spending patterns, identify trends, and provide actionable recommendations.

RESPONSE FORMAT (JSON only):
{
  "summary": "A 2-3 sentence natural language summary of the period",
  "highlights": ["Key observation 1", "Key observation 2", ...],
  "recommendations": ["Actionable recommendation 1", "Actionable recommendation 2", ...],
  "comparison_notes": "How this period compares to the previous period (if data provided)",
  "key_insights": [
    {"content": "Insight text", "importance": 1-10, "is_key": true/false, "type": "trend|spending_alert|recommendation|anomaly"}
  ]
}

GUIDELINES:
- Be specific with numbers and percentages
- Focus on actionable insights, not obvious observations
- Importance scale: 1-3 (minor), 4-6 (notable), 7-9 (important), 10 (critical)
- Mark is_key=true for insights that deserve dashboard prominence
- Types: "trend" (patterns), "spending_alert" (unusual spending), "recommendation" (advice), "anomaly" (outliers)
- If comparing to prior period, note significant changes (>10% different)
- Keep recommendations practical and specific`
}

func buildReportUserPrompt(req ReportRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Generate a %s financial report for the period %s to %s.\n\n",
		req.ReportType, req.PeriodStart.Format("Jan 2, 2006"), req.PeriodEnd.Format("Jan 2, 2006")))

	sb.WriteString("=== PERIOD SUMMARY ===\n")
	sb.WriteString(fmt.Sprintf("Total Income: %s\n", models.FormatCents(req.TotalIncome)))
	sb.WriteString(fmt.Sprintf("Total Expenses: %s\n", models.FormatCents(req.TotalExpenses)))
	sb.WriteString(fmt.Sprintf("Net Savings: %s\n\n", models.FormatCents(req.NetSavings)))

	if len(req.Accounts) > 0 {
		sb.WriteString("=== ACCOUNTS ===\n")
		for _, acc := range req.Accounts {
			sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", acc.Name, acc.Type, models.FormatCents(acc.BalanceCents)))
		}
		sb.WriteString("\n")
	}

	if len(req.CategoryTotals) > 0 {
		sb.WriteString("=== SPENDING BY CATEGORY ===\n")
		for cat, amount := range req.CategoryTotals {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", cat, models.FormatCents(amount)))
		}
		sb.WriteString("\n")
	}

	if len(req.EntityTotals) > 0 {
		sb.WriteString("=== TOP ENTITIES ===\n")
		for i, e := range req.EntityTotals {
			if i >= 10 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s: %s (%d transactions)\n", e.EntityName, models.FormatCents(e.AmountCents), e.Count))
		}
		sb.WriteString("\n")
	}

	if len(req.Transactions) > 0 {
		sb.WriteString("=== TRANSACTIONS (sample) ===\n")
		limit := 50
		if len(req.Transactions) < limit {
			limit = len(req.Transactions)
		}
		for i := 0; i < limit; i++ {
			t := req.Transactions[i]
			title := t.DisplayTitle
			if title == "" {
				title = t.Description
			}
			sb.WriteString(fmt.Sprintf("- %s | %s | %s | %s\n",
				t.Date.Format("Jan 2"), title, models.FormatCents(t.AmountCents), t.CategoryName))
		}
		if len(req.Transactions) > limit {
			sb.WriteString(fmt.Sprintf("... and %d more transactions\n", len(req.Transactions)-limit))
		}
		sb.WriteString("\n")
	}

	if req.PriorPeriod != nil {
		sb.WriteString("=== PRIOR PERIOD COMPARISON ===\n")
		sb.WriteString(fmt.Sprintf("Previous Income: %s\n", models.FormatCents(req.PriorPeriod.TotalIncome)))
		sb.WriteString(fmt.Sprintf("Previous Expenses: %s\n", models.FormatCents(req.PriorPeriod.TotalExpenses)))
		sb.WriteString(fmt.Sprintf("Previous Net: %s\n\n", models.FormatCents(req.PriorPeriod.NetSavings)))
	}

	sb.WriteString("Please analyze this data and provide insights in JSON format.")

	return sb.String()
}

func buildTransactionSystemPrompt() string {
	return `You are a personal finance assistant analyzing a new transaction in the context of recent spending.
Determine if this transaction is noteworthy enough to alert the user.

RESPONSE FORMAT (JSON only):
{
  "content": "Brief insight about this transaction (1-2 sentences)",
  "importance": 1-10,
  "is_key": true/false,
  "type": "transaction|spending_alert|trend|anomaly"
}

GUIDELINES:
- Most routine transactions should have importance 1-3 and is_key=false
- Only flag as is_key=true for genuinely unusual or important transactions
- Types: "transaction" (normal), "spending_alert" (over budget), "trend" (pattern), "anomaly" (unusual)
- Consider: amount relative to typical spending, category patterns, entity frequency
- Don't over-alert - users should only see truly actionable insights`
}

func buildTransactionUserPrompt(req TransactionRequest) string {
	var sb strings.Builder

	t := req.Transaction
	title := t.DisplayTitle
	if title == "" {
		title = t.Description
	}

	sb.WriteString("=== NEW TRANSACTION ===\n")
	sb.WriteString(fmt.Sprintf("Date: %s\n", t.Date.Format("Jan 2, 2006")))
	sb.WriteString(fmt.Sprintf("Description: %s\n", title))
	sb.WriteString(fmt.Sprintf("Amount: %s\n", models.FormatCents(t.AmountCents)))
	sb.WriteString(fmt.Sprintf("Category: %s\n", t.CategoryName))
	sb.WriteString(fmt.Sprintf("Entity: %s\n", t.EntityName))
	sb.WriteString(fmt.Sprintf("Account: %s\n\n", t.AccountName))

	if req.MonthToDate != nil {
		sb.WriteString("=== MONTH TO DATE ===\n")
		sb.WriteString(fmt.Sprintf("Income: %s\n", models.FormatCents(req.MonthToDate.TotalIncome)))
		sb.WriteString(fmt.Sprintf("Expenses: %s\n", models.FormatCents(req.MonthToDate.TotalExpenses)))
		if catSpend, ok := req.MonthToDate.ByCategory[t.CategoryName]; ok {
			sb.WriteString(fmt.Sprintf("This category MTD: %s\n", models.FormatCents(catSpend)))
		}
		sb.WriteString("\n")
	}

	if req.LastMonthSummary != nil {
		sb.WriteString("=== LAST MONTH ===\n")
		sb.WriteString(fmt.Sprintf("Total Expenses: %s\n", models.FormatCents(req.LastMonthSummary.TotalExpenses)))
		if catSpend, ok := req.LastMonthSummary.ByCategory[t.CategoryName]; ok {
			sb.WriteString(fmt.Sprintf("This category last month: %s\n", models.FormatCents(catSpend)))
		}
		sb.WriteString("\n")
	}

	sameEntityCount := 0
	var sameEntityTotal int64
	for _, rt := range req.RecentTransactions {
		if rt.EntityName == t.EntityName && rt.EntityName != "" {
			sameEntityCount++
			sameEntityTotal += rt.AmountCents
		}
	}
	if sameEntityCount > 0 {
		sb.WriteString(fmt.Sprintf("Recent at %s: %d transactions totaling %s\n\n",
			t.EntityName, sameEntityCount, models.FormatCents(sameEntityTotal)))
	}

	sb.WriteString("Analyze if this transaction is noteworthy. Respond with JSON.")

	return sb.String()
}

func parseReportResponse(content string) (*ReportResponse, error) {
	content = strings.TrimSpace(content)

	if start := strings.Index(content, "{"); start != -1 {
		if end := strings.LastIndex(content, "}"); end > start {
			content = content[start : end+1]
		}
	}

	var resp struct {
		Summary         string            `json:"summary"`
		Highlights      []string          `json:"highlights"`
		Recommendations []string          `json:"recommendations"`
		ComparisonNotes string            `json:"comparison_notes"`
		KeyInsights     []InsightResponse `json:"key_insights"`
	}

	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil, fmt.Errorf("parse report: %w", err)
	}

	return &ReportResponse{
		Summary:         resp.Summary,
		Highlights:      resp.Highlights,
		Recommendations: resp.Recommendations,
		ComparisonNotes: resp.ComparisonNotes,
		KeyInsights:     resp.KeyInsights,
	}, nil
}

func parseTransactionInsight(content string) (*TransactionInsight, error) {
	content = strings.TrimSpace(content)

	if start := strings.Index(content, "{"); start != -1 {
		if end := strings.LastIndex(content, "}"); end > start {
			content = content[start : end+1]
		}
	}

	var insight TransactionInsight
	if err := json.Unmarshal([]byte(content), &insight); err != nil {
		return nil, fmt.Errorf("parse insight: %w", err)
	}

	if insight.Importance < 1 {
		insight.Importance = 1
	}
	if insight.Importance > 10 {
		insight.Importance = 10
	}

	return &insight, nil
}

