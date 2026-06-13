package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/enrichment"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/storage"
	"github.com/asomervell/probably/internal/sync/providers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/plaid/plaid-go/v40/plaid"
)

// PlaidSyncService handles syncing data from Plaid to the local database
type PlaidSyncService struct {
	pool            *pgxpool.Pool
	cfg             *config.Config
	client          *PlaidClient
	accounts        *models.AccountStore
	transactions    *models.TransactionStore
	transferMatcher *TransferMatcher
	entities        *models.EntityStore
}

func NewPlaidSyncService(pool *pgxpool.Pool, client *PlaidClient, cfg *config.Config) *PlaidSyncService {
	return &PlaidSyncService{
		pool:            pool,
		cfg:             cfg,
		client:          client,
		accounts:        models.NewAccountStore(pool),
		transactions:    models.NewTransactionStore(pool),
		transferMatcher: NewTransferMatcher(pool),
		entities:        models.NewEntityStore(pool),
	}
}

// SyncAccounts syncs accounts from Plaid for an item
func (s *PlaidSyncService) SyncAccounts(ctx context.Context, ledgerID uuid.UUID, accessToken string) ([]*models.Account, error) {
	slog.DebugContext(ctx, "PlaidSyncAccounts: starting", "ledger_id", ledgerID)

	plaidAccounts, itemID, institutionID, err := s.client.GetAccounts(ctx, accessToken)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch Plaid accounts", "err", err)
		// Check if this is an item error (needs re-authentication)
		if isPlaidItemError(err) {
			slog.DebugContext(ctx, "PlaidSyncAccounts: item error detected, reconnection required", "err", err)
			return nil, &PlaidItemError{
				Message: "Item requires re-authentication",
				Err:     err,
			}
		}
		return nil, err
	}
	slog.DebugContext(ctx, "PlaidSyncAccounts: fetched accounts from Plaid", "count", len(plaidAccounts), "item_id", itemID, "institution_id", institutionID)

	var synced []*models.Account
	var errs []error
	now := time.Now()
	nameCounts := make(map[string]int)

	// Pre-count base names so we can disambiguate generic duplicates (e.g. "Credit Card").
	for _, pa := range plaidAccounts {
		baseName := buildPlaidAccountBaseName(pa)
		if baseName != "" {
			nameCounts[strings.ToLower(baseName)]++
		}
	}

	// Get institution name and logo
	var institutionName string
	var institutionLogo string
	var institutionURL string
	if institutionID != "" {
		inst, err := s.client.GetInstitution(ctx, institutionID, true) // Include optional metadata for logo
		if err == nil && inst != nil {
			institutionName = inst.GetName()
			// Plaid provides logo as base64 data URL
			if logo := inst.GetLogo(); logo != "" {
				institutionLogo = logo
			}
			institutionURL = inst.GetUrl()
		}
	}

	for _, pa := range plaidAccounts {
		accountID := pa.GetAccountId()
		accountName := buildPlaidDisplayName(pa, nameCounts)
		accountType := pa.GetType()
		accountSubtype := pa.GetSubtype()

		slog.DebugContext(ctx, "PlaidSyncAccounts: processing account", "account_id", accountID, "name", accountName, "institution", institutionName)

		// Check if account already exists by Plaid account ID
		existing, err := s.accounts.GetByExternalAccountID(ctx, accountID)
		if err == nil && existing != nil {
			slog.DebugContext(ctx, "PlaidSyncAccounts: found existing account by ID", "name", existing.Name, "account_id", existing.ID)

			metadata := parseMigrationMetadata(existing)
			if existing.Provider != "" && existing.Provider != "plaid" {
				slog.DebugContext(ctx, "PlaidSyncAccounts: migrating account to plaid", "account_id", existing.ID, "previous_provider", existing.Provider)
			}

			// Update existing with Plaid data
			existing.InstitutionName = institutionName
			existing.InstitutionID = institutionID
			existing.LastFour = extractLastFour(pa.GetMask())
			existing.LastSyncedAt = &now

			// Update generic provider fields
			if existing.Provider == "" {
				existing.Provider = "plaid"
			}
			if existing.Provider == "plaid" && accountName != "" {
				existing.Name = accountName
			}
			if existing.ExternalAccountID == "" {
				existing.ExternalAccountID = accountID
			}
			if existing.ConnectionID == "" {
				existing.ConnectionID = itemID
			}
			if existing.AccessToken == "" {
				existing.AccessToken = accessToken
			}
			if existing.AccountSubtype == "" {
				existing.AccountSubtype = string(accountSubtype)
			}

			// Update balances and merge with existing provider_metadata
			balances := pa.GetBalances()
			balanceData := map[string]interface{}{
				"available": balances.GetAvailable(),
				"current":   balances.GetCurrent(),
				"limit":     balances.GetLimit(),
			}
			if isoCurrency := balances.GetIsoCurrencyCode(); isoCurrency != "" {
				balanceData["iso_currency_code"] = isoCurrency
			}
			if unofficialCurrency := balances.GetUnofficialCurrencyCode(); unofficialCurrency != "" {
				balanceData["unofficial_currency_code"] = unofficialCurrency
			}
			// Merge balance data into metadata
			for k, v := range balanceData {
				metadata[k] = v
			}
			if metadataJSON, err := json.Marshal(metadata); err == nil {
				existing.ProviderMetadata = metadataJSON
			}

			if err := s.accounts.Update(ctx, existing); err != nil {
				slog.WarnContext(ctx, "failed to update reconnected Plaid account", "account_id", accountID, "err", err)
				errs = append(errs, fmt.Errorf("failed to update reconnected account %s: %w", accountID, err))
				continue
			}

			synced = append(synced, existing)
			slog.DebugContext(ctx, "PlaidSyncAccounts: successfully reconnected account", "account_id", accountID)
			continue
		}

		// Try to find a disconnected account by last four + institution
		lastFour := extractLastFour(pa.GetMask())
		if lastFour != "" {
			existing, err = s.accounts.FindByLastFourAndInstitution(ctx, ledgerID, lastFour, institutionName)
			if err == nil && existing != nil {
				slog.DebugContext(ctx, "PlaidSyncAccounts: reconnecting existing account by last four", "name", existing.Name, "account_id", existing.ID, "plaid_account_id", accountID, "last_four", lastFour)
			}
		}

		// Try to find by name + institution
		if existing == nil {
			existing, err = s.accounts.FindByNameAndInstitution(ctx, ledgerID, accountName, institutionName)
			if err == nil && existing != nil {
				slog.DebugContext(ctx, "PlaidSyncAccounts: reconnecting existing account by name", "name", existing.Name, "account_id", existing.ID, "plaid_account_id", accountID, "account_name", accountName)
			}
		}

		if err == nil && existing != nil {
			if existing.Provider != "" && existing.Provider != "plaid" {
				slog.DebugContext(ctx, "PlaidSyncAccounts: migrating account to plaid", "account_id", existing.ID, "previous_provider", existing.Provider)
				if metadataJSON, err := json.Marshal(parseMigrationMetadata(existing)); err == nil {
					existing.ProviderMetadata = metadataJSON
				}
			}

			// Reconnect: Update with new Plaid credentials
			existing.Provider = "plaid"
			if accountName != "" {
				existing.Name = accountName
			}
			existing.ExternalAccountID = accountID
			existing.ConnectionID = itemID
			existing.AccessToken = accessToken
			existing.InstitutionName = institutionName
			existing.InstitutionID = institutionID
			existing.AccountSubtype = string(accountSubtype)
			existing.LastFour = lastFour
			existing.LastSyncedAt = &now

			if err := s.accounts.Update(ctx, existing); err != nil {
				slog.WarnContext(ctx, "failed to update reconnected Plaid account", "account_id", accountID, "err", err)
				errs = append(errs, fmt.Errorf("failed to update reconnected account %s: %w", accountID, err))
				continue
			}

			synced = append(synced, existing)
			slog.DebugContext(ctx, "PlaidSyncAccounts: successfully migrated/reconnected account to Plaid", "account_id", accountID)
			continue
		}

		// Create new account
		slog.DebugContext(ctx, "PlaidSyncAccounts: creating new account", "account_id", accountID, "name", accountName, "institution", institutionName)
		acc := &models.Account{
			LedgerID:          ledgerID,
			Name:              accountName,
			Type:              mapPlaidAccountType(accountType, accountSubtype),
			InstitutionName:   institutionName,
			InstitutionID:     institutionID,
			Provider:          "plaid",
			ExternalAccountID: accountID,
			ConnectionID:      itemID,
			AccessToken:       accessToken,
			AccountSubtype:    string(accountSubtype),
			LastFour:          lastFour,
			LastSyncedAt:      &now,
			IsActive:          true,
		}

		if err := s.accounts.Create(ctx, acc); err != nil {
			slog.ErrorContext(ctx, "failed to create Plaid account", "account_id", accountID, "err", err)
			errs = append(errs, fmt.Errorf("failed to create account %s: %w", accountID, err))
			continue
		}

		synced = append(synced, acc)
		slog.DebugContext(ctx, "PlaidSyncAccounts: successfully created new account", "account_id", accountID)
	}

	slog.DebugContext(ctx, "PlaidSyncAccounts: completed", "synced", len(synced), "total", len(plaidAccounts), "errors", len(errs))

	// Sync institution logos if we have a logo from Plaid
	if institutionLogo != "" && institutionID != "" {
		s.syncPlaidInstitutionLogo(ctx, synced, institutionID, institutionLogo)
	} else if institutionID != "" {
		// Some institutions do not provide logo base64 (or even URL) via Plaid optional
		// metadata. Fall back to website/domain-based logo fetch.
		fallbackSource := institutionURL
		if fallbackSource == "" {
			fallbackSource = guessInstitutionDomain(institutionName)
		}
		if fallbackSource != "" {
			s.syncPlaidInstitutionLogoFromWebsite(ctx, synced, institutionID, fallbackSource)
		}
	}

	return synced, errors.Join(errs...)
}

// parseMigrationMetadata parses existing.ProviderMetadata into a map and stamps migration
// keys if the account is moving to Plaid from a different provider.
func parseMigrationMetadata(existing *models.Account) map[string]interface{} {
	m := make(map[string]interface{})
	if len(existing.ProviderMetadata) > 0 {
		if err := json.Unmarshal(existing.ProviderMetadata, &m); err != nil {
			m = make(map[string]interface{})
		}
	}
	if existing.Provider != "" && existing.Provider != "plaid" {
		m["previous_provider"] = existing.Provider
		m["previous_external_account_id"] = existing.ExternalAccountID
		m["previous_connection_id"] = existing.ConnectionID
		if existing.LastSyncedAt != nil {
			m["previous_last_synced_at"] = existing.LastSyncedAt.Format(time.RFC3339)
		}
		if existing.Provider == "teller" {
			m["teller_account_id"] = existing.TellerAccountID
			m["teller_enrollment_id"] = existing.TellerEnrollmentID
		}
	}
	return m
}

func (s *PlaidSyncService) buildLogoStore(ctx context.Context) *enrichment.LogoStore {
	storageInstance, err := storage.NewStorageFromEnv(ctx, s.cfg.BaseURL)
	if err != nil {
		slog.DebugContext(ctx, "PlaidSyncAccounts: cloud storage unavailable, falling back to local storage", "err", err)
		storageInstance = storage.NewLocalStorage("static/logos", s.cfg.BaseURL)
	}
	return enrichment.NewLogoStore(storageInstance, "", "")
}

// filterAccountsNeedingLogo returns accounts from the given set that belong to
// institutionID and have no logo yet.
func filterAccountsNeedingLogo(accounts []*models.Account, institutionID string) []*models.Account {
	var out []*models.Account
	for _, acc := range accounts {
		if acc.InstitutionID == institutionID && acc.InstitutionLogoURL == "" {
			out = append(out, acc)
		}
	}
	return out
}

// SyncTransactions syncs transactions for an account
func (s *PlaidSyncService) SyncTransactions(ctx context.Context, account *models.Account) (int, error) {
	slog.DebugContext(ctx, "PlaidSyncTransactions: starting", "account", account.Name, "account_id", account.ID)

	if account.ExternalAccountID == "" {
		slog.ErrorContext(ctx, "no external account ID for Plaid account", "account_id", account.ID)
		return 0, fmt.Errorf("account has no external account ID")
	}

	accessToken := account.AccessToken

	// Determine sync date range
	endDate := time.Now()

	// Check if this is the first sync or if we need to backfill history
	var existingTxnCount int
	var oldestTxnDate *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT t.id), MIN(t.date)
		FROM transactions t
		JOIN entries e ON t.id = e.transaction_id
		WHERE e.account_id = $1
	`, account.ID).Scan(&existingTxnCount, &oldestTxnDate)

	var startDate time.Time
	if err != nil || existingTxnCount == 0 {
		// First sync: fetch up to 2 years of history (Plaid's typical limit)
		startDate = endDate.AddDate(-2, 0, 0)
		slog.DebugContext(ctx, "PlaidSyncTransactions: first sync detected, fetching 2 years of history", "start_date", startDate.Format("2006-01-02"))
	} else if oldestTxnDate != nil {
		// Check if we have less than 2 years of history
		// If oldest transaction is less than 2 years old, backfill to get full history
		daysSinceOldest := int(endDate.Sub(*oldestTxnDate).Hours() / 24)
		twoYearsInDays := 730 // 2 years = ~730 days

		// Also check if account was recently created/synced (within last 6 months)
		// If so, always backfill to ensure we get full history
		shouldBackfill := false
		if account.LastSyncedAt != nil {
			daysSinceFirstSync := int(endDate.Sub(*account.LastSyncedAt).Hours() / 24)
			// If account was first synced within last 6 months, backfill
			if daysSinceFirstSync < 180 {
				shouldBackfill = true
				slog.DebugContext(ctx, "PlaidSyncTransactions: account was recently first synced, will backfill to get full history", "days_since_first_sync", daysSinceFirstSync)
			}
		}

		if shouldBackfill || daysSinceOldest < twoYearsInDays {
			// Backfill: fetch 2 years of history
			startDate = endDate.AddDate(-2, 0, 0)
			reason := "recent account"
			if !shouldBackfill {
				reason = fmt.Sprintf("oldest transaction is %d days old (less than 2 years)", daysSinceOldest)
			}
			slog.DebugContext(ctx, "PlaidSyncTransactions: backfilling history", "reason", reason, "start_date", startDate.Format("2006-01-02"))
		} else {
			// We already have 2+ years of history, just do incremental sync
			// Ongoing sync: default to last 90 days
			startDate = endDate.AddDate(0, 0, -90)
			slog.DebugContext(ctx, "PlaidSyncTransactions: incremental sync, fetching last 90 days", "days_since_oldest", daysSinceOldest, "start_date", startDate.Format("2006-01-02"))
		}
	} else {
		// Ongoing sync: default to last 90 days
		startDate = endDate.AddDate(0, 0, -90)

		// Check if this is a provider migration - if so, sync from last transaction date
		if len(account.ProviderMetadata) > 0 {
			var metadata map[string]interface{}
			if err := json.Unmarshal(account.ProviderMetadata, &metadata); err == nil {
				if prevProvider, ok := metadata["previous_provider"].(string); ok && prevProvider != "" {
					// This account was migrated from another provider
					// Find the most recent transaction date to continue from
					var lastTxnDate *time.Time
					err := s.pool.QueryRow(ctx, `
						SELECT MAX(t.date)
						FROM transactions t
						JOIN entries e ON t.id = e.transaction_id
						WHERE e.account_id = $1
					`, account.ID).Scan(&lastTxnDate)

					if err == nil && lastTxnDate != nil {
						// Start from 1 day before the last transaction (or 90 days ago, whichever is more recent)
						// This ensures we don't miss any transactions and don't duplicate
						proposedStart := lastTxnDate.AddDate(0, 0, -1) // 1 day before last transaction for overlap safety
						if proposedStart.After(startDate) {
							startDate = proposedStart
							slog.DebugContext(ctx, "PlaidSyncTransactions: continuing from previous provider", "previous_provider", prevProvider, "start_date", startDate.Format("2006-01-02"))
						}
					}
				}
			}
		}
	}

	plaidTransactions, err := s.client.GetTransactions(ctx, accessToken, startDate, endDate, []string{account.ExternalAccountID})
	if err != nil {
		if isPlaidItemError(err) {
			slog.WarnContext(ctx, "Plaid item error, reconnection required", "account_id", account.ID, "err", err)
			return 0, &PlaidItemError{
				Message: "Item requires re-authentication",
				Err:     err,
			}
		}
		slog.ErrorContext(ctx, "failed to fetch Plaid transactions", "account", account.Name, "err", err)
		// Still update the timestamp so we know sync was attempted
		now := time.Now()
		account.LastSyncedAt = &now
		if updateErr := s.accounts.Update(ctx, account); updateErr != nil {
			slog.WarnContext(ctx, "failed to update account last_synced_at after fetch error", "account", account.Name, "err", updateErr)
		}
		return 0, err
	}

	slog.DebugContext(ctx, "PlaidSyncTransactions: fetched transactions from Plaid", "count", len(plaidTransactions), "account", account.Name)

	expenseAccount, incomeAccount, err := getContraAccounts(ctx, s.accounts, account.LedgerID)
	if err != nil {
		return 0, err
	}

	var created, storeErrors int
	var newTxnIDs []uuid.UUID
	var needsEnrichment []uuid.UUID // Transactions that need enrichment (Plaid not confident)
	for _, pt := range plaidTransactions {
		// Check if transaction already exists by Plaid transaction ID
		existing, err := s.transactions.GetByTellerID(ctx, pt.GetTransactionId())
		if err == nil && existing != nil {
			continue // Skip existing
		}

		// Parse date - use authorized_date if available, otherwise date
		var date time.Time
		if authorizedDate := pt.GetAuthorizedDate(); authorizedDate != "" {
			date, err = time.Parse("2006-01-02", authorizedDate)
		}
		if err != nil || date.IsZero() {
			date, err = time.Parse("2006-01-02", pt.GetDate())
		}
		if err != nil {
			date = time.Now().UTC()
		}

		// Convert amount to cents
		amount := pt.GetAmount()
		amountCents := dollarsToCents(amount)

		// Get description - prefer merchant_name, then name
		description := pt.GetName()
		if merchantName := pt.GetMerchantName(); merchantName != "" {
			description = merchantName
		}

		// Check for duplicates by date/description/amount
		exists, err := s.transactions.ExistsByDateDescriptionAmount(ctx, account.ID, date, description, amountCents)
		if err == nil && exists {
			slog.DebugContext(ctx, "skipping duplicate transaction by content", "description", description, "date", pt.GetDate(), "amount_cents", amountCents)
			continue
		}

		// Plaid amount sign convention (per Plaid API documentation):
		// - Positive = money moves OUT of account (purchases, withdrawals, debits)
		// - Negative = money moves IN to account (deposits, refunds, credits)
		// Store amounts exactly as Plaid reports them
		amountForEntry := amountCents

		// Create transaction with all available Plaid data
		txn := &models.Transaction{
			LedgerID:             account.LedgerID,
			Date:                 date,
			Description:          description,
			TellerTransactionID:  pt.GetTransactionId(),
			CategorizationStatus: models.CategorizationStatusPending, // Queue for categorization
		}

		if originalDesc := pt.GetOriginalDescription(); originalDesc != "" && originalDesc != description {
			txn.Notes = appendNote(txn.Notes, "Original: "+originalDesc)
		}
		if checkNumber := pt.GetCheckNumber(); checkNumber != "" {
			txn.Notes = appendNote(txn.Notes, "Check #"+checkNumber)
		}
		if accountOwner := pt.GetAccountOwner(); accountOwner != "" {
			txn.Notes = appendNote(txn.Notes, "Owner: "+accountOwner)
		}

		// Process counterparties and create entities from Plaid merchant data
		// Plaid provides counterparties array with name, type, entity_id, website, logo_url
		// Also check merchant_entity_id, merchant_name, logo_url, website on transaction
		counterparties := pt.GetCounterparties()

		// Prefer merchant_entity_id (most stable), then counterparty.entity_id
		merchantEntityID := pt.GetMerchantEntityId()
		var plaidEntityID string
		var merchantName string
		var logoURL string
		var website string

		if merchantEntityID != "" {
			plaidEntityID = merchantEntityID
			merchantName = pt.GetMerchantName()
			logoURL = pt.GetLogoUrl()
			website = pt.GetWebsite()
		} else if len(counterparties) > 0 {
			cp := counterparties[0] // Use first counterparty as primary
			txn.CounterpartyName = cp.GetName()
			txn.CounterpartyType = string(cp.GetType())

			if cp.GetEntityId() != "" {
				plaidEntityID = cp.GetEntityId()
			}
			if cp.GetName() != "" {
				merchantName = cp.GetName()
			}
			if cp.GetLogoUrl() != "" {
				logoURL = cp.GetLogoUrl()
			}
			if cp.GetWebsite() != "" {
				website = cp.GetWebsite()
			}
		}

		// Extract Plaid categories before entity creation so subtype mapping is populated.
		// Prefer Personal Finance Category (PFC) over legacy categories.
		var plaidCategory string
		if pfc, ok := pt.GetPersonalFinanceCategoryOk(); ok && pfc != nil {
			if primary := pfc.GetPrimary(); primary != "" {
				plaidCategory = primary
				txn.TellerCategory = primary
			}
			if detailed := pfc.GetDetailed(); detailed != "" && detailed != pfc.GetPrimary() {
				txn.Notes = appendNote(txn.Notes, "PFC Detailed: "+detailed)
			}
		}
		if plaidCategory == "" {
			if categories := pt.GetCategory(); len(categories) > 0 {
				plaidCategory = strings.Join(categories, " > ")
				txn.TellerCategory = plaidCategory
			} else if categoryID := pt.GetCategoryId(); categoryID != "" {
				plaidCategory = categoryID
				txn.TellerCategory = categoryID
			}
		}
		if bfc, ok := pt.GetBusinessFinanceCategoryOk(); ok && bfc != nil {
			if primary := bfc.GetPrimary(); primary != "" {
				txn.Notes = appendNote(txn.Notes, "Business Category: "+primary)
			}
		}

		// Create or get entity from Plaid merchant data, now with category populated.
		if plaidEntityID != "" && merchantName != "" {
			entity, err := s.getOrCreatePlaidEntity(ctx, merchantName, plaidEntityID, logoURL, website, txn.TellerCategory)
			if err != nil {
				slog.WarnContext(ctx, "failed to create entity for Plaid merchant", "merchant", merchantName, "plaid_entity_id", plaidEntityID, "err", err)
			} else if entity != nil {
				txn.EntityID = &entity.ID
				slog.DebugContext(ctx, "PlaidSyncTransactions: linked transaction to entity", "entity_name", entity.Name, "plaid_entity_id", plaidEntityID)
			}
		}

		// Store payment channel
		paymentChannel := pt.GetPaymentChannel()
		txn.TellerType = paymentChannel // Store in TellerType field

		// Store transaction code (e.g., direct deposit, wire transfer)
		if txnCode, ok := pt.GetTransactionCodeOk(); ok && txnCode != nil {
			if codeStr := string(*txnCode); codeStr != "" {
				txn.Notes = appendNote(txn.Notes, "Code: "+codeStr)
			}
		}

		// Store location information if available
		location := pt.GetLocation()
		var locationParts []string
		if city := location.GetCity(); city != "" {
			locationParts = append(locationParts, city)
		}
		if region := location.GetRegion(); region != "" {
			locationParts = append(locationParts, region)
		}
		if country := location.GetCountry(); country != "" {
			locationParts = append(locationParts, country)
		}
		if len(locationParts) > 0 {
			txn.Notes = appendNote(txn.Notes, "Location: "+strings.Join(locationParts, ", "))
		}

		// Merchant entity ID, logo URL, and website are now stored in entity records
		// No need to store in notes anymore

		// Store pending transaction ID if available
		if pendingTxnID := pt.GetPendingTransactionId(); pendingTxnID != "" {
			txn.Notes = appendNote(txn.Notes, "Pending Txn ID: "+pendingTxnID)
		}

		// Store payment metadata
		paymentMeta := pt.GetPaymentMeta()
		var metaParts []string
		if referenceNumber := paymentMeta.GetReferenceNumber(); referenceNumber != "" {
			metaParts = append(metaParts, "Ref: "+referenceNumber)
		}
		if ppdID := paymentMeta.GetPpdId(); ppdID != "" {
			metaParts = append(metaParts, "PPD ID: "+ppdID)
		}
		if payee := paymentMeta.GetPayee(); payee != "" {
			metaParts = append(metaParts, "Payee: "+payee)
		}
		if payer := paymentMeta.GetPayer(); payer != "" {
			metaParts = append(metaParts, "Payer: "+payer)
		}
		if paymentMethod := paymentMeta.GetPaymentMethod(); paymentMethod != "" {
			metaParts = append(metaParts, "Method: "+paymentMethod)
		}
		if paymentProcessor := paymentMeta.GetPaymentProcessor(); paymentProcessor != "" {
			metaParts = append(metaParts, "Processor: "+paymentProcessor)
		}
		if reason := paymentMeta.GetReason(); reason != "" {
			metaParts = append(metaParts, "Reason: "+reason)
		}
		if len(metaParts) > 0 {
			txn.Notes = appendNote(txn.Notes, strings.Join(metaParts, ", "))
		}

		// Store pending status
		if pt.GetPending() {
			txn.TellerStatus = "pending"
		} else {
			txn.TellerStatus = "posted"
		}

		// Plaid convention: positive = money OUT (expense), negative = money IN (income).
		// This holds for both asset and liability accounts.
		var contraAccountID uuid.UUID
		isExpense := amountForEntry > 0

		// REFUNDS always map to expense so they appear as negative expense, not income.
		if strings.Contains(strings.ToLower(description), "refund") {
			isExpense = true
		}

		if isExpense {
			contraAccountID = expenseAccount.ID
		} else {
			contraAccountID = incomeAccount.ID
		}

		// Get currency
		currency := "USD"
		if isoCurrency := pt.GetIsoCurrencyCode(); isoCurrency != "" {
			currency = isoCurrency
		} else if unofficialCurrency := pt.GetUnofficialCurrencyCode(); unofficialCurrency != "" {
			currency = unofficialCurrency
		}

		entries := []*models.Entry{
			{AccountID: account.ID, AmountCents: amountForEntry, Currency: currency},
			{AccountID: contraAccountID, AmountCents: -amountForEntry, Currency: currency},
		}

		if err := s.transactions.CreateWithEntries(ctx, txn, entries); err != nil {
			// Log and skip — a single bad row (e.g., constraint violation) should not
			// abort the entire sync and leave newer transactions unimported.
			slog.WarnContext(ctx, "failed to store transaction, skipping", "plaid_txn_id", pt.GetTransactionId(), "description", pt.GetName(), "err", err)
			storeErrors++
			continue
		}
		created++
		newTxnIDs = append(newTxnIDs, txn.ID)

		// Determine if Plaid is confident about merchant data
		// Plaid is confident if:
		// 1. Has merchant_entity_id (most reliable indicator)
		// 2. OR has merchant_name AND (logo_url OR website) - indicates enriched merchant data
		// Otherwise, queue for enrichment
		isPlaidConfident := merchantEntityID != "" || (merchantName != "" && (logoURL != "" || website != ""))
		if !isPlaidConfident {
			needsEnrichment = append(needsEnrichment, txn.ID)
			slog.DebugContext(ctx, "PlaidSyncTransactions: transaction needs enrichment, Plaid merchant data incomplete", "txn_id", txn.ID)
		} else {
			slog.DebugContext(ctx, "PlaidSyncTransactions: transaction using Plaid merchant data", "txn_id", txn.ID, "entity_id", plaidEntityID, "merchant", merchantName)
		}

		if err := s.transferMatcher.ProcessNewTransaction(ctx, txn, entries[0]); err != nil {
			logTransferMatchingError(ctx, "sync_plaid", err, txn.ID)
		}
	}
	if storeErrors > 0 {
		storeErr := fmt.Errorf("failed to store %d of %d transactions", storeErrors, len(plaidTransactions))
		slog.WarnContext(ctx, "sync completed with store failures", "account_id", account.ID, "store_errors", storeErrors, "created", created)
		observability.CaptureFailure(ctx, storeErr, observability.FailureOptions{
			Component: "sync_plaid",
			Operation: "store_transaction",
			Tags:      map[string]string{"account_id": account.ID.String()},
		})
	}

	updateLastSyncedAt(ctx, account, s.accounts)

	// Queue only transactions where Plaid is not confident about merchant data
	// Plaid is confident if it has merchant_entity_id or merchant_name with logo/website
	// For transactions where Plaid lacks confidence, use our enrichment pipeline
	if len(needsEnrichment) > 0 {
		if err := s.transactions.QueueForEnrichment(ctx, needsEnrichment); err != nil {
			slog.WarnContext(ctx, "failed to queue transactions for enrichment", "err", err)
		} else {
			slog.DebugContext(ctx, "queued transactions for enrichment, Plaid merchant data incomplete", "count", len(needsEnrichment))
		}
	}

	trustedCount := len(newTxnIDs) - len(needsEnrichment)
	if trustedCount > 0 {
		slog.DebugContext(ctx, "trusted Plaid merchant data, skipped enrichment", "count", trustedCount)
	}

	slog.DebugContext(ctx, "PlaidSyncTransactions: completed", "account", account.Name, "created", created)
	return created, nil
}

// DeleteAndResync deletes all transactions for an account and re-imports them
func (s *PlaidSyncService) DeleteAndResync(ctx context.Context, account *models.Account) (int, error) {
	// Find all transaction IDs that have entries to this account
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT transaction_id FROM entries WHERE account_id = $1
	`, account.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to find transactions: %w", err)
	}
	defer rows.Close()

	txnIDs, err := scanUUIDRows(rows)
	if err != nil {
		return 0, fmt.Errorf("failed to scan transaction IDs: %w", err)
	}

	// Delete transactions (entries will cascade)
	for _, txnID := range txnIDs {
		if err := s.transactions.Delete(ctx, txnID); err != nil {
			slog.ErrorContext(ctx, "Error deleting transaction", "id", txnID, "err", err)
		}
	}

	// Re-sync transactions
	// SyncTransactions will detect 0 transactions and automatically fetch 2 years of history
	return s.SyncTransactions(ctx, account)
}

// Helper functions

func extractLastFour(mask string) string {
	if len(mask) >= 4 {
		return mask[len(mask)-4:]
	}
	return mask
}

func buildPlaidAccountBaseName(pa plaid.AccountBase) string {
	if officialName := strings.TrimSpace(pa.GetOfficialName()); officialName != "" {
		return officialName
	}
	if name := strings.TrimSpace(pa.GetName()); name != "" {
		return name
	}
	if subtype := strings.TrimSpace(string(pa.GetSubtype())); subtype != "" {
		return strings.ReplaceAll(subtype, "_", " ")
	}
	return strings.TrimSpace(string(pa.GetType()))
}

func buildPlaidDisplayName(pa plaid.AccountBase, nameCounts map[string]int) string {
	baseName := buildPlaidAccountBaseName(pa)
	if baseName == "" {
		baseName = "Account"
	}

	key := strings.ToLower(strings.TrimSpace(baseName))
	lastFour := extractLastFour(strings.TrimSpace(pa.GetMask()))
	needsDisambiguation := nameCounts[key] > 1 || key == "credit card"
	if needsDisambiguation && lastFour != "" {
		return fmt.Sprintf("%s •••• %s", baseName, lastFour)
	}

	return baseName
}

func mapPlaidAccountType(accountType plaid.AccountType, subtype plaid.AccountSubtype) models.AccountType {
	// Plaid account types are strings like "depository", "credit", "loan", "investment", etc.
	typeStr := string(accountType)
	switch strings.ToLower(typeStr) {
	case "depository":
		return models.AccountTypeAsset
	case "credit":
		return models.AccountTypeLiability
	case "loan":
		return models.AccountTypeLiability
	case "investment":
		return models.AccountTypeAsset
	case "other":
		// Try to infer from subtype
		subtypeStr := string(subtype)
		if strings.Contains(strings.ToLower(subtypeStr), "credit") {
			return models.AccountTypeLiability
		}
		return models.AccountTypeAsset
	default:
		return models.AccountTypeAsset
	}
}

// getOrCreatePlaidEntity gets or creates an entity from Plaid merchant data
// Uses Plaid's merchant_entity_id or counterparty.entity_id for 1:1 matching
func (s *PlaidSyncService) getOrCreatePlaidEntity(ctx context.Context, name, plaidEntityID, logoURL, website, category string) (*models.Entity, error) {
	if name == "" || plaidEntityID == "" {
		return nil, nil
	}

	// Try to find existing entity by Plaid external_id
	existing, err := s.entities.GetByExternalID(ctx, "plaid", plaidEntityID)
	if err == nil && existing != nil {
		// Found existing - update logo/website if missing
		updated := false
		if existing.LogoURL == "" && logoURL != "" {
			existing.LogoURL = logoURL
			updated = true
		}
		if existing.Website == "" && website != "" {
			existing.Website = website
			updated = true
		}
		if updated {
			if err := s.entities.Update(ctx, existing); err != nil {
				slog.WarnContext(ctx, "failed to update Plaid entity", "entity_id", existing.ID, "err", err)
			}
		}
		return existing, nil
	}

	// Create new entity from Plaid merchant data
	entity := &models.Entity{
		Type:           models.EntityTypeBusiness, // Plaid merchants are businesses
		Name:           name,
		ExternalID:     plaidEntityID,
		ExternalSource: "plaid",
		LogoURL:        logoURL,
		Website:        website,
		UserVerified:   false,
	}

	// Determine business subtype from Plaid category if available
	if category != "" {
		entity.Subtype = mapPlaidCategoryToSubtype(category)
	} else {
		entity.Subtype = models.BusinessSubtypeRetailer // Default fallback
	}

	entity.Slug = models.Slugify(name)

	if err := s.entities.Create(ctx, entity); err != nil {
		// Race condition - try to fetch again
		if existing, err2 := s.entities.GetByExternalID(ctx, "plaid", plaidEntityID); err2 == nil {
			return existing, nil
		}
		return nil, fmt.Errorf("failed to create Plaid entity: %w", err)
	}

	slog.DebugContext(ctx, "PlaidSyncTransactions: created entity from Plaid merchant", "entity_name", entity.Name, "plaid_entity_id", plaidEntityID)
	return entity, nil
}

// mapPlaidCategoryToSubtype maps Plaid Personal Finance Category to business subtype
func mapPlaidCategoryToSubtype(category string) string {
	categoryLower := strings.ToLower(category)

	// Food and drink
	if strings.Contains(categoryLower, "restaurant") || strings.Contains(categoryLower, "dining") {
		return models.BusinessSubtypeRestaurant
	}
	if strings.Contains(categoryLower, "cafe") || strings.Contains(categoryLower, "coffee") {
		return models.BusinessSubtypeCafe
	}
	if strings.Contains(categoryLower, "grocery") || strings.Contains(categoryLower, "supermarket") {
		return models.BusinessSubtypeSupermarket
	}
	if strings.Contains(categoryLower, "food") || strings.Contains(categoryLower, "beverage") {
		return models.BusinessSubtypeFoodAndBeverage
	}

	// Services and subscriptions
	if strings.Contains(categoryLower, "software") || strings.Contains(categoryLower, "saas") {
		return models.BusinessSubtypeSoftware
	}
	if strings.Contains(categoryLower, "utility") || strings.Contains(categoryLower, "electric") ||
		strings.Contains(categoryLower, "gas") || strings.Contains(categoryLower, "water") ||
		strings.Contains(categoryLower, "internet") || strings.Contains(categoryLower, "phone") {
		return models.BusinessSubtypeUtility
	}
	if strings.Contains(categoryLower, "entertainment") || strings.Contains(categoryLower, "streaming") {
		return models.BusinessSubtypeEntertainment
	}
	if strings.Contains(categoryLower, "gym") || strings.Contains(categoryLower, "fitness") {
		return models.BusinessSubtypeFitness
	}

	// Other
	if strings.Contains(categoryLower, "transportation") || strings.Contains(categoryLower, "taxi") ||
		strings.Contains(categoryLower, "uber") || strings.Contains(categoryLower, "lyft") ||
		strings.Contains(categoryLower, "parking") || strings.Contains(categoryLower, "fuel") {
		return models.BusinessSubtypeTransportation
	}
	if strings.Contains(categoryLower, "healthcare") || strings.Contains(categoryLower, "medical") ||
		strings.Contains(categoryLower, "pharmacy") {
		return models.BusinessSubtypeHealthcare
	}
	if strings.Contains(categoryLower, "education") || strings.Contains(categoryLower, "school") {
		return models.BusinessSubtypeEducation
	}
	if strings.Contains(categoryLower, "travel") || strings.Contains(categoryLower, "airline") ||
		strings.Contains(categoryLower, "hotel") {
		return models.BusinessSubtypeTravel
	}
	if strings.Contains(categoryLower, "bank") || strings.Contains(categoryLower, "financial") ||
		strings.Contains(categoryLower, "atm") {
		return models.BusinessSubtypeFinancialInstitution
	}
	if strings.Contains(categoryLower, "government") || strings.Contains(categoryLower, "tax") {
		return models.BusinessSubtypeGovernment
	}

	// Retail (default for most merchants)
	return models.BusinessSubtypeRetailer
}

// syncPlaidInstitutionLogo downloads and stores the Plaid institution logo for all accounts
func (s *PlaidSyncService) syncPlaidInstitutionLogo(ctx context.Context, accounts []*models.Account, institutionID, base64Logo string) {
	needsLogo := filterAccountsNeedingLogo(accounts, institutionID)
	if len(needsLogo) == 0 {
		return
	}

	logoStore := s.buildLogoStore(ctx)

	// Store the base64 logo
	slog.DebugContext(ctx, "PlaidSyncAccounts: storing logo for institution", "institution_id", institutionID)
	localPath, err := logoStore.StoreBase64Logo(ctx, base64Logo)
	if err != nil {
		slog.WarnContext(ctx, "could not store logo for institution", "institution_id", institutionID, "err", err)
		return
	}

	if localPath == "" {
		slog.WarnContext(ctx, "empty logo path returned for institution", "institution_id", institutionID)
		return
	}

	slog.DebugContext(ctx, "PlaidSyncAccounts: stored logo for institution", "institution_id", institutionID, "path", localPath)

	s.updateAccountsWithLogo(ctx, needsLogo, localPath)
}

func (s *PlaidSyncService) syncPlaidInstitutionLogoFromWebsite(ctx context.Context, accounts []*models.Account, institutionID, website string) {
	needsLogo := filterAccountsNeedingLogo(accounts, institutionID)
	if len(needsLogo) == 0 {
		return
	}

	source := strings.TrimSpace(website)
	if source == "" {
		return
	}
	if !strings.Contains(source, "://") {
		source = "https://" + source
	}

	parsed, err := url.Parse(source)
	if err != nil {
		return
	}
	domain := strings.TrimSpace(parsed.Hostname())
	domain = strings.TrimPrefix(domain, "www.")
	if domain == "" {
		return
	}

	logoStore := s.buildLogoStore(ctx)
	// Try multiple public logo sources because availability varies by environment.
	candidates := []string{
		"https://logo.clearbit.com/" + domain,
		"https://img.logo.dev/" + domain,
		"https://www.google.com/s2/favicons?sz=128&domain=" + domain,
	}

	var localPath string
	var lastErr error
	for _, candidate := range candidates {
		var err error
		localPath, err = logoStore.DownloadAndStore(ctx, candidate)
		if err == nil && localPath != "" {
			break
		}
		lastErr = err
	}

	if localPath == "" {
		slog.DebugContext(ctx, "PlaidSyncAccounts: fallback website logo fetch failed", "institution_id", institutionID, "domain", domain, "err", lastErr)
		return
	}

	s.updateAccountsWithLogo(ctx, needsLogo, localPath)
}

func (s *PlaidSyncService) updateAccountsWithLogo(ctx context.Context, accounts []*models.Account, logoPath string) {
	for _, acc := range accounts {
		acc.InstitutionLogoURL = logoPath
		if err := s.accounts.Update(ctx, acc); err != nil {
			slog.WarnContext(ctx, "could not update logo for Plaid account", "account_id", acc.ID, "err", err)
		}
	}
}

func guessInstitutionDomain(institutionName string) string {
	name := strings.ToLower(strings.TrimSpace(institutionName))
	if name == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	domain := b.String()
	if len(domain) < 3 {
		return ""
	}
	return domain + ".com"
}

// appendNote appends line to notes with a newline separator, or returns line when notes is empty.
func appendNote(notes, line string) string {
	if notes == "" {
		return line
	}
	return notes + "\n" + line
}

// PlaidItemError represents a Plaid item error that requires user action or is permanently unavailable.
type PlaidItemError struct {
	Message string
	Err     error
}

func (e *PlaidItemError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *PlaidItemError) Unwrap() error { return e.Err }

// IsConnectionDisconnected returns true — all PlaidItemErrors represent disconnected items.
func (e *PlaidItemError) IsConnectionDisconnected() bool { return true }

// Provider returns the provider name for this error.
func (e *PlaidItemError) Provider() providers.ProviderName { return providers.ProviderPlaid }

// SyncAccountsWithToken implements providers.ProviderImpl.
func (s *PlaidSyncService) SyncAccountsWithToken(ctx context.Context, ledgerID uuid.UUID, accessToken string) ([]*models.Account, error) {
	return s.SyncAccounts(ctx, ledgerID, accessToken)
}

// SyncTransactionsForAccount implements providers.ProviderImpl.
func (s *PlaidSyncService) SyncTransactionsForAccount(ctx context.Context, account *models.Account) (int, error) {
	return s.SyncTransactions(ctx, account)
}
