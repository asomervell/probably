package processing

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"log/slog"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/embedding"
	"github.com/asomervell/probably/internal/enrichment"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/recurring"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/semaphore"
)

// OrchestratorInterface defines the interface for LLM orchestration
// This allows the worker to use the orchestrator without importing it directly (avoiding import cycles)
type OrchestratorInterface interface {
	Execute(ctx context.Context, task interface{}) (interface{}, error)
	CallPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	IsConfigured() bool
}

// TaskBuilder is a function type for building orchestrator tasks
// This allows main.go to provide task builders without the worker importing orchestrator
type TaskBuilder func(ledgerID string, strategy string, transactionInputs, tagInputs, ruleInputs []interface{}, entitySearcher, purchaseMatcher interface{}, relationships []*models.Relationship) interface{}

// P2PTaskBuilder is a function type for building P2P orchestrator tasks
type P2PTaskBuilder func(ledgerID string, strategy string, transactionInputs, tagInputs []interface{}, householdPatterns []string, relationships []*models.Relationship) interface{}

// Worker processes transactions through the unified categorization pipeline.
// This replaces the separate enrichment and categorization workers.
//
// Pipeline:
// 1. Get pending transactions (categorization_status IN ('pending', 'queued', 'failed') AND no tags)
// 2. Call LLM with all context (description + Teller data + entity info)
// 3. LLM returns: category, title, entity_type, confidence
// 4. Apply category tag (with fuzzy matching fallback)
// 5. Create/link entity if entity_type != "none"
// 6. Fetch logos for entities
// 7. Mark as done (only if tag was successfully applied)
//
// Note: enrichment_status is set for backwards compatibility but not used by this worker.
// All enrichment (entity detection, logo fetching) happens during categorization.

var feedWorkCallCount atomic.Int64

type Worker struct {
	cfg              *config.Config
	logoClient       *enrichment.LogoClient
	firecrawlClient  *enrichment.FirecrawlClient
	orchestrator     OrchestratorInterface // Orchestrator for LLM calls (interface to avoid import cycle)
	embeddingService *embedding.Service    // Embedding service for semantic embeddings (optional)
	taskBuilder      TaskBuilder           // Function to build categorization tasks
	p2pTaskBuilder   P2PTaskBuilder        // Function to build P2P tasks
	transactions     *models.TransactionStore
	entities         *models.EntityStore
	tags             *models.TagStore
	rules            *models.RuleStore
	patternDetector  *recurring.Detector // Pattern detector for LLM-based pattern detection

	// Batch settings
	batchSize      int // transactions per LLM call
	llmConcurrency int // parallel LLM calls
	maxRetries     int

	// Parallelization
	llmSemaphore       *semaphore.Weighted // limits concurrent LLM calls
	logoSemaphore      *semaphore.Weighted
	postProcessWorkers int
	postWorkChan       chan *postProcessWork

	// Worker control
	stopCh   chan struct{}
	wg       sync.WaitGroup
	workerWg sync.WaitGroup
	mu       sync.Mutex
	running  bool

	// Logging control
	lastSkipLogTime int64 // Unix timestamp of last "skipping feed" log message
}

// postProcessWork contains the work needed after LLM categorization
type postProcessWork struct {
	txn       *models.Transaction
	llmResult *LLMResult
	tagMap    map[string]uuid.UUID
}

func NewWorker(
	cfg *config.Config,
	pool *pgxpool.Pool,
	transactions *models.TransactionStore,
	entities *models.EntityStore,
	tags *models.TagStore,
	rules *models.RuleStore,
	orchestrator OrchestratorInterface, // Optional orchestrator (can be nil)
	taskBuilder TaskBuilder, // Function to build tasks (required if orchestrator is provided)
	p2pTaskBuilder P2PTaskBuilder, // Function to build P2P tasks (required if orchestrator is provided)
) *Worker {
	batchSize := cfg.CategorizationBatchSize
	if batchSize <= 0 {
		batchSize = 20 // transactions per LLM call
	}

	llmConcurrency := cfg.LLMConcurrency
	if llmConcurrency <= 0 {
		llmConcurrency = 20 // parallel LLM calls
	}

	maxRetries := cfg.CategorizationMaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	// Post-processing workers handle entity creation and logo fetching
	postProcessWorkers := cfg.ProcessingWorkers
	if postProcessWorkers <= 0 {
		postProcessWorkers = 20
	}

	logoConcurrency := cfg.LogoConcurrency
	if logoConcurrency <= 0 {
		logoConcurrency = 10
	}

	logoClient, err := enrichment.NewLogoClient(cfg)
	if err != nil {
		// Logo client not critical - continue without it
		logoClient = nil
	}

	// Initialize Firecrawl cache
	firecrawlCache := enrichment.NewFirecrawlCache(pool)
	firecrawlClient := enrichment.NewFirecrawlClientWithCache(cfg, firecrawlCache)

	// Create pattern detector with LLM support if orchestrator is available
	var patternDetector *recurring.Detector
	if orchestrator != nil && orchestrator.IsConfigured() {
		patternDetector = recurring.NewDetectorWithLLM(orchestrator)
	} else {
		patternDetector = recurring.NewDetector()
	}

	return &Worker{
		cfg:                cfg,
		logoClient:         logoClient,
		firecrawlClient:    firecrawlClient,
		orchestrator:       orchestrator, // Use provided orchestrator (may be nil)
		taskBuilder:        taskBuilder,
		p2pTaskBuilder:     p2pTaskBuilder,
		transactions:       transactions,
		patternDetector:    patternDetector,
		entities:           entities,
		tags:               tags,
		rules:              rules,
		batchSize:          batchSize,
		llmConcurrency:     llmConcurrency,
		maxRetries:         maxRetries,
		llmSemaphore:       semaphore.NewWeighted(int64(llmConcurrency)),
		logoSemaphore:      semaphore.NewWeighted(int64(logoConcurrency)),
		postProcessWorkers: postProcessWorkers,
		postWorkChan:       make(chan *postProcessWork, 500), // larger buffer for parallel LLM results
		stopCh:             make(chan struct{}),
		lastSkipLogTime:    0,
	}
}

// SetEmbeddingService sets the embedding service for generating transaction embeddings
// This should be called before Start() if embeddings are desired
func (w *Worker) SetEmbeddingService(svc *embedding.Service) {
	w.embeddingService = svc
}

func (w *Worker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	// Warn if orchestrator is not configured
	if w.orchestrator == nil {
		slog.Warn("Processing worker started but orchestrator is nil - transactions will NOT be processed",
			"hint", "Set LLM_DEFAULT_MODEL and corresponding API key")
	}

	// Start post-processing workers (for entity/logo work after LLM)
	for i := 0; i < w.postProcessWorkers; i++ {
		w.workerWg.Add(1)
		go w.postProcessLoop(i)
	}

	w.wg.Add(1)
	go w.run()

	// Start pattern detection worker loop
	w.wg.Add(1)
	go w.runPatternDetection()

	slog.Info("Processing worker started",
		"batch_size", w.batchSize,
		"llm_concurrency", w.llmConcurrency,
		"post_workers", w.postProcessWorkers,
		"max_in_flight", w.llmConcurrency*w.batchSize)
}

func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	close(w.stopCh)
	w.wg.Wait()

	// Close post-process channel and wait for workers to finish
	close(w.postWorkChan)
	w.workerWg.Wait()

	slog.Info("Processing worker stopped")
}

func (w *Worker) run() {
	defer w.wg.Done()

	slog.Info("Worker run loop started",
		"poll_interval_sec", 2,
		"batch_size", w.batchSize,
		"llm_concurrency", w.llmConcurrency,
		"max_retries", w.maxRetries,
		"orchestrator_configured", w.orchestrator != nil && w.orchestrator.IsConfigured())

	// Poll for work every 2 seconds
	pollTicker := time.NewTicker(2 * time.Second)
	defer pollTicker.Stop()

	for {
		select {
		case <-w.stopCh:
			slog.Info("Worker run loop received stop signal")
			return
		case <-pollTicker.C:
			w.feedWork()
		}
	}
}

// feedWork fetches pending transactions, batches them for LLM categorization,
// and sends results to post-processing workers
func (w *Worker) feedWork() {
	ctx := observability.WithDistinctID(context.Background(), "worker:probably-categorization")

	// Need orchestrator configured
	if w.orchestrator == nil || !w.orchestrator.IsConfigured() {
		// Only log this warning once per minute to avoid log spam
		now := time.Now().Unix()
		if now-w.lastSkipLogTime >= 60 {
			slog.WarnContext(ctx, "Skipping feed: LLM not configured")
			w.lastSkipLogTime = now
		}
		return
	}

	feedWorkCallCount.Add(1)

	// Keep fetching and processing batches until queue is empty
	for {
		select {
		case <-w.stopCh:
			return
		default:
		}

		// Root transaction so AI spans in this batch are grouped for PostHog LLM / traces
		transaction, txnCtx := observability.StartTransaction(ctx, "ai.worker.categorization_batch", "ai.pipeline")

		// Fetch enough transactions for all concurrent LLM calls
		fetchSize := w.batchSize * w.llmConcurrency

		// Auto-recover stuck transactions periodically
		if feedWorkCallCount.Load()%10 == 1 {
			recoverQuery := `
				UPDATE transactions SET
					categorization_attempts = 0,
					categorization_status = 'queued',
					categorization_error = NULL,
					categorization_queued_at = NOW(),
					updated_at = NOW()
				WHERE categorization_status IN ('pending', 'queued', 'failed')
					AND is_transfer = false
					AND categorization_attempts >= $1
					AND NOT EXISTS (SELECT 1 FROM transaction_tags tt WHERE tt.transaction_id = transactions.id)
			`
			if result, err := w.transactions.GetPool().Exec(txnCtx, recoverQuery, w.maxRetries); err == nil {
				if recovered := result.RowsAffected(); recovered > 0 {
					slog.InfoContext(txnCtx, "Auto-recovered stuck transactions", "count", recovered)
				}
			}
		}

		transactions, err := w.transactions.GetAllQueuedForCategorization(txnCtx, fetchSize, w.maxRetries)
		if err != nil {
			slog.ErrorContext(txnCtx, "Failed to fetch categorization queue", "err", err)
			observability.CaptureFailure(txnCtx, err, observability.FailureOptions{
				Component: "processing_worker",
				Operation: "fetch_categorization_queue",
				Tags: map[string]string{
					"batch.fetch_size": fmt.Sprintf("%d", fetchSize),
				},
			})
			transaction.End(err)
			return
		}

		if len(transactions) == 0 {
			transaction.End(nil)
			return // Queue empty, wait for next poll
		}

		slog.InfoContext(txnCtx, "Processing transactions", "count", len(transactions))
		transaction.SetData("transaction_count", len(transactions))

		// Mark transactions as processing to prevent concurrent processing
		txnIDs := make([]uuid.UUID, len(transactions))
		for i, txn := range transactions {
			txnIDs[i] = txn.ID
		}
		if err := w.transactions.MarkAsProcessing(txnCtx, txnIDs); err != nil {
			slog.WarnContext(txnCtx, "Failed to mark transactions as processing", "err", err)
			// Continue anyway - worst case we process them twice
		}

		// Process through parallel LLM calls
		w.processParallelBatches(txnCtx, transactions)
		transaction.End(nil)
	}
}

// processParallelBatches splits transactions into batches and processes them in parallel
func (w *Worker) processParallelBatches(ctx context.Context, transactions []*models.Transaction) {
	batchStart := time.Now()

	// Group by ledger first (each ledger has its own tags)
	byLedger := make(map[uuid.UUID][]*models.Transaction)
	for _, txn := range transactions {
		byLedger[txn.LedgerID] = append(byLedger[txn.LedgerID], txn)
	}

	var wg sync.WaitGroup

	for ledgerID, ledgerTxns := range byLedger {
		// Split ledger transactions into batches
		for i := 0; i < len(ledgerTxns); i += w.batchSize {
			end := i + w.batchSize
			if end > len(ledgerTxns) {
				end = len(ledgerTxns)
			}
			batch := ledgerTxns[i:end]

			wg.Add(1)
			go func(lid uuid.UUID, txns []*models.Transaction) {
				defer wg.Done()

				// Acquire semaphore to limit concurrent LLM calls
				if err := w.llmSemaphore.Acquire(ctx, 1); err != nil {
					slog.ErrorContext(ctx, "Failed to acquire LLM semaphore", "err", err)
					return
				}
				defer w.llmSemaphore.Release(1)

				w.processBatchForLedger(ctx, lid, txns)
			}(ledgerID, batch)
		}
	}

	wg.Wait()
	slog.DebugContext(ctx, "Parallel batch completed",
		"total_transactions", len(transactions),
		"duration_ms", time.Since(batchStart).Milliseconds())
}

// processBatchForLedger processes a batch of transactions for a single ledger
func (w *Worker) processBatchForLedger(ctx context.Context, ledgerID uuid.UUID, transactions []*models.Transaction) {
	slog.DebugContext(ctx, "processBatchForLedger", "ledger", ledgerID, "count", len(transactions))

	// Get tags for this ledger
	tags, err := w.tags.GetByLedgerID(ctx, ledgerID)
	if err != nil {
		slog.ErrorContext(ctx, "fetching tags failed", "ledger", ledgerID, "error", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "processing_worker",
			Operation: "load_tags",
			Tags: map[string]string{
				"ledger_id":            ledgerID.String(),
				"transaction_count":    fmt.Sprintf("%d", len(transactions)),
				"categorization_batch": "regular",
			},
		})
		w.markCategorizationStatus(ctx, transactions, models.CategorizationStatusFailed, err.Error())
		return
	}

	if len(tags) == 0 {
		slog.DebugContext(ctx, "no tags configured, skipping", "ledger", ledgerID, "count", len(transactions))
		w.markCategorizationStatus(ctx, transactions, models.CategorizationStatusSkipped, "no tags configured")
		return
	}

	slog.DebugContext(ctx, "processing batch", "ledger", ledgerID, "tags", len(tags), "transactions", len(transactions))

	// Build tag lookup map
	tagMap := make(map[string]uuid.UUID)
	for _, tag := range tags {
		tagMap[strings.ToLower(tag.Name)] = tag.ID
	}

	// Get user rules
	rules, err := w.rules.GetActiveRules(ctx, ledgerID)
	if err != nil {
		slog.WarnContext(ctx, "failed to load active rules, proceeding without", "ledger", ledgerID, "error", err)
		rules = nil
	}

	// P2P DETECTION: Trust bank data for counterparty type
	// The LLM will determine the actual transfer_type (person_payment, person_receipt, household, etc.)
	p2pDetected := 0
	for _, txn := range transactions {
		if txn.TransferType == "" {
			// Load entries if needed (for amount direction)
			if len(txn.Entries) == 0 {
				if err := w.transactions.LoadEntries(ctx, txn); err != nil {
					slog.WarnContext(ctx, "failed to load transaction entries", "transaction_id", txn.ID, "error", err)
				}
			}

			// Trust bank data - if counterparty is a person, mark as P2P
			// The LLM will determine the specific transfer_type during categorization
			if recurring.IsP2PTransaction(txn.CounterpartyType) {
				// Set initial transfer type based on direction, LLM will refine
				var amount int64
				for _, entry := range txn.Entries {
					if entry.AccountType == models.AccountTypeAsset || entry.AccountType == models.AccountTypeLiability {
						amount = entry.AmountCents
						break
					}
				}

				var transferType string
				if amount >= 0 {
					transferType = models.TransferTypePersonReceipt
				} else {
					transferType = models.TransferTypePersonPayment
				}

				txn.TransferType = transferType
				if err := w.transactions.SetTransferType(ctx, txn.ID, transferType); err != nil {
					slog.WarnContext(ctx, "P2P failed to set transfer type", "txn", txn.ID, "error", err)
				} else {
					p2pDetected++
					slog.DebugContext(ctx, "P2P detected", "txn", txn.ID, "transfer_type", transferType, "counterparty", txn.CounterpartyName)
				}
			}
		}
	}
	if p2pDetected > 0 {
		slog.DebugContext(ctx, "P2P transactions detected from bank data", "ledger", ledgerID, "count", p2pDetected)
	}

	// Separate P2P transactions for different prompt handling
	var regularTxns []*models.Transaction
	var p2pTxns []*models.Transaction
	for _, txn := range transactions {
		if models.IsP2PTransferType(txn.TransferType) {
			p2pTxns = append(p2pTxns, txn)
		} else {
			regularTxns = append(regularTxns, txn)
		}
	}

	// Process P2P transactions with specialized prompt
	if len(p2pTxns) > 0 {
		slog.DebugContext(ctx, "routing P2P transactions", "ledger", ledgerID, "count", len(p2pTxns))
		w.processP2PBatch(ctx, ledgerID, p2pTxns, tags, tagMap)
	}

	// Process regular transactions
	if len(regularTxns) == 0 {
		return
	}
	transactions = regularTxns

	// Build contexts for all transactions
	contexts := make([]TransactionContext, 0, len(transactions))
	txnMap := make(map[uuid.UUID]*models.Transaction)
	for _, txn := range transactions {
		// Load entries if needed
		if len(txn.Entries) == 0 {
			if err := w.transactions.LoadEntries(ctx, txn); err != nil {
				slog.WarnContext(ctx, "failed to load transaction entries", "transaction_id", txn.ID, "error", err)
			}
		}
		contexts = append(contexts, w.buildTransactionContext(ctx, txn))
		txnMap[txn.ID] = txn
	}

	// Single LLM call for the entire batch (via orchestrator)
	llmStart := time.Now()
	purchaseMatcher := &WorkerPurchaseMatcher{pool: w.transactions.GetPool()}

	// Check if orchestrator is configured
	if w.orchestrator == nil || !w.orchestrator.IsConfigured() {
		slog.ErrorContext(ctx, "orchestrator not configured, skipping categorization")
		observability.CaptureFailure(ctx, fmt.Errorf("orchestrator not configured"), observability.FailureOptions{
			Component: "processing_worker",
			Operation: "orchestrator_execute",
			Tags: map[string]string{
				"ledger_id":            ledgerID.String(),
				"transaction_count":    fmt.Sprintf("%d", len(transactions)),
				"categorization_batch": "regular",
			},
		})
		w.markCategorizationStatus(ctx, transactions, models.CategorizationStatusFailed, "LLM not configured")
		return
	}

	// Convert inputs to orchestrator format
	transactionInputs := make([]interface{}, len(contexts))
	for i, ctx := range contexts {
		transactionInputs[i] = ctx
	}

	tagInputs := make([]interface{}, len(tags))
	for i, tag := range tags {
		tagInputs[i] = tag
	}

	ruleInputs := make([]interface{}, len(rules))
	for i, rule := range rules {
		ruleInputs[i] = rule
	}

	// Get strategy from config (default to escalate for cost optimization)
	strategy := w.cfg.LLMDefaultStrategy
	if strategy == "" {
		strategy = "escalate" // Default to escalate for worker
	}

	// Fetch "My Life" relationships for context
	relationshipStore := models.NewRelationshipStore(w.transactions.GetPool())
	relationships, err := relationshipStore.GetByLedgerID(ctx, ledgerID)
	if err != nil {
		slog.WarnContext(ctx, "failed to load relationships, proceeding without", "ledger", ledgerID, "error", err)
		relationships = nil
	}

	// Create task for orchestrator using provided builder
	var resultInterface interface{}
	if w.taskBuilder == nil {
		slog.ErrorContext(ctx, "task builder not provided", "ledger", ledgerID)
		observability.CaptureFailure(ctx, fmt.Errorf("task builder not provided"), observability.FailureOptions{
			Component: "processing_worker",
			Operation: "build_task",
			Tags: map[string]string{
				"ledger_id":            ledgerID.String(),
				"transaction_count":    fmt.Sprintf("%d", len(transactions)),
				"categorization_batch": "regular",
			},
		})
		w.markCategorizationStatus(ctx, transactions, models.CategorizationStatusFailed, "task builder not provided")
		return
	}
	task := w.taskBuilder(ledgerID.String(), strategy, transactionInputs, tagInputs, ruleInputs, w.entities, purchaseMatcher, relationships)

	slog.DebugContext(ctx, "calling orchestrator", "count", len(transactions), "ledger", ledgerID, "strategy", strategy)
	resultInterface, err = w.orchestrator.Execute(ctx, task)
	slog.DebugContext(ctx, "llm_batch timing", "count", len(transactions), "duration", time.Since(llmStart))

	var results []LLMResult
	if err == nil {
		results, err = extractLLMResults(resultInterface)
	}

	if err != nil {
		slog.ErrorContext(ctx, "LLM batch failed", "ledger", ledgerID, "error", err, "transaction_count", len(transactions), "error_type", fmt.Sprintf("%T", err))
		failureTags := map[string]string{
			"ledger_id":            ledgerID.String(),
			"transaction_count":    fmt.Sprintf("%d", len(transactions)),
			"strategy":             strategy,
			"categorization_batch": "regular",
		}
		if IsNonRetryableError(err) {
			failureTags["retryable"] = "false"
		} else {
			failureTags["retryable"] = "true"
		}
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "processing_worker",
			Operation: "categorize_batch",
			Tags:      failureTags,
		})

		// Check if this is a non-retryable error (e.g., permission errors)
		// These should be marked as failed with a special status to prevent retries
		if IsNonRetryableError(err) {
			slog.ErrorContext(ctx, "non-retryable error, marking permanently failed")
			w.markCategorizationStatus(ctx, transactions, models.CategorizationStatusFailed, "Permission error - check Vertex AI configuration: "+err.Error())
			return
		}

		// For retryable errors, mark as failed (will be retried up to maxRetries)
		w.markCategorizationStatus(ctx, transactions, models.CategorizationStatusFailed, err.Error())
		return
	}

	if len(results) == 0 {
		slog.WarnContext(ctx, "LLM returned empty results", "count", len(transactions))
		observability.CaptureFailure(ctx, fmt.Errorf("llm returned empty results"), observability.FailureOptions{
			Component: "processing_worker",
			Operation: "categorize_batch",
			Tags: map[string]string{
				"ledger_id":            ledgerID.String(),
				"transaction_count":    fmt.Sprintf("%d", len(transactions)),
				"strategy":             strategy,
				"categorization_batch": "regular",
			},
		})
		w.markCategorizationStatus(ctx, transactions, models.CategorizationStatusFailed, "LLM returned no results")
		return
	}

	slog.DebugContext(ctx, "LLM batch results", "results", len(results), "transactions", len(transactions))

	// Build a map of results by transaction_id for efficient lookup
	resultMap := make(map[uuid.UUID]*LLMResult)
	unmatchedResults := make([]*LLMResult, 0)

	for i := range results {
		result := &results[i]
		if result.TransactionID != "" {
			parsedID, err := uuid.Parse(result.TransactionID)
			if err == nil {
				if _, exists := txnMap[parsedID]; exists {
					resultMap[parsedID] = result
					continue
				}
			}
		}
		// Result doesn't have a valid transaction_id, use index-based matching
		unmatchedResults = append(unmatchedResults, result)
	}

	// Process transactions: match by ID first, then by index for unmatched results
	processedTxns := make(map[uuid.UUID]bool)
	unmatchedIndex := 0

	for _, txnCtx := range contexts {
		txnID := txnCtx.ID
		txn := txnMap[txnID]

		// Try to find result by transaction_id first
		result, foundByID := resultMap[txnID]
		if !foundByID {
			// Fallback to index-based matching if we have unmatched results
			if unmatchedIndex < len(unmatchedResults) {
				result = unmatchedResults[unmatchedIndex]
				unmatchedIndex++
			} else {
				// No result available for this transaction
				if sErr := w.transactions.UpdateCategorizationStatus(ctx, txnID, models.CategorizationStatusFailed, "LLM did not return a result for this transaction"); sErr != nil {
					slog.WarnContext(ctx, "failed to mark categorization failed", "txn", txnID, "error", sErr)
				}
				continue
			}
		}

		processedTxns[txnID] = true

		// Send to post-processing worker
		select {
		case w.postWorkChan <- &postProcessWork{
			txn:       txn,
			llmResult: result,
			tagMap:    tagMap,
		}:
		case <-w.stopCh:
			return
		}
	}
}

// postProcessLoop handles post-LLM work: applying tags, creating entities, fetching logos
func (w *Worker) postProcessLoop(id int) {
	defer w.workerWg.Done()
	slog.Debug("post-process worker started", "id", id)

	for work := range w.postWorkChan {
		w.postProcessTransaction(context.Background(), work)
	}
	slog.Debug("post-process worker stopped", "id", id)
}

// postProcessTransaction applies LLM results: tags, titles, entities, logos
func (w *Worker) postProcessTransaction(ctx context.Context, work *postProcessWork) {
	txn := work.txn
	result := work.llmResult
	tagMap := work.tagMap

	// Track if entity was linked for pattern inheritance
	var linkedEntityID *uuid.UUID

	tagApplied := w.applyTagByCategory(ctx, txn.ID, result.Category, tagMap)

	// Save display title
	if result.Title != "" {
		if err := w.transactions.SetDisplayTitle(ctx, txn.ID, result.Title); err != nil {
			slog.WarnContext(ctx, "failed to set display title", "txn", txn.ID, "error", err)
		}

		// Handle P2P transfers specially - they have both counterparty and intermediary
		if result.IsP2P() {
			start := time.Now()
			// Create/link the counterparty (person)
			if result.Counterparty != "" {
				person, err := w.getOrCreateEntity(ctx, result.Counterparty, "person")
				if err != nil {
					slog.WarnContext(ctx, "P2P counterparty entity creation failed", "txn", txn.ID, "error", err)
				} else if person != nil {
					if err := w.transactions.SetCounterpartyEntityID(ctx, txn.ID, person.ID); err != nil {
						slog.WarnContext(ctx, "failed to set counterparty entity", "txn", txn.ID, "error", err)
					}
				}
			}
			// Create/link the intermediary (payment processor like Venmo, Zelle)
			if result.Intermediary != "" {
				intermediary, err := w.getOrCreateEntity(ctx, result.Intermediary, "business")
				if err != nil {
					slog.WarnContext(ctx, "P2P intermediary entity creation failed", "txn", txn.ID, "error", err)
				} else if intermediary != nil {
					if err := w.transactions.SetIntermediaryEntityID(ctx, txn.ID, intermediary.ID); err != nil {
						slog.WarnContext(ctx, "failed to set intermediary entity", "txn", txn.ID, "error", err)
					}
				}
			}
			slog.DebugContext(ctx, "p2p_entities timing", "txn", txn.ID, "duration", time.Since(start))
		} else if result.ShouldLinkEntity() {
			// Regular entity linking for non-P2P transactions
			// Pass Teller category and description for better business type detection
			start := time.Now()
			entity, err := w.getOrCreateEntityWithCategory(ctx, result.Title, result.EntityType, txn.TellerCategory, txn.Description)
			if err != nil {
				slog.WarnContext(ctx, "entity creation failed", "txn", txn.ID, "error", err)
			} else if entity != nil {
				if err := w.transactions.SetEntityID(ctx, txn.ID, entity.ID); err != nil {
					slog.WarnContext(ctx, "failed to set entity", "txn", txn.ID, "error", err)
				} else {
					linkedEntityID = &entity.ID // Track for pattern inheritance
				}
				slog.DebugContext(ctx, "entity timing", "txn", txn.ID, "subtype", entity.Subtype, "duration", time.Since(start))
			}
		}
	}

	// Only mark as done if a tag was successfully applied
	if tagApplied {
		if err := w.transactions.UpdateCategorizationStatus(ctx, txn.ID, models.CategorizationStatusDone, ""); err != nil {
			slog.WarnContext(ctx, "failed to mark categorization done", "txn", txn.ID, "error", err)
		}

		// Generate embedding for this transaction (async, non-blocking)
		if w.embeddingService != nil && w.embeddingService.IsConfigured() {
			go w.generateTransactionEmbedding(txn, result.Title)
		}

		// PATTERN INHERITANCE: If linked to an entity with existing patterns, inherit them
		// This ensures single transactions (like monthly subscriptions) get pattern data
		// without waiting for 2+ transactions to accumulate
		patternInherited := false
		if linkedEntityID != nil {
			if existingPattern, err := w.transactions.GetExistingPatternForEntity(ctx, *linkedEntityID, txn.LedgerID); err == nil && existingPattern != nil {
				// Copy pattern metadata to this transaction
				if err := w.transactions.InheritPattern(ctx, txn.ID, existingPattern); err != nil {
					slog.WarnContext(ctx, "failed to inherit pattern", "txn", txn.ID, "error", err)
				} else {
					patternInherited = true
					slog.DebugContext(ctx, "PATTERN INHERITED", "txn", txn.ID, "pattern_type", existingPattern.PatternType, "entity", linkedEntityID)
				}
			}
		}

		// Only queue for pattern detection if we didn't inherit an existing pattern
		// Pattern detection will run asynchronously in a separate worker loop
		if !patternInherited {
			if err := w.transactions.QueueForPatternDetection(ctx, []uuid.UUID{txn.ID}); err != nil {
				slog.WarnContext(ctx, "failed to queue for pattern detection", "txn", txn.ID, "error", err)
			}
		}
	} else {
		// Mark as failed so it can be retried (maybe with different LLM response or new tags)
		errorMsg := fmt.Sprintf("No matching tag found for LLM category: %q", result.Category)
		if err := w.transactions.UpdateCategorizationStatus(ctx, txn.ID, models.CategorizationStatusFailed, errorMsg); err != nil {
			slog.WarnContext(ctx, "failed to mark categorization failed", "txn", txn.ID, "error", err)
		}
		slog.DebugContext(ctx, "MARKED FAILED", "txn", txn.ID, "reason", errorMsg)
	}
}

// generateTransactionEmbedding generates and stores an embedding for a transaction
// This runs asynchronously to not block the categorization pipeline
func (w *Worker) generateTransactionEmbedding(txn *models.Transaction, displayTitle string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build text for embedding - use display title if available, otherwise description
	text := displayTitle
	if text == "" {
		text = txn.Description
	}

	// Add pattern type if known
	if txn.PatternType != "" && txn.PatternType != "none" {
		text += ". Type: " + txn.PatternType
	}

	embedding, err := w.embeddingService.EmbedText(ctx, text)
	if err != nil {
		slog.WarnContext(ctx, "embedding generation failed", "txn", txn.ID, "error", err)
		return
	}

	if err := w.transactions.UpdateEmbedding(ctx, txn.ID, embedding, w.embeddingService.Model()); err != nil {
		slog.WarnContext(ctx, "embedding store failed", "txn", txn.ID, "error", err)
		return
	}

	slog.DebugContext(ctx, "EMBEDDING STORED", "txn", txn.ID)
}

// generateEntityEmbedding generates and stores an embedding for a new entity
// This runs asynchronously to not block the categorization pipeline
func (w *Worker) generateEntityEmbedding(entity *models.Entity) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build text for embedding
	var parts []string
	parts = append(parts, entity.Name)
	if entity.Type != "" {
		parts = append(parts, "Type: "+string(entity.Type))
	}
	if entity.Subtype != "" {
		parts = append(parts, "Subtype: "+entity.Subtype)
	}
	if entity.Description != "" {
		parts = append(parts, entity.Description)
	}
	text := strings.Join(parts, ". ")

	embedding, err := w.embeddingService.EmbedText(ctx, text)
	if err != nil {
		slog.WarnContext(ctx, "entity embedding generation failed", "entity", entity.Name, "error", err)
		return
	}

	if err := w.entities.UpdateEmbedding(ctx, entity.ID, embedding, w.embeddingService.Model()); err != nil {
		slog.WarnContext(ctx, "entity embedding store failed", "entity", entity.Name, "error", err)
		return
	}

	slog.DebugContext(ctx, "ENTITY EMBEDDING STORED", "entity", entity.Name)
}

func (w *Worker) buildTransactionContext(ctx context.Context, txn *models.Transaction) TransactionContext {
	// Get amount and account info from entries
	var amount int64
	var accountName, accountType string
	var accType models.AccountType
	for _, e := range txn.Entries {
		if e.AccountType == models.AccountTypeAsset || e.AccountType == models.AccountTypeLiability {
			amount = e.AmountCents
			accountName = e.AccountName
			accountType = string(e.AccountType)
			accType = e.AccountType
			break
		}
	}
	// Flip sign for liability accounts (credit card purchases show as positive in ledger)
	if accType == models.AccountTypeLiability {
		amount = -amount
	}

	return TransactionContext{
		ID:               txn.ID,
		Description:      txn.Description,
		Amount:           amount,
		AccountName:      accountName,
		AccountType:      accountType,
		CounterpartyName: txn.CounterpartyName,
		CounterpartyType: txn.CounterpartyType,
		TellerCategory:   txn.TellerCategory,
		TellerType:       txn.TellerType,
		IsTransfer:       txn.IsTransfer,
	}
}

func (w *Worker) getOrCreateEntity(ctx context.Context, displayName string, entityType string) (*models.Entity, error) {
	return w.getOrCreateEntityWithCategory(ctx, displayName, entityType, "", "")
}

// getOrCreateEntityWithCategory creates or retrieves an entity with category context
// tellerCategory and description are used to better determine the business subtype
func (w *Worker) getOrCreateEntityWithCategory(ctx context.Context, displayName string, entityType string, tellerCategory string, description string) (*models.Entity, error) {
	if displayName == "" {
		return nil, nil
	}

	// Map LLM entity type to model EntityType
	var modelEntityType models.EntityType
	switch entityType {
	case "person":
		modelEntityType = models.EntityTypePerson
	case "government":
		modelEntityType = models.EntityTypeGovernment
	case "business":
		modelEntityType = models.EntityTypeBusiness
	default:
		modelEntityType = models.EntityTypeBusiness // Default fallback
	}

	// Determine business subtype from Teller category (strongest signal from bank data)
	var businessSubtype string
	if modelEntityType == models.EntityTypeBusiness && tellerCategory != "" {
		businessSubtype = models.TellerCategoryToSubtype(tellerCategory)
	}

	// Acquire logo semaphore for any logo-related operations
	if err := w.logoSemaphore.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer w.logoSemaphore.Release(1)

	// Logo client and store are optional — logo enrichment is skipped when not configured.
	var logoStore *enrichment.LogoStore
	if w.logoClient != nil {
		logoStore = w.logoClient.GetLogoStore()
	}

	// Check for existing entity
	existing, err := w.entities.GetByName(ctx, displayName)
	if err == nil {
		// Found - try to backfill logo if missing
		if logoStore != nil && !logoStore.IsLocalLogo(existing.LogoURL) {
			start := time.Now()

			// Update enrichment status to searching
			if err := w.entities.UpdateEnrichmentStatus(ctx, existing.ID, models.EnrichmentStatusSearching, ""); err != nil {
				slog.WarnContext(ctx, "failed to update enrichment status", "entity", existing.ID, "error", err)
			}
			if err := w.entities.AddEnrichmentStep(ctx, existing.ID, "searching", fmt.Sprintf("Searching for logo and company info for %s", displayName)); err != nil {
				slog.WarnContext(ctx, "failed to add enrichment step", "entity", existing.ID, "error", err)
			}

			// Try Firecrawl Agent API first (better for complex searches)
			// Pass description as context - often contains location hints like "AUCKLAND NZ"
			descContext := extractLocationContext(description)
			if w.firecrawlClient.IsConfigured() {
				if err := w.entities.AddEnrichmentStep(ctx, existing.ID, "extracting", "Using Firecrawl Agent API"); err != nil {
					slog.WarnContext(ctx, "failed to add enrichment step", "entity", existing.ID, "error", err)
				}
				if info, err := w.firecrawlClient.AgentSearch(ctx, displayName, descContext); err == nil && info != nil && info.LogoURL != "" {
					if w.applyFirecrawlLogoResult(ctx, existing, info, logoStore, start) {
						return existing, nil
					}
				}
			}

			// Fallback to SearchAndExtract
			if w.firecrawlClient.IsConfigured() {
				if err := w.entities.AddEnrichmentStep(ctx, existing.ID, "extracting", "Using Firecrawl Search/Extract"); err != nil {
					slog.WarnContext(ctx, "failed to add enrichment step", "entity", existing.ID, "error", err)
				}
				if info, err := w.firecrawlClient.SearchAndExtract(ctx, displayName, descContext, ""); err == nil && info != nil && info.LogoURL != "" {
					if w.applyFirecrawlLogoResult(ctx, existing, info, logoStore, start) {
						return existing, nil
					}
				}
			}

			// Fallback to logo.dev
			if !logoStore.IsLocalLogo(existing.LogoURL) && w.logoClient.IsConfigured() {
				if err := w.entities.AddEnrichmentStep(ctx, existing.ID, "fetching_logo", "Using logo.dev API"); err != nil {
					slog.WarnContext(ctx, "failed to add enrichment step", "entity", existing.ID, "error", err)
				}
				if localURL, err := w.logoClient.DownloadLogoForEntity(ctx, existing.Website, existing.Name); err == nil && localURL != "" {
					existing.LogoURL = localURL
					if err := w.entities.Update(ctx, existing); err != nil {
						slog.WarnContext(ctx, "failed to update entity after logo fetch", "entity", existing.ID, "error", err)
					}
					if err := w.entities.UpdateEnrichmentStatus(ctx, existing.ID, models.EnrichmentStatusDone, ""); err != nil {
						slog.WarnContext(ctx, "failed to update enrichment status", "entity", existing.ID, "error", err)
					}
					slog.DebugContext(ctx, "logo_fetch timing", "entity", existing.Name, "duration", time.Since(start))
					return existing, nil
				}
			}

			// Mark as done even if no logo found
			if err := w.entities.UpdateEnrichmentStatus(ctx, existing.ID, models.EnrichmentStatusDone, ""); err != nil {
				slog.WarnContext(ctx, "failed to update enrichment status", "entity", existing.ID, "error", err)
			}
		}
		return existing, nil
	}

	// Create new entity with the type from LLM
	entity := &models.Entity{
		Type: modelEntityType,
		Name: displayName,
		Slug: models.Slugify(displayName),
	}
	// Set business subtype - use detected subtype or fall back to retailer
	if modelEntityType == models.EntityTypeBusiness {
		if businessSubtype != "" {
			entity.Subtype = businessSubtype
			slog.DebugContext(ctx, "ENTITY SUBTYPE", "entity", displayName, "subtype", businessSubtype, "teller_category", tellerCategory)
		} else {
			entity.Subtype = models.BusinessSubtypeRetailer // Default fallback
		}
	}

	if err := w.entities.Create(ctx, entity); err != nil {
		// Race condition - try to fetch again
		if existing, err2 := w.entities.GetByName(ctx, displayName); err2 == nil {
			return existing, nil
		}
		return nil, err
	}

	// Try Firecrawl for logo, description, and website
	start := time.Now()

	// Update enrichment status to searching
	if err := w.entities.UpdateEnrichmentStatus(ctx, entity.ID, models.EnrichmentStatusSearching, ""); err != nil {
		slog.WarnContext(ctx, "failed to update enrichment status", "entity", entity.ID, "error", err)
	}
	if err := w.entities.AddEnrichmentStep(ctx, entity.ID, "searching", fmt.Sprintf("Searching for logo and company info for %s", displayName)); err != nil {
		slog.WarnContext(ctx, "failed to add enrichment step", "entity", entity.ID, "error", err)
	}

	// Try Firecrawl Agent API first
	// Use description context for location hints (e.g., "YOUNG DANDY AUCKLAND NZ")
	descContext := extractLocationContext(description)
	if w.firecrawlClient.IsConfigured() {
		if err := w.entities.AddEnrichmentStep(ctx, entity.ID, "extracting", "Using Firecrawl Agent API"); err != nil {
			slog.WarnContext(ctx, "failed to add enrichment step", "entity", entity.ID, "error", err)
		}
		if info, err := w.firecrawlClient.AgentSearch(ctx, displayName, descContext); err == nil && info != nil {
			applyCompanyInfo(entity, info)
			if info.LogoURL != "" && logoStore != nil {
				if err := w.entities.AddEnrichmentStep(ctx, entity.ID, "fetching_logo", fmt.Sprintf("Downloading logo from %s", info.LogoURL)); err != nil {
					slog.WarnContext(ctx, "failed to add enrichment step", "entity", entity.ID, "error", err)
				}
				if localURL, err := logoStore.DownloadAndStore(ctx, info.LogoURL); err == nil {
					entity.LogoURL = localURL
				}
			}
			if err := w.entities.Update(ctx, entity); err != nil {
				slog.WarnContext(ctx, "failed to save enriched entity (firecrawl agent)", "entity", entity.ID, "error", err)
			}
			if err := w.entities.UpdateEnrichmentStatus(ctx, entity.ID, models.EnrichmentStatusDone, ""); err != nil {
				slog.WarnContext(ctx, "failed to mark enrichment done (firecrawl agent)", "entity", entity.ID, "error", err)
			}
			slog.DebugContext(ctx, "logo_fetch timing", "entity", entity.Name, "duration", time.Since(start))
			return entity, nil
		}
	}

	// Fallback to SearchAndExtract
	if w.firecrawlClient.IsConfigured() {
		if err := w.entities.AddEnrichmentStep(ctx, entity.ID, "extracting", "Using Firecrawl Search/Extract"); err != nil {
			slog.WarnContext(ctx, "failed to add enrichment step", "entity", entity.ID, "error", err)
		}
		if info, err := w.firecrawlClient.SearchAndExtract(ctx, displayName, descContext, ""); err == nil && info != nil {
			applyCompanyInfo(entity, info)
			if info.LogoURL != "" && logoStore != nil {
				if err := w.entities.AddEnrichmentStep(ctx, entity.ID, "fetching_logo", fmt.Sprintf("Downloading logo from %s", info.LogoURL)); err != nil {
					slog.WarnContext(ctx, "failed to add enrichment step", "entity", entity.ID, "error", err)
				}
				if localURL, err := logoStore.DownloadAndStore(ctx, info.LogoURL); err == nil {
					entity.LogoURL = localURL
				}
			}
			if err := w.entities.Update(ctx, entity); err != nil {
				slog.WarnContext(ctx, "failed to save enriched entity (firecrawl search)", "entity", entity.ID, "error", err)
			}
		}
	}

	// Fallback to logo.dev only if no logo yet
	if logoStore != nil && !logoStore.IsLocalLogo(entity.LogoURL) && w.logoClient.IsConfigured() {
		if err := w.entities.AddEnrichmentStep(ctx, entity.ID, "fetching_logo", "Using logo.dev API"); err != nil {
			slog.WarnContext(ctx, "failed to add enrichment step", "entity", entity.ID, "error", err)
		}
		if localURL, err := w.logoClient.DownloadLogoForEntity(ctx, entity.Website, entity.Name); err == nil && localURL != "" {
			entity.LogoURL = localURL
			if err := w.entities.Update(ctx, entity); err != nil {
				slog.WarnContext(ctx, "failed to save enriched entity (logo.dev)", "entity", entity.ID, "error", err)
			}
		}
	}

	// Mark enrichment as done
	if err := w.entities.UpdateEnrichmentStatus(ctx, entity.ID, models.EnrichmentStatusDone, ""); err != nil {
		slog.WarnContext(ctx, "failed to update enrichment status", "entity", entity.ID, "error", err)
	}
	slog.DebugContext(ctx, "logo_fetch timing", "entity", entity.Name, "duration", time.Since(start))

	// Generate embedding for new entity (async, non-blocking)
	if w.embeddingService != nil && w.embeddingService.IsConfigured() {
		go w.generateEntityEmbedding(entity)
	}

	return entity, nil
}

// applyCompanyInfo copies non-empty description and website from info to entity.
func applyCompanyInfo(entity *models.Entity, info *enrichment.CompanyInfo) {
	if info.Description != "" {
		entity.Description = info.Description
	}
	if info.Website != "" {
		entity.Website = info.Website
	}
}

// applyFirecrawlLogoResult downloads the logo from info, applies company info fields,
// updates the entity, records an enrichment step, and marks enrichment done.
// Returns true if the logo was stored successfully; the caller should return early on true.
func (w *Worker) applyFirecrawlLogoResult(ctx context.Context, entity *models.Entity, info *enrichment.CompanyInfo, logoStore *enrichment.LogoStore, start time.Time) bool {
	localURL, err := logoStore.DownloadAndStore(ctx, info.LogoURL)
	if err != nil {
		return false
	}
	entity.LogoURL = localURL
	applyCompanyInfo(entity, info)
	if err := w.entities.Update(ctx, entity); err != nil {
		slog.WarnContext(ctx, "failed to update entity after logo fetch", "entity", entity.ID, "error", err)
	}
	if err := w.entities.AddEnrichmentStep(ctx, entity.ID, "fetching_logo", fmt.Sprintf("Downloaded logo from %s", info.LogoURL)); err != nil {
		slog.WarnContext(ctx, "failed to add enrichment step", "entity", entity.ID, "error", err)
	}
	if err := w.entities.UpdateEnrichmentStatus(ctx, entity.ID, models.EnrichmentStatusDone, ""); err != nil {
		slog.WarnContext(ctx, "failed to update enrichment status", "entity", entity.ID, "error", err)
	}
	slog.DebugContext(ctx, "logo_fetch timing", "entity", entity.Name, "duration", time.Since(start))
	return true
}

// extractLLMResults unwraps the orchestrator result into []LLMResult.
// The orchestrator may return either a map[string]interface{}{"output": []LLMResult} or
// []LLMResult directly; both forms are handled here.
func extractLLMResults(result interface{}) ([]LLMResult, error) {
	if result == nil {
		return nil, nil
	}
	if resultMap, ok := result.(map[string]interface{}); ok {
		if output, ok := resultMap["output"].([]LLMResult); ok {
			return output, nil
		}
		return nil, fmt.Errorf("unexpected result type from orchestrator: %T", resultMap["output"])
	}
	if output, ok := result.([]LLMResult); ok {
		return output, nil
	}
	return nil, fmt.Errorf("unexpected result type: %T", result)
}

// extractLocationContext extracts location hints from a transaction description
// Transaction descriptions often contain location info like "YOUNG DANDY AUCKLAND NZ"
// which helps Firecrawl find the correct business
func extractLocationContext(description string) string {
	if description == "" {
		return ""
	}

	// Common patterns:
	// - "MERCHANT NAME CITY STATE" or "MERCHANT NAME CITY COUNTRY"
	// - Location codes like "AKL" (Auckland), "SYD" (Sydney), "NYC" (New York)
	// - Country codes at the end: "NZ", "AU", "US", "UK"

	// Clean up the description - remove common payment prefixes
	desc := strings.ToUpper(description)
	prefixes := []string{
		"CARD PURCHASE ", "POS ", "SQ *", "SQUAREUP*", "PAYPAL *",
		"TST* ", "CHK ", "ACH ", "WIRE ", "VENMO ", "ZELLE ",
	}
	for _, prefix := range prefixes {
		desc = strings.TrimPrefix(desc, prefix)
	}

	// Look for location indicators
	// Return the original description if it seems to have location info
	// The Firecrawl search will benefit from city/country names

	// Common country codes at the end
	countryCodes := []string{" NZ", " AU", " US", " UK", " CA", " GB"}
	for _, code := range countryCodes {
		if strings.HasSuffix(desc, code) {
			return description // Keep the full description with location
		}
	}

	// Common city indicators
	cityIndicators := []string{
		"AUCKLAND", "WELLINGTON", "SYDNEY", "MELBOURNE", "LONDON",
		"NEW YORK", "LOS ANGELES", "SAN FRANCISCO", "CHICAGO",
		"TORONTO", "VANCOUVER", "PARIS", "BERLIN", "TOKYO",
	}
	descLower := strings.ToLower(description)
	for _, city := range cityIndicators {
		if strings.Contains(descLower, strings.ToLower(city)) {
			return description // Keep the full description with location
		}
	}

	// No location detected - return empty to let Firecrawl do a general search
	return ""
}

// runPatternDetection spawns multiple independent pattern detection workers
// Uses ENTITY-FIRST approach: groups transactions by entity, analyzes full history
func (w *Worker) runPatternDetection() {
	defer w.wg.Done()

	patternWorkers := 5 // Fewer workers since each processes more data
	staleTimeout := 5 * time.Minute

	slog.Info("Pattern detection workers starting (entity-first)", "workers", patternWorkers)

	// Spawn stale job recovery goroutine
	var workerWg sync.WaitGroup
	workerWg.Add(1)
	go func() {
		defer workerWg.Done()
		w.recoverStalePatternJobs(staleTimeout)
	}()

	// Spawn independent workers
	for i := 0; i < patternWorkers; i++ {
		workerWg.Add(1)
		go func(workerID int) {
			defer workerWg.Done()
			w.entityPatternDetectionWorker(workerID)
		}(i)
	}

	// Wait for stop signal
	<-w.stopCh
	slog.Debug("pattern detection workers received stop signal")
	workerWg.Wait()
}

// recoverStalePatternJobs periodically resets jobs stuck in 'processing' status
func (w *Worker) recoverStalePatternJobs(staleTimeout time.Duration) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			ctx := observability.WithDistinctID(context.Background(), "worker:probably-categorization")
			// Reset transactions stuck in 'processing' for too long back to 'pending'
			// so they can be picked up again (attempts counter already incremented)
			result, err := w.transactions.GetPool().Exec(ctx, `
				UPDATE transactions
				SET pattern_detection_status = 'pending',
					updated_at = NOW()
				WHERE pattern_detection_status = 'processing'
					AND updated_at < NOW() - $1::INTERVAL
			`, staleTimeout.String())
			if err != nil {
				slog.ErrorContext(ctx, "Failed to recover stale pattern jobs", "err", err)
				continue
			}
			if result.RowsAffected() > 0 {
				slog.InfoContext(ctx, "Recovered stale pattern jobs", "count", result.RowsAffected())
			}
		}
	}
}

// entityPatternDetectionWorker processes patterns by ENTITY, not random transactions
// This gives the LLM full temporal context to detect recurring patterns
func (w *Worker) entityPatternDetectionWorker(workerID int) {
	for {
		select {
		case <-w.stopCh:
			return
		default:
			processed := w.claimAndProcessEntityPatterns(workerID)
			if !processed {
				time.Sleep(2 * time.Second)
			}
		}
	}
}

// claimAndProcessEntityPatterns claims an entity with pending transactions and processes ALL its transactions
func (w *Worker) claimAndProcessEntityPatterns(workerID int) bool {
	ctx := observability.WithDistinctID(context.Background(), "worker:probably-categorization")

	if w.patternDetector == nil || w.orchestrator == nil || !w.orchestrator.IsConfigured() {
		return false
	}

	// Find entities with 2+ pending transactions and claim ALL their pending transactions
	// This ensures we process by entity, not random batches
	rows, err := w.transactions.GetPool().Query(ctx, `
		WITH entity_to_process AS (
			-- Find ONE entity with 2+ pending transactions
			SELECT entity_id, ledger_id
			FROM transactions
			WHERE entity_id IS NOT NULL
				AND categorization_status = 'done'
				AND pattern_detection_attempts < $1
				AND (
					pattern_detection_status IN ('pending', 'queued')
					OR (pattern_detection_status = 'processing' AND updated_at < NOW() - INTERVAL '5 minutes')
				)
			GROUP BY entity_id, ledger_id
			HAVING COUNT(*) >= 2
			LIMIT 1
		),
		claimed_txns AS (
			-- Claim ALL pending transactions for that entity
			SELECT t.id
			FROM transactions t
			JOIN entity_to_process e ON t.entity_id = e.entity_id AND t.ledger_id = e.ledger_id
			WHERE t.categorization_status = 'done'
				AND t.pattern_detection_attempts < $1
				AND (
					t.pattern_detection_status IN ('pending', 'queued')
					OR (t.pattern_detection_status = 'processing' AND t.updated_at < NOW() - INTERVAL '5 minutes')
				)
			FOR UPDATE SKIP LOCKED
		)
		UPDATE transactions t
		SET pattern_detection_status = 'processing',
			pattern_detection_attempts = pattern_detection_attempts + 1,
			updated_at = NOW()
		FROM claimed_txns c
		WHERE t.id = c.id
		RETURNING t.id, t.entity_id, t.ledger_id
	`, w.maxRetries)
	if err != nil {
		slog.ErrorContext(ctx, "Entity pattern worker failed to claim", "worker", workerID, "err", err)
		return false
	}

	type claimedTxn struct {
		ID       uuid.UUID
		EntityID uuid.UUID
		LedgerID uuid.UUID
	}
	var claimed []claimedTxn
	for rows.Next() {
		var t claimedTxn
		if err := rows.Scan(&t.ID, &t.EntityID, &t.LedgerID); err != nil {
			slog.ErrorContext(ctx, "pattern claim: failed to scan row", "worker", workerID, "err", err)
			continue
		}
		claimed = append(claimed, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "Entity pattern worker rows error", "worker", workerID, "err", err)
		return false
	}

	// FALLBACK: If no entities with 2+ pending, try to inherit patterns for single transactions
	// This handles the case where a monthly subscription has only 1 new transaction
	if len(claimed) == 0 {
		return w.inheritPatternsForSingleTransactions(ctx, workerID)
	}

	startTime := time.Now()
	entityID := claimed[0].EntityID
	ledgerID := claimed[0].LedgerID

	// Collect the claimed transaction IDs
	claimedIDs := make([]uuid.UUID, len(claimed))
	for i, c := range claimed {
		claimedIDs[i] = c.ID
	}

	slog.DebugContext(ctx, "Entity pattern worker claimed transactions",
		"worker", workerID,
		"entity_id", entityID.String(),
		"pending_count", len(claimed))

	// Process this entity
	w.processEntityPatternDetection(ctx, entityID, ledgerID, claimedIDs)

	elapsed := time.Since(startTime)
	slog.DebugContext(ctx, "Entity pattern worker complete",
		"worker", workerID,
		"entity_id", entityID.String(),
		"txn_count", len(claimed),
		"elapsed_ms", elapsed.Milliseconds())

	return true
}

// processEntityPatternDetection loads ALL transactions for an entity and detects patterns
func (w *Worker) processEntityPatternDetection(ctx context.Context, entityID, ledgerID uuid.UUID, claimedIDs []uuid.UUID) {
	// Load the entity
	entityStore := models.NewEntityStore(w.transactions.GetPool())
	entity, err := entityStore.GetByID(ctx, entityID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to load entity", "entity_id", entityID.String(), "err", err)
		w.markTransactionsFailed(ctx, claimedIDs, "failed to load entity")
		return
	}

	// Load ALL transactions for this entity (not just pending - we need full history)
	allTxns, err := w.transactions.GetByEntityID(ctx, entityID, ledgerID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to load entity transactions", "entity_id", entityID.String(), "err", err)
		w.markTransactionsFailed(ctx, claimedIDs, "failed to load transactions")
		return
	}

	// Load entries for each transaction
	for _, txn := range allTxns {
		if len(txn.Entries) == 0 {
			if err := w.transactions.LoadEntries(ctx, txn); err != nil {
				slog.WarnContext(ctx, "failed to load transaction entries", "transaction_id", txn.ID, "error", err)
			}
		}
	}

	slog.DebugContext(ctx, "Analyzing entity patterns",
		"entity", entity.Name,
		"total_txns", len(allTxns),
		"pending_txns", len(claimedIDs))

	// Detect patterns using the entity-first approach
	result, err := w.patternDetector.DetectPatternsForEntity(ctx, entity, entity.Name, allTxns, ledgerID)
	if err != nil {
		slog.ErrorContext(ctx, "Entity pattern detection failed",
			"entity", entity.Name,
			"err", err)
		w.markTransactionsFailed(ctx, claimedIDs, err.Error())
		return
	}

	// Determine confidence threshold based on business type
	// For cafes, restaurants, supermarkets - require much higher confidence
	// because they're typically NOT subscription businesses
	confidenceThreshold := 70 // Default threshold
	isOneTimeBusinessType := false
	if entity != nil && models.IsOneTimeBusinessType(entity.Subtype) {
		confidenceThreshold = 90 // Much stricter for cafes, restaurants, supermarkets
		isOneTimeBusinessType = true
		slog.DebugContext(ctx, "Using higher confidence threshold for one-time business type",
			"entity", entity.Name,
			"subtype", entity.Subtype,
			"threshold", confidenceThreshold)
	}

	// Process results - assign patterns to transactions
	patternsFound := len(result.Patterns)
	txnsAssigned := 0
	patternsSkipped := 0

	for _, pattern := range result.Patterns {
		// Skip low-confidence patterns for one-time business types
		if isOneTimeBusinessType && pattern.Confidence < confidenceThreshold {
			slog.DebugContext(ctx, "Skipping low-confidence pattern for one-time business",
				"entity", entity.Name,
				"pattern_name", pattern.Name,
				"confidence", pattern.Confidence,
				"required", confidenceThreshold)
			patternsSkipped++
			// Mark these transactions as "none" pattern
			for _, txnIDStr := range pattern.TransactionIDs {
				if txnID, err := uuid.Parse(txnIDStr); err == nil {
					if err := w.transactions.UpdatePatternDetectionResult(ctx, txnID, "none", map[string]interface{}{
						"skipped_reason":   "low_confidence_one_time_business",
						"original_pattern": pattern.Name,
						"confidence":       pattern.Confidence,
					}); err != nil {
						slog.WarnContext(ctx, "failed to mark low-confidence txn as none", "txn", txnID, "error", err)
					}
				}
			}
			continue
		}

		for _, txnIDStr := range pattern.TransactionIDs {
			txnID, err := uuid.Parse(txnIDStr)
			if err != nil {
				continue
			}

			metadata := map[string]interface{}{
				"pattern_name":    pattern.Name,
				"frequency":       pattern.Frequency,
				"confidence":      pattern.Confidence,
				"reasoning":       pattern.Reasoning,
				"is_subscription": pattern.PatternType == "recurring_bill",
			}

			if err := w.transactions.UpdatePatternDetectionResult(ctx, txnID, pattern.PatternType, metadata); err != nil {
				slog.ErrorContext(ctx, "Failed to update transaction pattern",
					"txn_id", txnID.String(),
					"err", err)
			} else {
				txnsAssigned++
			}
		}

		// Update entity pattern hints only for high-confidence patterns
		if entity != nil && pattern.Confidence >= confidenceThreshold {
			if err := entityStore.UpdatePatternHint(ctx, entityID, pattern.PatternType, pattern.Frequency, pattern.Confidence); err != nil {
				slog.WarnContext(ctx, "failed to update entity pattern hint", "entity", entityID, "error", err)
			}
		}
	}

	// Mark remaining claimed transactions as having no pattern
	assignedSet := make(map[uuid.UUID]bool)
	for _, pattern := range result.Patterns {
		for _, txnIDStr := range pattern.TransactionIDs {
			if txnID, err := uuid.Parse(txnIDStr); err == nil {
				assignedSet[txnID] = true
			}
		}
	}

	for _, txnID := range claimedIDs {
		if !assignedSet[txnID] {
			if err := w.transactions.UpdatePatternDetectionResult(ctx, txnID, "none", nil); err != nil {
				slog.WarnContext(ctx, "failed to mark unclaimed txn as none", "txn", txnID, "error", err)
			}
		}
	}

	slog.DebugContext(ctx, "Entity patterns detected",
		"entity", entity.Name,
		"subtype", entity.Subtype,
		"patterns_found", patternsFound,
		"patterns_skipped", patternsSkipped,
		"txns_assigned", txnsAssigned)
}

// markTransactionsFailed marks a list of transactions as failed
func (w *Worker) markTransactionsFailed(ctx context.Context, txnIDs []uuid.UUID, errMsg string) {
	for _, txnID := range txnIDs {
		if err := w.transactions.UpdatePatternDetectionStatus(ctx, txnID, models.PatternDetectionStatusFailed, errMsg); err != nil {
			slog.WarnContext(ctx, "markTransactionsFailed: failed to update status", "txn_id", txnID.String(), "err", err)
		}
	}
}

// markCategorizationStatus updates the categorization status for a batch of transactions.
// errMsg is truncated to 500 chars to fit DB column constraints.
func (w *Worker) markCategorizationStatus(ctx context.Context, transactions []*models.Transaction, status string, errMsg string) {
	if len(errMsg) > 500 {
		errMsg = errMsg[:500] + "..."
	}
	for _, txn := range transactions {
		if sErr := w.transactions.UpdateCategorizationStatus(ctx, txn.ID, status, errMsg); sErr != nil {
			slog.WarnContext(ctx, "failed to update categorization status", "txn", txn.ID, "status", status, "error", sErr)
		}
	}
}

// applyTagByCategory resolves a raw LLM category string to a tag and applies it to txnID.
// It strips hierarchical prefixes (e.g. "Food > Groceries" → "Groceries"), attempts an
// exact case-insensitive lookup, then falls back to substring fuzzy matching. Returns
// true when a tag was successfully applied.
func (w *Worker) applyTagByCategory(ctx context.Context, txnID uuid.UUID, category string, tagMap map[string]uuid.UUID) bool {
	categoryName := category
	if idx := strings.LastIndex(categoryName, ">"); idx != -1 {
		categoryName = strings.TrimSpace(categoryName[idx+1:])
	}

	if tagID, ok := tagMap[strings.ToLower(categoryName)]; ok {
		if err := w.tags.CategorizeTransaction(ctx, txnID, tagID); err != nil {
			slog.WarnContext(ctx, "tag categorization failed", "txn", txnID, "error", err)
			return false
		}
		slog.DebugContext(ctx, "TAG APPLIED", "txn", txnID, "category", categoryName)
		return true
	}

	categoryLower := strings.ToLower(categoryName)
	for tagName, tagID := range tagMap {
		if strings.Contains(tagName, categoryLower) || strings.Contains(categoryLower, tagName) {
			if err := w.tags.CategorizeTransaction(ctx, txnID, tagID); err != nil {
				slog.WarnContext(ctx, "tag fuzzy match failed", "txn", txnID, "error", err, "tried", tagName, "for", categoryName)
				continue
			}
			slog.DebugContext(ctx, "TAG FUZZY APPLIED", "txn", txnID, "tag", tagName, "category", categoryName)
			return true
		}
	}
	slog.DebugContext(ctx, "TAG NOT FOUND", "txn", txnID, "category", categoryName, "llm_returned", category)
	return false
}

// inheritPatternsForSingleTransactions handles single pending transactions by inheriting existing patterns
// This is a fallback when no entity has 2+ pending transactions
// It ensures that single transactions (like monthly subscriptions) don't get stuck
//
// Strategy:
// 1. First, try to inherit from SAME entity's existing patterns (fast, no embeddings)
// 2. If no same-entity pattern, use COLD-START: find similar entities via embeddings
func (w *Worker) inheritPatternsForSingleTransactions(ctx context.Context, workerID int) bool {
	// Find a single pending transaction whose entity already has established patterns
	rows, err := w.transactions.GetPool().Query(ctx, `
		WITH single_pending AS (
			-- Find transactions that are pending but belong to entities with existing patterns
			SELECT DISTINCT ON (t.entity_id) t.id, t.entity_id, t.ledger_id
			FROM transactions t
			WHERE t.entity_id IS NOT NULL
				AND t.categorization_status = 'done'
				AND t.pattern_detection_attempts < $1
				AND t.pattern_detection_status IN ('pending', 'queued')
				-- Entity must have existing pattern-detected transactions
				AND EXISTS (
					SELECT 1 FROM transactions existing
					WHERE existing.entity_id = t.entity_id
						AND existing.pattern_type IS NOT NULL
						AND existing.pattern_type NOT IN ('none', '', 'dismissed')
						AND existing.pattern_detection_status = 'done'
						AND COALESCE((existing.pattern_metadata->>'confidence')::INT, 0) >= 70
				)
			LIMIT 10
		)
		UPDATE transactions t
		SET pattern_detection_status = 'processing',
			pattern_detection_attempts = pattern_detection_attempts + 1,
			updated_at = NOW()
		FROM single_pending sp
		WHERE t.id = sp.id
		RETURNING t.id, t.entity_id, t.ledger_id
	`, w.maxRetries)
	if err != nil {
		slog.ErrorContext(ctx, "Single pattern inheritance failed to claim", "worker", workerID, "err", err)
		return false
	}

	var processed int
	for rows.Next() {
		var txnID, entityID, ledgerID uuid.UUID
		if err := rows.Scan(&txnID, &entityID, &ledgerID); err != nil {
			slog.ErrorContext(ctx, "single pattern inherit: failed to scan row", "worker", workerID, "err", err)
			continue
		}

		// Get existing pattern for this entity
		existingPattern, err := w.transactions.GetExistingPatternForEntity(ctx, entityID, ledgerID)
		if err != nil || existingPattern == nil {
			// No existing pattern found - mark as 'none' and move on
			if err := w.transactions.UpdatePatternDetectionResult(ctx, txnID, "none", nil); err != nil {
				slog.WarnContext(ctx, "failed to mark txn as none (no existing pattern)", "txn", txnID, "error", err)
			}
			continue
		}

		// Inherit the pattern
		if err := w.transactions.InheritPattern(ctx, txnID, existingPattern); err != nil {
			slog.ErrorContext(ctx, "Failed to inherit pattern", "txn_id", txnID.String(), "err", err)
			if err := w.transactions.UpdatePatternDetectionStatus(ctx, txnID, models.PatternDetectionStatusFailed, err.Error()); err != nil {
				slog.WarnContext(ctx, "failed to mark txn as failed after inherit error", "txn", txnID, "error", err)
			}
		} else {
			processed++
			slog.DebugContext(ctx, "Pattern inherited for single transaction",
				"worker", workerID,
				"txn_id", txnID.String(),
				"entity_id", entityID.String(),
				"pattern_type", existingPattern.PatternType)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "Single pattern inherit rows error", "worker", workerID, "err", err)
		return false
	}

	// If we processed some via same-entity inheritance, we're done
	if processed > 0 {
		return true
	}

	// COLD-START: Try embedding-based similarity for entities with NO existing patterns
	// This handles brand new entities (e.g., first time seeing "Hulu")
	return w.coldStartPatternDetection(ctx, workerID)
}

// coldStartPatternDetection uses embeddings to find similar entities and suggest patterns
// for entities that have never had pattern detection run
func (w *Worker) coldStartPatternDetection(ctx context.Context, workerID int) bool {
	// Only run if embedding service is configured
	if w.embeddingService == nil || !w.embeddingService.IsConfigured() {
		return false
	}

	// Find transactions with entities that have embeddings but no pattern history
	rows, err := w.transactions.GetPool().Query(ctx, `
		WITH cold_start AS (
			SELECT DISTINCT ON (t.entity_id) t.id, t.entity_id, t.ledger_id
			FROM transactions t
			JOIN entities e ON t.entity_id = e.id
			WHERE t.entity_id IS NOT NULL
				AND t.categorization_status = 'done'
				AND t.pattern_detection_attempts < $1
				AND t.pattern_detection_status IN ('pending', 'queued')
				-- Entity must have an embedding
				AND e.embedding IS NOT NULL
				-- Entity must NOT have any existing pattern-detected transactions
				AND NOT EXISTS (
					SELECT 1 FROM transactions existing
					WHERE existing.entity_id = t.entity_id
						AND existing.pattern_type IS NOT NULL
						AND existing.pattern_type NOT IN ('none', '', 'dismissed')
						AND existing.pattern_detection_status = 'done'
				)
			LIMIT 5
		)
		UPDATE transactions t
		SET pattern_detection_status = 'processing',
			pattern_detection_attempts = pattern_detection_attempts + 1,
			updated_at = NOW()
		FROM cold_start cs
		WHERE t.id = cs.id
		RETURNING t.id, t.entity_id, t.ledger_id
	`, w.maxRetries)
	if err != nil {
		return false
	}

	entityStore := models.NewEntityStore(w.transactions.GetPool())
	var processed int

	for rows.Next() {
		var txnID, entityID, ledgerID uuid.UUID
		if err := rows.Scan(&txnID, &entityID, &ledgerID); err != nil {
			slog.ErrorContext(ctx, "cold-start pattern: failed to scan row", "worker", workerID, "err", err)
			continue
		}

		// Get the entity with its embedding
		entity, err := entityStore.GetByID(ctx, entityID)
		if err != nil || len(entity.Embedding) == 0 {
			if err := w.transactions.UpdatePatternDetectionResult(ctx, txnID, "none", nil); err != nil {
				slog.WarnContext(ctx, "failed to mark txn as none (no entity embedding)", "txn", txnID, "error", err)
			}
			continue
		}

		// Find similar entities that have recurring patterns
		similarEntities, err := entityStore.FindSimilarEntities(ctx, entity.Embedding, 10, 0.75)
		if err != nil || len(similarEntities) == 0 {
			if err := w.transactions.UpdatePatternDetectionResult(ctx, txnID, "none", nil); err != nil {
				slog.WarnContext(ctx, "failed to mark txn as none (no similar entities)", "txn", txnID, "error", err)
			}
			continue
		}

		// Check if any similar entity has a recurring_bill pattern
		var suggestedPattern *models.ExistingPattern
		var similarity float32
		var sourceEntityName string

		for _, similar := range similarEntities {
			// Skip self
			if similar.Entity.ID == entityID {
				continue
			}

			// Check if this similar entity has recurring patterns
			pattern, err := w.transactions.GetExistingPatternForEntity(ctx, similar.Entity.ID, ledgerID)
			if err != nil || pattern == nil {
				continue
			}

			// Found a similar entity with a pattern!
			if pattern.PatternType == "recurring_bill" {
				suggestedPattern = pattern
				similarity = similar.Similarity
				sourceEntityName = similar.Entity.Name
				break
			}
		}

		if suggestedPattern != nil {
			// Apply the suggested pattern with adjusted confidence
			metadata := make(map[string]interface{})
			metadata["frequency"] = suggestedPattern.Frequency
			if suggestedPattern.PatternName != "" {
				metadata["pattern_name"] = suggestedPattern.PatternName
			}
			// Adjust confidence based on similarity
			adjustedConfidence := int(float32(suggestedPattern.Confidence) * similarity)
			metadata["confidence"] = adjustedConfidence
			metadata["cold_start"] = true
			metadata["similar_entity"] = sourceEntityName
			metadata["similarity"] = similarity

			if err := w.transactions.UpdatePatternDetectionResult(ctx, txnID, suggestedPattern.PatternType, metadata); err == nil {
				processed++
				slog.DebugContext(ctx, "Cold-start pattern suggested",
					"worker", workerID,
					"txn_id", txnID.String(),
					"entity", entity.Name,
					"similar_to", sourceEntityName,
					"similarity", fmt.Sprintf("%.0f%%", similarity*100),
					"pattern", suggestedPattern.PatternType)
			}
		} else {
			// No similar entity has patterns - mark as 'none'
			if err := w.transactions.UpdatePatternDetectionResult(ctx, txnID, "none", nil); err != nil {
				slog.WarnContext(ctx, "failed to mark txn as none (no patterns in similar entities)", "txn", txnID, "error", err)
			}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "Cold-start pattern rows error", "worker", workerID, "err", err)
		return false
	}

	return processed > 0
}

// processP2PBatch handles P2P transactions with a specialized prompt
func (w *Worker) processP2PBatch(ctx context.Context, ledgerID uuid.UUID, transactions []*models.Transaction, tags []*models.Tag, tagMap map[string]uuid.UUID) {
	// Get household members for context
	householdStore := models.NewHouseholdMemberStore(w.transactions.GetPool())
	householdMembers, _ := householdStore.GetByLedgerID(ctx, ledgerID)
	var householdPatterns []string
	for _, m := range householdMembers {
		householdPatterns = append(householdPatterns, m.NamePattern)
	}

	// Build P2P contexts
	contexts := make([]P2PTransactionContext, 0, len(transactions))
	txnMap := make(map[uuid.UUID]*models.Transaction)
	for _, txn := range transactions {
		if len(txn.Entries) == 0 {
			if err := w.transactions.LoadEntries(ctx, txn); err != nil {
				slog.WarnContext(ctx, "failed to load transaction entries", "transaction_id", txn.ID, "error", err)
			}
		}

		var amount int64
		var accountName string
		for _, e := range txn.Entries {
			if e.AccountType == models.AccountTypeAsset || e.AccountType == models.AccountTypeLiability {
				amount = e.AmountCents
				accountName = e.AccountName
				if e.AccountType == models.AccountTypeLiability {
					amount = -amount
				}
				break
			}
		}

		// Get recent P2P to same person for pattern detection
		var recentP2P []string
		if txn.CounterpartyName != "" {
			recent, _ := w.transactions.GetP2PTransactionsByCounterparty(ctx, ledgerID, txn.CounterpartyName, 5)
			for _, r := range recent {
				recentP2P = append(recentP2P, r.Description)
			}
		}

		contexts = append(contexts, P2PTransactionContext{
			TransactionContext: TransactionContext{
				ID:               txn.ID,
				Description:      txn.Description,
				Amount:           amount,
				AccountName:      accountName,
				CounterpartyName: txn.CounterpartyName,
				CounterpartyType: txn.CounterpartyType,
				TellerType:       txn.TellerType,
			},
			HouseholdMembers: householdPatterns,
			RecentP2P:        recentP2P,
		})
		txnMap[txn.ID] = txn
	}

	// Call orchestrator with P2P task
	llmStart := time.Now()

	// Check if orchestrator is configured
	if w.orchestrator == nil || !w.orchestrator.IsConfigured() {
		slog.ErrorContext(ctx, "orchestrator not configured, skipping P2P categorization")
		observability.CaptureFailure(ctx, fmt.Errorf("orchestrator not configured"), observability.FailureOptions{
			Component: "processing_worker",
			Operation: "orchestrator_execute",
			Tags: map[string]string{
				"ledger_id":            ledgerID.String(),
				"transaction_count":    fmt.Sprintf("%d", len(transactions)),
				"categorization_batch": "p2p",
			},
		})
		w.markCategorizationStatus(ctx, transactions, models.CategorizationStatusFailed, "LLM not configured")
		return
	}

	// Convert inputs to orchestrator format
	transactionInputs := make([]interface{}, len(contexts))
	for i, ctx := range contexts {
		transactionInputs[i] = ctx
	}

	tagInputs := make([]interface{}, len(tags))
	for i, tag := range tags {
		tagInputs[i] = tag
	}

	// Get strategy from config (default to escalate for cost optimization)
	strategy := w.cfg.LLMDefaultStrategy
	if strategy == "" {
		strategy = "escalate" // Default to escalate for worker
	}

	// Fetch "My Life" relationships for context (especially important for P2P)
	relationshipStore := models.NewRelationshipStore(w.transactions.GetPool())
	relationships, err := relationshipStore.GetByLedgerID(ctx, ledgerID)
	if err != nil {
		slog.WarnContext(ctx, "failed to load relationships for P2P, proceeding without", "ledger", ledgerID, "error", err)
		relationships = nil
	}

	// Create task for orchestrator (P2P categorization) using provided builder
	var resultInterface interface{}
	if w.p2pTaskBuilder == nil {
		slog.ErrorContext(ctx, "P2P task builder not provided", "ledger", ledgerID)
		observability.CaptureFailure(ctx, fmt.Errorf("p2p task builder not provided"), observability.FailureOptions{
			Component: "processing_worker",
			Operation: "build_p2p_task",
			Tags: map[string]string{
				"ledger_id":            ledgerID.String(),
				"transaction_count":    fmt.Sprintf("%d", len(transactions)),
				"categorization_batch": "p2p",
			},
		})
		w.markCategorizationStatus(ctx, transactions, models.CategorizationStatusFailed, "P2P task builder not provided")
		return
	}
	task := w.p2pTaskBuilder(ledgerID.String(), strategy, transactionInputs, tagInputs, householdPatterns, relationships)

	slog.DebugContext(ctx, "calling orchestrator for P2P batch", "count", len(transactions), "ledger", ledgerID, "strategy", strategy)
	resultInterface, err = w.orchestrator.Execute(ctx, task)
	slog.DebugContext(ctx, "p2p_llm_batch timing", "count", len(transactions), "duration", time.Since(llmStart))

	var results []LLMResult
	if err == nil {
		results, err = extractLLMResults(resultInterface)
	}

	if err != nil {
		slog.ErrorContext(ctx, "P2P LLM batch failed", "error", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "processing_worker",
			Operation: "categorize_p2p_batch",
			Tags: map[string]string{
				"ledger_id":            ledgerID.String(),
				"transaction_count":    fmt.Sprintf("%d", len(transactions)),
				"strategy":             strategy,
				"categorization_batch": "p2p",
			},
		})
		for _, txn := range transactions {
			if sErr := w.transactions.UpdateCategorizationStatus(ctx, txn.ID, models.CategorizationStatusFailed, err.Error()); sErr != nil {
				slog.WarnContext(ctx, "failed to mark categorization failed", "txn", txn.ID, "error", sErr)
			}
		}
		return
	}

	// Build a map of results by transaction_id for efficient lookup
	resultMap := make(map[uuid.UUID]*LLMResult)
	unmatchedResults := make([]*LLMResult, 0)

	for i := range results {
		result := &results[i]
		if result.TransactionID != "" {
			parsedID, err := uuid.Parse(result.TransactionID)
			if err == nil {
				if _, exists := txnMap[parsedID]; exists {
					resultMap[parsedID] = result
					continue
				}
			}
		}
		// Result doesn't have a valid transaction_id, use index-based matching
		unmatchedResults = append(unmatchedResults, result)
	}

	// Process transactions: match by ID first, then by index for unmatched results
	unmatchedIndex := 0

	for _, txnCtx := range contexts {
		txnID := txnCtx.ID
		txn := txnMap[txnID]

		// Try to find result by transaction_id first
		result := resultMap[txnID]
		if result == nil {
			// Fallback to index-based matching if we have unmatched results
			if unmatchedIndex < len(unmatchedResults) {
				result = unmatchedResults[unmatchedIndex]
				unmatchedIndex++
			} else {
				// No result available for this transaction
				if sErr := w.transactions.UpdateCategorizationStatus(ctx, txnID, models.CategorizationStatusFailed, "LLM did not return a result for this P2P transaction"); sErr != nil {
					slog.WarnContext(ctx, "failed to mark categorization failed", "txn", txnID, "error", sErr)
				}
				continue
			}
		}

		tagApplied := w.applyTagByCategory(ctx, txnID, result.Category, tagMap)

		// Save title (usually the person's name)
		if result.Title != "" {
			if err := w.transactions.SetDisplayTitle(ctx, txnID, result.Title); err != nil {
				slog.WarnContext(ctx, "failed to set display title", "txn", txnID, "error", err)
			}
		}

		// Update transfer type if LLM provided a more specific one
		if result.TransferType != "" && result.TransferType != txn.TransferType {
			if err := w.transactions.SetTransferType(ctx, txnID, result.TransferType); err != nil {
				slog.WarnContext(ctx, "failed to set transfer type", "txn", txnID, "error", err)
			}
		}

		// Only mark as done if a tag was successfully applied
		if tagApplied {
			slog.DebugContext(ctx, "P2P TAGGED", "txn", txnID, "category", result.Category, "transfer_type", result.TransferType, "title", result.Title)
			if sErr := w.transactions.UpdateCategorizationStatus(ctx, txnID, models.CategorizationStatusDone, ""); sErr != nil {
				slog.WarnContext(ctx, "failed to mark categorization done", "txn", txnID, "error", sErr)
			}
		} else {
			errorMsg := fmt.Sprintf("No matching tag found for LLM category: %q", result.Category)
			if sErr := w.transactions.UpdateCategorizationStatus(ctx, txnID, models.CategorizationStatusFailed, errorMsg); sErr != nil {
				slog.WarnContext(ctx, "failed to mark categorization failed", "txn", txnID, "error", sErr)
			}
			slog.DebugContext(ctx, "P2P MARKED FAILED", "txn", txnID, "reason", errorMsg)
		}
	}
}

// WorkerPurchaseMatcher implements PurchaseMatcher for the processing worker
type WorkerPurchaseMatcher struct {
	pool *pgxpool.Pool
}

// FindMatchingPurchase finds a purchase that a credit might be offsetting
// Searches within ±5% of the amount to handle fees/taxes/partial refunds
func (m *WorkerPurchaseMatcher) FindMatchingPurchase(ctx context.Context, amountCents int64, accountName string) (*MatchingPurchase, error) {
	// Look for a purchase within ±5% of the amount from the same account
	// within the last 180 days, that has been categorized
	var purchase MatchingPurchase
	var tagName sql.NullString

	// Calculate range: ±5% or at least ±$5
	margin := amountCents / 20 // 5%
	if margin < 500 {
		margin = 500 // at least $5
	}
	minAmount := amountCents - margin
	maxAmount := amountCents + margin

	err := m.pool.QueryRow(ctx, `
		SELECT t.id, t.description, t.date, tag.name
		FROM transactions t
		JOIN entries e ON t.id = e.transaction_id
		JOIN accounts a ON e.account_id = a.id
		LEFT JOIN transaction_tags tt ON t.id = tt.transaction_id
		LEFT JOIN tags tag ON tt.tag_id = tag.id
		WHERE LOWER(a.name) = LOWER($1)
		AND e.amount_cents BETWEEN $2 AND $3
		AND e.amount_cents > 0
		AND t.date >= NOW() - INTERVAL '2 years'
		AND t.categorization_status = 'done'
		ORDER BY ABS(e.amount_cents - $4) ASC, t.date DESC
		LIMIT 1
	`, accountName, minAmount, maxAmount, amountCents).Scan(&purchase.ID, &purchase.Description, &purchase.Date, &tagName)

	if err != nil {
		return nil, err
	}

	if tagName.Valid {
		purchase.Category = tagName.String
	}

	return &purchase, nil
}
