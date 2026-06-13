package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// SupervisorStrategy implements the supervisor strategy: planner + worker pattern
type SupervisorStrategy struct{}

// Execute runs a task using the supervisor strategy
func (s *SupervisorStrategy) Execute(ctx context.Context, task *Task, orch *Orchestrator) (*Result, error) {
	switch task.Type {
	case TaskTypeChat, TaskTypeChatSQL:
		return s.executeChat(ctx, task, orch)
	default:
		// For other task types, fall back to Simple strategy
		simple := &SimpleStrategy{}
		return simple.Execute(ctx, task, orch)
	}
}

// executeChat handles chat tasks with supervisor pattern
func (s *SupervisorStrategy) executeChat(ctx context.Context, task *Task, orch *Orchestrator) (*Result, error) {
	input, ok := task.Input.(*ChatInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type for chat task")
	}

	// Get planner and worker models
	plannerModel := orch.GetModel(RolePlanner)
	if plannerModel == nil {
		plannerModel = orch.GetModel(RoleReasoning)
	}
	if plannerModel == nil {
		plannerModel = orch.GetModel(RoleFast)
	}

	workerModel := orch.GetModel(RoleFast)
	if workerModel == nil {
		workerModel = plannerModel
	}

	slog.DebugContext(ctx, "supervisor model selection",
		"task_type", task.Type,
		"planner", string(plannerModel.Provider)+"/"+plannerModel.Model,
		"worker", string(workerModel.Provider)+"/"+workerModel.Model)

	// Step 1: Planner breaks down the query into subtasks
	plannerPrompt := s.buildPlannerPrompt(input)
	plannerMessages := []LLMMessage{
		{Role: "system", Content: s.buildPlannerSystemPrompt()},
		{Role: "user", Content: plannerPrompt},
	}

	plannerResp, err := orch.callModel(ctx, plannerModel, plannerMessages, false, nil, nil)
	if err != nil {
		// Classify error for logging
		orchErr := classifyError(err, plannerModel, task.Strategy, task.Type)
		slog.WarnContext(ctx, "planner failed, falling back to simple",
			"retryable", orchErr == nil || orchErr.Retryable,
			"err", err)
		simple := &SimpleStrategy{}
		return simple.Execute(ctx, task, orch)
	}

	if len(plannerResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from planner")
	}

	plannerContent := plannerResp.Choices[0].Message.Content
	if plannerContent == "" {
		slog.WarnContext(ctx, "empty planner response, falling back to simple")
		simple := &SimpleStrategy{}
		return simple.Execute(ctx, task, orch)
	}

	// Parse planner response to get subtasks
	subtasks, err := s.parsePlannerResponse(plannerContent)
	if err != nil {
		slog.WarnContext(ctx, "failed to parse planner response, falling back to simple", "err", err)
		simple := &SimpleStrategy{}
		return simple.Execute(ctx, task, orch)
	}

	// If planner says it's simple, use Simple strategy
	if len(subtasks) <= 1 {
		slog.DebugContext(ctx, "planner determined simple query, using simple strategy")
		simple := &SimpleStrategy{}
		return simple.Execute(ctx, task, orch)
	}

	slog.DebugContext(ctx, "planner created subtasks", "task_type", task.Type, "count", len(subtasks))

	// Step 2: Execute subtasks (for now, execute sequentially)
	// In the future, could execute in parallel
	subtaskResults := make([]map[string]interface{}, 0, len(subtasks))
	for i, subtask := range subtasks {
		slog.DebugContext(ctx, "executing subtask", "task_type", task.Type, "subtask", i+1, "total", len(subtasks), "description", subtask.Description)

		workerPrompt := s.buildWorkerPrompt(subtask)
		workerMessages := []LLMMessage{
			{Role: "system", Content: s.buildWorkerSystemPrompt()},
			{Role: "user", Content: workerPrompt},
		}

		workerResp, err := orch.callModel(ctx, workerModel, workerMessages, false, nil, nil)
		if err != nil {
			// Classify error for logging
			orchErr := classifyError(err, workerModel, task.Strategy, task.Type)
			slog.WarnContext(ctx, "worker failed for subtask",
				"subtask", i+1,
				"retryable", orchErr == nil || orchErr.Retryable,
				"err", err)
			continue
		}

		if len(workerResp.Choices) > 0 && workerResp.Choices[0].Message.Content != "" {
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(workerResp.Choices[0].Message.Content), &result); err == nil {
				subtaskResults = append(subtaskResults, result)
			}
		}
	}

	// Step 3: Synthesize results
	synthesisPrompt := s.buildSynthesisPrompt(input, subtasks, subtaskResults)
	synthesisMessages := []LLMMessage{
		{Role: "system", Content: s.buildSynthesisSystemPrompt()},
		{Role: "user", Content: synthesisPrompt},
	}

	synthesisResp, err := orch.callModel(ctx, plannerModel, synthesisMessages, false, nil, nil)
	if err != nil {
		// Classify error for logging
		orchErr := classifyError(err, plannerModel, task.Strategy, task.Type)
		slog.WarnContext(ctx, "synthesis failed",
			"retryable", orchErr == nil || orchErr.Retryable,
			"err", err)
		// Return first subtask result as fallback
		if len(subtaskResults) > 0 {
			return &Result{
				Output:     subtaskResults[0],
				Confidence: 0.8,
				ModelPath:  []string{string(plannerModel.Role), string(workerModel.Role)},
				Escalated:  false,
				Iterations: len(subtasks) + 1,
			}, nil
		}
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}

	if len(synthesisResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from synthesis")
	}

	content := synthesisResp.Choices[0].Message.Content
	var finalResult map[string]interface{}
	if err := json.Unmarshal([]byte(content), &finalResult); err != nil {
		if err := json.Unmarshal([]byte(extractJSONFromMarkdown(content)), &finalResult); err != nil {
			return nil, fmt.Errorf("failed to parse synthesis response: %w", err)
		}
	}

	return &Result{
		Output:     finalResult,
		Confidence: 0.9, // Supervisor strategy has higher confidence
		ModelPath:  []string{string(plannerModel.Role), string(workerModel.Role)},
		Escalated:  false,
		Iterations: len(subtasks) + 2, // Planner + subtasks + synthesis
		Metadata: map[string]interface{}{
			"subtasks_count": len(subtasks),
		},
	}, nil
}

// Subtask represents a subtask from the planner
type Subtask struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Type        string `json:"type"` // "sql", "analysis", etc.
}

// parsePlannerResponse parses the planner's response to extract subtasks
func (s *SupervisorStrategy) parsePlannerResponse(content string) ([]Subtask, error) {
	var response struct {
		Subtasks []Subtask `json:"subtasks"`
		IsSimple bool      `json:"is_simple"`
	}

	if err := json.Unmarshal([]byte(content), &response); err != nil {
		// Try to extract JSON from markdown
		cleaned := strings.TrimSpace(content)
		if strings.Contains(cleaned, "```") {
			start := strings.Index(cleaned, "```json")
			if start == -1 {
				start = strings.Index(cleaned, "```")
			}
			if start != -1 {
				newlinePos := strings.Index(cleaned[start:], "\n")
				if newlinePos != -1 {
					jsonStart := start + newlinePos + 1
					end := strings.Index(cleaned[jsonStart:], "```")
					if end != -1 {
						cleaned = strings.TrimSpace(cleaned[jsonStart : jsonStart+end])
					} else {
						cleaned = strings.TrimSpace(cleaned[jsonStart:])
					}
				}
			}
		}
		if err := json.Unmarshal([]byte(cleaned), &response); err != nil {
			return nil, fmt.Errorf("failed to parse planner response: %w", err)
		}
	}

	if response.IsSimple {
		return []Subtask{}, nil
	}

	return response.Subtasks, nil
}

// buildPlannerSystemPrompt builds the system prompt for the planner
func (s *SupervisorStrategy) buildPlannerSystemPrompt() string {
	return `You are a query planner for a financial assistant.

Your job is to analyze user questions and break them down into simpler subtasks if needed.

For simple questions (e.g., "How much did I spend this month?"), return:
{
  "is_simple": true,
  "subtasks": []
}

For complex questions (e.g., "Compare my spending trends across categories this year vs last year"), break them down:
{
  "is_simple": false,
  "subtasks": [
    {"id": 1, "description": "Get spending by category for this year", "type": "sql"},
    {"id": 2, "description": "Get spending by category for last year", "type": "sql"},
    {"id": 3, "description": "Compare and format results", "type": "analysis"}
  ]
}

Return ONLY valid JSON.`
}

// buildPlannerPrompt builds the prompt for the planner
func (s *SupervisorStrategy) buildPlannerPrompt(input *ChatInput) string {
	// Extract the user's question from messages
	question := ""
	for _, msg := range input.Messages {
		if m, ok := msg.(map[string]interface{}); ok {
			if role, _ := m["role"].(string); role == "user" {
				if content, _ := m["content"].(string); content != "" {
					question = content
					break
				}
			}
		}
	}

	return fmt.Sprintf("Analyze this user question and determine if it needs to be broken down into subtasks:\n\n%s\n\nReturn your analysis as JSON.", question)
}

// buildWorkerSystemPrompt builds the system prompt for workers
func (s *SupervisorStrategy) buildWorkerSystemPrompt() string {
	return `You are a SQL query generator for a financial assistant.

Generate a PostgreSQL SQL query to answer the given subtask.

Rules:
- Always include "ledger_id = $1" in WHERE clause
- Use amount_cents / 100.0 for dollars
- Exclude transfers (is_transfer = false) for spending queries
- Return JSON: {"sql": "...", "answer_template": "..."}`
}

// buildWorkerPrompt builds the prompt for a worker
func (s *SupervisorStrategy) buildWorkerPrompt(subtask Subtask) string {
	return fmt.Sprintf("Generate SQL for this subtask: %s", subtask.Description)
}

// buildSynthesisSystemPrompt builds the system prompt for synthesis
func (s *SupervisorStrategy) buildSynthesisSystemPrompt() string {
	return `You are a result synthesizer for a financial assistant.

Combine the results from multiple subtasks into a single, coherent answer.

Return JSON:
{
  "sql": "combined SQL if applicable",
  "answer_template": "natural language answer template",
  "thought": "your reasoning"
}`
}

// buildSynthesisPrompt builds the prompt for synthesis
func (s *SupervisorStrategy) buildSynthesisPrompt(input *ChatInput, subtasks []Subtask, results []map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("Original question:\n")
	// Extract question from input
	for _, msg := range input.Messages {
		if m, ok := msg.(map[string]interface{}); ok {
			if role, _ := m["role"].(string); role == "user" {
				if content, _ := m["content"].(string); content != "" {
					sb.WriteString(content)
					break
				}
			}
		}
	}
	sb.WriteString("\n\nSubtasks executed:\n")
	for i, subtask := range subtasks {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, subtask.Description))
	}
	sb.WriteString("\nResults:\n")
	for i, result := range results {
		resultJSON, _ := json.Marshal(result)
		sb.WriteString(fmt.Sprintf("Subtask %d: %s\n", i+1, string(resultJSON)))
	}
	sb.WriteString("\nSynthesize these results into a final answer.")
	return sb.String()
}
