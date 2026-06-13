package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/categorize"
	"github.com/asomervell/probably/internal/chat"
	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/embedding"
	"github.com/asomervell/probably/internal/enrichment"
	"github.com/asomervell/probably/internal/llm"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/sync"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
)

type Handlers struct {
	cfg          *config.Config
	db           *db.DB
	ab           *authboss.Authboss
	sessionStore *sessions.CookieStore

	// Stores
	users          *models.UserStore
	ledgers        *models.LedgerStore
	accounts       *models.AccountStore
	transactions   *models.TransactionStore
	tags           *models.TagStore
	rules          *models.RuleStore
	pendingMatches *models.PendingMatchStore
	entities       *models.EntityStore
	reports        *models.ReportStore
	insights       *models.InsightStore
	permissions    *models.PermissionStore
	passkeys       *models.PasskeyStore

	// Permission checker
	permissionChecker *auth.PermissionChecker

	// Services
	transferMatcher *sync.TransferMatcher
	taxonomy        *categorize.TaxonomyService
	firecrawl       *enrichment.FirecrawlClient
	logoClient      *enrichment.LogoClient

	// WebAuthn for passkey authentication
	webauthn *auth.WebAuthnService

	// Chat
	chatContext *chat.ContextManager
	chatThreads *chat.ThreadStore

	// V2 Chat (tool-based with embeddings)
	chatV2 *llm.ChatHandler

	// Embedding service for similarity search (shared between v1 and v2 chat)
	embeddingService *embedding.Service

	// Teller client and sync service — initialized once at startup to avoid per-request TLS cert parsing
	tellerClient      *sync.TellerClient
	tellerSyncService *sync.TellerSyncService
}

func New(cfg *config.Config, database *db.DB, ab *authboss.Authboss, sessionStore *sessions.CookieStore) *Handlers {
	logoClient, err := enrichment.NewLogoClient(cfg)
	if err != nil {
		slog.Warn("failed to initialize logo client", "err", err)
		logoClient = nil
	}

	// Initialize Firecrawl cache
	firecrawlCache := enrichment.NewFirecrawlCache(database.Pool)
	firecrawl := enrichment.NewFirecrawlClientWithCache(cfg, firecrawlCache)

	entityStore := models.NewEntityStore(database.Pool)
	permissionStore := models.NewPermissionStore(database.Pool)
	permissionChecker := auth.NewPermissionChecker(permissionStore, entityStore)

	// Stores (some created here for v2 chat, reused below)
	transactionStore := models.NewTransactionStore(database.Pool)
	accountStore := models.NewAccountStore(database.Pool)
	tagStore := models.NewTagStore(database.Pool)
	ruleStore := models.NewRuleStore(database.Pool)
	ledgerStore := models.NewLedgerStore(database.Pool)
	recurringPatternStore := models.NewRecurringPatternStore(database.Pool)
	relationshipStore := models.NewEntityRelationshipStore(database.Pool)

	// Initialize v2 chat handler (tool-based with embedding support)
	chatV2Handler := llm.NewChatHandler(
		cfg,
		transactionStore,
		accountStore,
		tagStore,
		ruleStore,
		recurringPatternStore,
		entityStore,
		relationshipStore,
		ledgerStore,
	)

	// Initialize thread store for chat persistence
	threadStore := chat.NewThreadStore(database.Pool)

	// Set thread store on V2 chat handler for persistence (with adapter)
	chatV2Handler.SetThreadStore(&threadStoreAdapter{store: threadStore})

	// Initialize WebAuthn service for passkey authentication
	var webauthnService *auth.WebAuthnService
	webauthnService, err = auth.NewWebAuthnService(cfg, sessionStore)
	if err != nil {
		slog.Warn("failed to initialize WebAuthn service", "err", err)
		webauthnService = nil
	}

	// Initialize Teller client once to avoid per-request TLS certificate parsing
	tellerClient, err := sync.NewTellerClient(cfg)
	if err != nil {
		slog.Warn("failed to initialize Teller client — Teller sync disabled", "err", err)
		tellerClient = nil
	}

	var tellerSyncService *sync.TellerSyncService
	if tellerClient != nil {
		tellerSyncService = sync.NewTellerSyncService(database.Pool, tellerClient, cfg)
	}

	return &Handlers{
		cfg:          cfg,
		db:           database,
		ab:           ab,
		sessionStore: sessionStore,

		users:          models.NewUserStore(database.Pool),
		ledgers:        ledgerStore,
		accounts:       accountStore,
		transactions:   transactionStore,
		tags:           tagStore,
		rules:          ruleStore,
		pendingMatches: models.NewPendingMatchStore(database.Pool),
		entities:       entityStore,
		reports:        models.NewReportStore(database.Pool),
		insights:       models.NewInsightStore(database.Pool),
		permissions:    permissionStore,
		passkeys:       models.NewPasskeyStore(database.Pool),

		transferMatcher: sync.NewTransferMatcher(database.Pool),
		taxonomy:        categorize.NewTaxonomyService(database.Pool),
		firecrawl:       firecrawl,
		logoClient:      logoClient,

		// WebAuthn for passkeys
		webauthn: webauthnService,

		// Chat context manager (in-memory, 10 messages max, 1 hour TTL)
		chatContext:       chat.NewContextManager(10, time.Hour),
		chatThreads:       threadStore,
		permissionChecker: permissionChecker,

		// V2 chat handler
		chatV2: chatV2Handler,

		tellerClient:      tellerClient,
		tellerSyncService: tellerSyncService,
	}
}

// SetChatV2EmbeddingService sets the embedding service for chat handlers
// This enables similarity search in both V1 (enhanced) and V2 (tool-based) chat
func (h *Handlers) SetChatV2EmbeddingService(svc *embedding.Service) {
	h.embeddingService = svc
	if h.chatV2 != nil {
		h.chatV2.SetEmbeddingService(svc)
	}
}

// NewForTesting creates handlers without authboss for API-only testing
func NewForTesting(cfg *config.Config, database *db.DB) *Handlers {
	logoClient, err := enrichment.NewLogoClient(cfg)
	if err != nil {
		slog.Warn("failed to initialize logo client", "err", err)
		logoClient = nil
	}

	// Initialize Firecrawl cache
	firecrawlCache := enrichment.NewFirecrawlCache(database.Pool)
	firecrawl := enrichment.NewFirecrawlClientWithCache(cfg, firecrawlCache)

	entityStore := models.NewEntityStore(database.Pool)
	permissionStore := models.NewPermissionStore(database.Pool)
	permissionChecker := auth.NewPermissionChecker(permissionStore, entityStore)

	return &Handlers{
		cfg: cfg,
		db:  database,

		users:          models.NewUserStore(database.Pool),
		ledgers:        models.NewLedgerStore(database.Pool),
		accounts:       models.NewAccountStore(database.Pool),
		transactions:   models.NewTransactionStore(database.Pool),
		tags:           models.NewTagStore(database.Pool),
		rules:          models.NewRuleStore(database.Pool),
		pendingMatches: models.NewPendingMatchStore(database.Pool),
		entities:       entityStore,
		reports:        models.NewReportStore(database.Pool),
		insights:       models.NewInsightStore(database.Pool),
		permissions:    permissionStore,
		passkeys:       models.NewPasskeyStore(database.Pool),

		transferMatcher: sync.NewTransferMatcher(database.Pool),
		taxonomy:        categorize.NewTaxonomyService(database.Pool),
		firecrawl:       firecrawl,
		logoClient:      logoClient,

		// Chat context manager (in-memory, 10 messages max, 1 hour TTL)
		chatContext:       chat.NewContextManager(10, time.Hour),
		chatThreads:       chat.NewThreadStore(database.Pool),
		permissionChecker: permissionChecker,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// Authboss routes
	r.Group(func(r chi.Router) {
		r.Use(h.ab.LoadClientStateMiddleware)
		r.Mount("/auth", http.StripPrefix("/auth", h.ab.Config.Core.Router))
	})

	// Passkey login routes (public - no auth required)
	r.Group(func(r chi.Router) {
		r.Use(h.ab.LoadClientStateMiddleware)
		r.Post("/auth/passkey/login/begin", h.BeginPasskeyLogin)
		r.Post("/auth/passkey/login/finish", h.FinishPasskeyLogin)
		r.Get("/auth/passkey/check", h.HasPasskeys)
	})

	// Public API routes (no auth required - for webhooks)
	r.Route("/api", func(r chi.Router) {
		r.Post("/teller/webhook", h.TellerWebhook)
		r.Post("/akahu/webhook", h.AkahuWebhook)
		r.Post("/plaid/webhook", h.PlaidWebhook)
		r.Post("/stripe/webhook", h.StripeWebhook)
	})

	// JSON API routes (API key auth)
	h.RegisterAPIRoutes(r)

	// Public routes with optional auth (for marketing pages)
	r.Group(func(r chi.Router) {
		r.Use(h.ab.LoadClientStateMiddleware)
		r.Use(auth.LoadUserMiddleware(h.ab, h.users))
		r.Get("/", h.Home)
		r.Get("/blog", h.BlogIndex)
		r.Get("/blog/{slug}", h.BlogPost)
		r.Get("/legal", h.LegalIndex)
		r.Get("/legal/{slug}", h.LegalPage)

		// Demo panel routes (public, for homepage)
		r.Route("/demo", func(r chi.Router) {
			r.Get("/pulse", h.DemoPulse)
			r.Get("/transactions", h.DemoTransactions)
			r.Get("/chat", h.DemoChat)
			r.Post("/chat/message", h.DemoChatMessage)
		})
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(h.ab.LoadClientStateMiddleware)
		r.Use(auth.LoadUserMiddleware(h.ab, h.users))
		r.Use(auth.RequireAuth(h.ab))

		// Subscription gating when billing is enabled (Stripe checkout, etc.)
		r.Group(func(r chi.Router) {
			if h.cfg.BillingEnabled {
				subscriptionStore := models.NewSubscriptionStore(h.db.Pool)
				r.Use(auth.RequireSubscriptionOrTrial(subscriptionStore))
			}

			// Accounts
			r.Route("/accounts", func(r chi.Router) {
				r.Get("/", h.AccountsList)
				r.Get("/new", h.AccountsNew)
				r.Get("/new/manual", h.AccountsNewManual)
				r.Get("/{accountID}/statements/upload", h.StatementsUploadForm)
				r.Post("/{accountID}/statements/upload", h.StatementsUploadSingle)                     // Single file upload
				r.Get("/{accountID}/statements/upload/{uploadID}/status", h.StatementsUploadStatus)    // Status polling
				r.Post("/{accountID}/statements/upload/{uploadID}/requeue", h.StatementsUploadRequeue) // Re-queue transactions
				r.Delete("/{accountID}/statements/upload/{uploadID}", h.StatementsUploadDelete)
				r.Delete("/{accountID}/statements/upload/failed", h.StatementsUploadDeleteFailed)
				r.Post("/", h.AccountsCreate)
				r.Get("/{id}", h.AccountsShow)
				r.Get("/{id}/edit", h.AccountsEdit)
				r.Put("/{id}", h.AccountsUpdate)
				r.Post("/{id}", h.AccountsUpdate) // Support form POST with _method=PUT
				r.Delete("/{id}", h.AccountsDelete)
			})

			// Transactions
			r.Route("/transactions", func(r chi.Router) {
				r.Get("/", h.TransactionsList)
				r.Get("/content", h.TransactionsListContent) // HTMX lazy-loaded content
				r.Get("/new", h.TransactionsNew)
				r.Post("/", h.TransactionsCreate)
				r.Get("/{id}", h.TransactionsShow)
				r.Get("/{id}/edit", h.TransactionsEdit)
				r.Put("/{id}", h.TransactionsUpdate)
				r.Post("/{id}", h.TransactionsUpdate) // Support form POST with _method=PUT
				r.Delete("/{id}", h.TransactionsDelete)

				// Tagging
				r.Post("/{id}/tags", h.TransactionsAddTag)
				r.Delete("/{id}/tags/{tagId}", h.TransactionsRemoveTag)
				r.Post("/{id}/tags/{tagId}", h.TransactionsRemoveTag) // Support form POST with _method=DELETE

				// Recategorize
				r.Post("/{id}/recategorize", h.TransactionsRecategorize)

				// Bulk operations
				r.Post("/bulk/tag", h.TransactionsBulkTag)
				r.Post("/bulk/recategorize", h.TransactionsBulkRecategorize)
				r.Post("/bulk/mark-reviewed", h.TransactionsBulkMarkReviewed)
			})

			// Tags
			r.Route("/tags", func(r chi.Router) {
				r.Get("/", h.TagsList)
				r.Get("/new", h.TagsNew)
				r.Post("/", h.TagsCreate)
				r.Get("/{id}/edit", h.TagsEdit)
				r.Put("/{id}", h.TagsUpdate)
				r.Post("/{id}", h.TagsUpdate) // Support form POST with _method=PUT
				r.Delete("/{id}", h.TagsDelete)
			})

			// Entities
			r.Route("/entities", func(r chi.Router) {
				r.Get("/", h.EntitiesList)
				r.Get("/search", h.EntitiesSearch)
				r.Get("/{id}", h.EntitiesShow)
				r.Patch("/{id}", h.EntitiesUpdate)
				r.Post("/{id}/update", h.EntitiesUpdate)                    // Support form POST
				r.Get("/{id}/enrich/search", h.EntitiesEnrichSearch)        // Search for enrichment options
				r.Post("/{id}/enrich", h.EntitiesEnrich)                    // Firecrawl enrichment
				r.Post("/{id}/clear-enrichment", h.EntitiesClearEnrichment) // Clear enrichment data
				r.Get("/{id}/merge", h.EntitiesMerge)                       // Merge page
				r.Post("/{id}/merge", h.EntitiesMergeConfirm)               // Merge confirmation
				r.Delete("/{id}", h.EntitiesDelete)
				r.Post("/{id}/delete", h.EntitiesDelete) // Support form POST with _method=DELETE
			})

			// Pulse (forward-looking dashboard - the new hero page)
			r.Get("/pulse", h.Pulse)

			// Statements (Balance Sheet + P&L - renamed from Intelligence)
			r.Get("/statements", h.Statements)

			// Intelligence (AI insights - renamed from Insights)
			r.Route("/intelligence", func(r chi.Router) {
				r.Get("/", h.InsightsList)
				r.Get("/reports/{id}", h.InsightsReport)
				r.Post("/{id}/dismiss", h.InsightsDismiss)
			})

			// Redirects for old URLs
			r.Get("/insights", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/intelligence", http.StatusMovedPermanently)
			})
			r.Get("/insights/reports/{id}", func(w http.ResponseWriter, r *http.Request) {
				id := chi.URLParam(r, "id")
				http.Redirect(w, r, "/intelligence/reports/"+id, http.StatusMovedPermanently)
			})

			// Transfers
			r.Route("/transfers", func(r chi.Router) {
				r.Get("/", h.TransfersList)
				r.Post("/rematch", h.TransfersRematch) // Re-run transfer matching on all unmatched transactions
				r.Post("/{id}/confirm", h.TransfersConfirm)
				r.Post("/{id}/reject", h.TransfersReject)
				r.Post("/match", h.TransfersManualMatch)
				r.Post("/{id}/unlink", h.TransfersUnlink)
			})

			// Patterns (recurring charge tracker)
			r.Route("/patterns", func(r chi.Router) {
				r.Get("/", h.PatternsList)
				r.Get("/detail", h.PatternsDetail)    // Query params: ?entity=uuid&type=recurring_bill&name=...
				r.Post("/dismiss", h.PatternsDismiss) // Form params: entity, type, name
				r.Post("/rename", h.PatternsRename)   // Form params: entity, type, old_name, new_name
				r.Post("/refresh", h.PatternsRefresh)
			})

			// Generic provider connections
			r.Route("/connections", func(r chi.Router) {
				// Teller (US)
				r.Get("/connect/teller", h.TellerConnect)
				r.Post("/callback/teller", h.TellerCallback)
				r.Post("/sync/teller", h.TellerSync)
				r.Post("/sync-multi", h.TellerSyncMulti)
				r.Post("/resync/teller", h.TellerFullResync)
				r.Post("/resync-multi", h.TellerFullResyncMulti)
				r.Delete("/disconnect/teller/{connectionId}", h.TellerDisconnect) // Standardized: connectionId (also accepts enrollmentId for backwards compatibility)
				r.Post("/disconnect/teller/{connectionId}", h.TellerDisconnect)   // Support HTML forms posting _method=DELETE
				r.Post("/disconnect-multi", h.TellerDisconnectMulti)
				r.Get("/link/teller/{accountId}", h.TellerLink)
				r.Post("/link-callback/teller", h.TellerLinkCallback)
				r.Get("/reconnect/teller", h.TellerReconnect)

				// Akahu (NZ)
				r.Get("/connect/akahu", h.AkahuConnect)
				r.Get("/callback/akahu", h.AkahuCallback)
				r.Post("/sync/akahu", h.AkahuSync)
				r.Post("/sync/akahu/{connectionId}", h.AkahuSync)
				r.Delete("/disconnect/akahu/{connectionId}", h.AkahuDisconnect)
				r.Post("/disconnect/akahu/{connectionId}", h.AkahuDisconnect) // Support HTML forms posting _method=DELETE

				// Plaid (US)
				r.Get("/connect/plaid", h.PlaidConnect)
				r.Get("/callback/plaid", h.PlaidCallback)
				r.Post("/sync/plaid", h.PlaidSync)
				r.Post("/sync/plaid/{connectionId}", h.PlaidSync)               // Standardized: connectionId (also accepts itemId for backwards compatibility)
				r.Post("/resync/plaid/{connectionId}", h.PlaidFullResync)       // Standardized: connectionId (also accepts itemId for backwards compatibility)
				r.Delete("/disconnect/plaid/{connectionId}", h.PlaidDisconnect) // Standardized: connectionId (also accepts itemId for backwards compatibility)
				r.Post("/disconnect/plaid/{connectionId}", h.PlaidDisconnect)   // Support HTML forms posting _method=DELETE
				r.Get("/link/plaid/{accountId}", h.PlaidLink)
				r.Get("/reconnect/plaid", h.PlaidReconnect)
			})

			// Teller routes (legacy/compatibility - maps to same handlers as /connections)
			r.Route("/teller", func(r chi.Router) {
				r.Get("/connect", h.TellerConnect)
				r.Post("/callback", h.TellerCallback)
				r.Post("/sync", h.TellerSync)
				r.Post("/sync/{id}", h.TellerSync) // Handler accepts id as enrollmentId or connectionId
				r.Post("/sync-multi", h.TellerSyncMulti)
				r.Post("/resync", h.TellerFullResync)
				r.Post("/resync-multi", h.TellerFullResyncMulti)
				r.Delete("/disconnect/{id}", h.TellerDisconnect) // Handler accepts id as enrollmentId or connectionId
				r.Post("/disconnect/{id}", h.TellerDisconnect)   // Support HTML forms posting _method=DELETE
				r.Post("/disconnect-multi", h.TellerDisconnectMulti)
				r.Get("/link/{accountId}", h.TellerLink)
				r.Post("/link-callback", h.TellerLinkCallback)
				r.Get("/reconnect", h.TellerReconnect)
			})

			// AI categorization
			r.Post("/categorize", h.AICategorizeBatch)
			r.Post("/recategorize-by-tag", h.RecategorizeByTagName)

			// Chat (V2 - tool-based with SSE streaming)
			r.Route("/chat", func(r chi.Router) {
				r.Get("/", h.ChatPage)
				r.Post("/ask", h.ChatAskV2Wrapper)
				r.Get("/suggestions", h.ChatSuggestions)

				// Voice chat (WebSocket)
				r.Get("/voice", h.VoiceChat)

				// Thread management (JSON API)
				r.Get("/threads", h.ChatThreadsList)
				r.Get("/threads/{id}", h.ChatThreadsGet)
				r.Delete("/threads/{id}", h.ChatThreadsDelete)

				// Thread management (HTMX HTML partials)
				r.Get("/threads/list", h.ChatThreadsListHTML)
				r.Get("/threads/{id}/load", h.ChatThreadsLoadHTML)
			})
		})

		// Routes that don't require subscription (billing, settings, etc.)

		// Passkey registration routes (require auth)
		r.Post("/auth/passkey/register/begin", h.BeginPasskeyRegistration)
		r.Post("/auth/passkey/register/finish", h.FinishPasskeyRegistration)

		// Settings
		r.Route("/settings", func(r chi.Router) {
			r.Get("/", h.Settings)
			r.Get("/profile", h.SettingsProfile)
			r.Get("/preferences", h.SettingsPreferences)
			r.Get("/security", h.SettingsSecurity)
			r.Delete("/security/passkeys/{id}", h.DeletePasskey)
			r.Post("/security/passkeys/{id}", h.DeletePasskey) // Support form POST with _method=DELETE
			r.Get("/my-life", h.SettingsMyLife)
			r.Post("/my-life/relationship", h.SettingsAddRelationship)
			r.Post("/my-life/relationship/{id}/delete", h.SettingsDeleteRelationship)
			r.Get("/banks", h.SettingsBanks)
			r.Post("/banks/account/{id}/delete", h.SettingsDeleteBankAccount)
			r.Post("/banks/institution/delete", h.SettingsDeleteInstitutionAccounts)
			r.Get("/ledger", h.SettingsLedger)
			r.Get("/categories", h.SettingsCategories)
			r.Put("/profile", h.SettingsUpdateProfile)
			r.Put("/password", h.SettingsUpdatePassword)
			r.Post("/seed-tags", h.SettingsSeedTags)

			// Billing
			r.Get("/billing", h.SettingsBilling)
			r.Get("/billing/subscribe", h.SettingsBillingSubscribe)
			r.Post("/billing/subscribe", h.SettingsBillingSubscribePost)
			r.Get("/billing/callback", h.SettingsBillingCallback)
			r.Get("/billing/portal", h.SettingsBillingPortal)
			r.Post("/billing/cleanup", h.SettingsBillingCleanup)

			// Backup
			r.Get("/backup", h.SettingsBackup)
			r.Get("/backup/download", h.SettingsBackupDownload)
			r.Get("/backup/import", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/settings/backup", http.StatusSeeOther)
			})
			r.Post("/backup/import", h.SettingsBackupImport)

			// Danger Zone
			r.Get("/danger", h.SettingsDangerZone)
			r.Post("/danger/delete-data", h.SettingsDeleteData)
			r.Post("/danger/delete-account", h.SettingsDeleteAccount)
			r.Post("/danger/reprocess-all", h.SettingsReprocessAll)
			r.Post("/danger/reprocess-patterns", h.SettingsReprocessPatterns)
			r.Post("/danger/recalculate-balances", h.SettingsRecalculateBalances)
		})
	})
}

// getCurrentLedger gets the first ledger the user has access to, or creates a default one
// If a new ledger is created, it automatically seeds default tags
func (h *Handlers) getCurrentLedger(r *http.Request) (*models.Ledger, error) {
	user := auth.CurrentUser(r)
	if user == nil {
		return nil, models.ErrUnauthorized
	}

	// Get all ledgers user has access to
	ledgerPerms, err := h.permissionChecker.GetUserLedgerPermissions(r.Context(), user.ID)
	if err != nil {
		return nil, err
	}

	// If user has no ledgers, create a default one
	if len(ledgerPerms) == 0 {
		// Get user's person entity (should exist after migration)
		userPerms, err := h.permissions.GetUserEntityPermissions(r.Context(), user.ID)
		if err != nil {
			return nil, err
		}

		var personEntityID *uuid.UUID
		for _, perm := range userPerms {
			entity, err := h.entities.GetByID(r.Context(), perm.EntityID)
			if err == nil && entity.Type == models.EntityTypePerson && entity.Subtype == "individual" {
				personEntityID = &perm.EntityID
				break
			}
		}

		if personEntityID == nil {
			// Self-heal legacy/misaligned accounts by creating a default person entity.
			personEntity := &models.Entity{
				Type:           models.EntityTypePerson,
				Subtype:        models.PersonSubtypeIndividual,
				Name:           user.Email,
				ExternalSource: "system",
				UserVerified:   true,
			}
			if err := h.entities.Create(r.Context(), personEntity); err != nil {
				return nil, fmt.Errorf("create fallback person entity: %w", err)
			}
			userPerm := &models.UserEntityPermission{
				UserID:          user.ID,
				EntityID:        personEntity.ID,
				PermissionLevel: models.PermissionLevelOwner,
			}
			if err := h.permissions.CreateUserEntityPermission(r.Context(), userPerm); err != nil {
				return nil, fmt.Errorf("grant fallback person entity permission: %w", err)
			}
			personEntityID = &personEntity.ID
		}

		// Create new ledger
		ledger := &models.Ledger{
			UserID:   user.ID, // CRITICAL: Must set user_id to avoid foreign key constraint violation
			Name:     "Personal",
			Currency: "USD",
		}
		if err := h.ledgers.Create(r.Context(), ledger); err != nil {
			return nil, err
		}

		// Link ledger to person entity
		entityLedger := &models.EntityLedger{
			EntityID: *personEntityID,
			LedgerID: ledger.ID,
			Role:     "owner",
		}
		if err := h.permissions.CreateEntityLedger(r.Context(), entityLedger); err != nil {
			return nil, err
		}

		// Seed default tags
		slog.InfoContext(r.Context(), "new ledger created, seeding default tags", "email", user.Email)
		if err := h.taxonomy.SeedDefaultTags(r.Context(), ledger.ID); err != nil {
			slog.WarnContext(r.Context(), "failed to seed default tags", "err", err)
		} else {
			slog.InfoContext(r.Context(), "default tags seeded")
		}

		return ledger, nil
	}

	// Fetch all accessible ledgers and return the earliest-created one.
	// Map iteration is non-deterministic, so collect + sort to guarantee a
	// stable selection when users have multiple ledgers.
	var ledgers []*models.Ledger
	for ledgerID := range ledgerPerms {
		ledger, err := h.ledgers.GetByID(r.Context(), ledgerID)
		if err == nil {
			ledgers = append(ledgers, ledger)
		}
	}
	if len(ledgers) == 0 {
		return nil, fmt.Errorf("no accessible ledger found")
	}
	sort.Slice(ledgers, func(i, j int) bool {
		return ledgers[i].CreatedAt.Before(ledgers[j].CreatedAt)
	})
	return ledgers[0], nil
}

// htmxRedirect performs an HTMX-compatible redirect. For HTMX requests it sets
// HX-Redirect and responds 200; for plain requests it sends a 303 redirect.
// The caller must return immediately after calling this function.
func htmxRedirect(w http.ResponseWriter, r *http.Request, url string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// getLogoURL constructs the full CDN URL from a filename stored in the database
func (h *Handlers) getLogoURL(logoURL string) string {
	if h.logoClient == nil {
		// Fallback: if no logo client, try to construct URL from config
		if h.cfg != nil && h.cfg.CDNDomain != "" {
			return enrichment.GetLogoURL(logoURL, h.cfg.CDNDomain)
		}
		return logoURL
	}
	return h.logoClient.GetLogoURL(logoURL)
}

// threadStoreAdapter adapts chat.ThreadStore to llm.ThreadStore interface
type threadStoreAdapter struct {
	store *chat.ThreadStore
}

func (a *threadStoreAdapter) CreateThread(ctx context.Context, ledgerID, userID uuid.UUID, parentThreadID *uuid.UUID) (llm.ThreadStoreThread, error) {
	return a.store.CreateThread(ctx, ledgerID, userID, parentThreadID)
}

func (a *threadStoreAdapter) GetThreadForUser(ctx context.Context, id, ledgerID, userID uuid.UUID) (llm.ThreadStoreThread, error) {
	return a.store.GetThreadForUser(ctx, id, ledgerID, userID)
}

func (a *threadStoreAdapter) GetMessages(ctx context.Context, threadID uuid.UUID) ([]llm.ThreadStoreMessage, error) {
	msgs, err := a.store.GetMessages(ctx, threadID)
	if err != nil {
		return nil, err
	}
	result := make([]llm.ThreadStoreMessage, len(msgs))
	for i := range msgs {
		result[i] = &msgs[i]
	}
	return result, nil
}

func (a *threadStoreAdapter) AddMessage(ctx context.Context, threadID uuid.UUID, role, content, sqlQuery string, results interface{}) (llm.ThreadStoreMessage, error) {
	// Convert results to QueryResult if possible
	var queryResult *chat.QueryResult
	if results != nil {
		if qr, ok := results.(*chat.QueryResult); ok {
			queryResult = qr
		}
	}
	return a.store.AddMessage(ctx, threadID, role, content, sqlQuery, queryResult)
}

func (a *threadStoreAdapter) UpdateThreadTitle(ctx context.Context, id uuid.UUID, title string) error {
	return a.store.UpdateThreadTitle(ctx, id, title)
}

func (a *threadStoreAdapter) CountMessages(ctx context.Context, threadID uuid.UUID) (int, error) {
	return a.store.CountMessages(ctx, threadID)
}
