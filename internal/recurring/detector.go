// Package recurring implements recurring transaction detection algorithms.
// It analyzes transaction patterns to detect subscriptions, bills, and other
// recurring charges. Pattern results are stored directly on transaction rows
// (pattern_type, pattern_metadata columns).
package recurring

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// PatternDetectorInterface defines the interface for LLM-based pattern detection
// This allows the detector to use the orchestrator without importing it directly
type PatternDetectorInterface interface {
	CallPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	IsConfigured() bool
}

// Detector analyzes transaction patterns to detect recurring charges
type Detector struct {
	orchestrator PatternDetectorInterface
}

// NewDetector creates a new recurring pattern detector
func NewDetector() *Detector {
	return &Detector{}
}

// NewDetectorWithLLM creates a new detector with LLM support
func NewDetectorWithLLM(orchestrator PatternDetectorInterface) *Detector {
	return &Detector{orchestrator: orchestrator}
}

// DetectPatternsForEntity analyzes ALL transactions for a single entity
// This is the key insight: patterns can only be detected when you see the full history
// Returns multiple patterns since an entity may have several (e.g., Apple has iCloud, Apple One, etc.)
func (d *Detector) DetectPatternsForEntity(ctx context.Context, entity *models.Entity, entityName string, txns []*models.Transaction, ledgerID uuid.UUID) (*EntityPatternResult, error) {
	if d.orchestrator == nil || !d.orchestrator.IsConfigured() {
		return nil, fmt.Errorf("LLM not configured for pattern detection")
	}

	if len(txns) < 2 {
		// Need at least 2 transactions to detect a pattern
		return &EntityPatternResult{Patterns: []DetectedPattern{}}, nil
	}

	// Sort transactions by date (oldest first) for chronological display
	sortedTxns := make([]*models.Transaction, len(txns))
	copy(sortedTxns, txns)
	slices.SortFunc(sortedTxns, func(a, b *models.Transaction) int {
		return a.Date.Compare(b.Date)
	})

	// Get Teller category from transactions (most common one)
	tellerCategoryCount := make(map[string]int)
	for _, txn := range sortedTxns {
		if txn.TellerCategory != "" {
			tellerCategoryCount[txn.TellerCategory]++
		}
	}
	var tellerCategory string
	var maxCount int
	for cat, count := range tellerCategoryCount {
		if count > maxCount {
			maxCount = count
			tellerCategory = cat
		}
	}

	// Get entity subtype - important for pattern detection
	var entitySubtype string
	if entity != nil && entity.Subtype != "" {
		entitySubtype = entity.Subtype
	} else if tellerCategory != "" {
		// Fall back to deriving from Teller category
		entitySubtype = models.TellerCategoryToSubtype(tellerCategory)
	}

	// Build entity context with business type information
	entityCtx := &EntityPatternContext{
		Entity:         entity,
		EntityName:     entityName,
		Transactions:   sortedTxns,
		LedgerID:       ledgerID.String(),
		TellerCategory: tellerCategory,
		EntitySubtype:  entitySubtype,
	}

	// Build prompt
	userPrompt := BuildEntityPatternPrompt(entityCtx)
	systemPrompt := "You are a financial pattern detection expert. Analyze transaction history and identify ALL recurring patterns. A single merchant may have multiple distinct subscription patterns. Return valid JSON only."

	// Call LLM
	responseJSON, err := d.orchestrator.CallPrompt(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("entity pattern detection failed: %w", err)
	}

	// Clean up JSON if wrapped in code blocks
	responseJSON = strings.TrimSpace(responseJSON)
	if strings.HasPrefix(responseJSON, "```") {
		start := strings.Index(responseJSON, "\n")
		end := strings.LastIndex(responseJSON, "```")
		if start != -1 && end > start {
			responseJSON = strings.TrimSpace(responseJSON[start:end])
		}
	}

	// Parse result
	var result EntityPatternResult
	if err := json.Unmarshal([]byte(responseJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to parse entity pattern results: %w (raw: %s)", err, responseJSON[:min(500, len(responseJSON))])
	}

	return &result, nil
}

// IsP2PTransaction determines if a transaction is a P2P transfer based on bank data
// This is a simple, reliable check that trusts the bank's counterparty classification
// The actual transfer_type (person_payment, person_receipt, household, etc.) is determined
// by the LLM during categorization
func IsP2PTransaction(counterpartyType string) bool {
	// Trust bank data - if they say it's a person, it's a P2P transaction
	return strings.ToLower(counterpartyType) == "person"
}
