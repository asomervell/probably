package insights

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/asomervell/probably/internal/insights/providers"
	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DateRange represents a time period
type DateRange struct {
	Start time.Time
	End   time.Time
}

// FinancialSnapshot contains all aggregated financial data for a period
type FinancialSnapshot struct {
	LedgerID uuid.UUID
	Period   DateRange

	// Account data
	Accounts []providers.AccountSummary

	// Transaction data
	Transactions    []providers.TransactionSummary
	TotalIncome     int64
	TotalExpenses   int64
	NetSavings      int64
	TransactionCount int

	// Category breakdown
	CategoryTotals map[string]int64

	// Entity breakdown
	EntityTotals []providers.EntityTotal

	// For comparison
	PriorPeriod *FinancialSnapshot
}

// Aggregator gathers financial data for insight generation
type Aggregator struct {
	pool         *pgxpool.Pool
	accounts     *models.AccountStore
	transactions *models.TransactionStore
	tags         *models.TagStore
	entities     *models.EntityStore
}

// NewAggregator creates a new data aggregator
func NewAggregator(pool *pgxpool.Pool) *Aggregator {
	return &Aggregator{
		pool:         pool,
		accounts:     models.NewAccountStore(pool),
		transactions: models.NewTransactionStore(pool),
		tags:         models.NewTagStore(pool),
		entities:     models.NewEntityStore(pool),
	}
}

// BuildSnapshot creates a complete financial snapshot for the given period
func (a *Aggregator) BuildSnapshot(ctx context.Context, ledgerID uuid.UUID, period DateRange) (*FinancialSnapshot, error) {
	snapshot := &FinancialSnapshot{
		LedgerID:       ledgerID,
		Period:         period,
		CategoryTotals: make(map[string]int64),
	}

	// Get accounts with balances
	accounts, err := a.accounts.GetWithBalances(ctx, ledgerID)
	if err != nil {
		return nil, err
	}
	for _, acc := range accounts {
		snapshot.Accounts = append(snapshot.Accounts, providers.AccountSummary{
			ID:              acc.ID,
			Name:            acc.Name,
			Type:            string(acc.Type),
			InstitutionName: acc.InstitutionName,
			BalanceCents:    acc.Balance,
		})
	}

	// Get transactions for the period
	txns, err := a.getTransactionsForPeriod(ctx, ledgerID, period.Start, period.End)
	if err != nil {
		return nil, err
	}

	// Build account lookup for type checking
	accountTypes := make(map[uuid.UUID]models.AccountType)
	for _, acc := range accounts {
		accountTypes[acc.ID] = acc.Type
	}

	// Process transactions
	entityMap := make(map[uuid.UUID]*providers.EntityTotal)

	for _, txn := range txns {
		// Load entries if not loaded
		if len(txn.Entries) == 0 {
			if err := a.transactions.LoadEntries(ctx, txn); err != nil {
				slog.WarnContext(ctx, "failed to load transaction entries", "transaction_id", txn.ID, "error", err)
			}
		}
		// Load tags if not loaded
		if len(txn.Tags) == 0 {
			if err := a.transactions.LoadTags(ctx, txn); err != nil {
				slog.WarnContext(ctx, "failed to load transaction tags", "transaction_id", txn.ID, "error", err)
			}
		}

		// Find the primary entry (asset/liability account)
		var amount int64
		var accountName string
		for _, e := range txn.Entries {
			accType := accountTypes[e.AccountID]
			if accType == models.AccountTypeAsset || accType == models.AccountTypeLiability {
				amount = e.AmountCents
				accountName = e.AccountName
				// Flip sign for liabilities to show user perspective
				if accType == models.AccountTypeLiability {
					amount = -amount
				}
				break
			}
		}

		// Get category name
		categoryName := "Uncategorized"
		if len(txn.Tags) > 0 {
			categoryName = txn.Tags[0].Name
		}

		// Get entity name
		entityName := ""
		if txn.Entity != nil {
			entityName = txn.Entity.Name
		} else if txn.EntityID != nil {
			// Load entity if needed
			if entity, err := a.entities.GetByID(ctx, *txn.EntityID); err == nil {
				entityName = entity.Name
			}
		}

		// Build transaction summary
		summary := providers.TransactionSummary{
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
		snapshot.Transactions = append(snapshot.Transactions, summary)
		snapshot.TransactionCount++

		// Skip transfers for income/expense calculations
		if txn.IsTransfer {
			continue
		}

		// Aggregate totals
		if amount > 0 {
			snapshot.TotalIncome += amount
		} else {
			snapshot.TotalExpenses += -amount // Store as positive
		}

		// Category totals (expenses only, as positive values)
		if amount < 0 {
			snapshot.CategoryTotals[categoryName] += -amount
		}

		// Entity totals (expenses only)
		if amount < 0 && txn.EntityID != nil {
			if et, ok := entityMap[*txn.EntityID]; ok {
				et.AmountCents += -amount
				et.Count++
			} else {
				entityMap[*txn.EntityID] = &providers.EntityTotal{
					EntityID:   *txn.EntityID,
					EntityName: entityName,
					AmountCents:  -amount,
					Count:        1,
				}
			}
		}
	}

	// Calculate net savings
	snapshot.NetSavings = snapshot.TotalIncome - snapshot.TotalExpenses

	// Sort entities by spend
	for _, et := range entityMap {
		snapshot.EntityTotals = append(snapshot.EntityTotals, *et)
	}
	sort.Slice(snapshot.EntityTotals, func(i, j int) bool {
		return snapshot.EntityTotals[i].AmountCents > snapshot.EntityTotals[j].AmountCents
	})

	// Sort transactions by date (newest first)
	sort.Slice(snapshot.Transactions, func(i, j int) bool {
		return snapshot.Transactions[i].Date.After(snapshot.Transactions[j].Date)
	})

	return snapshot, nil
}

// BuildSnapshotWithComparison builds a snapshot with prior period comparison
func (a *Aggregator) BuildSnapshotWithComparison(ctx context.Context, ledgerID uuid.UUID, period DateRange) (*FinancialSnapshot, error) {
	snapshot, err := a.BuildSnapshot(ctx, ledgerID, period)
	if err != nil {
		return nil, err
	}

	// Calculate prior period (same duration, immediately before)
	duration := period.End.Sub(period.Start)
	priorPeriod := DateRange{
		Start: period.Start.Add(-duration),
		End:   period.Start.Add(-time.Second), // Day before current period
	}

	// Adjust for month boundaries
	if isMonthStart(period.Start) {
		priorPeriod = getPriorMonth(period.Start)
	}

	priorSnapshot, err := a.BuildSnapshot(ctx, ledgerID, priorPeriod)
	if err != nil {
		slog.WarnContext(ctx, "failed to build prior period snapshot", "ledger_id", ledgerID, "error", err)
	} else {
		snapshot.PriorPeriod = priorSnapshot
	}

	return snapshot, nil
}

func (a *Aggregator) getRecentTransactions(ctx context.Context, ledgerID uuid.UUID, days int) ([]providers.TransactionSummary, error) {
	end := time.Now()
	start := end.AddDate(0, 0, -days)

	snapshot, err := a.BuildSnapshot(ctx, ledgerID, DateRange{Start: start, End: end})
	if err != nil {
		return nil, err
	}

	return snapshot.Transactions, nil
}

func (a *Aggregator) getMonthToDate(ctx context.Context, ledgerID uuid.UUID) (*providers.PeriodSummary, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return a.buildPeriodSummary(ctx, ledgerID, start, now)
}

func (a *Aggregator) getLastMonthSummary(ctx context.Context, ledgerID uuid.UUID) (*providers.PeriodSummary, error) {
	now := time.Now().UTC()
	lastMonth := now.AddDate(0, -1, 0)
	start := time.Date(lastMonth.Year(), lastMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Add(-time.Second)
	return a.buildPeriodSummary(ctx, ledgerID, start, end)
}

func (a *Aggregator) buildPeriodSummary(ctx context.Context, ledgerID uuid.UUID, start, end time.Time) (*providers.PeriodSummary, error) {
	snapshot, err := a.BuildSnapshot(ctx, ledgerID, DateRange{Start: start, End: end})
	if err != nil {
		return nil, err
	}
	return &providers.PeriodSummary{
		TotalIncome:   snapshot.TotalIncome,
		TotalExpenses: snapshot.TotalExpenses,
		ByCategory:    snapshot.CategoryTotals,
		TopEntities:   snapshot.EntityTotals,
	}, nil
}

// ToReportRequest converts a snapshot to a ReportRequest for the LLM
func (a *Aggregator) ToReportRequest(snapshot *FinancialSnapshot, reportType string, categoryTree string) providers.ReportRequest {
	req := providers.ReportRequest{
		LedgerID:       snapshot.LedgerID,
		ReportType:     reportType,
		PeriodStart:    snapshot.Period.Start,
		PeriodEnd:      snapshot.Period.End,
		Accounts:       snapshot.Accounts,
		Transactions:   snapshot.Transactions,
		CategoryTotals: snapshot.CategoryTotals,
		EntityTotals:   snapshot.EntityTotals,
		TotalIncome:    snapshot.TotalIncome,
		TotalExpenses:  snapshot.TotalExpenses,
		NetSavings:     snapshot.NetSavings,
		CategoryTree:   categoryTree,
	}

	// Add prior period if available
	if snapshot.PriorPeriod != nil {
		req.PriorPeriod = &providers.ReportRequest{
			TotalIncome:    snapshot.PriorPeriod.TotalIncome,
			TotalExpenses:  snapshot.PriorPeriod.TotalExpenses,
			NetSavings:     snapshot.PriorPeriod.NetSavings,
			CategoryTotals: snapshot.PriorPeriod.CategoryTotals,
		}
	}

	return req
}

// Helper: get transactions for a period
func (a *Aggregator) getTransactionsForPeriod(ctx context.Context, ledgerID uuid.UUID, start, end time.Time) ([]*models.Transaction, error) {
	txns, _, err := a.transactions.List(ctx, models.TransactionFilter{
		LedgerID:  ledgerID,
		StartDate: &start,
		EndDate:   &end,
		Limit:     10000, // Get all transactions in period
	})
	return txns, err
}

// Helper: check if date is start of month
func isMonthStart(t time.Time) bool {
	return t.Day() == 1
}

// Helper: get prior month date range
func getPriorMonth(t time.Time) DateRange {
	priorMonth := t.AddDate(0, -1, 0)
	// Use UTC consistently with other period calculations (BuildMonthSnapshot, BuildQuarterSnapshot, etc.)
	start := time.Date(priorMonth.Year(), priorMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Add(-time.Second)
	return DateRange{Start: start, End: end}
}
