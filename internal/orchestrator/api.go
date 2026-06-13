package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"google.golang.org/genai"

	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/processing"
)

// ErrorType defines the type of orchestrator error
type ErrorType string

const (
	ErrTypeModelUnavailable ErrorType = "model_unavailable"
	ErrTypeInvalidInput     ErrorType = "invalid_input"
	ErrTypeTimeout          ErrorType = "timeout"
	ErrTypeRateLimit        ErrorType = "rate_limit"
	ErrTypePermissionDenied ErrorType = "permission_denied"
	ErrTypeAPIError         ErrorType = "api_error"
	ErrTypeParseError       ErrorType = "parse_error"
	ErrTypeUnknown          ErrorType = "unknown"
)

// OrchestratorError represents an error from the orchestrator with context
type OrchestratorError struct {
	Type      ErrorType
	Retryable bool
	Model     string
	Strategy  Strategy
	TaskType  TaskType
	Cause     error
}

func (e *OrchestratorError) Error() string {
	return fmt.Sprintf("orchestrator error [%s]: %v (model: %s, strategy: %s, task: %s, retryable: %v)",
		e.Type, e.Cause, e.Model, e.Strategy, e.TaskType, e.Retryable)
}

func (e *OrchestratorError) Unwrap() error { return e.Cause }

// IsRetryable satisfies the processing.retryable interface so the worker can
// detect non-retryable orchestrator errors without an import cycle.
func (e *OrchestratorError) IsRetryable() bool { return e.Retryable }

// classifyError classifies an error and creates an OrchestratorError
func classifyError(err error, model *ModelSpec, strategy Strategy, taskType TaskType) *OrchestratorError {
	if err == nil {
		return nil
	}

	// Already classified — enrich context fields without re-wrapping
	if orchErr, ok := err.(*OrchestratorError); ok {
		if strategy != "" && orchErr.Strategy == "" {
			orchErr.Strategy = strategy
		}
		if taskType != "" && orchErr.TaskType == "" {
			orchErr.TaskType = taskType
		}
		return orchErr
	}

	errStr := strings.ToLower(err.Error())
	modelName := ""
	if model != nil {
		modelName = fmt.Sprintf("%s/%s", model.Provider, model.Model)
	}

	orchErr := &OrchestratorError{
		Model:     modelName,
		Strategy:  strategy,
		TaskType:  taskType,
		Cause:     err,
		Retryable: true, // Default to retryable
	}

	// Classify error type and retryability
	if strings.Contains(errStr, "403") || strings.Contains(errStr, "permission") ||
		strings.Contains(errStr, "permission_denied") || strings.Contains(errStr, "consumer_invalid") ||
		strings.Contains(errStr, "api key not valid") || strings.Contains(errStr, "api_key_invalid") {
		orchErr.Type = ErrTypePermissionDenied
		orchErr.Retryable = false
	} else if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "quota") || strings.Contains(errStr, "too many requests") {
		orchErr.Type = ErrTypeRateLimit
		orchErr.Retryable = true // Rate limits are retryable with backoff
	} else if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "context deadline") {
		orchErr.Type = ErrTypeTimeout
		orchErr.Retryable = true
	} else if strings.Contains(errStr, "thought_signature") ||
		strings.Contains(errStr, "name cannot be empty") {
		// Structural Gemini protocol errors: retrying with the same messages won't help
		orchErr.Type = ErrTypeParseError
		orchErr.Retryable = false
	} else if strings.Contains(errStr, "invalid") || strings.Contains(errStr, "malformed") ||
		strings.Contains(errStr, "parse") {
		orchErr.Type = ErrTypeParseError
		orchErr.Retryable = false
	} else if strings.Contains(errStr, "not configured") || strings.Contains(errStr, "unavailable") ||
		strings.Contains(errStr, "not found") {
		orchErr.Type = ErrTypeModelUnavailable
		orchErr.Retryable = false
	} else if strings.Contains(errStr, "api") || strings.Contains(errStr, "http") {
		orchErr.Type = ErrTypeAPIError
		orchErr.Retryable = true
	} else {
		orchErr.Type = ErrTypeUnknown
		orchErr.Retryable = true // Default to retryable for unknown errors
	}

	return orchErr
}

// LLMMessage represents a message in an LLM conversation
type LLMMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	// RawVertexParts stores the original genai.Part objects from Vertex AI responses
	// This preserves thought signatures for Gemini 3 models
	RawVertexParts []*genai.Part `json:"-"`
}

// ToolCall represents a tool call from the LLM
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// LLMResponse represents the response from an LLM
type LLMResponse struct {
	Choices []struct {
		Message      LLMMessage `json:"message"`
		FinishReason string     `json:"finish_reason"`
	} `json:"choices"`
	// Thoughts contains the model's reasoning process (Gemini 3+ only)
	// These are human-readable and can be shown to users
	Thoughts []string `json:"thoughts,omitempty"`
	// Usage contains token usage information (if available from API)
	Usage *TokenUsage `json:"usage,omitempty"`
}

// TokenUsage represents token usage from an LLM API response
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
	// For Vertex AI
	CachedTokens    int `json:"cached_tokens,omitempty"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// LLMResult represents the LLM's response for a single transaction
type LLMResult = processing.LLMResult

// EntitySearcher allows the LLM to search for existing entities
type EntitySearcher = processing.EntitySearcher

// PurchaseMatcher allows the LLM to find matching purchases for credits/refunds
type PurchaseMatcher = processing.PurchaseMatcher

// callModel makes an API call to the specified model
func (o *Orchestrator) callModel(ctx context.Context, model *ModelSpec, messages []LLMMessage, includeTools bool, entitySearcher EntitySearcher, purchaseMatcher PurchaseMatcher) (*LLMResponse, error) {
	// Start AI request span following Sentry gen_ai.* conventions
	operation := "chat"
	if includeTools {
		operation = "chat_with_tools"
	}

	// Convert messages to any for JSON serialization
	var msgAny []any
	for _, m := range messages {
		msgAny = append(msgAny, map[string]string{"role": m.Role, "content": truncate(m.Content, 200)})
	}

	aiSpan := observability.StartAIRequest(ctx, observability.AIRequestOptions{
		Model:     model.Model,
		Provider:  string(model.Provider),
		Operation: operation,
		Messages:  msgAny,
		Tags: map[string]string{
			"model_role":    string(model.Role),
			"include_tools": fmt.Sprintf("%v", includeTools),
		},
	})
	spanCtx := aiSpan.Context()

	slog.InfoContext(spanCtx, "Making AI request",
		"operation", operation,
		"model", model.Model,
		"provider", string(model.Provider),
		"include_tools", includeTools,
	)

	var resp *LLMResponse
	var err error

	// Use Vertex AI if configured
	if model.Provider == ProviderGoogle && o.useVertex {
		slog.InfoContext(spanCtx, "LLM transport selected", "provider", string(model.Provider), "transport", "vertex")
		resp, err = o.callVertexAI(spanCtx, model, messages, includeTools, entitySearcher, purchaseMatcher)
	} else {
		// Use OpenAI-compatible API
		slog.InfoContext(spanCtx, "LLM transport selected", "provider", string(model.Provider), "transport", "openai_compat")
		resp, err = o.callOpenAIAPI(spanCtx, model, messages, includeTools)
	}

	// Record error if any
	if err != nil {
		slog.ErrorContext(spanCtx, "AI request failed", "err", err)
		aiSpan.SetError(err)
		aiSpan.End(err)
		return nil, err
	}

	// Record response metadata
	if resp != nil && len(resp.Choices) > 0 {
		slog.InfoContext(spanCtx, "AI request completed successfully",
			"finish_reason", resp.Choices[0].FinishReason,
			"choices", len(resp.Choices),
		)
		aiSpan.SetData("gen_ai.response.finish_reason", resp.Choices[0].FinishReason)

		// Record tool calls if any
		if len(resp.Choices[0].Message.ToolCalls) > 0 {
			var toolCalls []any
			for _, tc := range resp.Choices[0].Message.ToolCalls {
				toolCalls = append(toolCalls, map[string]string{
					"name": tc.Function.Name,
					"type": "function",
				})
			}
			aiSpan.SetToolCalls(toolCalls)
		}

		// Record response text (truncated)
		if resp.Choices[0].Message.Content != "" {
			aiSpan.SetResponse(truncate(resp.Choices[0].Message.Content, 500))
		}

		// Extract and record token usage + costs
		if resp.Usage != nil {
			inputTokens := resp.Usage.PromptTokens
			outputTokens := resp.Usage.CompletionTokens
			reasoningTokens := resp.Usage.ReasoningTokens
			cachedTokens := resp.Usage.CachedTokens

			// Set token usage
			if reasoningTokens > 0 || cachedTokens > 0 {
				aiSpan.SetTokenUsageDetailed(inputTokens, outputTokens, cachedTokens, reasoningTokens)
			} else {
				aiSpan.SetTokenUsage(inputTokens, outputTokens)
			}

			// Calculate and set costs
			modelName := fmt.Sprintf("%s/%s", model.Provider, model.Model)
			promptCost, completionCost, reasoningCost := observability.CalculateCost(
				modelName, inputTokens, outputTokens, reasoningTokens,
			)

			if reasoningCost > 0 {
				aiSpan.SetCostDetailed(promptCost, completionCost, reasoningCost, 0, 0)
			} else {
				aiSpan.SetCost(promptCost, completionCost)
			}

			slog.InfoContext(spanCtx, "Token usage and cost recorded",
				"input_tokens", inputTokens,
				"output_tokens", outputTokens,
				"cost_usd", promptCost+completionCost+reasoningCost,
			)
		}
	}

	aiSpan.End(nil)
	return resp, nil
}

// callVertexAIStreaming makes a streaming call to Vertex AI using the GenAI SDK
// for simple chat (no tools). Calls thoughtCallback as thoughts are received.
func (o *Orchestrator) callVertexAIStreaming(ctx context.Context, model *ModelSpec, messages []LLMMessage, thoughtCallback ThoughtCallback) (*LLMResponse, error) {
	// Start AI request span for streaming call
	aiSpan := observability.StartAIRequest(ctx, observability.AIRequestOptions{
		Model:     model.Model,
		Provider:  string(model.Provider),
		Operation: "chat_stream",
		Tags: map[string]string{
			"streaming": "true",
		},
	})
	defer aiSpan.End(nil)

	if o.vertexClient == nil {
		err := fmt.Errorf("Vertex AI client not initialized")
		aiSpan.SetError(err)
		return nil, err
	}

	vertexModel := o.mapVertexModelName(model.Model)

	contents, systemInstructionText := convertMessagesToVertexContents(messages)
	if len(contents) == 0 {
		return nil, fmt.Errorf("no content messages to send to Vertex AI")
	}

	config := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr[float32](0.1),
		MaxOutputTokens:  32768,
		ResponseMIMEType: "application/json",
	}

	config.ThinkingConfig = thinkingConfig(model.Model, false, messages)

	if systemInstructionText != "" {
		config.SystemInstruction = genai.NewContentFromText(systemInstructionText, genai.RoleModel)
	}

	// Use streaming API
	stream := o.vertexClient.Models.GenerateContentStream(ctx, vertexModel, contents, config)

	var textContent strings.Builder
	var thoughts []string

	// Process streamed chunks
	for chunk, err := range stream {
		if err != nil {
			return nil, fmt.Errorf("Vertex AI streaming error: %w", err)
		}

		if len(chunk.Candidates) == 0 {
			continue
		}

		candidate := chunk.Candidates[0]
		for _, part := range candidate.Content.Parts {
			if part.Thought && part.Text != "" {
				if thought := cleanThought(part.Text); thought != "" && !slices.Contains(thoughts, thought) {
					thoughts = append(thoughts, thought)
					if thoughtCallback != nil {
						thoughtCallback(thought)
					}
				}
			} else if part.Text != "" {
				textContent.WriteString(part.Text)
			}
		}
	}

	responseMsg := LLMMessage{
		Role:    "assistant",
		Content: textContent.String(),
	}

	return &LLMResponse{
		Choices: []struct {
			Message      LLMMessage `json:"message"`
			FinishReason string     `json:"finish_reason"`
		}{
			{
				Message:      responseMsg,
				FinishReason: "stop",
			},
		},
		Thoughts: thoughts,
	}, nil
}

// callVertexAI makes a non-streaming call to Vertex AI using the GenAI SDK.
func (o *Orchestrator) callVertexAI(ctx context.Context, model *ModelSpec, messages []LLMMessage, includeTools bool, entitySearcher EntitySearcher, purchaseMatcher PurchaseMatcher) (*LLMResponse, error) {
	if o.vertexClient == nil {
		return nil, fmt.Errorf("Vertex AI client not initialized")
	}

	vertexModel := o.mapVertexModelName(model.Model)

	contents, systemInstructionText := convertMessagesToVertexContents(messages)
	if len(contents) == 0 {
		return nil, fmt.Errorf("no content messages to send to Vertex AI")
	}

	config := &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0.1),
		MaxOutputTokens: 32768,
	}

	config.ThinkingConfig = thinkingConfig(model.Model, includeTools, messages)

	if systemInstructionText != "" {
		config.SystemInstruction = genai.NewContentFromText(systemInstructionText, genai.RoleModel)
	}

	if includeTools {
		config.Tools = []*genai.Tool{{FunctionDeclarations: o.buildVertexAITools()}}
	} else {
		config.ResponseMIMEType = "application/json"
	}

	sanitizeFunctionResponseNames(contents)

	apiResult, err := o.vertexClient.Models.GenerateContent(ctx, vertexModel, contents, config)
	if err != nil {
		return nil, fmt.Errorf("Vertex AI API error: %w", err)
	}

	if len(apiResult.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in Vertex AI response")
	}

	candidate := apiResult.Candidates[0]
	var functionCalls []ToolCall
	var textContent string
	var thoughts []string

	// Extract function calls, text, and thoughts from response parts
	// Thoughts are the model's visible reasoning process (Gemini 3+)
	for _, part := range candidate.Content.Parts {
		if part.FunctionCall != nil {
			argsJSON, _ := json.Marshal(part.FunctionCall.Args)
			functionCalls = append(functionCalls, ToolCall{
				ID: part.FunctionCall.Name,
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      part.FunctionCall.Name,
					Arguments: string(argsJSON),
				},
			})
		} else if part.Thought && part.Text != "" {
			thoughts = append(thoughts, part.Text)
		} else if part.Text != "" {
			textContent += part.Text
		}
	}

	var finishReason string
	if len(functionCalls) > 0 {
		finishReason = "tool_calls"
	} else {
		switch candidate.FinishReason {
		case genai.FinishReasonMaxTokens:
			return nil, fmt.Errorf("response truncated: max tokens exceeded")
		case genai.FinishReasonSafety:
			finishReason = "content_filter"
		default:
			finishReason = "stop"
		}
	}

	responseMsg := LLMMessage{
		Role:      "assistant",
		Content:   textContent,
		ToolCalls: functionCalls,
	}
	// Only preserve raw parts when thought signatures are consistent (all present or
	// all absent). Inconsistent signatures cause a 400 on the next turn even when
	// ThinkingBudget=0 is set — some models generate them anyway for some calls.
	if len(functionCalls) > 0 && candidate.Content != nil && thoughtSignaturesConsistent(candidate.Content.Parts) {
		responseMsg.RawVertexParts = candidate.Content.Parts
	}

	var usage *TokenUsage
	if apiResult.UsageMetadata != nil {
		usage = &TokenUsage{
			PromptTokens:     int(apiResult.UsageMetadata.PromptTokenCount),
			CompletionTokens: int(apiResult.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(apiResult.UsageMetadata.TotalTokenCount),
		}
		if apiResult.UsageMetadata.CachedContentTokenCount > 0 {
			usage.CachedTokens = int(apiResult.UsageMetadata.CachedContentTokenCount)
		}
	}

	return &LLMResponse{
		Choices: []struct {
			Message      LLMMessage `json:"message"`
			FinishReason string     `json:"finish_reason"`
		}{
			{
				Message:      responseMsg,
				FinishReason: finishReason,
			},
		},
		// Include captured thoughts for UI display
		Thoughts: thoughts,
		Usage:    usage,
	}, nil
}

// callOpenAIAPI makes a call to an OpenAI-compatible API
func (o *Orchestrator) callOpenAIAPI(ctx context.Context, model *ModelSpec, messages []LLMMessage, includeTools bool) (*LLMResponse, error) {
	apiKey := o.getAPIKey(model.Provider)
	if apiKey == "" {
		return nil, fmt.Errorf("no API key for provider %s", model.Provider)
	}

	endpoint := o.getEndpoint(model.Provider, model.Model)
	url := endpoint + "/chat/completions"

	serializedMessages := buildOpenAIRequestMessages(messages)

	reqBody := map[string]interface{}{
		"model":       model.Model,
		"messages":    serializedMessages,
		"temperature": 0.1,
		"max_tokens":  8192,
	}

	if includeTools {
		reqBody["tools"] = []interface{}{entitySearchTool, findMatchingPurchaseTool}
		reqBody["tool_choice"] = "auto"
	} else {
		reqBody["response_format"] = map[string]string{"type": "json_object"}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Choices []struct {
			Message      LLMMessage `json:"message"`
			FinishReason string     `json:"finish_reason"`
		} `json:"choices"`
		Usage *TokenUsage `json:"usage,omitempty"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Convert to LLMResponse format
	llmResp := LLMResponse{
		Choices: apiResp.Choices,
		Usage:   apiResp.Usage,
	}

	return &llmResp, nil
}

// parseResponse extracts LLMResults from an API response
func (o *Orchestrator) parseResponse(resp *LLMResponse) ([]LLMResult, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		return nil, fmt.Errorf("empty content in LLM response (finish_reason: %s)", resp.Choices[0].FinishReason)
	}

	content = extractJSONFromMarkdown(content)

	var results []LLMResult
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "[") {
		if err := json.Unmarshal([]byte(content), &results); err != nil {
			return nil, fmt.Errorf("failed to parse LLM results (raw array): %w", err)
		}
	} else if strings.HasPrefix(content, "{") {
		var resultWrapper struct {
			Results []LLMResult `json:"results"`
		}
		if err := json.Unmarshal([]byte(content), &resultWrapper); err != nil {
			return nil, fmt.Errorf("failed to parse LLM results (wrapped object): %w", err)
		}
		results = resultWrapper.Results
	} else {
		return nil, fmt.Errorf("unexpected response format: %s", content[:min(100, len(content))])
	}

	return results, nil
}

// buildVertexAITools converts tool definitions to Vertex AI FunctionDeclaration format
func (o *Orchestrator) buildVertexAITools() []*genai.FunctionDeclaration {
	return []*genai.FunctionDeclaration{
		{
			Name:        "search_entities",
			Description: "Search for existing entities (businesses) in the database using BM25 full-text search. Use this to find if a business already exists before deciding on a name. Returns entities ranked by relevance to the query.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"query": {
						Type:        genai.TypeString,
						Description: "The business name or keywords to search for (e.g., 'Starbucks', 'coffee shop', 'Amazon')",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "find_matching_purchase",
			Description: "Find a previous purchase that a credit/refund/points-redemption might be offsetting. Use this when you see a credit card credit like 'PwP' (Pay with Points), 'refund', 'cashback', or 'statement credit' to find the original purchase and use the same category.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"amount_cents": {
						Type:        genai.TypeInteger,
						Description: "The absolute amount in cents to search for (e.g., 166850 for $1,668.50)",
					},
					"account_name": {
						Type:        genai.TypeString,
						Description: "The account name to search within (e.g., 'Amex Platinum', 'Chase Sapphire')",
					},
				},
				Required: []string{"amount_cents", "account_name"},
			},
		},
	}
}

// Tool definitions for OpenAI-compatible APIs
var entitySearchTool = map[string]interface{}{
	"type": "function",
	"function": map[string]interface{}{
		"name":        "search_entities",
		"description": "Search for existing entities (businesses) in the database using BM25 full-text search. Use this to find if a business already exists before deciding on a name. Returns entities ranked by relevance to the query.",
		"parameters": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The business name or keywords to search for (e.g., 'Starbucks', 'coffee shop', 'Amazon')",
				},
			},
			"required": []string{"query"},
		},
	},
}

var findMatchingPurchaseTool = map[string]interface{}{
	"type": "function",
	"function": map[string]interface{}{
		"name":        "find_matching_purchase",
		"description": "Find a previous purchase that a credit/refund/points-redemption might be offsetting. Use this when you see a credit card credit like 'PwP' (Pay with Points), 'refund', 'cashback', or 'statement credit' to find the original purchase and use the same category.",
		"parameters": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"amount_cents": map[string]interface{}{
					"type":        "integer",
					"description": "The absolute amount in cents to search for (e.g., 166850 for $1,668.50)",
				},
				"account_name": map[string]interface{}{
					"type":        "string",
					"description": "The account name to search within (e.g., 'Amex Platinum', 'Chase Sapphire')",
				},
			},
			"required": []string{"amount_cents", "account_name"},
		},
	},
}

// callChatToolsModel makes a call to the LLM with custom chat tools
func (o *Orchestrator) callChatToolsModel(ctx context.Context, model *ModelSpec, messages []LLMMessage, tools []ToolDefinition, thoughtCallback ThoughtCallback) (*LLMResponse, error) {
	// Start AI request span with tools
	var toolsAny []any
	for _, t := range tools {
		toolsAny = append(toolsAny, map[string]string{"name": t.Name, "description": truncate(t.Description, 100)})
	}

	aiSpan := observability.StartAIRequest(ctx, observability.AIRequestOptions{
		Model:     model.Model,
		Provider:  string(model.Provider),
		Operation: "chat_with_tools",
		Tools:     toolsAny,
		Tags: map[string]string{
			"model_role":  string(model.Role),
			"tools_count": fmt.Sprintf("%d", len(tools)),
		},
	})
	spanCtx := aiSpan.Context()

	var resp *LLMResponse
	var err error

	if model.Provider == ProviderGoogle && o.useVertex {
		resp, err = o.callVertexAIChatToolsStreaming(spanCtx, model, messages, tools, thoughtCallback)
	} else {
		// For non-Vertex providers, use OpenAI-compatible format (no streaming)
		resp, err = o.callOpenAIChatTools(spanCtx, model, messages, tools)
	}

	// Record error if any
	if err != nil {
		aiSpan.SetError(err)
		aiSpan.End(err)
		return nil, err
	}

	// Record response metadata
	if resp != nil && len(resp.Choices) > 0 {
		aiSpan.SetData("gen_ai.response.finish_reason", resp.Choices[0].FinishReason)
		if len(resp.Choices[0].Message.ToolCalls) > 0 {
			var toolCalls []any
			for _, tc := range resp.Choices[0].Message.ToolCalls {
				toolCalls = append(toolCalls, map[string]string{
					"name": tc.Function.Name,
					"type": "function",
				})
			}
			aiSpan.SetToolCalls(toolCalls)
		}
		if len(resp.Thoughts) > 0 {
			aiSpan.SetData("gen_ai.thoughts_count", len(resp.Thoughts))
		}
		if resp.Choices[0].Message.Content != "" {
			aiSpan.SetResponse(truncate(resp.Choices[0].Message.Content, 500))
		}
	}

	aiSpan.End(nil)
	return resp, err
}

// callVertexAIChatToolsStreaming makes a streaming call to Vertex AI with custom tools
// Streams thoughts to the callback as they arrive
func (o *Orchestrator) callVertexAIChatToolsStreaming(ctx context.Context, model *ModelSpec, messages []LLMMessage, tools []ToolDefinition, thoughtCallback ThoughtCallback) (*LLMResponse, error) {
	if o.vertexClient == nil {
		return nil, fmt.Errorf("Vertex AI client not initialized")
	}

	vertexModel := o.mapVertexModelName(model.Model)

	contents, systemInstructionText := convertMessagesToVertexContents(messages)
	if len(contents) == 0 {
		return nil, fmt.Errorf("no content messages")
	}

	config := &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0.7),
		MaxOutputTokens: 4096,
	}

	config.ThinkingConfig = thinkingConfig(model.Model, len(tools) > 0, messages)

	if systemInstructionText != "" {
		config.SystemInstruction = genai.NewContentFromText(systemInstructionText, genai.RoleModel)
	}

	if len(tools) > 0 {
		config.Tools = []*genai.Tool{{FunctionDeclarations: convertToolsToVertexAI(tools)}}
	}

	sanitizeFunctionResponseNames(contents)

	stream := o.vertexClient.Models.GenerateContentStream(ctx, vertexModel, contents, config)

	var textContent strings.Builder
	var functionCalls []ToolCall
	var thoughts []string
	var rawParts []*genai.Part

	// Process streamed chunks
	for chunk, err := range stream {
		if err != nil {
			return nil, fmt.Errorf("Vertex AI streaming error: %w", err)
		}

		if len(chunk.Candidates) == 0 {
			continue
		}

		candidate := chunk.Candidates[0]
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			rawParts = append(rawParts, part)

			if part.Thought && part.Text != "" {
				if thought := cleanThought(part.Text); thought != "" && !slices.Contains(thoughts, thought) {
					thoughts = append(thoughts, thought)
					if thoughtCallback != nil {
						thoughtCallback(thought)
					}
				}
			} else if part.Text != "" {
				textContent.WriteString(part.Text)
			} else if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				tc := ToolCall{
					ID:   part.FunctionCall.Name,
					Type: "function",
				}
				tc.Function.Name = part.FunctionCall.Name
				tc.Function.Arguments = string(argsJSON)
				functionCalls = append(functionCalls, tc)
			}
		}
	}
	slog.DebugContext(ctx, "vertex stream finished", "thoughts", len(thoughts), "function_calls", len(functionCalls))

	finishReason := "stop"
	if len(functionCalls) > 0 {
		finishReason = "tool_calls"
	}

	responseMsg := LLMMessage{
		Role:      "assistant",
		Content:   textContent.String(),
		ToolCalls: functionCalls,
	}

	// Only preserve raw parts when thought signatures are consistent across all
	// function-call parts; inconsistent signatures cause a 400 on the next turn.
	if len(functionCalls) > 0 && thoughtSignaturesConsistent(rawParts) {
		responseMsg.RawVertexParts = rawParts
	}

	return &LLMResponse{
		Choices: []struct {
			Message      LLMMessage `json:"message"`
			FinishReason string     `json:"finish_reason"`
		}{
			{
				Message:      responseMsg,
				FinishReason: finishReason,
			},
		},
		Thoughts: thoughts,
	}, nil
}

// callOpenAIChatTools makes a call to OpenAI-compatible API with custom tools
func (o *Orchestrator) callOpenAIChatTools(ctx context.Context, model *ModelSpec, messages []LLMMessage, tools []ToolDefinition) (*LLMResponse, error) {
	apiKey := o.getAPIKey(model.Provider)
	if apiKey == "" {
		return nil, fmt.Errorf("no API key for provider %s", model.Provider)
	}

	endpoint := o.getEndpoint(model.Provider, model.Model)
	url := endpoint + "/chat/completions"

	// Convert tools to OpenAI format
	openAITools := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		openAITools[i] = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		}
	}

	serializedMessages := buildOpenAIRequestMessages(messages)

	reqBody := map[string]interface{}{
		"model":       model.Model,
		"messages":    serializedMessages,
		"temperature": 0.7,
		"max_tokens":  4096,
	}

	if len(openAITools) > 0 {
		reqBody["tools"] = openAITools
		reqBody["tool_choice"] = "auto"
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var llmResp LLMResponse
	if err := json.Unmarshal(body, &llmResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &llmResp, nil
}

// convertToolsToVertexAI converts tool definitions to Vertex AI format
func convertToolsToVertexAI(tools []ToolDefinition) []*genai.FunctionDeclaration {
	var declarations []*genai.FunctionDeclaration
	for _, tool := range tools {
		decl := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
		}

		// Convert parameters to genai.Schema
		if tool.Parameters != nil {
			decl.Parameters = convertParamsToSchema(tool.Parameters)
		}

		declarations = append(declarations, decl)
	}
	return declarations
}

// convertParamsToSchema converts a map to genai.Schema
func convertParamsToSchema(params map[string]interface{}) *genai.Schema {
	schema := &genai.Schema{
		Type: genai.TypeObject,
	}

	if props, ok := params["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for name, propVal := range props {
			if prop, ok := propVal.(map[string]interface{}); ok {
				propSchema := &genai.Schema{}
				if t, ok := prop["type"].(string); ok {
					switch t {
					case "string":
						propSchema.Type = genai.TypeString
					case "integer":
						propSchema.Type = genai.TypeInteger
					case "number":
						propSchema.Type = genai.TypeNumber
					case "boolean":
						propSchema.Type = genai.TypeBoolean
					case "array":
						propSchema.Type = genai.TypeArray
					default:
						propSchema.Type = genai.TypeString
					}
				}
				if desc, ok := prop["description"].(string); ok {
					propSchema.Description = desc
				}
				schema.Properties[name] = propSchema
			}
		}
	}

	if required, ok := params["required"].([]interface{}); ok {
		for _, r := range required {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}

	return schema
}

// parseJSONArgs parses JSON arguments string to map
func parseJSONArgs(args string) map[string]interface{} {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(args), &result); err != nil {
		return map[string]interface{}{}
	}
	return result
}

// parseJSONResult parses JSON result string to map
func parseJSONResult(content string) map[string]interface{} {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// If not valid JSON, wrap in a result object
		return map[string]interface{}{"result": content}
	}
	return result
}

func buildOpenAIRequestMessages(messages []LLMMessage) []map[string]interface{} {
	serialized := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		entry := map[string]interface{}{
			"role": msg.Role,
		}

		if msg.Content != "" {
			entry["content"] = msg.Content
		}

		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]map[string]interface{}, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			entry["tool_calls"] = toolCalls
		}

		if msg.ToolCallID != "" {
			entry["tool_call_id"] = msg.ToolCallID
		}

		if msg.Role == "tool" {
			// Prefer the function name embedded in annotated content over the
			// ToolCallID, which may be a provider-assigned UUID rather than the
			// function name.
			payload := parseJSONResult(msg.Content)
			toolName := inferToolResponseNameFromPayload(payload)
			if toolName == "" {
				toolName = strings.TrimSpace(msg.ToolCallID)
			}
			if toolName == "" {
				toolName = "search_entities"
			}
			entry["name"] = toolName
		}

		serialized = append(serialized, entry)
	}

	return serialized
}

// convertMessagesToVertexContents converts []LLMMessage to the genai.Content slice
// expected by the Vertex AI SDK. It preserves RawVertexParts (including thought
// signatures) when available, and resolves tool-response function names with a
// multi-level fallback so the API never receives an empty name.
func convertMessagesToVertexContents(messages []LLMMessage) ([]*genai.Content, string) {
	var contents []*genai.Content
	var systemInstruction string
	var pendingToolResponses []*genai.Part
	var expectedNames []string

	for i, msg := range messages {
		switch msg.Role {
		case "system":
			systemInstruction = msg.Content
		case "user":
			if len(pendingToolResponses) > 0 {
				contents = append(contents, &genai.Content{Parts: pendingToolResponses, Role: genai.RoleUser})
				pendingToolResponses = nil
			}
			contents = append(contents, genai.NewContentFromText(msg.Content, genai.RoleUser))
		case "assistant":
			if len(pendingToolResponses) > 0 {
				contents = append(contents, &genai.Content{Parts: pendingToolResponses, Role: genai.RoleUser})
				pendingToolResponses = nil
			}
			if len(msg.RawVertexParts) > 0 {
				// Preserve thought signatures required for multi-turn tool calling.
				expectedNames = append(expectedNames, extractFunctionCallNames(msg.RawVertexParts)...)
				contents = append(contents, &genai.Content{Parts: msg.RawVertexParts, Role: genai.RoleModel})
			} else {
				var parts []*genai.Part
				if msg.Content != "" {
					parts = append(parts, &genai.Part{Text: msg.Content})
				}
				for _, tc := range msg.ToolCalls {
					if name := strings.TrimSpace(tc.Function.Name); name != "" {
						expectedNames = append(expectedNames, name)
					}
					parts = append(parts, &genai.Part{
						FunctionCall: &genai.FunctionCall{
							Name: tc.Function.Name,
							Args: parseJSONArgs(tc.Function.Arguments),
						},
					})
				}
				if len(parts) > 0 {
					contents = append(contents, &genai.Content{Parts: parts, Role: genai.RoleModel})
				}
			}
		case "tool":
			payload := parseJSONResult(msg.Content)
			name := strings.TrimSpace(msg.ToolCallID)
			if name == "" && len(expectedNames) > 0 {
				name = expectedNames[0]
			}
			if len(expectedNames) > 0 {
				expectedNames = expectedNames[1:]
			}
			if name == "" {
				name = inferToolResponseNameFromPayload(payload)
			}
			if name == "" {
				name = "search_entities"
			}
			pendingToolResponses = append(pendingToolResponses, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{Name: name, Response: payload},
			})
		}

		if i == len(messages)-1 && len(pendingToolResponses) > 0 {
			contents = append(contents, &genai.Content{Parts: pendingToolResponses, Role: genai.RoleUser})
			pendingToolResponses = nil
		}
	}
	return contents, systemInstruction
}

// cleanThought strips markdown and trims a thought to its first line.
// Returns "" if the result is too short to be useful.
func cleanThought(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "\n"); idx > 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	if len(s) < 5 {
		return ""
	}
	return s
}

func sanitizeFunctionResponseNames(contents []*genai.Content) {
	for _, content := range contents {
		if content == nil {
			continue
		}
		for _, part := range content.Parts {
			if part == nil || part.FunctionResponse == nil {
				continue
			}
			if strings.TrimSpace(part.FunctionResponse.Name) == "" {
				part.FunctionResponse.Name = "search_entities"
			}
		}
	}
}

func extractFunctionCallNames(parts []*genai.Part) []string {
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == nil || part.FunctionCall == nil {
			continue
		}
		if name := strings.TrimSpace(part.FunctionCall.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func inferToolResponseNameFromPayload(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}

	if functionName, ok := payload["function_name"].(string); ok && strings.TrimSpace(functionName) != "" {
		return strings.TrimSpace(functionName)
	}
	if functionName, ok := payload["name"].(string); ok {
		candidate := strings.TrimSpace(functionName)
		// Avoid common data-field names being mistaken for tool names.
		if candidate == "search_entities" || candidate == "find_matching_purchase" {
			return candidate
		}
	}

	if _, ok := payload["entities"]; ok {
		return "search_entities"
	}
	if _, ok := payload["query"]; ok {
		return "search_entities"
	}
	if _, ok := payload["purchase_id"]; ok {
		return "find_matching_purchase"
	}
	if _, ok := payload["found"]; ok {
		return "find_matching_purchase"
	}
	if _, ok := payload["amount_cents"]; ok {
		return "find_matching_purchase"
	}

	return ""
}

// truncate truncates a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractJSONFromMarkdown strips a JSON value from a markdown code block if present.
func extractJSONFromMarkdown(s string) string {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "```") {
		return s
	}
	start := strings.Index(s, "```json")
	if start == -1 {
		start = strings.Index(s, "```")
	}
	if start == -1 {
		return s
	}
	newlinePos := strings.Index(s[start:], "\n")
	if newlinePos == -1 {
		return s
	}
	jsonStart := start + newlinePos + 1
	end := strings.Index(s[jsonStart:], "```")
	if end != -1 {
		return strings.TrimSpace(s[jsonStart : jsonStart+end])
	}
	return strings.TrimSpace(s[jsonStart:])
}

func isThinkingModel(model string) bool {
	return strings.Contains(model, "gemini-3") || strings.Contains(model, "gemini-2.5")
}

// thinkingConfig returns the ThinkingConfig for a Vertex AI request.
// Thinking is enabled only when no tools are active and message history contains
// no tool calls, to prevent "Function call is missing a thought_signature" 400s.
// Returns nil for non-thinking models (leaves the field at its zero value).
func thinkingConfig(model string, hasTools bool, messages []LLMMessage) *genai.ThinkingConfig {
	if !isThinkingModel(model) {
		return nil
	}
	if !hasTools && !hasToolHistory(messages) {
		return &genai.ThinkingConfig{IncludeThoughts: true}
	}
	return &genai.ThinkingConfig{ThinkingBudget: genai.Ptr[int32](0)}
}

// thoughtSignaturesConsistent returns true when ALL function-call parts carry a
// ThoughtSignature or NONE do. Inconsistent signatures (some present, some absent)
// cause a "Function call is missing a thought_signature" 400 error on the next turn.
func thoughtSignaturesConsistent(parts []*genai.Part) bool {
	var fcCount, signedCount int
	for _, p := range parts {
		if p == nil || p.FunctionCall == nil {
			continue
		}
		fcCount++
		if len(p.ThoughtSignature) > 0 {
			signedCount++
		}
	}
	return fcCount == 0 || signedCount == 0 || signedCount == fcCount
}

// hasToolHistory reports whether any message in the conversation is a tool
// response or carries tool calls. When true, enabling ThinkingConfig on the
// next request would cause a 400 "Function call is missing a thought_signature"
// error because the prior function-call parts were generated without thinking
// and therefore carry no thought signatures.
func hasToolHistory(messages []LLMMessage) bool {
	for _, msg := range messages {
		if msg.Role == "tool" || len(msg.ToolCalls) > 0 {
			return true
		}
	}
	return false
}
