package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func (hdl *Handlers) AccountsList(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	accounts, err := hdl.accounts.GetWithBalances(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Group by type
	grouped := make(map[models.AccountType][]*models.Account)
	for _, acc := range accounts {
		grouped[acc.Type] = append(grouped[acc.Type], acc)
	}

	page := layouts.AppLayout("Accounts", user.Email, user.ID.String(),
		shadcn.PageHeader("Accounts", "Manage your financial accounts",
			h.A(
				h.Href("/accounts/new"),
				shadcn.ButtonAnchor(shadcn.ButtonProps{Variant: shadcn.ButtonDefault},
					"/accounts/new",
					layouts.IconPlus(),
					g.Text("Add Account"),
				),
			),
		),

		// Account groups
		renderAccountGroup("Assets", models.AccountTypeAsset, grouped[models.AccountTypeAsset]),
		renderAccountGroup("Liabilities", models.AccountTypeLiability, grouped[models.AccountTypeLiability]),
		renderAccountGroup("Income", models.AccountTypeIncome, grouped[models.AccountTypeIncome]),
		renderAccountGroup("Expenses", models.AccountTypeExpense, grouped[models.AccountTypeExpense]),
	)

	renderHTML(w, page)
}

func renderAccountGroup(title string, accountType models.AccountType, accounts []*models.Account) g.Node {
	if len(accounts) == 0 {
		return nil
	}

	return h.Div(
		h.Class("mb-6"),
		h.H3(h.Class("text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3"), g.Text(title)),
		shadcn.Card(shadcn.CardProps{},
			h.Div(
				h.Class("divide-y divide-border"),
				g.Group(g.Map(accounts, func(acc *models.Account) g.Node {
					return h.A(
						h.Href("/accounts/"+acc.ID.String()),
						h.Class("flex items-center justify-between p-4 hover:bg-accent transition-colors"),
						h.Div(
							h.Class("flex items-center gap-3"),
							h.Div(
								h.Class("w-10 h-10 rounded-lg bg-secondary flex items-center justify-center text-muted-foreground"),
								g.If(acc.Provider != "" || acc.ExternalAccountID != "" || acc.TellerAccountID != "", layouts.IconBank()),
								g.If(acc.Provider == "" && acc.ExternalAccountID == "" && acc.TellerAccountID == "", layouts.IconWallet()),
							),
							h.Div(
								h.P(h.Class("font-medium text-foreground"), g.Text(acc.Name)),
								g.If(acc.InstitutionName != "",
									h.P(h.Class("text-sm text-muted-foreground"), g.Text(acc.InstitutionName)),
								),
							),
						),
						h.Div(
							h.Class("text-right"),
							h.P(
								h.Class("font-number font-medium "+amountColorClass(acc.Balance, acc.Type)),
								g.Text(displayBalance(acc.Balance, acc.Type)),
							),
						),
					)
				})),
			),
		),
	)
}

func (hdl *Handlers) AccountsNew(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)

	// Check if any provider is configured
	tellerConfigured := hdl.cfg.TellerAppID != ""
	plaidConfigured := hdl.cfg.PlaidClientID != "" && hdl.cfg.PlaidSecret() != ""
	akahuConfigured := hdl.cfg.AkahuAppID != "" && hdl.cfg.AkahuAppSecret != ""
	anyProviderConfigured := tellerConfigured || plaidConfigured || akahuConfigured

	page := layouts.AppLayout("New Account", user.Email, user.ID.String(),
		shadcn.PageHeader("New Account", "Choose how to add your account"),

		h.Div(
			h.Class("grid gap-4 md:grid-cols-2 max-w-2xl"),

			// Connect Bank option (primary)
			// Provider selection logic matches settings.go exactly:
			// 1. If only Akahu configured, link to Akahu
			// 2. If Plaid configured, link to Plaid (preferred)
			// 3. Otherwise, fallback to Teller
			g.If(anyProviderConfigured,
				func() g.Node {
					var connectURL string
					// Match settings.go logic exactly (lines 652-678)
					if akahuConfigured && !plaidConfigured && !tellerConfigured {
						// Only Akahu configured - use Akahu
						connectURL = "/connections/connect/akahu"
					} else if plaidConfigured {
						// Plaid configured (preferred when available) - use Plaid
						connectURL = "/connections/connect/plaid"
					} else {
						// Fallback to Teller if Plaid not configured
						connectURL = "/connections/connect/teller"
					}

					return h.A(
						h.Href(connectURL),
						h.Class("group block"),
						shadcn.Card(shadcn.CardProps{},
							shadcn.CardContentFull(
								h.Div(h.Class("text-center"),
									h.Div(
										h.Class("w-12 h-12 rounded-full bg-primary/20 flex items-center justify-center mx-auto mb-4"),
										layouts.IconBank(),
									),
									h.H3(h.Class("text-lg font-medium text-foreground mb-2"), g.Text("Connect Bank")),
									h.P(h.Class("text-sm text-muted-foreground"), g.Text("Securely link your bank account to automatically sync transactions")),
									h.Div(
										h.Class("mt-4 inline-flex items-center gap-2 text-primary text-sm font-medium group-hover:opacity-80"),
										g.Text("Connect Bank Account"),
										layouts.IconArrowRight(),
									),
								),
							),
						),
					)
				}(),
			),

			// Manual entry option
			h.A(
				h.Href("/accounts/new/manual"),
				h.Class("group block"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardContentFull(
						h.Div(h.Class("text-center"),
							h.Div(
								h.Class("w-12 h-12 rounded-full bg-secondary flex items-center justify-center mx-auto mb-4"),
								layouts.IconWallet(),
							),
							h.H3(h.Class("text-lg font-medium text-foreground mb-2"), g.Text("Add Manually")),
							h.P(h.Class("text-sm text-muted-foreground"), g.Text("Create a manual account for tracking balances and transactions")),
							h.Div(
								h.Class("mt-4 inline-flex items-center gap-2 text-muted-foreground text-sm font-medium group-hover:text-foreground"),
								g.Text("Create account"),
								layouts.IconArrowRight(),
							),
						),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

func (hdl *Handlers) AccountsNewManual(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)

	page := layouts.AppLayout("New Manual Account", user.Email, user.ID.String(),
		shadcn.PageHeader("New Manual Account", "Create a manual account for tracking"),
		renderAccountForm(nil, "/accounts", "POST"),
	)

	renderHTML(w, page)
}

func (hdl *Handlers) AccountsCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	account := &models.Account{
		LedgerID:        ledger.ID,
		Name:            r.FormValue("name"),
		Type:            models.AccountType(r.FormValue("type")),
		InstitutionName: r.FormValue("institution_name"),
		IsActive:        true,
	}

	if err := hdl.accounts.Create(r.Context(), account); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/accounts/"+account.ID.String(), http.StatusSeeOther)
}

func (hdl *Handlers) AccountsShow(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	accountID, ok := mustParamUUID(w, r, "id", "account ID")
	if !ok {
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Get transactions for this account (increased limit to show more transactions)
	transactions, total, err := hdl.transactions.List(r.Context(), models.TransactionFilter{
		LedgerID:  account.LedgerID,
		AccountID: &accountID,
		Limit:     200, // Increased from 50 to show more transactions
	})
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

	// Get the actual balance from the database (sum of ALL entries, not just displayed ones)
	balance, err := hdl.accounts.GetBalance(r.Context(), accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if any provider is configured
	tellerConfigured := hdl.cfg.TellerAppID != ""
	plaidConfigured := hdl.cfg.PlaidClientID != "" && hdl.cfg.PlaidSecret() != ""
	akahuConfigured := hdl.cfg.AkahuAppID != "" && hdl.cfg.AkahuAppSecret != ""
	anyProviderConfigured := tellerConfigured || plaidConfigured || akahuConfigured
	isConnected := account.Provider != "" || account.ExternalAccountID != "" || account.TellerAccountID != ""
	// Provider detection: Only check for Teller if Provider is explicitly "teller" or if Provider is empty but TellerAccountID is set
	// This prevents false positives when accounts have leftover TellerAccountID from migrations
	isConnectedToTeller := account.Provider == "teller" || (account.Provider == "" && account.TellerAccountID != "")
	isConnectedToPlaid := account.Provider == "plaid" && account.ConnectionID != ""
	isConnectedToAkahu := account.Provider == "akahu" && account.ConnectionID != ""
	isConnectedToAnyProvider := isConnectedToTeller || isConnectedToPlaid || isConnectedToAkahu

	page := layouts.AppLayout(account.Name, user.Email, user.ID.String(),
		// Update mode banner (if needed). Show for all providers when disconnected,
		// or for Plaid accounts with any non-empty connection status.
		g.If(account.ConnectionStatus != "" && (account.Provider == "plaid" || account.ConnectionStatus == "disconnected"),
			func() g.Node {
				var message, buttonText string
				var variant shadcn.AlertVariant = shadcn.AlertWarning

				switch account.ConnectionStatus {
				case "disconnected":
					message = "This account has been disconnected from your bank. Please reconnect to continue syncing."
					buttonText = "Reconnect Now"
					variant = shadcn.AlertDestructive
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

				var reconnectURL string
				switch account.Provider {
				case "teller":
					reconnectURL = "/connections/reconnect/teller?account_id=" + account.ID.String()
				default: // plaid and others fall back to plaid reconnect
					reconnectURL = "/connections/reconnect/plaid?connection_id=" + account.ConnectionID
					if account.ConnectionStatus == "new_accounts_available" {
						reconnectURL += "&account_selection_enabled=true"
					}
				}

				return h.Div(
					h.Class("mb-6"),
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
									g.Text("Action Required"),
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
		shadcn.PageHeader(account.Name, string(account.Type),
			h.A(
				h.Href("/accounts/"+account.ID.String()+"/edit"),
				shadcn.ButtonAnchor(shadcn.ButtonProps{Variant: shadcn.ButtonSecondary},
					"/accounts/"+account.ID.String()+"/edit",
					layouts.IconEdit(),
					g.Text("Edit"),
				),
			),
		),

		// Balance card with connection status
		h.Div(
			h.Class("mb-6"),
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					h.Div(
						h.Class("flex items-start justify-between"),
						h.Div(
							h.P(h.Class("text-sm text-muted-foreground"), g.Text("Current Balance")),
							h.P(
								h.Class("text-3xl font-bold font-number mt-1 "+amountColorClass(balance, account.Type)),
								g.Text(displayBalance(balance, account.Type)),
							),
							g.If(account.InstitutionName != "",
								h.P(h.Class("text-sm text-muted-foreground mt-2"), g.Text(account.InstitutionName)),
							),
						),
						// Connection status badge
						g.If(isConnectedToAnyProvider,
							shadcn.Badge(shadcn.BadgeProps{Variant: shadcn.BadgeSuccess}, layouts.IconLink(), g.Text("Connected")),
						),
					),
				),
			),
		),

		// Statement Upload Card (for accounts not connected to any provider)
		g.If(!isConnected,
			h.Div(
				h.Class("mb-6"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardHeaderActions("Manual Transaction Import", "Upload bank statements to add transactions"),
					h.Div(
						h.Class("p-4"),
						h.P(
							h.Class("text-sm text-muted-foreground mb-4"),
							g.Text("This account is not connected to a bank provider. You can manually add transactions by uploading bank statements (PDF or images)."),
						),
						h.A(
							h.Href("/accounts/"+account.ID.String()+"/statements/upload"),
							shadcn.ButtonAnchor(shadcn.ButtonProps{Variant: shadcn.ButtonDefault},
								"/accounts/"+account.ID.String()+"/statements/upload",
								layouts.IconUpload(),
								g.Text("Upload Statement"),
							),
						),
					),
				),
			),
		),

		// Bank Connection Card
		g.If(anyProviderConfigured,
			h.Div(
				h.Class("mb-6"),
				shadcn.Card(shadcn.CardProps{},
					shadcn.CardContentFull(
						g.If(isConnectedToAnyProvider,
							// Connected state - show sync and disconnect options
							h.Div(
								h.Class("flex items-center justify-between"),
								h.Div(
									h.Class("flex items-center gap-3"),
									h.Div(
										h.Class("w-10 h-10 rounded-lg bg-chart-2/10 flex items-center justify-center text-chart-2"),
										layouts.IconBank(),
									),
									h.Div(
										h.P(h.Class("font-medium text-foreground"), g.Text("Bank Connected")),
										h.P(h.Class("text-sm text-muted-foreground"), g.Text("Transactions sync automatically")),
									),
								),
								h.Div(
									h.Class("flex items-center gap-2"),
									// Sync button - provider-aware
									g.If(isConnectedToTeller,
										h.Form(
											h.Method("POST"),
											h.Action("/teller/sync/"+account.ConnectionID),
											shadcn.Button(shadcn.ButtonProps{
												Variant: shadcn.ButtonSecondary,
												Type:    "submit",
											},
												layouts.IconRefresh(),
												g.Text("Sync Now"),
											),
										),
									),
									g.If(isConnectedToPlaid,
										h.Div(
											h.Class("flex items-center gap-2"),
											h.Form(
												h.Method("POST"),
												h.Action("/connections/sync/plaid/"+account.ConnectionID),
												shadcn.Button(shadcn.ButtonProps{
													Variant: shadcn.ButtonSecondary,
													Type:    "submit",
												},
													layouts.IconRefresh(),
													g.Text("Sync Now"),
												),
											),
											h.Form(
												h.Method("POST"),
												h.Action("/connections/resync/plaid/"+account.ConnectionID),
												shadcn.Button(shadcn.ButtonProps{
													Variant: shadcn.ButtonSecondary,
													Type:    "submit",
												},
													g.Attr("title", "Full Resync - Re-imports all transactions with full history"),
													layouts.IconDatabase(),
													g.Text("Resync"),
												),
											),
										),
									),
									g.If(isConnectedToAkahu,
										h.Form(
											h.Method("POST"),
											h.Action("/connections/sync/akahu/"+account.ConnectionID),
											shadcn.Button(shadcn.ButtonProps{
												Variant: shadcn.ButtonSecondary,
												Type:    "submit",
											},
												layouts.IconRefresh(),
												g.Text("Sync Now"),
											),
										),
									),
									// Disconnect button - provider-aware
									g.If(isConnectedToTeller,
										h.Form(
											h.Method("POST"),
											h.Action("/teller/disconnect/"+account.ConnectionID),
											h.Input(h.Type("hidden"), h.Name("_method"), h.Value("DELETE")),
											shadcn.Button(shadcn.ButtonProps{
												Variant: shadcn.ButtonDestructive,
												Type:    "submit",
											},
												layouts.IconUnlink(),
												g.Text("Disconnect"),
											),
										),
									),
									g.If(isConnectedToPlaid,
										h.Form(
											h.Method("POST"),
											h.Action("/connections/disconnect/plaid/"+account.ConnectionID),
											h.Input(h.Type("hidden"), h.Name("_method"), h.Value("DELETE")),
											shadcn.Button(shadcn.ButtonProps{
												Variant: shadcn.ButtonDestructive,
												Type:    "submit",
											},
												layouts.IconUnlink(),
												g.Text("Disconnect"),
											),
										),
									),
									g.If(isConnectedToAkahu,
										h.Form(
											h.Method("POST"),
											h.Action("/connections/disconnect/akahu/"+account.ConnectionID),
											h.Input(h.Type("hidden"), h.Name("_method"), h.Value("DELETE")),
											shadcn.Button(shadcn.ButtonProps{
												Variant: shadcn.ButtonDestructive,
												Type:    "submit",
											},
												layouts.IconUnlink(),
												g.Text("Disconnect"),
											),
										),
									),
								),
							),
						),
						g.If(!isConnected,
							// Not connected state - show connect option
							h.Div(
								h.Class("flex items-center justify-between"),
								h.Div(
									h.Class("flex items-center gap-3"),
									h.Div(
										h.Class("w-10 h-10 rounded-lg bg-secondary flex items-center justify-center text-muted-foreground"),
										layouts.IconBank(),
									),
									h.Div(
										h.P(h.Class("font-medium text-foreground"), g.Text("Connect to Bank")),
										h.P(h.Class("text-sm text-muted-foreground"), g.Text("Link this account to automatically sync transactions")),
									),
								),
								h.A(
									h.Href("/teller/link/"+account.ID.String()),
									h.Class("inline-flex items-center gap-2 bg-primary text-primary-foreground rounded-lg px-4 py-2 text-sm font-medium hover:opacity-90 transition-colors"),
									layouts.IconLink(),
									g.Text("Connect"),
								),
							),
						),
					),
				),
			),
		),
		// Transactions
		shadcn.Card(shadcn.CardProps{},
			shadcn.CardHeaderActions("Transactions", fmt.Sprintf("Showing %d of %d transactions", len(transactions), total)),
			shadcn.CardContent(
				h.Div(
					h.Class("divide-y divide-border"),
					g.If(len(transactions) == 0,
						shadcn.EmptyNoData("No transactions", "This account has no transactions yet.", nil),
					),
					g.Group(g.Map(transactions, func(txn *models.Transaction) g.Node {
						return hdl.renderTransactionListItem(txn, []*models.Account{account})
					})),
				),
			),
		),
	)

	renderHTML(w, page)
}

func (hdl *Handlers) AccountsEdit(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	accountID, ok := mustParamUUID(w, r, "id", "account ID")
	if !ok {
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	page := layouts.AppLayout("Edit "+account.Name, user.Email, user.ID.String(),
		shadcn.PageHeader("Edit Account", account.Name),
		renderAccountForm(account, "/accounts/"+account.ID.String(), "PUT"),
	)

	renderHTML(w, page)
}

func (hdl *Handlers) AccountsUpdate(w http.ResponseWriter, r *http.Request) {
	accountID, ok := mustParamUUID(w, r, "id", "account ID")
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	account.Name = r.FormValue("name")
	account.Type = models.AccountType(r.FormValue("type"))
	account.InstitutionName = r.FormValue("institution_name")

	if err := hdl.accounts.Update(r.Context(), account); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/accounts/"+account.ID.String(), http.StatusSeeOther)
}

func (hdl *Handlers) AccountsDelete(w http.ResponseWriter, r *http.Request) {
	accountID, ok := mustParamUUID(w, r, "id", "account ID")
	if !ok {
		return
	}

	// Get account info for logging
	account, err := hdl.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	// Delete account and all its transactions
	deleted, err := hdl.accounts.DeleteWithTransactions(r.Context(), accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log for audit
	slog.InfoContext(r.Context(), "[accounts] Deleted account", "name", account.Name, "id", accountID, "transactions", deleted)

	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}

func renderAccountForm(account *models.Account, action, method string) g.Node {
	name := ""
	accountType := ""
	institutionName := ""

	if account != nil {
		name = account.Name
		accountType = string(account.Type)
		institutionName = account.InstitutionName
	}

	return shadcn.Card(shadcn.CardProps{},
		shadcn.CardContentFull(
			h.Form(
				h.Method("POST"),
				h.Action(action),
				h.Class("space-y-4"),

				// Method override for PUT
				g.If(method == "PUT",
					h.Input(h.Type("hidden"), h.Name("_method"), h.Value("PUT")),
				),

				shadcn.FormField(shadcn.FormFieldProps{Name: "name"},
					shadcn.Label(shadcn.LabelProps{For: "name", Required: true},
						g.Text("Account Name"),
					),
					shadcn.Input(shadcn.InputProps{
						Type:        "text",
						Name:        "name",
						Placeholder: "e.g., Chase Checking",
						Value:       name,
						Required:    true,
					}),
				),

				shadcn.FormField(shadcn.FormFieldProps{Name: "type"},
					shadcn.Label(shadcn.LabelProps{For: "type", Required: true},
						g.Text("Account Type"),
					),
					shadcn.NativeSelect(shadcn.NativeSelectProps{
						Name:     "type",
						Required: true,
					}, []shadcn.SelectOption{
						{Value: "asset", Label: "Asset (Checking, Savings, Investment)", Selected: accountType == "asset"},
						{Value: "liability", Label: "Liability (Credit Card, Loan)", Selected: accountType == "liability"},
						{Value: "income", Label: "Income", Selected: accountType == "income"},
						{Value: "expense", Label: "Expense", Selected: accountType == "expense"},
						{Value: "equity", Label: "Equity", Selected: accountType == "equity"},
					}),
				),

				shadcn.FormField(shadcn.FormFieldProps{Name: "institution_name"},
					shadcn.Label(shadcn.LabelProps{For: "institution_name"},
						g.Text("Institution Name"),
					),
					shadcn.Input(shadcn.InputProps{
						Type:        "text",
						Name:        "institution_name",
						Placeholder: "e.g., Chase Bank",
						Value:       institutionName,
					}),
				),

				h.Div(
					h.Class("flex items-center gap-3 pt-4"),
					shadcn.Button(shadcn.ButtonProps{
						Variant: shadcn.ButtonDefault,
						Type:    "submit",
					},
						h.Type("submit"),
						g.If(account == nil, g.Text("Create Account")),
						g.If(account != nil, g.Text("Save Changes")),
					),
					h.A(
						h.Href("/accounts"),
						h.Class("text-muted-foreground hover:text-foreground text-sm"),
						g.Text("Cancel"),
					),
				),
			),
		),
	)
}
