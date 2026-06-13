package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/embedding"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/orchestrator"
	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// ThreadStoreThread is returned by the thread store
type ThreadStoreThread interface {
	GetID() uuid.UUID
	GetTitle() string
}

// ThreadStoreMessage is returned by the thread store
type ThreadStoreMessage interface {
	GetRole() string
	GetContent() string
}

// ThreadStore interface for chat persistence
type ThreadStore interface {
	CreateThread(ctx context.Context, ledgerID, userID uuid.UUID, parentThreadID *uuid.UUID) (ThreadStoreThread, error)
	GetThreadForUser(ctx context.Context, id, ledgerID, userID uuid.UUID) (ThreadStoreThread, error)
	GetMessages(ctx context.Context, threadID uuid.UUID) ([]ThreadStoreMessage, error)
	AddMessage(ctx context.Context, threadID uuid.UUID, role, content, sqlQuery string, results interface{}) (ThreadStoreMessage, error)
	UpdateThreadTitle(ctx context.Context, id uuid.UUID, title string) error
	CountMessages(ctx context.Context, threadID uuid.UUID) (int, error)
}

// ChatHandler handles chat API requests
type ChatHandler struct {
	cfg              *config.Config
	transactions     *models.TransactionStore
	accounts         *models.AccountStore
	tags             *models.TagStore
	rules            *models.RuleStore
	patterns         *models.RecurringPatternStore
	entities         *models.EntityStore
	relationships    *models.EntityRelationshipStore
	ledgers          *models.LedgerStore
	embeddingService *embedding.Service // Optional: for similarity search
	threadStore      ThreadStore        // Optional: for thread persistence
}

// NewChatHandler creates a new chat handler
func NewChatHandler(
	cfg *config.Config,
	transactions *models.TransactionStore,
	accounts *models.AccountStore,
	tags *models.TagStore,
	rules *models.RuleStore,
	patterns *models.RecurringPatternStore,
	entities *models.EntityStore,
	relationships *models.EntityRelationshipStore,
	ledgers *models.LedgerStore,
) *ChatHandler {
	return &ChatHandler{
		cfg:           cfg,
		transactions:  transactions,
		accounts:      accounts,
		tags:          tags,
		rules:         rules,
		patterns:      patterns,
		entities:      entities,
		relationships: relationships,
		ledgers:       ledgers,
	}
}

// SetEmbeddingService sets the embedding service for similarity search tools
func (h *ChatHandler) SetEmbeddingService(svc *embedding.Service) {
	h.embeddingService = svc
}

// SetThreadStore sets the thread store for chat persistence
func (h *ChatHandler) SetThreadStore(store ThreadStore) {
	h.threadStore = store
}

// ChatRequest represents an incoming chat request
type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
	LedgerID string        `json:"ledger_id"`
	ThreadID string        `json:"thread_id,omitempty"` // Optional: continue existing thread

	// Optional: transaction context
	TransactionID string `json:"transaction_id,omitempty"`

	// V1 compatibility: if "question" is provided, convert to messages format
	Question string `json:"question,omitempty"`
}

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role       string     `json:"role"` // "user", "assistant", "system", "tool"
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ChatResponse represents the response from the chat endpoint
type ChatResponse struct {
	Message     ChatMessage  `json:"message"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// HandleChat processes a chat request (supports both SSE streaming and JSON)
func (h *ChatHandler) HandleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, ChatResponse{Error: "Invalid request body"})
		return
	}

	// V1 compatibility: convert "question" to messages format
	if req.Question != "" && len(req.Messages) == 0 {
		req.Messages = []ChatMessage{
			{Role: "user", Content: req.Question},
		}
	}

	if len(req.Messages) == 0 {
		respondJSON(w, http.StatusBadRequest, ChatResponse{Error: "No messages provided"})
		return
	}

	// Parse ledger ID
	ledgerID, err := uuid.Parse(req.LedgerID)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, ChatResponse{Error: "Invalid ledger_id"})
		return
	}

	// Verify ledger exists
	ledger, err := h.ledgers.GetByID(r.Context(), ledgerID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, ChatResponse{Error: "Ledger not found"})
		return
	}

	// Check if client wants SSE streaming
	if r.Header.Get("Accept") == "text/event-stream" {
		h.handleChatSSE(w, r, req, ledger)
		return
	}

	// JSON response (non-streaming)
	h.handleChatJSON(w, r, req, ledger)
}

// handleChatJSON handles chat with JSON response
func (h *ChatHandler) handleChatJSON(w http.ResponseWriter, r *http.Request, req ChatRequest, ledger *models.Ledger) {
	ctx := r.Context()

	// Start agent span for the entire chat request (child of HTTP transaction)
	// (OTEL → PostHog LLM analytics)
	userQuestion := ""
	if len(req.Messages) > 0 {
		userQuestion = req.Messages[len(req.Messages)-1].Content
	}
	agentSpan := observability.StartAgent(ctx, observability.AgentOptions{
		Name:        "chat_v2_json",
		Description: "Handle V2 chat JSON request",
		Input:       userQuestion,
		Tags: map[string]string{
			"ledger_id": ledger.ID.String(),
		},
	})
	defer agentSpan.End(nil)
	ctx = agentSpan.Context() // Use context with agent span for all subsequent operations

	// Create orchestrator
	orch, err := orchestrator.NewOrchestrator(h.cfg)
	if err != nil {
		slog.ErrorContext(ctx, "chat v2: orchestrator init failed", "err", err, "ledger_id", ledger.ID.String())
		observability.CaptureFailure(ctx, err, observability.FailureOptions{Component: "chat_v2", Operation: "orchestrator_new"})
		agentSpan.SetError(err)
		respondJSON(w, http.StatusInternalServerError, ChatResponse{Error: "AI assistant is not available. Please try again later."})
		return
	}

	// Build conversation context
	conversationCtx := h.buildContext(ctx, ledger, req.TransactionID)

	response, err := h.processChat(ctx, req.Messages, conversationCtx, h.newExecutor(ledger.ID), orch, nil)
	if err != nil {
		slog.ErrorContext(ctx, "chat v2: process failed", "err", err, "ledger_id", ledger.ID.String())
		observability.CaptureFailure(ctx, err, observability.FailureOptions{Component: "chat_v2", Operation: "process_chat"})
		agentSpan.SetError(err)
		respondJSON(w, http.StatusInternalServerError, ChatResponse{Error: err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, response)
}

// handleChatSSE handles chat with SSE streaming (for web frontend)
func (h *ChatHandler) handleChatSSE(w http.ResponseWriter, r *http.Request, req ChatRequest, ledger *models.Ledger) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ctx := r.Context()

	// Get user for thread persistence
	user := auth.APICurrentUser(r)
	if user == nil {
		user = auth.CurrentUser(r)
	}

	// Handle thread persistence if store is available
	var threadID uuid.UUID
	var isNewThread bool
	if h.threadStore != nil && user != nil {
		if req.ThreadID != "" {
			// Load existing thread
			tid, err := uuid.Parse(req.ThreadID)
			if err != nil {
				sendSSEEvent(w, "error", `{"error":"invalid thread_id"}`)
				sendSSEEvent(w, "done", "")
				return
			}
			thread, err := h.threadStore.GetThreadForUser(ctx, tid, ledger.ID, user.ID)
			if err != nil {
				slog.WarnContext(ctx, "chat v2: failed to load thread", "thread_id", req.ThreadID, "err", err)
				sendSSEEvent(w, "error", `{"error":"thread not found"}`)
				sendSSEEvent(w, "done", "")
				return
			}
			threadID = thread.GetID()
		} else {
			// Create new thread
			thread, err := h.threadStore.CreateThread(ctx, ledger.ID, user.ID, nil)
			if err != nil {
				slog.WarnContext(ctx, "chat v2: failed to create thread", "err", err)
				// Continue without thread - don't fail the request
			} else {
				threadID = thread.GetID()
				isNewThread = true
			}
		}
	}

	// Create orchestrator
	orch, err := orchestrator.NewOrchestrator(h.cfg)
	if err != nil {
		slog.ErrorContext(ctx, "chat v2 sse: orchestrator init failed", "err", err, "ledger_id", ledger.ID.String())
		observability.CaptureFailure(ctx, err, observability.FailureOptions{Component: "chat_v2_sse", Operation: "orchestrator_new"})
		sendSSEEvent(w, "error", `{"error":"AI assistant is not available. Please try again later."}`)
		sendSSEEvent(w, "done", "")
		return
	}

	// Build conversation context
	conversationCtx := h.buildContext(ctx, ledger, req.TransactionID)

	// Send initial thought
	sendSSEEvent(w, "thought", "Understanding your question...")

	// Create thought callback for streaming
	thoughtCallback := func(thought string) {
		sendSSEEvent(w, "thought", thought)
	}

	// Start agent span for the entire chat request (OTEL → PostHog LLM analytics)
	userQuestion := ""
	if len(req.Messages) > 0 {
		userQuestion = req.Messages[len(req.Messages)-1].Content
	}
	agentSpan := observability.StartAgent(ctx, observability.AgentOptions{
		Name:        "chat_v2_sse",
		Description: "Handle V2 chat SSE request",
		Input:       userQuestion,
		Tags: map[string]string{
			"ledger_id": ledger.ID.String(),
		},
	})
	defer agentSpan.End(nil)
	ctx = agentSpan.Context() // Use context with agent span for all subsequent operations

	// Process the chat with thought streaming
	response, err := h.processChat(ctx, req.Messages, conversationCtx, h.newExecutor(ledger.ID), orch, thoughtCallback)
	if err != nil {
		slog.ErrorContext(ctx, "chat v2 sse: process failed", "err", err, "ledger_id", ledger.ID.String())
		observability.CaptureFailure(ctx, err, observability.FailureOptions{Component: "chat_v2_sse", Operation: "process_chat"})
		agentSpan.SetError(err)
		sendSSEEvent(w, "error", fmt.Sprintf(`{"error":"%s"}`, err.Error()))
		sendSSEEvent(w, "done", "")
		return
	}

	// Save messages to thread if available
	if h.threadStore != nil && threadID != uuid.Nil {
		if userQuestion != "" {
			if _, err := h.threadStore.AddMessage(ctx, threadID, "user", userQuestion, "", nil); err != nil {
				slog.WarnContext(ctx, "failed to save user message to thread", "thread_id", threadID, "err", err)
			}
		}
		if _, err := h.threadStore.AddMessage(ctx, threadID, "assistant", response.Message.Content, "", nil); err != nil {
			slog.WarnContext(ctx, "failed to save assistant message to thread", "thread_id", threadID, "err", err)
		}
	}

	// Send summary
	summary := "Here's what I found:"
	sendSSEEvent(w, "summary", summary)

	// Convert markdown to HTML for frontend rendering
	answerHTML := renderMarkdown(response.Message.Content)

	// Format results for frontend
	results := map[string]any{
		"answer":  answerHTML,
		"summary": summary,
	}

	// Add thread_id to results if we have one
	if threadID != uuid.Nil {
		results["thread_id"] = threadID.String()
	}

	// Add tool information if tools were called
	if len(response.ToolCalls) > 0 {
		columns, rows := extractTableData(response.ToolResults)
		if len(rows) > 0 {
			results["columns"] = columns
			results["rows"] = rows
			results["count"] = len(rows)
		}
	}

	resultsJSON, _ := json.Marshal(results)
	sendSSEEvent(w, "results", string(resultsJSON))

	// Generate title for new threads synchronously and send via SSE
	if h.threadStore != nil && threadID != uuid.Nil && isNewThread && userQuestion != "" {
		title := h.generateThreadTitleSync(ctx, userQuestion)
		if title != "" {
			if err := h.threadStore.UpdateThreadTitle(ctx, threadID, title); err != nil {
				slog.WarnContext(ctx, "chat v2: failed to update thread title", "thread_id", threadID.String(), "err", err)
			} else {
				// Send title update via SSE
				titleData, _ := json.Marshal(map[string]string{
					"thread_id": threadID.String(),
					"title":     title,
				})
				sendSSEEvent(w, "title", string(titleData))
			}
		}
	}

	sendSSEEvent(w, "done", "")
}

func (h *ChatHandler) generateThreadTitleSync(ctx context.Context, firstMessage string) string {
	return orchestrator.GenerateChatTitle(ctx, h.cfg, firstMessage)
}

func (h *ChatHandler) newExecutor(ledgerID uuid.UUID) *Executor {
	e := NewExecutor(ledgerID, h.transactions, h.accounts, h.tags, h.rules, h.patterns, h.entities, h.relationships)
	if h.embeddingService != nil {
		e.SetEmbeddingService(h.embeddingService)
	}
	return e
}

// executorWrapper wraps the llm.Executor to implement orchestrator.ToolExecutor
type executorWrapper struct {
	executor *Executor
}

func (w *executorWrapper) Execute(ctx context.Context, toolName string, arguments string) (string, error) {
	// Create a ToolCall for the executor
	toolCall := ToolCall{
		ID:   toolName,
		Type: "function",
		Function: FunctionCall{
			Name:      toolName,
			Arguments: arguments,
		},
	}

	result := w.executor.Execute(ctx, toolCall)
	if result.Error != "" {
		return "", errors.New(result.Error)
	}
	return result.Content, nil
}

// convertToolsToOrchestrator converts llm.Tool definitions to orchestrator.ToolDefinition
func convertToolsToOrchestrator(tools []Tool) []orchestrator.ToolDefinition {
	var defs []orchestrator.ToolDefinition
	for _, tool := range tools {
		var params map[string]interface{}
		if err := json.Unmarshal(tool.Function.Parameters, &params); err != nil {
			params = map[string]interface{}{}
		}
		defs = append(defs, orchestrator.ToolDefinition{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  params,
		})
	}
	return defs
}

// processChat handles the main chat logic using the orchestrator with tool calling
func (h *ChatHandler) processChat(ctx context.Context, messages []ChatMessage, convCtx *ConversationContext, executor *Executor, orch *orchestrator.Orchestrator, thoughtCb orchestrator.ThoughtCallback) (*ChatResponse, error) {
	// Build system prompt
	systemPrompt := SystemPromptWithTools(convCtx)

	// Convert messages to orchestrator format
	var orchMessages []interface{}
	for _, msg := range messages {
		orchMessages = append(orchMessages, map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	// Create tool executor wrapper
	toolExecutor := &executorWrapper{executor: executor}

	// Get all available tools
	tools := AllTools()

	// Create chat tools task for orchestrator
	task := &orchestrator.Task{
		Type:     orchestrator.TaskTypeChatTools,
		Strategy: orchestrator.StrategySimple,
		Input: &orchestrator.ChatToolInput{
			Messages:        orchMessages,
			SystemPrompt:    systemPrompt,
			ToolExecutor:    toolExecutor,
			Tools:           convertToolsToOrchestrator(tools),
			ThoughtCallback: thoughtCb,
		},
		Context: &orchestrator.TaskContext{
			LedgerID: convCtx.LedgerID,
		},
	}

	// Execute via orchestrator (handles tool calling loop internally)
	result, err := orch.Execute(ctx, task)
	if err != nil {
		slog.ErrorContext(ctx, "chat v2: orchestrator execute failed", "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{Component: "chat_v2", Operation: "orchestrator_execute"})
		return nil, fmt.Errorf("Sorry, I couldn't process your request. Please try again.")
	}

	// Parse the response - Output is now just the answer string
	var assistantContent string
	var toolCalls []ToolCall

	switch output := result.Output.(type) {
	case string:
		assistantContent = output
	case map[string]interface{}:
		// Fallback for older format
		if answer, ok := output["answer"].(string); ok {
			assistantContent = answer
		}
	}

	// Get tool calls from metadata (for response info)
	if result.Metadata != nil {
		if tc, ok := result.Metadata["tool_calls"].([]map[string]interface{}); ok {
			for _, t := range tc {
				toolCalls = append(toolCalls, ToolCall{
					ID:   getString(t, "name"),
					Type: "function",
					Function: FunctionCall{
						Name:      getString(t, "name"),
						Arguments: getString(t, "arguments"),
					},
				})
			}
		}
	}

	if assistantContent == "" {
		return nil, fmt.Errorf("Sorry, I couldn't generate a response. Please try again.")
	}

	return &ChatResponse{
		Message: ChatMessage{
			Role:    "assistant",
			Content: assistantContent,
		},
		ToolCalls: toolCalls,
	}, nil
}

// sendSSEEvent sends a server-sent event
func sendSSEEvent(w http.ResponseWriter, event, data string) {
	// Only log non-thought events to reduce noise
	if event != "thought" {
		slog.Debug("sse-v2 event", "event", event, "preview", data[:min(50, len(data))])
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)

	// Use ResponseController to flush through middleware wrappers (Go 1.20+)
	rc := http.NewResponseController(w)
	if err := rc.Flush(); err != nil {
		slog.Warn("sse-v2 flush error", "err", err)
	}
}

// llmMarkdownRenderer is a configured goldmark instance for chat rendering
var llmMarkdownRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM, // GitHub Flavored Markdown: tables, strikethrough, autolinks, task lists
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(), // Convert single newlines to <br> tags
	),
)

// renderMarkdown converts markdown text to HTML for frontend display
func renderMarkdown(text string) string {
	var buf bytes.Buffer
	if err := llmMarkdownRenderer.Convert([]byte(text), &buf); err != nil {
		// If markdown conversion fails, return escaped plain text
		return text
	}
	return buf.String()
}

// extractTableData extracts tabular data from tool results for display
func extractTableData(results []ToolResult) ([]string, [][]any) {
	for _, result := range results {
		if result.Error != "" {
			continue
		}

		var data map[string]any
		if err := json.Unmarshal([]byte(result.Content), &data); err != nil {
			continue
		}

		// Look for array data in the result
		for key, val := range data {
			if arr, ok := val.([]any); ok && len(arr) > 0 {
				// Try to extract columns from first item
				if firstItem, ok := arr[0].(map[string]any); ok {
					var columns []string
					for col := range firstItem {
						columns = append(columns, col)
					}

					var rows [][]any
					for _, item := range arr {
						if itemMap, ok := item.(map[string]any); ok {
							var row []any
							for _, col := range columns {
								row = append(row, itemMap[col])
							}
							rows = append(rows, row)
						}
					}

					if len(rows) > 0 {
						slog.Debug("chat v2 extracted table", "source", key, "columns", len(columns), "rows", len(rows))
						return columns, rows
					}
				}
			}
		}
	}
	return nil, nil
}

// buildContext creates the conversation context from the ledger
func (h *ChatHandler) buildContext(ctx context.Context, ledger *models.Ledger, transactionID string) *ConversationContext {
	convCtx := &ConversationContext{
		LedgerID:   ledger.ID.String(),
		LedgerName: ledger.Name,
		Currency:   ledger.Currency,
	}

	// Get account count
	accounts, err := h.accounts.GetByLedgerID(ctx, ledger.ID)
	if err == nil {
		convCtx.AccountCount = len(accounts)
	}

	// Get transaction count (approximate)
	isTransfer := false
	txns, total, err := h.transactions.List(ctx, models.TransactionFilter{
		LedgerID:   ledger.ID,
		IsTransfer: &isTransfer,
		Limit:      1,
	})
	if err == nil {
		convCtx.TransactionCount = total
		if len(txns) > 0 {
			convCtx.RecentActivity = fmt.Sprintf("Last transaction: %s", txns[0].Date.Format("Jan 2"))
		}
	}

	// If a specific transaction is being discussed
	if transactionID != "" {
		if txnID, err := uuid.Parse(transactionID); err == nil {
			if txn, err := h.transactions.GetByID(ctx, txnID); err == nil && txn.LedgerID == ledger.ID {
				convCtx.CurrentTransactionID = transactionID
				desc := txn.Description
				if txn.DisplayTitle != "" {
					desc = txn.DisplayTitle
				}
				convCtx.CurrentTransactionDescription = desc
			}
		}
	}

	return convCtx
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
