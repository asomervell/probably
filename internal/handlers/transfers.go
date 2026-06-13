package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// TransfersList displays pending transfer matches for review
func (hdl *Handlers) TransfersList(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get pending matches
	matches, err := hdl.pendingMatches.GetPendingByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Load transaction details for each match (best-effort enrichment)
	for _, match := range matches {
		if err := hdl.pendingMatches.LoadTransactions(r.Context(), match, hdl.transactions); err != nil {
			slog.WarnContext(r.Context(), "failed to load pending match transactions", "match_id", match.ID, "err", err)
		}
		if match.Transaction != nil {
			if err := hdl.transactions.LoadEntries(r.Context(), match.Transaction); err != nil {
				slog.WarnContext(r.Context(), "failed to load entries", "txn_id", match.Transaction.ID, "err", err)
			}
		}
		if match.CandidateTransaction != nil {
			if err := hdl.transactions.LoadEntries(r.Context(), match.CandidateTransaction); err != nil {
				slog.WarnContext(r.Context(), "failed to load entries", "txn_id", match.CandidateTransaction.ID, "err", err)
			}
		}
	}

	// Count confirmed transfers for stats
	// For now, we'll just show pending
	pendingCount := len(matches)

	// Check for rematch results in query params
	rematchAuto := r.URL.Query().Get("auto")
	rematchPending := r.URL.Query().Get("pending")
	showRematchResults := r.URL.Query().Get("rematch") == "1"

	page := layouts.AppLayout("Transfers", user.Email, user.ID.String(),
		shadcn.PageHeader("Transfer Matching", fmt.Sprintf("%d pending matches to review", pendingCount),
			h.Form(
				h.Method("POST"),
				h.Action("/transfers/rematch"),
				shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonDefault, Type: "submit"},
					layouts.IconRefresh(),
					g.Text("Re-match All"),
				),
			),
		),

		// Rematch results notification
		g.If(showRematchResults,
			shadcn.Alert(shadcn.AlertProps{Variant: shadcn.AlertSuccess},
				shadcn.AlertDescription(g.Text(fmt.Sprintf("✓ Re-matched transfers: %s auto-linked, %s pending review", rematchAuto, rematchPending))),
			),
		),

		// Empty state
		g.If(len(matches) == 0,
			shadcn.Empty(shadcn.EmptyProps{
				Icon:        layouts.IconCheck(),
				Title:       "All caught up!",
				Description: "No pending transfer matches to review.",
			}),
		),

		// Matches list
		g.If(len(matches) > 0,
			h.Div(
				h.Class("space-y-4"),
				g.Group(g.Map(matches, func(match *models.PendingTransferMatch) g.Node {
					return renderPendingMatch(match)
				})),
			),
		),
	)

	renderHTML(w, page)
}

func renderPendingMatch(match *models.PendingTransferMatch) g.Node {
	txn := match.Transaction
	candidate := match.CandidateTransaction

	if txn == nil || candidate == nil {
		return nil
	}

	// Get amounts and PRIMARY account names (asset/liability, not income/expense)
	var txnAmount, candidateAmount int64
	var txnAccount, candidateAccount string

	// For the first transaction, prefer asset/liability accounts
	for _, e := range txn.Entries {
		if e.AccountType != models.AccountTypeIncome &&
			e.AccountType != models.AccountTypeExpense &&
			e.AccountType != models.AccountTypeEquity {
			txnAmount = e.AmountCents
			txnAccount = e.AccountName
			break
		}
	}
	// Fallback if no asset/liability found
	if txnAccount == "" && len(txn.Entries) > 0 {
		txnAmount = txn.Entries[0].AmountCents
		txnAccount = txn.Entries[0].AccountName
	}

	// For the candidate transaction, prefer asset/liability accounts
	for _, e := range candidate.Entries {
		if e.AccountType != models.AccountTypeIncome &&
			e.AccountType != models.AccountTypeExpense &&
			e.AccountType != models.AccountTypeEquity {
			candidateAmount = e.AmountCents
			candidateAccount = e.AccountName
			break
		}
	}
	// Fallback if no asset/liability found
	if candidateAccount == "" && len(candidate.Entries) > 0 {
		candidateAmount = candidate.Entries[0].AmountCents
		candidateAccount = candidate.Entries[0].AccountName
	}

	confidencePercent := int(match.ConfidenceScore * 100)

	return shadcn.Card(shadcn.CardProps{},
		shadcn.CardContentFull(
			// First transaction
			h.Div(
				h.Class("flex items-center justify-between mb-2"),
				h.Div(
					h.P(h.Class("font-medium text-foreground"), g.Text(txn.Description)),
					h.P(h.Class("text-sm text-muted-foreground"),
						g.Text(txn.Date.Format("Jan 2, 2006")),
						g.Text(" • "),
						g.Text(txnAccount),
					),
				),
				h.Span(
					h.Class("font-number font-medium "+simpleAmountColorClass(txnAmount)),
					g.Text(formatMoneyWithSign(txnAmount)),
				),
			),

			// Divider with arrow
			h.Div(
				h.Class("flex items-center gap-2 my-3"),
				h.Div(h.Class("flex-1 border-t border-border")),
				h.Div(
					h.Class("text-muted-foreground text-sm"),
					g.Text("↕"),
				),
				h.Div(h.Class("flex-1 border-t border-border")),
			),

			// Second transaction
			h.Div(
				h.Class("flex items-center justify-between mb-4"),
				h.Div(
					h.P(h.Class("font-medium text-foreground"), g.Text(candidate.Description)),
					h.P(h.Class("text-sm text-muted-foreground"),
						g.Text(candidate.Date.Format("Jan 2, 2006")),
						g.Text(" • "),
						g.Text(candidateAccount),
					),
				),
				h.Span(
					h.Class("font-number font-medium "+simpleAmountColorClass(candidateAmount)),
					g.Text(formatMoneyWithSign(candidateAmount)),
				),
			),

			// Match info and actions
			h.Div(
				h.Class("flex items-center justify-between pt-3 border-t border-border"),
				h.Div(
					h.Class("flex items-center gap-4"),
					// Confidence badge
					h.Span(
						h.Class("text-sm px-2 py-0.5 rounded "+confidenceBadgeClass(confidencePercent)),
						g.Text(fmt.Sprintf("%d%% confidence", confidencePercent)),
					),
					// Reasons
					h.Span(
						h.Class("text-sm text-muted-foreground"),
						g.Text(joinReasons(match.MatchReasons)),
					),
				),
				// Actions
				h.Div(
					h.Class("flex items-center gap-2"),
					// Confirm button
					h.Form(
						h.Method("POST"),
						h.Action("/transfers/"+match.ID.String()+"/confirm"),
						h.Button(
							h.Type("submit"),
							h.Class("inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-chart-2 bg-chart-2/30 rounded-lg hover:bg-chart-2/50 transition-colors"),
							layouts.IconCheck(),
							g.Text("Confirm"),
						),
					),
					// Reject button
					h.Form(
						h.Method("POST"),
						h.Action("/transfers/"+match.ID.String()+"/reject"),
						h.Button(
							h.Type("submit"),
							h.Class("inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-destructive bg-destructive/30 rounded-lg hover:bg-destructive/50 transition-colors"),
							layouts.IconX(),
							g.Text("Not a Transfer"),
						),
					),
				),
			),
		),
	)
}

func confidenceBadgeClass(percent int) string {
	if percent >= 85 {
		return "bg-chart-2/20 text-chart-2"
	} else if percent >= 70 {
		return "bg-ring/20 text-ring"
	}
	return "bg-secondary text-muted-foreground"
}

// renderPendingMatchRow renders a compact inline row for pending transfer matches
// Used in the Transfers tab of the transactions page
func renderPendingMatchRow(match *models.PendingTransferMatch) g.Node {
	txn := match.Transaction
	candidate := match.CandidateTransaction

	if txn == nil || candidate == nil {
		return nil
	}

	// Get amounts and PRIMARY account names (asset/liability, not income/expense)
	var txnAmount int64
	var txnAccount, candidateAccount string

	// For the first transaction, prefer asset/liability accounts
	for _, e := range txn.Entries {
		if e.AccountType != models.AccountTypeIncome &&
			e.AccountType != models.AccountTypeExpense &&
			e.AccountType != models.AccountTypeEquity {
			txnAmount = e.AmountCents
			txnAccount = e.AccountName
			break
		}
	}
	// Fallback if no asset/liability found
	if txnAccount == "" && len(txn.Entries) > 0 {
		txnAmount = txn.Entries[0].AmountCents
		txnAccount = txn.Entries[0].AccountName
	}

	// For the candidate transaction, prefer asset/liability accounts
	for _, e := range candidate.Entries {
		if e.AccountType != models.AccountTypeIncome &&
			e.AccountType != models.AccountTypeExpense &&
			e.AccountType != models.AccountTypeEquity {
			candidateAccount = e.AccountName
			break
		}
	}
	// Fallback if no asset/liability found
	if candidateAccount == "" && len(candidate.Entries) > 0 {
		candidateAccount = candidate.Entries[0].AccountName
	}

	confidencePercent := int(match.ConfidenceScore * 100)

	return h.Div(
		h.ID("match-"+match.ID.String()),
		h.Class("flex items-center gap-3 px-4 py-3 bg-ring/5 border-b border-ring/20 hover:bg-ring/10 transition-colors"),
		// Left: Match indicator icon
		h.Div(
			h.Class("flex-none w-8 h-8 rounded-full bg-ring/20 flex items-center justify-center text-ring"),
			g.Raw(`<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"/></svg>`),
		),
		// Center: Transaction details - takes remaining space
		h.Div(
			h.Class("flex-1 min-w-0"),
			h.Div(
				h.Class("flex items-center gap-2 text-sm"),
				h.Span(h.Class("font-medium text-foreground truncate"), g.Text(txn.Description)),
				h.Span(h.Class("text-muted-foreground"), g.Text("↔")),
				h.Span(h.Class("font-medium text-foreground truncate"), g.Text(candidate.Description)),
			),
			h.Div(
				h.Class("flex items-center gap-2 text-xs text-muted-foreground mt-0.5"),
				h.Span(g.Text(txnAccount)),
				h.Span(g.Text("→")),
				h.Span(g.Text(candidateAccount)),
				h.Span(h.Class("text-muted-foreground"), g.Text("•")),
				h.Span(g.Text(txn.Date.Format("Jan 2"))),
				h.Span(h.Class("text-muted-foreground"), g.Text("•")),
				h.Span(
					h.Class(func() string {
						if confidencePercent >= 85 {
							return "text-chart-2"
						} else if confidencePercent >= 70 {
							return "text-ring"
						}
						return "text-muted-foreground"
					}()),
					g.Textf("%d%%", confidencePercent),
				),
			),
		),
		// Amount
		h.Div(
			h.Class("flex-none text-right pr-2"),
			h.Span(
				h.Class("font-number font-medium "+simpleAmountColorClass(txnAmount)),
				g.Text(formatMoneyWithSign(txnAmount)),
			),
		),
		// Actions - compact buttons with HTMX
		h.Div(
			h.Class("flex-none flex items-center gap-1"),
			// Confirm button
			h.Button(
				h.Type("button"),
				h.Class("p-1.5 text-chart-2 hover:bg-chart-2/10 rounded transition-colors"),
				h.Title("Confirm match"),
				g.Attr("hx-post", "/transfers/"+match.ID.String()+"/confirm"),
				g.Attr("hx-target", "#match-"+match.ID.String()),
				g.Attr("hx-swap", "outerHTML"),
				layouts.IconCheck(),
			),
			// Reject button
			h.Button(
				h.Type("button"),
				h.Class("p-1.5 text-destructive hover:bg-destructive/10 rounded transition-colors"),
				h.Title("Not a transfer"),
				g.Attr("hx-post", "/transfers/"+match.ID.String()+"/reject"),
				g.Attr("hx-target", "#match-"+match.ID.String()),
				g.Attr("hx-swap", "outerHTML"),
				layouts.IconX(),
			),
		),
	)
}

func joinReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	if len(reasons) == 1 {
		return reasons[0]
	}
	if len(reasons) <= 3 {
		result := reasons[0]
		for i := 1; i < len(reasons); i++ {
			result += ", " + reasons[i]
		}
		return result
	}
	return reasons[0] + ", " + reasons[1] + " +" + strconv.Itoa(len(reasons)-2) + " more"
}

// TransfersConfirm confirms a pending transfer match
func (hdl *Handlers) TransfersConfirm(w http.ResponseWriter, r *http.Request) {
	matchID, ok := mustParamUUID(w, r, "id", "match ID")
	if !ok {
		return
	}

	if err := hdl.transferMatcher.ConfirmMatch(r.Context(), matchID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// HTMX: return empty string to remove the row
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		// Return empty - the row will be removed via hx-swap="outerHTML"
		return
	}

	http.Redirect(w, r, "/transactions?filter=transfers", http.StatusSeeOther)
}

// TransfersReject rejects a pending transfer match
func (hdl *Handlers) TransfersReject(w http.ResponseWriter, r *http.Request) {
	matchID, ok := mustParamUUID(w, r, "id", "match ID")
	if !ok {
		return
	}

	if err := hdl.transferMatcher.RejectMatch(r.Context(), matchID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// HTMX: return empty string to remove the row
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		// Return empty - the row will be removed via hx-swap="outerHTML"
		return
	}

	http.Redirect(w, r, "/transactions?filter=transfers", http.StatusSeeOther)
}

// TransfersManualMatch manually links two transactions as a transfer
func (hdl *Handlers) TransfersManualMatch(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	txn1ID, ok := mustFormParamUUID(w, r, "transaction_id", "transaction ID")
	if !ok {
		return
	}

	txn2ID, ok := mustFormParamUUID(w, r, "candidate_id", "candidate ID")
	if !ok {
		return
	}

	if err := hdl.transferMatcher.ManualMatch(r.Context(), txn1ID, txn2ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// HTMX or redirect
	htmxRedirect(w, r, "/transactions/"+txn1ID.String())
}

// TransfersUnlink unlinks a transfer pair
func (hdl *Handlers) TransfersUnlink(w http.ResponseWriter, r *http.Request) {
	txnID, ok := mustParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	if err := hdl.transferMatcher.UnlinkTransfer(r.Context(), txnID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// HTMX or redirect
	htmxRedirect(w, r, "/transactions/"+txnID.String())
}

// Helper functions are in dashboard.go: simpleAmountColorClass, formatMoneyWithSign

// TransfersRematch re-runs transfer matching on all unmatched transactions
// This is useful after fixing matching logic bugs or when matches were incorrectly rejected
func (hdl *Handlers) TransfersRematch(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get all asset/liability accounts
	accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var totalAutoLinked, totalPending int

	for _, acc := range accounts {
		// Only process asset and liability accounts
		if acc.Type != models.AccountTypeAsset && acc.Type != models.AccountTypeLiability {
			continue
		}

		autoLinked, pending, err := hdl.transferMatcher.MatchAllForAccountWithStats(r.Context(), acc)
		if err != nil {
			continue
		}
		totalAutoLinked += autoLinked
		totalPending += pending
	}

	slog.InfoContext(r.Context(), "transfers re-matched", "auto_linked", totalAutoLinked, "pending", totalPending)

	// Redirect to transactions page with transfers filter
	http.Redirect(w, r, fmt.Sprintf("/transactions?filter=transfers&rematch=1&auto=%d&pending=%d", totalAutoLinked, totalPending), http.StatusSeeOther)
}
