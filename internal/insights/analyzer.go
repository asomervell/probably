package insights

import (
	"context"
	"log/slog"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/insights/providers"
	"github.com/asomervell/probably/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Analyzer generates insights for individual transactions
type Analyzer struct {
	aggregator *Aggregator
	chain      *providers.ProviderChain
	insights   *models.InsightStore
}

// NewAnalyzer creates a new transaction analyzer
func NewAnalyzer(cfg *config.Config, pool *pgxpool.Pool) *Analyzer {
	return &Analyzer{
		aggregator: NewAggregator(pool),
		chain:      buildDefaultProviderChain(cfg),
		insights:   models.NewInsightStore(pool),
	}
}

// AnalyzeTransaction generates an insight for a new transaction
func (a *Analyzer) AnalyzeTransaction(ctx context.Context, txn *models.Transaction) (*models.Insight, error) {
	if !a.chain.IsConfigured() {
		return nil, nil // No providers configured
	}

	// Skip transfers and internal transactions
	if txn.IsTransfer {
		return nil, nil
	}

	slog.InfoContext(ctx, "analyzing transaction", "id", txn.ID, "description", txn.Description)

	// Build transaction summary
	txnSummary := a.buildTransactionSummary(txn)

	// Get recent context (last 90 days)
	recentTxns, err := a.aggregator.getRecentTransactions(ctx, txn.LedgerID, 90)
	if err != nil {
		slog.WarnContext(ctx, "failed to get recent transactions for insight context", "err", err)
		// Continue without context
		recentTxns = nil
	}

	// Get month-to-date summary
	monthToDate, _ := a.aggregator.getMonthToDate(ctx, txn.LedgerID)

	// Get last month summary for comparison
	lastMonth, _ := a.aggregator.getLastMonthSummary(ctx, txn.LedgerID)

	// Build request
	req := providers.TransactionRequest{
		Transaction:        txnSummary,
		RecentTransactions: recentTxns,
		MonthToDate:        monthToDate,
		LastMonthSummary:   lastMonth,
	}

	// Generate insight
	result, providerName, modelName, err := a.chain.AnalyzeTransaction(ctx, req)
	if err != nil {
		slog.WarnContext(ctx, "failed to analyze transaction", "txn_id", txn.ID, "err", err)
		return nil, err
	}

	// Only store meaningful insights (importance >= 4 or is_key)
	if result.Importance < 4 && !result.IsKey {
		slog.DebugContext(ctx, "skipping low-importance insight", "importance", result.Importance)
		return nil, nil
	}

	// Create and store insight
	insight := &models.Insight{
		LedgerID:      txn.LedgerID,
		TransactionID: &txn.ID,
		InsightType:   models.InsightType(result.Type),
		Content:       result.Content,
		Importance:    result.Importance,
		IsKey:         result.IsKey,
		LLMProvider:   providerName,
		LLMModel:      modelName,
	}

	if err := a.insights.Create(ctx, insight); err != nil {
		slog.WarnContext(ctx, "failed to store insight", "txn_id", txn.ID, "err", err)
		return nil, err
	}

	slog.InfoContext(ctx, "created insight", "txn_id", txn.ID, "importance", result.Importance, "is_key", result.IsKey)

	return insight, nil
}

// IsConfigured returns true if the analyzer has at least one LLM provider configured
func (a *Analyzer) IsConfigured() bool {
	return a.chain.IsConfigured()
}

// buildTransactionSummary converts a transaction to a summary for LLM context
func (a *Analyzer) buildTransactionSummary(txn *models.Transaction) providers.TransactionSummary {
	// Get primary amount and account
	var amount int64
	var accountName string
	for _, e := range txn.Entries {
		if e.AccountType == models.AccountTypeAsset || e.AccountType == models.AccountTypeLiability {
			amount = e.AmountCents
			accountName = e.AccountName
			// Flip sign for liabilities
			if e.AccountType == models.AccountTypeLiability {
				amount = -amount
			}
			break
		}
	}

	// Get category
	categoryName := "Uncategorized"
	if len(txn.Tags) > 0 {
		categoryName = txn.Tags[0].Name
	}

	// Get entity name
	entityName := ""
	if txn.Entity != nil {
		entityName = txn.Entity.Name
	}

	return providers.TransactionSummary{
		ID:           txn.ID,
		Date:         txn.Date,
		Description:  txn.Description,
		DisplayTitle: txn.DisplayTitle,
		AmountCents:  amount,
		CategoryName: categoryName,
		EntityName:   entityName,
		AccountName:  accountName,
		IsRecurring:  txn.RecurringPatternID != nil,
	}
}
