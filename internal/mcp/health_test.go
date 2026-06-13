package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newHealthServer builds a *Server with just the collaborators handleHealth
// touches: a tool registry and an injected *db.DB. No auth wiring, so a route
// mounted via RegisterRoutes is reachable without an Authorization header.
func newHealthServer(t *testing.T, database *db.DB) *Server {
	t.Helper()
	tools, err := NewToolRegistry()
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}
	return &Server{
		db:    database,
		tools: tools,
		auth:  &AuthHandler{}, // RegisterRoutes references s.auth for metadata routes
	}
}

// deadDB returns a *db.DB whose lazily-created pool points at an unreachable
// address, so Pool.Ping fails fast — exercising the 503 path without a real DB.
func deadDB(t *testing.T) *db.DB {
	t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://u:p@127.0.0.1:1/db")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewWithConfig (should be lazy): %v", err)
	}
	t.Cleanup(pool.Close)
	return &db.DB{Pool: pool}
}

func TestHandleHealth_UnhealthyDBReturns503(t *testing.T) {
	s := newHealthServer(t, deadDB(t))

	r := chi.NewRouter()
	s.RegisterRoutes(r, nil)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "unavailable" {
		t.Fatalf("status field = %v, want 'unavailable'", body["status"])
	}
	if _, ok := body["db"].(string); !ok || body["db"] == "ok" {
		t.Fatalf("expected db field to carry an error message, got %v", body["db"])
	}
}

func TestHandleHealth_NoAuthRequired(t *testing.T) {
	// The /health route must be registered outside any auth middleware group.
	// We confirm it does NOT return 401, regardless of DB health.
	s := newHealthServer(t, deadDB(t))

	r := chi.NewRouter()
	s.RegisterRoutes(r, nil)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// No Authorization header.
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("/health returned 401; it must be unauthenticated")
	}
}

func TestHandleHealth_HealthyReturns200(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB-backed health test in short mode")
	}

	// Requires a reachable Postgres (TEST_DATABASE_URL); skips otherwise.
	database := testutil.ConnectTestDB(t)
	t.Cleanup(database.Close)

	s := newHealthServer(t, database)

	r := chi.NewRouter()
	s.RegisterRoutes(r, nil)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %v, want 'ok'", body["status"])
	}
	if body["db"] != "ok" {
		t.Fatalf("db field = %v, want 'ok'", body["db"])
	}
	toolsN, ok := body["tools"].(float64)
	if !ok || toolsN <= 0 {
		t.Fatalf("tools field = %v, want a positive number", body["tools"])
	}
}
