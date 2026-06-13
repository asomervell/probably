package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReportType defines the period type for a report
type ReportType string

const (
	ReportTypeMonthly   ReportType = "monthly"
	ReportTypeQuarterly ReportType = "quarterly"
	ReportTypeAnnual    ReportType = "annual"
)

// EntitySpending represents spending data for an entity in a report
type EntitySpending struct {
	EntityID    uuid.UUID `json:"entity_id"`
	EntityName  string    `json:"entity_name"`
	AmountCents int64     `json:"amount_cents"`
	Count       int       `json:"count"`
}

// Report represents a periodic financial report (monthly, quarterly, annual)
type Report struct {
	ID          uuid.UUID  `json:"id"`
	LedgerID    uuid.UUID  `json:"ledger_id"`
	ReportType  ReportType `json:"report_type"`
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`

	// Aggregated metrics
	TotalIncomeCents   int64            `json:"total_income_cents"`
	TotalExpensesCents int64            `json:"total_expenses_cents"`
	NetSavingsCents    int64            `json:"net_savings_cents"`
	CategoryBreakdown  map[string]int64 `json:"category_breakdown,omitempty"`
	TopEntities        []EntitySpending `json:"top_entities,omitempty"`

	// LLM-generated content
	Summary         string   `json:"summary,omitempty"`
	Highlights      []string `json:"highlights,omitempty"`
	Recommendations []string `json:"recommendations,omitempty"`
	ComparisonNotes string   `json:"comparison_notes,omitempty"`

	// Generation metadata
	LLMProvider string     `json:"llm_provider,omitempty"`
	LLMModel    string     `json:"llm_model,omitempty"`
	GeneratedAt *time.Time `json:"generated_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ReportStore struct {
	pool *pgxpool.Pool
}

func NewReportStore(pool *pgxpool.Pool) *ReportStore {
	return &ReportStore{pool: pool}
}

func (s *ReportStore) Create(ctx context.Context, r *Report) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	now := time.Now()
	r.CreatedAt = now
	r.UpdatedAt = now

	categoryJSON, _ := json.Marshal(r.CategoryBreakdown)
	entitiesJSON, _ := json.Marshal(r.TopEntities)
	highlightsJSON, _ := json.Marshal(r.Highlights)
	recommendationsJSON, _ := json.Marshal(r.Recommendations)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO reports (
			id, ledger_id, report_type, period_start, period_end,
			total_income_cents, total_expenses_cents, net_savings_cents,
			category_breakdown, top_merchants,
			summary, highlights, recommendations, comparison_notes,
			llm_provider, llm_model, generated_at,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
	`, r.ID, r.LedgerID, r.ReportType, r.PeriodStart, r.PeriodEnd,
		r.TotalIncomeCents, r.TotalExpensesCents, r.NetSavingsCents,
		categoryJSON, entitiesJSON,
		NullString(r.Summary), highlightsJSON, recommendationsJSON, NullString(r.ComparisonNotes),
		NullString(r.LLMProvider), NullString(r.LLMModel), r.GeneratedAt,
		r.CreatedAt, r.UpdatedAt)

	return err
}

func (s *ReportStore) Update(ctx context.Context, r *Report) error {
	r.UpdatedAt = time.Now()

	categoryJSON, _ := json.Marshal(r.CategoryBreakdown)
	entitiesJSON, _ := json.Marshal(r.TopEntities)
	highlightsJSON, _ := json.Marshal(r.Highlights)
	recommendationsJSON, _ := json.Marshal(r.Recommendations)

	_, err := s.pool.Exec(ctx, `
		UPDATE reports SET
			total_income_cents = $2, total_expenses_cents = $3, net_savings_cents = $4,
			category_breakdown = $5, top_merchants = $6,
			summary = $7, highlights = $8, recommendations = $9, comparison_notes = $10,
			llm_provider = $11, llm_model = $12, generated_at = $13,
			updated_at = $14
		WHERE id = $1
	`, r.ID, r.TotalIncomeCents, r.TotalExpensesCents, r.NetSavingsCents,
		categoryJSON, entitiesJSON,
		NullString(r.Summary), highlightsJSON, recommendationsJSON, NullString(r.ComparisonNotes),
		NullString(r.LLMProvider), NullString(r.LLMModel), r.GeneratedAt,
		r.UpdatedAt)

	return err
}

func (s *ReportStore) Upsert(ctx context.Context, r *Report) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	now := time.Now()
	r.UpdatedAt = now

	categoryJSON, _ := json.Marshal(r.CategoryBreakdown)
	entitiesJSON, _ := json.Marshal(r.TopEntities)
	highlightsJSON, _ := json.Marshal(r.Highlights)
	recommendationsJSON, _ := json.Marshal(r.Recommendations)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO reports (
			id, ledger_id, report_type, period_start, period_end,
			total_income_cents, total_expenses_cents, net_savings_cents,
			category_breakdown, top_merchants,
			summary, highlights, recommendations, comparison_notes,
			llm_provider, llm_model, generated_at,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		ON CONFLICT (ledger_id, report_type, period_start) DO UPDATE SET
			total_income_cents = EXCLUDED.total_income_cents,
			total_expenses_cents = EXCLUDED.total_expenses_cents,
			net_savings_cents = EXCLUDED.net_savings_cents,
			category_breakdown = EXCLUDED.category_breakdown,
			top_merchants = EXCLUDED.top_merchants,
			summary = EXCLUDED.summary,
			highlights = EXCLUDED.highlights,
			recommendations = EXCLUDED.recommendations,
			comparison_notes = EXCLUDED.comparison_notes,
			llm_provider = EXCLUDED.llm_provider,
			llm_model = EXCLUDED.llm_model,
			generated_at = EXCLUDED.generated_at,
			updated_at = EXCLUDED.updated_at
	`, r.ID, r.LedgerID, r.ReportType, r.PeriodStart, r.PeriodEnd,
		r.TotalIncomeCents, r.TotalExpensesCents, r.NetSavingsCents,
		categoryJSON, entitiesJSON,
		NullString(r.Summary), highlightsJSON, recommendationsJSON, NullString(r.ComparisonNotes),
		NullString(r.LLMProvider), NullString(r.LLMModel), r.GeneratedAt,
		now, now)

	return err
}

// reportSelectCols is the shared SELECT column list used by all report read queries.
const reportSelectCols = `id, ledger_id, report_type, period_start, period_end,
	total_income_cents, total_expenses_cents, net_savings_cents,
	category_breakdown, top_merchants,
	summary, highlights, recommendations, comparison_notes,
	llm_provider, llm_model, generated_at,
	created_at, updated_at`

// reportScan holds the intermediate scan targets for nullable/JSON columns.
type reportScan struct {
	summary, comparisonNotes, llmProvider, llmModel      sql.NullString
	generatedAt                                           sql.NullTime
	categoryJSON, entitiesJSON, highlightsJSON, recommendationsJSON []byte
}

func (sc *reportScan) targets(r *Report) []any {
	return []any{
		&r.ID, &r.LedgerID, &r.ReportType, &r.PeriodStart, &r.PeriodEnd,
		&r.TotalIncomeCents, &r.TotalExpensesCents, &r.NetSavingsCents,
		&sc.categoryJSON, &sc.entitiesJSON,
		&sc.summary, &sc.highlightsJSON, &sc.recommendationsJSON, &sc.comparisonNotes,
		&sc.llmProvider, &sc.llmModel, &sc.generatedAt,
		&r.CreatedAt, &r.UpdatedAt,
	}
}

func (sc *reportScan) populate(r *Report) {
	r.Summary = sc.summary.String
	r.ComparisonNotes = sc.comparisonNotes.String
	r.LLMProvider = sc.llmProvider.String
	r.LLMModel = sc.llmModel.String
	if sc.generatedAt.Valid {
		r.GeneratedAt = &sc.generatedAt.Time
	}
	if len(sc.categoryJSON) > 0 {
		_ = json.Unmarshal(sc.categoryJSON, &r.CategoryBreakdown)
	}
	if len(sc.entitiesJSON) > 0 {
		_ = json.Unmarshal(sc.entitiesJSON, &r.TopEntities)
	}
	if len(sc.highlightsJSON) > 0 {
		_ = json.Unmarshal(sc.highlightsJSON, &r.Highlights)
	}
	if len(sc.recommendationsJSON) > 0 {
		_ = json.Unmarshal(sc.recommendationsJSON, &r.Recommendations)
	}
}

func (s *ReportStore) GetByID(ctx context.Context, id uuid.UUID) (*Report, error) {
	r := &Report{}
	var sc reportScan
	err := s.pool.QueryRow(ctx, `SELECT `+reportSelectCols+` FROM reports WHERE id = $1`, id).Scan(sc.targets(r)...)
	if err != nil {
		return nil, err
	}
	sc.populate(r)
	return r, nil
}

// GetByPeriod retrieves a report for a specific ledger, type, and period start
func (s *ReportStore) GetByPeriod(ctx context.Context, ledgerID uuid.UUID, reportType ReportType, periodStart time.Time) (*Report, error) {
	r := &Report{}
	var sc reportScan
	err := s.pool.QueryRow(ctx,
		`SELECT `+reportSelectCols+` FROM reports WHERE ledger_id = $1 AND report_type = $2 AND period_start = $3`,
		ledgerID, reportType, periodStart).Scan(sc.targets(r)...)
	if err != nil {
		return nil, err
	}
	sc.populate(r)
	return r, nil
}

// ListByLedger retrieves all reports for a ledger, optionally filtered by type
func (s *ReportStore) ListByLedger(ctx context.Context, ledgerID uuid.UUID, reportType *ReportType, limit int) ([]*Report, error) {
	query := `SELECT ` + reportSelectCols + ` FROM reports WHERE ledger_id = $1`
	args := []interface{}{ledgerID}

	if reportType != nil {
		query += ` AND report_type = $2`
		args = append(args, *reportType)
	}

	query += ` ORDER BY period_start DESC`

	if limit > 0 {
		query += ` LIMIT $` + strconv.Itoa(len(args)+1)
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []*Report
	for rows.Next() {
		r := &Report{}
		var sc reportScan
		if err := rows.Scan(sc.targets(r)...); err != nil {
			return nil, err
		}
		sc.populate(r)
		reports = append(reports, r)
	}

	return reports, rows.Err()
}

func (s *ReportStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM reports WHERE id = $1`, id)
	return err
}
