package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/billing"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// formatRelativeTime formats a time as a human-readable relative time string
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 48*time.Hour:
		return "yesterday"
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 2, 2006")
	}
}

// Settings redirects to profile section
func (hdl *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/settings/profile", http.StatusFound)
}

// SettingsProfile shows profile and password settings
func (hdl *Handlers) SettingsProfile(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)

	page := layouts.SettingsLayout("Profile Settings", user.Email, "profile", user.ID.String(),
		shadcn.PageHeader("Profile", "Manage your account settings"),

		h.Div(
			h.Class("space-y-6"),

			// Profile card
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					h.Form(
						h.Method("POST"),
						h.Action("/settings/profile"),
						h.Input(h.Type("hidden"), h.Name("_method"), h.Value("PUT")),
						h.Class("space-y-4"),

						shadcn.FormField(shadcn.FormFieldProps{Name: "email"},
							shadcn.Label(shadcn.LabelProps{For: "email"},
								g.Text("Email"),
							),
							shadcn.Input(shadcn.InputProps{
								Type:     "email",
								Name:     "email",
								Value:    user.Email,
								Disabled: true,
							}),
						),

						shadcn.Button(shadcn.ButtonProps{
							Variant:  shadcn.ButtonDefault,
							Type:     "submit",
							Disabled: true,
						},
							g.Text("Update Profile"),
						),
					),
				),
			),

			// Password card
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardHeaderActions("Change Password", ""),
				shadcn.CardContentFull(
					h.Form(
						h.Method("POST"),
						h.Action("/settings/password"),
						h.Input(h.Type("hidden"), h.Name("_method"), h.Value("PUT")),
						h.Class("space-y-4"),

						shadcn.FormField(shadcn.FormFieldProps{Name: "current_password"},
							shadcn.Label(shadcn.LabelProps{For: "current_password", Required: true},
								g.Text("Current Password"),
							),
							shadcn.Input(shadcn.InputProps{
								Type:     "password",
								Name:     "current_password",
								Required: true,
							}),
						),

						shadcn.FormField(shadcn.FormFieldProps{Name: "new_password"},
							shadcn.Label(shadcn.LabelProps{For: "new_password", Required: true},
								g.Text("New Password"),
							),
							shadcn.Input(shadcn.InputProps{
								Type:      "password",
								Name:      "new_password",
								Required:  true,
								MinLength: 8,
							}),
						),

						shadcn.FormField(shadcn.FormFieldProps{Name: "confirm_password"},
							shadcn.Label(shadcn.LabelProps{For: "confirm_password", Required: true},
								g.Text("Confirm New Password"),
							),
							shadcn.Input(shadcn.InputProps{
								Type:     "password",
								Name:     "confirm_password",
								Required: true,
							}),
						),

						shadcn.Button(shadcn.ButtonProps{
							Variant: shadcn.ButtonDefault,
							Type:    "submit",
						},
							g.Text("Change Password"),
						),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

// SettingsPreferences shows app-level preferences for this browser/device.
func (hdl *Handlers) SettingsPreferences(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)

	page := layouts.SettingsLayout("Preferences", user.Email, "preferences", user.ID.String(),
		shadcn.PageHeader("Preferences", "Customize how Probably looks on this device"),
		h.Div(
			h.Class("space-y-6"),
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardHeaderActions("Appearance", "Choose a theme mode"),
				shadcn.CardContentFull(
					h.Div(
						h.Class("space-y-4"),
						h.P(
							h.Class("text-sm text-muted-foreground"),
							g.Text("Theme preference is saved in this browser and applied immediately."),
						),
						h.Div(
							h.Class("space-y-3"),
							h.Label(
								h.For("theme-system"),
								h.Class("block cursor-pointer"),
								h.Input(
									h.Type("radio"),
									h.ID("theme-system"),
									h.Name("theme_preference"),
									h.Value("system"),
									h.Class("peer sr-only"),
								),
								h.Div(
									h.Class("flex items-start justify-between gap-4 rounded-lg border border-border bg-card/40 p-4 transition-colors peer-checked:border-primary peer-checked:bg-primary/5"),
									h.Div(
										h.Class("space-y-1"),
										h.P(h.Class("text-sm font-medium text-foreground"), g.Text("System")),
										h.P(h.Class("text-xs text-muted-foreground"), g.Text("Match your operating system light/dark setting.")),
									),
									h.Span(
										h.Class("text-xs text-primary opacity-0 transition-opacity peer-checked:opacity-100"),
										g.Text("Active"),
									),
								),
							),
							h.Label(
								h.For("theme-light"),
								h.Class("block cursor-pointer"),
								h.Input(
									h.Type("radio"),
									h.ID("theme-light"),
									h.Name("theme_preference"),
									h.Value("light"),
									h.Class("peer sr-only"),
								),
								h.Div(
									h.Class("flex items-start justify-between gap-4 rounded-lg border border-border bg-card/40 p-4 transition-colors peer-checked:border-primary peer-checked:bg-primary/5"),
									h.Div(
										h.Class("space-y-1"),
										h.P(h.Class("text-sm font-medium text-foreground"), g.Text("Light")),
										h.P(h.Class("text-xs text-muted-foreground"), g.Text("Always use the light theme.")),
									),
									h.Span(
										h.Class("text-xs text-primary opacity-0 transition-opacity peer-checked:opacity-100"),
										g.Text("Active"),
									),
								),
							),
							h.Label(
								h.For("theme-dark"),
								h.Class("block cursor-pointer"),
								h.Input(
									h.Type("radio"),
									h.ID("theme-dark"),
									h.Name("theme_preference"),
									h.Value("dark"),
									h.Class("peer sr-only"),
								),
								h.Div(
									h.Class("flex items-start justify-between gap-4 rounded-lg border border-border bg-card/40 p-4 transition-colors peer-checked:border-primary peer-checked:bg-primary/5"),
									h.Div(
										h.Class("space-y-1"),
										h.P(h.Class("text-sm font-medium text-foreground"), g.Text("Dark")),
										h.P(h.Class("text-xs text-muted-foreground"), g.Text("Always use the dark theme.")),
									),
									h.Span(
										h.Class("text-xs text-primary opacity-0 transition-opacity peer-checked:opacity-100"),
										g.Text("Active"),
									),
								),
							),
						),
						h.P(
							h.ID("theme-preference-status"),
							h.Class("text-xs text-muted-foreground"),
							g.Text("Current theme: System"),
						),
					),
				),
			),
		),
		h.Script(g.Raw(`
			(function() {
				const allowed = ['system', 'light', 'dark'];
				const statusEl = document.getElementById('theme-preference-status');
				const radios = document.querySelectorAll('input[name="theme_preference"]');

				const readPreference = () => {
					if (window.__probablyTheme && typeof window.__probablyTheme.getPreference === 'function') {
						return window.__probablyTheme.getPreference();
					}
					try {
						const raw = localStorage.getItem('probably-theme');
						if (raw && allowed.includes(raw)) return raw;
					} catch (_) {}
					return 'system';
				};

				const setStatus = (pref) => {
					const labels = { system: 'System', light: 'Light', dark: 'Dark' };
					if (statusEl) {
						statusEl.textContent = 'Current theme: ' + (labels[pref] || 'System');
					}
				};

				const setPreference = (pref) => {
					const safePref = allowed.includes(pref) ? pref : 'system';
					if (window.__probablyTheme && typeof window.__probablyTheme.setPreference === 'function') {
						window.__probablyTheme.setPreference(safePref);
					} else {
						try {
							localStorage.setItem('probably-theme', safePref);
						} catch (_) {}
					}
					setStatus(safePref);
				};

				const current = readPreference();
				radios.forEach((radio) => {
					radio.checked = radio.value === current;
					radio.addEventListener('change', () => {
						if (radio.checked) setPreference(radio.value);
					});
				});
				setStatus(current);
			})();
		`)),
	)

	renderHTML(w, page)
}

// SettingsMyLife shows the My Life page for managing relationships to people and things
func (hdl *Handlers) SettingsMyLife(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get relationships for this ledger
	relationshipStore := models.NewRelationshipStore(hdl.db.Pool)
	relationships, _ := relationshipStore.GetByLedgerID(r.Context(), ledger.ID)

	// Group relationships by category
	var people, work, assets []*models.Relationship
	for _, rel := range relationships {
		switch rel.Category {
		case "person":
			people = append(people, rel)
		case "work":
			work = append(work, rel)
		case "asset":
			assets = append(assets, rel)
		}
	}

	page := layouts.SettingsLayout("My Life", user.Email, "my-life", user.ID.String(),
		shadcn.PageHeader("My Life", "Define the people, work, and things in your life"),

		h.Div(
			h.Class("space-y-8"),

			// People section
			h.Div(
				h.Class("space-y-4"),
				h.Div(
					h.Class("flex items-center justify-between"),
					h.H3(h.Class("text-lg font-semibold text-foreground"), g.Text("People")),
				),
				h.P(h.Class("text-sm text-muted-foreground -mt-2"), g.Text("Family members, partners, and other important people")),

				// Add person form
				h.Form(
					h.Method("POST"),
					h.Action("/settings/my-life/relationship"),
					h.Input(h.Type("hidden"), h.Name("category"), h.Value("person")),
					h.Class("flex gap-3 mb-4"),
					shadcn.Input(shadcn.InputProps{
						Type:        "text",
						Name:        "name",
						Placeholder: "Name (e.g., Sarah, John Smith)",
						Required:    true,
						Class:       "flex-1",
					}),
					shadcn.NativeSelect(shadcn.NativeSelectProps{
						Name:  "relationship_type",
						Class: "!w-auto min-w-48",
					}, []shadcn.SelectOption{
						{Value: "partner", Label: "Partner/Spouse"},
						{Value: "parent", Label: "Parent"},
						{Value: "child", Label: "Child"},
						{Value: "sibling", Label: "Sibling"},
						{Value: "family_member", Label: "Family Member"},
						{Value: "friend", Label: "Friend"},
						{Value: "other", Label: "Other"},
					}),
					shadcn.Button(shadcn.ButtonProps{
						Variant: shadcn.ButtonDefault,
						Type:    "submit",
					},
						g.Text("Add"),
					),
				),

				// List of people
				g.If(len(people) == 0,
					h.P(h.Class("text-sm text-muted-foreground italic"), g.Text("No people added yet.")),
				),
				g.If(len(people) > 0,
					h.Div(
						h.Class("space-y-2"),
						g.Group(g.Map(people, func(rel *models.Relationship) g.Node {
							return renderRelationshipRow(rel)
						})),
					),
				),
			),

			// Work section
			h.Div(
				h.Class("space-y-4"),
				h.Div(
					h.Class("flex items-center justify-between"),
					h.H3(h.Class("text-lg font-semibold text-foreground"), g.Text("Work")),
				),
				h.P(h.Class("text-sm text-muted-foreground -mt-2"), g.Text("Your employer, clients, or business relationships")),

				// Add work form
				h.Form(
					h.Method("POST"),
					h.Action("/settings/my-life/relationship"),
					h.Input(h.Type("hidden"), h.Name("category"), h.Value("work")),
					h.Class("flex gap-3 mb-4"),
					shadcn.Input(shadcn.InputProps{
						Type:        "text",
						Name:        "name",
						Placeholder: "Company or client name",
						Required:    true,
						Class:       "flex-1",
					}),
					shadcn.NativeSelect(shadcn.NativeSelectProps{
						Name:  "relationship_type",
						Class: "!w-auto min-w-48",
					}, []shadcn.SelectOption{
						{Value: "employer", Label: "Employer"},
						{Value: "client", Label: "Client"},
						{Value: "contractor", Label: "Contractor"},
						{Value: "business_partner", Label: "Business Partner"},
						{Value: "other", Label: "Other"},
					}),
					shadcn.Button(shadcn.ButtonProps{
						Variant: shadcn.ButtonDefault,
						Type:    "submit",
					},
						g.Text("Add"),
					),
				),

				// List of work
				g.If(len(work) == 0,
					h.P(h.Class("text-sm text-muted-foreground italic"), g.Text("No work relationships added yet.")),
				),
				g.If(len(work) > 0,
					h.Div(
						h.Class("space-y-2"),
						g.Group(g.Map(work, func(rel *models.Relationship) g.Node {
							return renderRelationshipRow(rel)
						})),
					),
				),
			),

			// Assets section
			h.Div(
				h.Class("space-y-4"),
				h.Div(
					h.Class("flex items-center justify-between"),
					h.H3(h.Class("text-lg font-semibold text-foreground"), g.Text("Assets & Things")),
				),
				h.P(h.Class("text-sm text-muted-foreground -mt-2"), g.Text("Vehicles, properties, pets, and other important things")),

				// Add asset form
				h.Form(
					h.Method("POST"),
					h.Action("/settings/my-life/relationship"),
					h.Input(h.Type("hidden"), h.Name("category"), h.Value("asset")),
					h.Class("flex gap-3 mb-4"),
					shadcn.Input(shadcn.InputProps{
						Type:        "text",
						Name:        "name",
						Placeholder: "Name (e.g., My Tesla, Beach House, Max the Dog)",
						Required:    true,
						Class:       "flex-1",
					}),
					shadcn.NativeSelect(shadcn.NativeSelectProps{
						Name:  "relationship_type",
						Class: "!w-auto min-w-48",
					}, []shadcn.SelectOption{
						{Value: "vehicle", Label: "Vehicle"},
						{Value: "property", Label: "Property"},
						{Value: "pet", Label: "Pet"},
						{Value: "equipment", Label: "Equipment"},
						{Value: "other", Label: "Other"},
					}),
					shadcn.Button(shadcn.ButtonProps{
						Variant: shadcn.ButtonDefault,
						Type:    "submit",
					},
						g.Text("Add"),
					),
				),

				// List of assets
				g.If(len(assets) == 0,
					h.P(h.Class("text-sm text-muted-foreground italic"), g.Text("No assets added yet.")),
				),
				g.If(len(assets) > 0,
					h.Div(
						h.Class("space-y-2"),
						g.Group(g.Map(assets, func(rel *models.Relationship) g.Node {
							return renderRelationshipRow(rel)
						})),
					),
				),
			),

			// Info card
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					h.H3(h.Class("font-semibold text-foreground mb-2"), g.Text("How This Helps")),
					h.Ul(
						h.Class("space-y-2 text-sm text-muted-foreground"),
						h.Li(g.Text("• People you add help identify transfers vs. payments (e.g., sending money to your partner)")),
						h.Li(g.Text("• Work relationships help categorize salary and business income")),
						h.Li(g.Text("• Assets help track expenses related to vehicles, properties, and pets")),
						h.Li(g.Text("• This information improves AI categorization accuracy")),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

// renderRelationshipRow renders a single relationship row
func renderRelationshipRow(rel *models.Relationship) g.Node {
	typeLabel := rel.RelationshipType
	switch rel.RelationshipType {
	case "partner":
		typeLabel = "Partner/Spouse"
	case "parent":
		typeLabel = "Parent"
	case "child":
		typeLabel = "Child"
	case "sibling":
		typeLabel = "Sibling"
	case "family_member":
		typeLabel = "Family Member"
	case "friend":
		typeLabel = "Friend"
	case "employer":
		typeLabel = "Employer"
	case "client":
		typeLabel = "Client"
	case "contractor":
		typeLabel = "Contractor"
	case "business_partner":
		typeLabel = "Business Partner"
	case "vehicle":
		typeLabel = "Vehicle"
	case "property":
		typeLabel = "Property"
	case "pet":
		typeLabel = "Pet"
	case "equipment":
		typeLabel = "Equipment"
	}

	return h.Div(
		h.Class("flex items-center justify-between py-2 px-3 bg-secondary rounded-lg border border-border"),
		h.Div(
			h.Span(h.Class("font-medium text-foreground"), g.Text(rel.Name)),
			h.Span(h.Class("ml-2 text-sm text-muted-foreground"), g.Text("("+typeLabel+")")),
		),
		h.Form(
			h.Method("POST"),
			h.Action(fmt.Sprintf("/settings/my-life/relationship/%s/delete", rel.ID)),
			h.Button(
				h.Type("submit"),
				h.Class("text-destructive hover:opacity-80 text-sm"),
				g.Text("Remove"),
			),
		),
	)
}

// SettingsAddRelationship adds a new relationship
func (hdl *Handlers) SettingsAddRelationship(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	name := r.FormValue("name")
	category := r.FormValue("category")
	relationshipType := r.FormValue("relationship_type")

	if name == "" || category == "" || relationshipType == "" {
		http.Redirect(w, r, "/settings/my-life?error=required_fields", http.StatusSeeOther)
		return
	}

	relationshipStore := models.NewRelationshipStore(hdl.db.Pool)
	rel := &models.Relationship{
		LedgerID:         ledger.ID,
		Name:             name,
		Category:         category,
		RelationshipType: relationshipType,
	}

	if err := relationshipStore.Create(r.Context(), rel); err != nil {
		http.Error(w, "Failed to add relationship: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/my-life", http.StatusSeeOther)
}

// SettingsDeleteRelationship removes a relationship
func (hdl *Handlers) SettingsDeleteRelationship(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	relationshipStore := models.NewRelationshipStore(hdl.db.Pool)

	// Verify ownership before delete
	rel, err := relationshipStore.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Relationship not found", http.StatusNotFound)
		return
	}

	if rel.LedgerID != ledger.ID {
		http.Error(w, "Relationship not found", http.StatusNotFound)
		return
	}

	if err := relationshipStore.Delete(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete relationship: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/my-life", http.StatusSeeOther)
}

// institutionInfo holds institution data for display (groups multiple enrollments)
type institutionInfo struct {
	InstitutionName    string
	InstitutionID      string
	InstitutionLogoURL string
	LastSyncedAt       *time.Time
	EnrollmentIDs      []string          // All enrollment IDs for this institution
	Accounts           []*models.Account // All accounts across all enrollments
	Provider           string            // Provider name (teller, akahu, plaid)
}

// SettingsBanks shows connected banks with accounts grouped by institution
func (hdl *Handlers) SettingsBanks(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, _ := hdl.getCurrentLedger(r)

	// Get all accounts
	accounts, _ := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)

	// Group accounts by institution (not enrollment) so multiple Amex enrollments show as one
	institutions := make(map[string]*institutionInfo)
	var manualAccounts []*models.Account

	for _, acc := range accounts {
		// Skip system accounts (Income, Expense, etc.)
		if acc.Type != models.AccountTypeAsset && acc.Type != models.AccountTypeLiability {
			continue
		}

		// Check if account is connected: has ConnectionID, TellerEnrollmentID, or Provider+ExternalAccountID
		isConnected := acc.ConnectionID != "" || acc.TellerEnrollmentID != "" || (acc.Provider != "" && acc.ExternalAccountID != "")
		if isConnected {
			// Use institution name as key (fallback to enrollment ID if no institution)
			key := acc.InstitutionName
			if key == "" {
				key = acc.ConnectionID
				if key == "" {
					key = acc.TellerEnrollmentID // Fallback
				}
			}

			if existing, ok := institutions[key]; ok {
				existing.Accounts = append(existing.Accounts, acc)
				// Track unique enrollment IDs
				hasEnrollment := false
				for _, eid := range existing.EnrollmentIDs {
					if eid == acc.ConnectionID || eid == acc.TellerEnrollmentID {
						hasEnrollment = true
						break
					}
				}
				if !hasEnrollment {
					enrollmentID := acc.ConnectionID
					if enrollmentID == "" {
						enrollmentID = acc.TellerEnrollmentID // Fallback
					}
					existing.EnrollmentIDs = append(existing.EnrollmentIDs, enrollmentID)
				}
				// Update last synced time if this account has a more recent one
				if acc.LastSyncedAt != nil && (existing.LastSyncedAt == nil || acc.LastSyncedAt.After(*existing.LastSyncedAt)) {
					existing.LastSyncedAt = acc.LastSyncedAt
				}
				if existing.InstitutionID == "" && acc.InstitutionID != "" {
					existing.InstitutionID = acc.InstitutionID
				}
				if existing.InstitutionLogoURL == "" && acc.InstitutionLogoURL != "" {
					existing.InstitutionLogoURL = acc.InstitutionLogoURL
				}
			} else {
				provider := acc.Provider
				if provider == "" {
					provider = "teller" // Default for backwards compatibility
				}
				enrollmentID := acc.ConnectionID
				if enrollmentID == "" {
					enrollmentID = acc.TellerEnrollmentID // Fallback
				}
				institutions[key] = &institutionInfo{
					InstitutionName:    acc.InstitutionName,
					InstitutionID:      acc.InstitutionID,
					InstitutionLogoURL: acc.InstitutionLogoURL,
					LastSyncedAt:       acc.LastSyncedAt,
					EnrollmentIDs:      []string{enrollmentID},
					Accounts:           []*models.Account{acc},
					Provider:           provider,
				}
			}
		} else {
			// Manual/disconnected accounts
			manualAccounts = append(manualAccounts, acc)
		}
	}

	// Check which providers are configured
	tellerConfigured := hdl.cfg.TellerCert != "" && hdl.cfg.TellerKey != ""
	plaidConfigured := hdl.cfg.PlaidClientID != "" && hdl.cfg.PlaidSecret() != ""
	akahuConfigured := hdl.cfg.AkahuAppID != "" && hdl.cfg.AkahuAppSecret != ""

	page := layouts.SettingsLayout("Connected Banks", user.Email, "banks", user.ID.String(),
		shadcn.PageHeader("Connected Banks", "Manage your bank connections",
			// Connect Bank dropdown if multiple providers configured, or single button
			func() g.Node {
				// Show dropdown if multiple providers configured
				if plaidConfigured && (tellerConfigured || akahuConfigured) {
					return h.Div(
						h.Class("relative inline-block"),
						g.Attr("x-data", "{ open: false }"),
						shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonDefault},
							g.Attr("@click", "open = !open"),
							layouts.IconPlus(),
							g.Text("Connect Bank"),
							h.Span(h.Class("ml-1"), g.Raw(`<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path></svg>`)),
						),
						h.Div(
							h.Class("absolute right-0 mt-2 w-48 bg-card border border-border rounded-lg shadow-lg z-10"),
							g.Attr("x-show", "open"),
							g.Attr("@click.away", "open = false"),
							h.A(
								h.Href("/connections/connect/plaid"),
								h.Class("block px-4 py-2 text-sm text-foreground hover:bg-accent rounded-t-lg"),
								g.Text("🇺🇸 US Banks (Plaid)"),
							),
							g.If(tellerConfigured,
								h.A(
									h.Href("/connections/connect/teller"),
									h.Class("block px-4 py-2 text-sm text-foreground hover:bg-accent"),
									g.Text("🇺🇸 US Banks (Teller)"),
								),
							),
							g.If(akahuConfigured,
								h.A(
									h.Href("/connections/connect/akahu"),
									h.Class("block px-4 py-2 text-sm text-foreground hover:bg-accent rounded-b-lg"),
									g.Text("🇳🇿 NZ Banks (Akahu)"),
								),
							),
						),
					)
				} else if akahuConfigured {
					return h.A(
						h.Href("/connections/connect/akahu"),
						shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonDefault},
							layouts.IconPlus(),
							g.Text("Connect NZ Bank"),
						),
					)
				}
				// Default: Plaid (or Teller if Plaid not configured)
				if plaidConfigured {
					return h.A(
						h.Href("/connections/connect/plaid"),
						shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonDefault},
							layouts.IconPlus(),
							g.Text("Connect Bank"),
						),
					)
				}
				// Fallback to Teller if Plaid not configured
				return h.A(
					h.Href("/connections/connect/teller"),
					shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonDefault},
						layouts.IconPlus(),
						g.Text("Connect Bank"),
					),
				)
			}(),
		),

		h.Div(
			h.Class("space-y-6"),

			// Connected banks
			g.If(len(institutions) == 0 && len(manualAccounts) == 0,
				shadcn.EmptyNoData("No bank accounts yet", "Connect a bank to get started.", nil),
			),

			// Render each bank with its accounts (sorted alphabetically)
			g.Group(func() []g.Node {
				// Sort institution keys alphabetically
				keys := make([]string, 0, len(institutions))
				for k := range institutions {
					keys = append(keys, k)
				}
				sort.Strings(keys)

				var nodes []g.Node
				for _, k := range keys {
					nodes = append(nodes, hdl.renderBankCard(institutions[k]))
				}
				return nodes
			}()),

			// Manual/disconnected accounts
			g.If(len(manualAccounts) > 0,
				func() g.Node {
					// Sort manual accounts alphabetically
					sort.Slice(manualAccounts, func(i, j int) bool {
						return manualAccounts[i].Name < manualAccounts[j].Name
					})
					return shadcn.Card(shadcn.CardProps{},
						h.Div(
							h.Class("border-b border-border p-4"),
							h.Div(
								h.Class("flex items-center gap-3"),
								h.Div(
									h.Class("w-10 h-10 rounded-lg bg-secondary flex items-center justify-center text-muted-foreground flex-none"),
									layouts.IconBank(),
								),
								h.Div(
									h.P(h.Class("font-medium text-foreground"), g.Text("Manual & Disconnected Accounts")),
									h.P(h.Class("text-sm text-muted-foreground"), g.Text("Accounts not linked to a bank connection")),
								),
							),
						),
						h.Div(
							h.Class("divide-y divide-border"),
							g.Group(func() []g.Node {
								var accountNodes []g.Node
								for _, acc := range manualAccounts {
									accountNodes = append(accountNodes, renderSettingsAccountRow(acc, false))
								}
								return accountNodes
							}()),
						),
					)
				}(),
			),
		),
	)

	renderHTML(w, page)
}

// renderBankCard renders a card for a bank with its accounts
func (hdl *Handlers) renderBankCard(info *institutionInfo) g.Node {
	lastSyncedText := "Never synced"
	if info.LastSyncedAt != nil {
		lastSyncedText = "Last synced " + formatRelativeTime(*info.LastSyncedAt)
	}
	logoURL := hdl.getInstitutionLogoURLFull(info.InstitutionLogoURL)

	// Sort accounts alphabetically by name
	sort.Slice(info.Accounts, func(i, j int) bool {
		return info.Accounts[i].Name < info.Accounts[j].Name
	})

	// Join all enrollment IDs for multi-enrollment sync
	allEnrollmentIDs := strings.Join(info.EnrollmentIDs, ",")

	// Check for update mode status - find most urgent status across all accounts
	var mostUrgentStatus string
	var connectionIDForBanner string
	statusPriority := map[string]int{
		"login_required":         4, // Highest priority
		"pending_disconnect":     3,
		"pending_expiration":     2,
		"new_accounts_available": 1, // Lowest priority
	}
	maxPriority := 0
	for _, acc := range info.Accounts {
		if acc.Provider == "plaid" && acc.ConnectionStatus != "" {
			priority := statusPriority[acc.ConnectionStatus]
			if priority > maxPriority {
				maxPriority = priority
				mostUrgentStatus = acc.ConnectionStatus
				connectionIDForBanner = acc.ConnectionID
			}
		}
	}

	return shadcn.Card(shadcn.CardProps{},
		// Update mode banner (if needed)
		g.If(mostUrgentStatus != "",
			func() g.Node {
				var message, buttonText string
				var variant shadcn.AlertVariant = shadcn.AlertWarning

				switch mostUrgentStatus {
				case "login_required":
					message = "Your bank connection needs re-authentication. Please reconnect to continue syncing."
					buttonText = "Reconnect Now"
					variant = shadcn.AlertDestructive
				case "pending_disconnect":
					message = "Your bank connection will be disconnected soon. Please reconnect to continue syncing."
					buttonText = "Reconnect Now"
					variant = shadcn.AlertDestructive
				case "pending_expiration":
					message = "Your bank connection will expire soon. Please reconnect to continue syncing."
					buttonText = "Reconnect Now"
					variant = shadcn.AlertWarning
				case "new_accounts_available":
					message = "New accounts are available for your bank. Click to add them."
					buttonText = "Add New Accounts"
					variant = shadcn.AlertInfo
				default:
					message = "Your bank connection needs attention. Please reconnect."
					buttonText = "Reconnect Now"
				}

				reconnectURL := "/connections/reconnect/plaid?connection_id=" + connectionIDForBanner
				if mostUrgentStatus == "new_accounts_available" {
					reconnectURL += "&account_selection_enabled=true"
				}

				return h.Div(
					h.Class("p-4 border-b border-border"),
					shadcn.Alert(shadcn.AlertProps{Variant: variant},
						h.Div(
							h.Class("flex items-start gap-3"),
							h.Div(
								h.Class("mt-0.5"),
								shadcn.IconAlertTriangle(),
							),
							h.Div(
								h.Class("flex-1"),
								h.Div(
									h.Class("font-medium mb-1"),
									g.Text(info.InstitutionName+" - Action Required"),
								),
								h.Div(
									h.Class("text-sm mb-3"),
									g.Text(message),
								),
								h.A(
									h.Href(reconnectURL),
									shadcn.Button(shadcn.ButtonProps{
										Variant: shadcn.ButtonDefault,
										Size:    shadcn.ButtonSizeSm,
									},
										layouts.IconLink(),
										g.Text(buttonText),
									),
								),
							),
						),
					),
				)
			}(),
		),
		// Bank header
		h.Div(
			h.Class("flex items-center justify-between p-4 border-b border-border"),
			h.Div(
				h.Class("flex items-center gap-3 min-w-0"),
				func() g.Node {
					if logoURL != "" {
						return h.Img(
							h.Src(logoURL),
							h.Alt(info.InstitutionName),
							h.Class("w-10 h-10 rounded-lg object-contain flex-none bg-card"),
						)
					}
					return h.Div(
						h.Class("w-10 h-10 rounded-lg bg-secondary flex items-center justify-center text-muted-foreground flex-none"),
						layouts.IconBank(),
					)
				}(),
				h.Div(
					h.Class("min-w-0"),
					h.P(h.Class("font-medium text-foreground truncate"), g.Text(info.InstitutionName)),
					h.P(h.Class("text-sm text-muted-foreground"), g.Text(lastSyncedText)),
				),
			),
			// Bank-level actions - sync all enrollments for this institution
			func() g.Node {
				// Use provider-specific endpoints
				if info.Provider == "plaid" {
					connectionID := ""
					if len(info.EnrollmentIDs) > 0 {
						connectionID = info.EnrollmentIDs[0]
					}
					return h.Div(
						h.Class("flex items-center gap-1 flex-none"),
						h.Form(
							h.Method("POST"),
							h.Action("/connections/sync/plaid/"+connectionID),
							h.Button(
								h.Type("submit"),
								h.Class("flex items-center justify-center w-10 h-10 rounded-lg text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"),
								h.Title("Sync all accounts"),
								layouts.IconRefresh(),
							),
						),
						h.Form(
							h.Method("POST"),
							h.Action("/connections/disconnect/plaid/"+connectionID),
							h.Input(h.Type("hidden"), h.Name("_method"), h.Value("DELETE")),
							h.Button(
								h.Type("submit"),
								h.Class("flex items-center justify-center w-10 h-10 rounded-lg text-muted-foreground hover:text-destructive hover:bg-accent transition-colors"),
								h.Title("Disconnect bank"),
								g.Attr("onclick", "return confirm('Disconnect this bank? Account data will be preserved.')"),
								layouts.IconX(),
							),
						),
						h.Form(
							h.Method("POST"),
							h.Action("/settings/banks/institution/delete"),
							h.Input(h.Type("hidden"), h.Name("institution_name"), h.Value(info.InstitutionName)),
							g.If(info.InstitutionID != "",
								h.Input(h.Type("hidden"), h.Name("institution_id"), h.Value(info.InstitutionID)),
							),
							h.Input(h.Type("hidden"), h.Name("provider"), h.Value("plaid")),           // For logging/audit
							h.Input(h.Type("hidden"), h.Name("connection_id"), h.Value(connectionID)), // Fallback for edge cases
							h.Button(
								h.Type("submit"),
								h.Class("flex items-center justify-center w-10 h-10 rounded-lg text-muted-foreground hover:text-destructive hover:bg-accent transition-colors"),
								h.Title("Delete all accounts"),
								g.Attr("onclick", fmt.Sprintf("return confirm('Delete ALL %d accounts for %s and ALL their transactions? This cannot be undone.')", len(info.Accounts), info.InstitutionName)),
								layouts.IconTrash(),
							),
						),
					)
				}
				if info.Provider == "akahu" {
					connectionID := ""
					if len(info.EnrollmentIDs) > 0 {
						connectionID = info.EnrollmentIDs[0]
					}
					return h.Div(
						h.Class("flex items-center gap-1 flex-none"),
						h.Form(
							h.Method("POST"),
							h.Action("/connections/sync/akahu/"+connectionID),
							h.Button(
								h.Type("submit"),
								h.Class("flex items-center justify-center w-10 h-10 rounded-lg text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"),
								h.Title("Sync all accounts"),
								layouts.IconRefresh(),
							),
						),
						h.Form(
							h.Method("POST"),
							h.Action("/connections/disconnect/akahu/"+connectionID),
							h.Input(h.Type("hidden"), h.Name("_method"), h.Value("DELETE")),
							h.Button(
								h.Type("submit"),
								h.Class("flex items-center justify-center w-10 h-10 rounded-lg text-muted-foreground hover:text-destructive hover:bg-accent transition-colors"),
								h.Title("Disconnect bank"),
								g.Attr("onclick", "return confirm('Disconnect this bank? Account data will be preserved.')"),
								layouts.IconX(),
							),
						),
						h.Form(
							h.Method("POST"),
							h.Action("/settings/banks/institution/delete"),
							h.Input(h.Type("hidden"), h.Name("institution_name"), h.Value(info.InstitutionName)),
							g.If(info.InstitutionID != "",
								h.Input(h.Type("hidden"), h.Name("institution_id"), h.Value(info.InstitutionID)),
							),
							h.Input(h.Type("hidden"), h.Name("provider"), h.Value("akahu")),           // For logging/audit
							h.Input(h.Type("hidden"), h.Name("connection_id"), h.Value(connectionID)), // Fallback for edge cases
							h.Button(
								h.Type("submit"),
								h.Class("flex items-center justify-center w-10 h-10 rounded-lg text-muted-foreground hover:text-destructive hover:bg-accent transition-colors"),
								h.Title("Delete all accounts"),
								g.Attr("onclick", fmt.Sprintf("return confirm('Delete ALL %d accounts for %s and ALL their transactions? This cannot be undone.')", len(info.Accounts), info.InstitutionName)),
								layouts.IconTrash(),
							),
						),
					)
				}
				// Default: Teller endpoints
				return h.Div(
					h.Class("flex items-center gap-1 flex-none"),
					h.Form(
						h.Method("POST"),
						h.Action("/teller/sync-multi"),
						h.Input(h.Type("hidden"), h.Name("enrollment_ids"), h.Value(allEnrollmentIDs)),
						h.Button(
							h.Type("submit"),
							h.Class("flex items-center justify-center w-10 h-10 rounded-lg text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"),
							h.Title("Sync all accounts"),
							layouts.IconRefresh(),
						),
					),
					h.Form(
						h.Method("POST"),
						h.Action("/teller/resync-multi"),
						h.Input(h.Type("hidden"), h.Name("enrollment_ids"), h.Value(allEnrollmentIDs)),
						h.Button(
							h.Type("submit"),
							h.Class("flex items-center justify-center w-10 h-10 rounded-lg text-muted-foreground hover:text-primary hover:bg-accent transition-colors"),
							h.Title("Full resync all accounts"),
							g.Attr("onclick", "return confirm('Run full resync? This will update all transactions with the latest data from your bank.')"),
							layouts.IconDatabase(),
						),
					),
					h.Form(
						h.Method("POST"),
						h.Action("/teller/disconnect-multi"),
						h.Input(h.Type("hidden"), h.Name("enrollment_ids"), h.Value(allEnrollmentIDs)),
						h.Input(h.Type("hidden"), h.Name("_method"), h.Value("DELETE")),
						h.Button(
							h.Type("submit"),
							h.Class("flex items-center justify-center w-10 h-10 rounded-lg text-muted-foreground hover:text-destructive hover:bg-accent transition-colors"),
							h.Title("Disconnect bank"),
							g.Attr("onclick", "return confirm('Disconnect this bank? Account data will be preserved.')"),
							layouts.IconX(),
						),
					),
					h.Form(
						h.Method("POST"),
						h.Action("/settings/banks/institution/delete"),
						h.Input(h.Type("hidden"), h.Name("institution_name"), h.Value(info.InstitutionName)),
						g.If(info.InstitutionID != "",
							h.Input(h.Type("hidden"), h.Name("institution_id"), h.Value(info.InstitutionID)),
						),
						h.Input(h.Type("hidden"), h.Name("provider"), h.Value("teller")),               // For logging/audit
						h.Input(h.Type("hidden"), h.Name("enrollment_ids"), h.Value(allEnrollmentIDs)), // Fallback for edge cases
						h.Button(
							h.Type("submit"),
							h.Class("flex items-center justify-center w-10 h-10 rounded-lg text-muted-foreground hover:text-destructive hover:bg-accent transition-colors"),
							h.Title("Delete all accounts"),
							g.Attr("onclick", fmt.Sprintf("return confirm('Delete ALL %d accounts for %s and ALL their transactions? This cannot be undone.')", len(info.Accounts), info.InstitutionName)),
							layouts.IconTrash(),
						),
					),
				)
			}(),
		),
		// Account list
		h.Div(
			h.Class("divide-y divide-border"),
			g.Group(func() []g.Node {
				var accountNodes []g.Node
				for _, acc := range info.Accounts {
					accountNodes = append(accountNodes, renderSettingsAccountRow(acc, true))
				}
				return accountNodes
			}()),
		),
	)
}

// renderSettingsAccountRow renders a single account row with actions for the settings page
func renderSettingsAccountRow(acc *models.Account, hasEnrollment bool) g.Node {
	// Build account subtitle
	subtitle := string(acc.Type)
	if acc.AccountSubtype != "" {
		subtitle = acc.AccountSubtype
	} else if acc.TellerSubtype != "" {
		// Fallback for backward compatibility
		subtitle = acc.TellerSubtype
	}
	if acc.LastFour != "" {
		subtitle += " •••• " + acc.LastFour
	}

	return h.Div(
		h.Class("flex items-center justify-between p-4 pl-6 hover:bg-accent"),
		h.Div(
			h.Class("min-w-0"),
			h.P(h.Class("font-medium text-foreground truncate"), g.Text(acc.Name)),
			h.P(h.Class("text-sm text-muted-foreground"), g.Text(subtitle)),
		),
		// Account actions
		h.Div(
			h.Class("flex items-center gap-1 flex-none"),
			// Link/reconnect button for disconnected accounts
			g.If(!hasEnrollment || (acc.AccessToken == "" && acc.TellerAccessToken == ""),
				h.A(
					h.Href("/teller/link/"+acc.ID.String()),
					h.Class("flex items-center justify-center w-9 h-9 rounded-lg text-muted-foreground hover:text-primary hover:bg-accent transition-colors"),
					h.Title("Link to bank"),
					layouts.IconLink(),
				),
			),
			// Delete button
			h.Form(
				h.Method("POST"),
				h.Action(fmt.Sprintf("/settings/banks/account/%s/delete", acc.ID)),
				h.Button(
					h.Type("submit"),
					h.Class("flex items-center justify-center w-9 h-9 rounded-lg text-muted-foreground hover:text-destructive hover:bg-accent transition-colors"),
					h.Title("Delete account and all transactions"),
					g.Attr("onclick", fmt.Sprintf("return confirm('Delete %s and ALL its transactions? This cannot be undone.')", acc.Name)),
					layouts.IconTrash(),
				),
			),
		),
	)
}

// SettingsLedger shows ledger settings
func (hdl *Handlers) SettingsLedger(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, _ := hdl.getCurrentLedger(r)

	page := layouts.SettingsLayout("Ledger Settings", user.Email, "ledger", user.ID.String(),
		shadcn.PageHeader("Ledger", "Manage your ledger settings"),

		shadcn.Card(shadcn.CardProps{},
			shadcn.CardContentFull(
				h.Div(
					h.Class("space-y-4"),
					h.Div(
						h.P(h.Class("text-sm text-muted-foreground"), g.Text("Ledger Name")),
						h.P(h.Class("font-medium text-foreground"), g.Text(ledger.Name)),
					),
					h.Div(
						h.P(h.Class("text-sm text-muted-foreground"), g.Text("Currency")),
						h.P(h.Class("font-medium text-foreground"), g.Text(ledger.Currency)),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

// SettingsCategories redirects to tags page (consolidated)
func (hdl *Handlers) SettingsCategories(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/tags", http.StatusFound)
}

func (hdl *Handlers) SettingsUpdateProfile(w http.ResponseWriter, r *http.Request) {
	// Profile updates not implemented yet
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (hdl *Handlers) SettingsUpdatePassword(w http.ResponseWriter, r *http.Request) {
	// Password updates handled by authboss
	// This would need custom implementation
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (hdl *Handlers) SettingsSeedTags(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Seed the default taxonomy tags
	if err := hdl.taxonomy.SeedDefaultTags(r.Context(), ledger.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/tags?seeded=1", http.StatusSeeOther)
}

// SettingsDangerZone shows danger zone settings
func (hdl *Handlers) SettingsDangerZone(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)

	page := layouts.SettingsLayout("Danger Zone", user.Email, "danger", user.ID.String(),
		shadcn.PageHeader("Danger Zone", "Irreversible and heavy operations"),

		// Rebuild Operations section
		h.Div(
			h.Class("mb-8"),
			h.H3(h.Class("text-lg font-semibold text-foreground mb-4"), g.Text("Rebuild Operations")),
			h.P(h.Class("text-sm text-muted-foreground mb-4"), g.Text("These operations reprocess all your data. They may take a while to complete.")),
			h.Div(
				h.Class("grid grid-cols-1 sm:grid-cols-3 gap-4 sm:gap-6"),

				// Full Reprocessing
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardContentFull(
						h.H4(h.Class("text-base font-semibold text-card-foreground mb-2"), g.Text("Full Reprocessing")),
						h.P(h.Class("text-sm text-muted-foreground mb-4"), g.Text("Re-enrich and re-categorize all transactions. Clears entity links, tags, and display titles.")),
						h.Form(
							h.Method("POST"),
							h.Action("/settings/danger/reprocess-all"),
							shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonDestructive},
								h.Type("submit"),
								g.Attr("onclick", "return confirm('Reprocess all transactions? This will clear all tags and entity assignments.')"),
								g.Text("Reprocess All"),
							),
						),
					),
				),

				// Pattern Reprocessing
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardContentFull(
						h.H4(h.Class("text-base font-semibold text-card-foreground mb-2"), g.Text("Pattern Reprocessing")),
						h.P(h.Class("text-sm text-muted-foreground mb-4"), g.Text("Re-detect all recurring patterns. Clears existing pattern assignments.")),
						h.Form(
							h.Method("POST"),
							h.Action("/settings/danger/reprocess-patterns"),
							h.Button(
								h.Type("submit"),
								h.Class("inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2.5 text-sm font-medium transition-colors bg-ring text-primary-foreground hover:opacity-90 focus:ring-ring"),
								g.Attr("onclick", "return confirm('Reprocess all patterns? This will clear all pattern detections.')"),
								g.Text("Reprocess Patterns"),
							),
						),
					),
				),

				// Rebuild Statements
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardContentFull(
						h.H4(h.Class("text-base font-semibold text-card-foreground mb-2"), g.Text("Recalculate Balances")),
						h.P(h.Class("text-sm text-muted-foreground mb-4"), g.Text("Recalculate opening balances for all connected accounts.")),
						h.Form(
							h.Method("POST"),
							h.Action("/settings/danger/recalculate-balances"),
							h.Button(
								h.Type("submit"),
								h.Class("inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2.5 text-sm font-medium transition-colors bg-ring text-primary-foreground hover:opacity-90 focus:ring-ring"),
								g.Attr("onclick", "return confirm('Recalculate all account opening balances?')"),
								g.Text("Recalculate Balances"),
							),
						),
					),
				),
			),
		),

		// Destructive Actions section
		h.Div(
			h.H3(h.Class("text-lg font-semibold text-destructive mb-4"), g.Text("Destructive Actions")),
			h.P(h.Class("text-sm text-muted-foreground mb-4"), g.Text("These actions permanently delete data and cannot be undone.")),
			h.Div(
				h.Class("grid grid-cols-1 sm:grid-cols-2 gap-4 sm:gap-6"),

				// Delete Data
				shadcn.Card(shadcn.CardProps{Class: "border-destructive/50"},
					shadcn.CardContentFull(
						h.H4(h.Class("text-base font-semibold text-card-foreground mb-2"), g.Text("Delete All Data")),
						h.P(h.Class("text-sm text-muted-foreground mb-4"), g.Text("Wipe all data and start fresh.")),
						h.Form(
							h.Method("POST"),
							h.Action("/settings/danger/delete-data"),
							h.ID("delete-data-form"),
							shadcn.Button(shadcn.ButtonProps{
								Variant: shadcn.ButtonDestructive,
								Type:    "submit",
							},
								g.Attr("onclick", "return confirm('Delete all your data? This cannot be undone.')"),
								g.Text("Delete Data"),
							),
						),
					),
				),

				// Delete Account
				shadcn.Card(shadcn.CardProps{Class: "border-destructive/50"},
					shadcn.CardContentFull(
						h.H4(h.Class("text-base font-semibold text-card-foreground mb-2"), g.Text("Delete Account")),
						h.P(h.Class("text-sm text-muted-foreground mb-4"), g.Text("Permanently delete your account.")),
						h.Form(
							h.Method("POST"),
							h.Action("/settings/danger/delete-account"),
							shadcn.Button(shadcn.ButtonProps{
								Variant: shadcn.ButtonDestructive,
								Type:    "submit",
							},
								g.Attr("onclick", "return confirm('Delete your account? This cannot be undone.')"),
								g.Text("Delete Account"),
							),
						),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

// SettingsDeleteData deletes all user data but keeps the account
func (hdl *Handlers) SettingsDeleteData(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}

	slog.InfoContext(r.Context(), "delete all data requested", "id", user.ID, "email", user.Email)

	// Delete all ledgers user has access to (cascade deletes accounts, transactions, tags, etc.)
	ledgerPerms, err := hdl.permissionChecker.GetUserLedgerPermissions(r.Context(), user.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get ledgers for user", "id", user.ID, "err", err)
		http.Error(w, "Failed to get ledgers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "Found  ledger(s) for user", "count", len(ledgerPerms), "id", user.ID)

	if len(ledgerPerms) == 0 {
		slog.InfoContext(r.Context(), "user has no ledgers to delete", "user_id", user.ID)
		// User has no data, just redirect
		http.Redirect(w, r, "/intelligence?deleted=data", http.StatusSeeOther)
		return
	}

	deletedCount := 0
	var failedLedgers []uuid.UUID
	for ledgerID := range ledgerPerms {
		// Check if user has owner permission (required to delete)
		_, err := hdl.permissionChecker.CheckLedgerPermission(r.Context(), user.ID, ledgerID, models.PermissionLevelOwner)
		if err != nil {
			slog.ErrorContext(r.Context(), "[settings] User does not have owner permission for ledger, skipping", "user_id", user.ID, "ledger_id", ledgerID, "err", err)
			failedLedgers = append(failedLedgers, ledgerID)
			continue // Skip ledgers user can't delete
		}

		// Verify ledger exists before deletion
		ledger, err := hdl.ledgers.GetByID(r.Context(), ledgerID)
		if err != nil {
			slog.WarnContext(r.Context(), "Ledger not found (may already be deleted)", "ledger_id", ledgerID, "err", err)
			continue
		}

		// Verify the ledger's user_id is valid (exists in users table)
		// This prevents foreign key constraint violations during deletion
		ledgerUser, err := hdl.users.GetByID(r.Context(), ledger.UserID)
		if err != nil {
			slog.ErrorContext(r.Context(), "[settings] Ledger has invalid user_id; attempting to fix", "ledger_id", ledgerID, "bad_user_id", ledger.UserID, "err", err)
			// Try to fix the data integrity issue by updating the user_id to the current user
			// This allows the deletion to proceed
			_, updateErr := hdl.db.Pool.Exec(r.Context(), `UPDATE ledgers SET user_id = $1 WHERE id = $2`, user.ID, ledgerID)
			if updateErr != nil {
				slog.ErrorContext(r.Context(), "[settings] Failed to fix ledger user_id, skipping deletion", "ledger_id", ledgerID, "err", updateErr)
				failedLedgers = append(failedLedgers, ledgerID)
				continue
			}
			slog.InfoContext(r.Context(), "[settings] Fixed ledger user_id", "ledger_id", ledgerID, "old_user_id", ledger.UserID, "new_user_id", user.ID)
			ledger.UserID = user.ID // Update for logging
		} else if ledgerUser.ID != user.ID {
			slog.WarnContext(r.Context(), "[settings] Ledger user_id mismatch; user has permission via entity permissions", "ledger_id", ledgerID, "ledger_user_id", ledger.UserID, "current_user_id", user.ID)
		}

		slog.InfoContext(r.Context(), "[settings] Deleting ledger", "ledger_id", ledgerID, "name", ledger.Name, "user_id", user.ID, "ledger_user_id", ledger.UserID)
		if err := hdl.ledgers.Delete(r.Context(), ledgerID); err != nil {
			slog.ErrorContext(r.Context(), "[settings] Failed to delete ledger", "ledger_id", ledgerID, "user_id", user.ID, "err", err)
			// Check if it's a foreign key constraint error
			if strings.Contains(err.Error(), "foreign key constraint") || strings.Contains(err.Error(), "23503") {
				slog.InfoContext(r.Context(), "Foreign key constraint violation detected. This may indicate data corruption. Ledger user_id: , Current user", "user_id", ledger.UserID, "id", user.ID)
			}
			failedLedgers = append(failedLedgers, ledgerID)
			// Continue with other ledgers instead of failing completely
			continue
		}

		// Verify deletion succeeded
		_, err = hdl.ledgers.GetByID(r.Context(), ledgerID)
		if err == nil {
			slog.WarnContext(r.Context(), "ledger still exists after deletion attempt", "ledger_id", ledgerID)
			failedLedgers = append(failedLedgers, ledgerID)
			continue
		}

		deletedCount++
		slog.InfoContext(r.Context(), "[settings] Successfully deleted ledger", "ledger_id", ledgerID, "name", ledger.Name, "user_id", user.ID)
	}

	if deletedCount == 0 {
		if len(failedLedgers) > 0 {
			slog.ErrorContext(r.Context(), "[settings] User attempted to delete data but failed on all ledgers", "user_id", user.ID, "failed_count", len(failedLedgers), "failed_ledgers", failedLedgers)
			http.Error(w, "Failed to delete any ledgers. You may not have owner permissions, or the deletion failed. Check logs for details.", http.StatusInternalServerError)
		} else {
			slog.InfoContext(r.Context(), "user attempted to delete data but has no owner permissions on any ledgers", "user_id", user.ID)
			http.Error(w, "You don't have permission to delete any ledgers", http.StatusForbidden)
		}
		return
	}

	if len(failedLedgers) > 0 {
		slog.ErrorContext(r.Context(), "[settings] Partially deleted ledgers", "deleted", deletedCount, "failed", len(failedLedgers), "user_id", user.ID)
	} else {
		slog.InfoContext(r.Context(), "Successfully deleted all  ledger(s) for user", "count", deletedCount, "id", user.ID)
	}

	// Verify deletion by checking if any ledgers remain
	verifyPerms, err := hdl.permissionChecker.GetUserLedgerPermissions(r.Context(), user.ID)
	if err == nil && len(verifyPerms) > 0 {
		slog.WarnContext(r.Context(), "WARNING: After deletion, user  still has access to  ledger(s)! This should not happen.", "id", user.ID, "count", len(verifyPerms))
		http.Error(w, fmt.Sprintf("Deletion may have failed. %d ledger(s) still exist. Check logs for details.", len(verifyPerms)), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "verification passed: user has no remaining ledgers", "user_id", user.ID)

	// Redirect to dashboard - a fresh ledger will be created automatically when they navigate
	http.Redirect(w, r, "/intelligence?deleted=data", http.StatusSeeOther)
}

// SettingsDeleteAccount deletes the entire user account
func (hdl *Handlers) SettingsDeleteAccount(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}

	// Delete the user (cascade deletes everything)
	if err := hdl.users.Delete(r.Context(), user.ID); err != nil {
		http.Error(w, "Failed to delete account: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log them out by redirecting to logout
	http.Redirect(w, r, "/auth/logout", http.StatusSeeOther)
}

// SettingsReprocessAll resets all transactions for full re-enrichment and re-categorization
func (hdl *Handlers) SettingsReprocessAll(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Reset all transactions: clear entity links, display titles, tags, and queue for reprocessing
	result, err := hdl.db.Pool.Exec(ctx, `
		UPDATE transactions SET
			entity_id = NULL,
			counterparty_entity_id = NULL,
			intermediary_entity_id = NULL,
			display_title = NULL,
			enrichment_status = 'pending',
			enrichment_attempts = 0,
			enrichment_error = NULL,
			enriched_at = NULL,
			categorization_status = 'pending',
			categorization_queued_at = NOW(),
			categorization_attempts = 0,
			categorization_error = NULL,
			categorization_completed_at = NULL,
			updated_at = NOW()
		WHERE ledger_id = $1
	`, ledger.ID)
	if err != nil {
		http.Error(w, "Failed to reset transactions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Delete all tags from transactions in this ledger
	_, err = hdl.db.Pool.Exec(ctx, `
		DELETE FROM transaction_tags 
		WHERE transaction_id IN (SELECT id FROM transactions WHERE ledger_id = $1)
	`, ledger.ID)
	if err != nil {
		http.Error(w, "Failed to clear tags: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "full reprocessing queued", "id", ledger.ID, "rows_affected", result.RowsAffected())
	http.Redirect(w, r, "/settings/danger?reprocessed=all", http.StatusSeeOther)
}

// SettingsReprocessPatterns resets pattern detection for all transactions
func (hdl *Handlers) SettingsReprocessPatterns(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Reset pattern detection for ALL transactions (including dismissed patterns)
	result, err := hdl.db.Pool.Exec(ctx, `
		UPDATE transactions SET
			pattern_detection_status = 'queued',
			pattern_detection_attempts = 0,
			pattern_detection_error = NULL,
			pattern_type = NULL,
			pattern_metadata = NULL,
			updated_at = NOW()
		WHERE ledger_id = $1 AND is_transfer = false
	`, ledger.ID)
	if err != nil {
		http.Error(w, "Failed to reset pattern detection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "pattern reprocessing queued", "id", ledger.ID, "rows_affected", result.RowsAffected())
	http.Redirect(w, r, "/settings/danger?reprocessed=patterns", http.StatusSeeOther)
}

// SettingsRecalculateBalances recalculates opening balances for all connected accounts
func (hdl *Handlers) SettingsRecalculateBalances(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Get all accounts in this ledger that have opening balance transactions
	// Delete existing opening balance transactions and let them be recalculated on next sync
	result, err := hdl.db.Pool.Exec(ctx, `
		DELETE FROM transactions 
		WHERE ledger_id = $1 
		AND description = 'Opening Balance'
	`, ledger.ID)
	if err != nil {
		http.Error(w, "Failed to delete opening balances: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "deleted opening balance transactions, will recalculate on next sync", "rows_affected", result.RowsAffected(), "id", ledger.ID)
	http.Redirect(w, r, "/settings/danger?recalculated=balances", http.StatusSeeOther)
}

// SettingsDeleteBankAccount deletes a bank account and all its transactions
func (hdl *Handlers) SettingsDeleteBankAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := mustParamUUID(w, r, "id", "account ID")
	if !ok {
		return
	}

	// Verify the account belongs to the current user's ledger
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	if account.LedgerID != ledger.ID {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	// Delete account and all its transactions
	deleted, err := hdl.accounts.DeleteWithTransactions(r.Context(), accountID)
	if err != nil {
		http.Error(w, "Failed to delete account: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "account deleted", "account_name", account.Name, "account_id", accountID, "transactions_deleted", deleted)

	http.Redirect(w, r, "/settings/banks?deleted="+account.Name, http.StatusSeeOther)
}

// SettingsDeleteInstitutionAccounts deletes all accounts for an institution
// Deletes accounts by institution_name (and optionally institution_id) regardless of provider
func (hdl *Handlers) SettingsDeleteInstitutionAccounts(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Primary identifier: institution_name (required)
	institutionName := strings.TrimSpace(r.FormValue("institution_name"))
	// Optional: institution_id for more precise matching
	institutionID := strings.TrimSpace(r.FormValue("institution_id"))
	// Optional: provider for logging/audit (backwards compatibility)
	provider := r.FormValue("provider")

	// Fallback: if institution_name not provided, try provider-specific deletion for backwards compatibility
	if institutionName == "" {
		// Legacy behavior: delete by provider + connection/enrollment IDs
		if provider == "" {
			http.Error(w, "Missing institution_name or provider", http.StatusBadRequest)
			return
		}

		// Get all accounts for this ledger
		accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var accountsToDelete []*models.Account

		if provider == "teller" {
			enrollmentIDsStr := r.FormValue("enrollment_ids")
			if enrollmentIDsStr == "" {
				http.Error(w, "Missing enrollment IDs", http.StatusBadRequest)
				return
			}
			enrollmentIDs := strings.Split(enrollmentIDsStr, ",")
			for _, acc := range accounts {
				for _, eid := range enrollmentIDs {
					if acc.Provider == "teller" && (acc.ConnectionID == eid || acc.TellerEnrollmentID == eid) {
						accountsToDelete = append(accountsToDelete, acc)
						if institutionName == "" {
							institutionName = acc.InstitutionName
						}
						break
					}
				}
			}
		} else if provider == "plaid" || provider == "akahu" {
			connectionID := r.FormValue("connection_id")
			if connectionID == "" {
				http.Error(w, "Missing connection ID", http.StatusBadRequest)
				return
			}
			for _, acc := range accounts {
				if acc.Provider == provider && acc.ConnectionID == connectionID {
					accountsToDelete = append(accountsToDelete, acc)
					if institutionName == "" {
						institutionName = acc.InstitutionName
					}
				}
			}
		} else {
			http.Error(w, "Invalid provider", http.StatusBadRequest)
			return
		}

		if len(accountsToDelete) == 0 {
			http.Redirect(w, r, "/settings/banks", http.StatusSeeOther)
			return
		}

		// Delete all accounts and their transactions
		totalDeleted := int64(0)
		providerBreakdown := make(map[string]int)
		for _, acc := range accountsToDelete {
			deleted, err := hdl.accounts.DeleteWithTransactions(r.Context(), acc.ID)
			if err != nil {
				slog.ErrorContext(r.Context(), "Failed to delete account", "id", acc.ID, "err", err)
				continue
			}
			totalDeleted += deleted
			providerBreakdown[acc.Provider]++
		}

		// Log for audit
		slog.InfoContext(r.Context(), "[settings] Deleted accounts", "count", len(accountsToDelete), "institution", institutionName, "provider", provider, "breakdown", providerBreakdown, "transactions", totalDeleted)

		http.Redirect(w, r, "/settings/banks?deleted="+institutionName, http.StatusSeeOther)
		return
	}

	// Primary path: delete by institution name (all providers)
	// Get all accounts for this ledger
	accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find all accounts matching the institution name/ID, regardless of provider
	var accountsToDelete []*models.Account
	providerBreakdown := make(map[string]int)

	for _, acc := range accounts {
		// Only delete Asset and Liability accounts (exclude system accounts like Income, Expense)
		if acc.Type != models.AccountTypeAsset && acc.Type != models.AccountTypeLiability {
			continue
		}

		// Match by institution_name (case-insensitive, trimmed)
		accInstitutionName := strings.TrimSpace(acc.InstitutionName)
		matchesName := accInstitutionName != "" && strings.EqualFold(accInstitutionName, institutionName)

		// If institution_id provided, also match by that for more precision
		matchesID := true
		if institutionID != "" {
			accInstitutionID := strings.TrimSpace(acc.InstitutionID)
			matchesID = accInstitutionID != "" && accInstitutionID == institutionID
		}

		if matchesName && matchesID {
			accountsToDelete = append(accountsToDelete, acc)
			providerBreakdown[acc.Provider]++
		}
	}

	if len(accountsToDelete) == 0 {
		// If no accounts found by name, try fallback: accounts with empty institution_name but matching connection IDs
		// This handles edge cases where accounts don't have institution_name set
		connectionID := r.FormValue("connection_id")
		if connectionID != "" {
			for _, acc := range accounts {
				if acc.Type == models.AccountTypeAsset || acc.Type == models.AccountTypeLiability {
					if acc.ConnectionID == connectionID {
						accountsToDelete = append(accountsToDelete, acc)
						providerBreakdown[acc.Provider]++
					}
				}
			}
		}

		if len(accountsToDelete) == 0 {
			http.Redirect(w, r, "/settings/banks", http.StatusSeeOther)
			return
		}
	}

	// Delete all matching accounts and their transactions
	totalDeleted := int64(0)
	for _, acc := range accountsToDelete {
		deleted, err := hdl.accounts.DeleteWithTransactions(r.Context(), acc.ID)
		if err != nil {
			slog.ErrorContext(r.Context(), "[settings] Failed to delete account", "id", acc.ID, "name", acc.Name, "provider", acc.Provider, "err", err)
			continue
		}
		totalDeleted += deleted
	}

	// Log for audit with provider breakdown
	providersList := make([]string, 0, len(providerBreakdown))
	for p, count := range providerBreakdown {
		providersList = append(providersList, fmt.Sprintf("%s:%d", p, count))
	}
	slog.InfoContext(r.Context(), "[settings] Deleted accounts for institution", "count", len(accountsToDelete), "institution", institutionName, "providers", strings.Join(providersList, ", "), "transactions", totalDeleted)

	http.Redirect(w, r, "/settings/banks?deleted="+institutionName, http.StatusSeeOther)
}

// SettingsBilling shows the billing page with current subscriptions
func (hdl *Handlers) SettingsBilling(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ctx := r.Context()

	// If billing is disabled, show a simple message
	if !hdl.cfg.BillingEnabled {
		page := layouts.SettingsLayout("Billing", user.Email, "billing", user.ID.String(),
			shadcn.PageHeader("Billing", "Manage your subscriptions"),
			h.Div(
				h.Class("space-y-6"),
				shadcn.Card(shadcn.CardProps{},
					h.Div(
						h.Class("p-6 text-center text-muted-foreground"),
						g.Text("Billing is disabled. Set BILLING_ENABLED=true in your environment to enable."),
					),
				),
			),
		)
		renderHTML(w, page)
		return
	}

	subscriptionStore := models.NewSubscriptionStore(hdl.db.Pool)
	entitySubscriptionStore := models.NewEntitySubscriptionStore(hdl.db.Pool)

	// Get all subscriptions for user
	subscriptions, err := subscriptionStore.GetByUserID(ctx, user.ID)
	if err != nil {
		http.Error(w, "Failed to load subscriptions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get entities for each subscription and verify customer exists in Stripe
	type subscriptionWithEntities struct {
		Subscription *models.Subscription
		Entities     []*models.Entity
		IsOrphaned   bool // Customer doesn't exist in Stripe
	}
	var subsWithEntities []subscriptionWithEntities

	stripeClient := billing.NewStripeClient(hdl.cfg)

	for _, sub := range subscriptions {
		// Include all subscriptions, including canceled ones
		entitySubs, err := entitySubscriptionStore.GetBySubscriptionID(ctx, sub.ID)
		if err != nil {
			continue
		}

		var entities []*models.Entity
		for _, es := range entitySubs {
			entity, err := hdl.entities.GetByID(ctx, es.EntityID)
			if err == nil {
				entities = append(entities, entity)
			}
		}

		// Validate subscription against Stripe
		// Always check customer first, then subscription
		isOrphaned := false
		needsUpdate := false
		customerExists := true

		// First, always verify customer exists (if we have a customer ID)
		if sub.StripeCustomerID != nil && *sub.StripeCustomerID != "" {
			slog.InfoContext(r.Context(), "checking if customer exists in Stripe", "customer_id", *sub.StripeCustomerID)
			_, err := stripeClient.GetCustomer(ctx, *sub.StripeCustomerID)
			if err != nil {
				// Customer doesn't exist in Stripe - this is an orphaned subscription
				slog.WarnContext(r.Context(), "Customer not found in Stripe, marking as orphaned", "id", *sub.StripeCustomerID, "err", err)
				customerExists = false
				isOrphaned = true
				if sub.Status != models.SubscriptionStatusCanceled {
					slog.WarnContext(r.Context(), "customer no longer exists in Stripe, marking subscription as canceled", "customer_id", *sub.StripeCustomerID)
					sub.Status = models.SubscriptionStatusCanceled
					needsUpdate = true
				}
			} else {
				slog.InfoContext(r.Context(), "customer exists in Stripe", "customer_id", *sub.StripeCustomerID)
			}
		}

		// Then check subscription if customer exists and we have a subscription ID
		if customerExists && sub.StripeSubscriptionID != nil && *sub.StripeSubscriptionID != "" {
			stripeSub, err := stripeClient.GetSubscription(ctx, *sub.StripeSubscriptionID)
			if err != nil {
				// Subscription doesn't exist in Stripe anymore - mark as canceled
				if sub.Status != models.SubscriptionStatusCanceled {
					slog.InfoContext(r.Context(), "subscription no longer exists in Stripe, marking as canceled", "subscription_id", *sub.StripeSubscriptionID)
					sub.Status = models.SubscriptionStatusCanceled
					needsUpdate = true
				}
				isOrphaned = true
			} else {
				// Subscription exists - sync status from Stripe
				newStatus := billing.ParseSubscriptionStatus(string(stripeSub.Status))
				if sub.Status != newStatus {
					slog.InfoContext(r.Context(), "[billing] Syncing subscription status", "subscription_id", *sub.StripeSubscriptionID, "old_status", sub.Status, "new_status", newStatus)
					sub.Status = newStatus
					needsUpdate = true
				}
			}
		}

		// Update subscription in database if status changed
		if needsUpdate {
			if err := subscriptionStore.Update(ctx, sub); err != nil {
				slog.ErrorContext(r.Context(), "Failed to update subscription status", "err", err)
			}
		}

		subsWithEntities = append(subsWithEntities, subscriptionWithEntities{
			Subscription: sub,
			Entities:     entities,
			IsOrphaned:   isOrphaned,
		})
	}

	// Check if all entities have subscriptions to determine if we should show the Subscribe button
	// Get all entities the user owns
	userPerms, err := hdl.permissions.GetUserEntityPermissions(ctx, user.ID)
	if err != nil {
		http.Error(w, "Failed to load entities: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var userEntities []*models.Entity
	for _, perm := range userPerms {
		if perm.PermissionLevel == models.PermissionLevelOwner {
			entity, err := hdl.entities.GetByID(ctx, perm.EntityID)
			if err == nil {
				userEntities = append(userEntities, entity)
			}
		}
	}

	// Check which entities have active subscriptions
	entitiesWithSubscriptions := make(map[uuid.UUID]bool)
	for _, swe := range subsWithEntities {
		// Only count active subscriptions
		if swe.Subscription.Status == models.SubscriptionStatusActive {
			for _, entity := range swe.Entities {
				entitiesWithSubscriptions[entity.ID] = true
			}
		}
	}

	// Show subscribe button only if there are entities without active subscriptions
	showSubscribeButton := false
	for _, entity := range userEntities {
		if !entitiesWithSubscriptions[entity.ID] {
			showSubscribeButton = true
			break
		}
	}

	// Check for error query parameter
	errorParam := r.URL.Query().Get("error")
	showSubscriptionRequired := errorParam == "subscription_required"

	// Filter subscriptions to only show those with entities or orphaned ones
	var validSubsWithEntities []subscriptionWithEntities
	for _, swe := range subsWithEntities {
		if len(swe.Entities) > 0 || swe.IsOrphaned {
			validSubsWithEntities = append(validSubsWithEntities, swe)
		}
	}

	page := layouts.SettingsLayout("Billing", user.Email, "billing", user.ID.String(),
		shadcn.PageHeader("Billing", "Manage your subscriptions",
			g.If(showSubscribeButton,
				h.A(
					h.Href("/settings/billing/subscribe"),
					h.Class("inline-flex items-center gap-2 bg-primary text-primary-foreground rounded-lg px-4 py-2.5 text-sm font-medium hover:opacity-90 transition-colors"),
					layouts.IconPlus(),
					g.Text("Subscribe"),
				),
			),
		),

		h.Div(
			h.Class("space-y-6"),

			g.If(showSubscriptionRequired,
				shadcn.Card(shadcn.CardProps{Class: "bg-yellow-500/10 border-yellow-500/30"},
					shadcn.CardContentFull(
						h.Div(
							h.Class("flex items-start gap-3"),
							h.Div(h.Class("text-yellow-500 text-xl"), g.Text("⚠️")),
							h.Div(
								h.Class("flex-1"),
								h.P(h.Class("font-semibold text-foreground mb-1"), g.Text("Subscription Required")),
								h.P(h.Class("text-sm text-muted-foreground"),
									g.Text("You need an active subscription or trial to access the app. Start with a 45-day free trial to see a full month and really understand how game-changing Probably is."),
								),
							),
						),
					),
				),
			),

			g.If(len(validSubsWithEntities) == 0 && !showSubscriptionRequired,
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardContentFull(
						h.Div(h.Class("text-center text-muted-foreground"),
							g.Text("No subscriptions. Subscribe to get started."),
						),
					),
				),
			),

			g.If(len(validSubsWithEntities) > 0,
				g.Group(g.Map(validSubsWithEntities, func(swe subscriptionWithEntities) g.Node {
					return renderSubscriptionCard(swe.Subscription, swe.Entities, swe.IsOrphaned, hdl.cfg)
				})),
			),
		),
	)

	renderHTML(w, page)
}

// renderSubscriptionCard renders a subscription card
func renderSubscriptionCard(sub *models.Subscription, entities []*models.Entity, isOrphaned bool, cfg interface{}) g.Node {
	statusColor := "text-muted-foreground"
	statusBadge := "bg-secondary"
	switch sub.Status {
	case models.SubscriptionStatusActive:
		statusColor = "text-chart-2"
		statusBadge = "bg-chart-2/20"
	case models.SubscriptionStatusPastDue:
		statusColor = "text-ring"
		statusBadge = "bg-ring/20"
	case models.SubscriptionStatusCanceled:
		statusColor = "text-destructive"
		statusBadge = "bg-destructive/20"
	}

	planTypeDisplay := string(sub.PlanType)
	if sub.PlanType == models.PlanTypeMonthly {
		planTypeDisplay = "$9/month"
	} else if sub.PlanType == models.PlanTypeAnnual {
		planTypeDisplay = "$99/year"
	} else if sub.PlanType == models.PlanTypeBundle {
		planTypeDisplay = "Bundle"
	}

	periodText := "N/A"
	if sub.CurrentPeriodEnd != nil {
		periodText = sub.CurrentPeriodEnd.Format("Jan 2, 2006")
	} else if sub.Status == models.SubscriptionStatusCanceled {
		periodText = "Canceled"
	}

	return shadcn.Card(shadcn.CardProps{},
		h.Div(
			h.Class("p-6"),
			g.If(isOrphaned,
				h.Div(
					h.Class("mb-4 p-3 bg-ring/20 border border-ring/50 rounded-lg"),
					h.Div(
						h.Class("flex items-start justify-between gap-3"),
						h.Div(
							h.Class("flex-1"),
							h.P(
								h.Class("text-sm font-medium text-ring mb-1"),
								g.Text("Orphaned Subscription"),
							),
							h.P(
								h.Class("text-xs text-ring/80"),
								g.Text("This subscription's customer was deleted in Stripe. It cannot be managed."),
							),
						),
						h.Form(
							h.Method("POST"),
							h.Action(fmt.Sprintf("/settings/billing/cleanup?subscription_id=%s", sub.ID)),
							g.Attr("onsubmit", "return confirm('Are you sure you want to remove this orphaned subscription?');"),
							h.Button(
								h.Type("submit"),
								h.Class("text-xs px-3 py-1.5 bg-destructive hover:opacity-90 text-primary-foreground rounded transition-colors"),
								g.Text("Remove"),
							),
						),
					),
				),
			),
			h.Div(
				h.Class("flex items-start justify-between mb-4"),
				h.Div(
					h.Div(
						h.Class("flex items-center gap-3 mb-2 flex-wrap"),
						h.Span(
							h.Class("font-semibold text-foreground"),
							g.Text(planTypeDisplay),
						),
						h.Span(
							h.Class(fmt.Sprintf("px-2 py-1 rounded text-xs font-medium %s %s", statusBadge, statusColor)),
							g.Text(string(sub.Status)),
						),
						g.If(sub.CancelAtPeriodEnd && sub.Status == models.SubscriptionStatusActive,
							h.Span(
								h.Class("px-2 py-1 rounded text-xs font-medium bg-secondary text-muted-foreground flex items-center gap-1"),
								h.Span(g.Text("⏰")),
								g.Group([]g.Node{
									g.If(sub.CurrentPeriodEnd != nil,
										g.Text(fmt.Sprintf("Cancels %s", sub.CurrentPeriodEnd.Format("Jan 2, 2006"))),
									),
									g.If(sub.CurrentPeriodEnd == nil,
										g.Text("Cancels at period end"),
									),
								}),
							),
						),
					),
					h.P(
						h.Class("text-sm text-muted-foreground"),
						g.If(sub.Status == models.SubscriptionStatusCanceled,
							g.Group([]g.Node{
								g.Text("Canceled"),
								g.If(sub.CurrentPeriodEnd != nil,
									g.Text(fmt.Sprintf(" (ended %s)", sub.CurrentPeriodEnd.Format("Jan 2, 2006"))),
								),
							}),
						),
						g.If(sub.Status != models.SubscriptionStatusCanceled && sub.CancelAtPeriodEnd && sub.CurrentPeriodEnd != nil,
							g.Text(fmt.Sprintf("Active until %s, then canceled", sub.CurrentPeriodEnd.Format("Jan 2, 2006"))),
						),
						g.If(sub.Status != models.SubscriptionStatusCanceled && !sub.CancelAtPeriodEnd && sub.CurrentPeriodEnd != nil,
							g.Text(fmt.Sprintf("Renews on %s", periodText)),
						),
						g.If(sub.Status != models.SubscriptionStatusCanceled && sub.CurrentPeriodEnd == nil,
							g.Text("No renewal date"),
						),
					),
				),
				g.If(sub.StripeCustomerID != nil && !isOrphaned,
					h.A(
						h.Href(fmt.Sprintf("/settings/billing/portal?customer_id=%s", *sub.StripeCustomerID)),
						h.Class("text-sm text-primary hover:text-primary/80"),
						g.Text("Manage"),
					),
				),
			),
			g.If(len(entities) > 0,
				h.Div(
					h.Class("mt-4"),
					h.P(
						h.Class("text-sm text-muted-foreground mb-2"),
						g.Text("Entities:"),
					),
					h.Div(
						h.Class("flex flex-wrap gap-2"),
						g.Group(g.Map(entities, func(e *models.Entity) g.Node {
							return h.Span(
								h.Class("px-2 py-1 rounded bg-secondary text-sm text-foreground"),
								g.Text(e.Name),
							)
						})),
					),
				),
			),
		),
	)
}

// SettingsBillingSubscribe shows the subscription creation page
func (hdl *Handlers) SettingsBillingSubscribe(w http.ResponseWriter, r *http.Request) {
	// Only handle GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If billing is disabled, redirect to billing page
	if !hdl.cfg.BillingEnabled {
		http.Redirect(w, r, "/settings/billing", http.StatusSeeOther)
		return
	}

	user := auth.CurrentUser(r)
	ctx := r.Context()

	// Check trial eligibility
	subscriptionStore := models.NewSubscriptionStore(hdl.db.Pool)
	hasHadTrial, err := subscriptionStore.HasEverHadTrial(ctx, user.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to check trial eligibility", "err", err)
		hasHadTrial = false // Default to allowing trial if check fails
	}

	// Get all entities user owns (for subscription)
	userPerms, err := hdl.permissions.GetUserEntityPermissions(ctx, user.ID)
	if err != nil {
		http.Error(w, "Failed to load entities: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var availableEntities []*models.Entity
	for _, perm := range userPerms {
		entity, err := hdl.entities.GetByID(ctx, perm.EntityID)
		if err == nil && perm.PermissionLevel == models.PermissionLevelOwner {
			availableEntities = append(availableEntities, entity)
		}
	}

	// Check if bundle is configured
	bundleAvailable := hdl.cfg.StripePriceBundle != ""

	// JavaScript for subscription form
	subscriptionFormJS := `function updatePlanSelection(planType) { document.querySelectorAll('[onclick*="plan_"]').forEach(el => { el.classList.remove('border-primary', 'bg-primary/10'); el.classList.add('border-border'); }); const selectedEl = document.getElementById('plan_' + planType).closest('div'); if (selectedEl) { selectedEl.classList.add('border-primary', 'bg-primary/10'); selectedEl.classList.remove('border-border'); } } function validateSubscriptionForm() { const planType = document.querySelector('input[name="plan_type"]:checked'); if (!planType) { alert('Please select a plan'); return false; } const entityIds = document.querySelectorAll('input[name="entity_ids"]:checked'); if (entityIds.length === 0) { alert('Please select at least one entity'); return false; } return true; } document.addEventListener('DOMContentLoaded', function() { updatePlanSelection('monthly'); });`

	// Validate Stripe configuration
	var stripeConfigError string
	if hdl.cfg.StripeSecretKey == "" {
		stripeConfigError = "Stripe secret key not configured"
	} else if hdl.cfg.StripePriceMonthly == "" {
		stripeConfigError = "Stripe monthly price ID not configured"
	} else if hdl.cfg.StripePriceAnnual == "" {
		stripeConfigError = "Stripe annual price ID not configured"
	} else if !strings.HasPrefix(hdl.cfg.StripePriceMonthly, "price_") {
		stripeConfigError = fmt.Sprintf("Invalid monthly price ID format. Expected 'price_...', got '%s'. Make sure you're using a Price ID, not a Product ID.", hdl.cfg.StripePriceMonthly)
	} else if !strings.HasPrefix(hdl.cfg.StripePriceAnnual, "price_") {
		stripeConfigError = fmt.Sprintf("Invalid annual price ID format. Expected 'price_...', got '%s'. Make sure you're using a Price ID, not a Product ID.", hdl.cfg.StripePriceAnnual)
	}

	page := layouts.SettingsLayout("Subscribe", user.Email, "billing", user.ID.String(),
		shadcn.PageHeader("Subscribe", "Choose a plan and entities"),

		g.If(stripeConfigError != "",
			shadcn.Card(shadcn.CardProps{Class: "bg-ring/20 border-ring/30"},
				shadcn.CardContentFull(
					h.Div(
						h.Class("flex items-start gap-3"),
						h.Div(h.Class("text-ring text-xl"), g.Text("⚠️")),
						h.Div(
							h.Class("flex-1"),
							h.P(h.Class("font-semibold text-ring mb-1"), g.Text("Stripe Configuration Error")),
							h.P(h.Class("text-sm text-ring/80"), g.Text(stripeConfigError)),
							h.P(h.Class("text-xs text-ring/80 mt-2"),
								g.Text("To fix: Go to Stripe Dashboard → Products → Click your product → Copy the Price ID (starts with 'price_', not 'prod_') → Update your .env file"),
							),
						),
					),
				),
			),
		),

		g.If(len(availableEntities) > 0 && stripeConfigError == "",
			h.Div(
				h.Class("space-y-6"),
				// Trial eligibility message
				g.If(!hasHadTrial,
					shadcn.Card(shadcn.CardProps{Class: "bg-primary/10 border-primary/30"},
						shadcn.CardContentFull(
							h.Div(
								h.Class("flex items-start gap-3"),
								h.Div(h.Class("text-primary text-xl"), g.Text("🎉")),
								h.Div(
									h.Class("flex-1"),
									h.P(h.Class("font-semibold text-primary mb-1"), g.Text("45-Day Free Trial Available")),
									h.P(h.Class("text-sm text-primary/80"),
										g.Text("Start with a 45-day free trial. We want you to see a full month to really understand how game-changing Probably is."),
									),
								),
							),
						),
					),
				),
				g.If(hasHadTrial,
					shadcn.Card(shadcn.CardProps{Class: "bg-secondary"},
						shadcn.CardContentFull(
							h.Div(
								h.Class("flex items-start gap-3"),
								h.Div(h.Class("text-muted-foreground text-xl"), g.Text("ℹ️")),
								h.Div(
									h.Class("flex-1"),
									h.P(h.Class("font-semibold text-muted-foreground mb-1"), g.Text("Trial Already Used")),
									h.P(h.Class("text-sm text-foreground"),
										g.Text("You've already used your free trial. Please choose a paid plan to continue."),
									),
								),
							),
						),
					),
				),
				h.Form(
					h.Method("POST"),
					h.Action("/settings/billing/subscribe"),
					g.Attr("onsubmit", "return validateSubscriptionForm()"),
					h.Div(
						h.Class("space-y-6"),

						// Plan selection
						shadcn.Card(shadcn.CardProps{},
							shadcn.CardHeaderActions("Select Plan", ""),
							shadcn.CardContentFull(
								h.Div(
									h.Class("grid grid-cols-1 md:grid-cols-3 gap-4"),
									// Monthly plan
									h.Div(
										h.Class("border border-border rounded-lg p-4 hover:border-primary transition-colors cursor-pointer"),
										g.Attr("onclick", "document.getElementById('plan_monthly').checked = true; updatePlanSelection('monthly')"),
										h.Input(
											h.Type("radio"),
											h.ID("plan_monthly"),
											h.Name("plan_type"),
											h.Value("monthly"),
											h.Checked(),
											h.Required(),
											h.Class("mb-2"),
										),
										h.P(h.Class("font-semibold text-foreground"), g.Text("Monthly")),
										h.P(h.Class("text-2xl font-bold text-foreground mt-2"), g.Text("$9")),
										h.P(h.Class("text-sm text-muted-foreground"), g.Text("per month")),
									),
									// Annual plan
									h.Div(
										h.Class("border border-border rounded-lg p-4 hover:border-primary transition-colors cursor-pointer"),
										g.Attr("onclick", "document.getElementById('plan_annual').checked = true; updatePlanSelection('annual')"),
										h.Input(
											h.Type("radio"),
											h.ID("plan_annual"),
											h.Name("plan_type"),
											h.Value("annual"),
											h.Required(),
											h.Class("mb-2"),
										),
										h.P(h.Class("font-semibold text-foreground"), g.Text("Annual")),
										h.P(h.Class("text-2xl font-bold text-foreground mt-2"), g.Text("$99")),
										h.P(h.Class("text-sm text-muted-foreground"), g.Text("per year")),
									),
									// Bundle plan (if configured)
									g.If(bundleAvailable,
										h.Div(
											h.Class("border border-border rounded-lg p-4 hover:border-primary transition-colors cursor-pointer"),
											g.Attr("onclick", "document.getElementById('plan_bundle').checked = true; updatePlanSelection('bundle')"),
											h.Input(
												h.Type("radio"),
												h.ID("plan_bundle"),
												h.Name("plan_type"),
												h.Value("bundle"),
												h.Required(),
												h.Class("mb-2"),
											),
											h.P(h.Class("font-semibold text-foreground"), g.Text("Bundle")),
											h.P(h.Class("text-sm text-muted-foreground mt-2"), g.Text("Multiple entities")),
										),
									),
								),
							),
						),

						// Entity selection
						shadcn.Card(shadcn.CardProps{},
							shadcn.CardHeaderActions("Select Entities", "Choose which entities to subscribe for"),
							shadcn.CardContentFull(
								h.Div(
									h.Class("space-y-2"),
									g.Group(g.Map(availableEntities, func(e *models.Entity) g.Node {
										return h.Div(
											h.Class("flex items-center gap-3 p-3 rounded-lg border border-border hover:border-primary/50"),
											h.Input(
												h.Type("checkbox"),
												h.Name("entity_ids"),
												h.Value(e.ID.String()),
												h.Class("rounded"),
											),
											h.Label(
												h.Class("flex-1 cursor-pointer"),
												h.Span(h.Class("font-medium text-foreground"), g.Text(e.Name)),
												h.Span(h.Class("ml-2 text-sm text-muted-foreground"), g.Text(string(e.Type))),
											),
										)
									})),
								),
							),
						),

						// Subscribe button
						h.Div(
							h.Class("flex justify-end"),
							h.Button(
								h.Type("submit"),
								h.Class("bg-primary text-primary-foreground rounded-lg px-6 py-2.5 text-sm font-medium hover:opacity-90 transition-colors"),
								g.Text("Continue to Checkout"),
							),
						),
					),
				),
			),
		),
		g.If(len(availableEntities) == 0,
			shadcn.Card(shadcn.CardProps{},
				h.Div(
					h.Class("p-6 text-center text-muted-foreground"),
					g.Text("No entities available. Create an entity first."),
				),
			),
		),
		h.Script(g.Raw(subscriptionFormJS)),
	)

	renderHTML(w, page)
}

// SettingsBillingSubscribePost handles subscription creation
func (hdl *Handlers) SettingsBillingSubscribePost(w http.ResponseWriter, r *http.Request) {
	// Only handle POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If billing is disabled, redirect to billing page
	if !hdl.cfg.BillingEnabled {
		http.Redirect(w, r, "/settings/billing", http.StatusSeeOther)
		return
	}

	user := auth.CurrentUser(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	planTypeStr := r.FormValue("plan_type")
	if planTypeStr == "" {
		// Better error message with debugging
		slog.WarnContext(r.Context(), "billing subscribe missing plan_type", "method", r.Method, "url", r.URL.String())
		http.Error(w, "Plan type is required. Please select a plan (Monthly, Annual, or Bundle).", http.StatusBadRequest)
		return
	}

	planType := models.PlanType(planTypeStr)
	if planType != models.PlanTypeMonthly && planType != models.PlanTypeAnnual && planType != models.PlanTypeBundle {
		http.Error(w, "Invalid plan type", http.StatusBadRequest)
		return
	}

	// Get entity IDs
	entityIDsStr := r.Form["entity_ids"]
	if len(entityIDsStr) == 0 {
		http.Error(w, "At least one entity is required", http.StatusBadRequest)
		return
	}

	var entityIDs []uuid.UUID
	for _, eidStr := range entityIDsStr {
		eid, err := uuid.Parse(eidStr)
		if err != nil {
			continue
		}
		entityIDs = append(entityIDs, eid)
	}

	if len(entityIDs) == 0 {
		http.Error(w, "Invalid entity IDs", http.StatusBadRequest)
		return
	}

	// Get or create Stripe customer
	subscriptionStore := models.NewSubscriptionStore(hdl.db.Pool)
	stripeClient := billing.NewStripeClient(hdl.cfg)

	// First check if user already has a customer ID in our database
	existingSubs, _ := subscriptionStore.GetByUserID(ctx, user.ID)
	var customerID string
	for _, sub := range existingSubs {
		if sub.StripeCustomerID != nil {
			customerID = *sub.StripeCustomerID
			break
		}
	}

	// If not in database, search Stripe for existing customer by email
	// This prevents creating duplicate customers
	if customerID == "" {
		var err error
		customerID, err = stripeClient.GetOrCreateCustomer(ctx, user)
		if err != nil {
			http.Error(w, "Failed to get or create customer: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Check if user is eligible for trial
	hasHadTrial, err := subscriptionStore.HasEverHadTrial(ctx, user.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to check trial eligibility", "err", err)
		// Continue without trial if check fails
		hasHadTrial = true
	}

	// Create checkout session
	baseURL := hdl.cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	successURL := baseURL + "/settings/billing/callback?session_id={CHECKOUT_SESSION_ID}"
	cancelURL := baseURL + "/settings/billing"

	var checkoutURL string
	if !hasHadTrial {
		// Create checkout session with 45-day trial
		checkoutURL, err = stripeClient.CreateCheckoutSessionWithTrial(ctx, user.ID, customerID, entityIDs, planType, successURL, cancelURL, 45)
		if err != nil {
			http.Error(w, "Failed to create checkout session with trial: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Create regular checkout session (no trial)
		checkoutURL, err = stripeClient.CreateCheckoutSession(ctx, user.ID, customerID, entityIDs, planType, successURL, cancelURL)
		if err != nil {
			http.Error(w, "Failed to create checkout session: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Redirect to Stripe Checkout
	http.Redirect(w, r, checkoutURL, http.StatusSeeOther)
}

// SettingsBillingCallback handles the return from Stripe Checkout
func (hdl *Handlers) SettingsBillingCallback(w http.ResponseWriter, r *http.Request) {
	// If billing is disabled, redirect to billing page
	if !hdl.cfg.BillingEnabled {
		http.Redirect(w, r, "/settings/billing", http.StatusSeeOther)
		return
	}

	user := auth.CurrentUser(r)
	ctx := r.Context()

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Redirect(w, r, "/settings/billing?error=no_session", http.StatusSeeOther)
		return
	}

	// Try to sync subscription immediately (in case webhook hasn't arrived yet)
	stripeClient := billing.NewStripeClient(hdl.cfg)
	subscriptionStore := models.NewSubscriptionStore(hdl.db.Pool)
	entitySubscriptionStore := models.NewEntitySubscriptionStore(hdl.db.Pool)

	// Get checkout session from Stripe
	sess, err := stripeClient.GetCheckoutSession(ctx, sessionID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get checkout session", "err", err)
		// Still redirect - webhook might handle it
		http.Redirect(w, r, "/settings/billing?success=1&note=webhook_pending", http.StatusSeeOther)
		return
	}

	// Check if session is completed
	if sess.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		slog.InfoContext(r.Context(), "session not paid yet", "payment_status", sess.PaymentStatus)
		http.Redirect(w, r, "/settings/billing?error=payment_pending", http.StatusSeeOther)
		return
	}

	// Check if subscription already exists (webhook already processed it)
	if sess.Subscription != nil {
		existing, err := subscriptionStore.GetByStripeSubscriptionID(ctx, sess.Subscription.ID)
		if err == nil && existing != nil {
			// Already exists, just redirect
			http.Redirect(w, r, "/settings/billing?success=1&note=subscription_synced", http.StatusSeeOther)
			return
		}

		// Subscription doesn't exist yet - create it manually
		// This handles the case where webhook hasn't arrived yet
		userIDStr, ok := sess.Metadata["user_id"]
		if !ok || userIDStr != user.ID.String() {
			slog.WarnContext(r.Context(), "User ID mismatch or missing in session metadata")
			http.Redirect(w, r, "/settings/billing?error=invalid_session", http.StatusSeeOther)
			return
		}

		// Get plan type from metadata
		planTypeStr, ok := sess.Metadata["plan_type"]
		if !ok {
			slog.WarnContext(r.Context(), "No plan_type in metadata")
			http.Redirect(w, r, "/settings/billing?error=invalid_session", http.StatusSeeOther)
			return
		}
		planType := models.PlanType(planTypeStr)

		// Get entity IDs from metadata
		entityIDsStr := sess.Metadata["entity_ids"]
		var entityIDs []uuid.UUID
		if entityIDsStr != "" {
			for _, eidStr := range strings.Split(entityIDsStr, ",") {
				if eid, err := uuid.Parse(strings.TrimSpace(eidStr)); err == nil {
					entityIDs = append(entityIDs, eid)
				}
			}
		}

		// Fetch subscription from Stripe
		stripeSub, err := stripeClient.GetSubscription(ctx, sess.Subscription.ID)
		if err != nil {
			slog.ErrorContext(r.Context(), "Failed to get subscription", "err", err)
			http.Redirect(w, r, "/settings/billing?success=1&note=webhook_pending", http.StatusSeeOther)
			return
		}

		// Create subscription in database
		sub := &models.Subscription{
			UserID:               user.ID,
			StripeSubscriptionID: &stripeSub.ID,
			StripeCustomerID:     &stripeSub.Customer.ID,
			Status:               billing.ParseSubscriptionStatus(string(stripeSub.Status)),
			PlanType:             planType,
			CancelAtPeriodEnd:    stripeSub.CancelAtPeriodEnd,
		}

		if stripeSub.CurrentPeriodStart > 0 {
			periodStart := time.Unix(stripeSub.CurrentPeriodStart, 0)
			sub.CurrentPeriodStart = &periodStart
		}
		if stripeSub.CurrentPeriodEnd > 0 {
			periodEnd := time.Unix(stripeSub.CurrentPeriodEnd, 0)
			sub.CurrentPeriodEnd = &periodEnd
		}

		if err := subscriptionStore.Create(ctx, sub); err != nil {
			slog.ErrorContext(r.Context(), "Failed to create subscription", "err", err)
			// Still redirect - webhook will handle it
			http.Redirect(w, r, "/settings/billing?success=1&note=webhook_pending", http.StatusSeeOther)
			return
		}

		// Link entities to subscription
		for _, entityID := range entityIDs {
			es := &models.EntitySubscription{
				SubscriptionID: sub.ID,
				EntityID:       entityID,
			}
			if err := entitySubscriptionStore.Create(ctx, es); err != nil {
				slog.ErrorContext(r.Context(), "Failed to link entity", "err", err)
			}
		}

		slog.InfoContext(r.Context(), "Successfully synced subscription  for user", "id", sub.ID, "id", user.ID)
	}

	http.Redirect(w, r, "/settings/billing?success=1&note=subscription_synced", http.StatusSeeOther)
}

// SettingsBillingPortal creates a Stripe Customer Portal session and redirects
func (hdl *Handlers) SettingsBillingPortal(w http.ResponseWriter, r *http.Request) {
	// If billing is disabled, redirect to billing page
	if !hdl.cfg.BillingEnabled {
		http.Redirect(w, r, "/settings/billing", http.StatusSeeOther)
		return
	}

	user := auth.CurrentUser(r)
	ctx := r.Context()

	customerID := r.URL.Query().Get("customer_id")
	if customerID == "" {
		http.Error(w, "Customer ID is required", http.StatusBadRequest)
		return
	}

	// Verify the customer belongs to this user
	subscriptionStore := models.NewSubscriptionStore(hdl.db.Pool)
	subscriptions, err := subscriptionStore.GetByUserID(ctx, user.ID)
	if err != nil {
		http.Error(w, "Failed to verify customer: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if customer ID belongs to user
	validCustomer := false
	for _, sub := range subscriptions {
		if sub.StripeCustomerID != nil && *sub.StripeCustomerID == customerID {
			validCustomer = true
			break
		}
	}

	if !validCustomer {
		http.Error(w, "Customer not found", http.StatusNotFound)
		return
	}

	// Verify customer exists in Stripe before creating portal session
	stripeClient := billing.NewStripeClient(hdl.cfg)
	_, err = stripeClient.GetCustomer(ctx, customerID)
	if err != nil {
		// Customer doesn't exist in Stripe - try to get or create a new one
		slog.WarnContext(r.Context(), "Customer not found in Stripe, attempting to get or create", "id", customerID, "err", err)

		// Get or create customer for this user
		newCustomerID, createErr := stripeClient.GetOrCreateCustomer(ctx, user)
		if createErr != nil {
			slog.ErrorContext(r.Context(), "Failed to get or create customer", "err", createErr)
			http.Redirect(w, r, "/settings/billing?error=customer_not_found", http.StatusSeeOther)
			return
		}

		// Update the subscription with the new customer ID
		for _, sub := range subscriptions {
			if sub.StripeCustomerID != nil && *sub.StripeCustomerID == customerID {
				sub.StripeCustomerID = &newCustomerID
				if updateErr := subscriptionStore.Update(ctx, sub); updateErr != nil {
					slog.ErrorContext(r.Context(), "Failed to update subscription with new customer ID", "err", updateErr)
				}
				break
			}
		}

		customerID = newCustomerID
	}

	// Create portal session
	baseURL := hdl.cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	returnURL := baseURL + "/settings/billing"

	portalURL, err := stripeClient.CreatePortalSession(ctx, customerID, returnURL)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to create portal session for customer", "id", customerID, "err", err)
		// Check if it's a specific Stripe error we can handle
		if stripeErr, ok := err.(*stripe.Error); ok {
			slog.ErrorContext(r.Context(), "[billing-portal] Stripe error", "type", stripeErr.Type, "code", stripeErr.Code, "message", stripeErr.Msg)
		}
		http.Redirect(w, r, "/settings/billing?error=portal_failed", http.StatusSeeOther)
		return
	}

	// Redirect to Stripe Customer Portal
	http.Redirect(w, r, portalURL, http.StatusSeeOther)
}

// SettingsBillingCleanup removes an orphaned subscription
func (hdl *Handlers) SettingsBillingCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If billing is disabled, redirect to billing page
	if !hdl.cfg.BillingEnabled {
		http.Redirect(w, r, "/settings/billing", http.StatusSeeOther)
		return
	}

	user := auth.CurrentUser(r)
	ctx := r.Context()

	subscriptionIDStr := r.URL.Query().Get("subscription_id")
	if subscriptionIDStr == "" {
		http.Error(w, "Subscription ID is required", http.StatusBadRequest)
		return
	}

	subscriptionID, err := uuid.Parse(subscriptionIDStr)
	if err != nil {
		http.Error(w, "Invalid subscription ID", http.StatusBadRequest)
		return
	}

	// Verify subscription belongs to user
	subscriptionStore := models.NewSubscriptionStore(hdl.db.Pool)
	sub, err := subscriptionStore.GetByID(ctx, subscriptionID)
	if err != nil {
		http.Error(w, "Subscription not found", http.StatusNotFound)
		return
	}

	if sub.UserID != user.ID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	// Verify it's actually orphaned (customer doesn't exist)
	stripeClient := billing.NewStripeClient(hdl.cfg)
	isOrphaned := false
	if sub.StripeCustomerID != nil && *sub.StripeCustomerID != "" {
		_, err := stripeClient.GetCustomer(ctx, *sub.StripeCustomerID)
		if err != nil {
			isOrphaned = true
		}
	}

	if !isOrphaned {
		http.Redirect(w, r, "/settings/billing?error=not_orphaned", http.StatusSeeOther)
		return
	}

	// Delete entity subscriptions first
	entitySubscriptionStore := models.NewEntitySubscriptionStore(hdl.db.Pool)
	if err := entitySubscriptionStore.DeleteBySubscriptionID(ctx, subscriptionID); err != nil {
		slog.ErrorContext(r.Context(), "Failed to delete entity subscriptions", "err", err)
	}

	// Delete the subscription
	if err := subscriptionStore.Delete(ctx, subscriptionID); err != nil {
		slog.ErrorContext(r.Context(), "Failed to delete subscription", "err", err)
		http.Redirect(w, r, "/settings/billing?error=cleanup_failed", http.StatusSeeOther)
		return
	}

	slog.InfoContext(r.Context(), "Successfully removed orphaned subscription  for user", "id", subscriptionID, "id", user.ID)
	http.Redirect(w, r, "/settings/billing?success=1&note=orphaned_removed", http.StatusSeeOther)
}
