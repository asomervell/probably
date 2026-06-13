package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/handlers"
	"github.com/asomervell/probably/internal/insights"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/processing"
	"github.com/asomervell/probably/internal/sync"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed cmd/desktop/frontend
var assets embed.FS

// App struct holds the application state
type App struct {
	ctx              context.Context
	serverURL        string
	database         *db.DB
	httpServer       *http.Server
	processingWorker *processing.Worker      // Unified enrichment + categorization
	syncWorker *sync.SyncWorker  // Background sync for all providers
	insightsWorker   *insights.Worker        // AI-powered insights generation
}

// NewApp creates a new App instance
func NewApp() *App {
	return &App{}
}

// startup is called when the Wails app starts - this is where we initialize everything
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	fmt.Println("🚀 Starting Probably Desktop...")

	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Overload(".env"); err != nil {
			log.Printf("⚠️  could not load .env: %v", err)
		}
	}

	cfg := config.Load()
	if err := cfg.RequireDatabaseURL(); err != nil {
		log.Fatalf("%v", err)
	}
	cfg.Environment = "desktop"

	// Find an available port for the HTTP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	cfg.Port = fmt.Sprintf("%d", port)
	cfg.BaseURL = fmt.Sprintf("http://localhost:%d", port)
	a.serverURL = cfg.BaseURL

	// Connect to database
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	a.database = database

	// Run migrations
	fmt.Println("📊 Running database migrations...")
	if err := db.Migrate(database); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	fmt.Println("✅ Migrations complete")

	// Initialize auth
	authBoss, sessionStore, err := auth.SetupAuthboss(cfg, database)
	if err != nil {
		log.Fatalf("Failed to setup authboss: %v", err)
	}

	// Create router
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Static files
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Backward-compatible logo URL alias for fallback /logos/<filename> paths.
	logoAliasServer := http.FileServer(http.Dir(filepath.Join("static", "logos", "logos")))
	r.Handle("/logos/*", http.StripPrefix("/logos/", logoAliasServer))

	// Setup handlers
	h := handlers.New(cfg, database, authBoss, sessionStore)
	h.RegisterRoutes(r)

	// Initialize model stores
	rules := models.NewRuleStore(database.Pool)
	tags := models.NewTagStore(database.Pool)
	transactions := models.NewTransactionStore(database.Pool)
	entities := models.NewEntityStore(database.Pool)

	// Start processing worker for LLM categorization
	// Requires explicit LLM_DEFAULT_MODEL configuration
	if cfg.LLMDefaultModel != "" {
		// Desktop app doesn't use orchestrator - pass nil for orchestrator-related params
		a.processingWorker = processing.NewWorker(cfg, database.Pool, transactions, entities, tags, rules, nil, nil, nil)
		a.processingWorker.Start()
		fmt.Println("🔄 Processing worker started")
	} else {
		fmt.Println("⚠️  Processing worker NOT started - LLM not configured")
		fmt.Println("   Set LLM_DEFAULT_MODEL (e.g., 'google/gemini-2.5-flash') and corresponding API key")
	}

	// Start insights worker (generates AI-powered reports and insights)
	// Requires at least one LLM provider configured (Grok, Vertex, or Groq)
	a.insightsWorker = insights.NewWorker(cfg, database.Pool)
	a.insightsWorker.Start()
	if a.insightsWorker.IsConfigured() {
		fmt.Println("🔄 Insights worker started")
	} else {
		fmt.Println("⚠️  Insights worker started but no LLM providers configured")
	}

	// Start sync worker (syncs all provider accounts at 00, 15, 30, 45 minutes past each hour)
	anyProviderConfigured := (cfg.TellerCert != "" && cfg.TellerKey != "") ||
		(cfg.AkahuAppID != "" && cfg.AkahuAppSecret != "") ||
		(cfg.PlaidClientID != "" && cfg.PlaidSecret() != "")
	if anyProviderConfigured {
		worker, err := sync.NewSyncWorker(database.Pool, cfg)
		if err != nil {
			fmt.Printf("⚠️  Sync worker NOT started: %v\n", err)
		} else {
			a.syncWorker = worker
			a.syncWorker.Start()
			fmt.Println("🔄 Sync worker started")
		}
	} else {
		fmt.Println("⚠️  Sync worker NOT started - no providers configured (Teller, Akahu, or Plaid)")
	}

	// Create HTTP server
	a.httpServer = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in background
	go func() {
		fmt.Printf("🌐 HTTP server starting on %s\n", a.serverURL)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	fmt.Println("✅ Probably Desktop ready!")
}

// shutdown is called when the Wails app is closing
func (a *App) shutdown(ctx context.Context) {
	fmt.Println("\n🛑 Shutting down...")

	// Stop sync worker
	if a.syncWorker != nil {
		fmt.Println("Stopping sync worker...")
		a.syncWorker.Stop()
	}

	// Stop insights worker
	if a.insightsWorker != nil {
		fmt.Println("Stopping insights worker...")
		a.insightsWorker.Stop()
	}

	// Stop processing worker
	if a.processingWorker != nil {
		fmt.Println("Stopping processing worker...")
		a.processingWorker.Stop()
	}

	// Shutdown HTTP server
	if a.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}

	// Close database
	if a.database != nil {
		a.database.Close()
	}
}

// GetServerURL returns the URL of the embedded HTTP server
func (a *App) GetServerURL() string {
	return a.serverURL
}

func main() {
	app := NewApp()

	// Run Wails application
	err := wails.Run(&options.App{
		Title:     "Probably",
		Width:     1280,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 9, G: 9, B: 11, A: 1}, // background color (dark theme equivalent)
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
			},
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			About: &mac.AboutInfo{
				Title:   "Probably",
				Message: "Personal Finance Tracking\nVersion 1.0.0",
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})

	if err != nil {
		log.Fatalf("Wails error: %v", err)
	}
}

