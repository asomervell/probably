package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/handlers"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestEnv holds the test environment
type TestEnv struct {
	T       *testing.T
	DB      *db.DB
	Pool    *pgxpool.Pool
	Server  *httptest.Server
	Router  chi.Router
	Cleanup func()

	// Stores for direct database access in tests
	Users        *models.UserStore
	Ledgers      *models.LedgerStore
	Accounts     *models.AccountStore
	Transactions *models.TransactionStore
	Tags         *models.TagStore
	Rules        *models.RuleStore
	APIKeys      *models.APIKeyStore
	Entities     *models.EntityStore
	Permissions  *models.PermissionStore
}

// TestUser represents a test user with API key
type TestUser struct {
	User            *models.User
	Ledger          *models.Ledger
	APIKey          *models.APIKey
	APIKeyPlaintext string
}

func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL") // also needed for cfg.DatabaseURL below
	database := testutil.ConnectTestDB(t)

	// Create config
	cfg := &config.Config{
		DatabaseURL:   dbURL,
		SessionSecret: "test-secret-key-for-testing-only",
		BaseURL:       "http://localhost:8080",
		Port:          "8080",
		Environment:   "test",
	}

	// Create router
	r := chi.NewRouter()

	// Create handlers (without authboss for API-only testing)
	h := handlers.NewForTesting(cfg, database)
	h.RegisterAPIRoutes(r)

	// Create test server
	server := httptest.NewServer(r)

	env := &TestEnv{
		T:            t,
		DB:           database,
		Pool:         database.Pool,
		Server:       server,
		Router:       r,
		Users:        models.NewUserStore(database.Pool),
		Ledgers:      models.NewLedgerStore(database.Pool),
		Accounts:     models.NewAccountStore(database.Pool),
		Transactions: models.NewTransactionStore(database.Pool),
		Tags:         models.NewTagStore(database.Pool),
		Rules:        models.NewRuleStore(database.Pool),
		APIKeys:      models.NewAPIKeyStore(database.Pool),
		Entities:     models.NewEntityStore(database.Pool),
		Permissions:  models.NewPermissionStore(database.Pool),
		Cleanup: func() {
			server.Close()
			database.Close()
		},
	}

	return env
}

// CreateTestUser creates a test user with a ledger and API key
func (env *TestEnv) CreateTestUser(suffix string) *TestUser {
	env.T.Helper()
	ctx := context.Background()

	// Create user
	user := &models.User{
		ID:        uuid.New(),
		Email:     fmt.Sprintf("test-%s-%d@example.com", suffix, time.Now().UnixNano()),
		Password:  "hashed-password-not-used-for-api",
		Confirmed: true,
	}
	if err := env.Users.Create(ctx, user); err != nil {
		env.T.Fatalf("Failed to create test user: %v", err)
	}

	// Create ledger
	ledger := &models.Ledger{
		ID:       uuid.New(),
		UserID:   user.ID,
		Name:     "Test Ledger",
		Currency: "USD",
	}
	if err := env.Ledgers.Create(ctx, ledger); err != nil {
		env.T.Fatalf("Failed to create test ledger: %v", err)
	}

	// Wire up entity-permission chain so getCurrentLedger can find this ledger.
	personEntity := &models.Entity{
		Type:           models.EntityTypePerson,
		Subtype:        models.PersonSubtypeIndividual,
		Name:           user.Email,
		ExternalSource: "system",
		UserVerified:   true,
	}
	if err := env.Entities.Create(ctx, personEntity); err != nil {
		env.T.Fatalf("Failed to create person entity: %v", err)
	}
	entityPerm := &models.UserEntityPermission{
		UserID:          user.ID,
		EntityID:        personEntity.ID,
		PermissionLevel: models.PermissionLevelOwner,
	}
	if err := env.Permissions.CreateUserEntityPermission(ctx, entityPerm); err != nil {
		env.T.Fatalf("Failed to create entity permission: %v", err)
	}
	entityLedger := &models.EntityLedger{
		EntityID: personEntity.ID,
		LedgerID: ledger.ID,
		Role:     "owner",
	}
	if err := env.Permissions.CreateEntityLedger(ctx, entityLedger); err != nil {
		env.T.Fatalf("Failed to create entity-ledger link: %v", err)
	}

	// Create API key
	plaintext, apiKey, err := models.GenerateAPIKey(user.ID, "Test API Key")
	if err != nil {
		env.T.Fatalf("Failed to generate API key: %v", err)
	}
	if err := env.APIKeys.Create(ctx, apiKey); err != nil {
		env.T.Fatalf("Failed to create API key: %v", err)
	}

	return &TestUser{
		User:            user,
		Ledger:          ledger,
		APIKey:          apiKey,
		APIKeyPlaintext: plaintext,
	}
}

// CleanupTestUser removes all data for a test user
func (env *TestEnv) CleanupTestUser(tu *TestUser) {
	env.T.Helper()
	ctx := context.Background()

	// Delete user (cascades to ledger, accounts, transactions, etc.)
	_, _ = env.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", tu.User.ID)
}

// APIClient provides a simple HTTP client for API testing
type APIClient struct {
	BaseURL string
	APIKey  string
	T       *testing.T
}

func (env *TestEnv) NewAPIClient(tu *TestUser) *APIClient {
	return &APIClient{
		BaseURL: env.Server.URL,
		APIKey:  tu.APIKeyPlaintext,
		T:       env.T,
	}
}

// Request makes an API request and returns the response
func (c *APIClient) Request(method, path string, body any) *http.Response {
	c.T.Helper()

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			c.T.Fatalf("Failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		c.T.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.T.Fatalf("Request failed: %v", err)
	}

	return resp
}

// Get makes a GET request
func (c *APIClient) Get(path string) *http.Response {
	return c.Request(http.MethodGet, path, nil)
}

// Post makes a POST request
func (c *APIClient) Post(path string, body any) *http.Response {
	return c.Request(http.MethodPost, path, body)
}

// Put makes a PUT request
func (c *APIClient) Put(path string, body any) *http.Response {
	return c.Request(http.MethodPut, path, body)
}

// Delete makes a DELETE request
func (c *APIClient) Delete(path string) *http.Response {
	return c.Request(http.MethodDelete, path, nil)
}

// ParseJSON parses the response body as JSON
func ParseJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("Failed to parse JSON response: %v\nBody: %s", err, string(body))
	}
}

// AssertStatus checks that the response has the expected status code
func AssertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status %d, got %d. Body: %s", expected, resp.StatusCode, string(body))
	}
}

// Helper to create a test account
func (env *TestEnv) CreateAccount(ledgerID uuid.UUID, name string, accType models.AccountType) *models.Account {
	env.T.Helper()
	acc := &models.Account{
		ID:       uuid.New(),
		LedgerID: ledgerID,
		Name:     name,
		Type:     accType,
		IsActive: true,
	}
	if err := env.Accounts.Create(context.Background(), acc); err != nil {
		env.T.Fatalf("Failed to create account: %v", err)
	}
	return acc
}

// Helper to create a test transaction
func (env *TestEnv) CreateTransaction(ledgerID uuid.UUID, date time.Time, description string, entries []*models.Entry) *models.Transaction {
	env.T.Helper()
	txn := &models.Transaction{
		ID:          uuid.New(),
		LedgerID:    ledgerID,
		Date:        date,
		Description: description,
	}
	if err := env.Transactions.CreateWithEntries(context.Background(), txn, entries); err != nil {
		env.T.Fatalf("Failed to create transaction: %v", err)
	}
	return txn
}

// Helper to create a test tag
func (env *TestEnv) CreateTag(ledgerID uuid.UUID, name, color string) *models.Tag {
	env.T.Helper()
	tag := &models.Tag{
		ID:       uuid.New(),
		LedgerID: ledgerID,
		Name:     name,
		Color:    color,
	}
	if err := env.Tags.Create(context.Background(), tag); err != nil {
		env.T.Fatalf("Failed to create tag: %v", err)
	}
	return tag
}

// timeNow returns current time (helper for tests)
func timeNow() time.Time {
	return time.Now()
}
