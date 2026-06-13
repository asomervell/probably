package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
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

// ComboboxOption is a simple data structure for combobox options
type ComboboxOption struct {
	Value string
	Label string
}

func (hdl *Handlers) TransactionsList(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	_, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse filter type (tab) and search for the shell
	filterType := r.URL.Query().Get("filter")
	search := r.URL.Query().Get("search")

	// Check for rematch results in query params
	rematchAuto := r.URL.Query().Get("auto")
	rematchPending := r.URL.Query().Get("pending")
	showRematchResults := r.URL.Query().Get("rematch") == "1"

	// Build query string for HTMX content request
	contentURL := "/transactions/content?" + r.URL.RawQuery

	pageNode := layouts.AppLayout("Transactions", user.Email, user.ID.String(),
		shadcn.PageHeader("Transactions", "View and categorize your transactions",
			h.A(
				h.Href("/transactions/new"),
				shadcn.ButtonAnchor(shadcn.ButtonProps{Variant: shadcn.ButtonDefault},
					"/transactions/new",
					layouts.IconPlus(),
					g.Text("Add Transaction"),
				),
			),
		),

		// Rematch results notification
		g.If(showRematchResults,
			shadcn.Alert(shadcn.AlertProps{Variant: shadcn.AlertSuccess},
				shadcn.AlertDescription(g.Text(fmt.Sprintf("✓ Re-matched transfers: %s auto-linked, %s pending review", rematchAuto, rematchPending))),
			),
		),

		// Filter tabs and search - render immediately
		h.Div(
			h.Class("mb-6 flex flex-col sm:flex-row sm:items-center gap-4"),
			// Tabs placeholder - will be populated by content load
			h.Div(
				h.ID("filter-tabs-placeholder"),
				h.Class("flex items-center gap-1"),
				// Show current tab indicator while loading
				h.Span(h.Class("text-sm text-muted-foreground"), g.Text("Loading...")),
			),
			// Search
			h.Form(
				h.Method("GET"),
				h.Class("flex-1 max-w-md"),
				// Preserve filter type
				g.If(filterType != "",
					h.Input(h.Type("hidden"), h.Name("filter"), h.Value(filterType)),
				),
				h.Div(
					h.Class("relative"),
					shadcn.Input(shadcn.InputProps{
						Type:        "text",
						Name:        "search",
						Placeholder: "Search transactions...",
						Value:       search,
					}),
					h.Div(
						h.Class("absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground"),
						layouts.IconSearch(),
					),
				),
			),
		),

		// Transaction content container - loads via HTMX
		h.Div(
			h.ID("transactions-content"),
			g.Attr("hx-get", contentURL),
			g.Attr("hx-trigger", "load"),
			g.Attr("hx-swap", "innerHTML"),
			// Loading skeleton
			h.Div(
				h.Class("space-y-4"),
				// Skeleton for the transaction list
				shadcn.Card(shadcn.CardProps{Class: "overflow-hidden"},
					h.Div(
						h.Class("divide-y divide-border"),
						// Header skeleton
						h.Div(
							h.Class("flex items-center px-3 sm:px-4 py-2.5 border-b border-border"),
							h.Div(h.Class("h-4 w-24 bg-secondary animate-pulse rounded")),
						),
						// Transaction item skeletons
						g.Group([]g.Node{
							renderTransactionSkeleton(),
							renderTransactionSkeleton(),
							renderTransactionSkeleton(),
							renderTransactionSkeleton(),
							renderTransactionSkeleton(),
							renderTransactionSkeleton(),
							renderTransactionSkeleton(),
							renderTransactionSkeleton(),
						}),
					),
				),
			),
		),
	)

	renderHTML(w, pageNode)
}

// renderTransactionSkeleton renders a loading skeleton for a transaction list item
func renderTransactionSkeleton() g.Node {
	return h.Div(
		h.Class("flex items-center p-3 sm:p-4 gap-3"),
		// Checkbox placeholder
		h.Div(h.Class("w-4 h-4 bg-secondary animate-pulse rounded")),
		// Logo placeholder
		h.Div(h.Class("w-10 h-10 bg-secondary animate-pulse rounded-lg")),
		// Content placeholder
		h.Div(
			h.Class("flex-1 min-w-0 space-y-2"),
			h.Div(h.Class("h-4 w-48 bg-secondary animate-pulse rounded")),
			h.Div(h.Class("h-3 w-32 bg-secondary animate-pulse rounded")),
		),
		// Amount placeholder
		h.Div(h.Class("h-5 w-20 bg-secondary animate-pulse rounded")),
	)
}

// TransactionsListContent returns the transaction list content for HTMX lazy loading
func (hdl *Handlers) TransactionsListContent(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse query params
	page := queryPage(r)
	limit := 50
	offset := (page - 1) * limit

	// Parse filter type (tab)
	filterType := r.URL.Query().Get("filter")

	filter := models.TransactionFilter{
		LedgerID: ledger.ID,
		Limit:    limit,
		Offset:   offset,
		Search:   r.URL.Query().Get("search"),
	}

	// Apply filter based on tab
	switch filterType {
	case "uncategorized":
		filter.Uncategorized = true
	case "needs_review":
		filter.NeedsReview = true
	case "transfers":
		isTransfer := true
		filter.IsTransfer = &isTransfer
	}

	if accID := r.URL.Query().Get("account"); accID != "" {
		if id, err := uuid.Parse(accID); err == nil {
			filter.AccountID = &id
		}
	}

	if tagID := r.URL.Query().Get("tag"); tagID != "" {
		if id, err := uuid.Parse(tagID); err == nil {
			filter.TagID = &id
		}
	}

	// Parse date range filters (used by P&L drilldown)
	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if start, err := time.Parse("2006-01-02", startStr); err == nil {
			filter.StartDate = &start
		}
	}
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		if end, err := time.Parse("2006-01-02", endStr); err == nil {
			filter.EndDate = &end
		}
	}

	transactions, total, err := hdl.transactions.List(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Load entries, tags, and entities
	for _, txn := range transactions {
		if err := hdl.transactions.LoadEntries(r.Context(), txn); err != nil {
			slog.WarnContext(r.Context(), "failed to load entries", "txn_id", txn.ID, "err", err)
		}
		if err := hdl.transactions.LoadTags(r.Context(), txn); err != nil {
			slog.WarnContext(r.Context(), "failed to load tags", "txn_id", txn.ID, "err", err)
		}
		if err := hdl.transactions.LoadEntity(r.Context(), txn, hdl.entities); err != nil {
			slog.WarnContext(r.Context(), "failed to load entity", "txn_id", txn.ID, "err", err)
		}
	}

	// Calculate sum of amounts when searching or filtering
	var sumInflow, sumOutflow int64
	hasSum := filter.Search != "" || filter.AccountID != nil || filter.TagID != nil || filter.StartDate != nil || filter.EndDate != nil
	if hasSum {
		var err error
		if sumInflow, sumOutflow, err = hdl.transactions.SumAmounts(r.Context(), filter); err != nil {
			slog.WarnContext(r.Context(), "failed to sum amounts", "err", err)
		}
	}

	// Get counts for filter tabs
	var allCount, uncatCount, reviewCount int

	allFilter := models.TransactionFilter{LedgerID: ledger.ID, Limit: 0}
	if _, allCount, err = hdl.transactions.List(r.Context(), allFilter); err != nil {
		slog.WarnContext(r.Context(), "failed to count all transactions", "err", err)
	}

	uncatFilter := models.TransactionFilter{LedgerID: ledger.ID, Uncategorized: true, Limit: 0}
	if _, uncatCount, err = hdl.transactions.List(r.Context(), uncatFilter); err != nil {
		slog.WarnContext(r.Context(), "failed to count uncategorized transactions", "err", err)
	}

	reviewFilter := models.TransactionFilter{LedgerID: ledger.ID, NeedsReview: true, Limit: 0}
	if _, reviewCount, err = hdl.transactions.List(r.Context(), reviewFilter); err != nil {
		slog.WarnContext(r.Context(), "failed to count needs-review transactions", "err", err)
	}

	// Get pending transfer matches (for Transfers tab)
	var pendingMatches []*models.PendingTransferMatch
	if filterType == "transfers" {
		var pmErr error
		if pendingMatches, pmErr = hdl.pendingMatches.GetPendingByLedgerID(r.Context(), ledger.ID); pmErr != nil {
			slog.WarnContext(r.Context(), "failed to get pending matches", "err", pmErr)
		}
		// Load transaction details for each match
		for _, match := range pendingMatches {
			if err := hdl.pendingMatches.LoadTransactions(r.Context(), match, hdl.transactions); err != nil {
				slog.WarnContext(r.Context(), "failed to load match transactions", "err", err)
			}
			if match.Transaction != nil {
				if err := hdl.transactions.LoadEntries(r.Context(), match.Transaction); err != nil {
					slog.WarnContext(r.Context(), "failed to load match transaction entries", "err", err)
				}
			}
			if match.CandidateTransaction != nil {
				if err := hdl.transactions.LoadEntries(r.Context(), match.CandidateTransaction); err != nil {
					slog.WarnContext(r.Context(), "failed to load candidate transaction entries", "err", err)
				}
			}
		}
	}

	// Get all tags for bulk tagging dropdown
	allTags, _ := hdl.tags.GetByLedgerID(r.Context(), ledger.ID)

	// Get accounts for display
	accounts, _ := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)

	totalPages := (total + limit - 1) / limit

	// Build query string preserving filters
	buildURL := func(params map[string]string) string {
		base := "/transactions?"
		parts := []string{}
		for k, v := range params {
			if v != "" {
				parts = append(parts, k+"="+v)
			}
		}
		return base + strings.Join(parts, "&")
	}

	// Build filter tabs
	filterTabs := []shadcn.FilterTab{
		{Label: "All", Value: "", Count: allCount, Href: buildURL(map[string]string{"search": filter.Search})},
		{Label: "Uncategorized", Value: "uncategorized", Count: uncatCount, Href: buildURL(map[string]string{"filter": "uncategorized", "search": filter.Search})},
		{Label: "Needs Review", Value: "needs_review", Count: reviewCount, Href: buildURL(map[string]string{"filter": "needs_review", "search": filter.Search})},
		{Label: "Transfers", Value: "transfers", Count: len(pendingMatches), Href: buildURL(map[string]string{"filter": "transfers", "search": filter.Search})},
	}

	contentNode := g.Group([]g.Node{
		// Filter tabs (OOB swap to replace placeholder)
		h.Div(
			h.ID("filter-tabs-placeholder"),
			g.Attr("hx-swap-oob", "outerHTML"),
			shadcn.FilterTabs(filterTabs, filterType),
		),

		// Search results summary with totals
		g.If(hasSum && total > 0,
			shadcn.Card(shadcn.CardProps{Class: "mb-4"},
				shadcn.CardContentFull(
					h.Div(
						h.Class("flex items-center justify-between"),
						h.Div(
							h.Class("text-sm text-muted-foreground"),
							g.Textf("%d transactions found", total),
						),
						h.Div(
							h.Class("flex items-center gap-6 text-sm"),
							g.If(sumInflow > 0,
								h.Div(
									h.Class("flex items-center gap-2"),
									h.Span(h.Class("text-muted-foreground"), g.Text("In:")),
									h.Span(h.Class("text-chart-2 font-number font-medium"), g.Text(formatMoney(sumInflow))),
								),
							),
							g.If(sumOutflow < 0,
								h.Div(
									h.Class("flex items-center gap-2"),
									h.Span(h.Class("text-muted-foreground"), g.Text("Out:")),
									h.Span(h.Class("text-destructive font-number font-medium"), g.Text(formatMoney(sumOutflow))),
								),
							),
							h.Div(
								h.Class("flex items-center gap-2"),
								h.Span(h.Class("text-muted-foreground"), g.Text("Net:")),
								h.Span(h.Class(func() string {
									net := sumInflow + sumOutflow
									if net >= 0 {
										return "text-chart-2 font-number font-medium"
									}
									return "text-destructive font-number font-medium"
								}()), g.Text(formatMoney(sumInflow+sumOutflow))),
							),
						),
					),
				),
			),
		),

		// Pending transfer matches section (only shown on Transfers tab)
		g.If(filterType == "transfers" && len(pendingMatches) > 0,
			h.Div(
				h.ID("pending-matches-section"),
				h.Class("mb-6"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeader(
						h.Div(
							h.Class("flex items-center justify-between w-full"),
							h.Div(
								h.Class("flex items-center gap-2"),
								h.Span(h.Class("text-primary text-lg"), g.Text("⚡")),
								h.Span(h.Class("font-medium text-primary"), g.Text("Pending Matches")),
								h.Span(h.Class("text-sm text-muted-foreground"), g.Textf("(%d to review)", len(pendingMatches))),
							),
							h.Form(
								h.Method("POST"),
								h.Action("/transfers/rematch"),
								h.Button(
									h.Type("submit"),
									h.Class("text-xs text-muted-foreground hover:text-foreground px-2 py-1 bg-secondary rounded hover:bg-accent transition-colors"),
									g.Text("Re-match All"),
								),
							),
						),
					),
					shadcn.CardContent(
						h.Div(
							h.ID("pending-matches-list"),
							g.Group(g.Map(pendingMatches, func(match *models.PendingTransferMatch) g.Node {
								return renderPendingMatchRow(match)
							})),
						),
					),
				),
			),
		),

		// Bulk actions bar (hidden by default, shown via JS when items selected)
		h.Div(
			h.ID("bulk-actions-bar"),
			h.Class("hidden mb-4 bg-primary/10 border border-primary/30 rounded-lg p-3"),
			// Select all pages banner (hidden by default)
			h.Div(
				h.ID("select-all-banner"),
				h.Class("hidden mb-3 pb-3 border-b border-primary/20 text-center"),
				h.Span(
					h.Class("text-sm text-primary"),
					g.Textf("All %d on this page selected. ", min(limit, total)),
				),
				h.Button(
					h.Type("button"),
					h.Class("text-sm text-primary hover:opacity-80 font-medium underline underline-offset-2"),
					g.Attr("onclick", "selectAllPages()"),
					g.Textf("Select all %d transactions", total),
				),
			),
			// "All pages selected" confirmation (hidden by default)
			h.Div(
				h.ID("all-pages-selected"),
				h.Class("hidden mb-3 pb-3 border-b border-primary/20 text-center"),
				h.Span(
					h.Class("text-sm text-chart-2"),
					g.Textf("✓ All %d transactions selected across all pages", total),
				),
			),
			h.Div(
				h.Class("flex items-center justify-between"),
				h.Div(
					h.Class("flex items-center gap-3"),
					h.Span(
						h.ID("selected-count"),
						h.Class("text-sm text-primary"),
						g.Text("0 selected"),
					),
					h.Button(
						h.Type("button"),
						h.Class("text-sm text-muted-foreground hover:text-foreground"),
						g.Attr("onclick", "clearSelection()"),
						g.Text("Clear"),
					),
				),
				h.Div(
					h.Class("flex items-center gap-2"),
					// Tag dropdown
					shadcn.DropdownMenu(shadcn.DropdownMenuProps{
						ID:    "bulk-tag",
						Align: shadcn.PopoverAlignEnd,
					},
						shadcn.DropdownMenuTrigger("bulk-tag",
							shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonDefault},
								layouts.IconTag(),
								g.Text("Tag"),
								layouts.IconChevronDown(),
							),
						),
						shadcn.DropdownMenuContent("bulk-tag", shadcn.DropdownMenuProps{
							Align: shadcn.PopoverAlignEnd,
						},
							// Search input section
							h.Div(
								h.Class("p-2 border-b border-border"),
								shadcn.Input(shadcn.InputProps{
									Type:        "text",
									Placeholder: "Search tags...",
								},
									g.Attr("oninput", "filterBulkTags(this.value)"),
								),
							),
							// Tag list
							h.Div(
								h.Class("max-h-80 overflow-y-auto p-1"),
								g.Attr("id", "bulk-tag-list"),
								g.Group(g.Map(allTags, func(tag *models.Tag) g.Node {
									return shadcn.DropdownMenuItem(
										g.Attr("data-tag-name", strings.ToLower(tag.Name)),
										g.Attr("onclick", "bulkTagSelected('"+tag.ID.String()+"')"),
										h.Span(
											h.Class("w-2 h-2 rounded-full shrink-0"),
											h.Style("background-color: "+tag.Color),
										),
										g.Text(tag.Name),
									)
								})),
							),
						),
					),
					// Recategorize button
					shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonSecondary},
						g.Attr("onclick", "bulkRecategorize()"),
						layouts.IconSparkles(),
						g.Text("Run AI"),
					),
					// Mark as Reviewed button
					shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonSecondary},
						g.Attr("onclick", "bulkMarkReviewed()"),
						layouts.IconCheck(),
						g.Text("Mark Reviewed"),
					),
				),
			),
		),

		// Hidden forms for bulk actions
		h.Form(
			h.ID("bulk-tag-form"),
			h.Method("POST"),
			h.Action("/transactions/bulk/tag"),
			h.Class("hidden"),
			g.Attr("onsubmit", "return validateBulkTagForm()"),
			h.Input(h.Type("hidden"), h.Name("tag_id"), h.ID("bulk-tag-id")),
			h.Input(h.Type("hidden"), h.Name("transaction_ids"), h.ID("bulk-txn-ids")),
			// Filter params for "select all pages"
			h.Input(h.Type("hidden"), h.Name("select_all_pages"), h.ID("bulk-tag-select-all"), h.Value("false")),
			h.Input(h.Type("hidden"), h.Name("filter"), h.ID("bulk-tag-filter"), h.Value(filterType)),
			h.Input(h.Type("hidden"), h.Name("search"), h.ID("bulk-tag-search"), h.Value(filter.Search)),
		),
		h.Form(
			h.ID("bulk-recategorize-form"),
			h.Method("POST"),
			h.Action("/transactions/bulk/recategorize"),
			h.Class("hidden"),
			h.Input(h.Type("hidden"), h.Name("transaction_ids"), h.ID("bulk-recategorize-ids")),
			// Filter params for "select all pages"
			h.Input(h.Type("hidden"), h.Name("select_all_pages"), h.ID("bulk-recat-select-all"), h.Value("false")),
			h.Input(h.Type("hidden"), h.Name("filter"), h.ID("bulk-recat-filter"), h.Value(filterType)),
			h.Input(h.Type("hidden"), h.Name("search"), h.ID("bulk-recat-search"), h.Value(filter.Search)),
		),
		h.Form(
			h.ID("bulk-mark-reviewed-form"),
			h.Method("POST"),
			h.Action("/transactions/bulk/mark-reviewed"),
			h.Class("hidden"),
			h.Input(h.Type("hidden"), h.Name("transaction_ids"), h.ID("bulk-reviewed-ids")),
			// Filter params for "select all pages"
			h.Input(h.Type("hidden"), h.Name("select_all_pages"), h.ID("bulk-reviewed-select-all"), h.Value("false")),
			h.Input(h.Type("hidden"), h.Name("filter"), h.ID("bulk-reviewed-filter"), h.Value(filterType)),
			h.Input(h.Type("hidden"), h.Name("search"), h.ID("bulk-reviewed-search"), h.Value(filter.Search)),
		),

		// Hidden data for JS
		g.Raw(fmt.Sprintf(`<script>window.txnPageData = { total: %d, pageSize: %d, filterType: "%s" };</script>`, total, limit, filterType)),

		// Transaction list with checkboxes
		shadcn.Card(shadcn.CardProps{Class: "overflow-hidden"},
			h.Div(
				h.Class("divide-y divide-border"),
				// Header row with select all
				g.If(len(transactions) > 0,
					h.Div(
						h.Class("flex items-center px-3 sm:px-4 py-2.5 border-b border-border"),
						h.Label(
							h.Class("flex items-center gap-3 cursor-pointer group"),
							h.Input(
								h.Type("checkbox"),
								h.ID("select-all"),
								h.Class("peer sr-only"),
								g.Attr("onchange", "toggleSelectAll(this)"),
							),
							h.Span(
								h.Class("text-xs text-muted-foreground uppercase tracking-wider font-medium group-hover:text-foreground transition-colors"),
								g.Text("Select all"),
							),
						),
					),
				),
				g.If(len(transactions) == 0,
					shadcn.EmptyNoResults(filter.Search),
				),
				g.Group(g.Map(transactions, func(txn *models.Transaction) g.Node {
					return renderTransactionListItemWithCheckbox(txn, accounts, filterType, filter.Search, hdl.getLogoURL)
				})),
			),
		),

		// Pagination
		g.If(totalPages > 1,
			h.Div(
				h.Class("mt-6 flex items-center justify-between"),
				h.Div(
					h.Class("text-sm text-muted-foreground"),
					g.Textf("Showing %d-%d of %d", offset+1, min(offset+limit, total), total),
				),
				h.Div(
					h.Class("flex items-center gap-2"),
					g.If(page > 1,
						shadcn.ButtonAnchor(shadcn.ButtonProps{Variant: shadcn.ButtonOutline},
							buildURL(map[string]string{"page": strconv.Itoa(page - 1), "filter": filterType, "search": filter.Search}),
							g.Text("Previous"),
						),
					),
					h.Span(
						h.Class("px-3 py-2 text-sm text-muted-foreground"),
						g.Textf("Page %d of %d", page, totalPages),
					),
					g.If(page < totalPages,
						shadcn.ButtonAnchor(shadcn.ButtonProps{Variant: shadcn.ButtonOutline},
							buildURL(map[string]string{"page": strconv.Itoa(page + 1), "filter": filterType, "search": filter.Search}),
							g.Text("Next"),
						),
					),
				),
			),
		),

		// JavaScript for selection handling
		g.Raw(`<script>
let selectedTxns = new Set();
let selectAllPagesMode = false;

function toggleSelectAll(checkbox) {
	selectAllPagesMode = false;
	// Clear previous selections when using "Select All" to avoid mixing selections from different filters
	selectedTxns.clear();
	const checkboxes = document.querySelectorAll('.txn-checkbox');
	checkboxes.forEach(cb => {
		cb.checked = checkbox.checked;
		if (checkbox.checked) {
			selectedTxns.add(cb.value);
		}
	});
	updateBulkActionsBar();
}

function toggleTxnSelection(checkbox) {
	selectAllPagesMode = false;
	if (checkbox.checked) {
		selectedTxns.add(checkbox.value);
	} else {
		selectedTxns.delete(checkbox.value);
		document.getElementById('select-all').checked = false;
	}
	updateBulkActionsBar();
}

function selectAllPages() {
	selectAllPagesMode = true;
	// Select all visible checkboxes too
	const checkboxes = document.querySelectorAll('.txn-checkbox');
	checkboxes.forEach(cb => {
		cb.checked = true;
		selectedTxns.add(cb.value);
	});
	document.getElementById('select-all').checked = true;
	updateBulkActionsBar();
}

function updateBulkActionsBar() {
	const bar = document.getElementById('bulk-actions-bar');
	const countEl = document.getElementById('selected-count');
	const selectAllBanner = document.getElementById('select-all-banner');
	const allPagesSelected = document.getElementById('all-pages-selected');
	const pageData = window.txnPageData || { total: 0, pageSize: 50 };
	
	if (selectedTxns.size > 0 || selectAllPagesMode) {
		bar.classList.remove('hidden');
		
		if (selectAllPagesMode) {
			// All pages selected
			countEl.textContent = pageData.total + ' selected';
			selectAllBanner.classList.add('hidden');
			allPagesSelected.classList.remove('hidden');
		} else {
			countEl.textContent = selectedTxns.size + ' selected';
			allPagesSelected.classList.add('hidden');
			
			// Show "select all pages" banner if all visible are selected and there are more
			const allVisibleSelected = document.getElementById('select-all').checked;
			if (allVisibleSelected && pageData.total > selectedTxns.size) {
				selectAllBanner.classList.remove('hidden');
			} else {
				selectAllBanner.classList.add('hidden');
			}
		}
	} else {
		bar.classList.add('hidden');
		selectAllBanner.classList.add('hidden');
		allPagesSelected.classList.add('hidden');
	}
}

function clearSelection() {
	selectedTxns.clear();
	selectAllPagesMode = false;
	document.querySelectorAll('.txn-checkbox').forEach(cb => cb.checked = false);
	document.getElementById('select-all').checked = false;
	updateBulkActionsBar();
}

function filterBulkTags(query) {
	const buttons = document.querySelectorAll('#bulk-tag-list button');
	query = query.toLowerCase();
	buttons.forEach(btn => {
		const name = btn.getAttribute('data-tag-name');
		btn.style.display = name.includes(query) ? 'flex' : 'none';
	});
}

// Prevent accidental form submission - only allow if values are properly set
function validateBulkTagForm() {
	const tagId = document.getElementById('bulk-tag-id').value;
	const txnIds = document.getElementById('bulk-txn-ids').value;
	const selectAll = document.getElementById('bulk-tag-select-all').value;
	// Only allow if we have a tag AND either transaction IDs or select-all mode
	return tagId && (selectAll === 'true' || txnIds);
}

function bulkTagSelected(tagId) {
	if (selectedTxns.size === 0 && !selectAllPagesMode) return;
	document.getElementById('bulk-tag-id').value = tagId;
	document.getElementById('bulk-tag-select-all').value = selectAllPagesMode ? 'true' : 'false';
	document.getElementById('bulk-txn-ids').value = selectAllPagesMode ? '' : Array.from(selectedTxns).join(',');
	document.getElementById('bulk-tag-form').submit();
}

function bulkRecategorize() {
	if (selectedTxns.size === 0 && !selectAllPagesMode) return;
	document.getElementById('bulk-recat-select-all').value = selectAllPagesMode ? 'true' : 'false';
	document.getElementById('bulk-recategorize-ids').value = selectAllPagesMode ? '' : Array.from(selectedTxns).join(',');
	document.getElementById('bulk-recategorize-form').submit();
}

function bulkMarkReviewed() {
	if (selectedTxns.size === 0 && !selectAllPagesMode) return;
	document.getElementById('bulk-reviewed-select-all').value = selectAllPagesMode ? 'true' : 'false';
	document.getElementById('bulk-reviewed-ids').value = selectAllPagesMode ? '' : Array.from(selectedTxns).join(',');
	document.getElementById('bulk-mark-reviewed-form').submit();
}

</script>`),
		shadcn.DropdownMenuScript(),
	})

	renderHTML(w, contentNode)
}

// renderTransactionListItemWithCheckbox renders a transaction list item with a checkbox
func renderTransactionListItemWithCheckbox(txn *models.Transaction, accounts []*models.Account, filterType, searchQuery string, getLogoURL func(string) string) g.Node {
	// Find the main entry from asset/liability account (the "real" bank account)
	// This ensures we show the credit card or bank account, not the expense/income category
	var amount int64
	var accountName string
	var accountType models.AccountType
	var accountID uuid.UUID

	// First pass: prefer asset/liability accounts
	for _, e := range txn.Entries {
		if e.AmountCents != 0 && (e.AccountType == models.AccountTypeAsset || e.AccountType == models.AccountTypeLiability) {
			amount = e.AmountCents
			accountName = e.AccountName
			accountType = e.AccountType
			accountID = e.AccountID
			break
		}
	}
	// Fallback: take any non-zero entry
	if accountName == "" {
		for _, e := range txn.Entries {
			if e.AmountCents != 0 {
				amount = e.AmountCents
				accountName = e.AccountName
				accountType = e.AccountType
				accountID = e.AccountID
				break
			}
		}
	}

	// Find institution logo URL from the account
	var institutionLogoURL string
	for _, acc := range accounts {
		if acc.ID == accountID {
			if acc.InstitutionLogoURL != "" {
				institutionLogoURL = getLogoURL(acc.InstitutionLogoURL)
			}
			break
		}
	}

	// Build metadata parts (without tags - they'll be shown separately)
	var metaParts []string
	metaParts = append(metaParts, accountName)
	if txn.IsTransfer {
		metaParts = append(metaParts, "Transfer")
	}
	if len(txn.Tags) == 0 && txn.TellerCategory != "" && !txn.IsTransfer {
		metaParts = append(metaParts, formatTellerCategory(txn.TellerCategory))
	}

	// Show "Uncategorized" badge if no tags
	hasNoTags := len(txn.Tags) == 0 && !txn.IsTransfer

	// Add date to metadata
	allMetaParts := append([]string{txn.Date.Format("Jan 2")}, metaParts...)

	// Determine display name priority
	displayName := txn.Description
	var secondaryText string
	if txn.DisplayTitle != "" {
		displayName = txn.DisplayTitle
		secondaryText = txn.Description
	} else if txn.Entity != nil && txn.Entity.Name != "" {
		displayName = txn.Entity.Name
		secondaryText = txn.Description
	}

	// Build redirect URL for tag removal
	redirectParams := url.Values{}
	if filterType != "" {
		redirectParams.Set("filter", filterType)
	}
	if searchQuery != "" {
		redirectParams.Set("search", searchQuery)
	}
	redirectURL := "/transactions"
	if len(redirectParams) > 0 {
		redirectURL += "?" + redirectParams.Encode()
	}

	return h.Div(
		h.Class("flex items-center p-3 sm:p-4 hover:bg-accent transition-colors gap-3 group/row"),
		// Checkbox (shadcn-style custom checkbox)
		h.Label(
			h.Class("relative flex items-center justify-center cursor-pointer flex-none"),
			h.Input(
				h.Type("checkbox"),
				h.Class("txn-checkbox peer sr-only"),
				h.Value(txn.ID.String()),
				g.Attr("onchange", "toggleTxnSelection(this)"),
			),
			h.Span(
				h.Class("w-4 h-4 rounded border border-border bg-secondary/50 peer-checked:bg-primary peer-checked:border-primary peer-focus-visible:ring-2 peer-focus-visible:ring-ring peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-background group-hover/row:border-muted-foreground transition-all duration-150 flex items-center justify-center [&>svg]:hidden peer-checked:[&>svg]:block"),
				g.Raw(`<svg class="w-3 h-3 text-primary-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>`),
			),
		),
		// Merchant logo or fallback to institution logo
		renderEntityIcon(txn.Entity, getLogoURL, institutionLogoURL),
		// Merchant name and description - takes remaining space
		h.Div(
			h.Class("flex-1 min-w-0 pr-3"),
			h.Div(
				h.Class("flex items-center gap-2"),
				h.A(
					h.Href("/transactions/"+txn.ID.String()),
					h.Class("text-sm font-medium text-foreground hover:text-primary truncate"),
					g.Text(displayName),
				),
				// Uncategorized badge
				g.If(hasNoTags,
					shadcn.Badge(shadcn.BadgeProps{Variant: shadcn.BadgeSecondary, Class: "flex-none"},
						g.Text("Uncategorized"),
					),
				),
			),
			// Show secondary text when available
			g.If(secondaryText != "",
				h.Div(
					h.Class("text-xs text-muted-foreground truncate mt-0.5"),
					g.Text(secondaryText),
				),
			),
			// Row 3: metadata and tags inline
			h.Div(
				h.Class("flex items-center gap-1.5 mt-0.5 flex-wrap"),
				h.Span(
					h.Class("text-xs text-muted-foreground truncate"),
					g.Text(joinMeta(allMetaParts)),
				),
				// Tags - smaller and muted, inline with metadata
				g.If(len(txn.Tags) > 0,
					g.Group(g.Map(txn.Tags, func(tag *models.Tag) g.Node {
						return h.Span(
							h.Class("inline-flex items-center gap-0.5 px-1.5 py-0.5 text-[10px] leading-none rounded-full bg-primary/10 text-primary border border-primary/20 font-medium"),
							h.Span(g.Text(tag.Name)),
							h.Form(
								h.Method("POST"),
								h.Action("/transactions/"+txn.ID.String()+"/tags/"+tag.ID.String()),
								h.Class("inline-flex items-center"),
								h.Input(h.Type("hidden"), h.Name("redirect"), h.Value(redirectURL)),
								h.Button(
									h.Type("submit"),
									h.Class("flex items-center hover:text-destructive transition-colors p-0.5 rounded-full hover:bg-destructive/10"),
									h.Title("Remove tag"),
									g.Raw(`<svg class="w-2.5 h-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/></svg>`),
								),
							),
						)
					})),
				),
			),
		),
		// Amount - right aligned
		h.Div(
			h.Class("flex-none text-right"),
			h.Span(
				h.Class("font-number font-medium "+transactionAmountColorClass(amount, accountType)),
				g.Text(displayBalanceWithSign(amount, accountType)),
			),
		),
	)
}

// renderEntityIcon returns the appropriate icon for a transaction's entity.
// This is a separate function to avoid nil pointer dereference when accessing entity properties,
// since Go evaluates all function arguments before checking conditions in g.If.
// fallbackLogoURL is an optional institution logo to show when no entity logo is available.
func renderEntityIcon(entity *models.Entity, getLogoURL func(string) string, fallbackLogoURL string) g.Node {
	// If we have an entity with a logo, show it
	if entity != nil && entity.LogoURL != "" {
		entityURL := "/entities/" + entity.ID.String()
		logoURL := getLogoURL(entity.LogoURL)
		return h.A(
			h.Href(entityURL),
			h.Class("flex-none"),
			h.Img(
				h.Src(logoURL),
				h.Alt(entity.Name),
				h.Class("w-10 h-10 rounded-lg object-contain"),
			),
		)
	}

	// If we have an entity without a logo, show initial
	if entity != nil {
		entityURL := "/entities/" + entity.ID.String()
		displayName := entity.Name
		if displayName == "" {
			displayName = "?"
		}
		return h.A(
			h.Href(entityURL),
			h.Class("flex-none"),
			h.Div(
				h.Class("w-10 h-10 rounded-lg bg-gradient-to-br from-primary to-ring flex items-center justify-center text-sm font-bold text-primary-foreground"),
				g.Text(string([]rune(displayName)[0])),
			),
		)
	}

	// No entity - try fallback institution logo
	if fallbackLogoURL != "" {
		return h.Div(
			h.Class("flex-none"),
			h.Img(
				h.Src(fallbackLogoURL),
				h.Alt("Institution"),
				h.Class("w-10 h-10 rounded-lg object-contain"),
			),
		)
	}

	// No entity and no fallback - show generic icon
	return h.Div(
		h.Class("flex-none w-10 h-10 rounded-lg bg-secondary flex items-center justify-center text-muted-foreground"),
		layouts.IconList(),
	)
}

// getPersonInitials returns up to 2 initials from a person's name
func getPersonInitials(name string) string {
	if name == "" {
		return "?"
	}
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	if len(parts) == 1 {
		// Single name - return first character
		return strings.ToUpper(string([]rune(parts[0])[0]))
	}
	// Multiple parts - return first char of first and last
	first := strings.ToUpper(string([]rune(parts[0])[0]))
	last := strings.ToUpper(string([]rune(parts[len(parts)-1])[0]))
	return first + last
}

func (hdl *Handlers) renderTransactionListItem(txn *models.Transaction, accounts []*models.Account) g.Node {
	// Find the main entry from asset/liability account (the "real" bank account)
	// This ensures we get the correct institution logo and display the bank account
	var amount int64
	var accountName string
	var accountType models.AccountType
	var accountID uuid.UUID

	// First pass: prefer asset/liability accounts (real bank accounts with institution logos)
	for _, e := range txn.Entries {
		if e.AmountCents != 0 && (e.AccountType == models.AccountTypeAsset || e.AccountType == models.AccountTypeLiability) {
			amount = e.AmountCents
			accountName = e.AccountName
			accountType = e.AccountType
			accountID = e.AccountID
			break
		}
	}
	// Fallback: take any non-zero entry
	if accountName == "" {
		for _, e := range txn.Entries {
			if e.AmountCents != 0 {
				amount = e.AmountCents
				accountName = e.AccountName
				accountType = e.AccountType
				accountID = e.AccountID
				break
			}
		}
	}

	// Find institution logo URL from the account
	var institutionLogoURL string
	for _, acc := range accounts {
		if acc.ID == accountID {
			if acc.InstitutionLogoURL != "" {
				institutionLogoURL = hdl.getLogoURL(acc.InstitutionLogoURL)
			}
			break
		}
	}

	// Build metadata parts (without date - added separately for mobile)
	var metaParts []string
	metaParts = append(metaParts, accountName)
	if txn.IsTransfer {
		metaParts = append(metaParts, "Transfer")
	}
	for _, tag := range txn.Tags {
		metaParts = append(metaParts, tag.Name)
	}
	if len(txn.Tags) == 0 && txn.TellerCategory != "" && !txn.IsTransfer {
		metaParts = append(metaParts, formatTellerCategory(txn.TellerCategory))
	}

	// Add date to metadata (Teller only provides dates, not times)
	allMetaParts := append([]string{txn.Date.Format("Jan 2")}, metaParts...)

	// Determine display name priority: DisplayTitle > Merchant name > Description
	// ALWAYS show raw Teller description so user can identify/correct the transaction
	displayName := txn.Description
	var secondaryText string
	if txn.DisplayTitle != "" {
		displayName = txn.DisplayTitle
		secondaryText = txn.Description // Always show raw description
	} else if txn.Entity != nil && txn.Entity.Name != "" {
		displayName = txn.Entity.Name
		secondaryText = txn.Description // Always show raw description
	}

	return h.Div(
		h.Class("flex items-center p-3 sm:p-4 hover:bg-accent transition-colors gap-3"),
		// Merchant logo or fallback to institution logo
		renderEntityIcon(txn.Entity, hdl.getLogoURL, institutionLogoURL),
		// Merchant name and description - takes remaining space
		h.Div(
			h.Class("flex-1 min-w-0 pr-3"),
			h.A(
				h.Href("/transactions/"+txn.ID.String()),
				h.Class("text-sm font-medium text-foreground hover:text-primary truncate block"),
				g.Text(displayName),
			),
			// Show secondary text (merchant name or raw description) when available
			g.If(secondaryText != "",
				h.Div(
					h.Class("text-xs text-muted-foreground truncate mt-0.5"),
					g.Text(secondaryText),
				),
			),
			h.Div(
				h.Class("text-xs text-muted-foreground mt-0.5 truncate"),
				g.Text(joinMeta(allMetaParts)),
			),
		),
		// Amount - right aligned
		h.Div(
			h.Class("flex-none text-right"),
			h.Span(
				h.Class("font-number font-medium "+transactionAmountColorClass(amount, accountType)),
				g.Text(displayBalanceWithSign(amount, accountType)),
			),
		),
	)
}

// formatTellerCategory formats the Teller category for display
func formatTellerCategory(category string) string {
	// Teller categories are lowercase like "groceries", "dining", "transportation"
	// Capitalize first letter for display
	if category == "" {
		return ""
	}
	// Replace underscores with spaces and title case
	result := strings.ReplaceAll(category, "_", " ")
	if len(result) > 0 {
		result = strings.ToUpper(result[:1]) + result[1:]
	}
	return result
}

// joinMeta joins metadata parts with a separator
func joinMeta(parts []string) string {
	return strings.Join(parts, " · ")
}

func (hdl *Handlers) TransactionsNew(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	accounts, _ := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	allTags, _ := hdl.tags.GetByLedgerID(r.Context(), ledger.ID)

	pageNode := layouts.AppLayout("New Transaction", user.Email, user.ID.String(),
		shadcn.PageHeader("New Transaction", "Record a new transaction"),
		renderTransactionForm(nil, accounts, allTags, "/transactions", "POST"),
	)

	renderHTML(w, pageNode)
}

func (hdl *Handlers) TransactionsCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	date, err := time.Parse("2006-01-02", r.FormValue("date"))
	if err != nil {
		date = time.Now()
	}

	amountStr := r.FormValue("amount")
	// Parse amount (convert to cents)
	var amountCents int64
	if amt, err := strconv.ParseFloat(amountStr, 64); err == nil {
		amountCents = dollarsToCents(amt)
	}

	fromAccountID, _ := uuid.Parse(r.FormValue("from_account"))
	toAccountID, _ := uuid.Parse(r.FormValue("to_account"))

	txn := &models.Transaction{
		LedgerID:    ledger.ID,
		Date:        date,
		Description: r.FormValue("description"),
		Notes:       r.FormValue("notes"),
		IsTransfer:  r.FormValue("is_transfer") == "on",
	}

	entries := []*models.Entry{
		{AccountID: toAccountID, AmountCents: amountCents, Currency: "USD"},
		{AccountID: fromAccountID, AmountCents: -amountCents, Currency: "USD"},
	}

	if err := hdl.transactions.CreateWithEntries(r.Context(), txn, entries); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/transactions/"+txn.ID.String(), http.StatusSeeOther)
}

func (hdl *Handlers) TransactionsShow(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	txnID, ok := mustParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	txn, err := hdl.transactions.GetByID(r.Context(), txnID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Verify ownership
	if txn.LedgerID != ledger.ID {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	if err := hdl.transactions.LoadEntries(r.Context(), txn); err != nil {
		slog.WarnContext(r.Context(), "failed to load entries", "txn_id", txn.ID, "err", err)
	}
	if err := hdl.transactions.LoadTags(r.Context(), txn); err != nil {
		slog.WarnContext(r.Context(), "failed to load tags", "txn_id", txn.ID, "err", err)
	}
	if err := hdl.transactions.LoadEntity(r.Context(), txn, hdl.entities); err != nil {
		slog.WarnContext(r.Context(), "failed to load entity", "txn_id", txn.ID, "err", err)
	}

	// Load transfer information
	var transferPair *models.Transaction
	var pendingMatches []*models.PendingTransferMatch

	if txn.IsTransfer && txn.TransferPairID != nil {
		// This is a confirmed transfer - load the paired transaction
		if tp, err := hdl.transactions.GetByID(r.Context(), *txn.TransferPairID); err != nil {
			slog.WarnContext(r.Context(), "failed to load transfer pair", "pair_id", *txn.TransferPairID, "err", err)
		} else {
			transferPair = tp
			if err := hdl.transactions.LoadEntries(r.Context(), transferPair); err != nil {
				slog.WarnContext(r.Context(), "failed to load transfer pair entries", "pair_id", transferPair.ID, "err", err)
			}
		}
	} else {
		// Check for pending matches
		var pmErr error
		pendingMatches, pmErr = hdl.pendingMatches.GetByTransactionID(r.Context(), txn.ID)
		if pmErr != nil {
			slog.WarnContext(r.Context(), "failed to load pending matches", "txn_id", txn.ID, "err", pmErr)
		}
		for _, match := range pendingMatches {
			if err := hdl.pendingMatches.LoadTransactions(r.Context(), match, hdl.transactions); err != nil {
				slog.WarnContext(r.Context(), "failed to load pending match transactions", "txn_id", txn.ID, "err", err)
			}
			// Determine which transaction is the "other" one
			if match.Transaction != nil && match.Transaction.ID == txn.ID {
				if match.CandidateTransaction != nil {
					if err := hdl.transactions.LoadEntries(r.Context(), match.CandidateTransaction); err != nil {
						slog.WarnContext(r.Context(), "failed to load candidate entries", "txn_id", match.CandidateTransaction.ID, "err", err)
					}
				}
			} else if match.CandidateTransaction != nil {
				if match.Transaction != nil {
					if err := hdl.transactions.LoadEntries(r.Context(), match.Transaction); err != nil {
						slog.WarnContext(r.Context(), "failed to load match entries", "txn_id", match.Transaction.ID, "err", err)
					}
				}
			}
		}
	}

	// Find the primary account (asset/liability) and amount
	var primaryAccountName string
	var primaryAmount int64
	var primaryAccountType models.AccountType
	var primaryCurrency string

	// First try to find a non-zero asset/liability entry
	for _, e := range txn.Entries {
		if e.AmountCents != 0 && (e.AccountType == models.AccountTypeAsset || e.AccountType == models.AccountTypeLiability) {
			primaryAccountName = e.AccountName
			primaryAmount = e.AmountCents
			primaryAccountType = e.AccountType
			primaryCurrency = e.Currency
			break
		}
	}
	// Fallback to first non-zero entry if no asset/liability found
	if primaryAccountName == "" {
		for _, e := range txn.Entries {
			if e.AmountCents != 0 {
				primaryAccountName = e.AccountName
				primaryAmount = e.AmountCents
				primaryAccountType = e.AccountType
				primaryCurrency = e.Currency
				break
			}
		}
	}

	// Determine the best display title for the page
	pageTitle := txn.Description
	if txn.DisplayTitle != "" {
		pageTitle = txn.DisplayTitle
	} else if txn.Entity != nil && txn.Entity.Name != "" {
		pageTitle = txn.Entity.Name
	}

	pageNode := layouts.AppLayout(pageTitle, user.Email, user.ID.String(),
		shadcn.PageHeader("Transaction Details", txn.Date.Format("January 2, 2006")),

		h.Div(
			h.Class("grid grid-cols-1 lg:grid-cols-3 gap-6"),

			// Left Column (Main Content)
			h.Div(
				h.Class("lg:col-span-2 space-y-6"),

				// Overview Card
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardContentFull(
						h.Div(
							h.Class("flex flex-col sm:flex-row sm:items-start justify-between gap-6"),
							// Left: Merchant/Description
							h.Div(
								h.Class("flex gap-4"),
								// Logo/Icon - entity hierarchy: entity -> intermediary -> counterparty -> merchant -> fallback
								func() g.Node {
									// Priority 1: Entity with logo
									if txn.Entity != nil && txn.Entity.LogoURL != "" {
										logoURL := hdl.getLogoURL(txn.Entity.LogoURL)
										return h.A(
											h.Href("/entities/"+txn.Entity.ID.String()),
											h.Class("flex-none"),
											h.Img(
												h.Src(logoURL),
												h.Alt(txn.Entity.Name),
												h.Class("w-16 h-16 rounded-xl object-contain flex-none"),
											),
										)
									}
									// Priority 2: Intermediary entity with logo (Zelle, Venmo, etc.)
									if txn.IntermediaryEntity != nil && txn.IntermediaryEntity.LogoURL != "" {
										logoURL := hdl.getLogoURL(txn.IntermediaryEntity.LogoURL)
										return h.A(
											h.Href("/entities/"+txn.IntermediaryEntity.ID.String()),
											h.Class("flex-none"),
											h.Img(
												h.Src(logoURL),
												h.Alt(txn.IntermediaryEntity.Name),
												h.Class("w-16 h-16 rounded-xl object-contain flex-none"),
											),
										)
									}
									// Priority 3: Counterparty person entity - show initials
									if txn.CounterpartyEntity != nil && txn.CounterpartyEntity.IsPerson() {
										initials := getPersonInitials(txn.CounterpartyEntity.Name)
										return h.A(
											h.Href("/entities/"+txn.CounterpartyEntity.ID.String()),
											h.Class("flex-none"),
											h.Div(
												h.Class("w-16 h-16 rounded-full bg-gradient-to-br from-chart-2 to-chart-3 flex items-center justify-center text-2xl font-bold text-primary-foreground shadow-lg shadow-chart-2/20 flex-none"),
												g.Text(initials),
											),
										)
									}
									// Priority 4: Legacy merchant with logo
									if txn.Entity != nil && txn.Entity.LogoURL != "" {
										logoURL := hdl.getLogoURL(txn.Entity.LogoURL)
										return h.A(
											h.Href("/entities/"+txn.Entity.ID.String()),
											h.Class("flex-none"),
											h.Img(
												h.Src(logoURL),
												h.Alt(txn.Entity.Name),
												h.Class("w-16 h-16 rounded-xl object-contain flex-none"),
											),
										)
									}
									// Fallback: Use display title, then entity name, then description for the initial
									displayNameForIcon := txn.Description
									if txn.DisplayTitle != "" {
										displayNameForIcon = txn.DisplayTitle
									} else if txn.Entity != nil && txn.Entity.Name != "" {
										displayNameForIcon = txn.Entity.Name
									}
									initial := "?"
									if len(displayNameForIcon) > 0 {
										initial = string([]rune(displayNameForIcon)[0])
									}
									iconDiv := h.Div(
										h.Class("w-16 h-16 rounded-xl bg-gradient-to-br from-primary to-ring flex items-center justify-center text-2xl font-bold text-primary-foreground shadow-lg shadow-ring/20 flex-none"),
										g.Text(initial),
									)
									// Link to entity if one exists
									if txn.Entity != nil {
										return h.A(
											h.Href("/entities/"+txn.Entity.ID.String()),
											h.Class("flex-none"),
											iconDiv,
										)
									}
									return iconDiv
								}(),
								// Show secondary info: merchant name if we have display title, or description if we have merchant
								func() g.Node {
									var titleContent g.Node = g.Text(func() string {
										if txn.DisplayTitle != "" {
											return txn.DisplayTitle
										}
										if txn.Entity != nil && txn.Entity.Name != "" {
											return txn.Entity.Name
										}
										return txn.Description
									}())

									if txn.Entity != nil {
										titleContent = h.A(
											h.Href("/entities/"+txn.Entity.ID.String()),
											h.Class("hover:text-primary transition-colors"),
											titleContent,
										)
									}

									return h.Div(
										h.Class("space-y-1"),
										h.H2(
											h.Class("text-xl font-bold text-foreground leading-tight"),
											titleContent,
										),
										// Always show raw bank description when we have a merchant or display title
										// so users can verify/correct the AI's interpretation
										g.If(txn.Description != "" && (txn.DisplayTitle != "" || (txn.Entity != nil && txn.Entity.Name != "")),
											h.P(
												h.Class("text-sm text-muted-foreground truncate max-w-md"),
												h.Title(txn.Description), // Full description on hover
												g.Text(txn.Description),
											),
										),
										// Display Tags under the merchant name
										g.If(len(txn.Tags) > 0,
											h.Div(
												h.Class("flex flex-wrap gap-2 mt-2"),
												g.Group(g.Map(txn.Tags, func(tag *models.Tag) g.Node {
													return shadcn.Badge(shadcn.BadgeProps{Variant: shadcn.BadgeSecondary},
														g.Text(tag.Name),
													)
												})),
											),
										),
									)
								}(),
							),
							// Right: Amount
							h.Div(
								h.Class("text-left sm:text-right"),
								h.Div(
									h.Class("text-3xl font-bold font-number "+transactionAmountColorClass(primaryAmount, primaryAccountType)),
									g.Text(displayBalanceWithSign(primaryAmount, primaryAccountType)),
									// Show currency badge if non-USD
									g.If(primaryCurrency != "" && primaryCurrency != "USD",
										h.Span(
											h.Class("ml-2 text-base font-medium text-muted-foreground"),
											g.Text(primaryCurrency),
										),
									),
								),
								h.P(
									h.Class("text-sm text-muted-foreground mt-1"),
									g.Text(primaryAccountName),
								),
							),
						),
					),
				),

				// Confirmed Transfer
				g.If(transferPair != nil, renderConfirmedTransfer(txn, transferPair)),

				// Pending Matches
				g.If(len(pendingMatches) > 0, renderPendingTransferMatches(txn, pendingMatches)),

				// Raw Transaction Data (always show original bank info)
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Original Bank Data", "Raw data from your bank"),
					h.Div(
						h.Class("divide-y divide-border"),
						// Original Description (always visible)
						h.Div(
							h.Class("px-6 py-4 flex justify-between items-start gap-4"),
							h.Div(
								h.Span(h.Class("text-sm text-muted-foreground"), g.Text("Original Description")),
							),
							h.Div(
								h.Class("text-right"),
								h.Span(h.Class("text-sm text-foreground"), g.Text(func() string {
									if txn.Description != "" {
										return txn.Description
									}
									return "—"
								}())),
							),
						),
						// Teller Category
						g.If(txn.TellerCategory != "",
							h.Div(
								h.Class("px-6 py-4 flex justify-between items-start gap-4"),
								h.Div(
									h.Span(h.Class("text-sm text-muted-foreground"), g.Text("Bank Category")),
								),
								h.Div(
									h.Class("text-right"),
									h.Span(h.Class("text-sm text-foreground"), g.Text(formatTellerCategory(txn.TellerCategory))),
								),
							),
						),
						// Teller Type
						g.If(txn.TellerType != "",
							h.Div(
								h.Class("px-6 py-4 flex justify-between items-start gap-4"),
								h.Div(
									h.Span(h.Class("text-sm text-muted-foreground"), g.Text("Transaction Type")),
								),
								h.Div(
									h.Class("text-right"),
									h.Span(h.Class("text-sm text-foreground"), g.Text(formatTellerCategory(txn.TellerType))),
								),
							),
						),
						// Counterparty
						g.If(txn.CounterpartyName != "",
							h.Div(
								h.Class("px-6 py-4 flex justify-between items-start gap-4"),
								h.Div(
									h.Span(h.Class("text-sm text-muted-foreground"), g.Text("Counterparty")),
								),
								h.Div(
									h.Class("text-right"),
									h.Span(h.Class("text-sm text-foreground"), g.Text(txn.CounterpartyName)),
									g.If(txn.CounterpartyType != "",
										h.Span(h.Class("text-xs text-muted-foreground ml-2"), g.Text("("+txn.CounterpartyType+")")),
									),
								),
							),
						),
						// Status
						g.If(txn.TellerStatus != "",
							h.Div(
								h.Class("px-6 py-4 flex justify-between items-start gap-4"),
								h.Div(
									h.Span(h.Class("text-sm text-muted-foreground"), g.Text("Status")),
								),
								h.Div(
									h.Class("text-right"),
									h.Span(
										h.Class(func() string {
											if txn.TellerStatus == "pending" {
												return "inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-ring/35 text-muted-foreground dark:text-foreground/70"
											}
											return "inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-chart-2/35 text-muted-foreground dark:text-foreground/70"
										}()),
										g.Text(formatTellerCategory(txn.TellerStatus)),
									),
								),
							),
						),
						// Notes
						g.If(txn.Notes != "",
							h.Div(
								h.Class("px-6 py-4"),
								h.Div(
									h.Span(h.Class("text-sm text-muted-foreground"), g.Text("Notes")),
								),
								h.Div(
									h.Class("mt-2"),
									h.P(h.Class("text-sm text-foreground whitespace-pre-wrap"), g.Text(txn.Notes)),
								),
							),
						),
					),
				),

				// Entries
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Ledger Entries", "Double-entry accounting details"),
					shadcn.CardContent(
						h.Div(
							h.Class("overflow-hidden"),
							h.Table(
								h.Class("w-full"),
								h.THead(
									h.Class("bg-card/50 border-b border-border text-xs uppercase text-muted-foreground font-medium"),
									h.Tr(
										h.Th(h.Class("px-6 py-3 text-left"), g.Text("Account")),
										h.Th(h.Class("px-6 py-3 text-left"), g.Text("Type")),
										h.Th(h.Class("px-6 py-3 text-left"), g.Text("Currency")),
										h.Th(h.Class("px-6 py-3 text-right"), g.Text("Amount")),
									),
								),
								h.TBody(
									h.Class("divide-y divide-border"),
									g.Group(g.Map(txn.Entries, func(e *models.Entry) g.Node {
										return h.Tr(
											h.Class("hover:bg-accent transition-colors"),
											h.Td(
												h.Class("px-6 py-4 text-sm font-medium text-foreground"),
												g.Text(e.AccountName),
											),
											h.Td(
												h.Class("px-6 py-4"),
												shadcn.Badge(shadcn.BadgeProps{Variant: shadcn.BadgeOutline},
													g.Text(formatAccountType(e.AccountType)),
												),
											),
											h.Td(
												h.Class("px-6 py-4 text-sm text-muted-foreground"),
												g.Text(func() string {
													if e.Currency != "" {
														return e.Currency
													}
													return "USD"
												}()),
											),
											h.Td(
												h.Class("px-6 py-4 text-right font-number text-sm font-medium "+simpleAmountColorClass(e.AmountCents)),
												g.Text(formatMoneyWithSign(e.AmountCents)),
											),
										)
									})),
								),
							),
						),
					),
				),
			),

			// Right Column (Sidebar)
			h.Div(
				h.Class("space-y-6"),

				// Actions
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardContentFull(
						h.Div(
							h.Class("flex flex-col gap-2"),
							h.A(
								h.Href("/transactions/"+txn.ID.String()+"/edit"),
								h.Class("w-full inline-flex items-center justify-center gap-2 bg-secondary text-foreground rounded-lg px-4 py-2.5 text-sm font-medium hover:bg-accent transition-colors"),
								layouts.IconEdit(),
								g.Text("Edit Transaction"),
							),
							h.Form(
								h.Method("POST"),
								h.Action("/transactions/"+txn.ID.String()+"/recategorize"),
								h.Button(
									h.Type("submit"),
									h.Class("w-full inline-flex items-center justify-center gap-2 bg-secondary text-foreground rounded-lg px-4 py-2.5 text-sm font-medium hover:bg-accent transition-colors"),
									layouts.IconSparkles(),
									g.Text("Run AI Again"),
								),
							),
							h.Form(
								h.Method("POST"),
								h.Action("/transactions/"+txn.ID.String()+"/delete"),
								h.Input(h.Type("hidden"), h.Name("_method"), h.Value("DELETE")),
								h.Button(
									h.Type("submit"),
									h.Class("w-full inline-flex items-center justify-center gap-2 bg-secondary text-foreground hover:text-destructive hover:bg-accent rounded-lg px-4 py-2.5 text-sm font-medium transition-colors"),
									g.Attr("onclick", "return confirm('Are you sure you want to delete this transaction? This cannot be undone.')"),
									layouts.IconTrash(),
									g.Text("Delete Transaction"),
								),
							),
						),
					),
				),
			),
		),
	)

	renderHTML(w, pageNode)
}

// renderConfirmedTransfer renders the linked transfer card for confirmed transfers
func renderConfirmedTransfer(txn, pair *models.Transaction) g.Node {
	if pair == nil {
		return nil
	}

	var pairAmount int64
	var pairAccount string
	for _, e := range pair.Entries {
		if e.AmountCents != 0 {
			pairAmount = e.AmountCents
			pairAccount = e.AccountName
			break
		}
	}

	return h.Div(
		h.Class("mb-6"),
		shadcn.Card(shadcn.CardProps{},
			shadcn.CardHeader(
				h.Div(
					h.Class("flex items-center gap-2 text-primary"),
					h.Span(g.Text("🔗")),
					h.Span(h.Class("font-medium"), g.Text("Transfer")),
				),
			),
			shadcn.CardContent(
				h.P(h.Class("text-sm text-muted-foreground mb-3"), g.Text("This is the other side of this transfer:")),
				h.Div(
					h.Class("flex items-center justify-between"),
					h.Div(
						h.A(
							h.Href("/transactions/"+pair.ID.String()),
							h.Class("font-medium text-foreground hover:text-primary"),
							g.Text(pair.Description),
						),
						h.P(h.Class("text-sm text-muted-foreground"),
							g.Text(pair.Date.Format("Jan 2, 2006")),
							g.Text(" • "),
							g.Text(pairAccount),
						),
					),
					h.Span(
						h.Class("font-number font-medium "+simpleAmountColorClass(pairAmount)),
						g.Text(formatMoneyWithSign(pairAmount)),
					),
				),
				h.Div(
					h.Class("flex items-center gap-2 mt-4 pt-3 border-t border-border"),
					h.A(
						h.Href("/transactions/"+pair.ID.String()),
						h.Class("inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-primary bg-primary/30 rounded-lg hover:bg-primary/50 transition-colors"),
						g.Text("View Transaction →"),
					),
					h.Form(
						h.Method("POST"),
						h.Action("/transfers/"+txn.ID.String()+"/unlink"),
						h.Button(
							h.Type("submit"),
							h.Class("inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-muted-foreground bg-secondary rounded-lg hover:bg-accent transition-colors"),
							g.Text("Unlink Transfer"),
						),
					),
				),
			),
		),
	)
}

// renderPendingTransferMatches renders potential transfer matches for review
func renderPendingTransferMatches(txn *models.Transaction, matches []*models.PendingTransferMatch) g.Node {
	// Show the best match (highest confidence)
	if len(matches) == 0 {
		return nil
	}

	match := matches[0]

	// Determine which transaction is the "other" one
	var otherTxn *models.Transaction
	if match.Transaction != nil && match.Transaction.ID == txn.ID {
		otherTxn = match.CandidateTransaction
	} else {
		otherTxn = match.Transaction
	}

	if otherTxn == nil {
		return nil
	}

	// Find the real bank account (asset/liability) and amount for the other transaction
	var otherAmount int64
	var otherBankAccount string
	var otherCategory string

	for _, e := range otherTxn.Entries {
		if e.AmountCents == 0 {
			continue
		}
		// Prefer asset/liability accounts (real bank accounts) over expense/income (categories)
		if e.AccountType == models.AccountTypeAsset || e.AccountType == models.AccountTypeLiability {
			otherBankAccount = e.AccountName
			// Use absolute value of amount for clearer display
			if e.AmountCents < 0 {
				otherAmount = -e.AmountCents
			} else {
				otherAmount = e.AmountCents
			}
		} else if e.AccountType == models.AccountTypeExpense || e.AccountType == models.AccountTypeIncome {
			otherCategory = e.AccountName
		}
	}

	// Fallback if no asset/liability found
	if otherBankAccount == "" {
		for _, e := range otherTxn.Entries {
			if e.AmountCents != 0 {
				if e.AmountCents < 0 {
					otherAmount = -e.AmountCents
				} else {
					otherAmount = e.AmountCents
				}
				otherBankAccount = e.AccountName
				break
			}
		}
	}

	confidencePercent := int(match.ConfidenceScore * 100)

	return h.Div(
		h.Class("mb-6"),
		shadcn.Card(shadcn.CardProps{},
			shadcn.CardHeader(
				h.Div(
					h.Class("flex items-center gap-2 text-primary"),
					h.Span(h.Class("text-lg"), g.Text("⚡")),
					h.Span(h.Class("font-semibold"), g.Text("Potential Transfer Match")),
				),
			),
			shadcn.CardContent(
				h.P(h.Class("text-sm text-muted-foreground mb-4"), g.Text("This transaction may be a transfer to:")),

				// Match details card
				h.Div(
					h.Class("bg-secondary/50 rounded-lg p-4 mb-4"),
					// Transaction description row
					h.Div(
						h.Class("flex items-center justify-between gap-4 mb-3"),
						h.A(
							h.Href("/transactions/"+otherTxn.ID.String()),
							h.Class("font-medium text-foreground hover:text-primary"),
							g.Text(otherTxn.Description),
						),
						h.Span(
							h.Class("font-number text-lg font-semibold text-chart-2 whitespace-nowrap"),
							g.Text(formatMoney(otherAmount)),
						),
					),
					// Account info row
					h.Div(
						h.Class("flex items-center gap-2 text-sm"),
						h.Span(h.Class("text-muted-foreground"), g.Text(otherTxn.Date.Format("Jan 2, 2006"))),
						h.Span(h.Class("text-muted-foreground"), g.Text("•")),
						h.Span(h.Class("text-primary font-medium"), g.Text(otherBankAccount)),
					),
					g.If(otherCategory != "",
						h.Div(
							h.Class("mt-2 text-xs text-muted-foreground"),
							g.Text("Category: "+otherCategory),
						),
					),
				),

				// Confidence badge
				h.Div(
					h.Class("mb-5"),
					h.Span(
						h.Class("inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-ring/35 text-muted-foreground dark:text-foreground/70"),
						g.Textf("%d%% confidence", confidencePercent),
					),
				),

				// Action buttons
				h.Div(
					h.Class("flex flex-wrap items-center gap-3 pt-4 border-t border-border"),
					h.Form(
						h.Method("POST"),
						h.Action("/transfers/"+match.ID.String()+"/confirm"),
						h.Button(
							h.Type("submit"),
							h.Class("inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium rounded-lg bg-chart-2 text-primary-foreground hover:opacity-90 transition-all shadow-lg shadow-chart-2/30 hover:shadow-chart-2/50"),
							layouts.IconCheck(),
							g.Text("Link as Transfer"),
						),
					),
					h.Form(
						h.Method("POST"),
						h.Action("/transfers/"+match.ID.String()+"/reject"),
						h.Button(
							h.Type("submit"),
							h.Class("inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium rounded-lg border border-border text-foreground hover:bg-accent hover:border-primary transition-all"),
							g.Text("Not a Match"),
						),
					),
				),
			),
		),
	)
}

func (hdl *Handlers) TransactionsEdit(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	txnID, ok := mustParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	txn, err := hdl.transactions.GetByID(r.Context(), txnID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Verify ownership
	if txn.LedgerID != ledger.ID {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	if err := hdl.transactions.LoadEntries(r.Context(), txn); err != nil {
		slog.WarnContext(r.Context(), "failed to load entries", "txn_id", txn.ID, "err", err)
	}
	if err := hdl.transactions.LoadTags(r.Context(), txn); err != nil {
		slog.WarnContext(r.Context(), "failed to load tags", "txn_id", txn.ID, "err", err)
	}

	accounts, _ := hdl.accounts.GetByLedgerID(r.Context(), txn.LedgerID)
	allTags, _ := hdl.tags.GetByLedgerID(r.Context(), txn.LedgerID)

	pageNode := layouts.AppLayout("Edit Transaction", user.Email, user.ID.String(),
		shadcn.PageHeader("Edit Transaction", txn.Description),
		renderTransactionForm(txn, accounts, allTags, "/transactions/"+txn.ID.String(), "PUT"),
	)

	renderHTML(w, pageNode)
}

func (hdl *Handlers) TransactionsUpdate(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	txnID, ok := mustParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	txn, err := hdl.transactions.GetByID(r.Context(), txnID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Verify ownership
	if txn.LedgerID != ledger.ID {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	date, err := time.Parse("2006-01-02", r.FormValue("date"))
	if err == nil {
		txn.Date = date
	}

	txn.Description = r.FormValue("description")
	txn.DisplayTitle = r.FormValue("display_title")
	txn.Notes = r.FormValue("notes")
	txn.IsTransfer = r.FormValue("is_transfer") == "on"

	// Parse amount - only update entries if we have a valid non-zero amount
	// This prevents auto-save from wiping out entries when the field is temporarily empty
	amountStr := r.FormValue("amount")
	var amountCents int64
	var amountValid bool
	if amt, err := strconv.ParseFloat(amountStr, 64); err == nil && amt != 0 {
		amountCents = dollarsToCents(amt)
		amountValid = true
	}

	fromAccountID, _ := uuid.Parse(r.FormValue("from_account"))
	toAccountID, _ := uuid.Parse(r.FormValue("to_account"))

	// Only update entries if we have a valid amount AND both accounts selected
	// This prevents auto-save from creating $0 entries
	if amountValid && fromAccountID != uuid.Nil && toAccountID != uuid.Nil {
		entries := []*models.Entry{
			{AccountID: toAccountID, AmountCents: amountCents, Currency: "USD"},
			{AccountID: fromAccountID, AmountCents: -amountCents, Currency: "USD"},
		}

		if err := hdl.transactions.UpdateWithEntries(r.Context(), txn, entries); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Just update the transaction metadata without touching entries
		if err := hdl.transactions.Update(r.Context(), txn); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Return 204 for HTMX auto-save requests
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, "/transactions/"+txn.ID.String(), http.StatusSeeOther)
}

func (hdl *Handlers) TransactionsDelete(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	txnID, ok := mustParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	txn, err := hdl.transactions.GetByID(r.Context(), txnID)
	if err != nil {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if txn.LedgerID != ledger.ID {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	if err := hdl.transactions.Delete(r.Context(), txnID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/transactions", http.StatusSeeOther)
}

func (hdl *Handlers) TransactionsAddTag(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	txnID, ok := mustParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	// Verify ownership
	txn, err := hdl.transactions.GetByID(r.Context(), txnID)
	if err != nil || txn.LedgerID != ledger.ID {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tagID, ok := mustFormParamUUID(w, r, "tag_id", "tag ID")
	if !ok {
		return
	}

	// Verify tag belongs to same ledger
	tag, err := hdl.tags.GetByID(r.Context(), tagID)
	if err != nil || tag.LedgerID != ledger.ID {
		http.Error(w, "Invalid tag", http.StatusBadRequest)
		return
	}

	if err := hdl.tags.CategorizeTransaction(r.Context(), txnID, tagID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return 204 for HTMX or redirect
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Check for redirect parameter (used by edit form)
	if redirect := r.FormValue("redirect"); redirect != "" {
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/transactions/"+txnID.String(), http.StatusSeeOther)
}

func (hdl *Handlers) TransactionsRemoveTag(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	txnID, ok := mustParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	// Verify ownership
	txn, err := hdl.transactions.GetByID(r.Context(), txnID)
	if err != nil || txn.LedgerID != ledger.ID {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	tagID, ok := mustParamUUID(w, r, "tagId", "tag ID")
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := hdl.tags.RemoveTagFromTransaction(r.Context(), txnID, tagID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check for redirect parameter (used by edit form)
	if redirect := r.FormValue("redirect"); redirect != "" {
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/transactions/"+txnID.String(), http.StatusSeeOther)
}

func (hdl *Handlers) TransactionsRecategorize(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	txnID, ok := mustParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	// Verify ownership
	txn, err := hdl.transactions.GetByID(r.Context(), txnID)
	if err != nil || txn.LedgerID != ledger.ID {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	slog.InfoContext(r.Context(), "queueing transaction for recategorization", "id", txnID)

	// Mark the transaction for recategorization (removes existing tags and resets status)
	if err := hdl.transactions.MarkForRecategorization(r.Context(), []uuid.UUID{txnID}); err != nil {
		slog.ErrorContext(r.Context(), "mark transaction for recategorization failed", "id", txnID, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "queued transaction for recategorization", "id", txnID)
	http.Redirect(w, r, "/transactions/"+txnID.String(), http.StatusSeeOther)
}

// bulkCollectTxnIDs returns transaction IDs for a bulk operation.
// When select_all_pages=true it lists all transactions matching the current filter;
// otherwise it parses transaction_ids from the form (multi-value or comma-separated).
func (hdl *Handlers) bulkCollectTxnIDs(r *http.Request, ledgerID uuid.UUID) ([]uuid.UUID, error) {
	if r.FormValue("select_all_pages") == "true" {
		filter := hdl.buildFilterFromForm(r, ledgerID)
		filter.Limit = 0
		transactions, _, err := hdl.transactions.List(r.Context(), filter)
		if err != nil {
			return nil, err
		}
		ids := make([]uuid.UUID, len(transactions))
		for i, txn := range transactions {
			ids[i] = txn.ID
		}
		slog.InfoContext(r.Context(), "bulk: resolved transactions via filter", "count", len(ids))
		return ids, nil
	}
	var ids []uuid.UUID
	for _, val := range r.Form["transaction_ids"] {
		for _, raw := range strings.Split(val, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			id, err := uuid.Parse(raw)
			if err != nil {
				slog.WarnContext(r.Context(), "bulk: skipping malformed transaction ID", "id", raw, "err", err)
				continue
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// htmxRefreshOrRedirect sends HX-Refresh for HTMX requests; otherwise redirects to
// fallback, preserving filter and search query params from the request form.
func htmxRefreshOrRedirect(w http.ResponseWriter, r *http.Request, fallback string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	redirectURL := fallback
	params := url.Values{}
	if f := r.FormValue("filter"); f != "" {
		params.Set("filter", f)
	}
	if s := r.FormValue("search"); s != "" {
		params.Set("search", s)
	}
	if len(params) > 0 {
		redirectURL += "?" + params.Encode()
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// TransactionsBulkTag handles bulk tagging of multiple transactions
func (hdl *Handlers) TransactionsBulkTag(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tagID, ok := mustFormParamUUID(w, r, "tag_id", "tag ID")
	if !ok {
		return
	}

	txnIDs, err := hdl.bulkCollectTxnIDs(r, ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(txnIDs) == 0 {
		http.Error(w, "No valid transaction IDs provided", http.StatusBadRequest)
		return
	}

	slog.InfoContext(r.Context(), "tagging transactions with tag", "count", len(txnIDs), "tag_id", tagID)

	count, err := hdl.tags.BulkCategorizeTransactions(r.Context(), txnIDs, tagID)
	if err != nil {
		slog.ErrorContext(r.Context(), "bulk tag transactions failed", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "tagged transactions", "count", count)
	htmxRefreshOrRedirect(w, r, "/transactions")
}

// TransactionsBulkRecategorize handles bulk recategorization of multiple transactions
func (hdl *Handlers) TransactionsBulkRecategorize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	txnIDs, err := hdl.bulkCollectTxnIDs(r, ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(txnIDs) == 0 {
		http.Error(w, "No valid transaction IDs provided", http.StatusBadRequest)
		return
	}

	slog.InfoContext(r.Context(), "queueing transactions for recategorization", "count", len(txnIDs))

	if err := hdl.transactions.MarkForRecategorization(r.Context(), txnIDs); err != nil {
		slog.ErrorContext(r.Context(), "bulk recategorize transactions failed", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "queued transactions for recategorization", "count", len(txnIDs))
	htmxRefreshOrRedirect(w, r, "/transactions")
}

// TransactionsBulkMarkReviewed marks multiple transactions as reviewed
// This removes them from the "needs review" filter without changing their tags
func (hdl *Handlers) TransactionsBulkMarkReviewed(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	txnIDs, err := hdl.bulkCollectTxnIDs(r, ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(txnIDs) == 0 {
		http.Error(w, "No valid transaction IDs provided", http.StatusBadRequest)
		return
	}

	slog.InfoContext(r.Context(), "marking transactions as reviewed", "count", len(txnIDs))

	count, err := hdl.transactions.BulkMarkAsReviewed(r.Context(), txnIDs)
	if err != nil {
		slog.ErrorContext(r.Context(), "bulk mark reviewed failed", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "marked transactions as reviewed", "count", count)
	htmxRefreshOrRedirect(w, r, "/transactions")
}

// buildFilterFromForm builds a TransactionFilter from form values
func (hdl *Handlers) buildFilterFromForm(r *http.Request, ledgerID uuid.UUID) models.TransactionFilter {
	filter := models.TransactionFilter{
		LedgerID: ledgerID,
		Search:   r.FormValue("search"),
	}

	// Apply filter based on filter type
	switch r.FormValue("filter") {
	case "uncategorized":
		filter.Uncategorized = true
	case "needs_review":
		filter.NeedsReview = true
	case "transfers":
		isTransfer := true
		filter.IsTransfer = &isTransfer
	}

	if accID := r.FormValue("account"); accID != "" {
		if id, err := uuid.Parse(accID); err == nil {
			filter.AccountID = &id
		}
	}

	if tagID := r.FormValue("tag"); tagID != "" {
		if id, err := uuid.Parse(tagID); err == nil {
			filter.TagID = &id
		}
	}

	return filter
}

func renderTransactionForm(txn *models.Transaction, accounts []*models.Account, allTags []*models.Tag, action, method string) g.Node {
	dateVal := time.Now().Format("2006-01-02")
	descriptionVal := ""
	displayTitleVal := ""
	notesVal := ""
	isTransferVal := false
	var fromAccountIDVal, toAccountIDVal string
	var amountVal float64

	if txn != nil {
		dateVal = txn.Date.Format("2006-01-02")
		descriptionVal = txn.Description
		displayTitleVal = txn.DisplayTitle
		notesVal = txn.Notes
		isTransferVal = txn.IsTransfer

		// Get entries - find the positive and negative sides
		// Use absolute value to handle any entry ordering
		for _, e := range txn.Entries {
			if e.AmountCents > 0 {
				toAccountIDVal = e.AccountID.String()
				amountVal = float64(e.AmountCents) / 100
			} else if e.AmountCents < 0 {
				fromAccountIDVal = e.AccountID.String()
				// If we haven't found a positive amount yet, use absolute value of negative
				if amountVal == 0 {
					amountVal = float64(-e.AmountCents) / 100
				}
			}
		}
	}

	accountOptions := make([]shadcn.SelectOption, 0, len(accounts)+1)
	accountOptions = append(accountOptions, shadcn.SelectOption{Value: "", Label: "Select account..."})
	for _, acc := range accounts {
		accountOptions = append(accountOptions, shadcn.SelectOption{
			Value: acc.ID.String(),
			Label: acc.Name + " (" + string(acc.Type) + ")",
		})
	}

	// Build tag combobox options (exclude already-assigned tags)
	var tagOptions []ComboboxOption
	for _, tag := range allTags {
		isAssigned := false
		if txn != nil {
			for _, t := range txn.Tags {
				if t.ID == tag.ID {
					isAssigned = true
					break
				}
			}
		}
		if !isAssigned {
			tagOptions = append(tagOptions, ComboboxOption{Value: tag.ID.String(), Label: tag.Name})
		}
	}

	// For existing transactions, use auto-save with HTMX
	isEdit := txn != nil

	// Build form fields
	formFields := []g.Node{
		g.If(method == "PUT",
			h.Input(h.Type("hidden"), h.Name("_method"), h.Value("PUT")),
		),

		h.Div(
			h.Class("grid grid-cols-2 gap-4"),
			h.Div(
				h.Class("space-y-1.5"),
				shadcn.Label(shadcn.LabelProps{For: "date"},
					g.Text("Date"),
				),
				shadcn.Input(shadcn.InputProps{
					Type:     "date",
					Name:     "date",
					Value:    dateVal,
					Required: true,
				}),
			),
			h.Div(
				h.Class("space-y-1.5"),
				shadcn.Label(shadcn.LabelProps{For: "amount"},
					g.Text("Amount"),
				),
				shadcn.Input(shadcn.InputProps{
					Type:        "number",
					Name:        "amount",
					Placeholder: "0.00",
					Step:        "0.01",
					Value:       strconv.FormatFloat(amountVal, 'f', 2, 64),
					Required:    true,
				}),
			),
		),

		shadcn.FormField(shadcn.FormFieldProps{Name: "description"},
			shadcn.Label(shadcn.LabelProps{For: "description", Required: true},
				g.Text("Description"),
			),
			shadcn.Input(shadcn.InputProps{
				Type:        "text",
				Name:        "description",
				Placeholder: "Transaction description",
				Value:       descriptionVal,
				Required:    true,
			}),
		),

		// Display Title (editable AI-generated name)
		g.If(txn != nil,
			h.Div(
				h.Class("space-y-1.5"),
				shadcn.Label(shadcn.LabelProps{For: "display_title"},
					g.Text("Display Title"),
				),
				shadcn.Input(shadcn.InputProps{
					Type:        "text",
					Name:        "display_title",
					Placeholder: "Friendly name (e.g., Venmo, Starbucks)",
					Value:       displayTitleVal,
				}),
				h.P(
					h.Class("text-xs text-muted-foreground mt-1"),
					g.Text("This is the name shown in lists. Leave blank to use the merchant name or description."),
				),
			),
		),

		// Account selection with clear money flow explanation
		h.Div(
			h.Class("space-y-4"),
			// Visual money flow indicator
			h.Div(
				h.Class("flex items-center justify-center gap-3 py-3 px-4 bg-secondary/50 rounded-lg border border-border"),
				h.Span(h.Class("text-sm text-muted-foreground"), g.Text("Money flows:")),
				h.Span(h.Class("font-medium text-destructive"), g.Text("Source")),
				h.Span(h.Class("text-muted-foreground"), g.Text("→")),
				h.Span(h.Class("font-medium text-chart-2"), g.Text("Destination")),
			),
			h.Div(
				h.Class("grid grid-cols-2 gap-4"),
				h.Div(
					h.Class("space-y-1.5"),
					h.Div(
						h.Class("flex items-center gap-2"),
						h.Span(h.Class("w-2 h-2 rounded-full bg-destructive")),
						shadcn.Label(shadcn.LabelProps{For: "from_account", Required: true},
							g.Text("Source Account"),
						),
					),
					shadcn.NativeSelect(shadcn.NativeSelectProps{
						Name:     "from_account",
						Required: true,
					}, func() []shadcn.SelectOption {
						opts := make([]shadcn.SelectOption, len(accountOptions))
						for i, opt := range accountOptions {
							opts[i] = shadcn.SelectOption{
								Value:    opt.Value,
								Label:    opt.Label,
								Selected: opt.Value == fromAccountIDVal,
							}
						}
						return opts
					}()),
					h.P(
						h.Class("text-xs text-muted-foreground mt-1"),
						g.Text("Balance decreases (−). For income: your Income category. For expenses: your bank account."),
					),
				),
				h.Div(
					h.Class("space-y-1.5"),
					h.Div(
						h.Class("flex items-center gap-2"),
						h.Span(h.Class("w-2 h-2 rounded-full bg-chart-2")),
						shadcn.Label(shadcn.LabelProps{For: "to_account", Required: true},
							g.Text("Destination Account"),
						),
					),
					shadcn.NativeSelect(shadcn.NativeSelectProps{
						Name:     "to_account",
						Required: true,
					}, func() []shadcn.SelectOption {
						opts := make([]shadcn.SelectOption, len(accountOptions))
						for i, opt := range accountOptions {
							opts[i] = shadcn.SelectOption{
								Value:    opt.Value,
								Label:    opt.Label,
								Selected: opt.Value == toAccountIDVal,
							}
						}
						return opts
					}()),
					h.P(
						h.Class("text-xs text-muted-foreground mt-1"),
						g.Text("Balance increases (+). For income: your bank account. For expenses: the Expense category."),
					),
				),
			),
			// Preview of what will happen
			h.Div(
				h.ID("entry-preview"),
				h.Class("text-xs bg-card rounded-lg p-3 border border-border"),
				h.Div(h.Class("text-muted-foreground mb-2 font-medium"), g.Text("Preview of ledger entries:")),
				h.Div(
					h.Class("space-y-1 font-mono"),
					h.Div(
						h.Class("flex justify-between"),
						h.Span(h.Class("text-destructive"), g.Text("Source: −$amount")),
						h.Span(h.Class("text-muted-foreground"), g.Text("(balance goes down)")),
					),
					h.Div(
						h.Class("flex justify-between"),
						h.Span(h.Class("text-chart-2"), g.Text("Destination: +$amount")),
						h.Span(h.Class("text-muted-foreground"), g.Text("(balance goes up)")),
					),
				),
			),
		),

		h.Div(
			h.Class("space-y-1.5"),
			shadcn.Label(shadcn.LabelProps{For: "notes"},
				g.Text("Notes"),
			),
			shadcn.Textarea(shadcn.TextareaProps{
				Name:        "notes",
				Placeholder: "Optional notes...",
				Rows:        3,
				Value:       notesVal,
			}),
		),

		h.Label(
			h.Class("flex items-center gap-2 cursor-pointer"),
			h.Input(
				h.Type("checkbox"),
				h.Name("is_transfer"),
				g.If(isTransferVal, h.Checked()),
				h.Class("w-4 h-4 rounded bg-secondary border-border text-primary"),
			),
			h.Span(h.Class("text-sm text-foreground"), g.Text("This is a transfer between accounts")),
		),
	}

	// Build the form with or without HTMX based on edit mode
	var formNode g.Node
	if isEdit {
		// Auto-save form with HTMX
		formNode = h.Form(
			h.Method("POST"),
			h.Action(action),
			h.Class("space-y-4"),
			g.Attr("hx-post", action),
			g.Attr("hx-trigger", "input delay:250ms, change delay:100ms"),
			g.Attr("hx-swap", "none"),
			g.Attr("hx-indicator", "#save-indicator"),
			g.Group(formFields),
		)
	} else {
		// Regular form for new transactions
		formNode = h.Form(
			h.Method("POST"),
			h.Action(action),
			h.Class("space-y-4"),
			g.Group(formFields),
			h.Div(
				h.Class("flex items-center gap-3 pt-4"),
				h.Button(
					h.Type("submit"),
					h.Class("bg-primary text-primary-foreground rounded-lg px-4 py-2.5 text-sm font-medium hover:opacity-90 transition-colors"),
					g.Text("Create Transaction"),
				),
				h.A(
					h.Href("/transactions"),
					h.Class("text-muted-foreground hover:text-foreground text-sm"),
					g.Text("Cancel"),
				),
			),
		)
	}

	return shadcn.Card(shadcn.CardProps{},
		shadcn.CardContentFull(
			formNode,

			// Tags section (OUTSIDE main form to avoid nested forms) - only for existing transactions
			renderTagsSection(txn, tagOptions),

			// Auto-save indicator (only for edit mode)
			renderAutoSaveIndicator(txn),
		),
	)
}

// renderTagsSection renders the tags section for existing transactions only.
// Uses a separate function to avoid eager evaluation of txn fields when txn is nil.
func renderTagsSection(txn *models.Transaction, tagOptions []ComboboxOption) g.Node {
	if txn == nil {
		return nil
	}

	return h.Div(
		h.Class("space-y-1.5"),
		shadcn.Label(shadcn.LabelProps{For: "tags"},
			g.Text("Tags"),
		),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			// Existing tags as pills with X buttons
			g.Group(g.Map(txn.Tags, func(tag *models.Tag) g.Node {
				return h.Div(
					h.Class("inline-flex items-center gap-1 bg-primary/10 text-primary border border-primary/20 rounded-full pl-2.5 pr-1 py-0.5 text-xs font-medium"),
					g.Text(tag.Name),
					h.Form(
						h.Method("POST"),
						h.Action("/transactions/"+txn.ID.String()+"/tags/"+tag.ID.String()),
						h.Class("inline"),
						h.Input(h.Type("hidden"), h.Name("_method"), h.Value("DELETE")),
						h.Input(h.Type("hidden"), h.Name("redirect"), h.Value("/transactions/"+txn.ID.String()+"/edit")),
						h.Button(
							h.Type("submit"),
							h.Class("text-primary/60 hover:text-destructive transition-colors p-0.5 rounded-full hover:bg-destructive/10"),
							layouts.IconX(),
						),
					),
				)
			})),
			// (+) button with inline combobox using native <details>
			h.Details(
				h.ID("tag-add-combobox"),
				h.Class("relative group"),
				// Summary as the trigger button
				g.El("summary",
					h.Class("w-6 h-6 flex items-center justify-center rounded-full bg-secondary border border-border text-muted-foreground hover:text-foreground hover:border-primary hover:bg-accent transition-colors cursor-pointer list-none [&::-webkit-details-marker]:hidden"),
					layouts.IconPlus(),
				),
				// Dropdown content
				h.Div(
					h.Class("absolute z-50 top-full left-0 mt-1 min-w-48 bg-card border border-border rounded-lg shadow-xl"),
					// Search input
					h.Div(
						h.Class("p-2 border-b border-border"),
						h.Input(
							h.Type("text"),
							h.Placeholder("Search tags..."),
							h.Class("w-full bg-input border border-border rounded px-3 py-1.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"),
							g.Attr("oninput", "filterCombobox('tag-add-combobox', this.value)"),
						),
					),
					// Options list
					h.Div(
						h.Class("max-h-48 overflow-y-auto p-1"),
						g.Group(g.Map(tagOptions, func(opt ComboboxOption) g.Node {
							return h.Form(
								h.Method("POST"),
								h.Action("/transactions/"+txn.ID.String()+"/tags"),
								h.Input(h.Type("hidden"), h.Name("tag_id"), h.Value(opt.Value)),
								h.Input(h.Type("hidden"), h.Name("redirect"), h.Value("/transactions/"+txn.ID.String()+"/edit")),
								h.Button(
									h.Type("submit"),
									h.Class("w-full flex items-center gap-2 px-3 py-2 text-sm text-foreground hover:bg-accent rounded transition-colors text-left whitespace-nowrap"),
									g.Attr("data-combobox-item", ""),
									g.Attr("data-label", opt.Label),
									h.Span(h.Class("flex-1"), g.Text(opt.Label)),
								),
							)
						})),
						g.If(len(tagOptions) == 0,
							h.Div(
								h.Class("px-3 py-2 text-sm text-muted-foreground"),
								g.Text("No more tags available"),
							),
						),
					),
				),
			),
		),
	)
}

// renderAutoSaveIndicator renders the auto-save indicator for edit mode only.
// Uses a separate function to avoid eager evaluation when txn is nil.
func renderAutoSaveIndicator(txn *models.Transaction) g.Node {
	if txn == nil {
		return nil
	}

	return h.Div(
		h.Class("flex items-center justify-end gap-2 text-xs text-muted-foreground pt-2"),
		h.Span(
			h.ID("save-indicator"),
			h.Class("htmx-indicator flex items-center gap-1"),
			g.Raw(`<svg class="animate-spin h-3 w-3" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>`),
			g.Text("Saving..."),
		),
		h.Span(h.Class("text-muted-foreground"), g.Text("Auto-saves")),
	)
}
