package sync

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
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
)

const tellerAPIBaseURL = "https://api.teller.io"

// TellerAPIError represents a structured error from the Teller API
type TellerAPIError struct {
	StatusCode int
	Status     string
	Code       string
	Message    string
}

func (e *TellerAPIError) Error() string {
	return fmt.Sprintf("teller API error: %s - %s (code: %s)", e.Status, e.Message, e.Code)
}

// IsConnectionDisconnected returns true if this error indicates the enrollment
// needs re-authentication (e.g., MFA required, credentials expired, access revoked).
// 401 = invalid token. 403 = enrollment requires action or access has been revoked.
func (e *TellerAPIError) IsConnectionDisconnected() bool {
	return e.StatusCode == http.StatusUnauthorized ||
		e.StatusCode == http.StatusForbidden ||
		strings.HasPrefix(e.Code, "enrollment.disconnected") ||
		e.IsUnableToProcess()
}

// IsUnableToProcess returns true if Teller responded with an "unable to process"
// message, which indicates the account needs re-authentication.
func (e *TellerAPIError) IsUnableToProcess() bool {
	return strings.Contains(strings.ToLower(e.Message), "unable to process")
}

// Provider returns the provider name for this error
func (e *TellerAPIError) Provider() providers.ProviderName {
	return providers.ProviderTeller
}

// IsTellerTransientError reports whether err is a transient Teller server error (5xx or 429).
// Transient errors reflect a provider-side outage or rate limit, not an application bug. Callers
// should log a warning and allow the next sync cycle to retry rather than capturing
// to the error tracker.
func IsTellerTransientError(err error) bool {
	var tellerErr *TellerAPIError
	return errors.As(err, &tellerErr) && isTransientHTTPStatus(tellerErr.StatusCode)
}

// handleBalanceReconciliationErr handles an error from opening balance reconciliation.
// Returns true if the caller should abort (connection disconnected), false otherwise.
// TellerAPIErrors are treated as provider-side issues and logged as warnings without capture.
func handleBalanceReconciliationErr(ctx context.Context, err error, accountID uuid.UUID, op string) bool {
	if providers.IsConnectionDisconnectedError(err) {
		return true
	}
	var tellerErr *TellerAPIError
	if errors.As(err, &tellerErr) {
		// Any TellerAPIError (4xx or 5xx) is a provider-side issue — not an application bug.
		// 4xx often means the account type doesn't support the balance endpoint.
		// Balance will reconcile on the next sync cycle.
		slog.WarnContext(ctx, "teller error reconciling balance, will retry next cycle", "account_id", accountID, "status_code", tellerErr.StatusCode, "err", tellerErr)
	} else {
		slog.WarnContext(ctx, "failed to reconcile opening balance", "account_id", accountID, "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "sync_teller",
			Operation: op,
			Tags:      map[string]string{"account_id": accountID.String()},
		})
	}
	return false
}

// tellerErrorResponse is the JSON structure of Teller API errors
type tellerErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// parseTellerError parses a non-200 response into a TellerAPIError
func parseTellerError(resp *http.Response) error {
	body := readHTTPErrorBody(resp, "teller")
	apiErr := &TellerAPIError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}
	var errResp tellerErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Code != "" {
		apiErr.Code = errResp.Error.Code
		apiErr.Message = errResp.Error.Message
	} else {
		apiErr.Message = string(body)
	}
	return apiErr
}

// TellerClient handles communication with Teller API
type TellerClient struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewTellerClient creates a new Teller API client
func NewTellerClient(cfg *config.Config) (*TellerClient, error) {
	client := &TellerClient{cfg: cfg}

	// If we have certificates, set up mutual TLS
	if cfg.TellerCert != "" && cfg.TellerKey != "" {
		var certPEM, keyPEM []byte
		var err error

		// Trim whitespace from certificate values
		certValue := strings.TrimSpace(cfg.TellerCert)
		keyValue := strings.TrimSpace(cfg.TellerKey)

		// Try base64 decoding first (for environment variables)
		// Base64-encoded certificates are typically 1000+ characters
		certPEM, certErr := base64.StdEncoding.DecodeString(certValue)
		keyPEM, keyErr := base64.StdEncoding.DecodeString(keyValue)

		// If base64 decoding fails, try reading as file paths
		if certErr != nil || keyErr != nil {
			// Check if the values look like file paths
			if len(certValue) < 500 && (strings.HasPrefix(certValue, "/") || strings.HasPrefix(certValue, "./") || strings.HasPrefix(certValue, ".secure/")) {
				// Looks like a file path - read from file
				certPEM, err = os.ReadFile(certValue)
				if err != nil {
					return nil, fmt.Errorf("failed to read certificate file: %w", err)
				}
				keyPEM, err = os.ReadFile(keyValue)
				if err != nil {
					return nil, fmt.Errorf("failed to read key file: %w", err)
				}
			} else {
				// Not a valid base64 string and not a file path
				if certErr != nil {
					return nil, fmt.Errorf("failed to decode certificate (not valid base64, length=%d): %w", len(certValue), certErr)
				}
				return nil, fmt.Errorf("failed to decode key (not valid base64, length=%d): %w", len(keyValue), keyErr)
			}
		}

		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		client.httpClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
			Timeout:   30 * time.Second,
		}
	} else {
		client.httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return client, nil
}

// Teller API response types

type TellerAccount struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Subtype      string            `json:"subtype"`
	Status       string            `json:"status"`
	Currency     string            `json:"currency"`
	EnrollmentID string            `json:"enrollment_id"`
	Institution  TellerInstitution `json:"institution"`
	LastFour     string            `json:"last_four"`
}

type TellerInstitution struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TellerBalance struct {
	AccountID string `json:"account_id"`
	Ledger    string `json:"ledger"`
	Available string `json:"available"`
}

// TellerAccountDetails contains sensitive account information (account/routing numbers)
type TellerAccountDetails struct {
	AccountID      string               `json:"account_id"`
	AccountNumber  string               `json:"account_number"`
	RoutingNumbers TellerRoutingNumbers `json:"routing_numbers"`
}

type TellerRoutingNumbers struct {
	ACH  string `json:"ach"`
	Wire string `json:"wire"`
	BACS string `json:"bacs"`
}

// TellerIdentity contains beneficial owner information for an account
type TellerIdentity struct {
	AccountID string        `json:"account_id"`
	Owners    []TellerOwner `json:"owners"`
}

type TellerOwner struct {
	Name         string          `json:"name"`
	DateOfBirth  string          `json:"date_of_birth"`
	Addresses    []TellerAddress `json:"addresses"`
	PhoneNumbers []TellerPhone   `json:"phone_numbers"`
	Emails       []TellerEmail   `json:"emails"`
}

type TellerAddress struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	Region     string `json:"region"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
	Type       string `json:"type"` // "primary", "secondary", etc.
}

type TellerPhone struct {
	Number string `json:"number"`
	Type   string `json:"type"` // "mobile", "home", "work"
}

type TellerEmail struct {
	Address string `json:"address"`
	Type    string `json:"type"` // "primary", "secondary"
}

type TellerTransaction struct {
	ID               string                   `json:"id"`
	AccountID        string                   `json:"account_id"`
	Date             string                   `json:"date"`
	Description      string                   `json:"description"`
	Details          TellerTransactionDetails `json:"details"`
	Amount           string                   `json:"amount"`
	RunningBalance   string                   `json:"running_balance"`
	Status           string                   `json:"status"`
	Type             string                   `json:"type"`
	ProcessingStatus string                   `json:"processing_status"`
}

type TellerTransactionDetails struct {
	Category         string             `json:"category"`
	Counterparty     TellerCounterparty `json:"counterparty"`
	ProcessingStatus string             `json:"processing_status"`
}

type TellerCounterparty struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// get performs an authenticated GET to path and JSON-decodes the response into v.
// Transient server errors (5xx) and network failures are retried with exponential
// back-off (1s, 2s, 4s) to handle temporary Teller API outages. Non-transient
// errors (4xx, auth failures, disconnected enrollments) are returned immediately.
func (c *TellerClient) get(ctx context.Context, path, accessToken string, v any) error {
	return retryWithBackoff(ctx, 3, func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", tellerAPIBaseURL+path, nil)
		if err != nil {
			return false, err
		}
		req.SetBasicAuth(accessToken, "")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return true, err
		}
		if resp.StatusCode != http.StatusOK {
			apiErr := parseTellerError(resp)
			resp.Body.Close()
			return IsTellerTransientError(apiErr), apiErr
		}
		err = json.NewDecoder(resp.Body).Decode(v)
		resp.Body.Close()
		return false, err
	})
}

// GetAccounts fetches all accounts for an access token
func (c *TellerClient) GetAccounts(ctx context.Context, accessToken string) ([]TellerAccount, error) {
	var accounts []TellerAccount
	if err := c.get(ctx, "/accounts", accessToken, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

// GetAccountBalances fetches balances for an account
func (c *TellerClient) GetAccountBalances(ctx context.Context, accessToken, accountID string) (*TellerBalance, error) {
	var balance TellerBalance
	if err := c.get(ctx, "/accounts/"+accountID+"/balances", accessToken, &balance); err != nil {
		return nil, err
	}
	return &balance, nil
}

// GetTransactions fetches transactions for an account
func (c *TellerClient) GetTransactions(ctx context.Context, accessToken, accountID string) ([]TellerTransaction, error) {
	var transactions []TellerTransaction
	if err := c.get(ctx, "/accounts/"+accountID+"/transactions", accessToken, &transactions); err != nil {
		return nil, err
	}
	return transactions, nil
}

// GetAccountDetails fetches sensitive account details (account number, routing numbers)
func (c *TellerClient) GetAccountDetails(ctx context.Context, accessToken, accountID string) (*TellerAccountDetails, error) {
	var details TellerAccountDetails
	if err := c.get(ctx, "/accounts/"+accountID+"/details", accessToken, &details); err != nil {
		return nil, err
	}
	return &details, nil
}

// GetIdentity fetches beneficial owner information for an account
func (c *TellerClient) GetIdentity(ctx context.Context, accessToken, accountID string) (*TellerIdentity, error) {
	var identity TellerIdentity
	if err := c.get(ctx, "/accounts/"+accountID+"/identity", accessToken, &identity); err != nil {
		return nil, err
	}
	return &identity, nil
}

// TellerSyncService handles syncing data from Teller to the local database
type TellerSyncService struct {
	pool            *pgxpool.Pool
	cfg             *config.Config
	client          *TellerClient
	accounts        *models.AccountStore
	transactions    *models.TransactionStore
	accountOwners   *models.AccountOwnerStore
	transferMatcher *TransferMatcher
}

func NewTellerSyncService(pool *pgxpool.Pool, client *TellerClient, cfg *config.Config) *TellerSyncService {
	return &TellerSyncService{
		pool:            pool,
		cfg:             cfg,
		client:          client,
		accounts:        models.NewAccountStore(pool),
		transactions:    models.NewTransactionStore(pool),
		accountOwners:   models.NewAccountOwnerStore(pool),
		transferMatcher: NewTransferMatcher(pool),
	}
}

// applyTellerCredentials sets all Teller-specific and generic provider fields on an account.
// Use when creating a new account or fully reconnecting an existing one with fresh credentials.
func applyTellerCredentials(acc *models.Account, ta TellerAccount, accessToken string, now time.Time) {
	acc.TellerAccountID = ta.ID
	acc.TellerEnrollmentID = ta.EnrollmentID
	acc.TellerAccessToken = accessToken
	acc.Provider = "teller"
	acc.ExternalAccountID = ta.ID
	acc.ConnectionID = ta.EnrollmentID
	acc.AccessToken = accessToken
	acc.InstitutionName = ta.Institution.Name
	acc.InstitutionID = ta.Institution.ID
	acc.TellerSubtype = ta.Subtype
	acc.TellerStatus = ta.Status
	acc.AccountSubtype = ta.Subtype
	acc.AccountStatus = ta.Status
	acc.LastFour = ta.LastFour
	acc.LastSyncedAt = &now
}

// SyncAccounts syncs accounts from Teller for an enrollment
func (s *TellerSyncService) SyncAccounts(ctx context.Context, ledgerID uuid.UUID, accessToken string) ([]*models.Account, error) {
	slog.DebugContext(ctx, "SyncAccounts: starting", "ledger_id", ledgerID)

	tellerAccounts, err := s.client.GetAccounts(ctx, accessToken)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch Teller accounts", "err", err)
		return nil, err
	}
	slog.DebugContext(ctx, "SyncAccounts: fetched accounts from Teller", "count", len(tellerAccounts))

	var synced []*models.Account
	var errs []error
	now := time.Now()
	for _, ta := range tellerAccounts {
		slog.DebugContext(ctx, "SyncAccounts: processing account", "teller_id", ta.ID, "name", ta.Name, "institution", ta.Institution.Name)

		// Check if account already exists by Teller account ID
		existing, err := s.accounts.GetByExternalAccountID(ctx, ta.ID)
		if err == nil && existing != nil {
			slog.DebugContext(ctx, "SyncAccounts: found existing account by ID", "name", existing.Name, "account_id", existing.ID)
			// Update existing with Teller data, but preserve user-set name
			// Note: We intentionally don't overwrite existing.Name to preserve user customizations
			existing.InstitutionName = ta.Institution.Name
			existing.InstitutionID = ta.Institution.ID
			existing.TellerSubtype = ta.Subtype
			existing.TellerStatus = ta.Status
			existing.LastFour = ta.LastFour
			// Also update generic provider fields (ensure consistency)
			if existing.Provider == "" {
				existing.Provider = "teller"
			}
			if existing.ExternalAccountID == "" {
				existing.ExternalAccountID = ta.ID
			}
			if existing.ConnectionID == "" {
				existing.ConnectionID = ta.EnrollmentID
			}
			if existing.AccessToken == "" {
				existing.AccessToken = accessToken
			}
			if existing.AccountSubtype == "" {
				existing.AccountSubtype = ta.Subtype
			}
			if existing.AccountStatus == "" {
				existing.AccountStatus = ta.Status
			}
			existing.LastSyncedAt = &now

			// Fetch and update account details (routing numbers, etc.)
			s.syncAccountDetails(ctx, existing, accessToken)

			if err := s.accounts.Update(ctx, existing); err != nil {
				slog.WarnContext(ctx, "failed to update reconnected account", "teller_id", ta.ID, "err", err)
				errs = append(errs, fmt.Errorf("failed to update reconnected account %s: %w", ta.ID, err))
				continue // Continue with other accounts instead of returning
			}

			// Sync identity/owner information
			s.syncAccountIdentity(ctx, existing, accessToken)

			synced = append(synced, existing)
			slog.DebugContext(ctx, "SyncAccounts: successfully reconnected account", "teller_id", ta.ID)
			continue
		}

		// Fallback 1: Try to find a disconnected account by last four digits + institution
		// This is more robust than name matching since users often rename accounts
		if ta.LastFour != "" {
			existing, err = s.accounts.FindByLastFourAndInstitution(ctx, ledgerID, ta.LastFour, ta.Institution.Name)
			if err == nil && existing != nil {
				slog.DebugContext(ctx, "SyncAccounts: reconnecting existing account by last four", "name", existing.Name, "account_id", existing.ID, "teller_id", ta.ID, "last_four", ta.LastFour)
			} else {
				slog.DebugContext(ctx, "SyncAccounts: no account found by last four and institution", "last_four", ta.LastFour, "institution", ta.Institution.Name, "err", err)
			}
		} else {
			slog.DebugContext(ctx, "SyncAccounts: Teller account has no last four digits, skipping last four match", "teller_id", ta.ID)
		}

		// Fallback 2: Try to find by name + institution (less reliable if user renamed)
		if existing == nil {
			existing, err = s.accounts.FindByNameAndInstitution(ctx, ledgerID, ta.Name, ta.Institution.Name)
			if err == nil && existing != nil {
				slog.DebugContext(ctx, "SyncAccounts: reconnecting existing account by name", "name", existing.Name, "account_id", existing.ID, "teller_id", ta.ID, "teller_name", ta.Name)
			} else {
				slog.DebugContext(ctx, "SyncAccounts: no account found by name and institution", "name", ta.Name, "institution", ta.Institution.Name, "err", err)
			}
		}

		if err == nil && existing != nil {
			slog.DebugContext(ctx, "SyncAccounts: reconnecting existing account to Teller ID", "name", existing.Name, "account_id", existing.ID, "teller_id", ta.ID)

			applyTellerCredentials(existing, ta, accessToken, now)
			existing.ConnectionStatus = ""

			// Fetch and update account details (routing numbers, etc.)
			s.syncAccountDetails(ctx, existing, accessToken)

			if err := s.accounts.Update(ctx, existing); err != nil {
				slog.WarnContext(ctx, "failed to update reconnected account", "teller_id", ta.ID, "err", err)
				errs = append(errs, fmt.Errorf("failed to update reconnected account %s: %w", ta.ID, err))
				continue // Continue with other accounts instead of returning
			}

			// Sync identity/owner information
			s.syncAccountIdentity(ctx, existing, accessToken)

			synced = append(synced, existing)
			slog.DebugContext(ctx, "SyncAccounts: successfully reconnected account", "teller_id", ta.ID)
			continue
		}

		// Create new account with all available data
		slog.DebugContext(ctx, "SyncAccounts: creating new account", "teller_id", ta.ID, "name", ta.Name, "institution", ta.Institution.Name)
		acc := &models.Account{
			LedgerID: ledgerID,
			Name:     ta.Name,
			Type:     mapTellerAccountType(ta.Type),
			IsActive: true,
		}
		applyTellerCredentials(acc, ta, accessToken, now)

		// Fetch account details before creating
		s.syncAccountDetails(ctx, acc, accessToken)

		if err := s.accounts.Create(ctx, acc); err != nil {
			slog.ErrorContext(ctx, "failed to create Teller account", "teller_id", ta.ID, "err", err)
			errs = append(errs, fmt.Errorf("failed to create account %s: %w", ta.ID, err))
			continue // Continue with other accounts instead of returning
		}

		// Sync identity/owner information
		s.syncAccountIdentity(ctx, acc, accessToken)

		synced = append(synced, acc)
		slog.DebugContext(ctx, "SyncAccounts: successfully created new account", "teller_id", ta.ID)
	}

	// Fetch logos for any institutions that don't have Teller logos
	s.syncInstitutionLogos(ctx, synced)

	slog.DebugContext(ctx, "SyncAccounts: completed", "synced", len(synced), "total", len(tellerAccounts))
	return synced, errors.Join(errs...)
}

// syncInstitutionLogos fetches logos for institutions that don't have Teller-provided logos
func (s *TellerSyncService) syncInstitutionLogos(ctx context.Context, accounts []*models.Account) {
	if s.cfg == nil || s.cfg.FirecrawlAPIKey == "" {
		return
	}

	// Group accounts by institution to avoid duplicate fetches
	institutionAccounts := make(map[string][]*models.Account)
	for _, acc := range accounts {
		// Skip only if we already have a stored logo.
		// Teller-hosted fallback logo URLs are frequently unavailable (404),
		// so InstitutionID alone is not sufficient.
		if acc.InstitutionLogoURL != "" {
			continue
		}
		if acc.InstitutionName != "" {
			institutionAccounts[acc.InstitutionName] = append(institutionAccounts[acc.InstitutionName], acc)
		}
	}

	if len(institutionAccounts) == 0 {
		return
	}

	// Initialize Firecrawl cache
	firecrawlCache := enrichment.NewFirecrawlCache(s.pool)
	firecrawl := enrichment.NewFirecrawlClientWithCache(s.cfg, firecrawlCache)

	// Initialize cloud storage for logos (required - no local fallback)
	storageCtx := context.Background()
	storageInstance, err := storage.NewStorageFromEnv(storageCtx, s.cfg.BaseURL)
	if err != nil {
		slog.WarnContext(ctx, "failed to initialize storage for logos", "err", err)
		// Continue without logo storage - logos just won't be downloaded
		storageInstance = nil
	}

	var logoStore *enrichment.LogoStore
	if storageInstance != nil {
		logoStore = enrichment.NewLogoStore(storageInstance, "", "")
	}

	for institutionName, accs := range institutionAccounts {
		slog.DebugContext(ctx, "Fetching logo for institution", "institution", institutionName)

		// Search for the institution website
		info, err := firecrawl.SearchAndExtract(ctx, institutionName+" bank", "", "")
		if err != nil {
			slog.WarnContext(ctx, "could not fetch logo", "institution", institutionName, "err", err)
			continue
		}

		if info == nil || info.LogoURL == "" {
			slog.WarnContext(ctx, "no logo found", "institution", institutionName)
			continue
		}

		// Download and store the logo locally
		if logoStore == nil {
			slog.WarnContext(ctx, "logo store not available, skipping logo", "institution", institutionName)
			continue
		}

		localPath, downloadErr := logoStore.DownloadAndStore(ctx, info.LogoURL)
		if downloadErr != nil {
			slog.WarnContext(ctx, "could not download logo", "institution", institutionName, "err", downloadErr)
			continue
		}

		if localPath == "" {
			slog.WarnContext(ctx, "empty logo path returned", "institution", institutionName)
			continue
		}

		slog.DebugContext(ctx, "Downloaded logo for institution", "institution", institutionName, "path", localPath)

		// Update all accounts for this institution with the logo
		for _, acc := range accs {
			acc.InstitutionLogoURL = localPath
			if err := s.accounts.Update(ctx, acc); err != nil {
				slog.WarnContext(ctx, "could not update logo for account", "account_id", acc.ID, "err", err)
			}
		}
	}
}


// syncAccountDetails fetches and stores account details (routing numbers, masked account number)
func (s *TellerSyncService) syncAccountDetails(ctx context.Context, account *models.Account, accessToken string) {
	details, err := s.client.GetAccountDetails(ctx, accessToken, account.TellerAccountID)
	if err != nil {
		// Log but don't fail - details may not be available for all account types
		slog.WarnContext(ctx, "could not fetch account details", "teller_id", account.TellerAccountID, "err", err)
		return
	}

	// Mask account number (show last 4 only)
	if len(details.AccountNumber) > 4 {
		account.AccountNumberMasked = "****" + details.AccountNumber[len(details.AccountNumber)-4:]
	} else if details.AccountNumber != "" {
		account.AccountNumberMasked = details.AccountNumber
	}

	account.RoutingNumberACH = details.RoutingNumbers.ACH
	account.RoutingNumberWire = details.RoutingNumbers.Wire
}

// syncAccountIdentity fetches and atomically replaces beneficial owner information.
func (s *TellerSyncService) syncAccountIdentity(ctx context.Context, account *models.Account, accessToken string) {
	identity, err := s.client.GetIdentity(ctx, accessToken, account.TellerAccountID)
	if err != nil {
		// Identity may not be available for all accounts
		slog.WarnContext(ctx, "could not fetch identity", "teller_id", account.TellerAccountID, "err", err)
		return
	}

	owners := buildAccountOwners(account.ID, identity.Owners)
	if err := s.accountOwners.ReplaceForAccount(ctx, account.ID, owners); err != nil {
		slog.WarnContext(ctx, "failed to replace account owners", "teller_id", account.TellerAccountID, "err", err)
	}
}

// buildAccountOwners converts a Teller identity owner list into model structs.
func buildAccountOwners(accountID uuid.UUID, owners []TellerOwner) []*models.AccountOwner {
	result := make([]*models.AccountOwner, 0, len(owners))
	for _, owner := range owners {
		ao := &models.AccountOwner{
			AccountID: accountID,
			Name:      owner.Name,
		}

		if owner.DateOfBirth != "" {
			if dob, err := time.Parse("2006-01-02", owner.DateOfBirth); err == nil {
				ao.DateOfBirth = &dob
			}
		}
		if len(owner.Addresses) > 0 {
			addr := owner.Addresses[0]
			ao.AddressStreet = addr.Street
			ao.AddressCity = addr.City
			ao.AddressRegion = addr.Region
			ao.AddressPostalCode = addr.PostalCode
			ao.AddressCountry = addr.Country
		}
		if len(owner.PhoneNumbers) > 0 {
			ao.PhoneNumber = owner.PhoneNumbers[0].Number
		}
		if len(owner.Emails) > 0 {
			ao.Email = owner.Emails[0].Address
		}

		if len(owner.Addresses) > 1 || len(owner.PhoneNumbers) > 1 || len(owner.Emails) > 1 {
			additional := models.AdditionalOwnerData{}
			for _, addr := range owner.Addresses[1:] {
				additional.Addresses = append(additional.Addresses, models.OwnerAddress{
					Street: addr.Street, City: addr.City, Region: addr.Region,
					PostalCode: addr.PostalCode, Country: addr.Country, Type: addr.Type,
				})
			}
			for _, phone := range owner.PhoneNumbers[1:] {
				additional.PhoneNumbers = append(additional.PhoneNumbers, models.OwnerPhone{Number: phone.Number, Type: phone.Type})
			}
			for _, email := range owner.Emails[1:] {
				additional.Emails = append(additional.Emails, models.OwnerEmail{Address: email.Address, Type: email.Type})
			}
			if data, err := json.Marshal(additional); err == nil {
				ao.AdditionalData = data
			}
		}

		result = append(result, ao)
	}
	return result
}

// SyncTransactions syncs transactions from Teller for an account
func (s *TellerSyncService) SyncTransactions(ctx context.Context, account *models.Account) (int, error) {
	slog.DebugContext(ctx, "SyncTransactions: starting", "account", account.Name, "account_id", account.ID)

	tellerTxns, err := s.client.GetTransactions(ctx, account.TellerAccessToken, account.TellerAccountID)
	if err != nil {
		if IsTellerTransientError(err) {
			slog.WarnContext(ctx, "transient teller error fetching transactions, will retry", "account", account.Name, "err", err)
		} else if !providers.IsConnectionDisconnectedError(err) {
			slog.ErrorContext(ctx, "failed to fetch transactions", "account", account.Name, "err", err)
		}
		// Still update the timestamp so we know sync was attempted (helps diagnose issues)
		now := time.Now()
		account.LastSyncedAt = &now
		if updateErr := s.accounts.Update(ctx, account); updateErr != nil {
			slog.WarnContext(ctx, "failed to update account last_synced_at after fetch error", "account", account.Name, "err", updateErr)
		}
		return 0, err
	}
	slog.DebugContext(ctx, "SyncTransactions: fetched transactions from Teller", "count", len(tellerTxns), "account", account.Name)

	expenseAccount, incomeAccount, err := getContraAccounts(ctx, s.accounts, account.LedgerID)
	if err != nil {
		return 0, err
	}

	var created int
	var storeErrors int
	var newTxnIDs []uuid.UUID
	for _, tt := range tellerTxns {
		// Check if transaction already exists by Teller ID
		existing, err := s.transactions.GetByTellerID(ctx, tt.ID)
		if err == nil && existing != nil {
			continue // Skip existing
		}

		// Parse date and amount (also needed for the duplicate-by-content check below).
		date, err := time.Parse("2006-01-02", tt.Date)
		if err != nil {
			date = time.Now().UTC()
		}
		amountCents := parseAmount(tt.Amount)

		// Also check for duplicates by date/description/amount.
		// This handles the case where Teller assigns new transaction IDs after
		// an enrollment reconnection (e.g., MFA required, credential refresh).
		exists, err := s.transactions.ExistsByDateDescriptionAmount(ctx, account.ID, date, tt.Description, amountCents)
		if err == nil && exists {
			slog.DebugContext(ctx, "Skipping duplicate transaction (by content)", "description", tt.Description, "date", tt.Date, "amount_cents", amountCents)
			continue
		}

		txn, entries := buildTellerTransaction(tt, account, date, amountCents, expenseAccount.ID, incomeAccount.ID)

		if err := s.transactions.CreateWithEntries(ctx, txn, entries); err != nil {
			// Log and skip — a single bad row (e.g., constraint violation) should not
			// abort the entire sync and leave newer transactions unimported.
			slog.WarnContext(ctx, "failed to store transaction, skipping", "teller_id", tt.ID, "description", tt.Description, "err", err)
			storeErrors++
			continue
		}
		created++
		newTxnIDs = append(newTxnIDs, txn.ID)

		if err := s.transferMatcher.ProcessNewTransaction(ctx, txn, entries[0]); err != nil {
			logTransferMatchingError(ctx, "sync_teller", err, txn.ID)
		}
	}
	reportTellerStoreErrors(ctx, account.ID, storeErrors, len(tellerTxns), "sync completed with store failures", "store_transaction")

	// After syncing transactions, reconcile the balance with an opening balance entry
	if err := s.reconcileOpeningBalance(ctx, account); err != nil {
		if handleBalanceReconciliationErr(ctx, err, account.ID, "reconcile_opening_balance") {
			return 0, err
		}
	}

	updateLastSyncedAt(ctx, account, s.accounts)

	// Queue new transactions for enrichment (entity detection, logo fetching)
	if len(newTxnIDs) > 0 {
		if err := s.transactions.QueueForEnrichment(ctx, newTxnIDs); err != nil {
			slog.WarnContext(ctx, "failed to queue transactions for enrichment", "err", err)
		} else {
			slog.DebugContext(ctx, "Queued transactions for enrichment", "count", len(newTxnIDs))
		}
	}

	slog.DebugContext(ctx, "SyncTransactions: completed", "account", account.Name, "created", created)
	return created, nil
}

// reportTellerStoreErrors logs and captures a summary failure when one or more
// transactions could not be persisted. Callers log per-row warnings before
// calling this; this emits the aggregate for observability dashboards.
func reportTellerStoreErrors(ctx context.Context, accountID uuid.UUID, storeErrors, total int, logMsg, operation string) {
	if storeErrors == 0 {
		return
	}
	storeErr := fmt.Errorf("failed to store %d of %d transactions", storeErrors, total)
	slog.WarnContext(ctx, logMsg, "account_id", accountID, "store_errors", storeErrors)
	observability.CaptureFailure(ctx, storeErr, observability.FailureOptions{
		Component: "sync_teller",
		Operation: operation,
		Tags:      map[string]string{"account_id": accountID.String()},
	})
}

// SyncStats tracks statistics for a sync operation
type SyncStats struct {
	TellerTransactions  int
	NewTransactions     int
	UpdatedTransactions int
	DeletedTransactions int
	AutoLinkedTransfers int
	PendingTransfers    int
	StartTime           time.Time
	EndTime             time.Time
}

// buildTellerTransaction constructs a Transaction and its double-entry pair from a raw Teller
// transaction. This is the single authoritative implementation of contra-account determination:
// assets flip on sign, liabilities flip sign, refunds always map to expense.
func buildTellerTransaction(tt TellerTransaction, account *models.Account, date time.Time, amountCents int64, expenseAccountID, incomeAccountID uuid.UUID) (*models.Transaction, []*models.Entry) {
	txn := &models.Transaction{
		LedgerID:             account.LedgerID,
		Date:                 date,
		Description:          tt.Description,
		TellerTransactionID:  tt.ID,
		TellerType:           tt.Type,
		TellerCategory:       tt.Details.Category,
		TellerStatus:         tt.Status,
		CounterpartyName:     tt.Details.Counterparty.Name,
		CounterpartyType:     tt.Details.Counterparty.Type,
		RunningBalanceCents:  parseAmount(tt.RunningBalance),
		CategorizationStatus: models.CategorizationStatusPending,
	}

	// For assets: negative = spending (expense), positive = receiving (income).
	// For liabilities: positive = debt increase (expense), negative = payment.
	// Refunds always map to expense so they appear as negative expense, not income.
	isExpense := amountCents < 0
	if account.Type == models.AccountTypeLiability {
		isExpense = amountCents > 0
	}
	if strings.EqualFold(tt.Type, "refund") {
		isExpense = true
	}

	contraAccountID := incomeAccountID
	if isExpense {
		contraAccountID = expenseAccountID
	}

	entries := []*models.Entry{
		{AccountID: account.ID, AmountCents: amountCents, Currency: "USD"},
		{AccountID: contraAccountID, AmountCents: -amountCents, Currency: "USD"},
	}
	return txn, entries
}

// DeleteAndResync deletes all transactions for an account and re-imports everything fresh from Teller.
// This runs transfer matching and queues all transactions for categorization.
func (s *TellerSyncService) DeleteAndResync(ctx context.Context, account *models.Account) (int, error) {
	stats := &SyncStats{
		StartTime: time.Now(),
	}

	slog.DebugContext(ctx, "DeleteAndResync: starting", "account", account.Name, "institution", account.InstitutionName, "account_id", account.ID, "teller_id", account.TellerAccountID, "type", account.Type)

	if account.TellerAccessToken == "" {
		return 0, fmt.Errorf("no access token for account")
	}

	// Step 1: Re-sync account details (routing numbers, masked account number)
	s.syncAccountDetails(ctx, account, account.TellerAccessToken)
	if err := s.accounts.Update(ctx, account); err != nil {
		slog.WarnContext(ctx, "failed to update account details", "err", err)
	}

	// Step 2: Re-sync identity/owner information
	s.syncAccountIdentity(ctx, account, account.TellerAccessToken)

	// Step 3: Preserve confirmed transfers (by Teller transaction ID pairs)
	confirmedTransfers, err := s.getConfirmedTransferPairs(ctx, account.ID)
	if err != nil {
		slog.WarnContext(ctx, "failed to get confirmed transfers", "err", err)
		confirmedTransfers = nil
	} else {
		slog.DebugContext(ctx, "step 3 complete", "step", "preserved confirmed transfers", "count", len(confirmedTransfers))
	}

	// Step 4: Delete all existing transactions for this account
	deleted, err := s.deleteTransactionsForAccount(ctx, account.ID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete transactions", "err", err)
		return 0, err
	}
	stats.DeletedTransactions = deleted
	slog.DebugContext(ctx, "step 4 complete", "step", "deleted existing transactions", "deleted", deleted)

	// Step 5: Fetch and import all transactions from Teller
	tellerTxns, err := s.client.GetTransactions(ctx, account.TellerAccessToken, account.TellerAccountID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch transactions from Teller", "err", err)
		return 0, err
	}
	stats.TellerTransactions = len(tellerTxns)
	slog.DebugContext(ctx, "step 5 complete", "step", "fetched transactions from Teller", "count", len(tellerTxns))

	// Step 6: Import all transactions

	expenseAccount, incomeAccount, err := getContraAccounts(ctx, s.accounts, account.LedgerID)
	if err != nil {
		return 0, err
	}

	var created int
	var storeErrors int
	var newTxnIDs []uuid.UUID
	for _, tt := range tellerTxns {
		date, err := time.Parse("2006-01-02", tt.Date)
		if err != nil {
			date = time.Now().UTC()
		}
		amountCents := parseAmount(tt.Amount)

		txn, entries := buildTellerTransaction(tt, account, date, amountCents, expenseAccount.ID, incomeAccount.ID)

		if err := s.transactions.CreateWithEntries(ctx, txn, entries); err != nil {
			slog.WarnContext(ctx, "failed to create transaction", "teller_id", tt.ID, "description", tt.Description, "err", err)
			storeErrors++
			continue
		}
		created++
		newTxnIDs = append(newTxnIDs, txn.ID)
	}
	reportTellerStoreErrors(ctx, account.ID, storeErrors, len(tellerTxns), "resync completed with store failures", "store_transaction_resync")
	stats.NewTransactions = created
	slog.DebugContext(ctx, "step 6 complete", "step", "imported transactions", "imported", created)

	// Step 7: Restore previously confirmed transfers
	if len(confirmedTransfers) > 0 {
		restored, err := s.restoreConfirmedTransfers(ctx, confirmedTransfers)
		if err != nil {
			slog.WarnContext(ctx, "failed to restore some transfers", "err", err)
		}
		slog.DebugContext(ctx, "step 7 complete", "step", "restored confirmed transfers", "restored", restored, "total", len(confirmedTransfers))
	} else {
		slog.DebugContext(ctx, "step 7 complete", "step", "no confirmed transfers to restore")
	}

	// Step 8: Run transfer matching on remaining transactions
	autoLinked, pendingMatches, err := s.transferMatcher.MatchAllForAccountWithStats(ctx, account)
	if err != nil {
		slog.WarnContext(ctx, "transfer matching failed", "err", err)
	} else {
		stats.AutoLinkedTransfers = autoLinked
		stats.PendingTransfers = pendingMatches
		slog.DebugContext(ctx, "step 8 complete", "step", "transfer matching", "auto_linked", autoLinked, "pending", pendingMatches)
	}

	// Step 9: Reconcile opening balance (fresh calculation)
	if err := s.reconcileOpeningBalanceFresh(ctx, account); err != nil {
		if handleBalanceReconciliationErr(ctx, err, account.ID, "reconcile_opening_balance_fresh") {
			return 0, err
		}
	} else {
		slog.DebugContext(ctx, "step 9 complete", "step", "opening balance reconciled")
	}

	// Step 10: Queue all transactions for enrichment
	if len(newTxnIDs) > 0 {
		if err := s.transactions.QueueForEnrichment(ctx, newTxnIDs); err != nil {
			slog.WarnContext(ctx, "failed to queue for enrichment", "err", err)
		} else {
			slog.DebugContext(ctx, "step 10 complete", "step", "queued for enrichment", "count", len(newTxnIDs))
		}
	}

	updateLastSyncedAt(ctx, account, s.accounts)

	stats.EndTime = time.Now()
	duration := stats.EndTime.Sub(stats.StartTime)

	slog.DebugContext(ctx, "DeleteAndResync: complete", "duration", duration.Round(time.Millisecond), "deleted", stats.DeletedTransactions, "teller_transactions", stats.TellerTransactions, "imported", stats.NewTransactions, "auto_linked_transfers", stats.AutoLinkedTransfers, "pending_for_review", stats.PendingTransfers)

	return created, nil
}

// deleteTransactionsForAccount completely deletes all transactions and related data for an account.
// This is a COMPLETE cleanup - everything related to this account is removed.
func (s *TellerSyncService) deleteTransactionsForAccount(ctx context.Context, accountID uuid.UUID) (int, error) {
	// Use a database transaction for atomic cleanup
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// First, get all transaction IDs that have ANY entry for this account
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT t.id FROM transactions t
		JOIN entries e ON t.id = e.transaction_id
		WHERE e.account_id = $1
	`, accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to query transactions: %w", err)
	}

	txnIDs, err := scanUUIDRows(rows)
	rows.Close()
	if err != nil {
		return 0, fmt.Errorf("failed to scan transaction IDs: %w", err)
	}

	if len(txnIDs) == 0 {
		// No transactions to delete, but we still need to commit the transaction
		// to prevent the defer from rolling back
		if err := tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("failed to commit empty transaction: %w", err)
		}
		return 0, nil
	}

	// 1. Clear all transfer_pair_id references pointing TO these transactions
	_, err = tx.Exec(ctx, `
		UPDATE transactions SET transfer_pair_id = NULL, is_transfer = false
		WHERE transfer_pair_id = ANY($1)
	`, txnIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to clear incoming transfer references: %w", err)
	}

	// 2. Clear transfer_pair_id on transactions we're about to delete
	_, err = tx.Exec(ctx, `
		UPDATE transactions SET transfer_pair_id = NULL, is_transfer = false
		WHERE id = ANY($1)
	`, txnIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to clear outgoing transfer references: %w", err)
	}

	// 3. Delete pending transfer matches involving these transactions
	_, err = tx.Exec(ctx, `
		DELETE FROM pending_transfer_matches
		WHERE transaction_id = ANY($1)
		   OR candidate_transaction_id = ANY($1)
	`, txnIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to delete pending transfer matches: %w", err)
	}

	// 4. Delete transaction tags
	_, err = tx.Exec(ctx, `
		DELETE FROM transaction_tags WHERE transaction_id = ANY($1)
	`, txnIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to delete transaction tags: %w", err)
	}

	// 5. Delete ALL entries for these transactions (both account and contra-account entries)
	_, err = tx.Exec(ctx, `
		DELETE FROM entries WHERE transaction_id = ANY($1)
	`, txnIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to delete entries: %w", err)
	}

	// 6. Delete the transactions themselves
	_, err = tx.Exec(ctx, `
		DELETE FROM transactions WHERE id = ANY($1)
	`, txnIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to delete transactions: %w", err)
	}

	// 7. Clean up orphaned category accounts (income/expense accounts with no entries)
	// Only delete user-created category accounts, not the default Uncategorized ones
	_, err = tx.Exec(ctx, `
		DELETE FROM accounts a
		WHERE a.type IN ('income', 'expense')
		AND a.name NOT IN ('Uncategorized Income', 'Uncategorized Expenses', 'Opening Balance')
		AND NOT EXISTS (SELECT 1 FROM entries e WHERE e.account_id = a.id)
	`)
	if err != nil {
		slog.WarnContext(ctx, "failed to clean orphaned category accounts", "err", err)
		// Non-fatal - continue
	}

	// Note: Account balance is calculated dynamically from entries, so no need to reset it

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit cleanup transaction: %w", err)
	}

	return len(txnIDs), nil
}

// FullResync performs a complete resync of an account, updating existing transactions
// with new Teller data (category, running_balance, status, counterparty, etc.)
// This is useful for backfilling data after adding new fields.
func (s *TellerSyncService) FullResync(ctx context.Context, account *models.Account) (int, error) {
	stats := &SyncStats{
		StartTime: time.Now(),
	}

	slog.DebugContext(ctx, "FullResync: starting", "account", account.Name, "institution", account.InstitutionName, "account_id", account.ID, "teller_id", account.TellerAccountID, "type", account.Type)

	if account.TellerAccessToken == "" {
		return 0, fmt.Errorf("no access token for account")
	}

	// Step 1: Re-sync account details (routing numbers, masked account number)
	s.syncAccountDetails(ctx, account, account.TellerAccessToken)
	if err := s.accounts.Update(ctx, account); err != nil {
		slog.WarnContext(ctx, "failed to update account details", "err", err)
	}

	// Step 2: Re-sync identity/owner information
	s.syncAccountIdentity(ctx, account, account.TellerAccessToken)

	// Step 3: Fetch all transactions from Teller
	tellerTxns, err := s.client.GetTransactions(ctx, account.TellerAccessToken, account.TellerAccountID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch Teller transactions", "err", err)
		return 0, err
	}
	stats.TellerTransactions = len(tellerTxns)
	slog.DebugContext(ctx, "step 3 complete", "step", "fetched transactions from Teller", "count", len(tellerTxns))

	// Step 4: Update existing transactions with new Teller data
	var updated int
	var notFound int
	for _, tt := range tellerTxns {
		existing, err := s.transactions.GetByTellerID(ctx, tt.ID)
		if err != nil || existing == nil {
			notFound++
			continue
		}

		// Update existing transaction with fresh Teller data
		needsUpdate := false

		if existing.TellerCategory != tt.Details.Category {
			existing.TellerCategory = tt.Details.Category
			needsUpdate = true
		}
		if existing.TellerStatus != tt.Status {
			existing.TellerStatus = tt.Status
			needsUpdate = true
		}
		if existing.TellerType != tt.Type {
			existing.TellerType = tt.Type
			needsUpdate = true
		}
		if existing.CounterpartyName != tt.Details.Counterparty.Name {
			existing.CounterpartyName = tt.Details.Counterparty.Name
			needsUpdate = true
		}
		if existing.CounterpartyType != tt.Details.Counterparty.Type {
			existing.CounterpartyType = tt.Details.Counterparty.Type
			needsUpdate = true
		}

		runningBalanceCents := parseAmount(tt.RunningBalance)
		if existing.RunningBalanceCents != runningBalanceCents {
			existing.RunningBalanceCents = runningBalanceCents
			needsUpdate = true
		}

		if needsUpdate {
			if err := s.transactions.Update(ctx, existing); err != nil {
				slog.WarnContext(ctx, "failed to update transaction", "txn_id", existing.ID, "err", err)
				continue
			}
			updated++
		}
	}
	stats.UpdatedTransactions = updated
	stats.NewTransactions = notFound
	slog.DebugContext(ctx, "step 4 complete", "step", "updated existing transactions", "updated", updated, "not_in_db", notFound)

	// Step 5: Run transfer matching on all transactions for this account
	autoLinked, pendingMatches, err := s.transferMatcher.MatchAllForAccountWithStats(ctx, account)
	if err != nil {
		slog.WarnContext(ctx, "transfer matching failed", "err", err)
	} else {
		stats.AutoLinkedTransfers = autoLinked
		stats.PendingTransfers = pendingMatches
		slog.DebugContext(ctx, "step 5 complete", "step", "transfer matching", "auto_linked", autoLinked, "pending", pendingMatches)
	}

	// Step 6: Reconcile opening balance
	if err := s.reconcileOpeningBalance(ctx, account); err != nil {
		if handleBalanceReconciliationErr(ctx, err, account.ID, "reconcile_opening_balance") {
			return 0, err
		}
	} else {
		slog.DebugContext(ctx, "step 6 complete", "step", "opening balance reconciled")
	}

	updateLastSyncedAt(ctx, account, s.accounts)

	stats.EndTime = time.Now()
	duration := stats.EndTime.Sub(stats.StartTime)

	slog.DebugContext(ctx, "FullResync: complete", "duration", duration.Round(time.Millisecond), "teller_transactions", stats.TellerTransactions, "updated", stats.UpdatedTransactions, "not_yet_synced", stats.NewTransactions, "auto_linked_transfers", stats.AutoLinkedTransfers, "pending_for_review", stats.PendingTransfers)

	return updated, nil
}

// reconcileOpeningBalanceFresh deletes any existing opening balance and creates a fresh one.
// This is the correct approach for DeleteAndResync to ensure a clean slate.
func (s *TellerSyncService) reconcileOpeningBalanceFresh(ctx context.Context, account *models.Account) error {
	if err := s.deleteOpeningBalanceTransactions(ctx, account.ID); err != nil {
		slog.WarnContext(ctx, "failed to delete opening balance transactions", "err", err)
	}
	return s.reconcileOpeningBalance(ctx, account)
}

// deleteOpeningBalanceTransactions removes all Opening Balance transactions for an account
func (s *TellerSyncService) deleteOpeningBalanceTransactions(ctx context.Context, accountID uuid.UUID) error {
	// Find all Opening Balance transactions for this account
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT t.id FROM transactions t
		JOIN entries e ON t.id = e.transaction_id
		WHERE e.account_id = $1 AND t.description = 'Opening Balance'
	`, accountID)
	if err != nil {
		return err
	}
	defer rows.Close()

	txnIDs, err := scanUUIDRows(rows)
	if err != nil {
		return err
	}

	if len(txnIDs) == 0 {
		return nil
	}

	slog.DebugContext(ctx, "deleting existing Opening Balance transactions", "count", len(txnIDs), "account_id", accountID)

	// Delete entries first, then transactions
	_, err = s.pool.Exec(ctx, `DELETE FROM entries WHERE transaction_id = ANY($1)`, txnIDs)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `DELETE FROM transactions WHERE id = ANY($1)`, txnIDs)
	return err
}

// reconcileOpeningBalance creates or updates an opening balance entry to match Teller's reported balance
func (s *TellerSyncService) reconcileOpeningBalance(ctx context.Context, account *models.Account) error {
	tellerBalanceCents, err := s.getTellerBalanceCents(ctx, account)
	if err != nil {
		return err
	}

	currentBalanceCents, err := s.getAccountBalance(ctx, account.ID)
	if err != nil {
		return err
	}

	difference := tellerBalanceCents - currentBalanceCents

	if difference == 0 {
		return nil
	}

	existingOpeningTxn, err := s.getOpeningBalanceTransaction(ctx, account.ID)
	if err == nil && existingOpeningTxn != nil {
		return s.updateOpeningBalanceTransaction(ctx, existingOpeningTxn, account, difference)
	}

	return s.createOpeningBalanceTransaction(ctx, account, difference)
}

// getTellerBalanceCents returns the account's ledger balance in cents as reported by Teller.
func (s *TellerSyncService) getTellerBalanceCents(ctx context.Context, account *models.Account) (int64, error) {
	bal, err := s.client.GetAccountBalances(ctx, account.TellerAccessToken, account.TellerAccountID)
	if err != nil {
		return 0, fmt.Errorf("failed to get teller balance: %w", err)
	}
	if bal.Ledger == "" {
		return 0, fmt.Errorf("teller returned empty ledger balance for account %s", account.TellerAccountID)
	}
	return parseAmount(bal.Ledger), nil
}

// getAccountBalance calculates the current balance from all entries for an account
func (s *TellerSyncService) getAccountBalance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	var balance int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0) FROM entries WHERE account_id = $1
	`, accountID).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("failed to get current balance: %w", err)
	}
	return balance, nil
}

// getOpeningBalanceTransaction finds an existing opening balance transaction for an account
func (s *TellerSyncService) getOpeningBalanceTransaction(ctx context.Context, accountID uuid.UUID) (*models.Transaction, error) {
	var txnID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT t.id FROM transactions t
		JOIN entries e ON t.id = e.transaction_id
		WHERE e.account_id = $1 AND t.description = 'Opening Balance'
		LIMIT 1
	`, accountID).Scan(&txnID)
	if err != nil {
		return nil, err
	}
	return s.transactions.GetByID(ctx, txnID)
}

// createOpeningBalanceTransaction creates a new opening balance transaction
func (s *TellerSyncService) createOpeningBalanceTransaction(ctx context.Context, account *models.Account, amount int64) error {
	// Get or create the opening balance equity account
	equityAccount, err := s.getOrCreateOpeningBalanceAccount(ctx, account.LedgerID)
	if err != nil {
		return err
	}

	// Find the earliest transaction date to put opening balance before it
	var earliestDate time.Time
	err = s.pool.QueryRow(ctx, `
		SELECT MIN(t.date) FROM transactions t
		JOIN entries e ON t.id = e.transaction_id
		WHERE e.account_id = $1
	`, account.ID).Scan(&earliestDate)
	if err != nil || earliestDate.IsZero() {
		earliestDate = time.Now()
	}
	// Set opening balance to day before earliest transaction
	openingDate := earliestDate.AddDate(0, 0, -1)

	txn := &models.Transaction{
		LedgerID:    account.LedgerID,
		Date:        openingDate,
		Description: "Opening Balance",
	}

	entries := []*models.Entry{
		{AccountID: account.ID, AmountCents: amount, Currency: "USD"},
		{AccountID: equityAccount.ID, AmountCents: -amount, Currency: "USD"},
	}

	return s.transactions.CreateWithEntries(ctx, txn, entries)
}

// updateOpeningBalanceTransaction updates an existing opening balance transaction
func (s *TellerSyncService) updateOpeningBalanceTransaction(ctx context.Context, txn *models.Transaction, account *models.Account, additionalAmount int64) error {
	// Get the current entry for this account
	var currentAmount int64
	var entryID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id, amount_cents FROM entries 
		WHERE transaction_id = $1 AND account_id = $2
	`, txn.ID, account.ID).Scan(&entryID, &currentAmount)
	if err != nil {
		return err
	}

	newAmount := currentAmount + additionalAmount

	// Update both entries to maintain balance
	_, err = s.pool.Exec(ctx, `
		UPDATE entries SET amount_cents = $2 WHERE id = $1
	`, entryID, newAmount)
	if err != nil {
		return err
	}

	// Update the contra entry (equity account)
	_, err = s.pool.Exec(ctx, `
		UPDATE entries SET amount_cents = $2 
		WHERE transaction_id = $1 AND account_id != $3
	`, txn.ID, -newAmount, account.ID)

	return err
}

func (s *TellerSyncService) getOrCreateOpeningBalanceAccount(ctx context.Context, ledgerID uuid.UUID) (*models.Account, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE ledger_id = $1 AND type = 'equity' AND name = 'Opening Balance' LIMIT 1`,
		ledgerID,
	).Scan(&id)
	if err == nil {
		return s.accounts.GetByID(ctx, id)
	}

	acc := &models.Account{
		LedgerID: ledgerID,
		Name:     "Opening Balance",
		Type:     models.AccountTypeEquity,
		IsActive: true,
	}
	if err := s.accounts.Create(ctx, acc); err != nil {
		return nil, err
	}
	return acc, nil
}

func mapTellerAccountType(tellerType string) models.AccountType {
	switch tellerType {
	case "depository":
		return models.AccountTypeAsset
	case "credit":
		return models.AccountTypeLiability
	case "loan":
		return models.AccountTypeLiability
	case "investment":
		return models.AccountTypeAsset
	default:
		return models.AccountTypeAsset
	}
}

func parseAmount(amountStr string) int64 {
	amount, err := strconv.ParseFloat(strings.TrimSpace(amountStr), 64)
	if err != nil {
		slog.Warn("teller: could not parse amount", "raw", amountStr, "err", err)
		return 0
	}
	return dollarsToCents(amount)
}

// TransferPair represents a confirmed transfer by Teller transaction IDs
type TransferPair struct {
	TellerID1 string
	TellerID2 string
}

// getConfirmedTransferPairs retrieves all confirmed transfer pairs involving an account
// Returns pairs identified by their stable Teller transaction IDs
func (s *TellerSyncService) getConfirmedTransferPairs(ctx context.Context, accountID uuid.UUID) ([]TransferPair, error) {
	// Find all confirmed transfers where this account is involved
	rows, err := s.pool.Query(ctx, `
		SELECT t1.teller_transaction_id, t2.teller_transaction_id
		FROM transactions t1
		JOIN transactions t2 ON t1.transfer_pair_id = t2.id
		JOIN entries e ON e.transaction_id = t1.id
		WHERE t1.is_transfer = true
		  AND t1.transfer_pair_id IS NOT NULL
		  AND e.account_id = $1
		  AND t1.teller_transaction_id IS NOT NULL
		  AND t1.teller_transaction_id != ''
		  AND t2.teller_transaction_id IS NOT NULL
		  AND t2.teller_transaction_id != ''
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []TransferPair
	seen := make(map[string]bool)
	for rows.Next() {
		var pair TransferPair
		if err := rows.Scan(&pair.TellerID1, &pair.TellerID2); err != nil {
			return nil, err
		}
		// Deduplicate (same pair might appear twice due to bidirectional relationship)
		key := pair.TellerID1 + "|" + pair.TellerID2
		if pair.TellerID1 > pair.TellerID2 {
			key = pair.TellerID2 + "|" + pair.TellerID1
		}
		if !seen[key] {
			seen[key] = true
			pairs = append(pairs, pair)
		}
	}
	return pairs, rows.Err()
}

// restoreConfirmedTransfers restores previously confirmed transfer pairs after reimport
func (s *TellerSyncService) restoreConfirmedTransfers(ctx context.Context, pairs []TransferPair) (int, error) {
	restored := 0
	for _, pair := range pairs {
		// Look up the new transaction IDs by Teller ID
		var txn1ID, txn2ID uuid.UUID
		err := s.pool.QueryRow(ctx, `
			SELECT id FROM transactions WHERE teller_transaction_id = $1
		`, pair.TellerID1).Scan(&txn1ID)
		if err != nil {
			continue // Transaction might not exist anymore
		}

		err = s.pool.QueryRow(ctx, `
			SELECT id FROM transactions WHERE teller_transaction_id = $1
		`, pair.TellerID2).Scan(&txn2ID)
		if err != nil {
			continue // Transaction might not exist anymore
		}

		// Re-link the transfer pair
		if err := s.transactions.SetTransferPair(ctx, txn1ID, txn2ID); err != nil {
			slog.WarnContext(ctx, "failed to restore transfer pair", "teller_id_1", pair.TellerID1, "teller_id_2", pair.TellerID2, "err", err)
			continue
		}
		restored++
	}
	return restored, nil
}

// SyncAccountsWithToken implements providers.ProviderImpl.
func (s *TellerSyncService) SyncAccountsWithToken(ctx context.Context, ledgerID uuid.UUID, accessToken string) ([]*models.Account, error) {
	return s.SyncAccounts(ctx, ledgerID, accessToken)
}

// SyncTransactionsForAccount implements providers.ProviderImpl.
func (s *TellerSyncService) SyncTransactionsForAccount(ctx context.Context, account *models.Account) (int, error) {
	return s.SyncTransactions(ctx, account)
}
