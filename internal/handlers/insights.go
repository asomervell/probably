package handlers

import (
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// InsightsList shows all insights with filtering
func (hdl *Handlers) InsightsList(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse query parameters
	page := queryPage(r)
	limit := 20
	offset := (page - 1) * limit

	// Build filter
	filter := models.InsightFilter{
		LedgerID: ledger.ID,
		Limit:    limit,
		Offset:   offset,
	}

	// Filter by type if specified
	if typeParam := r.URL.Query().Get("type"); typeParam != "" {
		insightType := models.InsightType(typeParam)
		filter.InsightType = &insightType
	}

	// Filter by minimum importance if specified
	if impParam := r.URL.Query().Get("min_importance"); impParam != "" {
		if imp, err := strconv.Atoi(impParam); err == nil {
			filter.MinImportance = &imp
		}
	}

	// Key insights only
	if r.URL.Query().Get("key_only") == "true" {
		filter.KeyOnly = true
	}

	// Include dismissed
	if r.URL.Query().Get("include_dismissed") == "true" {
		filter.IncludeDismissed = true
	}

	// Get insights
	insights, total, err := hdl.insights.List(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get recent reports
	reports, err := hdl.reports.ListByLedger(r.Context(), ledger.ID, nil, 5)
	if err != nil {
		slog.WarnContext(r.Context(), "failed to fetch reports for insights list", "err", err)
		reports = nil
	}

	totalPages := (total + limit - 1) / limit

	// Build query string for pagination that preserves filters
	paginationQuery := buildPaginationQuery(r.URL.Query(), page)

	pageContent := renderInsightsListWithQuery(user.Email, user.ID.String(), insights, reports, page, totalPages, total, filter, paginationQuery)

	renderHTML(w, pageContent)
}

// InsightsReport shows a specific report with its insights
func (hdl *Handlers) InsightsReport(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reportID, ok := mustParamUUID(w, r, "id", "report ID")
	if !ok {
		return
	}

	// Get report
	report, err := hdl.reports.GetByID(r.Context(), reportID)
	if err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if report.LedgerID != ledger.ID {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	// Get insights for this report
	insights, err := hdl.insights.GetByReport(r.Context(), reportID)
	if err != nil {
		slog.WarnContext(r.Context(), "failed to fetch insights for report", "report_id", reportID, "err", err)
		insights = nil
	}

	pageContent := renderReportDetail(user.Email, user.ID.String(), report, insights)

	renderHTML(w, pageContent)
}

// InsightsDismiss dismisses an insight (HTMX endpoint)
func (hdl *Handlers) InsightsDismiss(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	insightID, ok := mustParamUUID(w, r, "id", "insight ID")
	if !ok {
		return
	}

	// Get insight to verify ownership
	insight, err := hdl.insights.GetByID(r.Context(), insightID)
	if err != nil {
		http.Error(w, "Insight not found", http.StatusNotFound)
		return
	}

	if insight.LedgerID != ledger.ID {
		http.Error(w, "Insight not found", http.StatusNotFound)
		return
	}

	// Dismiss the insight
	if err := hdl.insights.Dismiss(r.Context(), insightID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return empty response for HTMX to remove the element
	w.WriteHeader(http.StatusOK)
}

// buildPaginationQuery builds a query string that preserves existing filters
func buildPaginationQuery(query map[string][]string, currentPage int) string {
	var parts []string
	for key, values := range query {
		if key != "page" && len(values) > 0 {
			parts = append(parts, key+"="+values[0])
		}
	}
	if len(parts) == 0 {
		return ""
	}
	sort.Strings(parts)
	return "&" + strings.Join(parts, "&")
}

func renderInsightsListWithQuery(
	userEmail string,
	posthogDistinctID string,
	insights []*models.Insight,
	reports []*models.Report,
	currentPage, totalPages, total int,
	filter models.InsightFilter,
	paginationQuery string,
) g.Node {
	return layouts.AppLayout("Intelligence", userEmail, posthogDistinctID,
		// Page header
		shadcn.PageHeader("Intelligence", "AI-powered financial insights"),

		// Two column layout
		h.Div(
			h.Class("grid grid-cols-1 lg:grid-cols-3 gap-6"),

			// Main content - Insights list
			h.Div(
				h.Class("lg:col-span-2"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("All Insights", formatInsightCount(total)),
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							g.If(len(insights) == 0,
								h.Div(
									h.Class("p-8 text-center"),
									h.P(h.Class("text-muted-foreground mb-2"), g.Text("No insights yet")),
									h.P(h.Class("text-muted-foreground text-sm"), g.Text("Insights will appear as you add transactions and generate reports.")),
								),
							),
							g.Group(g.Map(insights, func(insight *models.Insight) g.Node {
								return renderInsightListItem(insight)
							})),
						),
						// Pagination with filter preservation
						g.If(totalPages > 1,
							renderPaginationWithQuery(currentPage, totalPages, "/intelligence", paginationQuery),
						),
					),
				),
			),

			// Sidebar - Reports
			h.Div(
				h.Class("lg:col-span-1"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Reports", ""),
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							g.If(len(reports) == 0,
								shadcn.EmptyNoData("No reports generated yet", "Generate a report to see insights.", nil),
							),
							g.Group(g.Map(reports, func(report *models.Report) g.Node {
								return renderReportListItem(report)
							})),
						),
					),
				),
			),
		),
	)
}

func renderInsightListItem(insight *models.Insight) g.Node {
	// Determine icon based on type
	iconClass := "text-primary"
	icon := insightIcon(insight.InsightType)

	switch insight.InsightType {
	case models.InsightTypeSpendingAlert:
		iconClass = "text-ring"
	case models.InsightTypeTrend:
		iconClass = "text-primary"
	case models.InsightTypeRecommendation:
		iconClass = "text-chart-2"
	case models.InsightTypeAnomaly:
		iconClass = "text-destructive"
	}

	return h.Div(
		h.Class("flex items-start gap-4 p-4 hover:bg-accent transition-colors"),
		// Icon
		h.Div(
			h.Class("flex-none mt-1 "+iconClass),
			g.Raw(icon),
		),
		// Content
		h.Div(
			h.Class("flex-1 min-w-0"),
			h.P(h.Class("text-foreground text-sm leading-relaxed"), g.Text(insight.Content)),
			h.Div(
				h.Class("flex items-center gap-3 mt-2"),
				// Type badge
				h.Span(
					h.Class("text-xs px-2 py-0.5 rounded-full bg-secondary text-muted-foreground"),
					g.Text(formatInsightType(insight.InsightType)),
				),
				// Importance badge
				g.If(insight.Importance >= 7,
					h.Span(
						h.Class("text-xs px-2 py-0.5 rounded-full bg-primary/20 text-primary"),
						g.Text("Important"),
					),
				),
				// Key badge
				g.If(insight.IsKey,
					h.Span(
						h.Class("text-xs px-2 py-0.5 rounded-full bg-ring/20 text-ring"),
						g.Text("Key"),
					),
				),
				// Date
				h.Span(
					h.Class("text-xs text-muted-foreground"),
					g.Text(insight.CreatedAt.Format("Jan 2")),
				),
			),
		),
		// Actions
		h.Div(
			h.Class("flex-none"),
			h.Button(
				h.Type("button"),
				h.Class("text-muted-foreground hover:text-foreground p-1"),
				h.Title("Dismiss"),
				g.Attr("hx-post", "/insights/"+insight.ID.String()+"/dismiss"),
				g.Attr("hx-swap", "outerHTML"),
				g.Attr("hx-target", "closest div.flex.items-start"),
				g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`),
			),
		),
	)
}

func renderReportListItem(report *models.Report) g.Node {
	periodLabel := report.PeriodStart.Format("January 2006")
	if report.ReportType == models.ReportTypeQuarterly {
		quarter := (int(report.PeriodStart.Month())-1)/3 + 1
		periodLabel = "Q" + strconv.Itoa(quarter) + " " + strconv.Itoa(report.PeriodStart.Year())
	} else if report.ReportType == models.ReportTypeAnnual {
		periodLabel = strconv.Itoa(report.PeriodStart.Year())
	}

	return h.A(
		h.Href("/insights/reports/"+report.ID.String()),
		h.Class("flex items-center justify-between p-4 hover:bg-accent transition-colors"),
		h.Div(
			h.P(h.Class("text-foreground font-medium"), g.Text(periodLabel)),
			h.P(h.Class("text-muted-foreground text-sm"), g.Text(string(report.ReportType))),
		),
		h.Div(
			h.Class("text-right"),
			h.P(
				h.Class("font-number text-sm "+func() string {
					if report.NetSavingsCents >= 0 {
						return "text-chart-2"
					}
					return "text-destructive"
				}()),
				g.Text(formatMoneyWithSign(report.NetSavingsCents)),
			),
		),
	)
}

func renderReportDetail(userEmail, posthogDistinctID string, report *models.Report, insights []*models.Insight) g.Node {
	periodLabel := report.PeriodStart.Format("January 2006")
	if report.ReportType == models.ReportTypeQuarterly {
		quarter := (int(report.PeriodStart.Month())-1)/3 + 1
		periodLabel = "Q" + strconv.Itoa(quarter) + " " + strconv.Itoa(report.PeriodStart.Year())
	} else if report.ReportType == models.ReportTypeAnnual {
		periodLabel = strconv.Itoa(report.PeriodStart.Year())
	}

	return layouts.AppLayout("Report - "+periodLabel, userEmail, posthogDistinctID,
		// Back link
		h.A(
			h.Href("/intelligence"),
			h.Class("inline-flex items-center text-muted-foreground hover:text-foreground mb-4"),
			g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mr-2"><path d="m15 18-6-6 6-6"/></svg>`),
			g.Text("Back to Intelligence"),
		),

		// Page header
		shadcn.PageHeader(periodLabel+" Report", string(report.ReportType)+" financial report"),

		// Stats row
		h.Div(
			h.Class("grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8"),
			shadcn.Stat(shadcn.StatProps{Label: "Income", Value: formatMoney(report.TotalIncomeCents), Trend: "", Positive: true}),
			shadcn.Stat(shadcn.StatProps{Label: "Expenses", Value: formatMoney(report.TotalExpensesCents), Trend: "", Positive: false}),
			shadcn.Stat(shadcn.StatProps{Label: "Net Savings", Value: formatMoneyWithSign(report.NetSavingsCents), Trend: "", Positive: report.NetSavingsCents >= 0}),
			shadcn.Stat(shadcn.StatProps{Label: "Generated", Value: formatTimeAgo(report.GeneratedAt), Trend: report.LLMProvider, Positive: true}),
		),

		// Summary section
		g.If(report.Summary != "",
			h.Div(
				h.Class("mb-8"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Summary", ""),
					shadcn.CardContent(
						h.P(h.Class("text-foreground leading-relaxed"), g.Text(report.Summary)),
					),
				),
			),
		),

		// Highlights and Recommendations
		h.Div(
			h.Class("grid grid-cols-1 md:grid-cols-2 gap-6 mb-8"),

			// Highlights
			g.If(len(report.Highlights) > 0,
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Highlights", ""),
					shadcn.CardContent(
						h.Ul(
							h.Class("divide-y divide-border"),
							g.Group(g.Map(report.Highlights, func(highlight string) g.Node {
								return h.Li(
									h.Class("p-4 flex items-start gap-3"),
									h.Span(h.Class("text-primary mt-0.5"), g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/></svg>`)),
									h.Span(h.Class("text-foreground text-sm"), g.Text(highlight)),
								)
							})),
						),
					),
				),
			),

			// Recommendations
			g.If(len(report.Recommendations) > 0,
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Recommendations", ""),
					shadcn.CardContent(
						h.Ul(
							h.Class("divide-y divide-border"),
							g.Group(g.Map(report.Recommendations, func(rec string) g.Node {
								return h.Li(
									h.Class("p-4 flex items-start gap-3"),
									h.Span(h.Class("text-chart-2 mt-0.5"), g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12s4.477 10 10 10z"/><path d="m9 12 2 2 4-4"/></svg>`)),
									h.Span(h.Class("text-foreground text-sm"), g.Text(rec)),
								)
							})),
						),
					),
				),
			),
		),

		// Comparison notes
		g.If(report.ComparisonNotes != "",
			h.Div(
				h.Class("mb-8"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Compared to Previous Period", ""),
					shadcn.CardContentFull(
						h.P(h.Class("text-foreground text-sm leading-relaxed"), g.Text(report.ComparisonNotes)),
					),
				),
			),
		),

		// Category breakdown
		g.If(len(report.CategoryBreakdown) > 0,
			h.Div(
				h.Class("mb-8"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Spending by Category", ""),
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							renderCategoryBreakdown(report.CategoryBreakdown),
						),
					),
				),
			),
		),

		// Report insights
		g.If(len(insights) > 0,
			h.Div(
				h.Class("mb-8"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Report Insights", ""),
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							g.Group(g.Map(insights, func(insight *models.Insight) g.Node {
								return renderInsightListItem(insight)
							})),
						),
					),
				),
			),
		),
	)
}

func renderPaginationWithQuery(currentPage, totalPages int, baseURL string, queryParams string) g.Node {
	return h.Div(
		h.Class("flex items-center justify-between p-4 border-t border-border"),
		h.Div(
			h.Class("flex items-center gap-2"),
			g.If(currentPage > 1,
				h.A(
					h.Href(baseURL+"?page="+strconv.Itoa(currentPage-1)+queryParams),
					h.Class("px-3 py-1 text-sm text-muted-foreground hover:text-foreground hover:bg-accent rounded"),
					g.Text("Previous"),
				),
			),
			h.Span(
				h.Class("text-sm text-muted-foreground"),
				g.Textf("Page %d of %d", currentPage, totalPages),
			),
			g.If(currentPage < totalPages,
				h.A(
					h.Href(baseURL+"?page="+strconv.Itoa(currentPage+1)+queryParams),
					h.Class("px-3 py-1 text-sm text-muted-foreground hover:text-foreground hover:bg-accent rounded"),
					g.Text("Next"),
				),
			),
		),
	)
}

func formatInsightCount(total int) string {
	if total == 0 {
		return ""
	}
	if total == 1 {
		return "1 insight"
	}
	return strconv.Itoa(total) + " insights"
}

func insightIcon(t models.InsightType) string {
	switch t {
	case models.InsightTypeSpendingAlert:
		return `<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" x2="12" y1="9" y2="13"/><line x1="12" x2="12.01" y1="17" y2="17"/></svg>`
	case models.InsightTypeTrend:
		return `<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m22 12-4-4v3H3v2h15v3z"/></svg>`
	case models.InsightTypeRecommendation:
		return `<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12s4.477 10 10 10z"/><path d="m9 12 2 2 4-4"/></svg>`
	case models.InsightTypeAnomaly:
		return `<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/></svg>`
	default:
		return `<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83"/></svg>`
	}
}

// formatInsightType returns a human-readable label for insight types
func formatInsightType(t models.InsightType) string {
	switch t {
	case models.InsightTypeTransaction:
		return "Transaction"
	case models.InsightTypeSpendingAlert:
		return "Spending Alert"
	case models.InsightTypeTrend:
		return "Trend"
	case models.InsightTypeRecommendation:
		return "Recommendation"
	case models.InsightTypeAnomaly:
		return "Unusual Activity"
	case models.InsightTypeSummary:
		return "Summary"
	default:
		return string(t)
	}
}

func formatTimeAgo(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatRelativeTime(*t)
}

// renderCategoryBreakdown renders the category breakdown map as a list
func renderCategoryBreakdown(breakdown map[string]int64) g.Node {
	// Sort categories by amount (descending)
	type catAmount struct {
		name   string
		amount int64
	}
	var items []catAmount
	for cat, amount := range breakdown {
		items = append(items, catAmount{cat, amount})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].amount > items[j].amount })

	var nodes []g.Node
	for _, item := range items {
		nodes = append(nodes, h.Div(
			h.Class("flex items-center justify-between p-4"),
			h.Span(h.Class("text-foreground"), g.Text(item.name)),
			h.Span(h.Class("font-number text-foreground"), g.Text(formatMoney(item.amount))),
		))
	}
	return g.Group(nodes)
}
