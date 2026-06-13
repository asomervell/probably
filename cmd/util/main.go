package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/insights"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/sync"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := config.Load()
	if err := cfg.RequireDatabaseURL(); err != nil {
		log.Fatalf("%v", err)
	}

	// Connect to database
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	switch command {
	case "migrate":
		runMigrations(database)
	case "migrate-status":
		migrationStatus(database)
	case "prune-entities":
		pruneEntities(ctx, database)
	case "backtest-insights":
		backtestInsights(ctx, cfg, database, os.Args[2:])
	case "requeue-enrichment":
		requeueEnrichment(ctx, database, os.Args[2:])
	case "plaid-sync-accounts":
		plaidSyncAccounts(ctx, cfg, database)
	case "firecrawl-cache-stats":
		firecrawlCacheStats(ctx, database, os.Args[2:])
	case "seed":
		seedDevUser(ctx, database)
	case "embedding":
		EmbeddingSubcommand()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Probably Utility CLI")
	fmt.Println()
	fmt.Println("Usage: util <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  migrate              Run pending database migrations")
	fmt.Println("  migrate-status       Show migration status")
	fmt.Println("  plaid-sync-accounts  Re-fetch Plaid account list + run transaction sync (needs valid access tokens in DB)")
	fmt.Println("  prune-entities       Remove entities with no associated transactions")
	fmt.Println("  requeue-enrichment   Reset all transactions for re-enrichment and clean up entities")
	fmt.Println("  backtest-insights    Back-test LLM insights on historical data")
	fmt.Println("  firecrawl-cache-stats Show Firecrawl cache hit/miss statistics")
	fmt.Println("  seed                 Seed dev user (dev@probably.test / devsecret1) for cloud environments")
	fmt.Println("  embedding            Generate and manage vector embeddings for entities")
	fmt.Println("  help                 Show this help message")
	fmt.Println()
	fmt.Println("Run 'util <command> --help' for command-specific options")
	fmt.Println()
}

const (
	devEmail    = "dev@probably.test"
	devPassword = "devsecret1"
)

func seedDevUser(ctx context.Context, database *db.DB) {
	users := models.NewUserStore(database.Pool)
	ledgers := models.NewLedgerStore(database.Pool)

	// Idempotent: skip if user already exists
	existing, err := users.GetByEmail(ctx, devEmail)
	if err == nil && existing != nil {
		fmt.Printf("✅ Dev user already exists: %s\n", devEmail)
		fmt.Printf("   Password: %s\n", devPassword)
		ledger, lerr := ledgers.GetByUserID(ctx, existing.ID)
		if lerr == nil {
			fmt.Printf("   Ledger ID: %s\n", ledger.ID)
		}
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(devPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	user := &models.User{
		ID:        uuid.New(),
		Email:     devEmail,
		Password:  string(hash),
		Confirmed: true,
	}
	if err := users.Create(ctx, user); err != nil {
		log.Fatalf("Failed to create dev user: %v", err)
	}

	ledger := &models.Ledger{
		ID:       uuid.New(),
		UserID:   user.ID,
		Name:     "Dev Ledger",
		Currency: "USD",
	}
	if err := ledgers.Create(ctx, ledger); err != nil {
		log.Fatalf("Failed to create dev ledger: %v", err)
	}

	fmt.Println("✅ Dev user seeded:")
	fmt.Printf("   Email:     %s\n", devEmail)
	fmt.Printf("   Password:  %s\n", devPassword)
	fmt.Printf("   User ID:   %s\n", user.ID)
	fmt.Printf("   Ledger ID: %s\n", ledger.ID)
}

func runMigrations(database *db.DB) {
	fmt.Println("Running database migrations...")
	if err := db.Migrate(database); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	fmt.Println("✅ Migrations complete")
}

// plaidSyncAccounts re-runs Plaid /accounts and transaction sync for every distinct
// (ledger, item) that still has a stored Plaid access token. Use this after
// schema changes (e.g. account uniqueness) so missing accounts are created.
func plaidSyncAccounts(ctx context.Context, cfg *config.Config, database *db.DB) {
	if cfg.PlaidClientID == "" || strings.TrimSpace(cfg.PlaidSecret()) == "" {
		log.Fatal("Plaid is not configured (set PLAID_CLIENT_ID and the secret for the current PLAID_ENVIRONMENT)")
	}
	plaidClient, err := sync.NewPlaidClient(cfg)
	if err != nil {
		log.Fatalf("plaid client: %v", err)
	}
	svc := sync.NewPlaidSyncService(database.Pool, plaidClient, cfg)
	store := models.NewAccountStore(database.Pool)

	accounts, err := store.GetAllWithProviderCredentials(ctx)
	if err != nil {
		log.Fatalf("list accounts: %v", err)
	}

	type connKey struct {
		ledgerID uuid.UUID
		itemID   string
	}
	byConn := make(map[connKey]string) // one access token per (ledger, item)
	for _, a := range accounts {
		if a.Provider != "plaid" || a.ConnectionID == "" || a.AccessToken == "" {
			continue
		}
		k := connKey{ledgerID: a.LedgerID, itemID: a.ConnectionID}
		if _, ok := byConn[k]; !ok {
			byConn[k] = a.AccessToken
		}
	}
	if len(byConn) == 0 {
		log.Println("No Plaid connections with a stored access token. Open the app and reconnect Chase (or use /connections/reconnect/plaid) so tokens are saved, then run this again.")
		return
	}

	for k, accessToken := range byConn {
		log.Printf("Plaid: syncing accounts for item_id=%s ledger=%s", k.itemID, k.ledgerID)
		synced, err := svc.SyncAccounts(ctx, k.ledgerID, accessToken)
		if err != nil {
			log.Printf("  SyncAccounts: %v", err)
			continue
		}
		log.Printf("  %d account(s) returned from Plaid", len(synced))
		for _, acc := range synced {
			n, err := svc.SyncTransactions(ctx, acc)
			if err != nil {
				log.Printf("  %s: transactions: %v", acc.Name, err)
				continue
			}
			log.Printf("  %s: %d new transaction(s) imported", acc.Name, n)
		}
	}
}

func migrationStatus(database *db.DB) {
	fmt.Println("Checking migration status...")
	if err := db.MigrationStatus(database); err != nil {
		log.Fatalf("Failed to get migration status: %v", err)
	}
}

func pruneEntities(ctx context.Context, database *db.DB) {
	entityStore := models.NewEntityStore(database.Pool)

	// First show how many orphans exist
	count, err := entityStore.CountOrphans(ctx)
	if err != nil {
		log.Fatalf("Failed to count orphan entities: %v", err)
	}

	if count == 0 {
		fmt.Println("✅ No orphan entities to prune")
		return
	}

	fmt.Printf("Found %d orphan entities (no associated transactions)\n", count)

	// Delete them
	deleted, err := entityStore.PruneOrphans(ctx)
	if err != nil {
		log.Fatalf("Failed to prune orphan entities: %v", err)
	}

	fmt.Printf("✅ Pruned %d orphan entities\n", deleted)
}

func requeueEnrichment(ctx context.Context, database *db.DB, args []string) {
	// Parse flags for this subcommand
	fs := flag.NewFlagSet("requeue-enrichment", flag.ExitOnError)
	includeUserVerified := fs.Bool("include-user-verified", false, "Also remove user-verified entities (default: preserve them)")
	dryRun := fs.Bool("dry-run", false, "Preview changes without modifying the database")

	fs.Usage = func() {
		fmt.Println("Usage: util requeue-enrichment [options]")
		fmt.Println()
		fmt.Println("Reset all transactions for re-enrichment and clean up entities.")
		fmt.Println("This is useful when you want to re-run entity matching with improved logic.")
		fmt.Println()
		fmt.Println("Steps performed:")
		fmt.Println("  1. Unlink entities from transactions (preserves user-verified by default)")
		fmt.Println("  2. Reset enrichment status to 'pending' for all transactions")
		fmt.Println("  3. Delete orphan entities (those no longer linked to any transaction)")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  util requeue-enrichment --dry-run")
		fmt.Println("  util requeue-enrichment --include-user-verified")
		fmt.Println()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *dryRun {
		fmt.Println("🔍 DRY RUN MODE - No changes will be made")
		fmt.Println()
	}

	// Step 1: Count and unlink entities
	var txnCount int
	var unlinkQuery string
	if *includeUserVerified {
		// Count all transactions with entities
		err := database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM transactions WHERE entity_id IS NOT NULL").Scan(&txnCount)
		if err != nil {
			log.Fatalf("Failed to count transactions: %v", err)
		}
		fmt.Printf("Step 1: Found %d transactions with entities (including user-verified)\n", txnCount)
		unlinkQuery = "UPDATE transactions SET entity_id = NULL, counterparty_entity_id = NULL, intermediary_entity_id = NULL"
	} else {
		// Count transactions with non-user-verified entities
		err := database.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM transactions t
			JOIN entities e ON t.entity_id = e.id
			WHERE NOT e.user_verified
		`).Scan(&txnCount)
		if err != nil {
			log.Fatalf("Failed to count transactions: %v", err)
		}
		fmt.Printf("Step 1: Found %d transactions with non-user-verified entities\n", txnCount)
		unlinkQuery = `
			UPDATE transactions t
			SET entity_id = NULL, counterparty_entity_id = NULL, intermediary_entity_id = NULL
			FROM entities e
			WHERE t.entity_id = e.id AND NOT e.user_verified
		`
	}

	if !*dryRun && txnCount > 0 {
		result, err := database.Pool.Exec(ctx, unlinkQuery)
		if err != nil {
			log.Fatalf("Failed to unlink entities: %v", err)
		}
		fmt.Printf("  ✅ Unlinked entities from %d transactions\n", result.RowsAffected())
	} else if *dryRun {
		fmt.Printf("  [DRY RUN] Would unlink entities from %d transactions\n", txnCount)
	}

	// Step 2: Reset categorization status (enrichment_status is no longer used)
	var pendingCount int
	err := database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM transactions").Scan(&pendingCount)
	if err != nil {
		log.Fatalf("Failed to count transactions: %v", err)
	}
	fmt.Printf("Step 2: Resetting categorization status for %d transactions\n", pendingCount)

	if !*dryRun {
		_, err := database.Pool.Exec(ctx, "UPDATE transactions SET categorization_status = 'queued', categorization_attempts = 0")
		if err != nil {
			log.Fatalf("Failed to reset enrichment status: %v", err)
		}
		fmt.Println("  ✅ Reset categorization status to 'queued'")
	} else {
		fmt.Println("  [DRY RUN] Would reset categorization status to 'queued'")
	}

	// Step 3: Count and delete orphan entities
	var orphanCount int
	err = database.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM entities
		WHERE NOT EXISTS (
			SELECT 1 FROM transactions t WHERE t.entity_id = entities.id
		)
	`).Scan(&orphanCount)
	if err != nil {
		log.Fatalf("Failed to count orphan entities: %v", err)
	}
	fmt.Printf("Step 3: Found %d orphan entities to delete\n", orphanCount)

	if !*dryRun && orphanCount > 0 {
		result, err := database.Pool.Exec(ctx, `
			DELETE FROM entities
			WHERE NOT EXISTS (
				SELECT 1 FROM transactions t WHERE t.entity_id = entities.id
			)
		`)
		if err != nil {
			log.Fatalf("Failed to delete orphan entities: %v", err)
		}
		fmt.Printf("  ✅ Deleted %d orphan entities\n", result.RowsAffected())
	} else if *dryRun && orphanCount > 0 {
		fmt.Printf("  [DRY RUN] Would delete %d orphan entities\n", orphanCount)
	}

	fmt.Println()
	if *dryRun {
		fmt.Println("🔍 DRY RUN complete. Run without --dry-run to apply changes.")
	} else {
		fmt.Println("✅ Re-enrichment queued! Start the server to begin processing.")
	}
}

func backtestInsights(ctx context.Context, cfg *config.Config, database *db.DB, args []string) {
	// Parse flags for this subcommand
	fs := flag.NewFlagSet("backtest-insights", flag.ExitOnError)
	ledgerIDStr := fs.String("ledger", "", "Ledger UUID (required)")
	startStr := fs.String("start", "", "Start date (YYYY-MM-DD, required)")
	endStr := fs.String("end", "", "End date (YYYY-MM-DD, defaults to today)")
	dryRun := fs.Bool("dry-run", false, "Preview without saving to database")
	reportsOnly := fs.Bool("reports-only", false, "Only generate monthly reports, skip transaction insights")
	verbose := fs.Bool("verbose", false, "Verbose output")

	fs.Usage = func() {
		fmt.Println("Usage: util backtest-insights [options]")
		fmt.Println()
		fmt.Println("Back-test LLM insights on historical transaction data.")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  util backtest-insights --ledger=<uuid> --start=2024-01-01 --end=2024-12-31")
		fmt.Println()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Validate required flags
	if *ledgerIDStr == "" {
		fmt.Println("Error: --ledger is required")
		fs.Usage()
		os.Exit(1)
	}

	if *startStr == "" {
		fmt.Println("Error: --start is required")
		fs.Usage()
		os.Exit(1)
	}

	// Parse ledger ID
	ledgerID, err := uuid.Parse(*ledgerIDStr)
	if err != nil {
		fmt.Printf("Error: invalid ledger ID: %v\n", err)
		os.Exit(1)
	}

	// Parse start date
	startDate, err := time.Parse("2006-01-02", *startStr)
	if err != nil {
		fmt.Printf("Error: invalid start date (use YYYY-MM-DD): %v\n", err)
		os.Exit(1)
	}

	// Parse end date (default to today)
	endDate := time.Now()
	if *endStr != "" {
		endDate, err = time.Parse("2006-01-02", *endStr)
		if err != nil {
			fmt.Printf("Error: invalid end date (use YYYY-MM-DD): %v\n", err)
			os.Exit(1)
		}
	}

	if endDate.Before(startDate) {
		fmt.Println("Error: end date must be after start date")
		os.Exit(1)
	}

	// Verify ledger exists
	var exists bool
	err = database.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM ledgers WHERE id = $1)", ledgerID).Scan(&exists)
	if err != nil || !exists {
		fmt.Printf("Error: ledger %s not found\n", ledgerID)
		os.Exit(1)
	}

	// Create reporter and analyzer
	reporter := insights.NewReporter(cfg, database.Pool)
	analyzer := insights.NewAnalyzer(cfg, database.Pool)

	if !reporter.IsConfigured() && !analyzer.IsConfigured() {
		fmt.Println("Error: No LLM providers configured. Set XAI_API_KEY, VERTEX_API_KEY, or GROQ_API_KEY")
		os.Exit(1)
	}

	fmt.Printf("Back-testing insights for ledger %s\n", ledgerID)
	fmt.Printf("Period: %s to %s\n", startDate.Format("Jan 2, 2006"), endDate.Format("Jan 2, 2006"))
	if *dryRun {
		fmt.Println("Mode: DRY RUN (no changes will be saved)")
	}
	fmt.Println()

	// Get all transactions in the date range
	txnStore := models.NewTransactionStore(database.Pool)
	txns, _, err := txnStore.List(ctx, models.TransactionFilter{
		LedgerID:  ledgerID,
		StartDate: &startDate,
		EndDate:   &endDate,
		Limit:     100000,
	})
	if err != nil {
		log.Fatalf("Failed to get transactions: %v", err)
	}

	fmt.Printf("Found %d transactions in date range\n\n", len(txns))

	// Sort transactions by date (oldest first for chronological processing)
	sort.Slice(txns, func(i, j int) bool {
		return txns[i].Date.Before(txns[j].Date)
	})

	// Track statistics
	var reportsGenerated, transactionInsights, reportErrors, transactionErrors int

	// Process by month
	currentMonth := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, time.Local)
	for currentMonth.Before(endDate) || currentMonth.Equal(time.Date(endDate.Year(), endDate.Month(), 1, 0, 0, 0, 0, time.Local)) {
		monthEnd := currentMonth.AddDate(0, 1, 0).Add(-time.Second)
		if monthEnd.After(endDate) {
			monthEnd = endDate
		}

		fmt.Printf("=== %s ===\n", currentMonth.Format("January 2006"))

		// Get transactions for this month
		var monthTxns []*models.Transaction
		for _, txn := range txns {
			if (txn.Date.Equal(currentMonth) || txn.Date.After(currentMonth)) && (txn.Date.Before(monthEnd) || txn.Date.Equal(monthEnd)) {
				monthTxns = append(monthTxns, txn)
			}
		}

		fmt.Printf("  Transactions: %d\n", len(monthTxns))

		// Analyze individual transactions (if not reports-only)
		if !*reportsOnly && analyzer.IsConfigured() {
			insightsGenerated := 0
			for _, txn := range monthTxns {
				// Load entries and tags
				_ = txnStore.LoadEntries(ctx, txn)
				_ = txnStore.LoadTags(ctx, txn)

				// Skip transfers
				if txn.IsTransfer {
					continue
				}

				if *verbose {
					fmt.Printf("    Analyzing: %s - %s\n", txn.Date.Format("Jan 2"), truncateStr(txn.Description, 40))
				}

				if !*dryRun {
					insight, err := analyzer.AnalyzeTransaction(ctx, txn)
					if err != nil {
						if *verbose {
							fmt.Printf("      Error: %v\n", err)
						}
						transactionErrors++
						continue
					}
					if insight != nil {
						insightsGenerated++
						transactionInsights++
						if *verbose {
							fmt.Printf("      Insight (importance=%d): %s\n", insight.Importance, truncateStr(insight.Content, 60))
						}
					}
				} else {
					// Dry run - just count
					transactionInsights++
				}

				// Rate limit to avoid API throttling
				time.Sleep(500 * time.Millisecond)
			}
			fmt.Printf("  Transaction insights: %d\n", insightsGenerated)
		}

		// Generate monthly report (only for completed months)
		if reporter.IsConfigured() {
			nextMonth := currentMonth.AddDate(0, 1, 0)
			if nextMonth.Before(time.Now()) || nextMonth.Equal(time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)) {
				fmt.Printf("  Generating monthly report...\n")

				if !*dryRun {
					report, err := reporter.GenerateMonthlyReport(ctx, ledgerID, currentMonth.Year(), currentMonth.Month())
					if err != nil {
						fmt.Printf("    Error: %v\n", err)
						reportErrors++
					} else {
						reportsGenerated++
						fmt.Printf("    Income: %s, Expenses: %s, Net: %s\n",
							models.FormatCents(report.TotalIncomeCents),
							models.FormatCents(report.TotalExpensesCents),
							models.FormatCents(report.NetSavingsCents))
						if report.Summary != "" {
							fmt.Printf("    Summary: %s\n", truncateStr(report.Summary, 100))
						}
					}
				} else {
					reportsGenerated++
					fmt.Printf("    [DRY RUN] Would generate report\n")
				}
			}
		}

		fmt.Println()

		// Move to next month
		currentMonth = currentMonth.AddDate(0, 1, 0)
	}

	// Print summary
	fmt.Println("=== SUMMARY ===")
	fmt.Printf("Reports generated: %d\n", reportsGenerated)
	fmt.Printf("Transaction insights: %d\n", transactionInsights)
	if reportErrors > 0 {
		fmt.Printf("Report errors: %d\n", reportErrors)
	}
	if transactionErrors > 0 {
		fmt.Printf("Transaction errors: %d\n", transactionErrors)
	}

	if *dryRun {
		fmt.Println("\n[DRY RUN] No changes were saved. Remove --dry-run to save results.")
	}
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}


func firecrawlCacheStats(ctx context.Context, database *db.DB, args []string) {
	// Parse flags for this subcommand
	fs := flag.NewFlagSet("firecrawl-cache-stats", flag.ExitOnError)
	logFile := fs.String("log", "", "Path to application log file to parse for hit/miss counts (optional)")
	fs.Usage = func() {
		fmt.Println("Usage: util firecrawl-cache-stats [options]")
		fmt.Println()
		fmt.Println("Show Firecrawl cache statistics from the database.")
		fmt.Println("Optionally parse log files to calculate hit/miss rates.")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  util firecrawl-cache-stats")
		fmt.Println("  util firecrawl-cache-stats --log=logs/processing.log")
		fmt.Println()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	fmt.Println("=== Firecrawl Cache Statistics ===")
	fmt.Println()

	// Query database for cache statistics
	var totalEntries, searchEntries, scrapeEntries int
	var expiredEntries int

	// Total entries
	err := database.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM firecrawl_cache
	`).Scan(&totalEntries)
	if err != nil {
		log.Fatalf("Failed to query cache: %v", err)
	}

	// Entries by type
	err = database.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM firecrawl_cache WHERE cache_type = 'search'
	`).Scan(&searchEntries)
	if err != nil {
		log.Fatalf("Failed to query search cache: %v", err)
	}

	err = database.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM firecrawl_cache WHERE cache_type = 'scrape'
	`).Scan(&scrapeEntries)
	if err != nil {
		log.Fatalf("Failed to query scrape cache: %v", err)
	}

	// Expired entries
	err = database.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM firecrawl_cache WHERE expires_at <= NOW()
	`).Scan(&expiredEntries)
	if err != nil {
		log.Fatalf("Failed to query expired cache: %v", err)
	}

	// Active (non-expired) entries
	activeEntries := totalEntries - expiredEntries

	fmt.Println("Database Cache Statistics:")
	fmt.Printf("  Total entries: %d\n", totalEntries)
	fmt.Printf("  Active (non-expired): %d\n", activeEntries)
	fmt.Printf("  Expired: %d\n", expiredEntries)
	fmt.Printf("  Search entries: %d\n", searchEntries)
	fmt.Printf("  Scrape entries: %d\n", scrapeEntries)
	fmt.Println()

	// Get oldest and newest cache entries
	var oldestCreated, newestCreated time.Time
	var oldestExpires, newestExpires time.Time

	err = database.Pool.QueryRow(ctx, `
		SELECT MIN(created_at), MAX(created_at), MIN(expires_at), MAX(expires_at)
		FROM firecrawl_cache
	`).Scan(&oldestCreated, &newestCreated, &oldestExpires, &newestExpires)
	if err == nil && totalEntries > 0 {
		fmt.Println("Cache Age:")
		fmt.Printf("  Oldest entry created: %s\n", oldestCreated.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Newest entry created: %s\n", newestCreated.Format("2006-01-02 15:04:05"))
		if !oldestExpires.IsZero() {
			fmt.Printf("  Earliest expiration: %s\n", oldestExpires.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Latest expiration: %s\n", newestExpires.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()
	}

	// Parse log file if provided
	if *logFile != "" {
		fmt.Println("Parsing log file for hit/miss statistics...")
		hits, misses := parseFirecrawlLogs(*logFile)
		totalLookups := hits + misses
		if totalLookups > 0 {
			hitRate := float64(hits) / float64(totalLookups) * 100
			fmt.Printf("  Cache hits: %d\n", hits)
			fmt.Printf("  Cache misses: %d\n", misses)
			fmt.Printf("  Total lookups: %d\n", totalLookups)
			fmt.Printf("  Hit rate: %.1f%%\n", hitRate)
			fmt.Printf("  Miss rate: %.1f%%\n", 100-hitRate)
		} else {
			fmt.Println("  No cache hit/miss entries found in log file")
		}
		fmt.Println()
	} else {
		fmt.Println("Note: To see hit/miss rates, provide a log file with --log")
		fmt.Println("      (e.g., --log=logs/processing.log or stdout/stderr output)")
		fmt.Println()
	}

	// Show breakdown by cache type
	if totalEntries > 0 {
		fmt.Println("Cache Breakdown by Type:")
		rows, err := database.Pool.Query(ctx, `
			SELECT 
				cache_type,
				COUNT(*) as count,
				COUNT(*) FILTER (WHERE expires_at > NOW()) as active,
				COUNT(*) FILTER (WHERE expires_at <= NOW()) as expired
			FROM firecrawl_cache
			GROUP BY cache_type
			ORDER BY cache_type
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var cacheType string
				var count, active, expired int
				if err := rows.Scan(&cacheType, &count, &active, &expired); err == nil {
					fmt.Printf("  %s: %d total (%d active, %d expired)\n", cacheType, count, active, expired)
				}
			}
			if err := rows.Err(); err != nil {
				fmt.Printf("  row error: %v\n", err)
			}
		}
	}
}

// parseFirecrawlLogs parses a log file to count cache hits and misses
func parseFirecrawlLogs(logPath string) (hits, misses int) {
	file, err := os.Open(logPath)
	if err != nil {
		fmt.Printf("Warning: Could not open log file %s: %v\n", logPath, err)
		return 0, 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "FIRECRAWL CACHE HIT") {
			hits++
		} else if strings.Contains(line, "FIRECRAWL CACHE MISS") {
			misses++
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Warning: Error reading log file: %v\n", err)
	}

	return hits, misses
}
