package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/enrichment"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// EntitiesList shows all entities
func (hdl *Handlers) EntitiesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get search query
	search := r.URL.Query().Get("search")

	// Get filter type
	entityType := r.URL.Query().Get("type")
	var filterType *models.EntityType
	if entityType != "" {
		t := models.EntityType(entityType)
		filterType = &t
	}

	// Get entities
	entities, total, err := hdl.entities.List(ctx, filterType, nil, search, 100, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build select options
	selectOptions := []shadcn.SelectOption{
		{Value: "", Label: "All types"},
		{Value: "person", Label: "People"},
		{Value: "business", Label: "Businesses"},
		{Value: "trust", Label: "Trusts"},
		{Value: "partnership", Label: "Partnerships"},
		{Value: "government", Label: "Government"},
	}

	pageNode := layouts.AppLayout("Entities", user.Email, user.ID.String(),
		shadcn.PageHeader("Entities", fmt.Sprintf("%d entities", total)),
		// Search form
		h.Form(
			h.Method("GET"),
			h.Action("/entities"),
			h.Class("mb-6"),
			h.Div(
				h.Class("flex gap-4"),
				h.Div(
					h.Class("flex-1"),
					shadcn.SearchInput("search", "Search entities...", search, layouts.IconSearch()),
				),
				shadcn.NativeSelect(shadcn.NativeSelectProps{
					Name: "type",
				}, selectOptions,
					g.Attr("onchange", "this.form.submit()"),
				),
				shadcn.Button(shadcn.ButtonProps{
					Variant: shadcn.ButtonDefault,
					Type:    "submit",
				},
					g.Text("Search"),
				),
			),
		),
		// Entities list
		g.If(len(entities) == 0,
			shadcn.EmptyNoData("No entities found", "Try adjusting your search or filters.", nil),
		),
		g.If(len(entities) > 0,
			h.Div(
				h.Class("grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"),
				g.Group(g.Map(entities, func(e *models.Entity) g.Node {
					subtype := e.Subtype
					if subtype == "" {
						subtype = string(e.Type)
					}
					return shadcn.EntityCard(shadcn.EntityCardProps{
						ID:         e.ID.String(),
						Name:       e.Name,
						Subtype:    subtype,
						LogoURL:    e.LogoURL,
						GetLogoURL: hdl.getLogoURL,
					})
				})),
			),
		),
	)

	if err := pageNode.Render(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// EntitiesShow shows a single entity
func (hdl *Handlers) EntitiesShow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid entity ID", http.StatusBadRequest)
		return
	}

	entity, err := hdl.entities.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	// Get ledger for transaction filter
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get transactions for this entity
	filter := models.TransactionFilter{
		LedgerID: ledger.ID,
		EntityID: &id,
		Limit:    50,
	}

	transactions, _, err := hdl.transactions.List(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Load entries and tags for transactions
	for _, txn := range transactions {
		if err := hdl.transactions.LoadEntries(ctx, txn); err != nil {
			slog.WarnContext(ctx, "failed to load entries", "txn_id", txn.ID, "err", err)
		}
		if err := hdl.transactions.LoadTags(ctx, txn); err != nil {
			slog.WarnContext(ctx, "failed to load tags", "txn_id", txn.ID, "err", err)
		}
	}

	// Check if Firecrawl is configured for the help identify feature
	firecrawlConfigured := hdl.firecrawl != nil && hdl.firecrawl.IsConfigured()

	// Calculate total amount for this entity
	var totalCents int64
	for _, txn := range transactions {
		for _, e := range txn.Entries {
			if e.AmountCents != 0 && (e.AccountType == models.AccountTypeAsset || e.AccountType == models.AccountTypeLiability) {
				totalCents += e.AmountCents
				break
			}
		}
	}

	pageNode := layouts.AppLayout(entity.Name, user.Email, user.ID.String(),
		// Header with entity info - no card wrapper
		h.Div(
			h.Class("mb-8 flex items-start gap-6"),
			// Logo
			h.Div(
				h.ID("entity-logo"),
				h.Class("flex-none"),
				g.If(entity.LogoURL != "",
					h.Img(
						h.Src(hdl.getLogoURL(entity.LogoURL)),
						h.Alt(entity.Name),
						h.Class("w-20 h-20 rounded-lg object-contain"),
					),
				),
				g.If(entity.LogoURL == "",
					h.Div(
						h.Class("w-20 h-20 rounded-xl bg-gradient-to-br from-accent to-accent/80 flex items-center justify-center text-3xl font-bold text-accent-foreground shadow-sm"),
						g.Text(getEntityInitials(entity.Name)),
					),
				),
			),
			// Info
			h.Div(
				h.Class("flex-1 min-w-0"),
				h.Div(
					h.Class("flex items-center gap-2 flex-wrap"),
					// Editable entity name (auto-sizing input with shadcn styling)
					shadcn.Input(shadcn.InputProps{
						Type:  "text",
						Name:  "name",
						ID:    "entity-name",
						Value: entity.Name,
						Class: "text-3xl font-bold border-none shadow-none bg-transparent px-0 h-auto py-0 focus-visible:ring-0",
					},
						g.Attr("hx-post", fmt.Sprintf("/entities/%s/update", entity.ID)),
						g.Attr("hx-trigger", "input changed delay:500ms"),
						g.Attr("hx-swap", "none"),
						g.Attr("size", fmt.Sprintf("%d", max(len(entity.Name), 10))),
						g.Attr("oninput", "this.size = Math.max(this.value.length, 10)"),
					),
					// Verified badge using shadcn Badge
					g.If(entity.UserVerified,
						shadcn.Badge(shadcn.BadgeProps{Variant: shadcn.BadgeSuccess, Class: "gap-1"},
							g.Raw(`<svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"></path></svg>`),
							g.Text("Verified"),
						),
					),
					// Action buttons using shadcn Button
					g.If(firecrawlConfigured,
						shadcn.Button(shadcn.ButtonProps{
							Variant: shadcn.ButtonGhost,
							Size:    shadcn.ButtonSizeIcon,
							Class:   "h-8 w-8",
						},
							g.Attr("onclick", "document.getElementById('help-identify-modal').classList.remove('hidden')"),
							g.Attr("title", "Help identify this entity"),
							layouts.IconSearch(),
						),
					),
					shadcn.ButtonAnchor(shadcn.ButtonProps{
						Variant: shadcn.ButtonGhost,
						Size:    shadcn.ButtonSizeIcon,
					}, fmt.Sprintf("/entities/%s/merge", entity.ID),
						g.Attr("title", "Merge with another entity"),
						layouts.IconMerge(),
					),
					g.If(entity.Website != "" || entity.LogoURL != "",
						shadcn.Button(shadcn.ButtonProps{
							Variant: shadcn.ButtonGhost,
							Size:    shadcn.ButtonSizeIcon,
							Class:   "h-8 w-8 text-muted-foreground hover:text-destructive",
						},
							g.Attr("title", "Clear business info (logo, website) - for personal transfers"),
							g.Attr("hx-post", fmt.Sprintf("/entities/%s/clear-enrichment", entity.ID)),
							g.Attr("hx-confirm", "Clear logo and website? This is useful for personal transfers that shouldn't be linked to a business."),
							layouts.IconTrash(),
						),
					),
				),
				// Editable description using shadcn Textarea
				shadcn.Textarea(shadcn.TextareaProps{
					Name:        "description",
					ID:          "entity-description",
					Placeholder: "Add a description...",
					Value:       entity.Description,
					Class:       "text-muted-foreground text-sm mt-2 border-none shadow-none bg-transparent px-0 resize-none overflow-hidden min-h-0",
					Rows:        1,
				},
					g.Attr("hx-post", fmt.Sprintf("/entities/%s/update", entity.ID)),
					g.Attr("hx-trigger", "input changed delay:500ms"),
					g.Attr("hx-swap", "none"),
					g.Attr("oninput", "this.style.height = 'auto'; this.style.height = this.scrollHeight + 'px'"),
				),
				// Auto-size the textarea on page load
				g.Raw(`<script>document.getElementById('entity-description').style.height = document.getElementById('entity-description').scrollHeight + 'px';</script>`),
				g.If(entity.Website != "",
					h.A(
						h.Href(func() string {
							if !strings.HasPrefix(entity.Website, "http") {
								return "https://" + entity.Website
							}
							return entity.Website
						}()),
						h.Target("_blank"),
						h.Class("text-primary hover:opacity-80 text-sm inline-block mt-2"),
						h.ID("entity-website"),
						g.Text(entity.Website),
					),
				),
			),
			// Total Amount (right side)
			h.Div(
				h.Class("text-right flex-none"),
				h.P(
					h.Class("text-3xl font-bold font-mono "+func() string {
						if totalCents < 0 {
							return "text-destructive"
						} else if totalCents > 0 {
							return "text-chart-2"
						}
						return "text-foreground"
					}()),
					g.Text(models.FormatCents(totalCents)),
				),
			),
		),
		// Help Identify Modal (hidden by default)
		g.If(firecrawlConfigured,
			helpIdentifyModal(entity),
		),
		g.If(len(transactions) == 0,
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					shadcn.EmptyNoData("No transactions found", "This entity has no transactions yet.", nil),
				),
			),
		),
		g.If(len(transactions) > 0,
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					shadcn.Table(shadcn.TableProps{},
						shadcn.TableHeader(
							shadcn.TableRow(
								shadcn.TableHead(g.Text("Date")),
								shadcn.TableHead(g.Text("Description")),
								shadcn.TableHead(g.Text("Category")),
								shadcn.TableHead(g.Text("Amount")),
							),
						),
						shadcn.TableBody(
							g.Group(g.Map(transactions, func(txn *models.Transaction) g.Node {
								return renderTransactionRowCompact(txn)
							})),
						),
					),
				),
			),
		),
	)

	if err := pageNode.Render(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// EntitiesUpdate updates an entity
func (hdl *Handlers) EntitiesUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid entity ID", http.StatusBadRequest)
		return
	}

	entity, err := hdl.entities.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	// Update fields - support both name and description from inline editing
	if name := r.FormValue("name"); name != "" {
		entity.Name = name
	}
	if description := r.FormValue("description"); description != "" {
		entity.Description = description
	}
	if website := r.FormValue("website"); website != "" {
		entity.Website = website
	}
	entity.UserVerified = true // Mark as verified when user edits

	if err := hdl.entities.Update(ctx, entity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response for HTMX
	htmxRedirect(w, r, fmt.Sprintf("/entities/%s", entity.ID))
}

// EntitiesEnrichSearch searches for companies using Firecrawl and returns JSON results
func (hdl *Handlers) EntitiesEnrichSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.CurrentUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	_, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid entity ID", http.StatusBadRequest)
		return
	}

	// Check if Firecrawl is configured
	if hdl.firecrawl == nil || !hdl.firecrawl.IsConfigured() {
		respondJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "Firecrawl is not configured", "companies": []enrichment.CompanyInfo{},
		})
		return
	}

	query := r.URL.Query().Get("q")
	hint := r.URL.Query().Get("hint")

	if query == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Query parameter 'q' is required", "companies": []enrichment.CompanyInfo{},
		})
		return
	}

	// Search with hint (using the query directly, entity name not needed for search)
	companies, err := hdl.firecrawl.SearchWithHint(ctx, query, hint, "", "")
	if err != nil {
		slog.ErrorContext(ctx, "firecrawl search error", "query", query, "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "entities", Operation: "firecrawl_search",
			Tags: map[string]string{"query": query},
		})

		errorMsg := "Search service temporarily unavailable"
		if errStr := err.Error(); strings.Contains(errStr, "status") {
			if strings.Contains(errStr, "401") || strings.Contains(errStr, "403") {
				errorMsg = "Search service authentication failed"
			} else if strings.Contains(errStr, "429") {
				errorMsg = "Search service rate limit exceeded. Please try again later"
			} else if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || strings.Contains(errStr, "503") {
				errorMsg = "Search service is temporarily unavailable. Please try again later"
			}
		}
		respondJSON(w, http.StatusInternalServerError, map[string]any{
			"error": errorMsg, "companies": []enrichment.CompanyInfo{},
		})
		return
	}

	if companies == nil {
		companies = []enrichment.CompanyInfo{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"companies": companies})
}

// EntitiesEnrich enriches an entity using Firecrawl with selected company data
func (hdl *Handlers) EntitiesEnrich(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid entity ID", http.StatusBadRequest)
		return
	}

	entity, err := hdl.entities.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	var info *enrichment.CompanyInfo

	// Check if company_data was provided (from modal selection)
	if companyDataJSON := r.FormValue("company_data"); companyDataJSON != "" {
		// Parse the selected company data
		var companyData struct {
			Name        string `json:"name"`
			Website     string `json:"website"`
			LogoURL     string `json:"logo_url"`
			Description string `json:"description"`
			SourceURL   string `json:"source_url"`
		}
		if err := json.Unmarshal([]byte(companyDataJSON), &companyData); err == nil {
			// If we have a source URL, re-extract to ensure we get the proper logo (apple-touch-icon, og:image, etc.)
			if companyData.SourceURL != "" && hdl.firecrawl != nil && hdl.firecrawl.IsConfigured() {
				// Re-extract to get the properly extracted logo
				extracted, err := hdl.firecrawl.ExtractCompanyInfo(ctx, companyData.SourceURL)
				if err == nil && extracted != nil {
					// Use extracted info, but preserve user-selected name if it's better
					info = extracted
					if companyData.Name != "" && len(companyData.Name) > len(extracted.Name) {
						info.Name = companyData.Name
					}
					// Ensure we have the source URL
					info.SourceURL = companyData.SourceURL
				} else {
					// Fallback to provided data
					info = &enrichment.CompanyInfo{
						Name:        companyData.Name,
						Website:     companyData.Website,
						LogoURL:     companyData.LogoURL,
						Description: companyData.Description,
						SourceURL:   companyData.SourceURL,
					}
				}
			} else {
				// Use provided data directly
				info = &enrichment.CompanyInfo{
					Name:        companyData.Name,
					Website:     companyData.Website,
					LogoURL:     companyData.LogoURL,
					Description: companyData.Description,
					SourceURL:   companyData.SourceURL,
				}
			}
		}
	}

	// Fallback: if no company_data, do automatic search
	if info == nil {
		// Check if Firecrawl is configured
		if hdl.firecrawl == nil || !hdl.firecrawl.IsConfigured() {
			http.Error(w, "Firecrawl is not configured", http.StatusServiceUnavailable)
			return
		}

		// Search for company info using Firecrawl
		info, err = hdl.firecrawl.SearchAndExtract(ctx, entity.Name, "", "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to enrich entity: %v", err), http.StatusInternalServerError)
			return
		}

		if info == nil {
			// Return success but no changes
			htmxRedirect(w, r, fmt.Sprintf("/entities/%s?enriched=no_results", entity.ID))
			return
		}
	}

	// Update entity with enriched data (only if not user-verified or if fields are empty)
	updated := false
	if !entity.UserVerified || entity.LogoURL == "" {
		if info.LogoURL != "" {
			// Download and store the logo
			logoStore := hdl.logoClient.GetLogoStore()
			if logoStore != nil {
				if localURL, err := logoStore.DownloadAndStore(ctx, info.LogoURL); err == nil {
					entity.LogoURL = localURL
					updated = true
				}
			} else if strings.HasPrefix(info.LogoURL, "http") {
				// If logo store not available, store the URL directly (will be fetched later)
				entity.LogoURL = info.LogoURL
				updated = true
			}
		}
	}

	if !entity.UserVerified || entity.Website == "" {
		if info.Website != "" {
			entity.Website = info.Website
			updated = true
		}
	}

	if !entity.UserVerified || entity.Description == "" {
		if info.Description != "" {
			entity.Description = info.Description
			updated = true
		}
	}

	// Update name if it's better (longer/more descriptive)
	if !entity.UserVerified && info.Name != "" && len(info.Name) > len(entity.Name) {
		entity.Name = info.Name
		updated = true
	}

	if updated {
		if err := hdl.entities.Update(ctx, entity); err != nil {
			http.Error(w, fmt.Sprintf("Failed to update entity: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Return success response for HTMX
	htmxRedirect(w, r, fmt.Sprintf("/entities/%s?enriched=success", entity.ID))
}

// EntitiesSearch performs a search for entities
func (hdl *Handlers) EntitiesSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.CurrentUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	limit := 20
	entities, err := hdl.entities.SearchByBM25(ctx, query, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type entityResult struct {
		ID      uuid.UUID `json:"id"`
		Name    string    `json:"name"`
		Type    string    `json:"type"`
		Subtype string    `json:"subtype"`
		LogoURL string    `json:"logo_url"`
	}
	results := make([]entityResult, len(entities))
	for i, e := range entities {
		results[i] = entityResult{ID: e.ID, Name: e.Name, Type: string(e.Type), Subtype: e.Subtype, LogoURL: e.LogoURL}
	}
	respondJSON(w, http.StatusOK, map[string]any{"entities": results})
}

// EntitiesMergeConfirm performs the merge operation
func (hdl *Handlers) EntitiesMergeConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	targetID, ok := mustParamUUID(w, r, "id", "entity ID")
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	sourceID, ok := mustFormParamUUID(w, r, "from_id", "source entity ID")
	if !ok {
		return
	}

	// Perform the merge
	if err := hdl.entities.Merge(ctx, sourceID, targetID); err != nil {
		http.Error(w, fmt.Sprintf("Merge failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Redirect back to the target entity page
	http.Redirect(w, r, fmt.Sprintf("/entities/%s", targetID), http.StatusSeeOther)
}

// EntitiesDelete deletes an entity
func (hdl *Handlers) EntitiesDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid entity ID", http.StatusBadRequest)
		return
	}

	// Check if entity exists
	entity, err := hdl.entities.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	// Check if entity has any transactions
	filter := models.TransactionFilter{
		EntityID: &id,
		Limit:    1,
	}
	transactions, _, err := hdl.transactions.List(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(transactions) > 0 {
		http.Error(w, "Cannot delete entity: it has associated transactions. Please merge it with another entity first.", http.StatusBadRequest)
		return
	}

	// Delete the entity
	if err := hdl.entities.Delete(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response for HTMX
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/entities")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/entities?deleted="+entity.Name, http.StatusSeeOther)
}

// getEntityInitials returns the initials for an entity name
func getEntityInitials(name string) string {
	if name == "" {
		return "?"
	}
	return string(name[0])
}


// renderTransactionRowCompact renders a compact transaction row for the entity page using shadcn components
func renderTransactionRowCompact(txn *models.Transaction) g.Node {
	// Calculate amount from entries
	var amount int64
	var accountType models.AccountType
	for _, e := range txn.Entries {
		if e.AmountCents != 0 && (e.AccountType == models.AccountTypeAsset || e.AccountType == models.AccountTypeLiability) {
			amount = e.AmountCents
			accountType = e.AccountType
			break
		}
	}

	displayTitle := txn.DisplayTitle
	if displayTitle == "" {
		displayTitle = txn.Description
	}

	// Get first tag
	var firstTag *models.Tag
	if len(txn.Tags) > 0 {
		firstTag = txn.Tags[0]
	}

	return shadcn.TableRow(
		shadcn.TableCell(
			h.Class("text-sm text-muted-foreground whitespace-nowrap"),
			g.Text(txn.Date.Format("Jan 2, 2006")),
		),
		shadcn.TableCell(
			h.A(
				h.Href(fmt.Sprintf("/transactions/%s", txn.ID)),
				h.Class("text-foreground hover:text-primary"),
				g.Text(displayTitle),
			),
		),
		shadcn.TableCell(
			g.If(firstTag != nil,
				shadcn.Badge(shadcn.BadgeProps{
					Variant: shadcn.BadgeOutline,
					Class:   "whitespace-nowrap",
				},
					h.Style(fmt.Sprintf("border-color: %s40; color: %s; background-color: %s15", firstTag.Color, firstTag.Color, firstTag.Color)),
					g.Text(firstTag.Name),
				),
			),
		),
		shadcn.TableCellNumeric(
			h.Span(
				h.Class(transactionAmountColorClass(amount, accountType)),
				g.Text(displayBalanceWithSign(amount, accountType)),
			),
		),
	)
}

// helpIdentifyModal renders the modal for helping identify an entity
func helpIdentifyModal(entity *models.Entity) g.Node {
	return h.Div(
		h.ID("help-identify-modal"),
		h.Class("hidden fixed inset-0 z-50 overflow-y-auto"),
		// Backdrop
		h.Div(
			h.Class("fixed inset-0 bg-black/60 backdrop-blur-sm"),
			g.Attr("onclick", "document.getElementById('help-identify-modal').classList.add('hidden')"),
		),
		// Modal content
		h.Div(
			h.Class("relative min-h-screen flex items-center justify-center p-4"),
			h.Div(
				h.Class("relative bg-card rounded-xl border border-border shadow-xl max-w-4xl w-full"),
				// Header
				h.Div(
					h.Class("flex items-center justify-between p-4 border-b border-border"),
					h.H3(
						h.Class("text-lg font-semibold text-card-foreground"),
						g.Text("Help Identify This Entity"),
					),
					h.Button(
						h.Type("button"),
						h.Class("text-muted-foreground hover:text-foreground"),
						g.Attr("onclick", "document.getElementById('help-identify-modal').classList.add('hidden')"),
						g.Raw(`<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>`),
					),
				),
				// Body
				h.Div(
					h.Class("p-4"),
					h.P(
						h.Class("text-muted-foreground text-sm mb-4"),
						g.Text("Search for company information, logo, and details using Firecrawl."),
					),
					// Search input
					h.Div(
						h.Class("relative mb-4"),
						shadcn.SearchInput("", "Search for company...", "", layouts.IconSearch(),
							h.ID("help-identify-search"),
							g.Attr("autocomplete", "off"),
						),
						h.Div(
							h.ID("help-identify-loading"),
							h.Class("hidden absolute right-3 top-1/2 -translate-y-1/2"),
							layouts.IconLoader(),
						),
					),
					// Results area
					h.Div(
						h.ID("help-identify-results"),
						h.Class("min-h-48 max-h-96 overflow-y-auto"),
					),
				),
			),
		),
		g.Raw(fmt.Sprintf(`<script>
(function() {
	const searchInput = document.getElementById('help-identify-search');
	const resultsDiv = document.getElementById('help-identify-results');
	const loadingDiv = document.getElementById('help-identify-loading');
	const entityId = '%s';
	
	let searchTimeout;
	
	function performSearch() {
		const query = searchInput.value.trim();
		
		if (query.length < 2) {
			resultsDiv.innerHTML = '';
			return;
		}
		
		loadingDiv.classList.remove('hidden');
		resultsDiv.innerHTML = '';
		
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			const url = '/entities/' + entityId + '/enrich/search?q=' + encodeURIComponent(query);
			
			fetch(url)
				.then(r => r.json())
				.then(data => {
					loadingDiv.classList.add('hidden');
					displayResults(data.companies || []);
				})
				.catch(err => {
					loadingDiv.classList.add('hidden');
					resultsDiv.innerHTML = '<div class="text-center py-8 text-red-400 text-sm">Error: ' + escapeHtml(err.message || 'Unknown error') + '</div>';
				});
		}, 500);
	}
	
	function displayResults(companies) {
		if (companies.length === 0) {
			resultsDiv.innerHTML = '<div class="text-center py-8 text-muted-foreground text-sm">No results found</div>';
			return;
		}
		
		resultsDiv.innerHTML = companies.map((company, idx) => {
			const logoHtml = company.logo_url ? 
				'<img src="' + escapeHtml(company.logo_url) + '" alt="" class="w-10 h-10 rounded object-contain bg-muted border border-border flex-shrink-0" onerror="this.style.display=\'none\'; this.nextElementSibling.style.display=\'flex\'" />' +
				'<div class="w-10 h-10 rounded bg-gradient-to-br from-accent to-accent/80 flex items-center justify-center text-sm font-semibold text-accent-foreground flex-shrink-0 shadow-sm" style="display:none">' + escapeHtml((company.name || '?')[0]) + '</div>' :
				'<div class="w-10 h-10 rounded bg-gradient-to-br from-accent to-accent/80 flex items-center justify-center text-sm font-semibold text-accent-foreground flex-shrink-0 shadow-sm">' + escapeHtml((company.name || '?')[0]) + '</div>';
			
			const name = escapeHtml(company.name || 'Unknown');
			const website = company.website ? '<div class="text-xs text-muted-foreground mt-0.5 truncate">' + escapeHtml(company.website) + '</div>' : '';
			const desc = company.description ? '<div class="text-xs text-muted-foreground mt-1 line-clamp-1">' + escapeHtml(company.description) + '</div>' : '';
			
			return '<button type="button" onclick="selectEnrichmentResult(' + idx + ')" class="w-full flex items-center gap-3 px-3 py-2.5 rounded border border-border bg-muted hover:bg-muted/75 hover:border-ring transition-colors text-left group" data-company="' + escapeHtml(JSON.stringify(company)) + '">' +
				logoHtml +
				'<div class="flex-1 min-w-0">' +
					'<div class="text-sm font-medium text-white truncate">' + name + '</div>' +
					website +
					desc +
				'</div>' +
				'<svg class="w-4 h-4 text-muted-foreground group-hover:text-primary transition-colors flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>' +
			'</button>';
		}).join('');
		
		window.enrichmentResults = companies;
	}
	
	window.selectEnrichmentResult = function(idx) {
		const company = window.enrichmentResults[idx];
		if (!company) return;
		
		document.getElementById('help-identify-modal').classList.add('hidden');
		
		const form = document.createElement('form');
		form.method = 'POST';
		form.action = '/entities/' + entityId + '/enrich';
		
		const data = document.createElement('input');
		data.type = 'hidden';
		data.name = 'company_data';
		data.value = JSON.stringify(company);
		form.appendChild(data);
		
		document.body.appendChild(form);
		form.submit();
	};
	
	function escapeHtml(text) {
		const div = document.createElement('div');
		div.textContent = text;
		return div.innerHTML;
	}
	
	searchInput.addEventListener('input', performSearch);
})();
</script>`, entity.ID.String())),
	)
}

// EntitiesClearEnrichment removes the logo, website, and description from an entity
func (hdl *Handlers) EntitiesClearEnrichment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid entity ID", http.StatusBadRequest)
		return
	}

	entity, err := hdl.entities.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	// Clear enrichment data
	entity.LogoURL = ""
	entity.Website = ""
	entity.Description = ""
	entity.UserVerified = false

	if err := hdl.entities.Update(ctx, entity); err != nil {
		http.Error(w, fmt.Sprintf("Failed to clear enrichment: %v", err), http.StatusInternalServerError)
		return
	}

	// For HTMX requests, return updated UI
	htmxRedirect(w, r, fmt.Sprintf("/entities/%s", id))
}

// EntitiesMerge shows the merge page with potential duplicate entities
func (hdl *Handlers) EntitiesMerge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entityID, ok := mustParamUUID(w, r, "id", "entity ID")
	if !ok {
		return
	}

	// Get the entity we're merging from
	entity, err := hdl.entities.GetByID(ctx, entityID)
	if err != nil {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	// Check if user is searching
	search := r.URL.Query().Get("search")

	type EntityCandidate struct {
		Entity *models.Entity
		Count  int
	}
	var candidates []EntityCandidate

	if search != "" {
		// User searched - show search results
		searchResults, _, err := hdl.entities.List(ctx, nil, nil, search, 20, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, e := range searchResults {
			// Exclude the target entity from results
			if e.ID != entityID {
				// Count transactions for this entity
				filter := models.TransactionFilter{
					LedgerID: ledger.ID,
					EntityID: &e.ID,
					Limit:    1,
				}
				_, count, _ := hdl.transactions.List(ctx, filter)
				candidates = append(candidates, EntityCandidate{Entity: e, Count: count})
			}
		}
	} else {
		// No search - show similar entities (same name, different case)
		allEntities, _, err := hdl.entities.List(ctx, nil, nil, "", 100, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entityNameLower := strings.ToLower(entity.Name)
		for _, e := range allEntities {
			if e.ID != entityID && strings.ToLower(e.Name) == entityNameLower {
				filter := models.TransactionFilter{
					LedgerID: ledger.ID,
					EntityID: &e.ID,
					Limit:    1,
				}
				_, count, _ := hdl.transactions.List(ctx, filter)
				candidates = append(candidates, EntityCandidate{Entity: e, Count: count})
			}
		}
	}

	pageNode := layouts.AppLayout("Merge "+entity.Name, user.Email, user.ID.String(),
		shadcn.PageHeader("Merge Entity", "Select an entity to merge into "+entity.Name),

		// Source entity info
		h.Div(
			h.Class("mb-8"),
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					h.H3(
						h.Class("text-sm font-medium text-muted-foreground mb-3"),
						g.Text("Keep this entity (target):"),
					),
					h.Div(
						h.Class("flex items-center gap-4"),
						// Logo
						g.If(entity.LogoURL != "",
							h.Img(
								h.Src(hdl.getLogoURL(entity.LogoURL)),
								h.Alt(entity.Name),
								h.Class("w-12 h-12 rounded-lg object-contain"),
							),
						),
						g.If(entity.LogoURL == "",
							h.Div(
								h.Class("w-12 h-12 rounded-lg bg-gradient-to-br from-accent to-accent/80 flex items-center justify-center text-lg font-bold text-accent-foreground shadow-sm"),
								g.Text(getEntityInitials(entity.Name)),
							),
						),
						// Info
						h.Div(
							h.Class("flex-1"),
							h.P(
								h.Class("font-semibold text-white"),
								g.Text(entity.Name),
							),
							g.If(entity.Website != "",
								h.P(
									h.Class("text-sm text-primary"),
									g.Text(entity.Website),
								),
							),
						),
					),
				),
			),
		),

		// Search box
		h.Form(
			h.Method("GET"),
			h.Action(fmt.Sprintf("/entities/%s/merge", entityID)),
			h.Class("mb-6"),
			h.Div(
				h.Class("flex gap-4"),
				h.Div(
					h.Class("flex-1"),
					h.Input(
						h.Type("text"),
						h.Name("search"),
						h.Placeholder("Search for an entity to merge..."),
						h.Value(search),
						h.Class("w-full rounded-lg border border-border bg-muted px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"),
					),
				),
				h.Button(
					h.Type("submit"),
					h.Class("rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:opacity-90 shadow-sm"),
					g.Text("Search"),
				),
				g.If(search != "",
					h.A(
						h.Href(fmt.Sprintf("/entities/%s/merge", entityID)),
						h.Class("rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90"),
						g.Text("Clear"),
					),
				),
			),
		),

		// Section header
		h.H3(
			h.Class("text-lg font-medium text-white mb-4"),
			g.If(search != "",
				g.Text(fmt.Sprintf("Search results for \"%s\":", search)),
			),
			g.If(search == "",
				g.Text("Similar entities:"),
			),
		),
		h.P(
			h.Class("text-sm text-muted-foreground mb-4"),
			g.Text("Transactions from the selected entity will be moved to the target entity above, and the selected entity will be deleted."),
		),

		g.If(len(candidates) == 0,
			h.Div(
				h.Class("text-center py-12 bg-card rounded-xl border border-border"),
				h.P(
					h.Class("text-muted-foreground"),
					g.If(search != "",
						g.Text("No entities found matching your search."),
					),
					g.If(search == "",
						g.Text("No similar entities found. Try searching for a specific entity above."),
					),
				),
			),
		),

		g.If(len(candidates) > 0,
			h.Div(
				h.Class("space-y-3"),
				g.Group(g.Map(candidates, func(c EntityCandidate) g.Node {
					return h.Div(
						h.Class("bg-card rounded-xl border border-border p-4 hover:border-border transition-colors"),
						h.Div(
							h.Class("flex items-center gap-4"),
							// Logo
							g.If(c.Entity.LogoURL != "",
								h.Img(
									h.Src(hdl.getLogoURL(c.Entity.LogoURL)),
									h.Alt(c.Entity.Name),
									h.Class("w-12 h-12 rounded-lg object-contain"),
								),
							),
							g.If(c.Entity.LogoURL == "",
								h.Div(
									h.Class("w-12 h-12 rounded-lg bg-gradient-to-br from-accent to-accent/80 flex items-center justify-center text-lg font-bold text-accent-foreground shadow-sm"),
									g.Text(getEntityInitials(c.Entity.Name)),
								),
							),
							// Info
							h.Div(
								h.Class("flex-1"),
								h.P(
									h.Class("font-medium text-white"),
									g.Text(c.Entity.Name),
								),
								h.P(
									h.Class("text-sm text-muted-foreground"),
									g.Text(fmt.Sprintf("%d transactions", c.Count)),
								),
								g.If(c.Entity.Website != "",
									h.P(
										h.Class("text-sm text-primary"),
										g.Text(c.Entity.Website),
									),
								),
							),
							// Merge button
							h.Form(
								h.Method("POST"),
								h.Action(fmt.Sprintf("/entities/%s/merge", entityID)),
								h.Input(h.Type("hidden"), h.Name("from_id"), h.Value(c.Entity.ID.String())),
								h.Button(
									h.Type("submit"),
									h.Class("px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm font-medium hover:opacity-90 transition-colors shadow-sm"),
									g.Attr("onclick", fmt.Sprintf("return confirm('Merge %s into %s? This will move %d transactions and delete %s.')",
										c.Entity.Name, entity.Name, c.Count, c.Entity.Name)),
									g.Text("Merge"),
								),
							),
						),
					)
				})),
			),
		),

		// Back link
		h.Div(
			h.Class("mt-6"),
			h.A(
				h.Href(fmt.Sprintf("/entities/%s", entityID)),
				h.Class("text-muted-foreground hover:text-foreground text-sm"),
				g.Text("← Back to entity"),
			),
		),
	)

	if err := pageNode.Render(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
