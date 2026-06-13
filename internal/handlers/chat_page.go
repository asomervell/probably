package handlers

import (
	"net/http"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/views/pages"
	"github.com/google/uuid"
)

// ChatPage renders the chat interface page
func (hdl *Handlers) ChatPage(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check for thread ID in URL
	threadID := r.URL.Query().Get("t")
	threadTitle := ""
	var messages []pages.ChatMessage

	if threadID != "" {
		// Try to load the thread with messages
		if tid, err := uuid.Parse(threadID); err == nil {
			if thread, err := hdl.chatThreads.GetThreadWithMessagesForUser(r.Context(), tid, ledger.ID, user.ID); err == nil {
				threadTitle = thread.Title
				if threadTitle == "" {
					threadTitle = "Chat"
				}
				// Convert messages to view model - render markdown to HTML for assistant messages
				for _, m := range thread.Messages {
					content := m.Content
					if m.Role == "assistant" {
						// Convert markdown to HTML for display
						content = renderMarkdownToHTML(content)
					}
					messages = append(messages, pages.ChatMessage{
						ID:      m.ID.String(),
						Role:    m.Role,
						Content: content,
					})
				}
			} else {
				// Thread not found, clear the ID
				threadID = ""
			}
		} else {
			threadID = ""
		}
	}

	// Render chat page
	renderHTML(w, pages.RenderChat(user.Email, user.ID.String(), ledger.ID.String(), threadID, threadTitle, messages))
}

