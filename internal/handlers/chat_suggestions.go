package handlers

import (
	"log/slog"
	"net/http"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/chat"
)

// ChatSuggestions returns suggested questions for the user
func (hdl *Handlers) ChatSuggestions(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	if user == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get ledger"})
		return
	}

	// Generate suggestions
	generator := chat.NewSuggestionsGenerator(hdl.db.Pool)
	suggestions, err := generator.GenerateSuggestions(r.Context(), ledger.ID, 6)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to generate suggestions", "err", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate suggestions"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"suggestions": suggestions,
	})
}
