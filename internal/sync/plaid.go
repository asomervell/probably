package sync

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/plaid/plaid-go/v40/plaid"
)

// PlaidClient wraps the Plaid API client
type PlaidClient struct {
	cfg    *config.Config
	client *plaid.APIClient
}

// plaidDevelopmentHost is Plaid's "development" API (distinct from sandbox).
// plaid-go only ships Production + Sandbox constants; development uses a raw URL.
const plaidDevelopmentHost = "https://development.plaid.com"
const plaidMaxTransactionHistoryDays int32 = 730

// plaidExtractError unpacks structured Plaid error fields from err using the two
// extraction paths the SDK exposes (ToPlaidError, then GenericOpenAPIError fallback).
// All three return values are empty strings when err carries no structured Plaid data.
func plaidExtractError(err error) (code, message string, errType plaid.PlaidErrorType) {
	if pe, convErr := plaid.ToPlaidError(err); convErr == nil && pe.GetErrorType() != "" {
		return pe.GetErrorCode(), pe.GetErrorMessage(), pe.GetErrorType()
	}
	if ge, ok := err.(*plaid.GenericOpenAPIError); ok {
		if ge.Model() != nil {
			if pe, ok := ge.Model().(plaid.PlaidError); ok {
				return pe.GetErrorCode(), pe.GetErrorMessage(), pe.GetErrorType()
			}
		}
	}
	return "", "", ""
}

// plaidErrorf wraps a Plaid API error with a descriptive prefix, surfacing
// the Plaid error type, code, and message when available.
func plaidErrorf(msg string, err error) error {
	if code, message, errType := plaidExtractError(err); errType != "" {
		return fmt.Errorf("%s: %s (%s) - %s: %w", msg, errType, code, message, err)
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// NewPlaidClient creates a new Plaid API client
func NewPlaidClient(cfg *config.Config) (*PlaidClient, error) {
	clientID := strings.TrimSpace(cfg.PlaidClientID)
	secret := cfg.PlaidSecret()
	if clientID == "" || secret == "" {
		return nil, fmt.Errorf("missing Plaid credentials for environment %q (set PLAID_CLIENT_ID and the matching secret)", strings.TrimSpace(cfg.PlaidEnvironment))
	}

	configuration := plaid.NewConfiguration()
	configuration.AddDefaultHeader("PLAID-CLIENT-ID", clientID)
	configuration.AddDefaultHeader("PLAID-SECRET", secret)

	// Plaid has three API hosts; plaid-go only defines production + sandbox.
	var env plaid.Environment
	switch strings.ToLower(strings.TrimSpace(cfg.PlaidEnvironment)) {
	case "production":
		env = plaid.Production
	case "development":
		env = plaid.Environment(plaidDevelopmentHost)
	default:
		env = plaid.Sandbox
	}
	configuration.UseEnvironment(env)

	client := plaid.NewAPIClient(configuration)

	return &PlaidClient{
		cfg:    cfg,
		client: client,
	}, nil
}

// ExchangePublicToken exchanges a public token for an access token
func (c *PlaidClient) ExchangePublicToken(ctx context.Context, publicToken string) (string, string, error) {
	request := plaid.NewItemPublicTokenExchangeRequest(publicToken)
	request.SetClientId(c.cfg.PlaidClientID)
	request.SetSecret(c.cfg.PlaidSecret())

	resp, _, err := c.client.PlaidApi.ItemPublicTokenExchange(ctx).ItemPublicTokenExchangeRequest(*request).Execute()
	if err != nil {
		return "", "", fmt.Errorf("failed to exchange public token: %w", err)
	}

	accessToken := resp.GetAccessToken()
	itemID := resp.GetItemId()

	return accessToken, itemID, nil
}

// GetAccounts fetches all accounts for an access token.
// Transient Plaid server errors (API_ERROR, 5xx) and network failures are retried
// with exponential back-off (1s, 2s, 4s). Item and auth errors return immediately.
func (c *PlaidClient) GetAccounts(ctx context.Context, accessToken string) ([]plaid.AccountBase, string, string, error) {
	request := plaid.NewAccountsGetRequest(accessToken)
	request.SetClientId(c.cfg.PlaidClientID)
	request.SetSecret(c.cfg.PlaidSecret())

	var resp plaid.AccountsGetResponse
	if err := retryWithBackoff(ctx, 3, func() (bool, error) {
		r, httpResp, err := c.client.PlaidApi.AccountsGet(ctx).AccountsGetRequest(*request).Execute()
		if err != nil {
			return IsPlaidTransientError(err, httpResp), err
		}
		resp = r
		return false, nil
	}); err != nil {
		return nil, "", "", fmt.Errorf("failed to get accounts: %w", err)
	}

	accounts := resp.GetAccounts()
	item := resp.GetItem()
	return accounts, item.GetItemId(), item.GetInstitutionId(), nil
}

// fetchTransactionsPage executes a single Plaid TransactionsGet request with retry.
func (c *PlaidClient) fetchTransactionsPage(ctx context.Context, request *plaid.TransactionsGetRequest) (plaid.TransactionsGetResponse, error) {
	var resp plaid.TransactionsGetResponse
	err := retryWithBackoff(ctx, 3, func() (bool, error) {
		r, httpResp, err := c.client.PlaidApi.TransactionsGet(ctx).TransactionsGetRequest(*request).Execute()
		if err != nil {
			return IsPlaidTransientError(err, httpResp), err
		}
		resp = r
		return false, nil
	})
	return resp, err
}

// GetTransactions fetches transactions for accounts.
// Transient Plaid server errors (API_ERROR, 5xx) and network failures are retried
// with exponential back-off (1s, 2s, 4s) for both the initial call and each page.
func (c *PlaidClient) GetTransactions(ctx context.Context, accessToken string, startDate, endDate time.Time, accountIDs []string) ([]plaid.Transaction, error) {
	request := plaid.NewTransactionsGetRequest(accessToken, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	request.SetClientId(c.cfg.PlaidClientID)
	request.SetSecret(c.cfg.PlaidSecret())

	options := plaid.NewTransactionsGetRequestOptions()
	options.SetCount(500)
	options.SetOffset(0)
	// Plaid defaults to 90 days unless explicitly configured at Link time.
	// This only has effect on Items where Transactions has not yet been initialized.
	options.SetDaysRequested(plaidMaxTransactionHistoryDays)
	if len(accountIDs) > 0 {
		options.SetAccountIds(accountIDs)
	}
	request.SetOptions(*options)

	resp, err := c.fetchTransactionsPage(ctx, request)
	if err != nil {
		return nil, plaidErrorf("failed to get transactions", err)
	}

	transactions := resp.GetTransactions()

	// Handle pagination using offset
	totalTransactions := resp.GetTotalTransactions()
	offset := int32(len(transactions))

	for offset < totalTransactions {
		options := plaid.NewTransactionsGetRequestOptions()
		options.SetCount(500)
		options.SetDaysRequested(plaidMaxTransactionHistoryDays)
		if len(accountIDs) > 0 {
			options.SetAccountIds(accountIDs)
		}
		options.SetOffset(offset)
		request.SetOptions(*options)

		resp, err = c.fetchTransactionsPage(ctx, request)
		if err != nil {
			return nil, plaidErrorf("failed to fetch transaction page", err)
		}

		newTransactions := resp.GetTransactions()
		if len(newTransactions) == 0 {
			break
		}

		transactions = append(transactions, newTransactions...)
		offset += int32(len(newTransactions))
	}

	return transactions, nil
}

// GetInstitution fetches institution information by ID.
// includeOptionalMetadata should be true to get logo, primary_color, and url.
// Transient errors are retried with exponential back-off (1s, 2s, 4s).
func (c *PlaidClient) GetInstitution(ctx context.Context, institutionID string, includeOptionalMetadata bool) (*plaid.Institution, error) {
	request := plaid.NewInstitutionsGetByIdRequest(institutionID, []plaid.CountryCode{plaid.COUNTRYCODE_US})
	request.SetClientId(c.cfg.PlaidClientID)
	request.SetSecret(c.cfg.PlaidSecret())

	if includeOptionalMetadata {
		options := plaid.NewInstitutionsGetByIdRequestOptions()
		options.SetIncludeOptionalMetadata(true)
		request.SetOptions(*options)
	}

	var resp plaid.InstitutionsGetByIdResponse
	if err := retryWithBackoff(ctx, 3, func() (bool, error) {
		r, httpResp, err := c.client.PlaidApi.InstitutionsGetById(ctx).InstitutionsGetByIdRequest(*request).Execute()
		if err != nil {
			return IsPlaidTransientError(err, httpResp), err
		}
		resp = r
		return false, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to get institution: %w", err)
	}

	institution := resp.GetInstitution()
	return &institution, nil
}

// CreateLinkToken creates a Link token for Plaid Link
// If accessToken is provided, the token will be created in 'update mode' for re-authentication
func (c *PlaidClient) CreateLinkToken(ctx context.Context, userID string, accessToken string, redirectURI string, accountSelectionEnabled bool) (string, error) {
	// Validate redirect URI format if provided
	if redirectURI != "" {
		if _, err := url.Parse(redirectURI); err != nil {
			return "", fmt.Errorf("invalid redirect URI format: %w", err)
		}
	}

	user := plaid.NewLinkTokenCreateRequestUser(userID)

	// Parse country codes from config
	countryCodes := []plaid.CountryCode{}
	for _, code := range strings.Split(c.cfg.PlaidCountryCodes, ",") {
		code = strings.TrimSpace(strings.ToUpper(code))
		if code != "" {
			countryCodes = append(countryCodes, plaid.CountryCode(code))
		}
	}
	// Default to US if no country codes configured
	if len(countryCodes) == 0 {
		countryCodes = []plaid.CountryCode{plaid.COUNTRYCODE_US}
	}

	request := plaid.NewLinkTokenCreateRequest(
		"Probably",
		"en",
		countryCodes,
	)
	request.SetUser(*user)
	request.SetClientId(c.cfg.PlaidClientID)
	request.SetSecret(c.cfg.PlaidSecret())

	// If accessToken is provided, we are in update mode
	if accessToken != "" {
		request.SetAccessToken(accessToken)
		// Products should not be set in update mode
	} else {
		// Only set products for new connections
		// Only request transactions - auth product requires special Plaid approval
		request.SetProducts([]plaid.Products{
			plaid.PRODUCTS_TRANSACTIONS,
		})
		transactions := plaid.NewLinkTokenTransactions()
		transactions.SetDaysRequested(plaidMaxTransactionHistoryDays)
		request.SetTransactions(*transactions)
	}

	// Set redirect URI if provided (caller is responsible for ensuring it's registered in Plaid dashboard)
	if redirectURI != "" {
		request.SetRedirectUri(redirectURI)
	}

	// Set webhook URL only if explicitly configured via PLAID_WEBHOOK_URL environment variable
	// This ensures webhook is only used when explicitly intended and registered in Plaid dashboard
	if c.cfg.PlaidWebhookURL != "" {
		request.SetWebhook(c.cfg.PlaidWebhookURL)
	}

	// Enable account selection if requested (for NEW_ACCOUNTS_AVAILABLE flow)
	// This is done via the update parameter
	if accountSelectionEnabled {
		update := plaid.NewLinkTokenCreateRequestUpdate()
		update.SetAccountSelectionEnabled(true)
		request.SetUpdate(*update)
	}

	resp, httpResp, err := c.client.PlaidApi.LinkTokenCreate(ctx).LinkTokenCreateRequest(*request).Execute()
	if err != nil {
		var errorMsg string
		code, message, errType := plaidExtractError(err)
		if errType != "" {
			errorMsg = fmt.Sprintf(": %s (%s) - %s", errType, code, message)
			isRedirectURIError := code == "INVALID_FIELD" &&
				(strings.Contains(message, "redirect URI") || strings.Contains(message, "OAuth redirect"))
			if isRedirectURIError && redirectURI != "" {
				return "", fmt.Errorf("failed to create link token%s: OAuth redirect URI '%s' not registered in Plaid dashboard - add it to Allowed redirect URIs at dashboard.plaid.com/developers/api: %w",
					errorMsg, redirectURI, err)
			}
		} else if httpResp != nil {
			errorMsg = fmt.Sprintf(" (HTTP %d)", httpResp.StatusCode)
		}
		return "", fmt.Errorf("failed to create link token%s: %w", errorMsg, err)
	}

	return resp.GetLinkToken(), nil
}

// ItemRemove removes an item (disconnects the bank)
func (c *PlaidClient) ItemRemove(ctx context.Context, accessToken string) error {
	request := plaid.NewItemRemoveRequest(accessToken)
	request.SetClientId(c.cfg.PlaidClientID)
	request.SetSecret(c.cfg.PlaidSecret())

	_, _, err := c.client.PlaidApi.ItemRemove(ctx).ItemRemoveRequest(*request).Execute()
	if err != nil {
		return fmt.Errorf("failed to remove item: %w", err)
	}

	return nil
}

// itemDisconnectionCodes are Plaid error codes that indicate an item is permanently
// unavailable or requires user action to reconnect (re-auth, unsupported product, etc.).
var itemDisconnectionCodes = []string{
	"ITEM_LOGIN_REQUIRED",
	"ITEM_NOT_SUPPORTED",
	"INVALID_ACCESS_TOKEN",
	"ITEM_ERROR",
	"INVALID_CREDENTIALS",
}

func isItemDisconnectionCode(code string, errorType plaid.PlaidErrorType) bool {
	if string(errorType) == "ITEM_ERROR" {
		return true
	}
	for _, c := range itemDisconnectionCodes {
		if code == c {
			return true
		}
	}
	return false
}

// isPlaidItemError checks if a raw Plaid API error indicates the item is disconnected.
func isPlaidItemError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*PlaidItemError); ok {
		return true
	}
	if code, _, errType := plaidExtractError(err); errType != "" || code != "" {
		return isItemDisconnectionCode(code, errType)
	}
	for _, code := range itemDisconnectionCodes {
		if strings.Contains(err.Error(), code) {
			return true
		}
	}
	return false
}

// IsPlaidTransientError reports whether a Plaid Execute() error is transient.
// Plaid API_ERROR (server-side 5xx), raw HTTP 5xx, and network failures are retried.
// Structured client errors (ITEM_ERROR, INVALID_REQUEST, etc.) return immediately.
// Note: passing nil httpResp treats any unstructured error as a network failure
// and returns true — only call from within Plaid API call sites, not as a generic
// provider transient check (use IsProviderTransientError for that).
func IsPlaidTransientError(err error, httpResp *http.Response) bool {
	if err == nil {
		return false
	}
	if isPlaidItemError(err) {
		return false
	}
	if _, _, errType := plaidExtractError(err); errType != "" {
		return errType == "API_ERROR" || errType == "RATE_LIMIT_EXCEEDED"
	}
	if httpResp != nil {
		return httpResp.StatusCode >= 500
	}
	// No structured Plaid error and no HTTP response: network/transport failure.
	return true
}
