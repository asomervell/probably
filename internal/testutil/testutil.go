package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// ConnectTestDB connects to the test database and runs migrations.
// Skips the test if TEST_DATABASE_URL is not set.
func ConnectTestDB(t *testing.T) *db.DB {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration tests")
	}

	database, err := db.Connect(dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	if err := db.Migrate(database); err != nil {
		database.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	return database
}

// TestUser holds a test user and their default ledger.
type TestUser struct {
	User   *models.User
	Ledger *models.Ledger
}

// CreateUserAndLedger creates a user and ledger in the database.
func CreateUserAndLedger(t *testing.T, users *models.UserStore, ledgers *models.LedgerStore, suffix string) *TestUser {
	t.Helper()
	ctx := context.Background()

	user := &models.User{
		ID:        uuid.New(),
		Email:     suffix + "-" + uuid.New().String()[:8] + "@test.com",
		Password:  "test-password",
		Confirmed: true,
	}
	if err := users.Create(ctx, user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	ledger := &models.Ledger{
		ID:       uuid.New(),
		UserID:   user.ID,
		Name:     "Test Ledger for " + suffix,
		Currency: "USD",
	}
	if err := ledgers.Create(ctx, ledger); err != nil {
		t.Fatalf("failed to create test ledger: %v", err)
	}

	return &TestUser{User: user, Ledger: ledger}
}
