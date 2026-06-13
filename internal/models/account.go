package models

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountType string

const (
	AccountTypeAsset     AccountType = "asset"
	AccountTypeLiability AccountType = "liability"
	AccountTypeIncome    AccountType = "income"
	AccountTypeExpense   AccountType = "expense"
	AccountTypeEquity    AccountType = "equity"
)


type Account struct {
	ID                 uuid.UUID   `json:"id"`
	LedgerID           uuid.UUID   `json:"ledger_id"`
	Name               string      `json:"name"`
	Type               AccountType `json:"type"`
	InstitutionName    string      `json:"institution_name,omitempty"`
	InstitutionID      string      `json:"institution_id,omitempty"`       // Teller institution ID
	InstitutionLogoURL string      `json:"institution_logo_url,omitempty"` // Downloaded logo path

	// Provider-agnostic connection fields
	Provider          string     `json:"provider,omitempty"`            // 'teller', 'plaid', 'akahu'
	ExternalAccountID string     `json:"external_account_id,omitempty"` // Provider's account ID
	ConnectionID      string     `json:"connection_id,omitempty"`       // Provider's connection/enrollment/item ID
	AccessToken       string     `json:"-"`                             // Encrypted access token
	AccountSubtype    string     `json:"account_subtype,omitempty"`     // e.g., "checking", "savings", "credit_card"
	AccountStatus     string     `json:"account_status,omitempty"`      // e.g., "open", "closed"
	ConnectionStatus  string     `json:"connection_status,omitempty"`   // 'disconnected', 'login_required', 'pending_expiration', 'pending_disconnect', 'new_accounts_available'
	LastSyncedAt      *time.Time `json:"last_synced_at,omitempty"`      // When this account was last synced
	ProviderMetadata  []byte     `json:"-"`                             // JSONB provider-specific metadata

	// Teller integration (kept for backward compatibility during migration)
	TellerAccountID    string     `json:"teller_account_id,omitempty"`
	TellerEnrollmentID string     `json:"teller_enrollment_id,omitempty"`
	TellerAccessToken  string     `json:"-"`                               // Encrypted
	TellerSubtype      string     `json:"teller_subtype,omitempty"`        // e.g., "checking", "savings", "credit_card"
	TellerStatus       string     `json:"teller_status,omitempty"`         // e.g., "open", "closed"

	// Common account fields
	LastFour            string `json:"last_four,omitempty"`             // Last 4 digits of account number
	AccountNumberMasked string `json:"account_number_masked,omitempty"` // Masked account number
	RoutingNumberACH    string `json:"routing_number_ach,omitempty"`
	RoutingNumberWire   string `json:"routing_number_wire,omitempty"`

	// Fixed asset fields (for Type=asset accounts representing physical assets)
	AssetSubtype       string     `json:"asset_subtype,omitempty"`        // real_estate, vehicle, equipment, collectible, other
	PurchaseDate       *time.Time `json:"purchase_date,omitempty"`        // When the asset was purchased
	PurchasePriceCents int64      `json:"purchase_price_cents,omitempty"` // Original purchase price
	CurrentValueCents  int64      `json:"current_value_cents,omitempty"`  // Current estimated value
	DepreciationMethod string     `json:"depreciation_method,omitempty"`  // none, straight_line, declining_balance

	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Left to Spend configuration
	IncludeInLeftToSpend *bool `json:"include_in_left_to_spend,omitempty"` // NULL = use smart default, TRUE/FALSE = user override

	// Computed field
	Balance int64 `json:"balance,omitempty"`
}

// TellerToken returns the Teller access token, preferring the canonical
// AccessToken field but falling back to the legacy TellerAccessToken field
// for accounts created before the unified credential migration.
func (a *Account) TellerToken() string {
	if a.AccessToken != "" {
		return a.AccessToken
	}
	return a.TellerAccessToken
}

type AccountStore struct {
	pool *pgxpool.Pool
}

func NewAccountStore(pool *pgxpool.Pool) *AccountStore {
	return &AccountStore{pool: pool}
}

func (s *AccountStore) Create(ctx context.Context, account *Account) error {
	if account.ID == uuid.Nil {
		account.ID = uuid.New()
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO accounts (id, ledger_id, name, type, institution_name, institution_id, institution_logo_url,
			provider, external_account_id, connection_id, access_token,
			account_subtype, account_status, connection_status, last_synced_at, provider_metadata,
			last_four, account_number_masked,
			routing_number_ach, routing_number_wire,
			asset_subtype, purchase_date, purchase_price_cents, current_value_cents, depreciation_method,
			is_active, include_in_left_to_spend, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
	`, account.ID, account.LedgerID, account.Name, account.Type, NullString(account.InstitutionName),
		NullString(account.InstitutionID), NullString(account.InstitutionLogoURL),
		NullString(account.Provider), NullString(account.ExternalAccountID), NullString(account.ConnectionID),
		NullString(account.AccessToken), NullString(account.AccountSubtype), NullString(account.AccountStatus),
		NullString(account.ConnectionStatus), account.LastSyncedAt, account.ProviderMetadata,
		NullString(account.LastFour), NullString(account.AccountNumberMasked),
		NullString(account.RoutingNumberACH), NullString(account.RoutingNumberWire),
		NullString(account.AssetSubtype), account.PurchaseDate, NullInt64(account.PurchasePriceCents),
		NullInt64(account.CurrentValueCents), NullString(account.DepreciationMethod),
		true, NullBool(account.IncludeInLeftToSpend), time.Now(), time.Now())

	return err
}

func (s *AccountStore) Update(ctx context.Context, account *Account) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE accounts SET
			name = $2, type = $3, institution_name = $4, institution_id = $5, institution_logo_url = $6,
			provider = $7, external_account_id = $8, connection_id = $9, access_token = $10,
			account_subtype = $11, account_status = $12, connection_status = $13, last_synced_at = $14, provider_metadata = $15,
			last_four = $16, account_number_masked = $17,
			routing_number_ach = $18, routing_number_wire = $19,
			asset_subtype = $20, purchase_date = $21, purchase_price_cents = $22, current_value_cents = $23, depreciation_method = $24,
			is_active = $25, include_in_left_to_spend = $26, updated_at = $27
		WHERE id = $1
	`, account.ID, account.Name, account.Type, NullString(account.InstitutionName),
		NullString(account.InstitutionID), NullString(account.InstitutionLogoURL),
		NullString(account.Provider), NullString(account.ExternalAccountID), NullString(account.ConnectionID),
		NullString(account.AccessToken), NullString(account.AccountSubtype), NullString(account.AccountStatus),
		NullString(account.ConnectionStatus), account.LastSyncedAt, account.ProviderMetadata,
		NullString(account.LastFour), NullString(account.AccountNumberMasked),
		NullString(account.RoutingNumberACH), NullString(account.RoutingNumberWire),
		NullString(account.AssetSubtype), account.PurchaseDate, NullInt64(account.PurchasePriceCents),
		NullInt64(account.CurrentValueCents), NullString(account.DepreciationMethod),
		account.IsActive, NullBool(account.IncludeInLeftToSpend), time.Now())

	return err
}

func (s *AccountStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM accounts WHERE id = $1`, id)
	return err
}

// DeleteWithTransactions deletes an account and ALL transactions that have entries to it.
// This is for removing duplicate accounts where the transactions are also duplicates.
// WARNING: This permanently deletes data!
func (s *AccountStore) DeleteWithTransactions(ctx context.Context, id uuid.UUID) (int64, error) {
	// First, find all transaction IDs that have entries to this account
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT transaction_id FROM entries WHERE account_id = $1
	`, id)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var txnIDs []uuid.UUID
	for rows.Next() {
		var txnID uuid.UUID
		if err := rows.Scan(&txnID); err != nil {
			return 0, err
		}
		txnIDs = append(txnIDs, txnID)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(txnIDs) == 0 {
		// No transactions, just delete the account
		_, err := s.pool.Exec(ctx, `DELETE FROM accounts WHERE id = $1`, id)
		return 0, err
	}

	// Before deleting transactions, we need to handle transfer_pair_id foreign key constraints.
	// Any transactions that reference our transactions via transfer_pair_id need to have
	// those references nullified to avoid foreign key constraint violations.
	_, err = s.pool.Exec(ctx, `
		UPDATE transactions 
		SET transfer_pair_id = NULL, is_transfer = false
		WHERE transfer_pair_id = ANY($1)
	`, txnIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to nullify transfer pair references: %w", err)
	}

	// Now we can safely delete the transactions (entries cascade via FK)
	result, err := s.pool.Exec(ctx, `DELETE FROM transactions WHERE id = ANY($1)`, txnIDs)
	if err != nil {
		return 0, err
	}
	deleted := result.RowsAffected()

	// Now delete the account (should have no entries left)
	_, err = s.pool.Exec(ctx, `DELETE FROM accounts WHERE id = $1`, id)
	if err != nil {
		return deleted, err
	}

	return deleted, nil
}

func (s *AccountStore) GetByID(ctx context.Context, id uuid.UUID) (*Account, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, name, type, institution_name, institution_id, institution_logo_url,
			provider, external_account_id, connection_id, access_token,
			account_subtype, account_status, connection_status, last_synced_at, provider_metadata,
			last_four, account_number_masked,
			routing_number_ach, routing_number_wire,
			asset_subtype, purchase_date, purchase_price_cents, current_value_cents, depreciation_method,
			is_active, include_in_left_to_spend, created_at, updated_at
		FROM accounts WHERE id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, pgx.ErrNoRows
	}
	return s.scanAccountRow(rows)
}

func (s *AccountStore) GetByLedgerID(ctx context.Context, ledgerID uuid.UUID) ([]*Account, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, name, type, institution_name, institution_id, institution_logo_url,
			provider, external_account_id, connection_id, access_token,
			account_subtype, account_status, connection_status, last_synced_at, provider_metadata,
			last_four, account_number_masked,
			routing_number_ach, routing_number_wire,
			asset_subtype, purchase_date, purchase_price_cents, current_value_cents, depreciation_method,
			is_active, include_in_left_to_spend, created_at, updated_at
		FROM accounts WHERE ledger_id = $1 ORDER BY type, name
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		a, err := s.scanAccountRow(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}

	return accounts, rows.Err()
}

// scanAccountRow is a helper to scan account rows from a query
func (s *AccountStore) scanAccountRow(rows pgx.Rows) (*Account, error) {
	var a Account
	var instName, instID, instLogoURL sql.NullString
	var provider, extAccID, connID, accessToken, accSubtype, accStatus, connStatus sql.NullString
	var lastFour, accNumMasked sql.NullString
	var routingACH, routingWire sql.NullString
	var lastSyncedAt, purchaseDate sql.NullTime
	var assetSubtype, deprecMethod sql.NullString
	var purchasePrice, currentValue sql.NullInt64
	var includeInLeftToSpend sql.NullBool
	var providerMetadata []byte

	if err := rows.Scan(&a.ID, &a.LedgerID, &a.Name, &a.Type, &instName, &instID, &instLogoURL,
		&provider, &extAccID, &connID, &accessToken,
		&accSubtype, &accStatus, &connStatus, &lastSyncedAt, &providerMetadata,
		&lastFour, &accNumMasked,
		&routingACH, &routingWire,
		&assetSubtype, &purchaseDate, &purchasePrice, &currentValue, &deprecMethod,
		&a.IsActive, &includeInLeftToSpend, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}

	a.InstitutionName = instName.String
	a.InstitutionID = instID.String
	a.InstitutionLogoURL = instLogoURL.String
	a.Provider = provider.String
	a.ExternalAccountID = extAccID.String
	a.ConnectionID = connID.String
	a.AccessToken = accessToken.String
	a.AccountSubtype = accSubtype.String
	a.AccountStatus = accStatus.String
	a.ConnectionStatus = connStatus.String
	a.LastSyncedAt = nullTimePtr(lastSyncedAt)
	a.ProviderMetadata = providerMetadata
	// Map to Teller fields for backward compatibility
	a.TellerAccountID = extAccID.String
	a.TellerEnrollmentID = connID.String
	a.TellerAccessToken = accessToken.String
	a.TellerSubtype = accSubtype.String
	a.TellerStatus = accStatus.String
	a.LastFour = lastFour.String
	a.AccountNumberMasked = accNumMasked.String
	a.RoutingNumberACH = routingACH.String
	a.RoutingNumberWire = routingWire.String
	a.AssetSubtype = assetSubtype.String
	a.PurchaseDate = nullTimePtr(purchaseDate)
	a.PurchasePriceCents = purchasePrice.Int64
	a.CurrentValueCents = currentValue.Int64
	a.DepreciationMethod = deprecMethod.String
	if includeInLeftToSpend.Valid {
		a.IncludeInLeftToSpend = &includeInLeftToSpend.Bool
	}

	return &a, nil
}

func (s *AccountStore) GetByExternalAccountID(ctx context.Context, externalAccountID string) (*Account, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, name, type, institution_name, institution_id, institution_logo_url,
			provider, external_account_id, connection_id, access_token,
			account_subtype, account_status, connection_status, last_synced_at, provider_metadata,
			last_four, account_number_masked,
			routing_number_ach, routing_number_wire,
			asset_subtype, purchase_date, purchase_price_cents, current_value_cents, depreciation_method,
			is_active, include_in_left_to_spend, created_at, updated_at
		FROM accounts WHERE external_account_id = $1 OR COALESCE((provider_metadata->>'teller_account_id')::text, '') = $1
		LIMIT 1
	`, externalAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, pgx.ErrNoRows
	}
	return s.scanAccountRow(rows)
}

// FindByNameAndInstitution finds an account by name and institution within a ledger.
// This is used for reconnecting accounts that lost their Teller IDs.
// It prioritizes accounts without existing Teller credentials (disconnected accounts).
func (s *AccountStore) FindByNameAndInstitution(ctx context.Context, ledgerID uuid.UUID, name, institutionName string) (*Account, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, name, type, institution_name, institution_id, institution_logo_url,
			provider, external_account_id, connection_id, access_token,
			account_subtype, account_status, connection_status, last_synced_at, provider_metadata,
			last_four, account_number_masked,
			routing_number_ach, routing_number_wire,
			asset_subtype, purchase_date, purchase_price_cents, current_value_cents, depreciation_method,
			is_active, include_in_left_to_spend, created_at, updated_at
		FROM accounts
		WHERE ledger_id = $1
			AND name = $2
			AND LOWER(TRIM(institution_name)) = LOWER(TRIM($3))
			AND (external_account_id IS NULL OR external_account_id = '')
		LIMIT 1
	`, ledgerID, name, institutionName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, pgx.ErrNoRows
	}
	return s.scanAccountRow(rows)
}

// FindByLastFourAndInstitution finds a disconnected account by last four digits and institution.
// This is more robust than name matching since users often rename accounts.
func (s *AccountStore) FindByLastFourAndInstitution(ctx context.Context, ledgerID uuid.UUID, lastFour, institutionName string) (*Account, error) {
	if lastFour == "" {
		return nil, sql.ErrNoRows
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, name, type, institution_name, institution_id, institution_logo_url,
			provider, external_account_id, connection_id, access_token,
			account_subtype, account_status, connection_status, last_synced_at, provider_metadata,
			last_four, account_number_masked,
			routing_number_ach, routing_number_wire,
			asset_subtype, purchase_date, purchase_price_cents, current_value_cents, depreciation_method,
			is_active, include_in_left_to_spend, created_at, updated_at
		FROM accounts
		WHERE ledger_id = $1
			AND last_four = $2
			AND LOWER(TRIM(institution_name)) = LOWER(TRIM($3))
			AND (external_account_id IS NULL OR external_account_id = '')
		LIMIT 1
	`, ledgerID, lastFour, institutionName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, pgx.ErrNoRows
	}
	return s.scanAccountRow(rows)
}

func (s *AccountStore) GetByConnectionID(ctx context.Context, connectionID string) ([]*Account, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, name, type, institution_name, institution_id, institution_logo_url,
			provider, external_account_id, connection_id, access_token,
			account_subtype, account_status, connection_status, last_synced_at, provider_metadata,
			last_four, account_number_masked,
			routing_number_ach, routing_number_wire,
			asset_subtype, purchase_date, purchase_price_cents, current_value_cents, depreciation_method,
			is_active, include_in_left_to_spend, created_at, updated_at
		FROM accounts WHERE connection_id = $1 OR COALESCE((provider_metadata->>'teller_enrollment_id')::text, '') = $1
	`, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		a, err := s.scanAccountRow(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}

	return accounts, rows.Err()
}

// GetAllWithProviderCredentials returns all accounts that have provider credentials configured
// This includes accounts with access_token (generic) or teller_access_token (backward compatibility)
func (s *AccountStore) GetAllWithProviderCredentials(ctx context.Context) ([]*Account, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, name, type, institution_name, institution_id, institution_logo_url,
			provider, external_account_id, connection_id, access_token,
			account_subtype, account_status, connection_status, last_synced_at, provider_metadata,
			last_four, account_number_masked,
			routing_number_ach, routing_number_wire,
			asset_subtype, purchase_date, purchase_price_cents, current_value_cents, depreciation_method,
			is_active, include_in_left_to_spend, created_at, updated_at
		FROM accounts 
		WHERE access_token IS NOT NULL AND access_token != ''
		  AND is_active = true
		  AND connection_status IS DISTINCT FROM 'disconnected'
		  AND connection_status IS DISTINCT FROM 'login_required'
		ORDER BY provider, institution_name, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		a, err := s.scanAccountRow(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}

	return accounts, rows.Err()
}

func (s *AccountStore) SetConnectionStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE accounts SET connection_status = $1, updated_at = NOW() WHERE id = $2`,
		status, id)
	return err
}

func (s *AccountStore) GetBalance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	var balance int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0) FROM entries WHERE account_id = $1
	`, accountID).Scan(&balance)
	return balance, err
}

// GetWithBalances returns accounts with their calculated balances
func (s *AccountStore) GetWithBalances(ctx context.Context, ledgerID uuid.UUID) ([]*Account, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.ledger_id, a.name, a.type, a.institution_name, a.institution_id, a.institution_logo_url,
			a.provider, a.external_account_id, a.connection_id, a.access_token,
			a.account_subtype, a.account_status, a.last_synced_at, a.provider_metadata,
			a.last_four, a.account_number_masked,
			a.routing_number_ach, a.routing_number_wire,
			a.is_active, a.created_at, a.updated_at,
			COALESCE(SUM(e.amount_cents), 0) as balance
		FROM accounts a
		LEFT JOIN entries e ON a.id = e.account_id
		WHERE a.ledger_id = $1
		GROUP BY a.id
		ORDER BY a.type, a.name
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		var a Account
		var instName, instID, instLogoURL, provider, extAccID, connID, accessToken sql.NullString
		var accSubtype, accStatus, lastFour, accNumMasked sql.NullString
		var routingACH, routingWire sql.NullString
		var lastSyncedAt sql.NullTime
		var providerMetadata []byte

		if err := rows.Scan(&a.ID, &a.LedgerID, &a.Name, &a.Type, &instName, &instID, &instLogoURL,
			&provider, &extAccID, &connID, &accessToken,
			&accSubtype, &accStatus, &lastSyncedAt, &providerMetadata,
			&lastFour, &accNumMasked,
			&routingACH, &routingWire,
			&a.IsActive, &a.CreatedAt, &a.UpdatedAt, &a.Balance); err != nil {
			return nil, err
		}

		a.InstitutionName = instName.String
		a.InstitutionID = instID.String
		a.InstitutionLogoURL = instLogoURL.String
		a.Provider = provider.String
		a.ExternalAccountID = extAccID.String
		a.ConnectionID = connID.String
		a.AccessToken = accessToken.String
		a.AccountSubtype = accSubtype.String
		a.AccountStatus = accStatus.String
		a.LastSyncedAt = nullTimePtr(lastSyncedAt)
		a.ProviderMetadata = providerMetadata
		// Map back to Teller fields for backward compatibility
		a.TellerAccountID = extAccID.String
		a.TellerEnrollmentID = connID.String
		a.TellerAccessToken = accessToken.String
		a.TellerSubtype = accSubtype.String
		a.TellerStatus = accStatus.String
		a.LastFour = lastFour.String
		a.AccountNumberMasked = accNumMasked.String
		a.RoutingNumberACH = routingACH.String
		a.RoutingNumberWire = routingWire.String

		accounts = append(accounts, &a)
	}

	return accounts, rows.Err()
}

func (s *AccountStore) getOrCreateSystemAccount(ctx context.Context, ledgerID uuid.UUID, name string, accType AccountType) (*Account, error) {
	accounts, err := s.GetByLedgerID(ctx, ledgerID)
	if err != nil {
		return nil, err
	}
	for _, acc := range accounts {
		if acc.Type == accType && acc.Name == name {
			return acc, nil
		}
	}
	acc := &Account{
		LedgerID: ledgerID,
		Name:     name,
		Type:     accType,
		IsActive: true,
	}
	if err := s.Create(ctx, acc); err != nil {
		return nil, err
	}
	return acc, nil
}

// GetOrCreateExpenseAccount returns the uncategorized expenses account for a ledger, creating it if needed.
func (s *AccountStore) GetOrCreateExpenseAccount(ctx context.Context, ledgerID uuid.UUID) (*Account, error) {
	return s.getOrCreateSystemAccount(ctx, ledgerID, "Uncategorized Expenses", AccountTypeExpense)
}

// GetOrCreateIncomeAccount returns the uncategorized income account for a ledger, creating it if needed.
func (s *AccountStore) GetOrCreateIncomeAccount(ctx context.Context, ledgerID uuid.UUID) (*Account, error) {
	return s.getOrCreateSystemAccount(ctx, ledgerID, "Uncategorized Income", AccountTypeIncome)
}
