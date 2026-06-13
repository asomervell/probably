package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/chat"
	"github.com/asomervell/probably/internal/orchestrator"
	"github.com/asomervell/probably/internal/views/pages"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// buildHistoryFromMessages builds a prompt history string from persisted messages
func buildHistoryFromMessages(messages []chat.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Previous conversation:\n")

	for _, msg := range messages {
		if msg.Role == "user" {
			sb.WriteString(fmt.Sprintf("User: %s\n", msg.Content))
		} else if msg.Role == "assistant" {
			sb.WriteString(fmt.Sprintf("Assistant: %s\n", msg.Content))
		}
	}

	return sb.String()
}

func (hdl *Handlers) generateThreadTitle(threadID uuid.UUID, firstMessage string) {
	ctx := context.Background()
	title := orchestrator.GenerateChatTitle(ctx, hdl.cfg, firstMessage)
	if err := hdl.chatThreads.UpdateThreadTitle(ctx, threadID, title); err != nil {
		slog.ErrorContext(ctx, "Failed to update thread title", "err", err)
	}
}


// chatMarkdownRenderer is a configured goldmark instance for chat rendering
var chatMarkdownRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM, // GitHub Flavored Markdown: tables, strikethrough, autolinks, task lists
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(), // Convert single newlines to <br> tags
	),
)

// renderMarkdownToHTML converts markdown text to HTML for frontend display
func renderMarkdownToHTML(text string) string {
	var buf bytes.Buffer
	if err := chatMarkdownRenderer.Convert([]byte(text), &buf); err != nil {
		// If markdown conversion fails, return original text
		return text
	}
	return buf.String()
}

// =============================================================================
// Similarity Detection
// =============================================================================

// similarityPatterns are regex patterns that indicate a similarity query
var similarityPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)similar\s+to`),
	regexp.MustCompile(`(?i)like\s+(my|the|this)`),
	regexp.MustCompile(`(?i)transactions?\s+like`),
	regexp.MustCompile(`(?i)find\s+.*similar`),
	regexp.MustCompile(`(?i)what\s+.*like\s+\w+`),
	regexp.MustCompile(`(?i)other\s+.*like`),
	regexp.MustCompile(`(?i)related\s+to`),
}

// isSimilarityQuery checks if the question is asking for similar transactions
func isSimilarityQuery(question string) bool {
	for _, pattern := range similarityPatterns {
		if pattern.MatchString(question) {
			return true
		}
	}
	return false
}

// extractSimilarityTargetPatterns are compiled regex patterns for extracting the similarity target
var extractSimilarityTargetPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)similar\s+to\s+(?:my\s+)?(.+?)(?:\s+subscription|\s+payment|\s+transaction)?[.?]?$`),
	regexp.MustCompile(`(?i)like\s+(?:my\s+)?(.+?)(?:\s+subscription|\s+payment|\s+transaction)?[.?]?$`),
	regexp.MustCompile(`(?i)transactions?\s+like\s+(?:my\s+)?(.+?)(?:\s+subscription|\s+payment|\s+transaction)?[.?]?$`),
	regexp.MustCompile(`(?i)related\s+to\s+(?:my\s+)?(.+?)(?:\s+subscription|\s+payment|\s+transaction)?[.?]?$`),
}

// extractSimilarityTarget tries to extract what the user wants to find similar items to
func extractSimilarityTarget(question string) string {
	for _, pattern := range extractSimilarityTargetPatterns {
		matches := pattern.FindStringSubmatch(question)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// detectAndFetchSimilarTransactions checks if this is a similarity query and fetches results
func (hdl *Handlers) detectAndFetchSimilarTransactions(ctx context.Context, question string, ledgerID uuid.UUID) (string, bool) {
	if !isSimilarityQuery(question) {
		return "", false
	}

	target := extractSimilarityTarget(question)
	if target == "" {
		// Fall back to using the whole question as the search term
		target = question
	}

	slog.InfoContext(ctx, "detected similarity query", "target", target)

	// Generate embedding for the search target
	embedding, err := hdl.embeddingService.EmbedText(ctx, target)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to generate embedding for similarity search", "err", err)
		return "", false
	}

	// Search for similar transactions
	results, err := hdl.transactions.FindSimilarTransactions(ctx, embedding, ledgerID, 10, 0.6)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to find similar transactions", "err", err)
		return "", false
	}

	if len(results) == 0 {
		return "", false
	}

	// Format results as context for the LLM
	var sb strings.Builder
	sb.WriteString("## Similar Transactions Found (via AI Embedding Search)\n\n")
	sb.WriteString("I found these transactions that are semantically similar to the query:\n\n")

	for i, r := range results {
		txn := r.Transaction
		title := txn.DisplayTitle
		if title == "" {
			title = txn.Description
		}

		// Get entity name if available
		var entityName string
		if txn.EntityID != nil {
			if entity, err := hdl.entities.GetByID(ctx, *txn.EntityID); err == nil {
				entityName = entity.Name
			}
		}

		sb.WriteString(fmt.Sprintf("%d. **%s**", i+1, title))
		if entityName != "" && entityName != title {
			sb.WriteString(fmt.Sprintf(" (%s)", entityName))
		}
		sb.WriteString(fmt.Sprintf("\n   - Date: %s\n", txn.Date.Format("2006-01-02")))
		sb.WriteString(fmt.Sprintf("   - Similarity: %.0f%%\n", r.Similarity*100))
		if txn.PatternType != "" {
			sb.WriteString(fmt.Sprintf("   - Pattern: %s\n", txn.PatternType))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nUse this information to answer the user's question about similar transactions.\n")

	return sb.String(), true
}


// ChatAskV2Wrapper handles /chat/ask for the web frontend.
func (hdl *Handlers) ChatAskV2Wrapper(w http.ResponseWriter, r *http.Request) {
	hdl.chatV2.HandleChat(w, r)
}

// =============================================================================
// Thread Management Handlers
// =============================================================================

// ThreadListItem represents a thread in the list response (without messages)
type ThreadListItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ThreadResponse represents a full thread with messages
type ThreadResponse struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
	Messages  []MessageResponse `json:"messages"`
}

// MessageResponse represents a message in the response (no SQL exposed)
type MessageResponse struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// ChatThreadsList returns the user's chat threads for the current ledger
func (hdl *Handlers) ChatThreadsList(w http.ResponseWriter, r *http.Request) {
	user := auth.AnyCurrentUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get ledger")
		return
	}

	threads, err := hdl.chatThreads.ListThreads(r.Context(), ledger.ID, user.ID, 50)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to list threads", "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list threads")
		return
	}

	// Convert to response format
	items := make([]ThreadListItem, len(threads))
	for i, t := range threads {
		title := t.Title
		if title == "" {
			title = "New conversation"
		}
		items[i] = ThreadListItem{
			ID:        t.ID.String(),
			Title:     title,
			CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"threads": items,
	})
}

// ChatThreadsGet returns a single thread with all its messages
func (hdl *Handlers) ChatThreadsGet(w http.ResponseWriter, r *http.Request) {
	user := auth.AnyCurrentUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get ledger")
		return
	}

	threadIDStr := chi.URLParam(r, "id")
	threadID, err := uuid.Parse(threadIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid thread id")
		return
	}

	thread, err := hdl.chatThreads.GetThreadWithMessagesForUser(r.Context(), threadID, ledger.ID, user.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get thread", "id", threadIDStr, "err", err)
		respondError(w, http.StatusNotFound, "thread not found")
		return
	}

	// Convert messages to response format (no SQL exposed)
	messages := make([]MessageResponse, len(thread.Messages))
	for i, m := range thread.Messages {
		messages[i] = MessageResponse{
			ID:        m.ID.String(),
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	title := thread.Title
	if title == "" {
		title = "New conversation"
	}

	respondJSON(w, http.StatusOK, ThreadResponse{
		ID:        thread.ID.String(),
		Title:     title,
		CreatedAt: thread.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: thread.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Messages:  messages,
	})
}

// ChatThreadsDelete deletes a thread and all its messages
func (hdl *Handlers) ChatThreadsDelete(w http.ResponseWriter, r *http.Request) {
	user := auth.AnyCurrentUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get ledger")
		return
	}

	threadIDStr := chi.URLParam(r, "id")
	threadID, err := uuid.Parse(threadIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid thread id")
		return
	}

	err = hdl.chatThreads.DeleteThreadForUser(r.Context(), threadID, ledger.ID, user.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to delete thread", "id", threadIDStr, "err", err)
		respondError(w, http.StatusNotFound, "thread not found")
		return
	}

	// Check if this is an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		// Return empty response - HTMX will remove the element
		w.WriteHeader(http.StatusOK)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
	})
}

// ChatThreadsListHTML returns the thread list as HTML for HTMX
func (hdl *Handlers) ChatThreadsListHTML(w http.ResponseWriter, r *http.Request) {
	user := auth.AnyCurrentUser(r)
	if user == nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, "failed to get ledger", http.StatusInternalServerError)
		return
	}

	// Get current thread ID from URL if any
	currentThreadID := r.URL.Query().Get("t")

	threads, err := hdl.chatThreads.ListThreads(r.Context(), ledger.ID, user.ID, 50)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to list threads", "err", err)
		http.Error(w, "failed to list threads", http.StatusInternalServerError)
		return
	}

	// Convert to view model
	items := make([]pages.ThreadListItem, len(threads))
	for i, t := range threads {
		title := t.Title
		if title == "" {
			title = "New conversation"
		}
		items[i] = pages.ThreadListItem{
			ID:        t.ID.String(),
			Title:     title,
			UpdatedAt: t.UpdatedAt,
			IsActive:  t.ID.String() == currentThreadID,
		}
	}

	// Render HTML partial
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.RenderThreadList(items).Render(w); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render thread list", "err", err)
	}
}

// ChatThreadsLoadHTML loads a thread and returns messages as HTML for HTMX
func (hdl *Handlers) ChatThreadsLoadHTML(w http.ResponseWriter, r *http.Request) {
	user := auth.AnyCurrentUser(r)
	if user == nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, "failed to get ledger", http.StatusInternalServerError)
		return
	}

	threadIDStr := chi.URLParam(r, "id")
	threadID, err := uuid.Parse(threadIDStr)
	if err != nil {
		http.Error(w, "invalid thread id", http.StatusBadRequest)
		return
	}

	thread, err := hdl.chatThreads.GetThreadWithMessagesForUser(r.Context(), threadID, ledger.ID, user.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get thread", "id", threadIDStr, "err", err)
		http.Error(w, "thread not found", http.StatusNotFound)
		return
	}

	// Convert to view model - render markdown to HTML for assistant messages
	messages := make([]pages.ChatMessage, len(thread.Messages))
	for i, m := range thread.Messages {
		content := m.Content
		if m.Role == "assistant" {
			// Convert markdown to HTML for display
			content = renderMarkdownToHTML(content)
		}
		messages[i] = pages.ChatMessage{
			ID:      m.ID.String(),
			Role:    m.Role,
			Content: content,
		}
	}

	title := thread.Title
	if title == "" {
		title = "Chat"
	}

	// Render HTML partial
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.RenderChatMessages(messages, threadIDStr, title).Render(w); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render messages", "err", err)
	}
}
