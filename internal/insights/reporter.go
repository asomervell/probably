package insights

import (
	"context"
	"log/slog"
	"time"

	"github.com/asomervell/probably/internal/categorize"
	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/insights/providers"
	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Reporter generates financial reports using LLM analysis
type Reporter struct {
	aggregator *Aggregator
	chain      *providers.ProviderChain
	reports    *models.ReportStore
	insights   *models.InsightStore
	tags       *models.TagStore
}

func buildDefaultProviderChain(cfg *config.Config) *providers.ProviderChain {
	claude := providers.NewClaudeProvider(cfg.AnthropicAPIKey, cfg.ClaudeModel)
	grok := providers.NewGrokProvider(cfg.XAIAPIKey, cfg.GrokModel)
	vertex := providers.NewVertexProvider(cfg.GoogleAPIKey, cfg.VertexModel)
	groq := providers.NewGroqProvider(cfg.GroqAPIKey, cfg.GroqModel)

	// In demo mode Claude is the primary path; otherwise it is a fallback
	// after the existing providers. NewProviderChain filters out any provider
	// whose key is unconfigured, so an empty Anthropic key is safe.
	var chain []providers.InsightProvider
	if cfg.DemoMode {
		chain = []providers.InsightProvider{claude, grok, vertex, groq}
	} else {
		chain = []providers.InsightProvider{grok, vertex, groq, claude}
	}

	return providers.NewProviderChain(chain)
}

// NewReporter creates a new report generator
func NewReporter(cfg *config.Config, pool *pgxpool.Pool) *Reporter {
	return &Reporter{
		aggregator: NewAggregator(pool),
		chain:      buildDefaultProviderChain(cfg),
		reports:    models.NewReportStore(pool),
		insights:   models.NewInsightStore(pool),
		tags:       models.NewTagStore(pool),
	}
}

// GenerateMonthlyReport generates or regenerates a monthly report
func (r *Reporter) GenerateMonthlyReport(ctx context.Context, ledgerID uuid.UUID, year int, month time.Month) (*models.Report, error) {
	slog.InfoContext(ctx, "generating monthly report", "month", month, "year", year)

	// Build the date range
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0).Add(-time.Second)
	period := DateRange{Start: start, End: end}

	return r.generateReport(ctx, ledgerID, period, models.ReportTypeMonthly)
}

// GenerateQuarterlyReport generates or regenerates a quarterly report
func (r *Reporter) GenerateQuarterlyReport(ctx context.Context, ledgerID uuid.UUID, year int, quarter int) (*models.Report, error) {
	slog.InfoContext(ctx, "generating quarterly report", "quarter", quarter, "year", year)

	startMonth := time.Month((quarter-1)*3 + 1)
	start := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 3, 0).Add(-time.Second)
	period := DateRange{Start: start, End: end}

	return r.generateReport(ctx, ledgerID, period, models.ReportTypeQuarterly)
}

// GenerateAnnualReport generates or regenerates an annual report
func (r *Reporter) GenerateAnnualReport(ctx context.Context, ledgerID uuid.UUID, year int) (*models.Report, error) {
	slog.InfoContext(ctx, "generating annual report", "year", year)

	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Second)
	period := DateRange{Start: start, End: end}

	return r.generateReport(ctx, ledgerID, period, models.ReportTypeAnnual)
}

// generateReport is the core report generation logic
func (r *Reporter) generateReport(ctx context.Context, ledgerID uuid.UUID, period DateRange, reportType models.ReportType) (*models.Report, error) {
	// Build financial snapshot with comparison
	snapshot, err := r.aggregator.BuildSnapshotWithComparison(ctx, ledgerID, period)
	if err != nil {
		return nil, err
	}

	// Get category tree for LLM context
	tags, err := r.tags.GetByLedgerID(ctx, ledgerID)
	if err != nil {
		return nil, err
	}
	categoryTree := categorize.GetCategoryTreeForPrompt(tags)

	// Convert to LLM request
	req := r.aggregator.ToReportRequest(snapshot, string(reportType), categoryTree)

	// Generate report using LLM chain
	resp, providerName, modelName, err := r.chain.GenerateReport(ctx, req)
	if err != nil {
		slog.WarnContext(ctx, "LLM report generation failed", "err", err)
		// Still create a report with just the metrics
		resp = &providers.ReportResponse{}
	}

	// Build entity totals for storage
	var topEntities []models.EntitySpending
	for i, et := range snapshot.EntityTotals {
		if i >= 10 {
			break
		}
		topEntities = append(topEntities, models.EntitySpending{
			EntityID:    et.EntityID,
			EntityName:  et.EntityName,
			AmountCents: et.AmountCents,
			Count:       et.Count,
		})
	}

	now := time.Now()

	// Create report model
	report := &models.Report{
		LedgerID:           ledgerID,
		ReportType:         reportType,
		PeriodStart:        period.Start,
		PeriodEnd:          period.End,
		TotalIncomeCents:   snapshot.TotalIncome,
		TotalExpensesCents: snapshot.TotalExpenses,
		NetSavingsCents:    snapshot.NetSavings,
		CategoryBreakdown:  snapshot.CategoryTotals,
		TopEntities:        topEntities,
		Summary:            resp.Summary,
		Highlights:         resp.Highlights,
		Recommendations:    resp.Recommendations,
		ComparisonNotes:    resp.ComparisonNotes,
		LLMProvider:        providerName,
		LLMModel:           modelName,
		GeneratedAt:        &now,
	}

	// Upsert the report (will update if exists for this period)
	if err := r.reports.Upsert(ctx, report); err != nil {
		return nil, err
	}

	// Get the report ID (may have been set by upsert)
	existing, err := r.reports.GetByPeriod(ctx, ledgerID, reportType, period.Start)
	if err != nil {
		slog.WarnContext(ctx, "failed to fetch report ID after upsert", "err", err)
	}
	if existing != nil {
		report.ID = existing.ID
	}

	// Delete any existing insights for this report (for regeneration)
	if report.ID != uuid.Nil {
		if err := r.insights.DeleteByReport(ctx, report.ID); err != nil {
			slog.WarnContext(ctx, "failed to delete existing insights before regeneration", "report_id", report.ID, "err", err)
		}
	}

	// Store key insights from LLM response
	for _, ki := range resp.KeyInsights {
		insight := &models.Insight{
			LedgerID:    ledgerID,
			ReportID:    &report.ID,
			InsightType: models.InsightType(ki.Type),
			Content:     ki.Content,
			Importance:  ki.Importance,
			IsKey:       ki.IsKey,
			LLMProvider: providerName,
			LLMModel:    modelName,
		}
		if err := r.insights.Create(ctx, insight); err != nil {
			slog.WarnContext(ctx, "failed to store report insight", "err", err)
		}
	}

	slog.InfoContext(ctx, "report generated",
		"income", snapshot.TotalIncome,
		"expenses", snapshot.TotalExpenses,
		"net", snapshot.NetSavings,
		"insights", len(resp.KeyInsights))

	return report, nil
}

// IsConfigured returns true if the reporter has at least one LLM provider configured
func (r *Reporter) IsConfigured() bool {
	return r.chain.IsConfigured()
}

// CheckPendingReports checks if any periodic reports need to be generated
// Returns list of (ledgerID, reportType, periodStart) that need generation
func (r *Reporter) CheckPendingReports(ctx context.Context, ledgerID uuid.UUID) ([]PendingReport, error) {
	var pending []PendingReport
	now := time.Now()

	// Check monthly report for last month
	lastMonth := now.AddDate(0, -1, 0)
	lastMonthStart := time.Date(lastMonth.Year(), lastMonth.Month(), 1, 0, 0, 0, 0, time.Local)

	_, err := r.reports.GetByPeriod(ctx, ledgerID, models.ReportTypeMonthly, lastMonthStart)
	if err != nil { // Report doesn't exist
		pending = append(pending, PendingReport{
			LedgerID:    ledgerID,
			ReportType:  models.ReportTypeMonthly,
			PeriodStart: lastMonthStart,
		})
	}

	// Check quarterly report (if we're in a new quarter)
	currentQuarter := (int(now.Month())-1)/3 + 1
	lastQuarter := currentQuarter - 1
	lastQuarterYear := now.Year()
	if lastQuarter <= 0 {
		lastQuarter = 4
		lastQuarterYear--
	}
	lastQuarterStart := time.Date(lastQuarterYear, time.Month((lastQuarter-1)*3+1), 1, 0, 0, 0, 0, time.Local)

	_, err = r.reports.GetByPeriod(ctx, ledgerID, models.ReportTypeQuarterly, lastQuarterStart)
	if err != nil {
		pending = append(pending, PendingReport{
			LedgerID:    ledgerID,
			ReportType:  models.ReportTypeQuarterly,
			PeriodStart: lastQuarterStart,
		})
	}

	// Check annual report (if we're in a new year)
	if now.Month() == time.January {
		lastYearStart := time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, time.Local)
		_, err = r.reports.GetByPeriod(ctx, ledgerID, models.ReportTypeAnnual, lastYearStart)
		if err != nil {
			pending = append(pending, PendingReport{
				LedgerID:    ledgerID,
				ReportType:  models.ReportTypeAnnual,
				PeriodStart: lastYearStart,
			})
		}
	}

	return pending, nil
}

// PendingReport represents a report that needs to be generated
type PendingReport struct {
	LedgerID    uuid.UUID
	ReportType  models.ReportType
	PeriodStart time.Time
}

// GeneratePendingReport generates a specific pending report
func (r *Reporter) GeneratePendingReport(ctx context.Context, pending PendingReport) (*models.Report, error) {
	switch pending.ReportType {
	case models.ReportTypeMonthly:
		return r.GenerateMonthlyReport(ctx, pending.LedgerID, pending.PeriodStart.Year(), pending.PeriodStart.Month())
	case models.ReportTypeQuarterly:
		quarter := (int(pending.PeriodStart.Month())-1)/3 + 1
		return r.GenerateQuarterlyReport(ctx, pending.LedgerID, pending.PeriodStart.Year(), quarter)
	case models.ReportTypeAnnual:
		return r.GenerateAnnualReport(ctx, pending.LedgerID, pending.PeriodStart.Year())
	}
	return nil, nil
}
