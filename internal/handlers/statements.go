package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	"github.com/google/uuid"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// DateRange represents a date range for reporting
type DateRange struct {
	Start time.Time
	End   time.Time
	Label string
	Key   string
}

func getDateRangePresets() []DateRange {
	now := time.Now()
	currentYear := now.Year()
	currentMonth := now.Month()

	// First day of current month
	thisMonthStart := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, time.Local)
	// Last day of current month
	thisMonthEnd := thisMonthStart.AddDate(0, 1, -1)

	// First day of last month
	lastMonthStart := thisMonthStart.AddDate(0, -1, 0)
	lastMonthEnd := thisMonthStart.AddDate(0, 0, -1)

	// Last 3 months
	last3MonthsStart := thisMonthStart.AddDate(0, -2, 0)

	// This year
	thisYearStart := time.Date(currentYear, 1, 1, 0, 0, 0, 0, time.Local)
	thisYearEnd := time.Date(currentYear, 12, 31, 23, 59, 59, 0, time.Local)

	// Last year
	lastYearStart := time.Date(currentYear-1, 1, 1, 0, 0, 0, 0, time.Local)
	lastYearEnd := time.Date(currentYear-1, 12, 31, 23, 59, 59, 0, time.Local)

	return []DateRange{
		{Start: thisMonthStart, End: thisMonthEnd, Label: "This Month", Key: "this_month"},
		{Start: lastMonthStart, End: lastMonthEnd, Label: "Last Month", Key: "last_month"},
		{Start: last3MonthsStart, End: thisMonthEnd, Label: "Last 3 Months", Key: "last_3_months"},
		{Start: thisYearStart, End: thisYearEnd, Label: "This Year", Key: "this_year"},
		{Start: lastYearStart, End: lastYearEnd, Label: "Last Year", Key: "last_year"},
	}
}

func parseDateRange(r *http.Request) DateRange {
	rangeKey := r.URL.Query().Get("range")
	if rangeKey == "" {
		rangeKey = "this_month"
	}

	presets := getDateRangePresets()

	// Handle custom range
	if rangeKey == "custom" {
		startStr := r.URL.Query().Get("start")
		endStr := r.URL.Query().Get("end")

		start, err1 := time.Parse("2006-01-02", startStr)
		end, err2 := time.Parse("2006-01-02", endStr)

		if err1 == nil && err2 == nil {
			return DateRange{Start: start, End: end, Label: "Custom", Key: "custom"}
		}
		// Fall back to this month if custom is invalid
		rangeKey = "this_month"
	}

	// Find matching preset
	for _, preset := range presets {
		if preset.Key == rangeKey {
			return preset
		}
	}

	// Default to this month
	return presets[0]
}

// AccountBalance holds account info with balance
type AccountBalance struct {
	ID                 uuid.UUID
	Name               string
	Type               models.AccountType
	Balance            int64
	InstitutionID      string
	InstitutionName    string
	InstitutionLogoURL string
}

// TagTotal holds aggregated totals by tag
type TagTotal struct {
	TagID    *uuid.UUID
	ParentID *uuid.UUID
	TagName  string
	Color    string
	Total    int64
	Children []TagTotal
}

// Statements renders the financial statements page (Balance Sheet + P&L)
// This was previously called "Intelligence" but renamed for clarity
func (hdl *Handlers) Statements(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse date range
	dateRange := parseDateRange(r)

	// Get position data (balance sheet as of end date)
	assets, liabilities, totalAssets, totalLiabilities, err := hdl.getPositionData(r, ledger.ID, dateRange.End)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	netWorth := totalAssets - totalLiabilities

	// Get performance data (P&L for date range)
	incomeByTag, expensesByTag, totalIncome, totalExpenses, err := hdl.getPerformanceData(r, ledger.ID, dateRange.Start, dateRange.End)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	netIncome := totalIncome - totalExpenses

	page := hdl.renderStatements(
		user.Email,
		user.ID.String(),
		dateRange,
		getDateRangePresets(),
		assets, liabilities, totalAssets, totalLiabilities, netWorth,
		incomeByTag, expensesByTag, totalIncome, totalExpenses, netIncome,
	)

	renderHTML(w, page)
}

func (hdl *Handlers) renderStatements(
	userEmail string,
	posthogDistinctID string,
	dateRange DateRange,
	presets []DateRange,
	assets []AccountBalance,
	liabilities []AccountBalance,
	totalAssets, totalLiabilities, netWorth int64,
	incomeByTag []TagTotal,
	expensesByTag []TagTotal,
	totalIncome, totalExpenses, netIncome int64,
) g.Node {
	return layouts.AppLayout("Statements", userEmail, posthogDistinctID,
		// Page header with date range selector
		shadcn.PageHeader("Statements", "Balance sheet and profit & loss reports",
			renderDateRangeDropdown(dateRange, presets),
		),

		// Overview stats
		h.Div(
			h.Class("grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8"),
			shadcn.Stat(shadcn.StatProps{Label: "Income", Value: formatMoney(totalIncome), Trend: dateRange.Label, Positive: true}),
			shadcn.Stat(shadcn.StatProps{Label: "Expenses", Value: formatMoney(totalExpenses), Trend: dateRange.Label, Positive: false}),
			shadcn.Stat(shadcn.StatProps{Label: "Net Income", Value: formatMoneyWithSign(netIncome), Trend: "This period", Positive: netIncome >= 0}),
			shadcn.Stat(shadcn.StatProps{Label: "Net Worth", Value: formatMoney(netWorth), Trend: "Total position", Positive: netWorth >= 0}),
		),

		// Position (Balance Sheet) Section
		h.Div(
			h.Class("mb-8"),
			h.Div(
				h.Class("flex items-center justify-between mb-4"),
				h.H2(h.Class("text-xl font-semibold text-foreground"), g.Text("Position (Balance Sheet)")),
				h.Span(h.Class("text-sm text-muted-foreground"), g.Text("as of "+dateRange.End.Format("Jan 2, 2006"))),
			),
			// Assets and Liabilities columns
			h.Div(
				h.Class("grid grid-cols-1 md:grid-cols-2 gap-6"),
				// Assets card
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Assets", formatMoney(totalAssets)),
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							g.If(len(assets) == 0,
								shadcn.EmptyNoData("No asset accounts", "", nil),
							),
							g.Group(g.Map(assets, func(a AccountBalance) g.Node {
								return hdl.renderAccountBalanceRow(a)
							})),
						),
					),
				),
				// Liabilities card
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Liabilities", formatMoney(totalLiabilities)),
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							g.If(len(liabilities) == 0,
								shadcn.EmptyNoData("No liability accounts", "", nil),
							),
							g.Group(g.Map(liabilities, func(a AccountBalance) g.Node {
								return hdl.renderAccountBalanceRow(a)
							})),
						),
					),
				),
			),
		),

		// Performance (P&L) Section
		h.Div(
			h.Div(
				h.Class("flex items-center justify-between mb-4"),
				h.H2(h.Class("text-xl font-semibold text-foreground"), g.Text("Performance (P&L)")),
				h.Span(h.Class("text-sm text-muted-foreground"), g.Text(dateRange.Start.Format("Jan 2")+" - "+dateRange.End.Format("Jan 2, 2006"))),
			),
			// Income and Expenses columns
			h.Div(
				h.Class("grid grid-cols-1 md:grid-cols-2 gap-6"),
				// Income card
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Income", formatMoney(totalIncome)),
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							g.If(len(incomeByTag) == 0,
								shadcn.EmptyNoData("No income in this period", "", nil),
							),
							g.Group(g.Map(incomeByTag, func(t TagTotal) g.Node {
								return renderTagTotalTree(t, totalIncome, dateRange, 0)
							})),
						),
					),
				),
				// Expenses card
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Expenses", formatMoney(totalExpenses)),
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							g.If(len(expensesByTag) == 0,
								shadcn.EmptyNoData("No expenses in this period", "", nil),
							),
							g.Group(g.Map(expensesByTag, func(t TagTotal) g.Node {
								return renderTagTotalTree(t, totalExpenses, dateRange, 0)
							})),
						),
					),
				),
			),
		),
	)
}

// getPositionData returns account balances as of a specific date
func (hdl *Handlers) getPositionData(r *http.Request, ledgerID uuid.UUID, asOf time.Time) (
	assets []AccountBalance,
	liabilities []AccountBalance,
	totalAssets int64,
	totalLiabilities int64,
	err error,
) {
	ctx := r.Context()

	// Query accounts with balances as of the specified date
	rows, err := hdl.db.Pool.Query(ctx, `
		SELECT a.id, a.name, a.type, COALESCE(a.institution_id, '') as institution_id, 
		       COALESCE(a.institution_name, '') as institution_name,
		       COALESCE(a.institution_logo_url, '') as institution_logo_url,
		       COALESCE(SUM(e.amount_cents), 0) as balance
		FROM accounts a
		LEFT JOIN entries e ON a.id = e.account_id
		LEFT JOIN transactions t ON e.transaction_id = t.id AND t.date <= $2
		WHERE a.ledger_id = $1 AND a.type IN ('asset', 'liability')
		GROUP BY a.id, a.name, a.type, a.institution_id, a.institution_name, a.institution_logo_url
		ORDER BY a.type, a.name
	`, ledgerID, asOf)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var ab AccountBalance
		if err := rows.Scan(&ab.ID, &ab.Name, &ab.Type, &ab.InstitutionID, &ab.InstitutionName, &ab.InstitutionLogoURL, &ab.Balance); err != nil {
			return nil, nil, 0, 0, err
		}

		if ab.Type == models.AccountTypeAsset {
			assets = append(assets, ab)
			totalAssets += ab.Balance
		} else if ab.Type == models.AccountTypeLiability {
			liabilities = append(liabilities, ab)
			totalLiabilities += ab.Balance
		}
	}

	return assets, liabilities, totalAssets, totalLiabilities, rows.Err()
}

// getPerformanceData returns income/expense totals by tag for a date range
func (hdl *Handlers) getPerformanceData(r *http.Request, ledgerID uuid.UUID, startDate, endDate time.Time) (
	incomeByTag []TagTotal,
	expensesByTag []TagTotal,
	totalIncome int64,
	totalExpenses int64,
	err error,
) {
	ctx := r.Context()

	// First, get the full tag hierarchy so we can build a proper tree
	allTags, err := hdl.tags.GetByLedgerID(ctx, ledgerID)
	if err != nil {
		return nil, nil, 0, 0, err
	}

	// Build tag lookup maps
	tagByID := make(map[string]*models.Tag)
	for _, t := range allTags {
		tagByID[t.ID.String()] = t
	}

	// Query income/expense entries grouped by tag
	rows, err := hdl.db.Pool.Query(ctx, `
		SELECT 
			tg.id as tag_id, 
			tg.parent_id,
			COALESCE(tg.name, 'Uncategorized') as tag_name, 
			COALESCE(tg.color, '#6b7280') as color,
			a.type as account_type, 
			SUM(e.amount_cents) as total
		FROM entries e
		JOIN transactions t ON e.transaction_id = t.id
		JOIN accounts a ON e.account_id = a.id
		LEFT JOIN transaction_tags tt ON t.id = tt.transaction_id
		LEFT JOIN tags tg ON tt.tag_id = tg.id
		WHERE t.ledger_id = $1 
			AND t.date >= $2 AND t.date <= $3
			AND a.type IN ('income', 'expense')
			AND a.name NOT IN ('Internal Transfer', 'Opening Balance')
			AND t.is_transfer = false
			AND COALESCE(tg.exclude_from_pnl, false) = false
		GROUP BY tg.id, tg.parent_id, tg.name, tg.color, a.type
		ORDER BY ABS(SUM(e.amount_cents)) DESC
	`, ledgerID, startDate, endDate)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	defer rows.Close()

	incomeMap := make(map[string]*TagTotal)
	expenseMap := make(map[string]*TagTotal)

	for rows.Next() {
		var tagID, parentID *uuid.UUID
		var tagName, color string
		var accountType models.AccountType
		var total int64

		if err := rows.Scan(&tagID, &parentID, &tagName, &color, &accountType, &total); err != nil {
			return nil, nil, 0, 0, err
		}

		tt := &TagTotal{
			TagID:    tagID,
			ParentID: parentID,
			TagName:  tagName,
			Color:    color,
			Total:    total,
		}

		key := "uncategorized"
		if tagID != nil {
			key = tagID.String()
		}

		if accountType == models.AccountTypeIncome {
			tt.Total = -total
			if existing, ok := incomeMap[key]; ok {
				existing.Total += tt.Total
			} else {
				incomeMap[key] = tt
			}
			totalIncome += tt.Total
		} else if accountType == models.AccountTypeExpense {
			if existing, ok := expenseMap[key]; ok {
				existing.Total += tt.Total
			} else {
				expenseMap[key] = tt
			}
			totalExpenses += total
		}
	}

	incomeByTag = buildTagTree(incomeMap, tagByID)
	expensesByTag = buildTagTree(expenseMap, tagByID)

	return incomeByTag, expensesByTag, totalIncome, totalExpenses, rows.Err()
}

// buildTagTree converts a flat map of tags into a hierarchical tree
func buildTagTree(tagMap map[string]*TagTotal, allTags map[string]*models.Tag) []TagTotal {
	if len(tagMap) == 0 {
		return nil
	}

	// Create parent tags that don't have direct transactions but have children that do
	for _, tt := range tagMap {
		if tt.ParentID != nil {
			parentKey := tt.ParentID.String()
			if _, exists := tagMap[parentKey]; !exists {
				if parentTag, ok := allTags[parentKey]; ok {
					tagMap[parentKey] = &TagTotal{
						TagID:    &parentTag.ID,
						ParentID: parentTag.ParentID,
						TagName:  parentTag.Name,
						Color:    parentTag.Color,
						Total:    0,
					}
				}
			}
		}
	}

	// Recursively create missing grandparents
	for {
		added := false
		for _, tt := range tagMap {
			if tt.ParentID != nil {
				parentKey := tt.ParentID.String()
				if _, exists := tagMap[parentKey]; !exists {
					if parentTag, ok := allTags[parentKey]; ok {
						tagMap[parentKey] = &TagTotal{
							TagID:    &parentTag.ID,
							ParentID: parentTag.ParentID,
							TagName:  parentTag.Name,
							Color:    parentTag.Color,
							Total:    0,
						}
						added = true
					}
				}
			}
		}
		if !added {
			break
		}
	}

	// Attach children to parents
	for key, tt := range tagMap {
		if tt.ParentID != nil {
			parentKey := tt.ParentID.String()
			if parent, ok := tagMap[parentKey]; ok {
				child := *tt
				parent.Children = append(parent.Children, child)
			}
		}
		tagMap[key] = tt
	}

	// Collect roots (tags with no parent)
	var roots []TagTotal
	for _, tt := range tagMap {
		if tt.ParentID == nil {
			roots = append(roots, *tt)
		}
	}

	// Roll up children totals to parents
	for i := range roots {
		rollUpTotals(&roots[i])
	}

	// Sort roots by total descending
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].Total < roots[j].Total
	})

	// Sort children within each root
	for i := range roots {
		sortTagChildren(&roots[i])
	}

	// Filter out roots with no total
	var filteredRoots []TagTotal
	for _, root := range roots {
		if root.Total != 0 {
			filteredRoots = append(filteredRoots, root)
		}
	}

	return filteredRoots
}

func sortTagChildren(t *TagTotal) {
	if len(t.Children) > 0 {
		sort.Slice(t.Children, func(i, j int) bool {
			return t.Children[i].Total < t.Children[j].Total
		})
		for i := range t.Children {
			sortTagChildren(&t.Children[i])
		}
	}
}

func rollUpTotals(t *TagTotal) int64 {
	total := t.Total
	for i := range t.Children {
		total += rollUpTotals(&t.Children[i])
	}
	t.Total = total
	return total
}

func (hdl *Handlers) renderAccountBalanceRow(a AccountBalance) g.Node {
	displayBalance := a.Balance
	colorClass := "text-foreground"

	if a.Type == models.AccountTypeAsset {
		if a.Balance >= 0 {
			colorClass = "amount-positive"
		} else {
			colorClass = "amount-negative"
		}
	} else if a.Type == models.AccountTypeLiability {
		displayBalance = -a.Balance
		if a.Balance > 0 {
			colorClass = "amount-negative"
		} else {
			colorClass = "amount-positive"
		}
	}

	logoURL := hdl.getInstitutionLogoURLFull(a.InstitutionLogoURL)

	return h.A(
		h.Href("/accounts/"+a.ID.String()),
		h.Class("flex items-center gap-3 p-4 hover:bg-accent transition-colors cursor-pointer"),
		func() g.Node {
			if logoURL != "" {
				return h.Img(
					h.Src(logoURL),
					h.Alt(a.Name),
					h.Class("w-8 h-8 rounded-lg object-contain flex-none bg-white"),
				)
			}
			return h.Div(
				h.Class("w-8 h-8 rounded-lg bg-secondary flex items-center justify-center text-muted-foreground flex-none"),
				layouts.IconWallet(),
			)
		}(),
		h.Span(h.Class("flex-1 text-foreground truncate"), g.Text(a.Name)),
		h.Span(h.Class("font-number font-medium "+colorClass), g.Text(formatMoney(displayBalance))),
	)
}

func (hdl *Handlers) getInstitutionLogoURLFull(storedLogoURL string) string {
	if storedLogoURL != "" {
		if strings.HasPrefix(storedLogoURL, "http://") || strings.HasPrefix(storedLogoURL, "https://") {
			return storedLogoURL
		}
		return hdl.getLogoURL(storedLogoURL)
	}
	// Do not fall back to provider-hosted logos — many URLs 404; returning ""
	// lets UI components show a local fallback icon instead.
	return ""
}

// renderTagTotalTree renders a tag and its children as a collapsible tree
// Uses native <details> element for JS-free expand/collapse
func renderTagTotalTree(t TagTotal, total int64, dateRange DateRange, depth int) g.Node {
	hasChildren := len(t.Children) > 0
	tagID := "tag-" + t.TagName

	if hasChildren && depth == 0 {
		// Use native <details> for collapsible section
		return h.Details(
			h.Class("group"),
			h.ID(tagID),
			// Summary is the clickable header
			g.El("summary",
				h.Class("flex items-center p-4 hover:bg-accent transition-colors cursor-pointer list-none [&::-webkit-details-marker]:hidden"),
				// Chevron icon rotates when open
				h.Span(
					h.Class("w-5 h-5 mr-2 flex items-center justify-center text-muted-foreground group-hover:text-foreground transition-all flex-none"),
					g.Raw(`<svg class="w-4 h-4 transition-transform group-open:rotate-90" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"></path></svg>`),
				),
				h.Div(
					h.Class("w-3 h-3 rounded-full mr-3 flex-none"),
					h.Style("background-color: "+t.Color),
				),
				renderTagLink(t, total, dateRange, false),
			),
			// Children container - shown when details is open
			h.Div(
				h.ID(tagID+"-children"),
				g.Group(g.Map(t.Children, func(child TagTotal) g.Node {
					return renderTagTotalTree(child, total, dateRange, depth+1)
				})),
			),
		)
	}

	if depth == 0 {
		percentage := float64(0)
		if total > 0 {
			percentage = float64(t.Total) / float64(total) * 100
		}
		startDate := dateRange.Start.Format("2006-01-02")
		endDate := dateRange.End.Format("2006-01-02")
		var href string
		if t.TagID != nil {
			href = "/transactions?tag=" + t.TagID.String() + "&start=" + startDate + "&end=" + endDate
		} else {
			href = "/transactions?filter=uncategorized&start=" + startDate + "&end=" + endDate
		}

		return h.A(
			h.Href(href),
			h.Class("flex items-center p-4 hover:bg-accent transition-colors cursor-pointer"),
			h.Div(h.Class("w-5 mr-2 flex-none")),
			h.Div(
				h.Class("w-3 h-3 rounded-full mr-3 flex-none"),
				h.Style("background-color: "+t.Color),
			),
			h.Div(
				h.Class("flex-1 min-w-0"),
				h.Div(
					h.Class("flex items-center justify-between mb-1"),
					h.Span(h.Class("text-foreground truncate"), g.Text(t.TagName)),
					h.Span(h.Class("text-muted-foreground text-sm ml-2"), g.Textf("%.1f%%", percentage)),
				),
				h.Div(
					h.Class("h-1.5 bg-secondary rounded-full overflow-hidden"),
					h.Div(
						h.Class("h-full rounded-full"),
						h.Style("width: "+formatPercentage(percentage)+"%; background-color: "+t.Color),
					),
				),
			),
			h.Span(h.Class("font-number font-medium text-foreground ml-4"), g.Text(formatMoney(t.Total))),
		)
	}

	return renderTagTotalRow(t, total, dateRange, depth)
}

func renderTagLink(t TagTotal, total int64, dateRange DateRange, isChild bool) g.Node {
	percentage := float64(0)
	if total > 0 {
		percentage = float64(t.Total) / float64(total) * 100
	}

	startDate := dateRange.Start.Format("2006-01-02")
	endDate := dateRange.End.Format("2006-01-02")

	var href string
	if t.TagID != nil {
		href = "/transactions?tag=" + t.TagID.String() + "&start=" + startDate + "&end=" + endDate
	} else {
		href = "/transactions?filter=uncategorized&start=" + startDate + "&end=" + endDate
	}

	return h.A(
		h.Href(href),
		h.Class("flex-1 flex items-center min-w-0"),
		h.Div(
			h.Class("flex-1 min-w-0"),
			h.Div(
				h.Class("flex items-center justify-between mb-1"),
				h.Span(
					g.If(isChild, h.Class("text-muted-foreground truncate")),
					g.If(!isChild, h.Class("text-foreground truncate")),
					g.Text(t.TagName),
				),
				h.Span(h.Class("text-muted-foreground text-sm ml-2"), g.Textf("%.1f%%", percentage)),
			),
			h.Div(
				h.Class("h-1.5 bg-secondary rounded-full overflow-hidden"),
				h.Div(
					h.Class("h-full rounded-full"),
					h.Style("width: "+formatPercentage(percentage)+"%; background-color: "+t.Color),
				),
			),
		),
		h.Span(
			g.If(isChild, h.Class("font-number font-medium text-muted-foreground ml-4")),
			g.If(!isChild, h.Class("font-number font-medium text-foreground ml-4")),
			g.Text(formatMoney(t.Total)),
		),
	)
}

func renderTagTotalRow(t TagTotal, total int64, dateRange DateRange, depth int) g.Node {
	percentage := float64(0)
	if total > 0 {
		percentage = float64(t.Total) / float64(total) * 100
	}

	startDate := dateRange.Start.Format("2006-01-02")
	endDate := dateRange.End.Format("2006-01-02")

	var href string
	if t.TagID != nil {
		href = "/transactions?tag=" + t.TagID.String() + "&start=" + startDate + "&end=" + endDate
	} else {
		href = "/transactions?filter=uncategorized&start=" + startDate + "&end=" + endDate
	}

	isChild := depth > 0

	return h.A(
		h.Href(href),
		h.Class("flex items-center p-4 hover:bg-accent transition-colors cursor-pointer"),
		g.If(isChild,
			h.Div(
				h.Class("flex-none w-8"),
			),
		),
		g.If(!isChild,
			h.Div(
				h.Class("w-3 h-3 rounded-full mr-3 flex-none"),
				h.Style("background-color: "+t.Color),
			),
		),
		h.Div(
			h.Class("flex-1 min-w-0"),
			h.Div(
				h.Class("flex items-center justify-between mb-1"),
				h.Span(
					g.If(isChild, h.Class("text-muted-foreground truncate text-sm")),
					g.If(!isChild, h.Class("text-foreground truncate")),
					g.Text(t.TagName),
				),
				h.Span(h.Class("text-muted-foreground text-sm ml-2"), g.Textf("%.1f%%", percentage)),
			),
			h.Div(
				h.Class("h-1.5 bg-secondary rounded-full overflow-hidden"),
				h.Div(
					h.Class("h-full rounded-full"),
					h.Style("width: "+formatPercentage(percentage)+"%; background-color: "+t.Color),
				),
			),
		),
		h.Span(
			g.If(isChild, h.Class("font-number font-medium text-muted-foreground ml-4 text-sm")),
			g.If(!isChild, h.Class("font-number font-medium text-foreground ml-4")),
			g.Text(formatMoney(t.Total)),
		),
	)
}

func formatPercentage(p float64) string {
	if p > 100 {
		return "100"
	}
	if p < 0 {
		return "0"
	}
	return itoa64(int64(p))
}

// renderDateRangeDropdown renders a ShadCN-style dropdown for date range selection
func renderDateRangeDropdown(current DateRange, presets []DateRange) g.Node {
	return h.Div(
		h.Class("relative"),
		h.ID("date-range-dropdown"),
		h.Button(
			h.Type("button"),
			h.Class("flex h-9 items-center justify-between gap-2 rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground shadow-sm hover:bg-accent focus:outline-none focus:ring-1 focus:ring-ring"),
			g.Attr("onclick", "toggleDateDropdown()"),
			h.Span(g.Text(current.Label)),
			g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="opacity-50"><path d="m6 9 6 6 6-6"/></svg>`),
		),
		h.Div(
			h.ID("date-dropdown-menu"),
			h.Class("absolute right-0 top-full mt-1 z-50 min-w-[8rem] rounded-md border border-border bg-card p-1 shadow-md hidden"),
			g.Group(g.Map(presets, func(p DateRange) g.Node {
				isSelected := p.Key == current.Key
				return h.A(
					h.Href("?range="+p.Key),
					h.Class("relative flex w-full cursor-default select-none items-center rounded-sm py-1.5 pl-2 pr-8 text-sm outline-none hover:bg-accent hover:text-accent-foreground "+
						func() string {
							if isSelected {
								return "bg-accent text-accent-foreground"
							}
							return "text-muted-foreground"
						}()),
					g.Text(p.Label),
					g.If(isSelected,
						h.Span(
							h.Class("absolute right-2 flex h-3.5 w-3.5 items-center justify-center"),
							g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>`),
						),
					),
				)
			})),
		),
	)
}
