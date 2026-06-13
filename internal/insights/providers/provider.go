package providers

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ReportRequest contains all data needed to generate a financial report
type ReportRequest struct {
	LedgerID    uuid.UUID
	ReportType  string    // "monthly", "quarterly", "annual"
	PeriodStart time.Time
	PeriodEnd   time.Time

	// Financial data
	Accounts           []AccountSummary
	Transactions       []TransactionSummary
	CategoryTotals map[string]int64
	EntityTotals   []EntityTotal
	TotalIncome        int64
	TotalExpenses      int64
	NetSavings         int64

	// Prior period for comparison
	PriorPeriod *ReportRequest

	// User's category taxonomy
	CategoryTree string
}

// AccountSummary represents an account for LLM context
type AccountSummary struct {
	ID              uuid.UUID
	Name            string
	Type            string // "asset", "liability", "income", "expense"
	InstitutionName string
	BalanceCents    int64
}

// TransactionSummary represents a transaction for LLM context
type TransactionSummary struct {
	ID           uuid.UUID
	Date         time.Time
	Description  string
	DisplayTitle string
	AmountCents  int64
	CategoryName string
	EntityName   string
	AccountName  string
	IsRecurring  bool
}

// EntityTotal represents spending at an entity
type EntityTotal struct {
	EntityID   uuid.UUID
	EntityName string
	AmountCents  int64
	Count        int
}

// ReportResponse contains the LLM-generated report content
type ReportResponse struct {
	Summary         string   // Natural language summary
	Highlights      []string // Key observations
	Recommendations []string // Actionable advice
	ComparisonNotes string   // vs previous period
	KeyInsights     []InsightResponse // Individual insights to store
}

// TransactionRequest contains context for analyzing a single transaction
type TransactionRequest struct {
	// The transaction to analyze
	Transaction TransactionSummary

	// Recent context
	RecentTransactions []TransactionSummary // Last 30-90 days
	MonthToDate        *PeriodSummary       // Current month progress
	LastMonthSummary   *PeriodSummary       // Previous month for comparison
	
	// Category taxonomy
	CategoryTree string
}

// PeriodSummary provides context for transaction analysis
type PeriodSummary struct {
	TotalIncome   int64
	TotalExpenses int64
	ByCategory   map[string]int64
	TopEntities  []EntityTotal
}

// InsightResponse represents a single insight from LLM analysis
type InsightResponse struct {
	Content    string `json:"content"`
	Importance int    `json:"importance"` // 1-10
	IsKey      bool   `json:"is_key"`     // Should surface to dashboard
	Type       string `json:"type"`       // "spending_alert", "trend", "recommendation", etc.
}

// TransactionInsight is the response from analyzing a transaction
type TransactionInsight struct {
	Content    string `json:"content"`
	Importance int    `json:"importance"`
	IsKey      bool   `json:"is_key"`
	Type       string `json:"type"`
}

// InsightProvider defines the interface for LLM providers that generate financial insights
type InsightProvider interface {
	// Name returns the provider name (e.g., "grok", "vertex", "groq")
	Name() string

	// IsConfigured returns true if the provider has valid configuration
	IsConfigured() bool

	// GenerateReport generates insights for a periodic financial report
	GenerateReport(ctx context.Context, req ReportRequest) (*ReportResponse, error)

	// AnalyzeTransaction generates insights for a single new transaction
	AnalyzeTransaction(ctx context.Context, req TransactionRequest) (*TransactionInsight, error)
}
