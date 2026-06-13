package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/processing"
)

// ConsensusStrategy implements the consensus strategy: parallel model calls with voting
type ConsensusStrategy struct {
	required int // Number of models required for consensus (default: 2)
}

// Execute runs a task using the consensus strategy
func (c *ConsensusStrategy) Execute(ctx context.Context, task *Task, orch *Orchestrator) (*Result, error) {
	// Get required count from config or use default
	required := c.required
	if required == 0 {
		required = orch.cfg.LLMConsensusRequired
		if required == 0 {
			required = 2 // Default: need 2 models to agree
		}
	}

	switch task.Type {
	case TaskTypeCategorize:
		return c.executeCategorize(ctx, task, orch, required)
	default:
		// For other task types, fall back to Simple strategy
		simple := &SimpleStrategy{}
		return simple.Execute(ctx, task, orch)
	}
}

// executeCategorize handles categorization with consensus voting
func (c *ConsensusStrategy) executeCategorize(ctx context.Context, task *Task, orch *Orchestrator, required int) (*Result, error) {
	input, ok := task.Input.(*CategorizeInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type for categorize task")
	}

	transactions, tags, rules, entitySearcher, purchaseMatcher := parseCategorizeInput(input)
	hasTools := entitySearcher != nil || purchaseMatcher != nil

	// Select models to use for consensus
	// We'll use fast model and reasoning model if both are available
	modelsToUse := make([]*ModelSpec, 0)
	fastModel := orch.GetModel(RoleFast)
	reasoningModel := orch.GetModel(RoleReasoning)

	if fastModel != nil {
		modelsToUse = append(modelsToUse, fastModel)
	}
	if reasoningModel != nil && reasoningModel != fastModel {
		modelsToUse = append(modelsToUse, reasoningModel)
	}

	if len(modelsToUse) < required {
		slog.WarnContext(ctx, "consensus: not enough models, falling back to simple",
			"task_type", task.Type, "available", len(modelsToUse), "required", required)
		simple := &SimpleStrategy{}
		return simple.Execute(ctx, task, orch)
	}

	modelNames := make([]string, len(modelsToUse))
	for i, m := range modelsToUse {
		modelNames[i] = fmt.Sprintf("%s/%s", m.Provider, m.Model)
	}
	slog.InfoContext(ctx, "consensus start",
		"task_type", task.Type, "transactions", len(transactions),
		"models", len(modelsToUse), "required", required, "model_names", modelNames)

	// Build prompts
	systemPrompt := processing.BuildSystemPrompt(hasTools)
	userPrompt := processing.BuildUserPrompt(transactions, tags, rules, hasTools, input.MyLifeContext)

	messages := []LLMMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Call all models in parallel
	type modelResult struct {
		model   *ModelSpec
		results []LLMResult
		err     error
	}

	resultsChan := make(chan modelResult, len(modelsToUse))
	var wg sync.WaitGroup

	for _, model := range modelsToUse {
		wg.Add(1)
		go func(m *ModelSpec) {
			defer wg.Done()
			defer observability.RecoverAndLog(ctx, "consensus_model_call")
			slog.DebugContext(ctx, "consensus calling model",
				"task_type", task.Type, "model", fmt.Sprintf("%s/%s", m.Provider, m.Model), "required", required)

			var resp *LLMResponse
			var err error

			if !hasTools {
				resp, err = orch.callModel(ctx, m, messages, false, entitySearcher, purchaseMatcher)
			} else {
				// For tool calls, use simple strategy per model
				simpleTask := &Task{
					Type:     TaskTypeCategorize,
					Strategy: StrategySimple,
					Input:    task.Input,
				}
				simple := &SimpleStrategy{}
				result, err2 := simple.Execute(ctx, simpleTask, orch)
				if err2 != nil {
					resultsChan <- modelResult{model: m, err: err2}
					return
				}
				resultsChan <- modelResult{
					model:   m,
					results: result.Output.([]LLMResult),
				}
				return
			}

			if err != nil {
				orchErr := classifyError(err, m, task.Strategy, task.Type)
				slog.WarnContext(ctx, "consensus model failed",
					"model", fmt.Sprintf("%s/%s", m.Provider, m.Model),
					"error_type", orchErr.Type, "retryable", orchErr.Retryable, "error", err)
				resultsChan <- modelResult{model: m, err: err}
				return
			}

			results, err := orch.parseResponse(resp)
			if err != nil {
				orchErr := classifyError(err, m, task.Strategy, task.Type)
				slog.WarnContext(ctx, "consensus model parse failed",
					"model", fmt.Sprintf("%s/%s", m.Provider, m.Model),
					"error_type", orchErr.Type, "retryable", orchErr.Retryable, "error", err)
				resultsChan <- modelResult{model: m, err: err}
				return
			}

			resultsChan <- modelResult{model: m, results: results}
		}(model)
	}

	wg.Wait()
	close(resultsChan)

	// Collect results
	allResults := make([]modelResult, 0)
	for result := range resultsChan {
		if result.err != nil {
			continue
		}
		allResults = append(allResults, result)
	}

	if len(allResults) < required {
		// Graceful degradation: if we have at least one result, use it with lower confidence
		if len(allResults) > 0 {
			slog.WarnContext(ctx, "partial consensus: using best available result",
				"succeeded", len(allResults), "required", required)
			// Use the result from the first successful model
			bestResult := allResults[0]
			results := bestResult.results
			// Reduce confidence to indicate partial consensus
			for i := range results {
				results[i].Confidence = results[i].Confidence * 0.8 // Reduce by 20%
			}
			return &Result{
				Output:     results,
				Confidence: calculateAverageConfidence(results),
				ModelPath:  []string{fmt.Sprintf("%s/%s", bestResult.model.Provider, bestResult.model.Model)},
				Escalated:  false,
				Iterations: 1,
				Metadata: map[string]interface{}{
					"consensus_partial": true,
					"models_succeeded":  len(allResults),
					"models_required":   required,
				},
			}, nil
		}
		// No models succeeded - return error
		return nil, fmt.Errorf("consensus failed: only %d models succeeded, need %d (task: %s, strategy: %s)", len(allResults), required, task.Type, task.Strategy)
	}

	slog.InfoContext(ctx, "computing consensus",
		"task_type", task.Type, "succeeded", len(allResults), "required", required)

	// Compute consensus for each transaction
	finalResults := make([]LLMResult, len(transactions))

	for i := range transactions {
		// Collect votes for this transaction
		votes := make(map[string]int)                 // category -> vote count
		categoryResults := make(map[string]LLMResult) // category -> best result

		for _, modelResult := range allResults {
			if i < len(modelResult.results) {
				result := modelResult.results[i]
				category := result.Category
				votes[category]++
				// Keep the highest confidence result for each category
				if existing, ok := categoryResults[category]; !ok || result.Confidence > existing.Confidence {
					categoryResults[category] = result
				}
			}
		}

		// Find category with most votes
		maxVotes := 0
		winningCategory := ""
		for category, voteCount := range votes {
			if voteCount > maxVotes {
				maxVotes = voteCount
				winningCategory = category
			}
		}

		// Use the winning category's result
		if winningCategory != "" && maxVotes >= required {
			finalResults[i] = categoryResults[winningCategory]
			// Update confidence based on consensus strength
			consensusStrength := float64(maxVotes) / float64(len(allResults))
			finalResults[i].Confidence = finalResults[i].Confidence * consensusStrength
		} else {
			// No consensus - use the first model's result
			if len(allResults) > 0 && i < len(allResults[0].results) {
				finalResults[i] = allResults[0].results[i]
				finalResults[i].Confidence *= 0.5 // Lower confidence for no consensus
			}
		}
	}

	resultModelPath := make([]string, len(allResults))
	for i, r := range allResults {
		resultModelPath[i] = fmt.Sprintf("%s/%s", r.model.Provider, r.model.Model)
	}

	return &Result{
		Output:     finalResults,
		Confidence: calculateAverageConfidence(finalResults),
		ModelPath:  resultModelPath,
		Escalated:  false,
		Iterations: 1,
		Metadata: map[string]interface{}{
			"models_used":     len(allResults),
			"consensus_count": required,
		},
	}, nil
}
