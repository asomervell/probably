package handlers

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"log/slog"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	"github.com/google/uuid"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// PatternsList shows all detected patterns aggregated from transactions
func (hdl *Handlers) PatternsList(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	store := models.NewPatternStore(hdl.db.Pool)

	// Get all patterns aggregated from transactions
	patterns, err := store.GetAggregatedPatterns(r.Context(), ledger.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to fetch patterns", "ledger_id", ledger.ID.String(), "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use start of today in UTC for date comparisons
	// Transaction dates from Teller are date-only (no time), stored/calculated as UTC midnight
	// So we compare against start of today UTC to avoid local timezone issues
	nowUTC := time.Now().UTC()
	todayUTC := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)

	// Separate patterns by type and active/inactive status
	var recurringBills []*models.AggregatedPattern
	var salaryPatterns []*models.AggregatedPattern
	var otherPatterns []*models.AggregatedPattern
	var inactivePatterns []*models.AggregatedPattern

	for _, p := range patterns {
		// Check if pattern is inactive (next expected date is before today UTC)
		// NextExpected is calculated from transaction dates which are UTC midnight
		isInactive := p.NextExpected != nil && p.NextExpected.Before(todayUTC)

		if isInactive {
			inactivePatterns = append(inactivePatterns, p)
			continue
		}

		switch p.PatternType {
		case "recurring_bill":
			recurringBills = append(recurringBills, p)
		case "salary":
			salaryPatterns = append(salaryPatterns, p)
		default:
			otherPatterns = append(otherPatterns, p)
		}
	}

	// Group patterns by entity for better display
	recurringBillGroups := groupPatternsByEntity(recurringBills)
	salaryGroups := groupPatternsByEntity(salaryPatterns)
	otherGroups := groupPatternsByEntity(otherPatterns)
	inactiveGroups := groupPatternsByEntity(inactivePatterns)

	// Calculate monthly total from CURRENT recurring bills only
	var monthlyTotal int64
	for _, p := range recurringBills {
		monthlyTotal += toMonthlyCents(p.AvgAmountCents, p.Frequency)
	}

	pageContent := renderPatternsList(
		user.Email,
		user.ID.String(),
		recurringBillGroups,
		salaryGroups,
		otherGroups,
		inactiveGroups,
		monthlyTotal,
		hdl.getLogoURL,
	)

	renderHTML(w, pageContent)
}

// groupPatternsByEntity groups patterns by their entity, preserving order
func groupPatternsByEntity(patterns []*models.AggregatedPattern) []*models.EntityPatternGroup {
	if len(patterns) == 0 {
		return nil
	}

	// Use a map to collect patterns by entity key, maintain order with slice
	groupMap := make(map[string]*models.EntityPatternGroup)
	var groupOrder []string

	for _, p := range patterns {
		// Create a key for grouping (entity_id or "null" for no entity)
		key := "null"
		if p.EntityID != nil {
			key = p.EntityID.String()
		}

		group, exists := groupMap[key]
		if !exists {
			group = &models.EntityPatternGroup{
				EntityID:   p.EntityID,
				EntityName: p.EntityName,
				EntityLogo: p.EntityLogo,
			}
			groupMap[key] = group
			groupOrder = append(groupOrder, key)
		}
		group.Patterns = append(group.Patterns, p)
		group.TotalOccurrences += p.OccurrenceCount

		group.TotalMonthlyCents += toMonthlyCents(p.AvgAmountCents, p.Frequency)
	}

	// Build result slice in order
	var groups []*models.EntityPatternGroup
	for _, key := range groupOrder {
		groups = append(groups, groupMap[key])
	}

	return groups
}

// PatternsDetail shows details for a single pattern (by entity_id, pattern_type, and pattern_name)
func (hdl *Handlers) PatternsDetail(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get parameters from query string
	entityIDStr := r.URL.Query().Get("entity")
	patternType := r.URL.Query().Get("type")
	patternName := r.URL.Query().Get("name") // New: specific subscription name

	if patternType == "" {
		http.Error(w, "Pattern type required", http.StatusBadRequest)
		return
	}

	var entityID *uuid.UUID
	if entityIDStr != "" && entityIDStr != "null" {
		parsed, err := uuid.Parse(entityIDStr)
		if err != nil {
			http.Error(w, "Invalid entity ID", http.StatusBadRequest)
			return
		}
		entityID = &parsed
	}

	store := models.NewPatternStore(hdl.db.Pool)

	detail, err := store.GetPatternDetail(r.Context(), ledger.ID, entityID, patternType, patternName)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to fetch pattern detail", "entity_id", entityIDStr, "pattern_type", patternType, "pattern_name", patternName, "error", err)
		http.Error(w, "Pattern not found", http.StatusNotFound)
		return
	}

	pageContent := renderPatternDetail(user.Email, user.ID.String(), detail, hdl.getLogoURL)

	renderHTML(w, pageContent)
}

// PatternsDismiss marks a pattern as "not a pattern" - updates transactions to be immune from reprocessing
func (hdl *Handlers) PatternsDismiss(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get parameters from form
	entityIDStr := r.FormValue("entity")
	patternType := r.FormValue("type")
	patternName := r.FormValue("name")

	if patternType == "" {
		http.Error(w, "Pattern type required", http.StatusBadRequest)
		return
	}

	var entityIDValue interface{}
	if entityIDStr != "" && entityIDStr != "null" {
		parsed, err := uuid.Parse(entityIDStr)
		if err != nil {
			http.Error(w, "Invalid entity ID", http.StatusBadRequest)
			return
		}
		entityIDValue = parsed
	}

	// Update all transactions matching this pattern to be dismissed
	// pattern_type = 'dismissed' makes them immune from refresh
	result, err := hdl.db.Pool.Exec(r.Context(), `
		UPDATE transactions t SET
			pattern_type = 'dismissed',
			pattern_metadata = jsonb_build_object(
				'dismissed_at', NOW(),
				'original_type', t.pattern_type,
				'original_name', COALESCE(t.pattern_metadata->>'pattern_name', ''),
				'reason', 'user_dismissed'
			),
			pattern_detection_status = 'done',
			updated_at = NOW()
		WHERE t.ledger_id = $1
			AND t.pattern_type = $2
			AND (($3::UUID IS NULL AND t.entity_id IS NULL) OR t.entity_id = $3)
			AND COALESCE(NULLIF(t.pattern_metadata->>'pattern_name', ''), '') = $4
			AND t.pattern_detection_status = 'done'
	`, ledger.ID, patternType, entityIDValue, patternName)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to dismiss pattern",
			"ledger_id", ledger.ID.String(),
			"entity_id", entityIDStr,
			"pattern_type", patternType,
			"error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dismissed := result.RowsAffected()
	slog.InfoContext(r.Context(), "Pattern dismissed",
		"ledger_id", ledger.ID.String(),
		"entity_id", entityIDStr,
		"pattern_type", patternType,
		"pattern_name", patternName,
		"transactions_updated", dismissed)

	// Redirect back to patterns list
	http.Redirect(w, r, "/patterns", http.StatusSeeOther)
}

// PatternsRename updates the display name of a pattern
func (hdl *Handlers) PatternsRename(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get parameters from form
	entityIDStr := r.FormValue("entity")
	patternType := r.FormValue("type")
	oldName := r.FormValue("old_name")
	newName := strings.TrimSpace(r.FormValue("new_name"))

	if patternType == "" || newName == "" {
		http.Error(w, "Pattern type and new name required", http.StatusBadRequest)
		return
	}

	var entityIDValue interface{}
	if entityIDStr != "" && entityIDStr != "null" {
		parsed, err := uuid.Parse(entityIDStr)
		if err != nil {
			http.Error(w, "Invalid entity ID", http.StatusBadRequest)
			return
		}
		entityIDValue = parsed
	}

	// Update pattern_name in pattern_metadata for all matching transactions
	result, err := hdl.db.Pool.Exec(r.Context(), `
		UPDATE transactions t SET
			pattern_metadata = jsonb_set(
				COALESCE(t.pattern_metadata, '{}'),
				'{pattern_name}',
				to_jsonb($5::text)
			),
			updated_at = NOW()
		WHERE t.ledger_id = $1
			AND t.pattern_type = $2
			AND (($3::UUID IS NULL AND t.entity_id IS NULL) OR t.entity_id = $3)
			AND COALESCE(NULLIF(t.pattern_metadata->>'pattern_name', ''), '') = $4
			AND t.pattern_detection_status = 'done'
	`, ledger.ID, patternType, entityIDValue, oldName, newName)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to rename pattern",
			"ledger_id", ledger.ID.String(),
			"entity_id", entityIDStr,
			"pattern_type", patternType,
			"old_name", oldName,
			"new_name", newName,
			"error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	updated := result.RowsAffected()
	slog.InfoContext(r.Context(), "Pattern renamed",
		"ledger_id", ledger.ID.String(),
		"entity_id", entityIDStr,
		"pattern_type", patternType,
		"old_name", oldName,
		"new_name", newName,
		"transactions_updated", updated)

	// Redirect back to pattern detail with new name
	redirectURL := fmt.Sprintf("/patterns/detail?entity=%s&type=%s&name=%s",
		entityIDStr, url.QueryEscape(patternType), url.QueryEscape(newName))
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// PatternsRefresh queues all transactions for LLM-based pattern detection
func (hdl *Handlers) PatternsRefresh(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Reset pattern detection for all categorized transactions
	// This clears existing patterns and re-queues for LLM analysis
	// IMPORTANT: Excludes 'dismissed' patterns - those are user-confirmed as NOT patterns
	result, err := hdl.db.Pool.Exec(r.Context(), `
		UPDATE transactions SET
			pattern_detection_status = 'queued',
			pattern_detection_attempts = 0,
			pattern_detection_error = NULL,
			pattern_type = NULL,
			pattern_metadata = NULL,
			updated_at = NOW()
		WHERE ledger_id = $1
			AND categorization_status = 'done'
			AND is_transfer = false
			AND (pattern_type IS NULL OR pattern_type != 'dismissed')
	`, ledger.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to queue transactions for pattern detection", "ledger_id", ledger.ID.String(), "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	queued := result.RowsAffected()
	slog.InfoContext(r.Context(), "Queued transactions for LLM pattern detection",
		"ledger_id", ledger.ID.String(),
		"transactions_queued", queued)

	// Redirect back to patterns list
	http.Redirect(w, r, "/patterns", http.StatusSeeOther)
}

// renderPatternsList renders the patterns list page using grouped entity data
func renderPatternsList(
	userEmail string,
	posthogDistinctID string,
	recurringBillGroups []*models.EntityPatternGroup,
	salaryGroups []*models.EntityPatternGroup,
	otherGroups []*models.EntityPatternGroup,
	inactiveGroups []*models.EntityPatternGroup,
	monthlyTotal int64,
	getLogoURL func(string) string,
) g.Node {
	totalCurrentGroups := len(recurringBillGroups) + len(salaryGroups) + len(otherGroups)
	totalGroups := totalCurrentGroups + len(inactiveGroups)

	return layouts.AppLayout("Patterns", userEmail, posthogDistinctID,
		// Page header with totals
		h.Div(
			h.Class("flex items-center justify-between mb-6"),
			h.Div(
				h.H1(h.Class("text-2xl font-semibold text-foreground"), g.Text("Patterns")),
				h.P(h.Class("text-muted-foreground mt-1"), g.Text("AI-detected recurring patterns from your transactions")),
			),
			g.If(monthlyTotal > 0,
				h.Div(
					h.Class("text-right"),
					h.P(h.Class("text-2xl font-semibold font-number text-foreground"), g.Text(formatMoney(monthlyTotal)+"/mo")),
					h.P(h.Class("text-muted-foreground text-sm font-number"), g.Text(formatMoney(monthlyTotal*12)+"/yr")),
				),
			),
		),

		// Refresh button
		h.Div(
			h.Class("mb-6"),
			h.Form(
				h.Action("/patterns/refresh"),
				h.Method("POST"),
				shadcn.Button(shadcn.ButtonProps{
					Variant: shadcn.ButtonSecondary,
					Type:    "submit",
				},
					g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/><path d="M16 16h5v5"/></svg>`),
					g.Text("Refresh Detection"),
				),
			),
		),

		// Empty state
		g.If(totalGroups == 0,
			h.Div(
				h.Class("text-center py-16"),
				h.Div(
					h.Class("text-muted-foreground mb-4"),
					g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="mx-auto"><path d="M21 12V7H5a2 2 0 0 1 0-4h14v4"/><path d="M3 5v14a2 2 0 0 0 2 2h16v-5"/><path d="M18 12a2 2 0 0 0 0 4h4v-4Z"/></svg>`),
				),
				h.H2(h.Class("text-xl font-medium text-foreground mb-2"), g.Text("No patterns detected yet")),
				h.P(h.Class("text-muted-foreground mb-6 max-w-md mx-auto"), g.Text("Click 'Refresh Detection' to analyze your transactions for recurring patterns like subscriptions, salary, and bills.")),
				h.Form(
					h.Action("/patterns/refresh"),
					h.Method("POST"),
					shadcn.Button(shadcn.ButtonProps{
						Variant: shadcn.ButtonDefault,
						Type:    "submit",
					},
						g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/><path d="M16 16h5v5"/></svg>`),
						g.Text("Run Detection"),
					),
				),
			),
		),

		// Current Patterns section header (only if there are current patterns)
		g.If(totalCurrentGroups > 0,
			h.Div(
				h.Class("mb-4"),
				h.H2(h.Class("text-lg font-medium text-foreground"), g.Text("Current Patterns")),
			),
		),

		// Recurring Bills section
		g.If(len(recurringBillGroups) > 0,
			h.Div(
				h.Class("mb-8"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Recurring Bills", strconv.Itoa(countPatternsInGroups(recurringBillGroups))+" subscriptions"),
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							g.Group(g.Map(recurringBillGroups, func(group *models.EntityPatternGroup) g.Node {
								return renderEntityGroup(group, getLogoURL, false)
							})),
						),
					),
				),
			),
		),

		// Salary patterns section
		g.If(len(salaryGroups) > 0,
			h.Div(
				h.Class("mb-8"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Income Patterns", strconv.Itoa(countPatternsInGroups(salaryGroups))+" patterns"),
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							g.Group(g.Map(salaryGroups, func(group *models.EntityPatternGroup) g.Node {
								return renderEntityGroup(group, getLogoURL, false)
							})),
						),
					),
				),
			),
		),

		// Other patterns section
		g.If(len(otherGroups) > 0,
			h.Div(
				h.Class("mb-8"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Other Patterns", strconv.Itoa(countPatternsInGroups(otherGroups))+" patterns"),
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							g.Group(g.Map(otherGroups, func(group *models.EntityPatternGroup) g.Node {
								return renderEntityGroup(group, getLogoURL, false)
							})),
						),
					),
				),
			),
		),

		// Past Patterns section (at the bottom, dimmed)
		g.If(len(inactiveGroups) > 0,
			h.Div(
				h.Class("mt-8 pt-8 border-t border-border"),
				h.Div(
					h.Class("mb-4"),
					h.H2(h.Class("text-lg font-medium text-muted-foreground"), g.Text("Past Patterns")),
					h.P(h.Class("text-muted-foreground text-sm"), g.Text("Patterns that haven't occurred recently")),
				),
				shadcn.Card(shadcn.CardProps{Class: "opacity-70"},
					shadcn.CardContent(
						h.Div(
							h.Class("divide-y divide-border"),
							g.Group(g.Map(inactiveGroups, func(group *models.EntityPatternGroup) g.Node {
								return renderEntityGroup(group, getLogoURL, true)
							})),
						),
					),
				),
			),
		),
	)
}

// countPatternsInGroups returns total number of patterns across all groups
func countPatternsInGroups(groups []*models.EntityPatternGroup) int {
	count := 0
	for _, g := range groups {
		count += len(g.Patterns)
	}
	return count
}

// renderEntityGroup renders an entity with its patterns (expandable if multiple)
func renderEntityGroup(group *models.EntityPatternGroup, getLogoURL func(string) string, isInactive bool) g.Node {
	// If only one pattern, render it directly
	if len(group.Patterns) == 1 {
		if isInactive {
			return renderInactivePatternRow(group.Patterns[0], getLogoURL)
		}
		return renderPatternRow(group.Patterns[0], getLogoURL)
	}

	// Multiple patterns - render as expandable group
	return renderEntityGroupExpanded(group, getLogoURL, isInactive)
}

// renderEntityGroupExpanded renders an entity with multiple subscriptions
func renderEntityGroupExpanded(group *models.EntityPatternGroup, getLogoURL func(string) string, isInactive bool) g.Node {
	name := group.EntityName
	if name == "" {
		name = "Unknown"
	}

	textClass := "text-foreground"
	if isInactive {
		textClass = "text-muted-foreground"
	}

	return h.Div(
		h.Class(""),
		// Entity header row
		h.Div(
			h.Class("flex items-center gap-4 p-4"),
			// Logo
			renderEntityGroupLogo(group, getLogoURL),
			// Entity info
			h.Div(
				h.Class("flex-1 min-w-0"),
				h.P(h.Class(textClass+" font-medium truncate"), g.Text(name)),
				h.Div(
					h.Class("flex items-center gap-2 text-muted-foreground text-sm"),
					h.Span(g.Textf("%d subscriptions", len(group.Patterns))),
					h.Span(g.Text("·")),
					h.Span(g.Textf("%d occurrences", group.TotalOccurrences)),
				),
			),
			// Total amount
			h.Div(
				h.Class("text-right"),
				h.P(h.Class("font-number "+textClass), g.Text(formatMoney(group.TotalMonthlyCents)+"/mo")),
				h.P(h.Class("text-muted-foreground text-xs font-number"), g.Text(formatMoney(group.TotalMonthlyCents*12)+"/yr")),
			),
		),
		// Nested subscriptions
		h.Div(
			h.Class("pl-14 border-l-2 border-border ml-5"),
			g.Group(g.Map(group.Patterns, func(p *models.AggregatedPattern) g.Node {
				return renderNestedPatternRow(p, getLogoURL, isInactive)
			})),
		),
	)
}

// renderNestedPatternRow renders a pattern row nested under an entity group
func renderNestedPatternRow(p *models.AggregatedPattern, getLogoURL func(string) string, isInactive bool) g.Node {
	// Use pattern name if available, otherwise fall back to entity name
	name := p.PatternName
	if name == "" {
		name = p.EntityName
		if name == "" {
			name = "Unknown"
		}
	}

	// Build detail URL with entity_id, pattern_type, and pattern_name
	detailURL := buildPatternDetailURL(p)

	textClass := "text-foreground"
	amountClass := "text-foreground"
	if isInactive {
		textClass = "text-muted-foreground"
		amountClass = "text-muted-foreground"
	}

	return h.A(
		h.Href(detailURL),
		h.Class("flex items-center gap-3 py-3 px-4 hover:bg-accent transition-colors cursor-pointer"),
		// Subscription name
		h.Div(
			h.Class("flex-1 min-w-0"),
			h.P(h.Class(textClass+" font-medium truncate text-sm"), g.Text(name)),
			h.Div(
				h.Class("flex items-center gap-2 text-muted-foreground text-xs"),
				h.Span(g.Text(formatFrequencyDisplay(p.Frequency))),
				h.Span(g.Text("·")),
				h.Span(g.Textf("%d occurrences", p.OccurrenceCount)),
			),
		),
		// Amount
		h.Div(
			h.Class("text-right"),
			h.P(h.Class("font-number text-sm "+amountClass), g.Text(formatMoney(p.AvgAmountCents)+"/"+formatFrequencyShortDisplay(p.Frequency))),
			g.If(!isInactive && p.NextExpected != nil,
				h.P(h.Class("text-muted-foreground text-xs"), g.Text("Next: "+p.NextExpected.Format("Jan 2"))),
			),
			g.If(isInactive,
				h.P(h.Class("text-muted-foreground text-xs"), g.Text("Last: "+p.LastOccurrence.Format("Jan 2"))),
			),
		),
		// Arrow indicator
		h.Div(
			h.Class("text-muted-foreground"),
			g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>`),
		),
	)
}

// renderEntityGroupLogo renders the logo for an entity group
func renderEntityGroupLogo(group *models.EntityPatternGroup, getLogoURL func(string) string) g.Node {
	if group.EntityLogo != "" {
		logoURL := getLogoURL(group.EntityLogo)
		return h.Div(
			h.Class("flex-none w-10 h-10 rounded-lg bg-secondary overflow-hidden"),
			h.Img(
				h.Src(logoURL),
				h.Alt(group.EntityName),
				h.Class("w-full h-full object-contain"),
			),
		)
	}

	// Fallback with initials
	initials := "?"
	if group.EntityName != "" {
		initials = strings.ToUpper(string(group.EntityName[0]))
	}
	return h.Div(
		h.Class("flex-none w-10 h-10 rounded-lg bg-secondary flex items-center justify-center text-muted-foreground font-medium"),
		g.Text(initials),
	)
}

// buildPatternDetailURL builds the detail URL for a pattern
func buildPatternDetailURL(p *models.AggregatedPattern) string {
	var entityParam string
	if p.EntityID != nil {
		entityParam = p.EntityID.String()
	} else {
		entityParam = "null"
	}

	detailURL := fmt.Sprintf("/patterns/detail?entity=%s&type=%s", entityParam, url.QueryEscape(p.PatternType))
	if p.PatternName != "" {
		detailURL += "&name=" + url.QueryEscape(p.PatternName)
	}
	return detailURL
}

// renderInactivePatternRow renders a past/inactive pattern row with "Last seen" instead of "Next"
func renderInactivePatternRow(p *models.AggregatedPattern, getLogoURL func(string) string) g.Node {
	// Use pattern name if available, otherwise entity name
	name := p.PatternName
	if name == "" {
		name = p.EntityName
	}
	if name == "" {
		name = "Unknown"
	}

	return h.A(
		h.Href(buildPatternDetailURL(p)),
		h.Class("flex items-center gap-4 p-4 hover:bg-accent transition-colors cursor-pointer"),
		// Logo
		renderPatternLogo(p, getLogoURL),
		// Info
		h.Div(
			h.Class("flex-1 min-w-0"),
			h.P(h.Class("text-muted-foreground font-medium truncate"), g.Text(name)),
			h.Div(
				h.Class("flex items-center gap-2 text-muted-foreground text-sm"),
				h.Span(g.Text(formatFrequencyDisplay(p.Frequency))),
				h.Span(g.Text("·")),
				h.Span(g.Textf("%d occurrences", p.OccurrenceCount)),
			),
		),
		// Amount and last seen
		h.Div(
			h.Class("text-right"),
			h.P(h.Class("font-number text-muted-foreground"), g.Text(formatMoney(p.AvgAmountCents)+"/"+formatFrequencyShortDisplay(p.Frequency))),
			h.P(h.Class("text-muted-foreground text-xs"), g.Text("Last: "+p.LastOccurrence.Format("Jan 2"))),
		),
		// Arrow indicator
		h.Div(
			h.Class("text-muted-foreground"),
			g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>`),
		),
	)
}

// renderPatternRow renders a single pattern in the list (clickable)
func renderPatternRow(p *models.AggregatedPattern, getLogoURL func(string) string) g.Node {
	// Use pattern name if available, otherwise entity name
	name := p.PatternName
	if name == "" {
		name = p.EntityName
	}
	if name == "" {
		name = "Unknown"
	}

	return h.A(
		h.Href(buildPatternDetailURL(p)),
		h.Class("flex items-center gap-4 p-4 hover:bg-accent transition-colors cursor-pointer"),
		// Logo
		renderPatternLogo(p, getLogoURL),
		// Info
		h.Div(
			h.Class("flex-1 min-w-0"),
			h.P(h.Class("text-foreground font-medium truncate"), g.Text(name)),
			h.Div(
				h.Class("flex items-center gap-2 text-muted-foreground text-sm"),
				h.Span(g.Text(formatFrequencyDisplay(p.Frequency))),
				h.Span(g.Text("·")),
				h.Span(g.Textf("%d occurrences", p.OccurrenceCount)),
				h.Span(g.Text("·")),
				h.Span(g.Textf("%d%% confident", p.AvgConfidence)),
			),
		),
		// Amount
		h.Div(
			h.Class("text-right"),
			h.P(h.Class("font-number text-foreground"), g.Text(formatMoney(p.AvgAmountCents)+"/"+formatFrequencyShortDisplay(p.Frequency))),
			g.If(p.NextExpected != nil,
				h.P(h.Class("text-muted-foreground text-xs"), g.Text("Next: "+p.NextExpected.Format("Jan 2"))),
			),
		),
		// Arrow indicator
		h.Div(
			h.Class("text-muted-foreground"),
			g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>`),
		),
	)
}

// renderPatternLogo renders the entity logo or fallback
func renderPatternLogo(p *models.AggregatedPattern, getLogoURL func(string) string) g.Node {
	if p.EntityLogo != "" {
		logoURL := getLogoURL(p.EntityLogo)
		return h.Div(
			h.Class("flex-none w-10 h-10 rounded-lg bg-secondary overflow-hidden"),
			h.Img(
				h.Src(logoURL),
				h.Alt(p.EntityName),
				h.Class("w-full h-full object-contain"),
			),
		)
	}

	// Fallback with first letter
	initial := "?"
	if p.EntityName != "" {
		initial = string(p.EntityName[0])
	}
	return h.Div(
		h.Class("flex-none w-10 h-10 rounded-lg bg-gradient-to-br from-accent to-accent/80 flex items-center justify-center text-accent-foreground font-medium"),
		g.Text(initial),
	)
}


// formatFrequencyDisplay formats frequency for display
func formatFrequencyDisplay(f string) string {
	switch f {
	case "weekly":
		return "Weekly"
	case "biweekly":
		return "Biweekly"
	case "monthly":
		return "Monthly"
	case "quarterly":
		return "Quarterly"
	case "annual":
		return "Annual"
	default:
		return f
	}
}

// formatFrequencyShortDisplay formats frequency for compact display
func formatFrequencyShortDisplay(f string) string {
	switch f {
	case "weekly":
		return "wk"
	case "biweekly":
		return "2wk"
	case "monthly":
		return "mo"
	case "quarterly":
		return "qtr"
	case "annual":
		return "yr"
	default:
		return f
	}
}

// getFirstChar safely returns the first character of a string, or "?" if empty
func getFirstChar(s string) string {
	if s == "" {
		return "?"
	}
	return string(s[0])
}

// renderPatternDetail renders the pattern detail page with reasoning and transactions
func renderPatternDetail(
	userEmail string,
	posthogDistinctID string,
	detail *models.PatternDetail,
	getLogoURL func(string) string,
) g.Node {
	p := detail.Pattern

	// Calculate annual cost for display
	annualCost := calculatePatternAnnualCost(p.AvgAmountCents, p.Frequency)

	// Format pattern type for display
	patternTypeDisplay := formatPatternTypeDisplay(p.PatternType)

	// Determine display title - use pattern name if available, otherwise entity name
	displayTitle := p.PatternName
	if displayTitle == "" {
		displayTitle = p.EntityName
	}

	// Page title for browser tab
	pageTitle := displayTitle
	if p.PatternName != "" && p.EntityName != "" && p.PatternName != p.EntityName {
		pageTitle = p.PatternName + " - " + p.EntityName
	}

	return layouts.AppLayout(pageTitle, userEmail, posthogDistinctID,
		// Back link
		h.A(
			h.Href("/patterns"),
			h.Class("inline-flex items-center text-muted-foreground hover:text-foreground mb-6"),
			g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mr-2"><path d="m15 18-6-6 6-6"/></svg>`),
			g.Text("Back to Patterns"),
		),

		// Header with logo
		h.Div(
			h.Class("flex items-start gap-4 mb-8"),
			// Logo
			h.Div(
				h.Class("w-16 h-16 rounded-xl overflow-hidden bg-secondary"),
				g.If(p.EntityLogo != "",
					h.Img(h.Src(getLogoURL(p.EntityLogo)), h.Alt(p.EntityName), h.Class("w-full h-full object-contain")),
				),
				g.If(p.EntityLogo == "",
					h.Div(
						h.Class("w-full h-full bg-gradient-to-br from-accent to-accent/80 flex items-center justify-center text-accent-foreground text-2xl font-medium"),
						g.Text(getFirstChar(p.EntityName)),
					),
				),
			),
			// Name and type (editable)
			h.Div(
				h.Class("flex-1"),
				// Display mode - title with edit button
				h.Div(
					h.ID("pattern-name-display"),
					h.Class("flex items-center gap-2"),
					h.H1(h.Class("text-2xl font-semibold text-foreground"), g.Text(displayTitle)),
					h.Button(
						h.Type("button"),
						h.Class("text-muted-foreground hover:text-foreground transition-colors p-1"),
						g.Attr("onclick", "document.getElementById('pattern-name-display').classList.add('hidden'); document.getElementById('pattern-name-edit').classList.remove('hidden'); document.getElementById('new-pattern-name').focus();"),
						h.Title("Edit pattern name"),
						g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/><path d="m15 5 4 4"/></svg>`),
					),
				),
				// Edit mode - inline form (hidden by default)
				h.Form(
					h.ID("pattern-name-edit"),
					h.Class("hidden"),
					h.Action("/patterns/rename"),
					h.Method("POST"),
					h.Input(h.Type("hidden"), h.Name("entity"), h.Value(func() string {
						if p.EntityID != nil {
							return p.EntityID.String()
						}
						return "null"
					}())),
					h.Input(h.Type("hidden"), h.Name("type"), h.Value(p.PatternType)),
					h.Input(h.Type("hidden"), h.Name("old_name"), h.Value(p.PatternName)),
					h.Div(
						h.Class("flex items-center gap-2"),
						h.Input(
							h.Type("text"),
							h.ID("new-pattern-name"),
							h.Name("new_name"),
							h.Value(displayTitle),
							h.Class("text-2xl font-semibold bg-muted text-foreground border border-border rounded-lg px-3 py-1 focus:outline-none focus:ring-2 focus:ring-indigo-600 focus:border-transparent"),
							h.Required(),
						),
						h.Button(
							h.Type("submit"),
							h.Class("text-chart-2 hover:text-chart-2/80 transition-colors p-1"),
							h.Title("Save"),
							g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`),
						),
						h.Button(
							h.Type("button"),
							h.Class("text-muted-foreground hover:text-foreground transition-colors p-1"),
							h.Title("Cancel"),
							g.Attr("onclick", "document.getElementById('pattern-name-edit').classList.add('hidden'); document.getElementById('pattern-name-display').classList.remove('hidden');"),
							g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>`),
						),
					),
				),
				// Show entity name below if pattern name is different (clickable link to entity)
				g.If(p.PatternName != "" && p.EntityName != "" && p.PatternName != p.EntityName && p.EntityID != nil,
					h.A(
						h.Href("/entities/"+p.EntityID.String()),
						h.Class("text-muted-foreground hover:text-primary transition-colors"),
						g.Text(p.EntityName),
					),
				),
				// Entity name when there's no pattern name but we have an entity (make it clickable)
				g.If((p.PatternName == "" || p.PatternName == p.EntityName) && p.EntityID != nil,
					h.A(
						h.Href("/entities/"+p.EntityID.String()),
						h.Class("text-muted-foreground hover:text-primary transition-colors text-sm"),
						g.Text("View entity →"),
					),
				),
				h.P(h.Class("text-muted-foreground mt-1"), g.Text(patternTypeDisplay+" · "+formatFrequencyDisplay(p.Frequency))),
			),
			// Pricing
			h.Div(
				h.Class("text-right"),
				h.P(h.Class("text-3xl font-semibold font-number text-foreground"), g.Text(formatMoney(p.AvgAmountCents))),
				h.P(h.Class("text-muted-foreground text-sm"), g.Text("/"+formatFrequencyShortDisplay(p.Frequency))),
				g.If(p.Frequency != "annual",
					h.P(h.Class("text-muted-foreground text-sm font-number mt-1"), g.Text(formatMoney(annualCost)+"/yr")),
				),
			),
		),

		// Stats row
		h.Div(
			h.Class("grid grid-cols-2 md:grid-cols-4 gap-4 mb-8"),
			shadcn.Stat(shadcn.StatProps{Label: "Confidence", Value: strconv.Itoa(p.AvgConfidence) + "%", Trend: "", Positive: p.AvgConfidence >= 70}),
			shadcn.Stat(shadcn.StatProps{Label: "Occurrences", Value: strconv.Itoa(p.OccurrenceCount), Trend: "", Positive: true}),
			g.If(p.NextExpected != nil,
				shadcn.Stat(shadcn.StatProps{Label: "Next Expected", Value: p.NextExpected.Format("Jan 2, 2006"), Trend: "", Positive: true}),
			),
			shadcn.Stat(shadcn.StatProps{Label: "Total Spent", Value: formatMoney(p.TotalAmountCents), Trend: "", Positive: true}),
		),

		// Why it's a pattern (reasoning section)
		g.If(detail.Reasoning != "",
			h.Div(
				h.Class("mb-8"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeader(
						h.Span(h.Class("text-primary"), g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>`)),
						g.Text("Why It's a Pattern"),
					),
					shadcn.CardContentFull(
						h.P(h.Class("text-card-foreground leading-relaxed"), g.Text(detail.Reasoning)),
					),
				),
			),
		),

		// Transactions list
		renderPatternTransactions(detail.Transactions),

		// Time span info
		h.Div(
			h.Class("mt-6 p-4 bg-card rounded-lg border border-border"),
			h.Div(
				h.Class("flex items-center justify-center gap-6 text-sm"),
				h.Div(
					h.Class("text-center"),
					h.P(h.Class("text-muted-foreground"), g.Text("First Occurrence")),
					h.P(h.Class("text-foreground"), g.Text(p.FirstOccurrence.Format("Jan 2, 2006"))),
				),
				h.Span(h.Class("text-muted-foreground"), g.Text("·")),
				h.Div(
					h.Class("text-center"),
					h.P(h.Class("text-muted-foreground"), g.Text("Last Occurrence")),
					h.P(h.Class("text-foreground"), g.Text(p.LastOccurrence.Format("Jan 2, 2006"))),
				),
				h.Span(h.Class("text-muted-foreground"), g.Text("·")),
				h.Div(
					h.Class("text-center"),
					h.P(h.Class("text-muted-foreground"), g.Text("Pattern Type")),
					h.P(h.Class("text-foreground"), g.Text(patternTypeDisplay)),
				),
			),
		),

		// Dismiss pattern section
		h.Div(
			h.Class("mt-8 pt-6 border-t border-border"),
			h.Div(
				h.Class("flex items-center justify-between"),
				h.Div(
					h.P(h.Class("text-muted-foreground text-sm"), g.Text("Think this isn't a real pattern?")),
					h.P(h.Class("text-muted-foreground text-xs mt-1"), g.Text("Dismissed patterns won't reappear when you refresh detection.")),
				),
				h.Form(
					h.Action("/patterns/dismiss"),
					h.Method("POST"),
					h.Input(h.Type("hidden"), h.Name("entity"), h.Value(func() string {
						if p.EntityID != nil {
							return p.EntityID.String()
						}
						return "null"
					}())),
					h.Input(h.Type("hidden"), h.Name("type"), h.Value(p.PatternType)),
					h.Input(h.Type("hidden"), h.Name("name"), h.Value(p.PatternName)),
					h.Button(
						h.Type("submit"),
						h.Class("inline-flex items-center gap-2 px-4 py-2 bg-muted hover:bg-destructive/50 text-muted-foreground hover:text-destructive rounded-lg text-sm transition-colors border border-border hover:border-destructive"),
						g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>`),
						g.Text("Not a Pattern"),
					),
				),
			),
		),
	)
}

// renderPatternTransactions renders the transactions list for a pattern using standard format
func renderPatternTransactions(transactions []*models.PatternTransaction) g.Node {
	if len(transactions) == 0 {
		return nil
	}

	transactionItems := make([]g.Node, 0, len(transactions))
	for _, txn := range transactions {
		txnURL := "/transactions/" + txn.ID.String()
		transactionItems = append(transactionItems, renderPatternTransactionItem(txn, txnURL))
	}

	return h.Div(
		h.Class("mb-8"),
		shadcn.Card(shadcn.CardProps{},
			shadcn.CardHeaderActions("Transactions", fmt.Sprintf("%d total", len(transactions))),
			shadcn.CardContent(
				h.Div(
					h.Class("divide-y divide-border"),
					g.Group(transactionItems),
				),
			),
		),
	)
}

// renderPatternTransactionItem renders a single pattern transaction in standard format
func renderPatternTransactionItem(txn *models.PatternTransaction, txnURL string) g.Node {
	return h.Div(
		h.Class("flex items-center p-3 sm:p-4 hover:bg-accent transition-colors gap-3 group"),
		g.Attr("onclick", "window.location='"+txnURL+"'"),
		g.Attr("style", "cursor: pointer;"),
		// Date
		h.Div(
			h.Class("w-24 text-sm text-muted-foreground flex-shrink-0"),
			g.Text(txn.Date.Format("Jan 2, 2006")),
		),
		// Description and account info
		h.Div(
			h.Class("flex-1 min-w-0 pr-3"),
			h.A(
				h.Href(txnURL),
				h.Class("text-sm font-medium text-card-foreground hover:text-primary truncate block"),
				g.Text(txn.Description),
			),
			h.Div(
				h.Class("text-xs text-muted-foreground mt-0.5 truncate"),
				g.Text(txn.AccountName),
			),
		),
		// Amount - right aligned
		h.Div(
			h.Class("flex-none text-right"),
			h.Span(
				h.Class("font-number font-medium text-card-foreground"),
				g.Text(formatMoney(txn.AmountCents)),
			),
		),
	)
}

// formatPatternTypeDisplay formats the pattern type for display
func formatPatternTypeDisplay(patternType string) string {
	switch patternType {
	case "recurring_bill":
		return "Recurring Bill"
	case "salary":
		return "Salary"
	case "account_transfer":
		return "Account Transfer"
	case "investment_contribution":
		return "Investment"
	case "household_transfer":
		return "Household Transfer"
	default:
		return patternType
	}
}

// calculatePatternAnnualCost converts any frequency amount to annual cost
func calculatePatternAnnualCost(amountCents int64, freq string) int64 {
	switch freq {
	case "weekly":
		return amountCents * 52
	case "biweekly":
		return amountCents * 26
	case "monthly":
		return amountCents * 12
	case "quarterly":
		return amountCents * 4
	case "annual":
		return amountCents
	default:
		return amountCents * 12 // Assume monthly
	}
}

// toMonthlyCents converts a recurring amount (in cents) at the given frequency to its
// monthly equivalent, using math.Round for fractional multipliers.
func toMonthlyCents(amountCents int64, frequency string) int64 {
	switch frequency {
	case "weekly":
		return int64(math.Round(float64(amountCents) * 4.333))
	case "biweekly":
		return int64(math.Round(float64(amountCents) * 2.167))
	case "monthly":
		return amountCents
	case "quarterly":
		return amountCents / 3
	case "annual":
		return amountCents / 12
	default:
		return amountCents // Assume monthly
	}
}
