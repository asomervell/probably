package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/processing"
)

// SimpleStrategy implements the simple single-model execution strategy
// This ports the existing LLMRouter.Process logic
type SimpleStrategy struct{}

// Execute runs a task using a single model call
func (s *SimpleStrategy) Execute(ctx context.Context, task *Task, orch *Orchestrator) (*Result, error) {
	switch task.Type {
	case TaskTypeCategorize:
		return s.executeCategorize(ctx, task, orch)
	case TaskTypeCategorizeP2P:
		return s.executeCategorizeP2P(ctx, task, orch)
	case TaskTypeChat, TaskTypeChatSQL:
		return s.executeChat(ctx, task, orch)
	case TaskTypeChatTools:
		return s.executeChatTools(ctx, task, orch)
	default:
		return nil, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// executeCategorize handles categorization tasks
func (s *SimpleStrategy) executeCategorize(ctx context.Context, task *Task, orch *Orchestrator) (*Result, error) {
	// Start pipeline span for categorization
	span := observability.StartPipeline(ctx, observability.PipelineOptions{
		Name:        "categorize_transactions",
		Description: "Categorize transactions using LLM",
	})
	defer span.End(nil)
	ctx = span.Context()

	input, ok := task.Input.(*CategorizeInput)
	if !ok {
		err := fmt.Errorf("invalid input type for categorize task")
		span.SetError(err)
		return nil, err
	}
	span.SetData("transaction_count", len(input.Transactions))

	transactions, tags, rules, entitySearcher, purchaseMatcher := parseCategorizeInput(input)

	// Select model based on task characteristics
	model := orch.GetModel(RoleFast)
	hasTools := entitySearcher != nil || purchaseMatcher != nil
	if hasTools {
		model = orch.GetModel(RoleToolCall)
	} else if len(transactions) > 10 {
		model = orch.GetModel(RoleReasoning)
	}

	// Gemini thinking models on the OpenAI-compat (non-Vertex) path return function calls
	// even when no tool definitions are sent, because the prompt instructs tool use.
	// Those orphaned tool calls corrupt the message history on the next request, triggering
	// "thought_signature missing" and "function_response.name empty" 400 errors.
	// Disable tool calls for that path and build non-tool prompts so the model produces
	// a plain JSON completion instead.
	allowToolCalls := hasTools
	if allowToolCalls && model != nil && model.Provider == ProviderGoogle && orch != nil && !orch.useVertex {
		allowToolCalls = false
	}

	// Only advertise tool use in prompts when we can actually execute tool calls.
	promptHasTools := allowToolCalls
	systemPrompt := processing.BuildSystemPrompt(promptHasTools)
	userPrompt := processing.BuildUserPrompt(transactions, tags, rules, promptHasTools, input.MyLifeContext)

	messages := []LLMMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// If not using tools, make a single call
	if !hasTools {
		resp, err := orch.callModel(ctx, model, messages, false, entitySearcher, purchaseMatcher)
		if err != nil {
			return nil, classifyError(err, model, task.Strategy, task.Type)
		}
		results, err := orch.parseResponse(resp)
		if err != nil {
			return nil, classifyError(err, model, task.Strategy, task.Type)
		}
		return &Result{
			Output:     results,
			Confidence: 1.0,
			ModelPath:  []string{string(model.Role)},
			Escalated:  false,
			Iterations: 1,
			Thoughts:   resp.Thoughts,
		}, nil
	}

	// Tool calling loop - collect thoughts from all iterations
	const maxIterations = 5
	var allThoughts []string

	for i := 0; i < maxIterations; i++ {
		resp, err := orch.callModel(ctx, model, messages, allowToolCalls && i < maxIterations-1, entitySearcher, purchaseMatcher)
		if err != nil {
			return nil, classifyError(err, model, task.Strategy, task.Type)
		}

		allThoughts = append(allThoughts, resp.Thoughts...)

		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("no response from LLM")
		}

		choice := resp.Choices[0]
		// Treat as final when: no tool calls, stop signal, or tool calls are disabled.
		// The !allowToolCalls case handles thinking models that return function calls
		// despite not having tool definitions — we take whatever content they produced.
		if len(choice.Message.ToolCalls) == 0 || choice.FinishReason == "stop" || !allowToolCalls {
			results, err := orch.parseResponse(resp)
			if err != nil {
				return nil, classifyError(err, model, task.Strategy, task.Type)
			}
			return &Result{
				Output:     results,
				Confidence: 1.0,
				ModelPath:  []string{string(model.Role)},
				Escalated:  false,
				Iterations: i + 1,
				Thoughts:   allThoughts,
			}, nil
		}

		// Execute tool calls (only reached when allowToolCalls == true)
		messages = append(messages, choice.Message)
		for _, toolCall := range choice.Message.ToolCalls {
			toolName := resolveCategorizeToolName(toolCall)
			if toolName == "" {
				slog.WarnContext(ctx, "skipping tool call with unresolved name", "id", toolCall.ID, "args", toolCall.Function.Arguments)
				continue
			}

			var result string
			var err error

			switch toolName {
			case "search_entities":
				result, err = s.executeSearchEntities(ctx, toolCall, entitySearcher)
			case "find_matching_purchase":
				result, err = s.executeFindMatchingPurchase(ctx, toolCall, purchaseMatcher)
			default:
				err = fmt.Errorf("unknown tool: %s", toolName)
			}

			if err != nil {
				slog.WarnContext(ctx, "tool call error", "tool", toolName, "err", err)
				result = fmt.Sprintf(`{"error": %q}`, err.Error())
			}

			// Ensure tool response payload always carries the resolved function name.
			// Vertex/Gemini function_response messages reject empty names.
			result = annotateToolResponse(result, toolName)
			responseToolCallID := strings.TrimSpace(toolCall.ID)
			if responseToolCallID == "" {
				responseToolCallID = toolName
			}

			messages = append(messages, LLMMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: responseToolCallID,
			})
		}
	}

	return nil, fmt.Errorf("max tool call iterations reached without final response")
}

// executeSearchEntities executes the search_entities tool call
func (s *SimpleStrategy) executeSearchEntities(ctx context.Context, toolCall ToolCall, searcher EntitySearcher) (string, error) {
	// Start tool execution span
	toolSpan := observability.StartTool(ctx, observability.ToolOptions{
		Name:        "search_entities",
		Description: "Search for existing entities in the database",
		Input:       toolCall.Function.Arguments,
		Type:        "function",
	})
	defer func() {
		toolSpan.End(nil)
	}()

	if searcher == nil {
		result := `{"entities": [], "message": "Entity search not available"}`
		toolSpan.SetToolOutput(result)
		return result, nil
	}

	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Search entities tool call

	entities, err := searcher.SearchByBM25(ctx, args.Query, 10)
	if err != nil {
		return "", err
	}

	var results []map[string]string
	for _, e := range entities {
		entry := map[string]string{"name": e.Name}
		if e.Website != "" {
			entry["website"] = e.Website
		}
		if e.Description != "" {
			entry["description"] = e.Description
		}
		results = append(results, entry)
	}

	if len(results) == 0 {
		return `{"entities": [], "message": "No existing entities found matching this query. You may create a new name."}`, nil
	}

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"entities": results,
		"message":  fmt.Sprintf("Found %d existing entities. Use one of these exact names if applicable.", len(results)),
	})
	return string(resultJSON), nil
}

// executeFindMatchingPurchase executes the find_matching_purchase tool call
func (s *SimpleStrategy) executeFindMatchingPurchase(ctx context.Context, toolCall ToolCall, matcher PurchaseMatcher) (string, error) {
	// Start tool execution span
	toolSpan := observability.StartTool(ctx, observability.ToolOptions{
		Name:        "find_matching_purchase",
		Description: "Find a previous purchase that a credit might be offsetting",
		Input:       toolCall.Function.Arguments,
		Type:        "function",
	})
	defer func() {
		toolSpan.End(nil)
	}()

	if matcher == nil {
		result := `{"error": "purchase matching not available"}`
		toolSpan.SetToolOutput(result)
		return result, nil
	}

	var args struct {
		AmountCents int64  `json:"amount_cents"`
		AccountName string `json:"account_name"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		toolSpan.SetError(err)
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Find matching purchase tool call
	toolSpan.SetData("amount_cents", args.AmountCents)
	toolSpan.SetData("account_name", args.AccountName)

	purchase, err := matcher.FindMatchingPurchase(ctx, args.AmountCents, args.AccountName)
	if err != nil || purchase == nil {
		return `{"found": false, "message": "No matching purchase found. Categorize this credit based on the description."}`, nil
	}

	// Found matching purchase

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"found":       true,
		"purchase_id": purchase.ID.String(),
		"description": purchase.Description,
		"date":        purchase.Date.Format("2006-01-02"),
		"category":    purchase.Category,
		"message":     fmt.Sprintf("Found matching purchase. Use category '%s' for this credit to offset the original expense.", purchase.Category),
	})
	return string(resultJSON), nil
}

func resolveCategorizeToolName(toolCall ToolCall) string {
	toolName := strings.TrimSpace(toolCall.Function.Name)
	if toolName != "" {
		return toolName
	}

	toolName = strings.TrimSpace(toolCall.ID)
	if toolName != "" {
		return toolName
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return ""
	}
	return inferToolResponseNameFromPayload(args)
}

func annotateToolResponse(content string, toolName string) string {
	if toolName == "" {
		return content
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		// Keep a structured fallback so downstream parsers can recover the function name.
		fallback := map[string]interface{}{
			"function_name": toolName,
			"name":          toolName,
			"result":        content,
		}
		if out, marshalErr := json.Marshal(fallback); marshalErr == nil {
			return string(out)
		}
		return content
	}

	payload["function_name"] = toolName
	payload["name"] = toolName

	out, err := json.Marshal(payload)
	if err != nil {
		return content
	}
	return string(out)
}

// executeCategorizeP2P handles P2P categorization tasks
func (s *SimpleStrategy) executeCategorizeP2P(ctx context.Context, task *Task, orch *Orchestrator) (*Result, error) {
	// Start pipeline span for P2P categorization
	span := observability.StartPipeline(ctx, observability.PipelineOptions{
		Name:        "categorize_p2p_transactions",
		Description: "Categorize P2P transactions using LLM",
	})
	defer span.End(nil)
	ctx = span.Context()

	input, ok := task.Input.(*CategorizeInput)
	if !ok {
		err := fmt.Errorf("invalid input type for categorize P2P task")
		span.SetError(err)
		return nil, err
	}
	span.SetData("transaction_count", len(input.Transactions))

	// Convert input to the types expected by processing package
	p2pTransactions := make([]processing.P2PTransactionContext, 0, len(input.Transactions))
	for _, t := range input.Transactions {
		if p2pTxn, ok := t.(processing.P2PTransactionContext); ok {
			p2pTransactions = append(p2pTransactions, p2pTxn)
		}
	}

	tags := make([]*models.Tag, 0, len(input.Tags))
	for _, t := range input.Tags {
		if tag, ok := t.(*models.Tag); ok {
			tags = append(tags, tag)
		}
	}

	// Extract household patterns from task metadata
	var householdPatterns []string
	if task.Context != nil && task.Context.Metadata != nil {
		if patternStr, ok := task.Context.Metadata["household_patterns"]; ok && patternStr != "" {
			// Split comma-separated patterns
			householdPatterns = strings.Split(patternStr, ",")
			// Trim whitespace from each pattern
			for i, pattern := range householdPatterns {
				householdPatterns[i] = strings.TrimSpace(pattern)
			}
		}
	}

	// Select model - P2P is simpler, always use fast model
	model := orch.GetModel(RoleFast)
	if model == nil {
		return nil, fmt.Errorf("no fast model configured")
	}

	// Processing P2P transactions

	// Build prompts using processing package functions
	systemPrompt := processing.BuildP2PSystemPrompt()
	userPrompt := processing.BuildP2PUserPrompt(p2pTransactions, tags, householdPatterns, input.MyLifeContext)

	messages := []LLMMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// P2P doesn't use tools - make a single call
	resp, err := orch.callModel(ctx, model, messages, false, nil, nil)
	if err != nil {
		return nil, classifyError(err, model, task.Strategy, task.Type)
	}

	results, err := orch.parseResponse(resp)
	if err != nil {
		return nil, classifyError(err, model, task.Strategy, task.Type)
	}

	return &Result{
		Output:     results,
		Confidence: 1.0, // Simple strategy doesn't calculate confidence
		ModelPath:  []string{string(model.Role)},
		Escalated:  false,
		Iterations: 1,
		Thoughts:   resp.Thoughts, // Include model's reasoning for UI display
	}, nil
}

// executeChat handles chat tasks
func (s *SimpleStrategy) executeChat(ctx context.Context, task *Task, orch *Orchestrator) (*Result, error) {
	// Start pipeline span for chat
	span := observability.StartPipeline(ctx, observability.PipelineOptions{
		Name:        "chat_completion",
		Description: "Handle chat request using LLM",
	})
	defer span.End(nil)
	ctx = span.Context()

	input, ok := task.Input.(*ChatInput)
	if !ok {
		err := fmt.Errorf("invalid input type for chat task")
		span.SetError(err)
		return nil, err
	}
	span.SetData("message_count", len(input.Messages))
	span.SetData("generate_sql", input.GenerateSQL)

	// Convert messages to LLMMessage format
	messages := make([]LLMMessage, 0, len(input.Messages))
	for _, msg := range input.Messages {
		if m, ok := msg.(map[string]interface{}); ok {
			gm := LLMMessage{
				Role:    getString(m, "role"),
				Content: getString(m, "content"),
			}
			messages = append(messages, gm)
		}
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	// Select model - use reasoning model for chat (better for SQL generation)
	model := orch.GetModel(RoleReasoning)
	if model == nil {
		model = orch.GetModel(RoleFast)
	}

	// Processing chat request

	// Use streaming if callback provided and using Vertex AI
	var resp *LLMResponse
	var err error

	if input.ThoughtCallback != nil && model.Provider == ProviderGoogle && orch.useVertex {
		resp, err = orch.callVertexAIStreaming(ctx, model, messages, input.ThoughtCallback)
	} else {
		resp, err = orch.callModel(ctx, model, messages, false, nil, nil)
	}

	if err != nil {
		return nil, classifyError(err, model, task.Strategy, task.Type)
	}

	// Parse response - expect JSON with sql, answer_template, thought
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		return nil, fmt.Errorf("empty response from model")
	}

	// Parse JSON response
	var chatResponse map[string]interface{}

	if err := json.Unmarshal([]byte(content), &chatResponse); err != nil {
		if err := json.Unmarshal([]byte(extractJSONFromMarkdown(content)), &chatResponse); err != nil {
			previewLen := 200
			if len(content) < previewLen {
				previewLen = len(content)
			}
			parseErr := fmt.Errorf("failed to parse chat response: %w (content: %s)", err, content[:previewLen])
			return nil, classifyError(parseErr, model, task.Strategy, task.Type)
		}
	}

	return &Result{
		Output:     chatResponse,
		Confidence: 1.0,
		ModelPath:  []string{string(model.Role)},
		Escalated:  false,
		Iterations: 1,
		Thoughts:   resp.Thoughts, // Include model's reasoning for chat UI
	}, nil
}

// getString safely extracts a string from a map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// executeChatTools handles chat tasks with tool calling (V2)
func (s *SimpleStrategy) executeChatTools(ctx context.Context, task *Task, orch *Orchestrator) (*Result, error) {
	// Start pipeline span for chat with tools
	span := observability.StartPipeline(ctx, observability.PipelineOptions{
		Name:        "chat_with_tools",
		Description: "Handle chat request with tool calling",
	})
	defer span.End(nil)
	ctx = span.Context()

	input, ok := task.Input.(*ChatToolInput)
	if !ok {
		err := fmt.Errorf("invalid input type for chat_tools task")
		span.SetError(err)
		return nil, err
	}
	span.SetData("message_count", len(input.Messages))
	span.SetData("tool_count", len(input.Tools))

	// Track all thoughts sent to dedupe across iterations (normalized keys)
	sentThoughts := make(map[string]bool)

	// Normalize thought for deduplication (lowercase, trimmed)
	normalizeThought := func(t string) string {
		return strings.ToLower(strings.TrimSpace(t))
	}

	// Helper to send thoughts with deduplication
	sendThought := func(thought string) {
		if input.ThoughtCallback != nil {
			normalized := normalizeThought(thought)

			// Skip if we've already sent this exact thought (normalized)
			if sentThoughts[normalized] {
				return
			}

			// Skip if this thought is similar to one we've sent (substring match)
			for prev := range sentThoughts {
				if strings.Contains(normalized, prev) || strings.Contains(prev, normalized) {
					return
				}
			}

			sentThoughts[normalized] = true
			input.ThoughtCallback(thought)
		}
	}

	// Select model - use tool_call model for tool support
	model := orch.GetModel(RoleToolCall)
	if model == nil {
		model = orch.GetModel(RoleFast)
	}
	if model == nil {
		return nil, fmt.Errorf("no model configured for tool calling")
	}

	// Build initial messages
	messages := []LLMMessage{
		{Role: "system", Content: input.SystemPrompt},
	}
	for _, msg := range input.Messages {
		if m, ok := msg.(map[string]interface{}); ok {
			gm := LLMMessage{
				Role:    getString(m, "role"),
				Content: getString(m, "content"),
			}
			messages = append(messages, gm)
		}
	}

	if len(messages) < 2 {
		return nil, fmt.Errorf("no user messages provided")
	}

	// Tool calling loop - kept low to prevent over-thinking
	const maxIterations = 5
	var allThoughts []string
	var allToolCalls []map[string]interface{}

	// Create wrapped callback for LLM thoughts that uses our deduplication
	wrappedCallback := func(thought string) {
		sendThought(thought)
	}

	for iteration := 0; iteration < maxIterations; iteration++ {
		// On last iteration, don't include tools to force a final response
		isLastIteration := iteration == maxIterations-1
		var tools []ToolDefinition
		if !isLastIteration {
			tools = input.Tools
		}

		// Call model with tools - stream thoughts via wrapped callback
		resp, err := orch.callChatToolsModel(ctx, model, messages, tools, wrappedCallback)
		if err != nil {
			return nil, classifyError(err, model, task.Strategy, task.Type)
		}

		// Collect thoughts (already streamed via callback)
		allThoughts = append(allThoughts, resp.Thoughts...)

		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("no response from LLM")
		}

		choice := resp.Choices[0]

		// Check if there are tool calls
		if len(choice.Message.ToolCalls) == 0 || choice.FinishReason == "stop" {
			// Return just the answer string (not JSON)
			return &Result{
				Output:     choice.Message.Content,
				Confidence: 1.0,
				ModelPath:  []string{string(model.Role)},
				Escalated:  false,
				Iterations: iteration + 1,
				Thoughts:   allThoughts,
				Metadata: map[string]interface{}{
					"tool_calls": allToolCalls,
				},
			}, nil
		}

		// Execute tool calls

		// Add assistant message with tool calls
		messages = append(messages, choice.Message)

		// Track which tool descriptions we've already shown this iteration
		shownDescriptions := make(map[string]bool)

		for _, toolCall := range choice.Message.ToolCalls {
			toolName := toolCall.Function.Name
			toolArgs := toolCall.Function.Arguments

			// Only show each unique tool description once per iteration
			desc := describeToolCallSimple(toolName)
			if !shownDescriptions[desc] {
				sendThought(desc)
				shownDescriptions[desc] = true
			}

			// Track tool call
			allToolCalls = append(allToolCalls, map[string]interface{}{
				"name":      toolName,
				"arguments": toolArgs,
			})

			// Execute the tool
			var result string
			if input.ToolExecutor != nil {
				res, err := input.ToolExecutor.Execute(ctx, toolName, toolArgs)
				if err != nil {
					result = fmt.Sprintf(`{"error": %q}`, err.Error())
				} else {
					result = res
				}
			} else {
				result = `{"error": "tool executor not configured"}`
			}

			// Embed function name in content so buildOpenAIRequestMessages can infer
			// the name even when ToolCallID holds a provider-generated UUID.
			result = annotateToolResponse(result, toolName)
			// Use the actual tool call ID so OpenAI-compat APIs can match responses
			// back to their calls. For Vertex, toolCall.ID equals the function name.
			toolCallID := toolCall.ID
			if toolCallID == "" {
				toolCallID = toolName
			}
			messages = append(messages, LLMMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCallID,
			})
		}
	}

	return nil, fmt.Errorf("max tool call iterations reached")
}

// describeToolCallSimple returns a user-friendly description of a tool
func describeToolCallSimple(toolName string) string {
	switch toolName {
	case "get_transaction":
		return "Looking up transaction details..."
	case "search_transactions":
		return "Searching your transactions..."
	case "get_accounts":
		return "Checking your accounts..."
	case "get_tags":
		return "Loading your categories..."
	case "get_spending_summary":
		return "Calculating your spending..."
	case "get_recurring_patterns":
		return "Analyzing your subscriptions and bills..."
	case "get_entities":
		return "Looking up merchants and contacts..."
	case "find_similar_transactions":
		return "Finding similar transactions..."
	case "find_similar_entities":
		return "Finding similar merchants..."
	default:
		return "Processing..."
	}
}
