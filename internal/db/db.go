package db

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// DB wraps the pgx connection pool
type DB struct {
	Pool *pgxpool.Pool
}

// Connect creates a new database connection pool
func Connect(databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	db.Pool.Close()
}

// Migrate runs database migrations
func Migrate(db *DB) error {
	// Get underlying *sql.DB for goose
	conn, err := db.Pool.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	// goose needs a *sql.DB, so we need to use the stdlib adapter
	sqlDB, err := openStdlib(db.Pool)
	if err != nil {
		return fmt.Errorf("failed to open stdlib connection: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(log.New(os.Stdout, "  ", 0)) // Log migrations to stdout

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	// Allow out-of-order migrations (for migrations added retroactively)
	if err := goose.Up(sqlDB, "migrations", goose.WithAllowMissing()); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// MigrationStatus prints the current migration status
func MigrationStatus(db *DB) error {
	sqlDB, err := openStdlib(db.Pool)
	if err != nil {
		return fmt.Errorf("failed to open stdlib connection: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	if err := goose.Status(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}

	return nil
}
