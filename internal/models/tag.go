package models

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Tag struct {
	ID             uuid.UUID  `json:"id"`
	LedgerID       uuid.UUID  `json:"ledger_id"`
	ParentID       *uuid.UUID `json:"parent_id,omitempty"`
	Name           string     `json:"name"`
	Color          string     `json:"color"`
	ExcludeFromPnL bool       `json:"exclude_from_pnl"` // True for balance sheet movements (transfers, payments)
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Loaded separately
	Children []*Tag `json:"children,omitempty"`
}

type TagStore struct {
	pool *pgxpool.Pool
}

func NewTagStore(pool *pgxpool.Pool) *TagStore {
	return &TagStore{pool: pool}
}

func (s *TagStore) Create(ctx context.Context, tag *Tag) error {
	if tag.ID == uuid.Nil {
		tag.ID = uuid.New()
	}
	if tag.Color == "" {
		tag.Color = "#6366f1" // Default indigo
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO tags (id, ledger_id, parent_id, name, color, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, tag.ID, tag.LedgerID, tag.ParentID, tag.Name, tag.Color, time.Now(), time.Now())

	return err
}

func (s *TagStore) Update(ctx context.Context, tag *Tag) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE tags SET parent_id = $2, name = $3, color = $4, updated_at = $5
		WHERE id = $1
	`, tag.ID, tag.ParentID, tag.Name, tag.Color, time.Now())

	return err
}

func (s *TagStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM tags WHERE id = $1`, id)
	return err
}

func (s *TagStore) GetByID(ctx context.Context, id uuid.UUID) (*Tag, error) {
	var t Tag
	var parentID *uuid.UUID

	err := s.pool.QueryRow(ctx, `
		SELECT id, ledger_id, parent_id, name, color, created_at, updated_at
		FROM tags WHERE id = $1
	`, id).Scan(&t.ID, &t.LedgerID, &parentID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		return nil, err
	}
	t.ParentID = parentID

	return &t, nil
}

func (s *TagStore) GetByLedgerID(ctx context.Context, ledgerID uuid.UUID) ([]*Tag, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, parent_id, name, color, created_at, updated_at
		FROM tags WHERE ledger_id = $1 ORDER BY name
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		var t Tag
		var parentID *uuid.UUID
		if err := rows.Scan(&t.ID, &t.LedgerID, &parentID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.ParentID = parentID
		tags = append(tags, &t)
	}

	return tags, rows.Err()
}

// GetHierarchy returns tags organized as a tree (top-level tags with children)
func (s *TagStore) GetHierarchy(ctx context.Context, ledgerID uuid.UUID) ([]*Tag, error) {
	tags, err := s.GetByLedgerID(ctx, ledgerID)
	if err != nil {
		return nil, err
	}

	// Build lookup map
	tagMap := make(map[uuid.UUID]*Tag)
	for _, t := range tags {
		tagMap[t.ID] = t
	}

	// Build tree
	var roots []*Tag
	for _, t := range tags {
		if t.ParentID == nil {
			roots = append(roots, t)
		} else if parent, ok := tagMap[*t.ParentID]; ok {
			parent.Children = append(parent.Children, t)
		}
	}

	// Sort roots and children alphabetically
	sortTags(roots)

	return roots, nil
}

// sortTags sorts tags alphabetically and recursively sorts children
func sortTags(tags []*Tag) {
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})
	for _, t := range tags {
		if len(t.Children) > 0 {
			sortTags(t.Children)
		}
	}
}

// AddTagToTransaction adds a tag to a transaction (metadata only, does not update entries)
func (s *TagStore) AddTagToTransaction(ctx context.Context, transactionID, tagID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO transaction_tags (transaction_id, tag_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, transactionID, tagID, time.Now())

	return err
}

// CategorizeTransaction adds a tag AND updates the entry to point to the correct category account.
// This moves the entry from any income/expense account to a category-specific account.
// Handles both initial categorization (from Uncategorized) and re-categorization.
func (s *TagStore) CategorizeTransaction(ctx context.Context, transactionID, tagID uuid.UUID) error {
	// Start a transaction for atomic updates
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Add the tag
	_, err = tx.Exec(ctx, `
		INSERT INTO transaction_tags (transaction_id, tag_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, transactionID, tagID, time.Now())
	if err != nil {
		return err
	}

	// Get tag info including parent (to determine if this is income or expense category)
	var tagName string
	var parentName *string
	var ledgerID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT t.name, p.name, txn.ledger_id
		FROM tags t
		JOIN transactions txn ON txn.id = $1
		LEFT JOIN tags p ON t.parent_id = p.id
		WHERE t.id = $2
	`, transactionID, tagID).Scan(&tagName, &parentName, &ledgerID)
	if err != nil {
		return err
	}

	// Determine target account type based on the parent category
	// Income parent → income account
	// Transfers parent with investment → asset account (not expense!)
	// Everything else → expense account
	var targetAccountType AccountType = AccountTypeExpense
	var targetAccountName string = tagName // Usually the tag name becomes the account name

	if parentName != nil {
		switch *parentName {
		case "Income":
			targetAccountType = AccountTypeIncome
		case "Transfers":
			// Transfer categories should go to asset accounts, not expense
			// This prevents investment transfers from being counted as spending
			if tagName == "Investment Transfer" {
				targetAccountType = AccountTypeAsset
				targetAccountName = "Investments" // Generic asset account for external investments
			}
			// Other transfer types (Internal Transfer, Person Payment) stay as expense
			// for now since they may need manual transfer matching
		}
	}

	// Find the contra entry (any income or expense account, not just Uncategorized)
	// This allows re-categorization from one category to another
	var entryID uuid.UUID
	var currentAccountType AccountType
	var currentAccountName string
	err = tx.QueryRow(ctx, `
		SELECT e.id, a.type, a.name
		FROM entries e
		JOIN accounts a ON e.account_id = a.id
		WHERE e.transaction_id = $1
		AND a.type IN ('expense', 'income')
		LIMIT 1
	`, transactionID).Scan(&entryID, &currentAccountType, &currentAccountName)
	if err != nil {
		// No income/expense entry found - probably a transfer
		// Just commit the tag addition
		return tx.Commit(ctx)
	}

	// Skip if already pointing to the correct category account
	if currentAccountName == targetAccountName && currentAccountType == targetAccountType {
		return tx.Commit(ctx)
	}

	// Find or create the category account with the correct type
	var categoryAccountID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM accounts
		WHERE ledger_id = $1 AND name = $2 AND type = $3
	`, ledgerID, targetAccountName, targetAccountType).Scan(&categoryAccountID)
	if err != nil {
		// Account doesn't exist - create it
		categoryAccountID = uuid.New()
		_, err = tx.Exec(ctx, `
			INSERT INTO accounts (id, ledger_id, name, type, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, true, $5, $5)
		`, categoryAccountID, ledgerID, targetAccountName, targetAccountType, time.Now())
		if err != nil {
			return err
		}
	}

	// Handle sign logic based on target account type
	if targetAccountType == AccountTypeAsset {
		// When moving to an asset account (like Investments), we need to ensure
		// the entry balances with the bank account entry.
		// Asset entry = negative of bank account entry (double-entry bookkeeping)
		// Find the bank account (asset/liability) entry and set our amount to its negative
		var bankEntryAmount int64
		err = tx.QueryRow(ctx, `
			SELECT e.amount_cents
			FROM entries e
			JOIN accounts a ON e.account_id = a.id
			WHERE e.transaction_id = $1
			AND e.id != $2
			AND a.type IN ('asset', 'liability')
			LIMIT 1
		`, transactionID, entryID).Scan(&bankEntryAmount)
		if err != nil {
			// Fall back to keeping current amount
			_, err = tx.Exec(ctx, `
				UPDATE entries SET account_id = $2 WHERE id = $1
			`, entryID, categoryAccountID)
		} else {
			// Set the investment entry to the negative of the bank entry
			_, err = tx.Exec(ctx, `
				UPDATE entries SET account_id = $2, amount_cents = $3 WHERE id = $1
			`, entryID, categoryAccountID, -bankEntryAmount)
		}
	} else {
		// For income/expense categories, use the original sign flip logic
		// This handles statement credits: moving from income (+$25) to expense (-$25)
		// ensures the credit REDUCES expenses rather than increasing them.
		shouldFlipSign := false
		if currentAccountType == AccountTypeIncome && targetAccountType == AccountTypeExpense {
			shouldFlipSign = true
		} else if currentAccountType == AccountTypeExpense && targetAccountType == AccountTypeIncome {
			shouldFlipSign = true
		}

		if shouldFlipSign {
			_, err = tx.Exec(ctx, `
				UPDATE entries SET account_id = $2, amount_cents = -amount_cents WHERE id = $1
			`, entryID, categoryAccountID)
		} else {
			_, err = tx.Exec(ctx, `
				UPDATE entries SET account_id = $2 WHERE id = $1
			`, entryID, categoryAccountID)
		}
	}
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// RemoveTagFromTransaction removes a tag from a transaction
func (s *TagStore) RemoveTagFromTransaction(ctx context.Context, transactionID, tagID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM transaction_tags WHERE transaction_id = $1 AND tag_id = $2
	`, transactionID, tagID)

	return err
}

// SetTransactionTags replaces all tags on a transaction
func (s *TagStore) SetTransactionTags(ctx context.Context, transactionID uuid.UUID, tagIDs []uuid.UUID) error {
	// Delete existing
	_, err := s.pool.Exec(ctx, `DELETE FROM transaction_tags WHERE transaction_id = $1`, transactionID)
	if err != nil {
		return err
	}

	// Add new
	for _, tagID := range tagIDs {
		if err := s.AddTagToTransaction(ctx, transactionID, tagID); err != nil {
			return err
		}
	}

	return nil
}

// GetTagUsageCount returns the number of transactions using a single tag.
func (s *TagStore) GetTagUsageCount(ctx context.Context, tagID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transaction_tags WHERE tag_id = $1`,
		tagID,
	).Scan(&count)
	return count, err
}

// GetTagUsageCounts returns the number of transactions using each tag
func (s *TagStore) GetTagUsageCounts(ctx context.Context, ledgerID uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, COUNT(tt.transaction_id)
		FROM tags t
		LEFT JOIN transaction_tags tt ON t.id = tt.tag_id
		WHERE t.ledger_id = $1
		GROUP BY t.id
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[uuid.UUID]int)
	for rows.Next() {
		var id uuid.UUID
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		counts[id] = count
	}

	return counts, rows.Err()
}

// BulkCategorizeTransactions applies a tag to multiple transactions at once.
// This is used for bulk tagging from the transactions list UI.
// Returns the number of transactions successfully tagged.
func (s *TagStore) BulkCategorizeTransactions(ctx context.Context, transactionIDs []uuid.UUID, tagID uuid.UUID) (int, error) {
	if len(transactionIDs) == 0 {
		return 0, nil
	}

	count := 0
	for _, txnID := range transactionIDs {
		if err := s.CategorizeTransaction(ctx, txnID, tagID); err != nil {
			// Log error but continue with other transactions
			continue
		}
		count++
	}

	return count, nil
}

// BulkRemoveTags removes all tags from multiple transactions.
// Used for bulk "uncategorize" operations.
func (s *TagStore) BulkRemoveTags(ctx context.Context, transactionIDs []uuid.UUID) error {
	if len(transactionIDs) == 0 {
		return nil
	}

	_, err := s.pool.Exec(ctx, `
		DELETE FROM transaction_tags WHERE transaction_id = ANY($1)
	`, transactionIDs)

	return err
}

// RecategorizeTransactionsByEntity recategorizes all transactions for an entity.
// It removes existing tags, adds the new tag, and updates ledger entry accounts.
// Returns the number of transactions recategorized.
func (s *TagStore) RecategorizeTransactionsByEntity(ctx context.Context, entityID, tagID uuid.UUID) (int, error) {
	// Get all transaction IDs for this entity
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM transactions WHERE entity_id = $1
	`, entityID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var txnIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		txnIDs = append(txnIDs, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(txnIDs) == 0 {
		return 0, nil
	}

	// Remove existing tags from all these transactions
	_, err = s.pool.Exec(ctx, `
		DELETE FROM transaction_tags WHERE transaction_id = ANY($1)
	`, txnIDs)
	if err != nil {
		return 0, err
	}

	// Apply the new tag to each transaction (this also updates ledger entries)
	count := 0
	for _, txnID := range txnIDs {
		if err := s.CategorizeTransaction(ctx, txnID, tagID); err != nil {
			// Log error but continue with other transactions
			continue
		}
		count++
	}

	return count, nil
}
