package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/pulse"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Pulse renders the forward-looking financial dashboard
// This is the hero page showing "Left to Spend", upcoming bills, spending pace, etc.
func (hdl *Handlers) Pulse(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get quick account balances for the summary
	totalAssets, totalLiabilities, err := hdl.getQuickBalances(r, ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	netWorth := totalAssets - totalLiabilities

	// Calculate left to spend
	calc := pulse.NewCalculator(hdl.db.Pool)
	ctx := r.Context()
	leftToSpend, err := calc.CalculateLeftToSpend(ctx, ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get upcoming bills
	upcomingBills, err := calc.GetUpcomingBills(ctx, ledger.ID, leftToSpend.AvailableBalance)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate spending pace
	spendingPace, err := calc.CalculateSpendingPace(ctx, ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	page := hdl.renderPulse(user.Email, user.ID.String(), totalAssets, totalLiabilities, netWorth, leftToSpend, upcomingBills, spendingPace)

	renderHTML(w, page)
}

func (hdl *Handlers) renderPulse(userEmail, posthogDistinctID string, totalAssets, totalLiabilities, netWorth int64, leftToSpend *pulse.LeftToSpendResult, upcomingBills []pulse.UpcomingBill, spendingPace *pulse.SpendingPace) g.Node {
	return layouts.AppLayout("Pulse", userEmail, posthogDistinctID,
		// Page header
		shadcn.PageHeader("Pulse", "Your financial snapshot"),

		// Quick stats row
		h.Div(
			h.Class("grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8"),
			shadcn.Stat(shadcn.StatProps{Label: "Net Worth", Value: formatMoney(netWorth), Trend: "Current", Positive: netWorth >= 0}),
			shadcn.Stat(shadcn.StatProps{Label: "Assets", Value: formatMoney(totalAssets), Trend: "Total", Positive: true}),
			shadcn.Stat(shadcn.StatProps{Label: "Liabilities", Value: formatMoney(totalLiabilities), Trend: "Total", Positive: false}),
			// Left to Spend
			hdl.renderLeftToSpend(leftToSpend),
		),

		// Two column layout for main content
		h.Div(
			h.Class("grid grid-cols-1 lg:grid-cols-2 gap-6"),

			// Left column - Upcoming Bills
			hdl.renderUpcomingBills(upcomingBills),

			// Right column - Spending Pace
			hdl.renderSpendingPace(spendingPace),
		),
	)
}

// getQuickBalances returns total assets and liabilities for quick display
func (hdl *Handlers) getQuickBalances(r *http.Request, ledgerID interface{}) (totalAssets, totalLiabilities int64, err error) {
	ctx := r.Context()

	rows, err := hdl.db.Pool.Query(ctx, `
		SELECT a.type, COALESCE(SUM(e.amount_cents), 0) as balance
		FROM accounts a
		LEFT JOIN entries e ON a.id = e.account_id
		WHERE a.ledger_id = $1 AND a.type IN ('asset', 'liability')
		GROUP BY a.type
	`, ledgerID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var accountType string
		var balance int64
		if err := rows.Scan(&accountType, &balance); err != nil {
			return 0, 0, err
		}
		if accountType == "asset" {
			totalAssets = balance
		} else if accountType == "liability" {
			totalLiabilities = balance
		}
	}

	return totalAssets, totalLiabilities, rows.Err()
}

// renderLeftToSpend renders the Left to Spend hero component
func (hdl *Handlers) renderLeftToSpend(result *pulse.LeftToSpendResult) g.Node {
	isPositive := result.LeftToSpend >= 0
	colorClass := "text-chart-2"
	if !isPositive {
		colorClass = "text-destructive"
	}

	// Calculate progress percentage (cap at 100%)
	progressPercent := float64(0)
	if result.AvailableBalance > 0 {
		progressPercent = float64(result.LeftToSpend) / float64(result.AvailableBalance) * 100
		if progressPercent < 0 {
			progressPercent = 0
		}
		if progressPercent > 100 {
			progressPercent = 100
		}
	}

	return h.Div(
		h.Class("bg-card border border-border rounded-xl p-6"),
		h.Div(
			h.Class("flex items-center justify-between mb-4"),
			h.Div(
				h.P(h.Class("text-sm text-muted-foreground mb-1"), g.Text("Left to Spend")),
				h.P(h.Class("text-2xl font-bold "+colorClass), g.Text(formatMoney(result.LeftToSpend))),
			),
			h.Div(
				h.Class("w-10 h-10 rounded-full bg-primary/20 flex items-center justify-center text-primary"),
				g.Raw(`<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>`),
			),
		),
		// Progress bar
		h.Div(
			h.Class("w-full bg-secondary rounded-full h-2 mb-3"),
			h.Div(
				h.Class("h-2 rounded-full transition-all duration-300"),
				h.Style("width: "+formatFloat(progressPercent)+"%; background-color: "+getProgressColor(progressPercent)+";"),
			),
		),
		// Summary text
		h.P(h.Class("text-xs text-muted-foreground"),
			g.Text(formatMoney(result.AvailableBalance)+" available · "+formatMoney(result.UpcomingBills)+" in upcoming bills"),
		),
	)
}

// renderUpcomingBills renders the upcoming bills list component
func (hdl *Handlers) renderUpcomingBills(bills []pulse.UpcomingBill) g.Node {
	if len(bills) == 0 {
		return shadcn.Card(shadcn.CardProps{},
			shadcn.CardHeaderActions("Upcoming Bills", ""),
			shadcn.CardContentFull(
				h.Div(h.Class("text-center"),
					h.Div(
						h.Class("w-16 h-16 mx-auto mb-4 rounded-full bg-secondary flex items-center justify-center text-muted-foreground"),
						g.Raw(`<svg class="w-8 h-8" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>`),
					),
					h.H3(h.Class("text-lg font-medium text-foreground mb-2"), g.Text("No Upcoming Bills")),
					h.P(h.Class("text-sm text-muted-foreground max-w-sm mx-auto"),
						g.Text("We'll automatically detect your recurring bills and show them here once we find them."),
					),
				),
			),
		)
	}

	var billItems []g.Node
	for _, bill := range bills {
		statusClass := "bg-chart-2/20 text-chart-2"
		statusText := "COVERED"
		if !bill.IsCovered {
			statusClass = "bg-destructive/20 text-destructive"
			statusText = "SHORT"
		}

		// Amount prefix for variable bills (lower confidence = less certain amount)
		amountPrefix := ""
		if bill.Confidence < 80 {
			amountPrefix = "~"
		}

		billItems = append(billItems,
			h.Div(
				h.Class("flex items-center justify-between p-4 border-b border-border last:border-0"),
				h.Div(
					h.Class("flex items-center gap-3 flex-1"),
					// Logo or placeholder
					h.Div(
						h.Class("w-10 h-10 rounded-lg bg-secondary flex items-center justify-center flex-none"),
						g.If(bill.EntityLogo != "",
							h.Img(h.Src(hdl.getLogoURL(bill.EntityLogo)), h.Alt(bill.EntityName), h.Class("w-full h-full object-cover rounded-lg")),
						),
						g.If(bill.EntityLogo == "",
							h.Span(h.Class("text-muted-foreground text-lg"), g.Text(getInitials(bill.EntityName))),
						),
					),
					// Bill info
					h.Div(
						h.Class("flex-1 min-w-0"),
						h.P(h.Class("text-sm font-medium text-foreground truncate"), g.Text(bill.EntityName)),
						h.Span(h.Class("text-xs text-muted-foreground"), g.Attr("data-date", bill.ExpectedDate.Format("2006-01-02"))),
					),
					// Amount and status
					h.Div(
						h.Class("text-right"),
						h.P(h.Class("text-sm font-medium text-foreground"), g.Text(amountPrefix+formatMoney(bill.ExpectedAmount))),
						h.Span(
							h.Class("inline-flex items-center px-2 py-0.5 rounded text-xs font-medium "+statusClass),
							g.Text(statusText),
						),
					),
				),
			),
		)
	}

	var billNodes []g.Node
	billNodes = append(billNodes, billItems...)

	return shadcn.Card(shadcn.CardProps{},
		shadcn.CardHeaderActions("Upcoming Bills", ""),
		shadcn.CardContent(
			h.Div(h.Class("divide-y divide-border"), g.Group(billNodes)),
		),
	)
}

// renderSpendingPace renders the spending pace line chart
func (hdl *Handlers) renderSpendingPace(pace *pulse.SpendingPace) g.Node {
	// Early in month (day 1-2) - still show what we have
	if !pace.HasEnoughData {
		return shadcn.Card(shadcn.CardProps{},
			shadcn.CardHeaderActions("Spending Pace", ""),
			shadcn.CardContentFull(
				h.Div(
					h.Class("flex items-center justify-between"),
					h.Div(
						h.P(h.Class("text-2xl font-bold text-foreground"), g.Text(formatMoney(pace.CurrentMonthSpent))),
						h.P(h.Class("text-sm text-muted-foreground"), g.Text("spent so far")),
					),
					h.Div(
						h.Class("text-right"),
						h.P(h.Class("text-sm text-muted-foreground"), g.Textf("Day %d", pace.DayOfMonth)),
					),
				),
			),
		)
	}

	// If no last month data, show current month spending with a simple visualization
	if !pace.HasLastMonth {
		// Build current month data only
		currentMonthData := make([]float64, pace.DayOfMonth)
		for i := 0; i < pace.DayOfMonth && i < len(pace.CurrentMonthDaily); i++ {
			currentMonthData[i] = float64(pace.CurrentMonthDaily[i].Cumulative) / 100
		}

		var labels []string
		for i := 1; i <= pace.DayOfMonth; i++ {
			if i == 1 || i%5 == 0 || i == pace.DayOfMonth {
				labels = append(labels, fmt.Sprintf("%d", i))
			} else {
				labels = append(labels, "")
			}
		}

		// Y-axis formatter
		formatYAxis := func(v float64) string {
			if v >= 1000 {
				return fmt.Sprintf("$%.0fk", v/1000)
			}
			return fmt.Sprintf("$%.0f", v)
		}

		return shadcn.Card(shadcn.CardProps{},
			shadcn.CardHeaderActions("Spending Pace", ""),
			shadcn.CardContentFull(
				// Summary
				h.Div(
					h.Class("flex items-center justify-between mb-6"),
					h.Div(
						h.P(h.Class("text-2xl font-bold text-foreground"), g.Text(formatMoney(pace.CurrentMonthSpent))),
						h.P(h.Class("text-sm text-muted-foreground"), g.Textf("spent in %s so far", pace.CurrentMonthName)),
					),
					h.Div(
						h.Class("text-right"),
						h.P(h.Class("text-sm text-muted-foreground"), g.Textf("Day %d of %d", pace.DayOfMonth, pace.DaysInMonth)),
					),
				),
				// Single line chart for current month
				h.Div(
					h.Class("mb-6"),
					shadcn.LineChart(shadcn.LineChartProps{
						ChartProps: shadcn.ChartProps{
							Width:  520,
							Height: 220,
						},
						Labels: labels,
						Series: []shadcn.ChartSeries{
							{
								Name:  "This month",
								Color: "#6366f1",
								Data:  currentMonthData,
							},
						},
						ShowGrid:    true,
						ShowYAxis:   true,
						YAxisFormat: formatYAxis,
					}),
				),
			),
		)
	}

	// Build chart data - show full last month trajectory
	// Last month: show full month so user can see the complete trajectory they're heading toward
	lastMonthData := make([]float64, len(pace.LastMonthDaily))
	for i, d := range pace.LastMonthDaily {
		lastMonthData[i] = float64(d.Cumulative) / 100 // Convert cents to dollars
	}

	// Current month: only up to today, padded to match last month length for chart alignment
	// We pad with -1 which we'll handle specially (won't be drawn)
	chartDays := len(lastMonthData)
	currentMonthData := make([]float64, chartDays)
	for i := 0; i < chartDays; i++ {
		if i < pace.DayOfMonth && i < len(pace.CurrentMonthDaily) {
			currentMonthData[i] = float64(pace.CurrentMonthDaily[i].Cumulative) / 100
		} else {
			// Future days - use last known value to show "current position"
			// This creates a flat line from today to end of month showing "you are here"
			if pace.DayOfMonth > 0 && pace.DayOfMonth-1 < len(pace.CurrentMonthDaily) {
				currentMonthData[i] = float64(pace.CurrentMonthDaily[pace.DayOfMonth-1].Cumulative) / 100
			}
		}
	}

	// Build labels (every 5 days + last day)
	var labels []string
	for i := 1; i <= chartDays; i++ {
		if i == 1 || i%5 == 0 || i == chartDays {
			labels = append(labels, fmt.Sprintf("%d", i))
		} else {
			labels = append(labels, "")
		}
	}

	// Status styling
	statusIcon := "✓"
	statusColor := "text-chart-2"
	switch pace.Status {
	case "faster":
		statusIcon = "↑"
		statusColor = "text-ring"
	case "slower":
		statusIcon = "↓"
		statusColor = "text-chart-2"
	}

	// Y-axis formatter for dollar amounts
	formatYAxis := func(v float64) string {
		if v >= 1000 {
			return fmt.Sprintf("$%.0fk", v/1000)
		}
		return fmt.Sprintf("$%.0f", v)
	}

	return shadcn.Card(shadcn.CardProps{},
		shadcn.CardHeaderActions("Spending Pace", ""),
		shadcn.CardContentFull(
			// Summary stats
			h.Div(
				h.Class("flex items-center justify-between mb-6"),
				h.Div(
					h.P(h.Class("text-2xl font-bold text-foreground"), g.Text(formatMoney(pace.CurrentMonthSpent))),
					h.P(h.Class("text-sm text-muted-foreground"), g.Textf("spent in %s so far", pace.CurrentMonthName)),
				),
				h.Div(
					h.Class("text-right"),
					h.Div(
						h.Class("flex items-center gap-2 justify-end"),
						h.Span(h.Class("text-lg "+statusColor), g.Text(statusIcon)),
						h.Span(h.Class("text-lg font-medium "+statusColor),
							g.If(pace.Status == "on_track", g.Text("On track")),
							g.If(pace.Status == "faster", g.Textf("%d%% faster", pace.PercentChange)),
							g.If(pace.Status == "slower", g.Textf("%d%% slower", pace.PercentChange)),
						),
					),
					h.P(h.Class("text-sm text-muted-foreground"), g.Textf("vs %s", pace.LastMonthName)),
				),
			),

			// Line chart
			h.Div(
				h.Class("mb-6"),
				shadcn.LineChart(shadcn.LineChartProps{
					ChartProps: shadcn.ChartProps{
						Width:  520,
						Height: 220,
					},
					Labels: labels,
					Series: []shadcn.ChartSeries{
						{
							Name:  "This month",
							Color: "#6366f1", // indigo
							Data:  currentMonthData,
						},
						{
							Name:  "Last month",
							Color: "var(--color-muted-foreground)", // muted-foreground
							Data:  lastMonthData,
						},
					},
					ShowGrid:    true,
					ShowLegend:  true,
					ShowDots:    false,
					ShowYAxis:   true,
					YAxisFormat: formatYAxis,
				}),
			),

			// Comparison stats
			h.Div(
				h.Class("pt-4 border-t border-border"),
				h.Div(
					h.Class("flex items-center justify-between text-sm"),
					h.Span(h.Class("text-muted-foreground"), g.Textf("%s total", pace.LastMonthName)),
					h.Span(h.Class("text-foreground"), g.Text(formatMoney(pace.LastMonthTotal))),
				),
			),

			// Top spending this month
			g.If(len(pace.DebugTopTransactions) > 0,
				h.Div(
					h.Class("mt-4 pt-4 border-t border-border"),
					h.P(h.Class("text-xs text-muted-foreground mb-3"), g.Text("Largest expenses")),
					h.Div(
						h.Class("space-y-2"),
						g.Group(g.Map(pace.DebugTopTransactions, func(t pulse.DebugTransaction) g.Node {
							return h.Div(
								h.Class("flex items-center justify-between text-sm"),
								h.Div(
									h.Class("flex-1 min-w-0"),
									h.P(h.Class("text-foreground truncate"), g.Text(t.Description)),
									h.P(h.Class("text-xs text-muted-foreground"), g.Text(t.Date)),
								),
								h.P(h.Class("text-foreground font-mono ml-2"), g.Text(formatMoney(t.AmountCents))),
							)
						})),
					),
				),
			),
		),
	)
}

// Helper functions
func formatFloat(f float64) string {
	return fmt.Sprintf("%.1f", f)
}

func getProgressColor(percent float64) string {
	if percent >= 50 {
		return "#10b981" // green-500
	} else if percent >= 25 {
		return "#f59e0b" // amber-500
	}
	return "#ef4444" // red-500
}

func getInitials(name string) string {
	if name == "" {
		return "?"
	}
	words := strings.Fields(name)
	if len(words) == 0 {
		return strings.ToUpper(string(name[0]))
	}
	if len(words) == 1 {
		return strings.ToUpper(string(name[0]))
	}
	return strings.ToUpper(string(words[0][0]) + string(words[len(words)-1][0]))
}
