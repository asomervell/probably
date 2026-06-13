package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/asomervell/probably/internal/processing"
)

// EscalateStrategy implements the escalate strategy: cheap model first, escalate on low confidence
type EscalateStrategy struct {
	threshold float64 // Confidence threshold for escalation (default: 0.85)
}

// Execute runs a task using the escalate strategy
func (e *EscalateStrategy) Execute(ctx context.Context, task *Task, orch *Orchestrator) (*Result, error) {
	// Get threshold from config or use default
	threshold := e.threshold
	if threshold == 0 {
		threshold = orch.cfg.LLMEscalateThreshold
		if threshold == 0 {
			threshold = 0.85 // Default threshold
		}
	}

	switch task.Type {
	case TaskTypeCategorize:
		return e.executeCategorize(ctx, task, orch, threshold)
	default:
		// For other task types, fall back to Simple strategy
		simple := &SimpleStrategy{}
		return simple.Execute(ctx, task, orch)
	}
}

// executeCategorize handles categorization with escalation
func (e *EscalateStrategy) executeCategorize(ctx context.Context, task *Task, orch *Orchestrator, threshold float64) (*Result, error) {
	input, ok := task.Input.(*CategorizeInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type for categorize task")
	}

	transactions, tags, rules, entitySearcher, purchaseMatcher := parseCategorizeInput(input)
	hasTools := entitySearcher != nil || purchaseMatcher != nil

	// Step 1: Try fast model first
	fastModel := orch.GetModel(RoleFast)
	if fastModel == nil {
		return nil, fmt.Errorf("no fast model configured")
	}
	slog.DebugContext(ctx, "escalate step 1: processing with fast model",
		"task_type", task.Type,
		"transactions", len(transactions),
		"fast_model", string(fastModel.Provider)+"/"+fastModel.Model,
		"threshold", threshold)

	// Build prompts
	systemPrompt := processing.BuildSystemPrompt(hasTools)
	userPrompt := processing.BuildUserPrompt(transactions, tags, rules, hasTools, input.MyLifeContext)

	messages := []LLMMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Call fast model (without tools for now - can add tool support later)
	var fastResults []LLMResult

	if !hasTools {
		resp, err := orch.callModel(ctx, fastModel, messages, false, entitySearcher, purchaseMatcher)
		if err != nil {
			return nil, classifyError(err, fastModel, task.Strategy, task.Type)
		}
		fastResults, err = orch.parseResponse(resp)
		if err != nil {
			return nil, classifyError(err, fastModel, task.Strategy, task.Type)
		}
	} else {
		// For tool calling, we'd need to implement the full tool loop
		// For now, fall back to simple strategy for tool calls
		simple := &SimpleStrategy{}
		result, err := simple.Execute(ctx, task, orch)
		if err != nil {
			return nil, err
		}
		fastResults = result.Output.([]LLMResult)
	}

	// Step 2: Check confidence and escalate if needed
	lowConfidenceResults := make([]processing.TransactionContext, 0)
	lowConfidenceIndices := make([]int, 0)

	for i, result := range fastResults {
		if result.Confidence < threshold {
			// Find the corresponding transaction
			if i < len(transactions) {
				lowConfidenceResults = append(lowConfidenceResults, transactions[i])
				lowConfidenceIndices = append(lowConfidenceIndices, i)
			}
		}
	}

	// If all results are high confidence, return fast model results
	if len(lowConfidenceResults) == 0 {
		slog.DebugContext(ctx, "all results above threshold, using fast model results",
			"task_type", task.Type, "threshold", threshold, "avg_confidence", calculateAverageConfidence(fastResults))
		return &Result{
			Output:     fastResults,
			Confidence: calculateAverageConfidence(fastResults),
			ModelPath:  []string{string(RoleFast)},
			Escalated:  false,
			Iterations: 1,
		}, nil
	}

	// Step 3: Escalate low-confidence results to reasoning model
	reasoningModel := orch.GetModel(RoleReasoning)
	if reasoningModel == nil {
		slog.WarnContext(ctx, "no reasoning model configured, using fast model results for low-confidence items", "task_type", task.Type)
		return &Result{
			Output:     fastResults,
			Confidence: calculateAverageConfidence(fastResults),
			ModelPath:  []string{string(RoleFast)},
			Escalated:  false,
			Iterations: 1,
		}, nil
	}
	slog.DebugContext(ctx, "escalate step 2: escalating low-confidence results",
		"task_type", task.Type,
		"escalating", len(lowConfidenceResults),
		"threshold", threshold,
		"reasoning_model", string(reasoningModel.Provider)+"/"+reasoningModel.Model)

	// Build prompts for escalated transactions
	escalatedUserPrompt := processing.BuildUserPrompt(lowConfidenceResults, tags, rules, hasTools, input.MyLifeContext)
	escalatedMessages := []LLMMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: escalatedUserPrompt},
	}

	var escalatedResults []LLMResult
	if !hasTools {
		resp, err := orch.callModel(ctx, reasoningModel, escalatedMessages, false, entitySearcher, purchaseMatcher)
		if err != nil {
			// Classify error for logging
			orchErr := classifyError(err, reasoningModel, task.Strategy, task.Type)
			slog.WarnContext(ctx, "reasoning model call failed, using fast model results",
				"retryable", orchErr == nil || orchErr.Retryable,
				"err", err)
			return &Result{
				Output:     fastResults,
				Confidence: calculateAverageConfidence(fastResults),
				ModelPath:  []string{string(RoleFast)},
				Escalated:  false,
				Iterations: 1,
				Metadata: map[string]interface{}{
					"escalation_failed": true,
					"escalation_error":  err.Error(),
				},
			}, nil
		}
		escalatedResults, err = orch.parseResponse(resp)
		if err != nil {
			// Classify parse error
			orchErr := classifyError(err, reasoningModel, task.Strategy, task.Type)
			slog.WarnContext(ctx, "failed to parse reasoning model response, using fast model results",
				"retryable", orchErr == nil || orchErr.Retryable,
				"err", err)
			return &Result{
				Output:     fastResults,
				Confidence: calculateAverageConfidence(fastResults),
				ModelPath:  []string{string(RoleFast)},
				Escalated:  false,
				Iterations: 1,
				Metadata: map[string]interface{}{
					"escalation_parse_failed": true,
					"parse_error":             err.Error(),
				},
			}, nil
		}
	} else {
		// For tool calls, use simple strategy
		escalatedTask := &Task{
			Type:     TaskTypeCategorize,
			Strategy: StrategySimple,
			Input:    task.Input,
		}
		simple := &SimpleStrategy{}
		result, err := simple.Execute(ctx, escalatedTask, orch)
		if err != nil {
			return nil, err
		}
		escalatedResults = result.Output.([]LLMResult)
	}

	// Step 4: Merge results (replace low-confidence fast results with escalated results)
	finalResults := make([]LLMResult, len(fastResults))
	copy(finalResults, fastResults)

	for i, idx := range lowConfidenceIndices {
		if i < len(escalatedResults) && idx < len(finalResults) {
			finalResults[idx] = escalatedResults[i]
			slog.DebugContext(ctx, "escalated result replaced",
				"idx", idx, "from_confidence", fastResults[idx].Confidence, "to_confidence", escalatedResults[i].Confidence)
		}
	}

	slog.DebugContext(ctx, "escalation completed",
		"task_type", task.Type,
		"fast_results", len(fastResults)-len(lowConfidenceResults),
		"escalated", len(lowConfidenceResults),
		"avg_confidence", calculateAverageConfidence(finalResults))

	return &Result{
		Output:     finalResults,
		Confidence: calculateAverageConfidence(finalResults),
		ModelPath:  []string{string(RoleFast), string(RoleReasoning)},
		Escalated:  true,
		Iterations: 2,
		Metadata: map[string]interface{}{
			"escalated_count": len(lowConfidenceResults),
			"threshold":       threshold,
		},
	}, nil
}

// calculateAverageConfidence calculates the average confidence across results
func calculateAverageConfidence(results []LLMResult) float64 {
	if len(results) == 0 {
		return 0.0
	}
	sum := 0.0
	for _, r := range results {
		sum += r.Confidence
	}
	return sum / float64(len(results))
}
