package handlers

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	g "maragu.dev/gomponents"
)

// API response types

type APIError struct {
	Error string `json:"error"`
}

type APIPagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// JSON helper functions

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, APIError{Error: message})
}

func respondDeleted(w http.ResponseWriter) {
	respondJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func parseJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// mustAPIParamUUID parses a UUID from a URL path parameter.
// On failure it writes a 400 JSON error and returns false.
func mustAPIParamUUID(w http.ResponseWriter, r *http.Request, key, label string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid "+label)
		return uuid.UUID{}, false
	}
	return id, true
}

// mustParamUUID parses a UUID from a URL path parameter.
// On failure it writes a 400 plain-text error and returns false.
func mustParamUUID(w http.ResponseWriter, r *http.Request, key, label string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		http.Error(w, "Invalid "+label, http.StatusBadRequest)
		return uuid.UUID{}, false
	}
	return id, true
}

// userContext returns (email, posthog-ID) for the current request user, or ("", "") for guests.
func userContext(r *http.Request) (string, string) {
	if user := auth.CurrentUser(r); user != nil {
		return user.Email, user.ID.String()
	}
	return "", ""
}

// mustFormParamUUID parses a UUID from a POST form value.
// On failure it writes a 400 plain-text error and returns false.
func mustFormParamUUID(w http.ResponseWriter, r *http.Request, key, label string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.FormValue(key))
	if err != nil {
		http.Error(w, "Invalid "+label, http.StatusBadRequest)
		return uuid.UUID{}, false
	}
	return id, true
}

// dollarsToCents converts a dollar amount (float64) to integer cents.
func dollarsToCents(dollars float64) int64 {
	return int64(math.Round(dollars * 100))
}

// queryPage parses the "page" query parameter and returns a value >= 1.
func queryPage(r *http.Request) int {
	p, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if p < 1 {
		return 1
	}
	return p
}

// renderHTML renders a gomponents node as an HTML response, buffering the output
// so headers can be set after rendering. Writes a 500 on render failure.
func renderHTML(w http.ResponseWriter, node g.Node) bool {
	var buf bytes.Buffer
	if err := node.Render(&buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false
	}
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write(buf.Bytes())
	return true
}

// APIHandlers holds API-specific dependencies
type APIHandlers struct {
	*Handlers
	apiKeys *models.APIKeyStore
}

// NewAPIHandlers creates API handlers with the necessary stores
func NewAPIHandlers(h *Handlers) *APIHandlers {
	return &APIHandlers{
		Handlers: h,
		apiKeys:  models.NewAPIKeyStore(h.db.Pool),
	}
}

// RegisterAPIRoutes sets up the /api/v1/ routes
func (h *Handlers) RegisterAPIRoutes(r chi.Router) {
	apiHandlers := NewAPIHandlers(h)

	// Public health check — no auth required; used by MCP clients and uptime monitors.
	r.Get("/api/v1/check", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`)) //nolint:errcheck
	})

	r.Route("/api/v1", func(r chi.Router) {
		// API key authentication middleware
		r.Use(auth.APIKeyMiddleware(apiHandlers.apiKeys, h.users))

		// Dashboard / Stats
		r.Get("/dashboard", apiHandlers.APIDashboard)
		r.Get("/categorization/stats", apiHandlers.APICategorizationStats)

		// Accounts
		r.Route("/accounts", func(r chi.Router) {
			r.Get("/", apiHandlers.APIAccountsList)
			r.Post("/", apiHandlers.APIAccountsCreate)
			r.Get("/{id}", apiHandlers.APIAccountsGet)
			r.Put("/{id}", apiHandlers.APIAccountsUpdate)
			r.Delete("/{id}", apiHandlers.APIAccountsDelete)
		})

		// Transactions
		r.Route("/transactions", func(r chi.Router) {
			r.Get("/", apiHandlers.APITransactionsList)
			r.Post("/", apiHandlers.APITransactionsCreate)
			r.Get("/{id}", apiHandlers.APITransactionsGet)
			r.Put("/{id}", apiHandlers.APITransactionsUpdate)
			r.Delete("/{id}", apiHandlers.APITransactionsDelete)
			r.Post("/{id}/tags", apiHandlers.APITransactionsAddTag)
			r.Delete("/{id}/tags/{tagId}", apiHandlers.APITransactionsRemoveTag)
		})

		// Tags
		r.Route("/tags", func(r chi.Router) {
			r.Get("/", apiHandlers.APITagsList)
			r.Post("/", apiHandlers.APITagsCreate)
			r.Get("/{id}", apiHandlers.APITagsGet)
			r.Put("/{id}", apiHandlers.APITagsUpdate)
			r.Delete("/{id}", apiHandlers.APITagsDelete)
		})

		// Rules
		r.Route("/rules", func(r chi.Router) {
			r.Get("/", apiHandlers.APIRulesList)
			r.Post("/", apiHandlers.APIRulesCreate)
			r.Get("/{id}", apiHandlers.APIRulesGet)
			r.Put("/{id}", apiHandlers.APIRulesUpdate)
			r.Delete("/{id}", apiHandlers.APIRulesDelete)
			r.Post("/apply", apiHandlers.APIRulesApply)
		})

		// Patterns
		r.Route("/patterns", func(r chi.Router) {
			r.Get("/", apiHandlers.APIPatternsList)
			r.Get("/stats", apiHandlers.APIPatternsStats)
			r.Post("/detect", apiHandlers.APIPatternsDetect)
			r.Get("/{id}", apiHandlers.APIPatternsGet)
		})

		// Chat (now uses V2 tool-based handler)
		r.Route("/chat", func(r chi.Router) {
			r.Post("/ask", h.ChatAskV2Wrapper) // V2: tool-based with SSE streaming and similarity search

			// Thread management
			r.Get("/threads", h.ChatThreadsList)
			r.Get("/threads/{id}", h.ChatThreadsGet)
			r.Delete("/threads/{id}", h.ChatThreadsDelete)
		})

		// Transfers
		r.Route("/transfers", func(r chi.Router) {
			r.Get("/pending", apiHandlers.APITransfersPending)
			r.Post("/match", apiHandlers.APITransfersManualMatch)
			r.Post("/{id}/confirm", apiHandlers.APITransfersConfirm)
			r.Post("/{id}/reject", apiHandlers.APITransfersReject)
			r.Post("/{id}/unlink", apiHandlers.APITransfersUnlink)
		})

		// API Keys management
		r.Route("/api-keys", func(r chi.Router) {
			r.Get("/", apiHandlers.APIKeysList)
			r.Post("/", apiHandlers.APIKeysCreate)
			r.Delete("/{id}", apiHandlers.APIKeysDelete)
		})
	})

	// V2 API - same as V1 now, kept for backwards compatibility
	r.Route("/api/v2", func(r chi.Router) {
		r.Use(auth.APIKeyMiddleware(apiHandlers.apiKeys, h.users))

		r.Route("/chat", func(r chi.Router) {
			r.Post("/ask", h.ChatAskV2Wrapper)
		})
	})
}

// Helper to get current ledger for API user
func (h *APIHandlers) getAPILedger(r *http.Request) (*models.Ledger, error) {
	user := auth.APICurrentUser(r)
	if user == nil {
		return nil, models.ErrUnauthorized
	}

	// Use the same logic as getCurrentLedger
	return h.getCurrentLedger(r)
}

func (h *APIHandlers) requireAPILedger(w http.ResponseWriter, r *http.Request) (*models.Ledger, bool) {
	ledger, err := h.getAPILedger(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	return ledger, true
}

// Dashboard endpoint
func (h *APIHandlers) APIDashboard(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	accounts, err := h.accounts.GetWithBalances(r.Context(), ledger.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Calculate totals by type
	var totalAssets, totalLiabilities int64
	assetAccounts := make([]*models.Account, 0)
	liabilityAccounts := make([]*models.Account, 0)

	for _, acc := range accounts {
		switch acc.Type {
		case models.AccountTypeAsset:
			totalAssets += acc.Balance
			assetAccounts = append(assetAccounts, acc)
		case models.AccountTypeLiability:
			// Liability balances are stored as negative (credit balances)
			totalLiabilities += acc.Balance
			liabilityAccounts = append(liabilityAccounts, acc)
		}
	}

	// Net worth = Assets - abs(Liabilities)
	// Since liabilities are negative, netWorth = assets + liabilities
	netWorth := totalAssets + totalLiabilities

	respondJSON(w, http.StatusOK, map[string]any{
		"net_worth":          netWorth,
		"total_assets":       totalAssets,
		"total_liabilities":  -totalLiabilities, // Return as positive for display
		"asset_accounts":     assetAccounts,
		"liability_accounts": liabilityAccounts,
	})
}

// Categorization stats endpoint
func (h *APIHandlers) APICategorizationStats(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	stats, err := h.transactions.GetCategorizationQueueStats(r.Context(), ledger.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// API Keys endpoints

func (h *APIHandlers) APIKeysList(w http.ResponseWriter, r *http.Request) {
	user := auth.APICurrentUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	keys, err := h.apiKeys.GetByUserID(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Don't return the hash
	result := make([]map[string]any, len(keys))
	for i, k := range keys {
		result[i] = map[string]any{
			"id":           k.ID,
			"name":         k.Name,
			"key_prefix":   k.KeyPrefix,
			"last_used_at": k.LastUsedAt,
			"created_at":   k.CreatedAt,
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{"data": result})
}

type createAPIKeyRequest struct {
	Name string `json:"name"`
}

func (h *APIHandlers) APIKeysCreate(w http.ResponseWriter, r *http.Request) {
	user := auth.APICurrentUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createAPIKeyRequest
	if err := parseJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Generate the key
	plaintext, key, err := models.GenerateAPIKey(user.ID, req.Name)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate key")
		return
	}

	// Store it
	if err := h.apiKeys.Create(r.Context(), key); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return the plaintext key (only time it's shown)
	respondJSON(w, http.StatusCreated, map[string]any{
		"id":         key.ID,
		"name":       key.Name,
		"key":        plaintext, // Only returned once!
		"key_prefix": key.KeyPrefix,
		"created_at": key.CreatedAt,
		"message":    "Save this key now. It will not be shown again.",
	})
}

func (h *APIHandlers) APIKeysDelete(w http.ResponseWriter, r *http.Request) {
	user := auth.APICurrentUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	keyID, ok := mustAPIParamUUID(w, r, "id", "key ID")
	if !ok {
		return
	}

	// Verify ownership
	key, err := h.apiKeys.GetByID(r.Context(), keyID)
	if err != nil {
		respondError(w, http.StatusNotFound, "key not found")
		return
	}

	if key.UserID != user.ID {
		respondError(w, http.StatusForbidden, "not your key")
		return
	}

	if err := h.apiKeys.Delete(r.Context(), keyID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondDeleted(w)
}
