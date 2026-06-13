package handlers

import (
	"log/slog"
	"net/http"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// Rule API response type
type RuleResponse struct {
	ID           uuid.UUID `json:"id"`
	LedgerID     uuid.UUID `json:"ledger_id"`
	Name         string    `json:"name"`
	Prompt       string    `json:"prompt"`
	Examples     string    `json:"examples,omitempty"`
	MatchPattern string    `json:"match_pattern,omitempty"`
	IsRegex      bool      `json:"is_regex"`
	TagID        uuid.UUID `json:"tag_id"`
	TagName      string    `json:"tag_name"`
	TagColor     string    `json:"tag_color"`
	Priority     int       `json:"priority"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

func ruleToResponse(rule *models.CategorizationRule) RuleResponse {
	return RuleResponse{
		ID:           rule.ID,
		LedgerID:     rule.LedgerID,
		Name:         rule.Name,
		Prompt:       rule.Prompt,
		Examples:     rule.Examples,
		MatchPattern: rule.MatchPattern,
		IsRegex:      rule.IsRegex,
		TagID:        rule.TagID,
		TagName:      rule.TagName,
		TagColor:     rule.TagColor,
		Priority:     rule.Priority,
		IsActive:     rule.IsActive,
		CreatedAt:    rule.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    rule.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// APIRulesList returns all categorization rules for the current ledger
func (h *APIHandlers) APIRulesList(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	// Check if only active rules are requested
	activeOnly := r.URL.Query().Get("active_only") == "true"

	var (
		rules []*models.CategorizationRule
		err   error
	)
	if activeOnly {
		rules, err = h.rules.GetActiveRules(r.Context(), ledger.ID)
	} else {
		rules, err = h.rules.GetByLedgerID(r.Context(), ledger.ID)
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response format
	result := make([]RuleResponse, len(rules))
	for i, rule := range rules {
		result[i] = ruleToResponse(rule)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": result,
	})
}

type createRuleRequest struct {
	Name         string    `json:"name"`
	Prompt       string    `json:"prompt"`
	Examples     string    `json:"examples,omitempty"`
	MatchPattern string    `json:"match_pattern,omitempty"`
	IsRegex      bool      `json:"is_regex,omitempty"`
	TagID        uuid.UUID `json:"tag_id"`
	Priority     int       `json:"priority,omitempty"`
}

// APIRulesCreate creates a new categorization rule
func (h *APIHandlers) APIRulesCreate(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	var req createRuleRequest
	if err := parseJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.TagID == uuid.Nil {
		respondError(w, http.StatusBadRequest, "tag_id is required")
		return
	}

	// Verify tag belongs to this ledger
	tag, err := h.tags.GetByID(r.Context(), req.TagID)
	if err != nil || tag.LedgerID != ledger.ID {
		respondError(w, http.StatusBadRequest, "invalid tag_id")
		return
	}

	rule := &models.CategorizationRule{
		LedgerID:     ledger.ID,
		Name:         req.Name,
		Prompt:       req.Prompt,
		Examples:     req.Examples,
		MatchPattern: req.MatchPattern,
		IsRegex:      req.IsRegex,
		TagID:        req.TagID,
		Priority:     req.Priority,
		IsActive:     true,
		TagName:      tag.Name,
		TagColor:     tag.Color,
	}

	if err := h.rules.Create(r.Context(), rule); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, ruleToResponse(rule))
}

// APIRulesGet returns a single rule by ID
func (h *APIHandlers) APIRulesGet(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	ruleID, ok := mustAPIParamUUID(w, r, "id", "rule ID")
	if !ok {
		return
	}

	rule, ok := h.getOwnedRule(w, r, ruleID, ledger.ID)
	if !ok {
		return
	}

	respondJSON(w, http.StatusOK, ruleToResponse(rule))
}

type updateRuleRequest struct {
	Name         *string    `json:"name,omitempty"`
	Prompt       *string    `json:"prompt,omitempty"`
	Examples     *string    `json:"examples,omitempty"`
	MatchPattern *string    `json:"match_pattern,omitempty"`
	IsRegex      *bool      `json:"is_regex,omitempty"`
	TagID        *uuid.UUID `json:"tag_id,omitempty"`
	Priority     *int       `json:"priority,omitempty"`
	IsActive     *bool      `json:"is_active,omitempty"`
}

// APIRulesUpdate updates a rule
func (h *APIHandlers) APIRulesUpdate(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	ruleID, ok := mustAPIParamUUID(w, r, "id", "rule ID")
	if !ok {
		return
	}

	rule, ok := h.getOwnedRule(w, r, ruleID, ledger.ID)
	if !ok {
		return
	}

	var req updateRuleRequest
	if err := parseJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Prompt != nil {
		rule.Prompt = *req.Prompt
	}
	if req.Examples != nil {
		rule.Examples = *req.Examples
	}
	if req.MatchPattern != nil {
		rule.MatchPattern = *req.MatchPattern
	}
	if req.IsRegex != nil {
		rule.IsRegex = *req.IsRegex
	}
	if req.TagID != nil {
		// Verify tag belongs to this ledger
		tag, err := h.tags.GetByID(r.Context(), *req.TagID)
		if err != nil || tag.LedgerID != ledger.ID {
			respondError(w, http.StatusBadRequest, "invalid tag_id")
			return
		}
		rule.TagID = *req.TagID
		rule.TagName = tag.Name
		rule.TagColor = tag.Color
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.IsActive != nil {
		rule.IsActive = *req.IsActive
	}

	if err := h.rules.Update(r.Context(), rule); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, ruleToResponse(rule))
}

// APIRulesDelete deletes a rule
func (h *APIHandlers) APIRulesDelete(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	ruleID, ok := mustAPIParamUUID(w, r, "id", "rule ID")
	if !ok {
		return
	}

	rule, ok := h.getOwnedRule(w, r, ruleID, ledger.ID)
	if !ok {
		return
	}

	if err := h.rules.Delete(r.Context(), rule.ID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondDeleted(w)
}

// APIRulesApply applies active rules to uncategorized transactions
func (h *APIHandlers) APIRulesApply(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	// Get active rules
	rules, err := h.rules.GetActiveRules(r.Context(), ledger.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(rules) == 0 {
		respondJSON(w, http.StatusOK, map[string]any{
			"matched": 0,
			"message": "no active rules found",
		})
		return
	}

	// Get uncategorized transactions (those without tags)
	filter := models.TransactionFilter{
		LedgerID: ledger.ID,
		Limit:    1000, // Process in batches
	}
	isTransfer := false
	filter.IsTransfer = &isTransfer

	transactions, _, err := h.transactions.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	matched := 0
	for _, txn := range transactions {
		// Load tags to check if already categorized
		if err := h.transactions.LoadTags(r.Context(), txn); err != nil {
			slog.WarnContext(r.Context(), "failed to load tags for rule matching", "transaction_id", txn.ID, "err", err)
		}
		if len(txn.Tags) > 0 {
			continue // Already has tags
		}

		// Try each rule (sorted by priority)
		// Prefer LLM-based rules (prompt field) over pattern matching
		for _, rule := range rules {
			// Skip legacy pattern matching if prompt is available (LLM-based rule)
			if rule.Prompt != "" {
				continue // LLM-based rules are handled by the categorization worker
			}
			// Fallback to legacy pattern matching for backward compatibility
			if rule.MatchPattern != "" && rule.Match(txn.Description) {
				// Apply the tag
				if err := h.tags.CategorizeTransaction(r.Context(), txn.ID, rule.TagID); err == nil {
					matched++
				}
				break // Only apply first matching rule
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"matched":   matched,
		"processed": len(transactions),
	})
}

func (h *APIHandlers) getOwnedRule(w http.ResponseWriter, r *http.Request, id, ledgerID uuid.UUID) (*models.CategorizationRule, bool) {
	rule, err := h.rules.GetByID(r.Context(), id)
	if err != nil || rule.LedgerID != ledgerID {
		respondError(w, http.StatusNotFound, "rule not found")
		return nil, false
	}
	return rule, true
}
