package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ExportVersion = "1.3" // Updated to include reviewed_at for transactions

// Manifest contains metadata about the export
type Manifest struct {
	Version    string    `json:"version"`
	ExportedAt time.Time `json:"exported_at"`
	LedgerID   string    `json:"ledger_id"`
	LedgerName string    `json:"ledger_name"`
}

// ExportLedger represents a ledger in the export
type ExportLedger struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExportAccount represents an account in the export
type ExportAccount struct {
	ID                  string    `json:"id"`
	LedgerID            string    `json:"ledger_id"`
	Name                string    `json:"name"`
	Type                string    `json:"type"`
	InstitutionName     string    `json:"institution_name,omitempty"`
	Provider            string    `json:"provider,omitempty"`
	ExternalAccountID   string    `json:"external_account_id,omitempty"`
	ConnectionID        string    `json:"connection_id,omitempty"`
	AccountSubtype      string    `json:"account_subtype,omitempty"`
	AccountStatus       string    `json:"account_status,omitempty"`
	LastFour            string    `json:"last_four,omitempty"`
	AccountNumberMasked string    `json:"account_number_masked,omitempty"`
	RoutingNumberACH    string    `json:"routing_number_ach,omitempty"`
	RoutingNumberWire   string    `json:"routing_number_wire,omitempty"`
	IsActive            bool      `json:"is_active"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ExportTransaction represents a transaction in the export
type ExportTransaction struct {
	ID                        string     `json:"id"`
	LedgerID                  string     `json:"ledger_id"`
	Date                      string     `json:"date"` // YYYY-MM-DD
	Description               string     `json:"description"`
	DisplayTitle              string     `json:"display_title,omitempty"`
	Notes                     string     `json:"notes,omitempty"`
	TellerTransactionID       string     `json:"teller_transaction_id,omitempty"`
	TellerType                string     `json:"teller_type,omitempty"`
	TellerCategory            string     `json:"teller_category,omitempty"`
	TellerStatus              string     `json:"teller_status,omitempty"`
	CounterpartyName          string     `json:"counterparty_name,omitempty"`
	CounterpartyType          string     `json:"counterparty_type,omitempty"`
	RunningBalanceCents       int64      `json:"running_balance_cents,omitempty"`
	IsTransfer                bool       `json:"is_transfer"`
	TransferPairID            string     `json:"transfer_pair_id,omitempty"`
	CategorizationStatus      string     `json:"categorization_status,omitempty"`
	CategorizationError       string     `json:"categorization_error,omitempty"`
	CategorizationAttempts    int        `json:"categorization_attempts"`
	CategorizationQueuedAt    *time.Time `json:"categorization_queued_at,omitempty"`
	CategorizationCompletedAt *time.Time `json:"categorization_completed_at,omitempty"`
	// Entity fields (v1.2 - replaces merchant_id)
	EntityID             string `json:"entity_id,omitempty"`
	CounterpartyEntityID string `json:"counterparty_entity_id,omitempty"`
	IntermediaryEntityID string `json:"intermediary_entity_id,omitempty"`
	// Legacy field for backwards compatibility with v1.1 exports
	MerchantID         string `json:"merchant_id,omitempty"`
	RecurringPatternID string `json:"recurring_pattern_id,omitempty"`
	EnrichmentStatus   string `json:"enrichment_status,omitempty"`
	EnrichmentAttempts int        `json:"enrichment_attempts"`
	EnrichmentError    string     `json:"enrichment_error,omitempty"`
	EnrichedAt         *time.Time `json:"enriched_at,omitempty"`
	ReviewedAt         *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// ExportEntry represents an entry in the export
type ExportEntry struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	AccountID     string    `json:"account_id"`
	AmountCents   int64     `json:"amount_cents"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at"`
}

// ExportTag represents a tag in the export
type ExportTag struct {
	ID        string    `json:"id"`
	LedgerID  string    `json:"ledger_id"`
	ParentID  string    `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExportTransactionTag represents a transaction-tag mapping
type ExportTransactionTag struct {
	TransactionID string    `json:"transaction_id"`
	TagID         string    `json:"tag_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// ExportRule represents a categorization rule
type ExportRule struct {
	ID           string    `json:"id"`
	LedgerID     string    `json:"ledger_id"`
	Name         string    `json:"name"`
	Prompt       string    `json:"prompt,omitempty"`
	Examples     string    `json:"examples,omitempty"`
	MatchPattern string    `json:"match_pattern,omitempty"`
	IsRegex      bool      `json:"is_regex"`
	TagID        string    `json:"tag_id"`
	Priority     int       `json:"priority"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ExportPendingMatch represents a pending transfer match
type ExportPendingMatch struct {
	ID                     string     `json:"id"`
	TransactionID          string     `json:"transaction_id"`
	CandidateTransactionID string     `json:"candidate_transaction_id"`
	ConfidenceScore        float64    `json:"confidence_score"`
	MatchReasons           []string   `json:"match_reasons"`
	Status                 string     `json:"status"`
	CreatedAt              time.Time  `json:"created_at"`
	ReviewedAt             *time.Time `json:"reviewed_at,omitempty"`
}

// ExportEntity represents an entity (person, business, trust, etc.) in the export
type ExportEntity struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"` // person, business, trust, partnership, government
	Subtype        string          `json:"subtype,omitempty"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug,omitempty"`
	LogoURL        string          `json:"logo_url,omitempty"`
	Website        string          `json:"website,omitempty"`
	Description    string          `json:"description,omitempty"`
	ExternalID     string          `json:"external_id,omitempty"`
	ExternalSource string          `json:"external_source,omitempty"` // teller, manual
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	UserVerified   bool            `json:"user_verified"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ExportEntityRelationship represents a relationship between two entities
type ExportEntityRelationship struct {
	ID               string     `json:"id"`
	LedgerID         string     `json:"ledger_id"`
	EntityAID        string     `json:"entity_a_id"`
	EntityBID        string     `json:"entity_b_id"`
	RelationshipType string     `json:"relationship_type"` // spouse, partner, family, trustee, beneficiary, employer, self
	ValidFrom        *time.Time `json:"valid_from,omitempty"`
	ValidTo          *time.Time `json:"valid_to,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// ExportAccountEntityOwnership represents ownership of an account by an entity
type ExportAccountEntityOwnership struct {
	ID                  string    `json:"id"`
	AccountID           string    `json:"account_id"`
	EntityID            string    `json:"entity_id"`
	OwnershipPercentage float64   `json:"ownership_percentage"`
	Role                string    `json:"role"` // owner, trustee, beneficiary
	CreatedAt           time.Time `json:"created_at"`
}

// ExportRecurringPattern represents a recurring payment pattern
type ExportRecurringPattern struct {
	ID             string     `json:"id"`
	LedgerID       string     `json:"ledger_id"`
	EntityID       string     `json:"entity_id,omitempty"`
	MerchantID     string     `json:"merchant_id,omitempty"` // Legacy field for backwards compatibility
	Frequency      string     `json:"frequency,omitempty"`
	PredictedDates []string   `json:"predicted_dates,omitempty"` // JSON array of date strings
	AvgAmountCents int64      `json:"avg_amount_cents,omitempty"`
	LastSeenAt     *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Stats tracks what was exported or imported.
type Stats struct {
	Accounts                int
	Tags                    int
	Transactions            int
	Entries                 int
	Rules                   int
	TransactionTags         int
	PendingMatches          int
	Entities                int
	EntityRelationships     int
	AccountEntityOwnerships int
	RecurringPatterns       int
}

// Export exports all ledger data to a ZIP file
func Export(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) (*bytes.Buffer, *Stats, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Get ledger info
	ledger, err := exportLedger(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export ledger: %w", err)
	}

	// Create manifest
	manifest := Manifest{
		Version:    ExportVersion,
		ExportedAt: time.Now().UTC(),
		LedgerID:   ledgerID.String(),
		LedgerName: ledger.Name,
	}

	// Write manifest
	if err := writeJSON(zipWriter, "manifest.json", manifest); err != nil {
		return nil, nil, fmt.Errorf("write manifest: %w", err)
	}

	// Write ledger
	if err := writeJSON(zipWriter, "ledger.json", ledger); err != nil {
		return nil, nil, fmt.Errorf("write ledger: %w", err)
	}

	// Export accounts
	accounts, err := exportAccounts(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export accounts: %w", err)
	}
	if err := writeJSON(zipWriter, "accounts.json", accounts); err != nil {
		return nil, nil, fmt.Errorf("write accounts: %w", err)
	}

	// Export transactions
	transactions, err := exportTransactions(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export transactions: %w", err)
	}
	if err := writeJSON(zipWriter, "transactions.json", transactions); err != nil {
		return nil, nil, fmt.Errorf("write transactions: %w", err)
	}

	// Export entries
	entries, err := exportEntries(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export entries: %w", err)
	}
	if err := writeJSON(zipWriter, "entries.json", entries); err != nil {
		return nil, nil, fmt.Errorf("write entries: %w", err)
	}

	// Export tags
	tags, err := exportTags(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export tags: %w", err)
	}
	if err := writeJSON(zipWriter, "tags.json", tags); err != nil {
		return nil, nil, fmt.Errorf("write tags: %w", err)
	}

	// Export transaction tags
	txnTags, err := exportTransactionTags(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export transaction tags: %w", err)
	}
	if err := writeJSON(zipWriter, "transaction_tags.json", txnTags); err != nil {
		return nil, nil, fmt.Errorf("write transaction tags: %w", err)
	}

	// Export rules
	rules, err := exportRules(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export rules: %w", err)
	}
	if err := writeJSON(zipWriter, "rules.json", rules); err != nil {
		return nil, nil, fmt.Errorf("write rules: %w", err)
	}

	// Export pending matches
	matches, err := exportPendingMatches(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export pending matches: %w", err)
	}
	if err := writeJSON(zipWriter, "pending_matches.json", matches); err != nil {
		return nil, nil, fmt.Errorf("write pending matches: %w", err)
	}

	// Export entities (replaces merchants in v1.2)
	entities, err := exportEntities(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export entities: %w", err)
	}
	if err := writeJSON(zipWriter, "entities.json", entities); err != nil {
		return nil, nil, fmt.Errorf("write entities: %w", err)
	}

	// Export entity relationships
	entityRelationships, err := exportEntityRelationships(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export entity relationships: %w", err)
	}
	if err := writeJSON(zipWriter, "entity_relationships.json", entityRelationships); err != nil {
		return nil, nil, fmt.Errorf("write entity relationships: %w", err)
	}

	// Export account entity ownerships
	accountEntityOwnerships, err := exportAccountEntityOwnerships(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export account entity ownerships: %w", err)
	}
	if err := writeJSON(zipWriter, "account_entity_ownerships.json", accountEntityOwnerships); err != nil {
		return nil, nil, fmt.Errorf("write account entity ownerships: %w", err)
	}

	// Export recurring patterns
	recurringPatterns, err := exportRecurringPatterns(ctx, pool, ledgerID)
	if err != nil {
		return nil, nil, fmt.Errorf("export recurring patterns: %w", err)
	}
	if err := writeJSON(zipWriter, "recurring_patterns.json", recurringPatterns); err != nil {
		return nil, nil, fmt.Errorf("write recurring patterns: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, nil, fmt.Errorf("close zip: %w", err)
	}

	stats := &Stats{
		Accounts:                len(accounts),
		Tags:                    len(tags),
		Transactions:            len(transactions),
		Entries:                 len(entries),
		Rules:                   len(rules),
		TransactionTags:         len(txnTags),
		PendingMatches:          len(matches),
		Entities:                len(entities),
		EntityRelationships:     len(entityRelationships),
		AccountEntityOwnerships: len(accountEntityOwnerships),
		RecurringPatterns:       len(recurringPatterns),
	}

	slog.InfoContext(ctx, "export complete",
		"accounts", stats.Accounts,
		"tags", stats.Tags,
		"transactions", stats.Transactions,
		"entries", stats.Entries,
		"transaction_tags", stats.TransactionTags,
		"rules", stats.Rules,
		"entities", stats.Entities,
		"entity_relationships", stats.EntityRelationships,
		"account_entity_ownerships", stats.AccountEntityOwnerships,
		"recurring_patterns", stats.RecurringPatterns)

	return buf, stats, nil
}

// Import imports ledger data from a ZIP file, replacing existing data
func Import(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, r io.ReaderAt, size int64) (*Stats, error) {
	zipReader, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	// Read manifest
	var manifest Manifest
	if err := readJSON(zipReader, "manifest.json", &manifest); err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	// Read all data
	var ledger ExportLedger
	if err := readJSON(zipReader, "ledger.json", &ledger); err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}

	var accounts []ExportAccount
	if err := readJSON(zipReader, "accounts.json", &accounts); err != nil {
		return nil, fmt.Errorf("read accounts: %w", err)
	}

	var transactions []ExportTransaction
	if err := readJSON(zipReader, "transactions.json", &transactions); err != nil {
		return nil, fmt.Errorf("read transactions: %w", err)
	}

	var entries []ExportEntry
	if err := readJSON(zipReader, "entries.json", &entries); err != nil {
		return nil, fmt.Errorf("read entries: %w", err)
	}

	var tags []ExportTag
	if err := readJSON(zipReader, "tags.json", &tags); err != nil {
		return nil, fmt.Errorf("read tags: %w", err)
	}

	var txnTags []ExportTransactionTag
	if err := readJSON(zipReader, "transaction_tags.json", &txnTags); err != nil {
		return nil, fmt.Errorf("read transaction tags: %w", err)
	}

	var rules []ExportRule
	if err := readJSON(zipReader, "rules.json", &rules); err != nil {
		return nil, fmt.Errorf("read rules: %w", err)
	}

	var pendingMatches []ExportPendingMatch
	if err := readJSON(zipReader, "pending_matches.json", &pendingMatches); err != nil {
		return nil, fmt.Errorf("read pending matches: %w", err)
	}

	// Read v1.2 data (optional - may not exist in old exports)
	var entities []ExportEntity
	_ = readJSON(zipReader, "entities.json", &entities) // Ignore error for backwards compatibility

	var entityRelationships []ExportEntityRelationship
	_ = readJSON(zipReader, "entity_relationships.json", &entityRelationships)

	var accountEntityOwnerships []ExportAccountEntityOwnership
	_ = readJSON(zipReader, "account_entity_ownerships.json", &accountEntityOwnerships)

	var recurringPatterns []ExportRecurringPattern
	_ = readJSON(zipReader, "recurring_patterns.json", &recurringPatterns)

	slog.InfoContext(ctx, "importing from ZIP",
		"accounts", len(accounts),
		"tags", len(tags),
		"transactions", len(transactions),
		"entries", len(entries),
		"transaction_tags", len(txnTags),
		"rules", len(rules),
		"pending_matches", len(pendingMatches),
		"entities", len(entities),
		"entity_relationships", len(entityRelationships),
		"account_entity_ownerships", len(accountEntityOwnerships),
		"recurring_patterns", len(recurringPatterns))

	// Build ID mapping from old IDs to new IDs
	idMap := make(map[string]uuid.UUID)

	// Generate new ledger ID
	newLedgerID := uuid.New()
	idMap[ledger.ID] = newLedgerID

	// Generate new IDs for all entities
	for _, a := range accounts {
		idMap[a.ID] = uuid.New()
	}
	for _, t := range transactions {
		idMap[t.ID] = uuid.New()
	}
	for _, e := range entries {
		idMap[e.ID] = uuid.New()
	}
	for _, t := range tags {
		idMap[t.ID] = uuid.New()
	}
	for _, m := range pendingMatches {
		idMap[m.ID] = uuid.New()
	}
	for _, r := range rules {
		idMap[r.ID] = uuid.New()
	}
	for _, e := range entities {
		idMap[e.ID] = uuid.New()
	}
	for _, er := range entityRelationships {
		idMap[er.ID] = uuid.New()
	}
	for _, aeo := range accountEntityOwnerships {
		idMap[aeo.ID] = uuid.New()
	}
	for _, rp := range recurringPatterns {
		idMap[rp.ID] = uuid.New()
	}

	stats := &Stats{
		Accounts:                len(accounts),
		Tags:                    len(tags),
		Transactions:            len(transactions),
		Entries:                 len(entries),
		Rules:                   len(rules),
		TransactionTags:         len(txnTags),
		PendingMatches:          len(pendingMatches),
		Entities:                len(entities),
		EntityRelationships:     len(entityRelationships),
		AccountEntityOwnerships: len(accountEntityOwnerships),
		RecurringPatterns:       len(recurringPatterns),
	}

	// Import within a transaction
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// Get user's person entity (should exist after migration)
		var personEntityID uuid.UUID
		err := tx.QueryRow(ctx, `
			SELECT e.id FROM entities e
			JOIN user_entity_permissions uep ON e.id = uep.entity_id
			WHERE uep.user_id = $1 AND e.type = 'person' AND e.subtype = 'individual'
			LIMIT 1
		`, userID).Scan(&personEntityID)
		if err != nil {
			return fmt.Errorf("find user person entity: %w", err)
		}

		// Delete existing entity-ledger links for this user's entity (cascade deletes ledgers and all related data)
		_, err = tx.Exec(ctx, `
			DELETE FROM ledgers WHERE id IN (
				SELECT ledger_id FROM entity_ledgers WHERE entity_id = $1
			)
		`, personEntityID)
		if err != nil {
			return fmt.Errorf("delete existing ledgers: %w", err)
		}

		// Insert ledger
		_, err = tx.Exec(ctx, `
			INSERT INTO ledgers (id, user_id, name, currency, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, newLedgerID, userID, ledger.Name, ledger.Currency, ledger.CreatedAt, ledger.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert ledger: %w", err)
		}

		// Link ledger to user's person entity
		_, err = tx.Exec(ctx, `
			INSERT INTO entity_ledgers (id, entity_id, ledger_id, role, created_at)
			VALUES (gen_random_uuid(), $1, $2, 'owner', NOW())
		`, personEntityID, newLedgerID)
		if err != nil {
			return fmt.Errorf("link ledger to entity: %w", err)
		}

		// Insert accounts (without access tokens)
		for _, a := range accounts {
			_, err = tx.Exec(ctx, `
				INSERT INTO accounts (id, ledger_id, name, type, institution_name,
					provider, external_account_id, connection_id,
					account_subtype, account_status, last_four, account_number_masked,
					routing_number_ach, routing_number_wire,
					is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			`, idMap[a.ID], newLedgerID, a.Name, a.Type, models.NullString(a.InstitutionName),
				models.NullString(a.Provider), models.NullString(a.ExternalAccountID), models.NullString(a.ConnectionID),
				models.NullString(a.AccountSubtype), models.NullString(a.AccountStatus),
				models.NullString(a.LastFour), models.NullString(a.AccountNumberMasked),
				models.NullString(a.RoutingNumberACH), models.NullString(a.RoutingNumberWire),
				a.IsActive, a.CreatedAt, a.UpdatedAt)
			if err != nil {
				return fmt.Errorf("insert account %s: %w", a.Name, err)
			}
		}

		// Insert tags (topologically sorted so parents come before children)
		sortedTags := sortTagsByParent(tags)
		for _, t := range sortedTags {
			var parentID *uuid.UUID
			if t.ParentID != "" {
				newParentID := idMap[t.ParentID]
				parentID = &newParentID
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO tags (id, ledger_id, parent_id, name, color, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, idMap[t.ID], newLedgerID, parentID, t.Name, t.Color, t.CreatedAt, t.UpdatedAt)
			if err != nil {
				return fmt.Errorf("insert tag %s: %w", t.Name, err)
			}
		}

		// Insert entities (must come before transactions that reference them)
		for _, e := range entities {
			_, err = tx.Exec(ctx, `
				INSERT INTO entities (id, type, subtype, name, slug, logo_url, website, description,
					external_id, external_source, metadata, user_verified, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			`, idMap[e.ID], e.Type, models.NullString(e.Subtype), e.Name, models.NullString(e.Slug),
				models.NullString(e.LogoURL), models.NullString(e.Website), models.NullString(e.Description),
				models.NullString(e.ExternalID), models.NullString(e.ExternalSource), e.Metadata,
				e.UserVerified, e.CreatedAt, e.UpdatedAt)
			if err != nil {
				return fmt.Errorf("insert entity %s: %w", e.Name, err)
			}
		}

		// Insert recurring patterns (must come before transactions that reference them)
		for _, rp := range recurringPatterns {
			var entityID *uuid.UUID
			// Support both new EntityID and legacy MerchantID fields
			if rp.EntityID != "" {
				newEntityID := idMap[rp.EntityID]
				entityID = &newEntityID
			} else if rp.MerchantID != "" {
				// Backwards compatibility: merchant_id maps to entity_id
				newEntityID := idMap[rp.MerchantID]
				entityID = &newEntityID
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO recurring_patterns (id, ledger_id, entity_id, frequency,
					predicted_dates, avg_amount_cents, last_seen_at, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`, idMap[rp.ID], newLedgerID, entityID, models.NullString(rp.Frequency),
				rp.PredictedDates, models.NullInt64(rp.AvgAmountCents), rp.LastSeenAt, rp.CreatedAt, rp.UpdatedAt)
			if err != nil {
				return fmt.Errorf("insert recurring pattern %s: %w", rp.ID, err)
			}
		}

		// Insert transactions (first pass without transfer_pair_id)
		for _, t := range transactions {
			date, _ := time.Parse("2006-01-02", t.Date)

			// Handle optional entity_id, counterparty_entity_id, intermediary_entity_id, and recurring_pattern_id
			var entityID, counterpartyEntityID, intermediaryEntityID, recurringPatternID *uuid.UUID

			// Support both new EntityID and legacy MerchantID fields
			if t.EntityID != "" {
				newEntityID := idMap[t.EntityID]
				entityID = &newEntityID
			} else if t.MerchantID != "" {
				// Backwards compatibility: merchant_id maps to entity_id
				newEntityID := idMap[t.MerchantID]
				entityID = &newEntityID
			}
			if t.CounterpartyEntityID != "" {
				newCounterpartyEntityID := idMap[t.CounterpartyEntityID]
				counterpartyEntityID = &newCounterpartyEntityID
			}
			if t.IntermediaryEntityID != "" {
				newIntermediaryEntityID := idMap[t.IntermediaryEntityID]
				intermediaryEntityID = &newIntermediaryEntityID
			}
			if t.RecurringPatternID != "" {
				newRecurringPatternID := idMap[t.RecurringPatternID]
				recurringPatternID = &newRecurringPatternID
			}

			_, err = tx.Exec(ctx, `
				INSERT INTO transactions (id, ledger_id, date, description, display_title, notes,
					teller_transaction_id, teller_type, teller_category, teller_status,
					counterparty_name, counterparty_type, running_balance_cents,
					is_transfer, transfer_pair_id,
					categorization_status, categorization_error, categorization_attempts,
					categorization_queued_at, categorization_completed_at,
					entity_id, counterparty_entity_id, intermediary_entity_id, recurring_pattern_id,
					enrichment_status, enrichment_attempts, enrichment_error, enriched_at,
					reviewed_at,
					created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NULL, $15, $16, $17, $18, $19,
					$20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30)
			`, idMap[t.ID], newLedgerID, date, t.Description, models.NullString(t.DisplayTitle), models.NullString(t.Notes),
				models.NullString(t.TellerTransactionID), models.NullString(t.TellerType),
				models.NullString(t.TellerCategory), models.NullString(t.TellerStatus),
				models.NullString(t.CounterpartyName), models.NullString(t.CounterpartyType),
				models.NullInt64(t.RunningBalanceCents),
				t.IsTransfer,
				models.NullString(t.CategorizationStatus), models.NullString(t.CategorizationError),
				t.CategorizationAttempts, t.CategorizationQueuedAt, t.CategorizationCompletedAt,
				entityID, counterpartyEntityID, intermediaryEntityID, recurringPatternID,
				models.NullString(t.EnrichmentStatus), t.EnrichmentAttempts, models.NullString(t.EnrichmentError), t.EnrichedAt,
				t.ReviewedAt,
				t.CreatedAt, t.UpdatedAt)
			if err != nil {
				return fmt.Errorf("insert transaction %s: %w", t.Description, err)
			}
		}

		// Update transfer_pair_id references
		for _, t := range transactions {
			if t.TransferPairID != "" {
				newPairID := idMap[t.TransferPairID]
				_, err = tx.Exec(ctx, `
					UPDATE transactions SET transfer_pair_id = $2 WHERE id = $1
				`, idMap[t.ID], newPairID)
				if err != nil {
					return fmt.Errorf("update transfer pair: %w", err)
				}
			}
		}

		// Insert entries
		for _, e := range entries {
			_, err = tx.Exec(ctx, `
				INSERT INTO entries (id, transaction_id, account_id, amount_cents, currency, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, idMap[e.ID], idMap[e.TransactionID], idMap[e.AccountID], e.AmountCents, e.Currency, e.CreatedAt)
			if err != nil {
				return fmt.Errorf("insert entry: %w", err)
			}
		}

		// Insert transaction tags
		for _, tt := range txnTags {
			_, err = tx.Exec(ctx, `
				INSERT INTO transaction_tags (transaction_id, tag_id, created_at)
				VALUES ($1, $2, $3)
			`, idMap[tt.TransactionID], idMap[tt.TagID], tt.CreatedAt)
			if err != nil {
				return fmt.Errorf("insert transaction tag: %w", err)
			}
		}

		// Insert rules
		for _, r := range rules {
			_, err = tx.Exec(ctx, `
				INSERT INTO categorization_rules (id, ledger_id, name, prompt, examples, match_pattern, is_regex, tag_id, priority, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`, idMap[r.ID], newLedgerID, r.Name, models.NullString(r.Prompt), models.NullString(r.Examples),
				models.NullString(r.MatchPattern), r.IsRegex, idMap[r.TagID], r.Priority, r.IsActive,
				r.CreatedAt, r.UpdatedAt)
			if err != nil {
				return fmt.Errorf("insert rule %s: %w", r.Name, err)
			}
		}

		// Insert pending matches
		for _, m := range pendingMatches {
			_, err = tx.Exec(ctx, `
				INSERT INTO pending_transfer_matches (id, transaction_id, candidate_transaction_id, confidence_score, match_reasons, status, created_at, reviewed_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`, idMap[m.ID], idMap[m.TransactionID], idMap[m.CandidateTransactionID],
				m.ConfidenceScore, m.MatchReasons, m.Status, m.CreatedAt, m.ReviewedAt)
			if err != nil {
				return fmt.Errorf("insert pending match: %w", err)
			}
		}

		// Insert entity relationships
		for _, er := range entityRelationships {
			_, err = tx.Exec(ctx, `
				INSERT INTO entity_relationships (id, ledger_id, entity_a_id, entity_b_id, relationship_type, valid_from, valid_to, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`, idMap[er.ID], newLedgerID, idMap[er.EntityAID], idMap[er.EntityBID],
				er.RelationshipType, er.ValidFrom, er.ValidTo, er.CreatedAt)
			if err != nil {
				return fmt.Errorf("insert entity relationship: %w", err)
			}
		}

		// Insert account entity ownerships
		for _, aeo := range accountEntityOwnerships {
			_, err = tx.Exec(ctx, `
				INSERT INTO account_entity_ownership (id, account_id, entity_id, ownership_percentage, role, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, idMap[aeo.ID], idMap[aeo.AccountID], idMap[aeo.EntityID],
				aeo.OwnershipPercentage, aeo.Role, aeo.CreatedAt)
			if err != nil {
				return fmt.Errorf("insert account entity ownership: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return stats, nil
}

// Helper functions

func writeJSON(zw *zip.Writer, filename string, data any) error {
	w, err := zw.Create(filename)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func readJSON(zr *zip.Reader, filename string, v any) error {
	for _, f := range zr.File {
		if f.Name == filename {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			return json.NewDecoder(rc).Decode(v)
		}
	}
	return fmt.Errorf("file not found in zip: %s", filename)
}

// sortTagsByParent performs a topological sort on tags so parent tags come before their children
func sortTagsByParent(tags []ExportTag) []ExportTag {
	if len(tags) == 0 {
		return tags
	}

	// Build lookup maps
	tagByID := make(map[string]ExportTag)
	for _, t := range tags {
		tagByID[t.ID] = t
	}

	// Track which tags have been added to result
	added := make(map[string]bool)
	result := make([]ExportTag, 0, len(tags))

	// Helper function to add a tag and its ancestors first
	var addTag func(t ExportTag)
	addTag = func(t ExportTag) {
		if added[t.ID] {
			return
		}

		// If has parent, add parent first
		if t.ParentID != "" {
			if parent, ok := tagByID[t.ParentID]; ok {
				addTag(parent)
			}
		}

		// Add this tag
		added[t.ID] = true
		result = append(result, t)
	}

	// Process all tags
	for _, t := range tags {
		addTag(t)
	}

	return result
}

// Export helper functions

func exportLedger(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) (*ExportLedger, error) {
	var l ExportLedger
	err := pool.QueryRow(ctx, `
		SELECT id, name, currency, created_at, updated_at
		FROM ledgers WHERE id = $1
	`, ledgerID).Scan(&l.ID, &l.Name, &l.Currency, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func exportAccounts(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) ([]ExportAccount, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, ledger_id, name, type, institution_name,
			provider, external_account_id, connection_id,
			account_subtype, account_status, last_four, account_number_masked,
			routing_number_ach, routing_number_wire,
			is_active, created_at, updated_at
		FROM accounts WHERE ledger_id = $1
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []ExportAccount
	for rows.Next() {
		var a ExportAccount
		var instName, provider, extAccID, connID sql.NullString
		var accSubtype, accStatus, lastFour, accNumMasked sql.NullString
		var routingACH, routingWire sql.NullString

		if err := rows.Scan(&a.ID, &a.LedgerID, &a.Name, &a.Type, &instName,
			&provider, &extAccID, &connID,
			&accSubtype, &accStatus, &lastFour, &accNumMasked,
			&routingACH, &routingWire,
			&a.IsActive, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}

		a.InstitutionName = instName.String
		a.Provider = provider.String
		a.ExternalAccountID = extAccID.String
		a.ConnectionID = connID.String
		a.AccountSubtype = accSubtype.String
		a.AccountStatus = accStatus.String
		a.LastFour = lastFour.String
		a.AccountNumberMasked = accNumMasked.String
		a.RoutingNumberACH = routingACH.String
		a.RoutingNumberWire = routingWire.String

		accounts = append(accounts, a)
	}

	return accounts, rows.Err()
}

func exportTransactions(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) ([]ExportTransaction, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, ledger_id, date, description, display_title, notes,
			teller_transaction_id, teller_type, teller_category, teller_status,
			counterparty_name, counterparty_type, running_balance_cents,
			is_transfer, transfer_pair_id,
			categorization_status, categorization_error, categorization_attempts,
			categorization_queued_at, categorization_completed_at,
			entity_id, counterparty_entity_id, intermediary_entity_id, recurring_pattern_id,
			enrichment_status, enrichment_attempts, enrichment_error, enriched_at,
			reviewed_at,
			created_at, updated_at
		FROM transactions WHERE ledger_id = $1
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []ExportTransaction
	for rows.Next() {
		var t ExportTransaction
		var date time.Time
		var displayTitle, notes, tellerTxnID, tellerType, tellerCategory, tellerStatus sql.NullString
		var counterpartyName, counterpartyType sql.NullString
		var runningBalanceCents sql.NullInt64
		var transferPairID, entityID, counterpartyEntityID, intermediaryEntityID, recurringPatternID *uuid.UUID
		var catStatus, catError sql.NullString
		var catAttempts sql.NullInt64
		var catQueuedAt, catCompletedAt sql.NullTime
		var enrichmentStatus, enrichmentError sql.NullString
		var enrichmentAttempts sql.NullInt64
		var enrichedAt, reviewedAt sql.NullTime

		if err := rows.Scan(&t.ID, &t.LedgerID, &date, &t.Description, &displayTitle, &notes,
			&tellerTxnID, &tellerType, &tellerCategory, &tellerStatus,
			&counterpartyName, &counterpartyType, &runningBalanceCents,
			&t.IsTransfer, &transferPairID,
			&catStatus, &catError, &catAttempts,
			&catQueuedAt, &catCompletedAt,
			&entityID, &counterpartyEntityID, &intermediaryEntityID, &recurringPatternID,
			&enrichmentStatus, &enrichmentAttempts, &enrichmentError, &enrichedAt,
			&reviewedAt,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}

		t.Date = date.Format("2006-01-02")
		t.DisplayTitle = displayTitle.String
		t.Notes = notes.String
		t.TellerTransactionID = tellerTxnID.String
		t.TellerType = tellerType.String
		t.TellerCategory = tellerCategory.String
		t.TellerStatus = tellerStatus.String
		t.CounterpartyName = counterpartyName.String
		t.CounterpartyType = counterpartyType.String
		t.RunningBalanceCents = runningBalanceCents.Int64
		if transferPairID != nil {
			t.TransferPairID = transferPairID.String()
		}
		t.CategorizationStatus = catStatus.String
		t.CategorizationError = catError.String
		t.CategorizationAttempts = int(catAttempts.Int64)
		if catQueuedAt.Valid {
			t.CategorizationQueuedAt = &catQueuedAt.Time
		}
		if catCompletedAt.Valid {
			t.CategorizationCompletedAt = &catCompletedAt.Time
		}
		if entityID != nil {
			t.EntityID = entityID.String()
		}
		if counterpartyEntityID != nil {
			t.CounterpartyEntityID = counterpartyEntityID.String()
		}
		if intermediaryEntityID != nil {
			t.IntermediaryEntityID = intermediaryEntityID.String()
		}
		if recurringPatternID != nil {
			t.RecurringPatternID = recurringPatternID.String()
		}
		t.EnrichmentStatus = enrichmentStatus.String
		t.EnrichmentAttempts = int(enrichmentAttempts.Int64)
		t.EnrichmentError = enrichmentError.String
		if enrichedAt.Valid {
			t.EnrichedAt = &enrichedAt.Time
		}
		if reviewedAt.Valid {
			t.ReviewedAt = &reviewedAt.Time
		}

		transactions = append(transactions, t)
	}

	return transactions, rows.Err()
}

func exportEntries(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) ([]ExportEntry, error) {
	rows, err := pool.Query(ctx, `
		SELECT e.id, e.transaction_id, e.account_id, e.amount_cents, e.currency, e.created_at
		FROM entries e
		JOIN transactions t ON e.transaction_id = t.id
		WHERE t.ledger_id = $1
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ExportEntry
	for rows.Next() {
		var e ExportEntry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.AmountCents, &e.Currency, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}

func exportTags(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) ([]ExportTag, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, ledger_id, parent_id, name, color, created_at, updated_at
		FROM tags WHERE ledger_id = $1
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []ExportTag
	for rows.Next() {
		var t ExportTag
		var parentID *uuid.UUID
		if err := rows.Scan(&t.ID, &t.LedgerID, &parentID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if parentID != nil {
			t.ParentID = parentID.String()
		}
		tags = append(tags, t)
	}

	return tags, rows.Err()
}

func exportTransactionTags(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) ([]ExportTransactionTag, error) {
	rows, err := pool.Query(ctx, `
		SELECT tt.transaction_id, tt.tag_id, tt.created_at
		FROM transaction_tags tt
		JOIN transactions t ON tt.transaction_id = t.id
		WHERE t.ledger_id = $1
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txnTags []ExportTransactionTag
	for rows.Next() {
		var tt ExportTransactionTag
		if err := rows.Scan(&tt.TransactionID, &tt.TagID, &tt.CreatedAt); err != nil {
			return nil, err
		}
		txnTags = append(txnTags, tt)
	}

	return txnTags, rows.Err()
}

func exportRules(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) ([]ExportRule, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, ledger_id, name, prompt, examples, match_pattern, is_regex, tag_id, priority, is_active, created_at, updated_at
		FROM categorization_rules WHERE ledger_id = $1
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []ExportRule
	for rows.Next() {
		var r ExportRule
		var prompt, examples, matchPattern sql.NullString
		if err := rows.Scan(&r.ID, &r.LedgerID, &r.Name, &prompt, &examples, &matchPattern, &r.IsRegex, &r.TagID, &r.Priority, &r.IsActive, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Prompt = prompt.String
		r.Examples = examples.String
		r.MatchPattern = matchPattern.String
		rules = append(rules, r)
	}

	return rules, rows.Err()
}

func exportPendingMatches(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) ([]ExportPendingMatch, error) {
	rows, err := pool.Query(ctx, `
		SELECT pm.id, pm.transaction_id, pm.candidate_transaction_id, pm.confidence_score,
			pm.match_reasons, pm.status, pm.created_at, pm.reviewed_at
		FROM pending_transfer_matches pm
		JOIN transactions t ON pm.transaction_id = t.id
		WHERE t.ledger_id = $1
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []ExportPendingMatch
	for rows.Next() {
		var m ExportPendingMatch
		var reviewedAt sql.NullTime
		if err := rows.Scan(&m.ID, &m.TransactionID, &m.CandidateTransactionID, &m.ConfidenceScore,
			&m.MatchReasons, &m.Status, &m.CreatedAt, &reviewedAt); err != nil {
			return nil, err
		}
		if reviewedAt.Valid {
			m.ReviewedAt = &reviewedAt.Time
		}
		matches = append(matches, m)
	}

	return matches, rows.Err()
}

func exportEntities(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) ([]ExportEntity, error) {
	// Export entities that are referenced by transactions in this ledger
	// This includes entity_id, counterparty_entity_id, and intermediary_entity_id
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT e.id, e.type, e.subtype, e.name, e.slug, e.logo_url, e.website, e.description,
			e.external_id, e.external_source, e.metadata, e.user_verified, e.created_at, e.updated_at
		FROM entities e
		WHERE e.id IN (
			SELECT entity_id FROM transactions WHERE ledger_id = $1 AND entity_id IS NOT NULL
			UNION
			SELECT counterparty_entity_id FROM transactions WHERE ledger_id = $1 AND counterparty_entity_id IS NOT NULL
			UNION
			SELECT intermediary_entity_id FROM transactions WHERE ledger_id = $1 AND intermediary_entity_id IS NOT NULL
			UNION
			SELECT entity_id FROM recurring_patterns WHERE ledger_id = $1 AND entity_id IS NOT NULL
		)
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []ExportEntity
	for rows.Next() {
		var e ExportEntity
		var subtype, slug, logoURL, website, description, externalID, externalSource sql.NullString

		if err := rows.Scan(&e.ID, &e.Type, &subtype, &e.Name, &slug, &logoURL, &website, &description,
			&externalID, &externalSource, &e.Metadata, &e.UserVerified, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}

		e.Subtype = subtype.String
		e.Slug = slug.String
		e.LogoURL = logoURL.String
		e.Website = website.String
		e.Description = description.String
		e.ExternalID = externalID.String
		e.ExternalSource = externalSource.String

		entities = append(entities, e)
	}

	return entities, rows.Err()
}

func exportEntityRelationships(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) ([]ExportEntityRelationship, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, ledger_id, entity_a_id, entity_b_id, relationship_type, valid_from, valid_to, created_at
		FROM entity_relationships
		WHERE ledger_id = $1
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []ExportEntityRelationship
	for rows.Next() {
		var er ExportEntityRelationship
		var validFrom, validTo sql.NullTime

		if err := rows.Scan(&er.ID, &er.LedgerID, &er.EntityAID, &er.EntityBID,
			&er.RelationshipType, &validFrom, &validTo, &er.CreatedAt); err != nil {
			return nil, err
		}

		if validFrom.Valid {
			er.ValidFrom = &validFrom.Time
		}
		if validTo.Valid {
			er.ValidTo = &validTo.Time
		}

		relationships = append(relationships, er)
	}

	return relationships, rows.Err()
}

func exportAccountEntityOwnerships(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) ([]ExportAccountEntityOwnership, error) {
	// Export account entity ownerships for accounts in this ledger
	rows, err := pool.Query(ctx, `
		SELECT aeo.id, aeo.account_id, aeo.entity_id, aeo.ownership_percentage, aeo.role, aeo.created_at
		FROM account_entity_ownership aeo
		JOIN accounts a ON aeo.account_id = a.id
		WHERE a.ledger_id = $1
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ownerships []ExportAccountEntityOwnership
	for rows.Next() {
		var aeo ExportAccountEntityOwnership

		if err := rows.Scan(&aeo.ID, &aeo.AccountID, &aeo.EntityID,
			&aeo.OwnershipPercentage, &aeo.Role, &aeo.CreatedAt); err != nil {
			return nil, err
		}

		ownerships = append(ownerships, aeo)
	}

	return ownerships, rows.Err()
}

func exportRecurringPatterns(ctx context.Context, pool *pgxpool.Pool, ledgerID uuid.UUID) ([]ExportRecurringPattern, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, ledger_id, entity_id, frequency,
			predicted_dates, avg_amount_cents, last_seen_at, created_at, updated_at
		FROM recurring_patterns
		WHERE ledger_id = $1
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patterns []ExportRecurringPattern
	for rows.Next() {
		var rp ExportRecurringPattern
		var entityID *uuid.UUID
		var frequency sql.NullString
		var predictedDates []string
		var avgAmount sql.NullInt64
		var lastSeenAt sql.NullTime

		if err := rows.Scan(&rp.ID, &rp.LedgerID, &entityID, &frequency,
			&predictedDates, &avgAmount, &lastSeenAt, &rp.CreatedAt, &rp.UpdatedAt); err != nil {
			return nil, err
		}

		if entityID != nil {
			rp.EntityID = entityID.String()
		}
		rp.Frequency = frequency.String
		rp.PredictedDates = predictedDates
		rp.AvgAmountCents = avgAmount.Int64
		if lastSeenAt.Valid {
			rp.LastSeenAt = &lastSeenAt.Time
		}

		patterns = append(patterns, rp)
	}

	return patterns, rows.Err()
}

