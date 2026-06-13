package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/embedding"
	"github.com/asomervell/probably/internal/handlers"
	"github.com/asomervell/probably/internal/insights"
	"github.com/asomervell/probably/internal/mcp"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/orchestrator"
	"github.com/asomervell/probably/internal/processing"
	"github.com/asomervell/probably/internal/sync"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
)

// Runtime modes
const (
	RuntimeModeApp    = "app"    // Web server + API only
	RuntimeModeWorker = "worker" // Processing workers only
	RuntimeModeBoth   = "both"   // Both (default for backward compatibility)
)

func main() {
	// Parse runtime mode flag (can also be set via RUNTIME_MODE env var for air compatibility)
	runtimeMode := flag.String("mode", "", "Runtime mode: 'app' (web server only), 'worker' (workers only), 'both' (default)")
	flag.Parse()

	mode := *runtimeMode
	// If not set via flag, check environment variable (for air compatibility)
	if mode == "" {
		if envMode := os.Getenv("RUNTIME_MODE"); envMode != "" {
			mode = envMode
		} else {
			mode = RuntimeModeBoth // Default
		}
	}

	if mode != RuntimeModeApp && mode != RuntimeModeWorker && mode != RuntimeModeBoth {
		log.Fatalf("Invalid runtime mode: %s. Must be 'app', 'worker', or 'both'", mode)
	}

	// Load .env from the process working directory (usually the repo root under air).
	// Use Overload when the file exists so values in .env win over stale exports
	// (e.g. BASE_URL=http://localhost:8080 left in the shell from an earlier session).
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Overload(".env"); err != nil {
			log.Printf("⚠️  could not load .env: %v", err)
		}
	} else {
		_ = godotenv.Load()
	}

	// Load configuration
	cfg := config.Load()
	if err := cfg.RequireDatabaseURL(); err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("🚀 Probably starting (mode=%s, env=%s)\n", mode, cfg.Environment)

	// PostHog + OpenTelemetry (OTLP logs + AI traces to PostHog)
	serviceName := "probably-app"
	if mode == RuntimeModeWorker {
		serviceName = "probably-worker"
	}
	rootCtx := context.Background()
	otelShutdown, err := observability.InitOTEL(rootCtx, cfg, serviceName)
	var otelInitErr error
	if err != nil {
		otelInitErr = err
		log.Printf("⚠️  Failed to initialize OTEL export: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if otelShutdown != nil {
			if e := otelShutdown(shutdownCtx); e != nil {
				log.Printf("OTEL shutdown: %v", e)
			}
		}
	}()
	observability.InitPostHog(cfg, serviceName)
	defer observability.ClosePostHog()
	layouts.InitPostHogFromConfig(cfg)

	if otelInitErr != nil {
		observability.CaptureFailure(context.Background(), otelInitErr, observability.FailureOptions{
			Component: "startup",
			Operation: "init_otel",
		})
	}

	phEnv := cfg.PostHogEnvironment
	if phEnv == "" {
		phEnv = cfg.Environment
	}
	log.Printf("[PostHog] project key configured: %v, env=%s", cfg.PostHogProjectAPIKey != "", phEnv)
	if observability.PostHogEnabled() {
		ctx := context.Background()
		observability.CaptureMessage(ctx, fmt.Sprintf("Probably started (mode=%s, service=%s)", mode, serviceName), nil)
	}

	// Default slog (development uses DEBUG; production uses INFO)
	initAppSlog(cfg)

	// Connect to database
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	// Run migrations
	if err := db.Migrate(database); err != nil {
		slog.Error("Failed to run migrations", "err", err)
		os.Exit(1)
	}

	// Initialize auth
	authBoss, sessionStore, err := auth.SetupAuthboss(cfg, database)
	if err != nil {
		slog.Error("Failed to setup authboss", "err", err)
		os.Exit(1)
	}

	// Initialize model stores (needed for both app and worker)
	rules := models.NewRuleStore(database.Pool)
	tags := models.NewTagStore(database.Pool)
	transactions := models.NewTransactionStore(database.Pool)
	entities := models.NewEntityStore(database.Pool)

	// Start application server (if mode is "app" or "both")
	var server *http.Server
	if mode == RuntimeModeApp || mode == RuntimeModeBoth {
		// Create router
		r := chi.NewRouter()
		r.Use(middleware.RequestID)
		r.Use(middleware.RealIP)
		r.Use(observability.NewHTTPMiddleware())
		// Structured access logs → default slog (stdout + PostHog OTLP when configured)
		r.Use(observability.SlogAccessMiddleware())
		r.Use(middleware.Recoverer)
		r.Use(middleware.Timeout(5 * time.Minute)) // Allow time for backup imports/exports

		// Static files
		// Use absolute path /static since that's where Docker copies the files
		staticDir := "static"
		if _, err := os.Stat("/static"); err == nil {
			// If /static exists (Docker container), use it
			staticDir = "/static"
		}
		fileServer := http.FileServer(http.Dir(staticDir))
		r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

		// Backward-compatible logo URL alias:
		// fallback logo URLs are stored as /logos/<filename>, while files live under
		// static/logos/logos/<filename> when running without CDN-backed logo storage.
		logoAliasDir := filepath.Join(staticDir, "logos", "logos")
		logoAliasServer := http.FileServer(http.Dir(logoAliasDir))
		r.Handle("/logos/*", http.StripPrefix("/logos/", logoAliasServer))

		// Serve MCP UI bundles at /mcp-ui/ for local development
		// This allows HTML templates to reference bundles without CDN
		mcpUIServer := http.FileServer(http.Dir(filepath.Join(staticDir, "mcp-ui")))
		r.Handle("/mcp-ui/*", http.StripPrefix("/mcp-ui/", mcpUIServer))

		// Serve install script
		r.Get("/install.sh", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			http.ServeFile(w, r, "install.sh")
		})

		// Setup handlers
		h := handlers.New(cfg, database, authBoss, sessionStore)

		// Create embedding service for V2 chat similarity search (optional)
		if embSvc, err := embedding.NewServiceFromConfig(context.Background(), cfg); err == nil {
			h.SetChatV2EmbeddingService(embSvc)
			slog.Info("embedding service configured", "component", "v2_chat")
		} else {
			slog.Warn("V2 chat similarity search disabled", "err", err)
		}

		// Setup MCP server (ChatGPT App integration) - mounted at /mcp
		// Register BEFORE main handlers so MCP can handle POST / for refresh
		if mcpServer, err := mcp.NewServer(cfg, database, authBoss); err == nil {
			// Pass Authboss LoadClientStateMiddleware so MCP can read session cookies
			mcpServer.RegisterRoutes(r, authBoss.LoadClientStateMiddleware)
			slog.Info("MCP server configured", "path", "/mcp")
		} else {
			slog.Warn("MCP server disabled", "err", err)
		}

		// Register main handlers AFTER MCP so MCP POST / takes precedence
		h.RegisterRoutes(r)

		// Create HTTP server
		port := cfg.Port
		if port == "" {
			port = "8080"
		}

		server = &http.Server{
			Addr:         ":" + port,
			Handler:      r,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 5 * time.Minute, // Allow time for backup imports/exports
			IdleTimeout:  60 * time.Second,
		}

		// Start server in goroutine
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("HTTP server error", "err", err)
				os.Exit(1)
			}
		}()

		slog.Info("app ready", "addr", "http://localhost:"+port)
	}

	// Start workers (if mode is "worker" or "both")
	var processingWorker *processing.Worker
	var insightsWorker *insights.Worker
	var syncWorker *sync.SyncWorker
	var workerServer *http.Server

	if mode == RuntimeModeWorker || mode == RuntimeModeBoth {
		// Worker runs on port 8081 for health checks
		workerPort := "8081"
		if mode == RuntimeModeBoth {
			workerPort = "" // Don't start separate health server when running both
		}

		// Start processing worker for LLM categorization
		// Requires explicit LLM_DEFAULT_MODEL configuration
		if cfg.LLMDefaultModel != "" {
			// Create orchestrator for the worker
			orch, err := orchestrator.NewOrchestrator(cfg)
			if err != nil {
				orch = nil
			}

			// Wrap orchestrator to break import cycle
			var orchWrapper processing.OrchestratorInterface
			var taskBuilder processing.TaskBuilder
			var p2pTaskBuilder processing.P2PTaskBuilder
			if orch != nil {
				orchWrapper = NewWorkerOrchestratorWrapper(orch)
				taskBuilder = func(ledgerID string, strategy string, transactionInputs, tagInputs, ruleInputs []interface{}, entitySearcher, purchaseMatcher interface{}, relationships []*models.Relationship) interface{} {
					return BuildCategorizeTask(ledgerID, strategy, transactionInputs, tagInputs, ruleInputs, entitySearcher, purchaseMatcher, relationships)
				}
				p2pTaskBuilder = func(ledgerID string, strategy string, transactionInputs, tagInputs []interface{}, householdPatterns []string, relationships []*models.Relationship) interface{} {
					return BuildP2PTask(ledgerID, strategy, transactionInputs, tagInputs, householdPatterns, relationships)
				}
			}

			processingWorker = processing.NewWorker(cfg, database.Pool, transactions, entities, tags, rules, orchWrapper, taskBuilder, p2pTaskBuilder)

			// Create embedding service for transaction embeddings (optional - continues without it)
			if embSvc, err := embedding.NewServiceFromConfig(context.Background(), cfg); err == nil {
				processingWorker.SetEmbeddingService(embSvc)
				slog.Info("embedding service configured", "component", "processing_worker")
			} else {
				slog.Warn("embedding service not configured", "err", err)
			}

			processingWorker.Start()
			slog.Info("processing worker started")
		} else {
			slog.Warn("processing worker not started", "reason", "LLM_DEFAULT_MODEL not configured")
		}

		// Start insights worker (generates AI-powered reports and insights)
		insightsWorker = insights.NewWorker(cfg, database.Pool)
		insightsWorker.Start()
		slog.Info("insights worker started")

		// Start sync worker (syncs all provider accounts at 00, 15, 30, 45 minutes past each hour)
		anyProviderConfigured := (cfg.TellerCert != "" && cfg.TellerKey != "") ||
			(cfg.AkahuAppID != "" && cfg.AkahuAppSecret != "") ||
			(cfg.PlaidClientID != "" && cfg.PlaidSecret() != "")
		if anyProviderConfigured {
			worker, err := sync.NewSyncWorker(database.Pool, cfg)
			if err == nil {
				syncWorker = worker
				syncWorker.Start()
				slog.Info("sync worker started")
			}
		} else {
			slog.Warn("sync worker not started", "reason", "no providers configured (Teller, Akahu, or Plaid)")
		}

		// Start health check server for worker mode
		if workerPort != "" {
			workerRouter := chi.NewRouter()
			workerRouter.Get("/health", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			workerServer = &http.Server{
				Addr:    ":" + workerPort,
				Handler: workerRouter,
			}

			go func() {
				if err := workerServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					slog.Error("Worker health server error", "err", err)
				}
			}()

			slog.Info("worker ready", "health_addr", "http://localhost:"+workerPort+"/health")
		}
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Wait for interrupt signal
	<-quit
	slog.Info("shutting down")

	// Stop workers (if they were started)
	if syncWorker != nil {
		syncWorker.Stop()
	}
	if insightsWorker != nil {
		insightsWorker.Stop()
	}
	if processingWorker != nil {
		processingWorker.Stop()
	}

	// Shutdown HTTP servers (if they were started)
	if server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "err", err)
		}
	}
	if workerServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := workerServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("Worker health server shutdown error", "err", err)
		}
	}

}

func initAppSlog(cfg *config.Config) {
	logLevel := slog.LevelInfo
	if cfg.Environment == "development" {
		logLevel = slog.LevelDebug
	}
	slogOpts := &slog.HandlerOptions{Level: logLevel}
	stdoutH := slog.NewTextHandler(os.Stdout, slogOpts)
	var h slog.Handler = stdoutH
	if otelH := observability.NewOTELSLogHandler(cfg); otelH != nil {
		h = observability.NewMultiSlogHandler(stdoutH, otelH)
	}
	slog.SetDefault(slog.New(h))
	observability.RedirectStdLogToSlog()
	slog.Info("slog wired (stdout + PostHog OTLP logs when POSTHOG_PROJECT_API_KEY is set)", "env", cfg.Environment, "log_level", logLevel.String())
}
