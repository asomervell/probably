package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/sync/providers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultAkahuAPIBaseURL = "https://api.akahu.io/v1"

// AkahuAPIError represents a structured error from the Akahu API
type AkahuAPIError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e *AkahuAPIError) Error() string {
	return fmt.Sprintf("akahu API error: %s - %s", e.Status, e.Message)
}

// IsConnectionDisconnected returns true if this error indicates the connection needs re-authentication.
// 401/403 = token invalid or revoked. 404 = account removed from Akahu connection.
func (e *AkahuAPIError) IsConnectionDisconnected() bool {
	return e.StatusCode == 401 || e.StatusCode == 403 || e.StatusCode == 404
}

// Provider returns the provider name for this error
func (e *AkahuAPIError) Provider() providers.ProviderName {
	return providers.ProviderAkahu
}

// IsAkahuTransientError reports whether err is a transient Akahu server error (5xx or 429).
// Transient errors reflect a provider-side outage or rate limit, not an application bug. Callers
// should retry with back-off rather than capturing to the error tracker.
func IsAkahuTransientError(err error) bool {
	var akahuErr *AkahuAPIError
	return errors.As(err, &akahuErr) && isTransientHTTPStatus(akahuErr.StatusCode)
}

// akahuErrorResponse is the JSON structure of Akahu API errors
type akahuErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// parseAkahuError parses a non-200 response into an AkahuAPIError
func parseAkahuError(resp *http.Response) error {
	body := readHTTPErrorBody(resp, "akahu")
	apiErr := &AkahuAPIError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}
	var errResp akahuErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
		apiErr.Message = errResp.Message
	} else {
		apiErr.Message = string(body)
	}
	return apiErr
}

// AkahuClient handles communication with Akahu API
type AkahuClient struct {
	cfg        *config.Config
	httpClient *http.Client
	baseURL    string
}

// NewAkahuClient creates a new Akahu API client
func NewAkahuClient(cfg *config.Config) (*AkahuClient, error) {
	if cfg.AkahuAppID == "" {
		return nil, fmt.Errorf("AKAHU_APP_ID not configured")
	}

	baseURL := cfg.AkahuBaseURL
	if baseURL == "" {
		baseURL = defaultAkahuAPIBaseURL
	}
	// Remove trailing slash if present
	baseURL = strings.TrimSuffix(baseURL, "/")

	client := &AkahuClient{
		cfg:     cfg,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	return client, nil
}

// makeRequest makes an authenticated request to the Akahu API
func (c *AkahuClient) makeRequest(ctx context.Context, method, path string, userToken string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	// Akahu uses Bearer token + X-Akahu-Id header for authentication
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("X-Akahu-Id", c.cfg.AkahuAppID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

// getJSON performs an authenticated GET request and decodes the JSON response.
// Transient server errors (5xx) and network failures are retried with exponential
// back-off (1s, 2s, 4s) to handle temporary Akahu API outages.
func (c *AkahuClient) getJSON(ctx context.Context, path, token string, v any) error {
	return retryWithBackoff(ctx, 3, func() (bool, error) {
		resp, err := c.makeRequest(ctx, "GET", path, token)
		if err != nil {
			return true, err
		}
		if resp.StatusCode != http.StatusOK {
			apiErr := parseAkahuError(resp)
			resp.Body.Close()
			return IsAkahuTransientError(apiErr), apiErr
		}
		err = json.NewDecoder(resp.Body).Decode(v)
		resp.Body.Close()
		return false, err
	})
}

// Akahu API response types

// AkahuConnection represents a bank connection in Akahu
type AkahuConnection struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
	Logo string `json:"logo"`
}

// AkahuAccount represents an account from Akahu
type AkahuAccount struct {
	ID               string          `json:"_id"`
	Connection       AkahuConnection `json:"connection"`
	Name             string          `json:"name"`
	Status           string          `json:"status"` // ACTIVE, INACTIVE
	Attributes       []string        `json:"attributes"` // TRANSACTIONS, TRANSFER_TO, etc.
	Type             string          `json:"type"`       // SAVINGS, CHECKING, CREDIT_CARD, LOAN, etc.
	FormattedAccount string          `json:"formatted_account"`
}

// AkahuMerchant represents merchant information
type AkahuMerchant struct {
	ID      string `json:"_id"`
	Name    string `json:"name"`
	Website string `json:"website,omitempty"`
	NZBN    string `json:"nzbn,omitempty"` // New Zealand Business Number
}

// AkahuCategory represents transaction category
type AkahuCategory struct {
	ID     string                 `json:"_id"`
	Name   string                 `json:"name"`
	Groups map[string]interface{} `json:"groups,omitempty"` // NZFCC category groups
}

// AkahuTransactionMeta contains additional transaction metadata
type AkahuTransactionMeta struct {
	Reference    string `json:"reference,omitempty"`
	Particulars  string `json:"particulars,omitempty"`
	Code         string `json:"code,omitempty"`
	OtherAccount string `json:"other_account,omitempty"`
	Conversion   string `json:"conversion,omitempty"`
	Logo         string `json:"logo,omitempty"`
}

// AkahuTransaction represents a transaction from Akahu
type AkahuTransaction struct {
	ID          string                `json:"_id"`
	Account     string                `json:"_account"`
	Connection  string                `json:"_connection"`
	CreatedAt   string                `json:"created_at"`
	UpdatedAt   string                `json:"updated_at"`
	Date        string                `json:"date"`
	Description string                `json:"description"`
	Amount      float64               `json:"amount"`
	Balance     float64               `json:"balance"`
	Type        string                `json:"type"` // CREDIT, DEBIT, EFTPOS, TRANSFER, etc.
	Merchant    *AkahuMerchant        `json:"merchant,omitempty"`
	Category    *AkahuCategory        `json:"category,omitempty"`
	Meta        *AkahuTransactionMeta `json:"meta,omitempty"`
	Hash        string                `json:"hash,omitempty"` // Transaction hash for deduplication
}

// AkahuAccountsResponse is the response from GET /accounts
type AkahuAccountsResponse struct {
	Success bool           `json:"success"`
	Items   []AkahuAccount `json:"items"`
}

// AkahuTransactionsResponse is the response from GET /transactions
type AkahuTransactionsResponse struct {
	Success bool               `json:"success"`
	Items   []AkahuTransaction `json:"items"`
	Cursor  *AkahuCursor       `json:"cursor,omitempty"`
}

// AkahuCursor represents pagination cursor
type AkahuCursor struct {
	Next string `json:"next,omitempty"`
}

// AkahuTokenResponse is the response from POST /token
type AkahuTokenResponse struct {
	Success      bool   `json:"success"`
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// AkahuUserResponse is the response from GET /me
type AkahuUserResponse struct {
	Success bool `json:"success"`
	Item    struct {
		ID    string `json:"_id"`
		Email string `json:"email"`
	} `json:"item"`
}

// GetAccounts fetches all accounts for an access token
func (c *AkahuClient) GetAccounts(ctx context.Context, accessToken string) ([]AkahuAccount, error) {
	var response AkahuAccountsResponse
	if err := c.getJSON(ctx, "/accounts", accessToken, &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("akahu API returned success=false")
	}
	return response.Items, nil
}

// GetTransactions fetches transactions for an account with pagination
// If accountID is empty, fetches transactions for all accounts
// startDate and endDate are optional ISO 8601 timestamps
func (c *AkahuClient) GetTransactions(ctx context.Context, accessToken string, accountID string, startDate, endDate string) ([]AkahuTransaction, error) {
	var allTransactions []AkahuTransaction
	var cursor string

	for {
		// Build URL with query params
		path := "/transactions"
		params := url.Values{}

		if accountID != "" {
			// Fetch transactions for a specific account
			path = "/accounts/" + accountID + "/transactions"
		}

		if startDate != "" {
			params.Set("start", startDate)
		}
		if endDate != "" {
			params.Set("end", endDate)
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		if len(params) > 0 {
			path += "?" + params.Encode()
		}

		var response AkahuTransactionsResponse
		if err := c.getJSON(ctx, path, accessToken, &response); err != nil {
			return nil, err
		}

		if !response.Success {
			return nil, fmt.Errorf("akahu API returned success=false")
		}

		allTransactions = append(allTransactions, response.Items...)

		// Check for more pages
		if response.Cursor == nil || response.Cursor.Next == "" {
			break
		}
		cursor = response.Cursor.Next
	}

	return allTransactions, nil
}

// GetUser fetches the authenticated user's information
func (c *AkahuClient) GetUser(ctx context.Context, accessToken string) (*AkahuUserResponse, error) {
	var response AkahuUserResponse
	if err := c.getJSON(ctx, "/me", accessToken, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// ExchangeAuthCode exchanges an authorization code for access tokens
func (c *AkahuClient) ExchangeAuthCode(ctx context.Context, code, redirectURI string) (*AkahuTokenResponse, error) {
	// Build form data
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", c.cfg.AkahuAppID)
	data.Set("client_secret", c.cfg.AkahuAppSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAkahuError(resp)
	}

	var response AkahuTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

// RevokeToken revokes an access token
func (c *AkahuClient) RevokeToken(ctx context.Context, accessToken string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/token", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Akahu-Id", c.cfg.AkahuAppID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return parseAkahuError(resp)
	}

	return nil
}

// AkahuSyncService handles syncing data from Akahu to the local database
type AkahuSyncService struct {
	pool            *pgxpool.Pool
	cfg             *config.Config
	client          *AkahuClient
	accounts        *models.AccountStore
	transactions    *models.TransactionStore
	entities        *models.EntityStore
	transferMatcher *TransferMatcher
}

// NewAkahuSyncService creates a new Akahu sync service
func NewAkahuSyncService(pool *pgxpool.Pool, client *AkahuClient, cfg *config.Config) *AkahuSyncService {
	return &AkahuSyncService{
		pool:            pool,
		cfg:             cfg,
		client:          client,
		accounts:        models.NewAccountStore(pool),
		transactions:    models.NewTransactionStore(pool),
		entities:        models.NewEntityStore(pool),
		transferMatcher: NewTransferMatcher(pool),
	}
}

// SyncAccounts syncs accounts from Akahu for a user
func (s *AkahuSyncService) SyncAccounts(ctx context.Context, ledgerID uuid.UUID, accessToken string) ([]*models.Account, error) {
	slog.DebugContext(ctx, "AkahuSyncAccounts: starting", "ledger_id", ledgerID)

	akahuAccounts, err := s.client.GetAccounts(ctx, accessToken)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch Akahu accounts", "err", err)
		return nil, err
	}
	slog.DebugContext(ctx, "AkahuSyncAccounts: fetched accounts from Akahu", "count", len(akahuAccounts))

	var synced []*models.Account
	var errs []error
	now := time.Now()

	for _, aa := range akahuAccounts {
		// Check if account already exists by Akahu account ID
		existing, err := s.accounts.GetByExternalAccountID(ctx, aa.ID)
		if err == nil && existing != nil {
			// Update existing account
			existing.InstitutionName = aa.Connection.Name
			existing.InstitutionID = aa.Connection.ID
			existing.AccountSubtype = strings.ToLower(aa.Type)
			existing.AccountStatus = strings.ToLower(aa.Status)
			existing.LastSyncedAt = &now

			// Update institution logo if available
			if aa.Connection.Logo != "" && existing.InstitutionLogoURL == "" {
				existing.InstitutionLogoURL = aa.Connection.Logo
			}

			if err := s.accounts.Update(ctx, existing); err != nil {
				slog.WarnContext(ctx, "failed to update existing Akahu account", "akahu_id", aa.ID, "err", err)
				errs = append(errs, fmt.Errorf("failed to update account %s: %w", aa.ID, err))
				continue
			}
			synced = append(synced, existing)
			continue
		}

		// Try to find by formatted account number (NZ bank account format)
		if aa.FormattedAccount != "" {
			existing, err = s.findAccountByAccountNumber(ctx, ledgerID, aa.FormattedAccount)
			if err == nil && existing != nil {
				slog.DebugContext(ctx, "AkahuSyncAccounts: reconnecting existing account by account number", "name", existing.Name, "akahu_id", aa.ID)

				// Reconnect: Update with new Akahu credentials
				existing.Provider = "akahu"
				existing.ExternalAccountID = aa.ID
				existing.ConnectionID = aa.Connection.ID
				existing.AccessToken = accessToken
				existing.InstitutionName = aa.Connection.Name
				existing.InstitutionID = aa.Connection.ID
				existing.AccountSubtype = strings.ToLower(aa.Type)
				existing.AccountStatus = strings.ToLower(aa.Status)
				existing.AccountNumberMasked = aa.FormattedAccount
				existing.LastSyncedAt = &now

				if aa.Connection.Logo != "" {
					existing.InstitutionLogoURL = aa.Connection.Logo
				}

				if err := s.accounts.Update(ctx, existing); err != nil {
					slog.WarnContext(ctx, "failed to update reconnected Akahu account", "akahu_id", aa.ID, "err", err)
					errs = append(errs, fmt.Errorf("failed to update reconnected account %s: %w", aa.ID, err))
					continue
				}
				synced = append(synced, existing)
				continue
			}
		}

		// Create new account
		acc := &models.Account{
			LedgerID:            ledgerID,
			Name:                aa.Name,
			Type:                mapAkahuAccountType(aa.Type),
			Provider:            "akahu",
			ExternalAccountID:   aa.ID,
			ConnectionID:        aa.Connection.ID,
			AccessToken:         accessToken,
			InstitutionName:     aa.Connection.Name,
			InstitutionID:       aa.Connection.ID,
			InstitutionLogoURL:  aa.Connection.Logo,
			AccountSubtype:      strings.ToLower(aa.Type),
			AccountStatus:       strings.ToLower(aa.Status),
			AccountNumberMasked: aa.FormattedAccount,
			LastSyncedAt:        &now,
			IsActive:            true,
		}

		// Extract last four digits from formatted account if possible
		if aa.FormattedAccount != "" {
			parts := strings.Split(aa.FormattedAccount, "-")
			if len(parts) >= 3 {
				// NZ format: BB-bbbb-AAAAAAA-SS (bank-branch-account-suffix)
				accountNum := parts[len(parts)-2] // Get account number part
				if len(accountNum) >= 4 {
					acc.LastFour = accountNum[len(accountNum)-4:]
				}
			}
		}

		if err := s.accounts.Create(ctx, acc); err != nil {
			slog.ErrorContext(ctx, "failed to create Akahu account", "akahu_id", aa.ID, "err", err)
			errs = append(errs, fmt.Errorf("failed to create account %s: %w", aa.ID, err))
			continue
		}
		synced = append(synced, acc)
	}

	slog.DebugContext(ctx, "AkahuSyncAccounts: completed", "synced", len(synced), "errors", len(errs))
	return synced, errors.Join(errs...)
}

// SyncTransactions syncs transactions from Akahu for an account
func (s *AkahuSyncService) SyncTransactions(ctx context.Context, account *models.Account) (int, error) {
	slog.DebugContext(ctx, "AkahuSyncTransactions: starting", "account", account.Name, "account_id", account.ID)

	// Get transactions from the last 90 days by default
	// If this is a fresh sync (no transactions), get all available
	startDate := time.Now().AddDate(0, 0, -90).Format(time.RFC3339)

	akahuTxns, err := s.client.GetTransactions(ctx, account.AccessToken, account.ExternalAccountID, startDate, "")
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch Akahu transactions", "account", account.Name, "err", err)
		now := time.Now()
		account.LastSyncedAt = &now
		if updateErr := s.accounts.Update(ctx, account); updateErr != nil {
			slog.WarnContext(ctx, "failed to update account last_synced_at after fetch error", "account", account.Name, "err", updateErr)
		}
		return 0, err
	}
	slog.DebugContext(ctx, "AkahuSyncTransactions: fetched transactions from Akahu", "count", len(akahuTxns), "account", account.Name)

	expenseAccount, incomeAccount, err := getContraAccounts(ctx, s.accounts, account.LedgerID)
	if err != nil {
		return 0, err
	}

	var created, storeErrors int
	var newTxnIDs []uuid.UUID

	for _, at := range akahuTxns {
		// Check if transaction already exists by Akahu ID
		// Uses the teller_transaction_id field which is used for all provider transaction IDs
		existing, err := s.transactions.GetByTellerID(ctx, at.ID)
		if err == nil && existing != nil {
			continue // Skip existing
		}

		// Parse date
		date, err := time.Parse("2006-01-02", at.Date)
		if err != nil {
			// Try ISO 8601 format
			date, err = time.Parse(time.RFC3339, at.Date)
			if err != nil {
				date = time.Now().UTC()
			}
		}

		// Convert amount to cents (Akahu uses NZD with decimals)
		amountCents := dollarsToCents(at.Amount)
		runningBalanceCents := dollarsToCents(at.Balance)

		// Check for duplicates by date/description/amount
		exists, err := s.transactions.ExistsByDateDescriptionAmount(ctx, account.ID, date, at.Description, amountCents)
		if err == nil && exists {
			slog.DebugContext(ctx, "skipping duplicate Akahu transaction by content", "description", at.Description, "date", at.Date, "amount_cents", amountCents)
			continue
		}

		// Determine contra account
		var contraAccountID uuid.UUID
		isExpense := amountCents < 0
		if account.Type == models.AccountTypeLiability {
			isExpense = amountCents > 0
		}

		// Special case: Check transaction type for refunds
		if strings.EqualFold(at.Type, "CREDIT") && amountCents > 0 && account.Type == models.AccountTypeLiability {
			isExpense = true // Credit card credit = likely refund
		}

		if isExpense {
			contraAccountID = expenseAccount.ID
		} else {
			contraAccountID = incomeAccount.ID
		}

		// Extract category from Akahu if available
		var akahuCategory string
		if at.Category != nil {
			akahuCategory = at.Category.Name
		}

		// Extract counterparty from merchant info
		// Akahu provides pre-enriched merchant data including name, website, and NZBN
		var counterpartyName, counterpartyType string
		var merchantWebsite, merchantNZBN string
		if at.Merchant != nil {
			counterpartyName = at.Merchant.Name
			counterpartyType = "organization"
			merchantWebsite = at.Merchant.Website
			merchantNZBN = at.Merchant.NZBN
		}

		// Use display title from merchant name if available (Akahu enrichment)
		displayTitle := ""
		if counterpartyName != "" {
			displayTitle = counterpartyName
		}

		// Create transaction
		// Note: TellerTransactionID is used generically for all provider transaction IDs
		txn := &models.Transaction{
			LedgerID:             account.LedgerID,
			Date:                 date,
			Description:          at.Description,
			DisplayTitle:         displayTitle,
			TellerTransactionID:  at.ID, // External transaction ID
			TellerType:           at.Type,
			TellerCategory:       akahuCategory,
			TellerStatus:         "posted", // Akahu doesn't distinguish pending
			CounterpartyName:     counterpartyName,
			CounterpartyType:     counterpartyType,
			RunningBalanceCents:  runningBalanceCents,
			CategorizationStatus: models.CategorizationStatusPending,
		}

		// If we have merchant data from Akahu, try to find or create an entity
		// This gives us better enrichment data from Akahu's Genie service
		if counterpartyName != "" {
			entity, err := s.findOrCreateEntityFromAkahu(ctx, account.LedgerID, counterpartyName, merchantWebsite, merchantNZBN)
			if err == nil && entity != nil {
				txn.EntityID = &entity.ID
			}
		}

		entries := []*models.Entry{
			{AccountID: account.ID, AmountCents: amountCents, Currency: "NZD"},
			{AccountID: contraAccountID, AmountCents: -amountCents, Currency: "NZD"},
		}

		if err := s.transactions.CreateWithEntries(ctx, txn, entries); err != nil {
			// Log and skip — a single bad row (e.g., constraint violation) should not
			// abort the entire sync and leave newer transactions unimported.
			slog.WarnContext(ctx, "failed to store akahu transaction, skipping", "akahu_id", at.ID, "description", at.Description, "err", err)
			storeErrors++
			continue
		}
		created++
		newTxnIDs = append(newTxnIDs, txn.ID)

		if err := s.transferMatcher.ProcessNewTransaction(ctx, txn, entries[0]); err != nil {
			logTransferMatchingError(ctx, "sync_akahu", err, txn.ID)
		}
	}
	if storeErrors > 0 {
		storeErr := fmt.Errorf("failed to store %d of %d transactions", storeErrors, len(akahuTxns))
		slog.WarnContext(ctx, "sync completed with store failures", "account_id", account.ID, "store_errors", storeErrors, "created", created)
		observability.CaptureFailure(ctx, storeErr, observability.FailureOptions{
			Component: "sync_akahu",
			Operation: "store_transaction",
			Tags:      map[string]string{"account_id": account.ID.String()},
		})
	}

	updateLastSyncedAt(ctx, account, s.accounts)

	// Queue new transactions for enrichment
	if len(newTxnIDs) > 0 {
		if err := s.transactions.QueueForEnrichment(ctx, newTxnIDs); err != nil {
			slog.WarnContext(ctx, "failed to queue Akahu transactions for enrichment", "err", err)
		} else {
			slog.DebugContext(ctx, "queued Akahu transactions for enrichment", "count", len(newTxnIDs))
		}
	}

	slog.DebugContext(ctx, "AkahuSyncTransactions: completed", "account", account.Name, "created", created)
	return created, nil
}

// findAccountByAccountNumber finds an account by its masked account number
func (s *AkahuSyncService) findAccountByAccountNumber(ctx context.Context, ledgerID uuid.UUID, accountNumber string) (*models.Account, error) {
	accounts, err := s.accounts.GetByLedgerID(ctx, ledgerID)
	if err != nil {
		return nil, err
	}

	for _, acc := range accounts {
		if acc.AccountNumberMasked == accountNumber {
			return acc, nil
		}
	}

	return nil, fmt.Errorf("account not found")
}

// mapAkahuAccountType maps Akahu account types to internal account types
func mapAkahuAccountType(akahuType string) models.AccountType {
	switch strings.ToUpper(akahuType) {
	case "SAVINGS", "CHECKING", "TRANSACTION", "TERMDEPOSIT", "KIWISAVER", "INVESTMENT":
		return models.AccountTypeAsset
	case "CREDIT_CARD", "CREDITCARD", "LOAN", "MORTGAGE", "OVERDRAFT":
		return models.AccountTypeLiability
	default:
		return models.AccountTypeAsset
	}
}

// findOrCreateEntityFromAkahu finds or creates an entity from Akahu merchant data
// Akahu provides pre-enriched merchant data including name, website, and NZBN
func (s *AkahuSyncService) findOrCreateEntityFromAkahu(ctx context.Context, ledgerID uuid.UUID, merchantName, website, nzbn string) (*models.Entity, error) {
	if merchantName == "" {
		return nil, nil
	}

	// First, try to find by exact name
	existing, err := s.entities.GetByName(ctx, merchantName)
	if err == nil && existing != nil {
		// Update with Akahu data if we have more info
		updated := false
		if website != "" && existing.Website == "" {
			existing.Website = website
			updated = true
		}
		// Store NZBN in metadata if provided
		if nzbn != "" && existing.Metadata == nil {
			metadata := map[string]interface{}{"nzbn": nzbn, "source": "akahu"}
			if metaJSON, err := json.Marshal(metadata); err == nil {
				existing.Metadata = metaJSON
				updated = true
			}
		}
		if updated {
			if err := s.entities.Update(ctx, existing); err != nil {
				slog.WarnContext(ctx, "akahu: failed to update entity metadata", "entity_id", existing.ID, "err", err)
			}
		}
		return existing, nil
	}

	// Build metadata with Akahu-specific data
	var metadata json.RawMessage
	if nzbn != "" {
		metaMap := map[string]interface{}{
			"nzbn":   nzbn,
			"source": "akahu",
		}
		if metaJSON, err := json.Marshal(metaMap); err == nil {
			metadata = metaJSON
		}
	}

	// Create new entity from Akahu merchant data
	entity := &models.Entity{
		Name:           merchantName,
		Slug:           models.Slugify(merchantName),
		Type:           models.EntityTypeBusiness,
		Website:        website,
		Metadata:       metadata,
		ExternalSource: "akahu",
	}

	if err := s.entities.Create(ctx, entity); err != nil {
		// If creation fails (e.g., race condition), try to find again
		existing, findErr := s.entities.GetByName(ctx, merchantName)
		if findErr == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}

	slog.DebugContext(ctx, "AkahuSyncService: created entity from Akahu merchant data", "name", merchantName, "website", website, "nzbn", nzbn)
	return entity, nil
}

// SyncAccountsWithToken implements providers.ProviderImpl.
func (s *AkahuSyncService) SyncAccountsWithToken(ctx context.Context, ledgerID uuid.UUID, accessToken string) ([]*models.Account, error) {
	return s.SyncAccounts(ctx, ledgerID, accessToken)
}

// SyncTransactionsForAccount implements providers.ProviderImpl.
func (s *AkahuSyncService) SyncTransactionsForAccount(ctx context.Context, account *models.Account) (int, error) {
	return s.SyncTransactions(ctx, account)
}
