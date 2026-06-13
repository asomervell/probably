package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/processing"
)

// VerifyStrategy implements the verify strategy: generate -> check -> refine loop
type VerifyStrategy struct {
	maxIterations int // Maximum number of verify iterations (default: 3)
}

// Execute runs a task using the verify strategy
func (v *VerifyStrategy) Execute(ctx context.Context, task *Task, orch *Orchestrator) (*Result, error) {
	// Get max iterations from config or use default
	maxIterations := v.maxIterations
	if maxIterations == 0 {
		maxIterations = orch.cfg.LLMMaxVerifyIterations
		if maxIterations == 0 {
			maxIterations = 3 // Default: max 3 iterations
		}
	}

	switch task.Type {
	case TaskTypeCategorize:
		return v.executeCategorize(ctx, task, orch, maxIterations)
	default:
		// For other task types, fall back to Simple strategy
		simple := &SimpleStrategy{}
		return simple.Execute(ctx, task, orch)
	}
}

// executeCategorize handles categorization with verification loop
func (v *VerifyStrategy) executeCategorize(ctx context.Context, task *Task, orch *Orchestrator, maxIterations int) (*Result, error) {
	input, ok := task.Input.(*CategorizeInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type for categorize task")
	}

	transactions, tags, rules, entitySearcher, purchaseMatcher := parseCategorizeInput(input)
	hasTools := entitySearcher != nil || purchaseMatcher != nil

	// Get models: generator (fast) and verifier (reasoning or verifier role)
	generatorModel := orch.GetModel(RoleFast)
	verifierModel := orch.GetModel(RoleVerifier)
	if verifierModel == nil {
		verifierModel = orch.GetModel(RoleReasoning)
	}
	if verifierModel == nil {
		verifierModel = generatorModel // Fall back to same model
	}

	if generatorModel == nil {
		return nil, fmt.Errorf("no generator model configured")
	}

	slog.InfoContext(ctx, "verify strategy start",
		"task_type", task.Type,
		"generator", fmt.Sprintf("%s/%s", generatorModel.Provider, generatorModel.Model),
		"verifier", fmt.Sprintf("%s/%s", verifierModel.Provider, verifierModel.Model),
		"max_iterations", maxIterations, "transactions", len(transactions))

	// Build initial prompts
	systemPrompt := processing.BuildSystemPrompt(hasTools)
	userPrompt := processing.BuildUserPrompt(transactions, tags, rules, hasTools, input.MyLifeContext)

	messages := []LLMMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	var currentResults []LLMResult
	var iteration int

	for iteration = 0; iteration < maxIterations; iteration++ {
		slog.DebugContext(ctx, "verify iteration",
			"task_type", task.Type, "iteration", iteration+1, "max", maxIterations,
			"avg_confidence", calculateAverageConfidence(currentResults))

		// Step 1: Generate with generator model
		if iteration == 0 {
			// First iteration: generate initial results
			var resp *LLMResponse
			var err error

			if !hasTools {
				resp, err = orch.callModel(ctx, generatorModel, messages, false, entitySearcher, purchaseMatcher)
			} else {
				// For tool calls, use simple strategy
				simpleTask := &Task{
					Type:     TaskTypeCategorize,
					Strategy: StrategySimple,
					Input:    task.Input,
				}
				simple := &SimpleStrategy{}
				result, err2 := simple.Execute(ctx, simpleTask, orch)
				if err2 != nil {
					return nil, fmt.Errorf("generation failed: %w", err2)
				}
				currentResults = result.Output.([]LLMResult)
				iteration++
				break // Skip verification for tool calls for now
			}

			if err != nil {
				return nil, classifyError(err, generatorModel, task.Strategy, task.Type)
			}

			currentResults, err = orch.parseResponse(resp)
			if err != nil {
				return nil, classifyError(err, generatorModel, task.Strategy, task.Type)
			}
		}

		// Step 2: Verify with verifier model
		if verifierModel == generatorModel && iteration == 0 {
			// Same model, skip verification on first iteration
			break
		}

		verificationPrompt := v.buildVerificationPrompt(transactions, currentResults, tags, rules, input.MyLifeContext)
		verifyMessages := []LLMMessage{
			{Role: "system", Content: v.buildVerifierSystemPrompt()},
			{Role: "user", Content: verificationPrompt},
		}

		resp, err := orch.callModel(ctx, verifierModel, verifyMessages, false, nil, nil)
		if err != nil {
			orchErr := classifyError(err, verifierModel, task.Strategy, task.Type)
			slog.WarnContext(ctx, "verification failed, using current results",
				"error_type", orchErr.Type, "retryable", orchErr.Retryable, "error", err)
			break
		}

		verificationResults, err := orch.parseResponse(resp)
		if err != nil {
			orchErr := classifyError(err, verifierModel, task.Strategy, task.Type)
			slog.WarnContext(ctx, "verification parse failed, using current results",
				"error_type", orchErr.Type, "retryable", orchErr.Retryable, "error", err)
			break
		}

		// Step 3: Check if verification found issues
		issuesFound := false
		for i, verifyResult := range verificationResults {
			if i < len(currentResults) {
				// If verifier suggests a different category or lower confidence, we have an issue
				if verifyResult.Category != currentResults[i].Category || verifyResult.Confidence < currentResults[i].Confidence {
					issuesFound = true
					break
				}
			}
		}

		if !issuesFound {
			slog.DebugContext(ctx, "verification passed")
			break
		}

		// Step 4: Refine - use verifier's suggestions to improve results
		slog.DebugContext(ctx, "verification found issues, refining")
		for i := range currentResults {
			if i < len(verificationResults) {
				// Use verifier's result if it has higher confidence
				if verificationResults[i].Confidence > currentResults[i].Confidence {
					currentResults[i] = verificationResults[i]
				}
			}
		}

		// If this is the last iteration, break
		if iteration == maxIterations-1 {
			break
		}

		// Prepare for next iteration with refined results
		refinementPrompt := v.buildRefinementPrompt(transactions, currentResults, verificationResults, tags, rules, input.MyLifeContext)
		messages = []LLMMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: refinementPrompt},
		}
	}

	return &Result{
		Output:     currentResults,
		Confidence: calculateAverageConfidence(currentResults),
		ModelPath:  []string{string(generatorModel.Role), string(verifierModel.Role)},
		Escalated:  false,
		Iterations: iteration + 1,
		Metadata: map[string]interface{}{
			"max_iterations":  maxIterations,
			"iterations_used": iteration + 1,
		},
	}, nil
}

// buildVerifierSystemPrompt builds the system prompt for the verifier model
func (v *VerifyStrategy) buildVerifierSystemPrompt() string {
	return `You are a verification assistant for financial transaction categorization.

Your job is to review categorization results and identify any errors or low-confidence classifications.

For each transaction, check:
1. Is the category appropriate for the transaction description?
2. Is the confidence level reasonable given the clarity of the transaction?
3. Are there any obvious mistakes or inconsistencies?

Return your verification in the same JSON format as the original results, with your assessment.
If you agree with a categorization, keep it. If you disagree, provide your corrected version.`
}

// buildVerificationPrompt builds the prompt for verification
func (v *VerifyStrategy) buildVerificationPrompt(transactions []processing.TransactionContext, results []LLMResult, tags []*models.Tag, rules []*models.CategorizationRule, myLifeContext string) string {
	// This would build a prompt that includes the transactions and current results
	// For now, use a simple approach
	return processing.BuildUserPrompt(transactions, tags, rules, false, myLifeContext) + "\n\nPlease verify these categorizations are correct."
}

// buildRefinementPrompt builds the prompt for refinement iteration
func (v *VerifyStrategy) buildRefinementPrompt(transactions []processing.TransactionContext, current []LLMResult, verified []LLMResult, tags []*models.Tag, rules []*models.CategorizationRule, myLifeContext string) string {
	// This would build a prompt that includes feedback from verification
	// For now, use a simple approach
	return processing.BuildUserPrompt(transactions, tags, rules, false, myLifeContext) + "\n\nPlease refine these categorizations based on the verification feedback."
}
