package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// Server
	Port        string
	Environment string
	BaseURL     string

	// Database
	DatabaseURL string

	// Session
	SessionSecret string

	// Teller
	TellerAppID         string
	TellerEnvironment   string
	TellerCert          string
	TellerKey           string
	TellerWebhookSecret string

	// Plaid
	PlaidClientID          string
	PlaidProductionSecret  string
	PlaidSandboxSecret     string
	PlaidDevelopmentSecret string // development.plaid.com (see Plaid dashboard "development" secret)
	PlaidEnvironment       string // 'sandbox', 'development', 'production'
	PlaidWebhookSecret     string
	PlaidWebhookURL        string // Optional: explicit webhook URL (if not set, webhook is disabled)
	PlaidRedirectURI       string // Optional: OAuth redirect URI (if not set, redirect URI is not used)
	PlaidCountryCodes      string // Comma-separated country codes (default: US,CA,GB,IE,FR,ES,NL,DE)

	// Akahu
	AkahuAppID         string
	AkahuAppSecret     string
	AkahuUserToken     string // Personal app user token (for single-user apps without OAuth)
	AkahuWebhookSecret string
	AkahuBaseURL       string // API base URL (default: https://api.akahu.io/v1)

	// Categorization worker settings
	CategorizationBatchSize  int // Max transactions per API call (default: 20)
	CategorizationRateLimit  int // Max API calls per minute (default: 30)
	CategorizationMaxRetries int // Max retries for failed transactions (default: 3)

	// Logo.dev - Company logo API
	LogoDevPublishableKey string // Publishable key for logo.dev image URLs
	LogoDevSecretKey      string // Secret key for logo.dev API calls (optional)

	// Firecrawl - Web scraping API for entity lookup fallback
	FirecrawlAPIKey string // API key for Firecrawl

	// LLM Model Configuration
	// Format: "provider/model" (e.g., "xai/grok-4-1", "google/gemini-2.0-flash", "groq/llama-3.3-70b")
	// Supported providers: xai, google, groq, anthropic
	LLMDefaultModel   string // Default model for most tasks. Defaults to "anthropic/<ClaudeModel>" when unset; an explicit LLM_DEFAULT_MODEL (any provider) always wins.
	LLMReasoningModel string // Complex reasoning tasks (e.g., "xai/grok-4-1")
	LLMToolCallModel  string // Tool/function calling (e.g., "google/gemini-2.0-flash")

	// Orchestrator Configuration
	LLMDefaultStrategy     string  // Default execution strategy: simple, escalate, consensus, supervisor, verify (default: simple)
	LLMChatStrategy        string  // Strategy for chat tasks (default: supervisor)
	LLMEscalateThreshold   float64 // Confidence threshold for escalation (0.0-1.0, default: 0.85)
	LLMConsensusRequired   int     // Number of models required for consensus (default: 2)
	LLMMaxVerifyIterations int     // Max iterations for verify strategy (default: 3)

	// API Keys for LLM providers
	XAIAPIKey       string // xAI API key (XAI_API_KEY)
	GoogleAPIKey    string // Google AI Studio API key (GOOGLE_API_KEY)
	GroqAPIKey      string // Groq API key (GROQ_API_KEY)
	AnthropicAPIKey string // Anthropic API key (ANTHROPIC_API_KEY)

	// Insights-specific LLM config (separate from categorization)
	// Uses provider chain: Anthropic -> xAI -> Google -> Groq
	GrokModel      string // Model for xAI insights (default: grok-3)
	VertexProject  string // GCP project ID (optional)
	VertexLocation string // GCP location (default: us-central1)
	VertexModel    string // Model for Google insights (default: gemini-1.5-pro)
	GroqModel      string // Model for Groq insights (default: llama-3.3-70b-versatile)
	ClaudeModel    string // Model for Anthropic insights (default: claude-sonnet-4-6)

	// Parallelization settings for processing worker
	ProcessingWorkers int // Concurrent transaction processing workers (default: 20)
	LLMConcurrency    int // Max concurrent LLM API calls (default: 3)
	LogoConcurrency   int // Max concurrent logo fetch operations (default: 10)

	// Cloud Storage for logos
	StorageType            string // "local", "gcs", or "s3"
	StorageBucket          string // Bucket name for GCS or S3
	StorageRegion          string // Region for S3
	StorageEndpoint        string // Custom endpoint for S3-compatible storage
	StorageAccessKeyID     string // Access key for S3
	StorageSecretAccessKey string // Secret key for S3
	GCSCredentialsJSON     string // GCS credentials JSON (optional, can use default credentials)
	CDNDomain              string // CDN domain for serving logos (e.g., "https://cdn.probably.money")

	// Statement processing
	StatementMaxFileSizeMB int    // Max file size in MB (default: 100)
	StatementAllowedTypes  string // Comma-separated list of allowed MIME types

	// Stripe billing
	BillingEnabled      bool   // Enable/disable billing features (for offline dev)
	StripeSecretKey     string // Stripe secret key
	StripeWebhookSecret string // Stripe webhook signing secret
	StripePriceMonthly  string // Monthly price ID
	StripePriceAnnual   string // Annual price ID
	StripePriceBundle   string // Bundle price ID (optional)

	// PostHog (product analytics, exceptions, feature flags, OTEL logs + AI traces)
	PostHogProjectAPIKey   string // phc_… project token (browser + server + OTLP)
	PostHogPersonalAPIKey  string // phx_… optional, faster server-side feature flags
	PostHogAPIHost         string // e.g. https://us.i.posthog.com (ingest + JS api_host)
	PostHogOTELHost        string // OTLP host without scheme, e.g. us.i.posthog.com
	PostHogEnvironment     string // overrides ENVIRONMENT for events if set
	PostHogSessionReplay   bool   // enable session replay in browser SDK
	PostHogAutocapture     bool   // element autocapture
	PostHogCapturePageview bool   // initial $pageview (HTMX sends additional)
	PostHogDisableWeb      bool   // skip injecting posthog-js (e.g. tests)

	// MCP Server (ChatGPT App integration)
	MCPBaseURL  string // Public URL for MCP server
	MCPUICDNURL string // CDN URL for UI resources

	// Voice Chat Configuration
	VoiceEnabled bool // Enable voice chat features

}

// RequireDatabaseURL returns an error if DATABASE_URL was not set. The app
// does not start or download an in-process database; PostgreSQL is always external.
func (c *Config) RequireDatabaseURL() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required: set it to a PostgreSQL connection string (e.g. docker compose, local install, or hosted)")
	}
	return nil
}

func Load() *Config {
	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "production"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),

		// No default: PostgreSQL must be provided (Docker, system install, or hosted).
		DatabaseURL: strings.TrimSpace(getEnv("DATABASE_URL", "")),

		SessionSecret: getEnv("SESSION_SECRET", "change-me-in-production-32-chars"),

		TellerAppID:         getEnv("TELLER_APP_ID", ""),
		TellerEnvironment:   getEnv("TELLER_ENVIRONMENT", "sandbox"),
		TellerCert:          getEnv("TELLER_CERT", ""),
		TellerKey:           getEnv("TELLER_KEY", ""),
		TellerWebhookSecret: getEnv("TELLER_WEBHOOK_SECRET", ""),

		// Plaid (trim IDs/secrets — trailing spaces in .env break Plaid with INVALID_API_KEYS)
		PlaidClientID:          strings.TrimSpace(getEnv("PLAID_CLIENT_ID", "")),
		PlaidProductionSecret:  strings.TrimSpace(getEnv("PLAID_PRODUCTION_SECRET", "")),
		PlaidSandboxSecret:     strings.TrimSpace(getEnv("PLAID_SANDBOX_SECRET", "")),
		PlaidDevelopmentSecret: strings.TrimSpace(getEnv("PLAID_DEVELOPMENT_SECRET", "")),
		PlaidEnvironment:       strings.TrimSpace(getEnv("PLAID_ENVIRONMENT", "sandbox")),
		PlaidWebhookSecret:     strings.TrimSpace(getEnv("PLAID_WEBHOOK_SECRET", "")),
		PlaidWebhookURL:        strings.TrimSpace(getEnv("PLAID_WEBHOOK_URL", "")),
		PlaidRedirectURI:       strings.TrimSpace(getEnv("PLAID_REDIRECT_URI", "")),
		PlaidCountryCodes:      getEnv("PLAID_COUNTRY_CODES", "US"),

		// Akahu
		AkahuAppID:         getEnv("AKAHU_APP_ID", ""),
		AkahuAppSecret:     getEnv("AKAHU_APP_SECRET", ""),
		AkahuUserToken:     getEnv("AKAHU_USER_TOKEN", ""),
		AkahuWebhookSecret: getEnv("AKAHU_WEBHOOK_SECRET", ""),
		AkahuBaseURL:       getEnv("AKAHU_URL", "https://api.akahu.io/v1"),

		CategorizationBatchSize:  getEnvInt("CATEGORIZATION_BATCH_SIZE", 20),
		CategorizationRateLimit:  getEnvInt("CATEGORIZATION_RATE_LIMIT", 30),
		CategorizationMaxRetries: getEnvInt("CATEGORIZATION_MAX_RETRIES", 3),

		LogoDevPublishableKey: getEnv("LOGO_DEV_PUBLISHABLE_KEY", ""),
		LogoDevSecretKey:      getEnv("LOGO_DEV_SECRET_KEY", ""),

		FirecrawlAPIKey: getEnv("FIRECRAWL_API_KEY", ""),

		// LLM Model Configuration (provider/model format)
		LLMDefaultModel:   strings.TrimSpace(getEnv("LLM_DEFAULT_MODEL", "")),   // e.g., "google/gemini-3-flash-preview"
		LLMReasoningModel: strings.TrimSpace(getEnv("LLM_REASONING_MODEL", "")), // e.g., "xai/grok-4-1"
		LLMToolCallModel:  strings.TrimSpace(getEnv("LLM_TOOL_CALL_MODEL", "")), // e.g., "google/gemini-3-flash-preview"

		// Orchestrator configuration
		LLMDefaultStrategy:     getEnv("LLM_DEFAULT_STRATEGY", "simple"),    // simple, escalate, consensus, supervisor, verify
		LLMChatStrategy:        getEnv("LLM_CHAT_STRATEGY", "supervisor"),   // Strategy for chat tasks
		LLMEscalateThreshold:   getEnvFloat("LLM_ESCALATE_THRESHOLD", 0.85), // Confidence threshold for escalation
		LLMConsensusRequired:   getEnvInt("LLM_CONSENSUS_REQUIRED", 2),      // Number of models for consensus
		LLMMaxVerifyIterations: getEnvInt("LLM_MAX_VERIFY_ITERATIONS", 3),   // Max iterations for verify

		// API Keys
		XAIAPIKey:       strings.TrimSpace(getEnv("XAI_API_KEY", "")),
		GoogleAPIKey:    strings.TrimSpace(getEnv("GOOGLE_API_KEY", "")),
		GroqAPIKey:      strings.TrimSpace(getEnv("GROQ_API_KEY", "")),
		AnthropicAPIKey: strings.TrimSpace(getEnv("ANTHROPIC_API_KEY", "")),

		// Insights-specific LLM config
		GrokModel:      getEnv("GROK_MODEL", "grok-3"),
		VertexProject:  strings.TrimSpace(getEnv("VERTEX_PROJECT", "")),
		VertexLocation: strings.TrimSpace(getEnv("VERTEX_LOCATION", "us-central1")),
		VertexModel:    strings.TrimSpace(getEnv("VERTEX_MODEL", "gemini-1.5-pro")),
		GroqModel:      getEnv("GROQ_MODEL", "llama-3.3-70b-versatile"),
		ClaudeModel:    getEnv("CLAUDE_MODEL", "claude-sonnet-4-6"),

		// Parallelization
		ProcessingWorkers: getEnvInt("PROCESSING_WORKERS", 20),
		LLMConcurrency:    getEnvInt("LLM_CONCURRENCY", 3),
		LogoConcurrency:   getEnvInt("LOGO_CONCURRENCY", 10),

		// Cloud Storage
		StorageType:            getEnv("STORAGE_TYPE", "local"),
		StorageBucket:          getEnv("STORAGE_BUCKET", ""),
		StorageRegion:          getEnv("STORAGE_REGION", "us-central1"),
		StorageEndpoint:        getEnv("STORAGE_ENDPOINT", ""),
		StorageAccessKeyID:     getEnv("STORAGE_ACCESS_KEY_ID", ""),
		StorageSecretAccessKey: getEnv("STORAGE_SECRET_ACCESS_KEY", ""),
		GCSCredentialsJSON:     getEnv("GCS_CREDENTIALS_JSON", ""),
		CDNDomain:              getEnv("CDN_DOMAIN", ""),

		// Statement processing
		StatementMaxFileSizeMB: getEnvInt("STATEMENT_MAX_FILE_SIZE_MB", 100),
		StatementAllowedTypes:  getEnv("STATEMENT_ALLOWED_TYPES", "application/pdf,image/png,image/jpeg,image/jpg"),

		// Stripe billing
		BillingEnabled:      getEnvBool("BILLING_ENABLED", true),
		StripeSecretKey:     getEnv("STRIPE_SECRET", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripePriceMonthly:  getEnv("STRIPE_PRICE_MONTHLY", ""),
		StripePriceAnnual:   getEnv("STRIPE_PRICE_ANNUAL", ""),
		StripePriceBundle:   getEnv("STRIPE_PRICE_BUNDLE", ""),

		// PostHog
		PostHogProjectAPIKey:   getEnv("POSTHOG_PROJECT_API_KEY", ""),
		PostHogPersonalAPIKey:  getEnv("POSTHOG_PERSONAL_API_KEY", ""),
		PostHogAPIHost:         getEnv("POSTHOG_API_HOST", "https://us.i.posthog.com"),
		PostHogOTELHost:        getEnv("POSTHOG_OTEL_HOST", "us.i.posthog.com"),
		PostHogEnvironment:     getEnv("POSTHOG_ENVIRONMENT", ""),
		PostHogSessionReplay:   getEnvBool("POSTHOG_SESSION_REPLAY", false),
		PostHogAutocapture:     getEnvBool("POSTHOG_AUTOCAPTURE", true),
		PostHogCapturePageview: getEnvBool("POSTHOG_CAPTURE_PAGEVIEW", true),
		PostHogDisableWeb:      getEnvBool("POSTHOG_DISABLE_WEB", false),

		// MCP Server
		MCPBaseURL:  getEnv("MCP_BASE_URL", ""),
		MCPUICDNURL: getEnv("MCP_UI_CDN_URL", ""),

		// Voice Chat
		VoiceEnabled: getEnvBool("VOICE_ENABLED", false),
	}

	// Anthropic Claude is the default provider for all text LLM tasks. An
	// explicit LLM_DEFAULT_MODEL (any provider) always wins, so operators can
	// opt out per-environment without code changes.
	if cfg.LLMDefaultModel == "" {
		cfg.LLMDefaultModel = "anthropic/" + cfg.ClaudeModel
	}
	if provider, _, _ := strings.Cut(cfg.LLMDefaultModel, "/"); provider == "anthropic" && cfg.AnthropicAPIKey == "" {
		slog.Warn("LLM default routes to Anthropic but ANTHROPIC_API_KEY is not set; LLM features will be unavailable until it is configured")
	}

	return cfg
}

// PlaidSecret returns the appropriate Plaid secret for PLAID_ENVIRONMENT.
// Values are trimmed — stray whitespace in .env breaks Plaid with opaque API errors.
func (c *Config) PlaidSecret() string {
	switch strings.ToLower(strings.TrimSpace(c.PlaidEnvironment)) {
	case "production":
		return strings.TrimSpace(c.PlaidProductionSecret)
	case "development":
		if s := strings.TrimSpace(c.PlaidDevelopmentSecret); s != "" {
			return s
		}
		return strings.TrimSpace(c.PlaidSandboxSecret)
	default:
		return strings.TrimSpace(c.PlaidSandboxSecret)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true" || value == "1" || value == "yes"
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return result
}

func getEnvFloat(key string, defaultValue float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	var result float64
	_, err := fmt.Sscanf(value, "%f", &result)
	if err != nil {
		return defaultValue
	}
	return result
}
