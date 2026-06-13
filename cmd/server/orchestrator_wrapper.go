package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/orchestrator"
)

// WorkerOrchestratorWrapper wraps the orchestrator to provide a simple interface for the worker
// This breaks the import cycle by having the wrapper in main package
type WorkerOrchestratorWrapper struct {
	orch *orchestrator.Orchestrator
}

// NewWorkerOrchestratorWrapper creates a wrapper around an orchestrator
func NewWorkerOrchestratorWrapper(orch *orchestrator.Orchestrator) *WorkerOrchestratorWrapper {
	return &WorkerOrchestratorWrapper{orch: orch}
}

// Execute executes a categorization task
func (w *WorkerOrchestratorWrapper) Execute(ctx context.Context, task interface{}) (interface{}, error) {
	// Convert interface{} to orchestrator.Task
	orchTask, ok := task.(*orchestrator.Task)
	if !ok {
		return nil, fmt.Errorf("invalid task type: %T", task)
	}

	result, err := w.orch.Execute(ctx, orchTask)
	if err != nil {
		return nil, err
	}

	// Convert Result to a map so worker can extract Output without importing orchestrator
	return map[string]interface{}{
		"output":     result.Output,
		"confidence": result.Confidence,
		"model_path": result.ModelPath,
		"escalated":  result.Escalated,
		"iterations": result.Iterations,
		"metadata":   result.Metadata,
		"thoughts":   result.Thoughts, // AI reasoning for UI display (Gemini 3+)
	}, nil
}

// IsConfigured returns whether the orchestrator is configured
func (w *WorkerOrchestratorWrapper) IsConfigured() bool {
	return w.orch.IsConfigured()
}

// CallPrompt sends a prompt directly to the LLM and returns the JSON response
func (w *WorkerOrchestratorWrapper) CallPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return w.orch.CallPrompt(ctx, systemPrompt, userPrompt)
}

// FormatMyLifeContext formats relationships into a short context string for LLM prompts
// Output format: "People: Sarah (partner), Dad (parent) | Work: Acme (employer) | Assets: Tesla (vehicle)"
func FormatMyLifeContext(relationships []*models.Relationship) string {
	if len(relationships) == 0 {
		return ""
	}

	// Group by category
	people := []string{}
	work := []string{}
	assets := []string{}

	for _, rel := range relationships {
		entry := fmt.Sprintf("%s (%s)", rel.Name, rel.RelationshipType)
		switch rel.Category {
		case "person":
			people = append(people, entry)
		case "work":
			work = append(work, entry)
		case "asset":
			assets = append(assets, entry)
		}
	}

	var parts []string
	if len(people) > 0 {
		parts = append(parts, "People: "+strings.Join(people, ", "))
	}
	if len(work) > 0 {
		parts = append(parts, "Work: "+strings.Join(work, ", "))
	}
	if len(assets) > 0 {
		parts = append(parts, "Assets: "+strings.Join(assets, ", "))
	}

	return strings.Join(parts, " | ")
}

// BuildCategorizeTask builds a categorization task for the orchestrator
func BuildCategorizeTask(ledgerID string, strategy string, transactionInputs, tagInputs, ruleInputs []interface{}, entitySearcher, purchaseMatcher interface{}, relationships []*models.Relationship) *orchestrator.Task {
	orchStrategy := orchestrator.Strategy(strategy)
	if orchStrategy == "" {
		orchStrategy = orchestrator.StrategyEscalate
	}

	return &orchestrator.Task{
		Type:     orchestrator.TaskTypeCategorize,
		Strategy: orchStrategy,
		Input: &orchestrator.CategorizeInput{
			Transactions:    transactionInputs,
			Tags:            tagInputs,
			Rules:           ruleInputs,
			EntitySearcher:  entitySearcher,
			PurchaseMatcher: purchaseMatcher,
			MyLifeContext:   FormatMyLifeContext(relationships),
		},
		Context: &orchestrator.TaskContext{
			LedgerID: ledgerID,
		},
	}
}

// BuildP2PTask builds a P2P categorization task for the orchestrator
func BuildP2PTask(ledgerID string, strategy string, transactionInputs, tagInputs []interface{}, householdPatterns []string, relationships []*models.Relationship) *orchestrator.Task {
	orchStrategy := orchestrator.Strategy(strategy)
	if orchStrategy == "" {
		orchStrategy = orchestrator.StrategyEscalate
	}

	metadata := make(map[string]string)
	if len(householdPatterns) > 0 {
		metadata["household_patterns"] = strings.Join(householdPatterns, ",")
	}

	return &orchestrator.Task{
		Type:     orchestrator.TaskTypeCategorizeP2P,
		Strategy: orchStrategy,
		Input: &orchestrator.CategorizeInput{
			Transactions:  transactionInputs,
			Tags:          tagInputs,
			Rules:         []interface{}{}, // P2P doesn't use rules
			MyLifeContext: FormatMyLifeContext(relationships),
		},
		Context: &orchestrator.TaskContext{
			LedgerID: ledgerID,
			Metadata: metadata,
		},
	}
}
