package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/embedding"
	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// Executor handles the execution of LLM tool calls
type Executor struct {
	ledgerID         uuid.UUID
	transactions     *models.TransactionStore
	accounts         *models.AccountStore
	tags             *models.TagStore
	rules            *models.RuleStore
	patterns         *models.RecurringPatternStore
	entities         *models.EntityStore
	relationships    *models.EntityRelationshipStore
	embeddingService *embedding.Service // Optional: for similarity search
}

// NewExecutor creates a new tool executor for a specific ledger
func NewExecutor(
	ledgerID uuid.UUID,
	transactions *models.TransactionStore,
	accounts *models.AccountStore,
	tags *models.TagStore,
	rules *models.RuleStore,
	patterns *models.RecurringPatternStore,
	entities *models.EntityStore,
	relationships *models.EntityRelationshipStore,
) *Executor {
	return &Executor{
		ledgerID:      ledgerID,
		transactions:  transactions,
		accounts:      accounts,
		tags:          tags,
		rules:         rules,
		patterns:      patterns,
		entities:      entities,
		relationships: relationships,
	}
}

// SetEmbeddingService sets the embedding service for similarity search
func (e *Executor) SetEmbeddingService(svc *embedding.Service) {
	e.embeddingService = svc
}

// Execute runs a tool call and returns the result
func (e *Executor) Execute(ctx context.Context, call ToolCall) ToolResult {
	result := ToolResult{
		ToolCallID: call.ID,
	}

	// Parse arguments
	var args map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		result.Error = fmt.Sprintf("Invalid arguments: %v", err)
		return result
	}

	// Execute the appropriate tool
	var response any
	var err error

	switch call.Function.Name {
	// Read-only tools
	case ToolGetTransaction:
		response, err = e.getTransaction(ctx, args)
	case ToolSearchTransactions:
		response, err = e.searchTransactions(ctx, args)
	case ToolGetAccounts:
		response, err = e.getAccounts(ctx, args)
	case ToolGetTags:
		response, err = e.getTags(ctx, args)
	case ToolGetSpendingSummary:
		response, err = e.getSpendingSummary(ctx, args)
	case ToolGetRecurringPatterns:
		response, err = e.getRecurringPatterns(ctx, args)
	case ToolGetEntities:
		response, err = e.getEntities(ctx, args)

	// Semantic similarity tools
	case ToolFindSimilarTransactions:
		response, err = e.findSimilarTransactions(ctx, args)
	case ToolFindSimilarEntities:
		response, err = e.findSimilarEntities(ctx, args)

	// Write tools
	case ToolUpdateTransaction:
		response, err = e.updateTransaction(ctx, args)
	case ToolCategorizeTransaction:
		response, err = e.categorizeTransaction(ctx, args)
	case ToolBulkCategorize:
		response, err = e.bulkCategorize(ctx, args)
	case ToolFindTransferCandidates:
		response, err = e.findTransferCandidates(ctx, args)
	case ToolLinkTransfer:
		response, err = e.linkTransfer(ctx, args)
	case ToolUnlinkTransfer:
		response, err = e.unlinkTransfer(ctx, args)
	case ToolCreateJournalEntry:
		response, err = e.createJournalEntry(ctx, args)
	case ToolSplitTransaction:
		response, err = e.splitTransaction(ctx, args)

	// Entity tools
	case ToolCreateEntity:
		response, err = e.createEntity(ctx, args)
	case ToolSetTransactionEntity:
		response, err = e.setTransactionEntity(ctx, args)
	case ToolCreateEntityRelationship:
		response, err = e.createEntityRelationship(ctx, args)

	// Rule tools
	case ToolCreateRule:
		response, err = e.createRule(ctx, args)
	case ToolSuggestRule:
		response, err = e.suggestRule(ctx, args)

	default:
		result.Error = fmt.Sprintf("Unknown tool: %s", call.Function.Name)
		return result
	}

	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Serialize response
	responseJSON, err := json.Marshal(response)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to serialize response: %v", err)
		return result
	}

	result.Content = string(responseJSON)
	return result
}

// =============================================================================
// Read-Only Query Tools Implementation
// =============================================================================

// TransactionResponse is the LLM-friendly response for a transaction
type TransactionResponse struct {
	ID                 string          `json:"id"`
	Date               string          `json:"date"`
	Description        string          `json:"description"`
	DisplayTitle       string          `json:"display_title,omitempty"`
	Notes              string          `json:"notes,omitempty"`
	IsTransfer         bool            `json:"is_transfer"`
	TransferType       string          `json:"transfer_type,omitempty"`
	CounterpartyName   string          `json:"counterparty_name,omitempty"`
	Entries            []EntryResponse `json:"entries,omitempty"`
	Tags               []TagResponse   `json:"tags,omitempty"`
	Entity             *EntityResponse `json:"entity,omitempty"`
	CounterpartyEntity *EntityResponse `json:"counterparty_entity,omitempty"`
	IntermediaryEntity *EntityResponse `json:"intermediary_entity,omitempty"`
}

// EntryResponse is the LLM-friendly response for an entry
type EntryResponse struct {
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
	AccountType string `json:"account_type"`
	AmountCents int64  `json:"amount_cents"`
	AmountStr   string `json:"amount"` // Human-readable "$12.34"
}

// TagResponse is the LLM-friendly response for a tag
type TagResponse struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Color    string        `json:"color"`
	ParentID string        `json:"parent_id,omitempty"`
	Children []TagResponse `json:"children,omitempty"`
}

// EntityResponse is the LLM-friendly response for an entity (person/business)
type EntityResponse struct {
	ID          string `json:"id"`
	Type        string `json:"type,omitempty"` // person, business, government, etc.
	Name        string `json:"name"`
	LogoURL     string `json:"logo_url,omitempty"`
	Website     string `json:"website,omitempty"`
	Description string `json:"description,omitempty"`
}

// AccountResponse is the LLM-friendly response for an account
type AccountResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	InstitutionName string `json:"institution_name,omitempty"`
	BalanceCents    int64  `json:"balance_cents"`
	BalanceStr      string `json:"balance"` // Human-readable "$1,234.56"
	IsActive        bool   `json:"is_active"`
}

func (e *Executor) getTransaction(ctx context.Context, args map[string]any) (*TransactionResponse, error) {
	txn, _, err := e.mustGetTransaction(ctx, args, "transaction_id")
	if err != nil {
		return nil, err
	}
	e.loadTxnDetails(ctx, txn)
	return e.transactionToResponse(txn), nil
}

func (e *Executor) searchTransactions(ctx context.Context, args map[string]any) (map[string]any, error) {
	filter := models.TransactionFilter{
		LedgerID: e.ledgerID,
		Limit:    20, // Default limit
	}

	// Parse optional filters
	if search, ok := args["search"].(string); ok {
		filter.Search = search
	}

	if dateFrom, ok := args["date_from"].(string); ok {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			filter.StartDate = &t
		}
	}

	if dateTo, ok := args["date_to"].(string); ok {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			filter.EndDate = &t
		}
	}

	if accountID, ok := args["account_id"].(string); ok {
		if id, err := uuid.Parse(accountID); err == nil {
			filter.AccountID = &id
		}
	}

	if tagID, ok := args["tag_id"].(string); ok {
		if id, err := uuid.Parse(tagID); err == nil {
			filter.TagID = &id
		}
	}

	if entityID, ok := args["entity_id"].(string); ok {
		if id, err := uuid.Parse(entityID); err == nil {
			filter.EntityID = &id
		}
	}

	if isUncategorized, ok := args["is_uncategorized"].(bool); ok && isUncategorized {
		filter.Uncategorized = true
	}

	if isTransfer, ok := args["is_transfer"].(bool); ok {
		filter.IsTransfer = &isTransfer
	}

	if limit, ok := args["limit"].(float64); ok {
		filter.Limit = int(limit)
		if filter.Limit > 100 {
			filter.Limit = 100
		}
	}

	transactions, total, err := e.transactions.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to search transactions: %v", err)
	}

	// Load entries and tags for each
	results := make([]*TransactionResponse, len(transactions))
	for i, txn := range transactions {
		e.loadTxnDetails(ctx, txn)
		results[i] = e.transactionToResponse(txn)
	}

	return map[string]any{
		"transactions": results,
		"total":        total,
		"showing":      len(results),
	}, nil
}

func (e *Executor) getAccounts(ctx context.Context, args map[string]any) (map[string]any, error) {
	accounts, err := e.accounts.GetWithBalances(ctx, e.ledgerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %v", err)
	}

	// Filter by type if specified
	filterType, hasFilter := args["type"].(string)

	results := make([]*AccountResponse, 0, len(accounts))
	for _, acc := range accounts {
		if hasFilter && string(acc.Type) != filterType {
			continue
		}
		results = append(results, e.accountToResponse(acc))
	}

	return map[string]any{
		"accounts": results,
		"total":    len(results),
	}, nil
}

func (e *Executor) getTags(ctx context.Context, args map[string]any) (map[string]any, error) {
	tags, err := e.tags.GetHierarchy(ctx, e.ledgerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %v", err)
	}

	results := make([]TagResponse, len(tags))
	for i, tag := range tags {
		results[i] = e.tagToResponse(tag)
	}

	return map[string]any{
		"tags": results,
	}, nil
}

func (e *Executor) getSpendingSummary(ctx context.Context, args map[string]any) (map[string]any, error) {
	startDateStr, ok := args["start_date"].(string)
	if !ok {
		return nil, fmt.Errorf("start_date is required")
	}
	endDateStr, ok := args["end_date"].(string)
	if !ok {
		return nil, fmt.Errorf("end_date is required")
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date: %v", err)
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date: %v", err)
	}

	groupBy := "tag"
	if gb, ok := args["group_by"].(string); ok {
		groupBy = gb
	}

	// Use fast SQL aggregation - single query instead of N+1
	groups, totalSpending, err := e.transactions.GetSpendingSummary(ctx, e.ledgerID, startDate, endDate, groupBy)
	if err != nil {
		return nil, fmt.Errorf("failed to get spending summary: %v", err)
	}

	// Format response
	type SpendingGroup struct {
		Name       string `json:"name"`
		TotalCents int64  `json:"total_cents"`
		TotalStr   string `json:"total"`
		Count      int    `json:"transaction_count"`
	}

	results := make([]*SpendingGroup, 0, len(groups))
	for _, g := range groups {
		results = append(results, &SpendingGroup{
			Name:       g.GroupName,
			TotalCents: g.TotalCents,
			TotalStr:   models.FormatCents(g.TotalCents),
			Count:      g.TransactionCount,
		})
	}

	return map[string]any{
		"period": map[string]string{
			"start": startDateStr,
			"end":   endDateStr,
		},
		"group_by":       groupBy,
		"groups":         results,
		"total_spending": models.FormatCents(totalSpending),
	}, nil
}

func (e *Executor) getRecurringPatterns(ctx context.Context, args map[string]any) (map[string]any, error) {
	if e.patterns == nil {
		return map[string]any{
			"patterns": []any{},
			"message":  "Recurring pattern detection not configured",
		}, nil
	}

	patterns, err := e.patterns.GetByLedgerID(ctx, e.ledgerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recurring patterns: %v", err)
	}

	type PatternResponse struct {
		ID            string `json:"id"`
		EntityName    string `json:"entity_name,omitempty"`
		Frequency     string `json:"frequency"`
		AverageAmount string `json:"average_amount"`
		LastSeen      string `json:"last_seen,omitempty"`
	}

	results := make([]*PatternResponse, len(patterns))
	for i, p := range patterns {
		var entityName string
		if p.Entity != nil {
			entityName = p.Entity.Name
		}

		results[i] = &PatternResponse{
			ID:            p.ID.String(),
			EntityName:    entityName,
			Frequency:     p.Frequency,
			AverageAmount: models.FormatCents(p.AvgAmountCents),
		}
		if p.LastSeenAt != nil {
			results[i].LastSeen = p.LastSeenAt.Format("2006-01-02")
		}
	}

	return map[string]any{
		"patterns": results,
		"total":    len(results),
	}, nil
}

// =============================================================================
// Transaction Modification Tools Implementation
// =============================================================================

func (e *Executor) updateTransaction(ctx context.Context, args map[string]any) (*TransactionResponse, error) {
	txn, _, err := e.mustGetTransaction(ctx, args, "transaction_id")
	if err != nil {
		return nil, err
	}

	// Apply updates
	if dateStr, ok := args["date"].(string); ok {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			txn.Date = t
		}
	}
	if description, ok := args["description"].(string); ok {
		txn.Description = description
	}
	if displayTitle, ok := args["display_title"].(string); ok {
		txn.DisplayTitle = displayTitle
	}
	if notes, ok := args["notes"].(string); ok {
		txn.Notes = notes
	}

	// Save updates
	if err := e.transactions.Update(ctx, txn); err != nil {
		return nil, fmt.Errorf("failed to update transaction: %v", err)
	}

	return e.loadAndRespond(ctx, txn), nil
}

func (e *Executor) categorizeTransaction(ctx context.Context, args map[string]any) (*TransactionResponse, error) {
	txn, txnID, err := e.mustGetTransaction(ctx, args, "transaction_id")
	if err != nil {
		return nil, err
	}
	_, tagID, err := e.mustGetTag(ctx, args, "tag_id")
	if err != nil {
		return nil, err
	}

	if err := e.tags.CategorizeTransaction(ctx, txnID, tagID); err != nil {
		return nil, fmt.Errorf("failed to categorize transaction: %v", err)
	}

	return e.loadAndRespond(ctx, txn), nil
}

func (e *Executor) bulkCategorize(ctx context.Context, args map[string]any) (map[string]any, error) {
	txnIDsRaw, ok := args["transaction_ids"].([]any)
	if !ok {
		return nil, fmt.Errorf("transaction_ids is required")
	}
	tag, tagID, err := e.mustGetTag(ctx, args, "tag_id")
	if err != nil {
		return nil, err
	}

	// Parse and verify transaction IDs
	var txnIDs []uuid.UUID
	for _, idRaw := range txnIDsRaw {
		idStr, ok := idRaw.(string)
		if !ok {
			continue
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		// Verify ownership
		txn, err := e.transactions.GetByID(ctx, id)
		if err != nil || txn.LedgerID != e.ledgerID {
			continue
		}
		txnIDs = append(txnIDs, id)
	}

	if len(txnIDs) == 0 {
		return nil, fmt.Errorf("no valid transaction_ids provided")
	}

	// Apply categorization to all
	count, err := e.tags.BulkCategorizeTransactions(ctx, txnIDs, tagID)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk categorize: %v", err)
	}

	return map[string]any{
		"categorized_count": count,
		"requested_count":   len(txnIDs),
		"tag_name":          tag.Name,
	}, nil
}

// =============================================================================
// Transfer Matching Tools Implementation
// =============================================================================

func (e *Executor) findTransferCandidates(ctx context.Context, args map[string]any) (map[string]any, error) {
	txn, _, err := e.mustGetTransaction(ctx, args, "transaction_id")
	if err != nil {
		return nil, err
	}

	// Load entries to get the amount and account
	if err := e.transactions.LoadEntries(ctx, txn); err != nil {
		return nil, fmt.Errorf("failed to load entries: %v", err)
	}

	// Find the asset/liability entry
	var amountCents int64
	var accountID uuid.UUID
	for _, entry := range txn.Entries {
		if entry.AccountType == models.AccountTypeAsset || entry.AccountType == models.AccountTypeLiability {
			amountCents = entry.AmountCents
			accountID = entry.AccountID
			break
		}
	}

	// Find candidates
	candidates, err := e.transactions.FindTransferCandidates(ctx, txn, amountCents, accountID, 7)
	if err != nil {
		return nil, fmt.Errorf("failed to find candidates: %v", err)
	}

	type CandidateResponse struct {
		TransactionID string `json:"transaction_id"`
		Date          string `json:"date"`
		Description   string `json:"description"`
		AccountName   string `json:"account_name"`
		Amount        string `json:"amount"`
	}

	results := make([]*CandidateResponse, len(candidates))
	for i, c := range candidates {
		results[i] = &CandidateResponse{
			TransactionID: c.Transaction.ID.String(),
			Date:          c.Transaction.Date.Format("2006-01-02"),
			Description:   c.Transaction.Description,
			AccountName:   c.AccountName,
			Amount:        models.FormatCents(c.AmountCents),
		}
	}

	return map[string]any{
		"original_transaction": e.transactionToResponse(txn),
		"candidates":           results,
		"candidate_count":      len(results),
	}, nil
}

func (e *Executor) linkTransfer(ctx context.Context, args map[string]any) (map[string]any, error) {
	txn1, txn1ID, err := e.mustGetTransaction(ctx, args, "transaction_id_1")
	if err != nil {
		return nil, err
	}
	txn2, txn2ID, err := e.mustGetTransaction(ctx, args, "transaction_id_2")
	if err != nil {
		return nil, err
	}

	if txn1ID == txn2ID {
		return nil, fmt.Errorf("cannot link a transaction to itself")
	}

	// Check if either is already a transfer
	if txn1.IsTransfer || txn2.IsTransfer {
		return nil, fmt.Errorf("one or both transactions are already marked as transfers")
	}

	// Link them
	if err := e.transactions.SetTransferPair(ctx, txn1ID, txn2ID); err != nil {
		return nil, fmt.Errorf("failed to link transfer: %v", err)
	}

	// Reload both (best-effort; keep stale non-nil value if DB hiccups)
	if reloaded, err := e.transactions.GetByID(ctx, txn1ID); err == nil {
		txn1 = reloaded
	}
	if reloaded, err := e.transactions.GetByID(ctx, txn2ID); err == nil {
		txn2 = reloaded
	}
	if err := e.transactions.LoadEntries(ctx, txn1); err != nil {
		slog.WarnContext(ctx, "failed to load entries", "txn_id", txn1.ID, "err", err)
	}
	if err := e.transactions.LoadEntries(ctx, txn2); err != nil {
		slog.WarnContext(ctx, "failed to load entries", "txn_id", txn2.ID, "err", err)
	}

	return map[string]any{
		"linked":       true,
		"transaction1": e.transactionToResponse(txn1),
		"transaction2": e.transactionToResponse(txn2),
	}, nil
}

func (e *Executor) unlinkTransfer(ctx context.Context, args map[string]any) (map[string]any, error) {
	txn, txnID, err := e.mustGetTransaction(ctx, args, "transaction_id")
	if err != nil {
		return nil, err
	}

	if !txn.IsTransfer {
		return nil, fmt.Errorf("transaction is not a transfer")
	}

	// Unlink
	if err := e.transactions.UnlinkTransferPair(ctx, txnID); err != nil {
		return nil, fmt.Errorf("failed to unlink transfer: %v", err)
	}

	// Reload (best-effort; keep stale non-nil value if DB hiccups)
	if reloaded, err := e.transactions.GetByID(ctx, txnID); err == nil {
		txn = reloaded
	}
	if err := e.transactions.LoadEntries(ctx, txn); err != nil {
		slog.WarnContext(ctx, "failed to load entries", "txn_id", txn.ID, "err", err)
	}

	return map[string]any{
		"unlinked":    true,
		"transaction": e.transactionToResponse(txn),
	}, nil
}

// =============================================================================
// Journal Entry Tools Implementation
// =============================================================================

func (e *Executor) createJournalEntry(ctx context.Context, args map[string]any) (*TransactionResponse, error) {
	dateStr, ok := args["date"].(string)
	if !ok {
		return nil, fmt.Errorf("date is required")
	}
	description, ok := args["description"].(string)
	if !ok {
		return nil, fmt.Errorf("description is required")
	}
	entriesRaw, ok := args["entries"].([]any)
	if !ok || len(entriesRaw) < 2 {
		return nil, fmt.Errorf("at least 2 entries are required")
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date: %v", err)
	}

	// Parse entries
	var entries []*models.Entry
	var sum int64
	for _, entryRaw := range entriesRaw {
		entryMap, ok := entryRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid entry format")
		}

		accountIDStr, ok := entryMap["account_id"].(string)
		if !ok {
			return nil, fmt.Errorf("entry missing account_id")
		}
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid account_id: %v", err)
		}

		// Verify account ownership
		account, err := e.accounts.GetByID(ctx, accountID)
		if err != nil || account.LedgerID != e.ledgerID {
			return nil, fmt.Errorf("account not found: %s", accountIDStr)
		}

		amountCentsRaw, ok := entryMap["amount_cents"]
		if !ok {
			return nil, fmt.Errorf("entry missing amount_cents")
		}
		amountCents, err := parseAmountCents(amountCentsRaw)
		if err != nil {
			return nil, err
		}

		sum += amountCents
		entries = append(entries, &models.Entry{
			AccountID:   accountID,
			AmountCents: amountCents,
			Currency:    "USD",
		})
	}

	// Verify entries sum to zero
	if sum != 0 {
		return nil, fmt.Errorf("entries must sum to zero (got %d cents)", sum)
	}

	// Create transaction
	txn := &models.Transaction{
		LedgerID:    e.ledgerID,
		Date:        date,
		Description: description,
	}

	if notes, ok := args["notes"].(string); ok {
		txn.Notes = notes
	}

	if err := e.transactions.CreateWithEntries(ctx, txn, entries); err != nil {
		return nil, fmt.Errorf("failed to create journal entry: %v", err)
	}

	// Apply tags if provided
	if tagIDsRaw, ok := args["tag_ids"].([]any); ok {
		for _, tagIDRaw := range tagIDsRaw {
			if tagIDStr, ok := tagIDRaw.(string); ok {
				if tagID, err := uuid.Parse(tagIDStr); err == nil {
					// Verify tag ownership
					tag, err := e.tags.GetByID(ctx, tagID)
					if err == nil && tag.LedgerID == e.ledgerID {
						if err := e.tags.AddTagToTransaction(ctx, txn.ID, tagID); err != nil {
							slog.WarnContext(ctx, "failed to add tag to new journal entry", "txn", txn.ID, "tag", tagID, "error", err)
						}
					}
				}
			}
		}
	}

	return e.loadAndRespond(ctx, txn), nil
}

func (e *Executor) splitTransaction(ctx context.Context, args map[string]any) (*TransactionResponse, error) {
	txn, txnID, err := e.mustGetTransaction(ctx, args, "transaction_id")
	if err != nil {
		return nil, err
	}
	splitsRaw, ok := args["splits"].([]any)
	if !ok || len(splitsRaw) < 2 {
		return nil, fmt.Errorf("at least 2 splits are required")
	}

	if err := e.transactions.LoadEntries(ctx, txn); err != nil {
		slog.WarnContext(ctx, "failed to load entries", "txn_id", txn.ID, "err", err)
	}

	// Find the original amount from the asset/liability entry
	var originalAmountCents int64
	var assetEntry *models.Entry
	for _, entry := range txn.Entries {
		if entry.AccountType == models.AccountTypeAsset || entry.AccountType == models.AccountTypeLiability {
			originalAmountCents = entry.AmountCents
			assetEntry = entry
			break
		}
	}

	if assetEntry == nil {
		return nil, fmt.Errorf("cannot split this transaction - no asset/liability entry found")
	}

	// Parse splits and verify they sum to the original amount
	type Split struct {
		TagID       uuid.UUID
		AmountCents int64
		Notes       string
	}
	var splits []Split
	var splitSum int64

	for _, splitRaw := range splitsRaw {
		splitMap, ok := splitRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid split format")
		}

		_, tagID, err := e.mustGetTag(ctx, splitMap, "tag_id")
		if err != nil {
			return nil, err
		}

		amountCentsRaw, ok := splitMap["amount_cents"]
		if !ok {
			return nil, fmt.Errorf("split missing amount_cents")
		}
		amountCents, err := parseAmountCents(amountCentsRaw)
		if err != nil {
			return nil, err
		}

		splitSum += amountCents

		split := Split{TagID: tagID, AmountCents: amountCents}
		if notes, ok := splitMap["notes"].(string); ok {
			split.Notes = notes
		}
		splits = append(splits, split)
	}

	// For expenses, original amount is negative, splits should be positive (what we spent)
	// Verify splits match the absolute value
	absOriginal := originalAmountCents
	if absOriginal < 0 {
		absOriginal = -absOriginal
	}
	if splitSum != absOriginal {
		return nil, fmt.Errorf("splits must sum to %s (got %s)", models.FormatCents(absOriginal), models.FormatCents(splitSum))
	}

	// Note: Full split implementation would require updating the transaction's entries
	// with multiple expense accounts. For now, we'll just add the tags. A complete
	// implementation would need UpdateWithEntries to handle multiple expense accounts
	// per transaction.

	// For the MVP, we'll categorize with the first tag and add all tags
	if len(splits) > 0 {
		if err := e.tags.CategorizeTransaction(ctx, txnID, splits[0].TagID); err != nil {
			slog.WarnContext(ctx, "failed to categorize split transaction", "txn", txnID, "tag", splits[0].TagID, "error", err)
		}
		for i := 1; i < len(splits); i++ {
			if err := e.tags.AddTagToTransaction(ctx, txnID, splits[i].TagID); err != nil {
				slog.WarnContext(ctx, "failed to add tag to split transaction", "txn", txnID, "tag", splits[i].TagID, "error", err)
			}
		}
	}

	return e.loadAndRespond(ctx, txn), nil
}

// =============================================================================
// Rule Tools Implementation
// =============================================================================

func (e *Executor) createRule(ctx context.Context, args map[string]any) (map[string]any, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name is required")
	}
	matchPattern, ok := args["match_pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("match_pattern is required")
	}
	tag, tagID, err := e.mustGetTag(ctx, args, "tag_id")
	if err != nil {
		return nil, err
	}

	rule := &models.CategorizationRule{
		LedgerID:     e.ledgerID,
		Name:         name,
		MatchPattern: matchPattern,
		TagID:        tagID,
		IsActive:     true,
	}

	if isRegex, ok := args["is_regex"].(bool); ok {
		rule.IsRegex = isRegex
	}
	if priority, ok := args["priority"].(float64); ok {
		rule.Priority = int(priority)
	}

	if err := e.rules.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to create rule: %v", err)
	}

	return map[string]any{
		"id":            rule.ID.String(),
		"name":          rule.Name,
		"match_pattern": rule.MatchPattern,
		"is_regex":      rule.IsRegex,
		"tag_name":      tag.Name,
		"priority":      rule.Priority,
	}, nil
}

func (e *Executor) suggestRule(ctx context.Context, args map[string]any) (map[string]any, error) {
	txn, _, err := e.mustGetTransaction(ctx, args, "transaction_id")
	if err != nil {
		return nil, err
	}
	tag, tagID, err := e.mustGetTag(ctx, args, "tag_id")
	if err != nil {
		return nil, err
	}

	// Generate a suggested pattern from the description
	// This is a simple heuristic - extract the first word or business-like pattern
	description := txn.Description
	suggestedPattern := description

	// If there's a display title, use that as the pattern
	if txn.DisplayTitle != "" {
		suggestedPattern = txn.DisplayTitle
	}

	// Generate a rule name
	ruleName := fmt.Sprintf("Auto-tag %s as %s", suggestedPattern, tag.Name)

	return map[string]any{
		"suggested_rule": map[string]any{
			"name":          ruleName,
			"match_pattern": suggestedPattern,
			"is_regex":      false,
			"tag_id":        tagID.String(),
			"tag_name":      tag.Name,
		},
		"transaction_description": description,
		"message":                 "Review the suggested rule. You can create it with create_rule if it looks correct.",
	}, nil
}

// =============================================================================
// Entity Tools Implementation
// =============================================================================

func (e *Executor) getEntities(ctx context.Context, args map[string]any) (map[string]any, error) {
	var entityType *models.EntityType
	var subtype *string
	search := ""
	limit := 20

	if typeStr, ok := args["type"].(string); ok {
		et := models.EntityType(typeStr)
		entityType = &et
	}

	if s, ok := args["search"].(string); ok {
		search = s
	}

	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
		if limit > 100 {
			limit = 100
		}
	}

	entities, total, err := e.entities.List(ctx, entityType, subtype, search, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list entities: %v", err)
	}

	results := make([]*EntityResponse, len(entities))
	for i, ent := range entities {
		results[i] = e.entityToResponse(ent)
	}

	return map[string]any{
		"entities": results,
		"total":    total,
		"showing":  len(results),
	}, nil
}

func (e *Executor) createEntity(ctx context.Context, args map[string]any) (*EntityResponse, error) {
	typeStr, ok := args["type"].(string)
	if !ok {
		return nil, fmt.Errorf("type is required")
	}
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name is required")
	}

	entity := &models.Entity{
		Type: models.EntityType(typeStr),
		Name: name,
	}

	if subtype, ok := args["subtype"].(string); ok {
		entity.Subtype = subtype
	}
	if website, ok := args["website"].(string); ok {
		entity.Website = website
	}
	if description, ok := args["description"].(string); ok {
		entity.Description = description
	}

	if err := e.entities.Create(ctx, entity); err != nil {
		return nil, fmt.Errorf("failed to create entity: %v", err)
	}

	return e.entityToResponse(entity), nil
}

func (e *Executor) setTransactionEntity(ctx context.Context, args map[string]any) (*TransactionResponse, error) {
	txn, _, err := e.mustGetTransaction(ctx, args, "transaction_id")
	if err != nil {
		return nil, err
	}

	// Parse optional entity IDs
	if entityIDStr, ok := args["entity_id"].(string); ok {
		if id, err := uuid.Parse(entityIDStr); err == nil {
			// Verify entity exists
			if _, err := e.entities.GetByID(ctx, id); err != nil {
				return nil, fmt.Errorf("entity not found: %s", entityIDStr)
			}
			txn.EntityID = &id
		}
	}

	if counterpartyIDStr, ok := args["counterparty_entity_id"].(string); ok {
		if id, err := uuid.Parse(counterpartyIDStr); err == nil {
			if _, err := e.entities.GetByID(ctx, id); err != nil {
				return nil, fmt.Errorf("counterparty entity not found: %s", counterpartyIDStr)
			}
			txn.CounterpartyEntityID = &id
		}
	}

	if intermediaryIDStr, ok := args["intermediary_entity_id"].(string); ok {
		if id, err := uuid.Parse(intermediaryIDStr); err == nil {
			if _, err := e.entities.GetByID(ctx, id); err != nil {
				return nil, fmt.Errorf("intermediary entity not found: %s", intermediaryIDStr)
			}
			txn.IntermediaryEntityID = &id
		}
	}

	// Update the transaction with entity references
	if err := e.transactions.UpdateEnrichment(ctx, txn); err != nil {
		return nil, fmt.Errorf("failed to update transaction: %v", err)
	}

	return e.loadAndRespond(ctx, txn), nil
}

func (e *Executor) createEntityRelationship(ctx context.Context, args map[string]any) (map[string]any, error) {
	entityAIDStr, ok := args["entity_a_id"].(string)
	if !ok {
		return nil, fmt.Errorf("entity_a_id is required")
	}
	entityBIDStr, ok := args["entity_b_id"].(string)
	if !ok {
		return nil, fmt.Errorf("entity_b_id is required")
	}
	relType, ok := args["relationship_type"].(string)
	if !ok {
		return nil, fmt.Errorf("relationship_type is required")
	}

	entityAID, err := uuid.Parse(entityAIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid entity_a_id: %v", err)
	}
	entityBID, err := uuid.Parse(entityBIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid entity_b_id: %v", err)
	}

	// Verify both entities exist
	entityA, err := e.entities.GetByID(ctx, entityAID)
	if err != nil {
		return nil, fmt.Errorf("entity_a not found")
	}
	entityB, err := e.entities.GetByID(ctx, entityBID)
	if err != nil {
		return nil, fmt.Errorf("entity_b not found")
	}

	relationship := &models.EntityRelationship{
		LedgerID:         e.ledgerID,
		EntityAID:        entityAID,
		EntityBID:        entityBID,
		RelationshipType: relType,
	}

	if err := e.relationships.Create(ctx, relationship); err != nil {
		return nil, fmt.Errorf("failed to create relationship: %v", err)
	}

	return map[string]any{
		"id":                relationship.ID.String(),
		"entity_a":          e.entityToResponse(entityA),
		"entity_b":          e.entityToResponse(entityB),
		"relationship_type": relType,
	}, nil
}

// =============================================================================
// Helper Functions
// =============================================================================

func (e *Executor) transactionToResponse(txn *models.Transaction) *TransactionResponse {
	resp := &TransactionResponse{
		ID:               txn.ID.String(),
		Date:             txn.Date.Format("2006-01-02"),
		Description:      txn.Description,
		DisplayTitle:     txn.DisplayTitle,
		Notes:            txn.Notes,
		IsTransfer:       txn.IsTransfer,
		TransferType:     txn.TransferType,
		CounterpartyName: txn.CounterpartyName,
	}

	if txn.Entries != nil {
		resp.Entries = make([]EntryResponse, len(txn.Entries))
		for i, entry := range txn.Entries {
			resp.Entries[i] = EntryResponse{
				AccountID:   entry.AccountID.String(),
				AccountName: entry.AccountName,
				AccountType: string(entry.AccountType),
				AmountCents: entry.AmountCents,
				AmountStr:   models.FormatCents(entry.AmountCents),
			}
		}
	}

	if txn.Tags != nil {
		resp.Tags = make([]TagResponse, len(txn.Tags))
		for i, tag := range txn.Tags {
			resp.Tags[i] = TagResponse{
				ID:    tag.ID.String(),
				Name:  tag.Name,
				Color: tag.Color,
			}
			if tag.ParentID != nil {
				resp.Tags[i].ParentID = tag.ParentID.String()
			}
		}
	}

	// Entity references (once Entity model is fully implemented)
	if txn.Entity != nil {
		resp.Entity = e.entityToResponse(txn.Entity)
	}
	if txn.CounterpartyEntity != nil {
		resp.CounterpartyEntity = e.entityToResponse(txn.CounterpartyEntity)
	}
	if txn.IntermediaryEntity != nil {
		resp.IntermediaryEntity = e.entityToResponse(txn.IntermediaryEntity)
	}

	return resp
}

func (e *Executor) entityToResponse(entity *models.Entity) *EntityResponse {
	if entity == nil {
		return nil
	}
	return &EntityResponse{
		ID:          entity.ID.String(),
		Type:        string(entity.Type),
		Name:        entity.Name,
		LogoURL:     entity.LogoURL,
		Website:     entity.Website,
		Description: entity.Description,
	}
}

func (e *Executor) accountToResponse(acc *models.Account) *AccountResponse {
	return &AccountResponse{
		ID:              acc.ID.String(),
		Name:            acc.Name,
		Type:            string(acc.Type),
		InstitutionName: acc.InstitutionName,
		BalanceCents:    acc.Balance,
		BalanceStr:      models.FormatCents(acc.Balance),
		IsActive:        acc.IsActive,
	}
}

func (e *Executor) tagToResponse(tag *models.Tag) TagResponse {
	resp := TagResponse{
		ID:    tag.ID.String(),
		Name:  tag.Name,
		Color: tag.Color,
	}
	if tag.ParentID != nil {
		resp.ParentID = tag.ParentID.String()
	}
	if tag.Children != nil {
		resp.Children = make([]TagResponse, len(tag.Children))
		for i, child := range tag.Children {
			resp.Children[i] = e.tagToResponse(child)
		}
	}
	return resp
}

func (e *Executor) mustGetTransaction(ctx context.Context, args map[string]any, key string) (*models.Transaction, uuid.UUID, error) {
	idStr, ok := args[key].(string)
	if !ok {
		return nil, uuid.Nil, fmt.Errorf("%s is required", key)
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("invalid %s: %v", key, err)
	}
	txn, err := e.transactions.GetByID(ctx, id)
	if err != nil || txn.LedgerID != e.ledgerID {
		return nil, uuid.Nil, fmt.Errorf("%s not found", strings.TrimSuffix(key, "_id"))
	}
	return txn, id, nil
}

func (e *Executor) mustGetTag(ctx context.Context, args map[string]any, key string) (*models.Tag, uuid.UUID, error) {
	idStr, ok := args[key].(string)
	if !ok {
		return nil, uuid.Nil, fmt.Errorf("%s is required", key)
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("invalid %s: %v", key, err)
	}
	tag, err := e.tags.GetByID(ctx, id)
	if err != nil || tag.LedgerID != e.ledgerID {
		return nil, uuid.Nil, fmt.Errorf("%s not found", strings.TrimSuffix(key, "_id"))
	}
	return tag, id, nil
}

func (e *Executor) loadAndRespond(ctx context.Context, txn *models.Transaction) *TransactionResponse {
	if reloaded, err := e.transactions.GetByID(ctx, txn.ID); err == nil {
		txn = reloaded
	}
	e.loadTxnDetails(ctx, txn)
	return e.transactionToResponse(txn)
}

// loadTxnDetails loads entries and tags for txn, logging any DB errors.
// Failures are non-fatal: the caller receives a partial transaction.
func (e *Executor) loadTxnDetails(ctx context.Context, txn *models.Transaction) {
	if err := e.transactions.LoadEntries(ctx, txn); err != nil {
		slog.WarnContext(ctx, "failed to load entries", "txn_id", txn.ID, "err", err)
	}
	if err := e.transactions.LoadTags(ctx, txn); err != nil {
		slog.WarnContext(ctx, "failed to load tags", "txn_id", txn.ID, "err", err)
	}
}

// =============================================================================
// Semantic Similarity Tools
// =============================================================================

func parseLimitAndMinSimilarity(args map[string]any) (int, float32) {
	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
		if limit > 50 {
			limit = 50
		}
	}
	minSimilarity := float32(0.7)
	if ms, ok := args["min_similarity"].(float64); ok && ms > 0 && ms <= 1 {
		minSimilarity = float32(ms)
	}
	return limit, minSimilarity
}

type similarTransactionResult struct {
	TransactionID string  `json:"transaction_id"`
	Date          string  `json:"date"`
	Description   string  `json:"description"`
	DisplayTitle  string  `json:"display_title,omitempty"`
	AmountStr     string  `json:"amount"`
	EntityName    string  `json:"entity_name,omitempty"`
	PatternType   string  `json:"pattern_type,omitempty"`
	Similarity    float32 `json:"similarity"`     // 0-1 score
	SimilarityPct string  `json:"similarity_pct"` // "85%"
}

// findSimilarTransactions finds transactions similar to a reference or query
func (e *Executor) findSimilarTransactions(ctx context.Context, args map[string]any) (any, error) {
	if e.embeddingService == nil || !e.embeddingService.IsConfigured() {
		return nil, fmt.Errorf("similarity search not available - embedding service not configured")
	}

	limit, minSimilarity := parseLimitAndMinSimilarity(args)

	var embedding []float32
	var err error

	// Option 1: Find similar to a specific transaction
	if txnIDStr, ok := args["transaction_id"].(string); ok && txnIDStr != "" {
		txnID, err := uuid.Parse(txnIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid transaction_id: %v", err)
		}

		// Get the transaction's embedding
		txn, err := e.transactions.GetByID(ctx, txnID)
		if err != nil {
			return nil, fmt.Errorf("transaction not found: %v", err)
		}

		if len(txn.Embedding) == 0 {
			// Generate embedding on-the-fly
			text := txn.DisplayTitle
			if text == "" {
				text = txn.Description
			}
			embedding, err = e.embeddingService.EmbedText(ctx, text)
			if err != nil {
				return nil, fmt.Errorf("failed to generate embedding: %v", err)
			}
		} else {
			embedding = txn.Embedding
		}
	}

	// Option 2: Find similar to a natural language query
	if query, ok := args["query"].(string); ok && query != "" && embedding == nil {
		embedding, err = e.embeddingService.EmbedText(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("failed to embed query: %v", err)
		}
	}

	if embedding == nil {
		return nil, fmt.Errorf("must provide either transaction_id or query")
	}

	// Find similar transactions
	results, err := e.transactions.FindSimilarTransactions(ctx, embedding, e.ledgerID, limit+1, minSimilarity)
	if err != nil {
		return nil, fmt.Errorf("similarity search failed: %v", err)
	}

	// Convert to response format
	var response []similarTransactionResult
	for _, r := range results {
		// Skip the reference transaction if it's in results
		if txnIDStr, ok := args["transaction_id"].(string); ok && r.Transaction.ID.String() == txnIDStr {
			continue
		}

		result := similarTransactionResult{
			TransactionID: r.Transaction.ID.String(),
			Date:          r.Transaction.Date.Format("2006-01-02"),
			Description:   r.Transaction.Description,
			DisplayTitle:  r.Transaction.DisplayTitle,
			PatternType:   r.Transaction.PatternType,
			Similarity:    r.Similarity,
			SimilarityPct: fmt.Sprintf("%.0f%%", r.Similarity*100),
		}

		// Load entity name if available
		if r.Transaction.EntityID != nil {
			if entity, err := e.entities.GetByID(ctx, *r.Transaction.EntityID); err == nil {
				result.EntityName = entity.Name
			}
		}

		// Get amount from entries
		if err := e.transactions.LoadEntries(ctx, r.Transaction); err == nil {
			for _, entry := range r.Transaction.Entries {
				if entry.AccountType == models.AccountTypeAsset || entry.AccountType == models.AccountTypeLiability {
					result.AmountStr = models.FormatCents(entry.AmountCents)
					break
				}
			}
		}

		response = append(response, result)
		if len(response) >= limit {
			break
		}
	}

	return map[string]any{
		"similar_transactions": response,
		"count":                len(response),
	}, nil
}

type similarEntityResult struct {
	EntityID      string  `json:"entity_id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Subtype       string  `json:"subtype,omitempty"`
	Description   string  `json:"description,omitempty"`
	LogoURL       string  `json:"logo_url,omitempty"`
	Similarity    float32 `json:"similarity"`
	SimilarityPct string  `json:"similarity_pct"`
}

// findSimilarEntities finds entities similar to a reference or query
func (e *Executor) findSimilarEntities(ctx context.Context, args map[string]any) (any, error) {
	if e.embeddingService == nil || !e.embeddingService.IsConfigured() {
		return nil, fmt.Errorf("similarity search not available - embedding service not configured")
	}

	limit, minSimilarity := parseLimitAndMinSimilarity(args)

	var embedding []float32
	var err error
	var referenceEntityID *uuid.UUID

	// Option 1: Find similar to a specific entity by ID
	if entityIDStr, ok := args["entity_id"].(string); ok && entityIDStr != "" {
		entityID, err := uuid.Parse(entityIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid entity_id: %v", err)
		}
		referenceEntityID = &entityID

		entity, err := e.entities.GetByID(ctx, entityID)
		if err != nil {
			return nil, fmt.Errorf("entity not found: %v", err)
		}

		if len(entity.Embedding) == 0 {
			// Generate embedding on-the-fly
			text := entity.Name
			if entity.Description != "" {
				text += ". " + entity.Description
			}
			embedding, err = e.embeddingService.EmbedText(ctx, text)
			if err != nil {
				return nil, fmt.Errorf("failed to generate embedding: %v", err)
			}
		} else {
			embedding = entity.Embedding
		}
	}

	// Option 2: Find similar to an entity by name
	if entityName, ok := args["entity_name"].(string); ok && entityName != "" && embedding == nil {
		entity, err := e.entities.GetByName(ctx, entityName)
		if err != nil {
			// Entity not found, use the name as a query
			embedding, err = e.embeddingService.EmbedText(ctx, entityName)
			if err != nil {
				return nil, fmt.Errorf("failed to embed entity name: %v", err)
			}
		} else {
			referenceEntityID = &entity.ID
			if len(entity.Embedding) > 0 {
				embedding = entity.Embedding
			} else {
				embedding, err = e.embeddingService.EmbedText(ctx, entityName)
				if err != nil {
					return nil, fmt.Errorf("failed to embed entity name: %v", err)
				}
			}
		}
	}

	// Option 3: Find similar to a natural language query
	if query, ok := args["query"].(string); ok && query != "" && embedding == nil {
		embedding, err = e.embeddingService.EmbedText(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("failed to embed query: %v", err)
		}
	}

	if embedding == nil {
		return nil, fmt.Errorf("must provide entity_id, entity_name, or query")
	}

	// Find similar entities
	results, err := e.entities.FindSimilarEntities(ctx, embedding, limit+1, minSimilarity)
	if err != nil {
		return nil, fmt.Errorf("similarity search failed: %v", err)
	}

	// Convert to response format
	var response []similarEntityResult
	for _, r := range results {
		// Skip the reference entity if it's in results
		if referenceEntityID != nil && r.Entity.ID == *referenceEntityID {
			continue
		}

		result := similarEntityResult{
			EntityID:      r.Entity.ID.String(),
			Name:          r.Entity.Name,
			Type:          string(r.Entity.Type),
			Subtype:       r.Entity.Subtype,
			Description:   r.Entity.Description,
			LogoURL:       r.Entity.LogoURL,
			Similarity:    r.Similarity,
			SimilarityPct: fmt.Sprintf("%.0f%%", r.Similarity*100),
		}

		response = append(response, result)
		if len(response) >= limit {
			break
		}
	}

	return map[string]any{
		"similar_entities": response,
		"count":            len(response),
	}, nil
}

// parseAmountCents converts an any value (float64, int64, or int) to int64 cents.
// LLM tool calls return JSON numbers as float64; this handles all three numeric types.
func parseAmountCents(raw any) (int64, error) {
	switch v := raw.(type) {
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("invalid amount_cents type")
	}
}
