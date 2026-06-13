package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// healthPath is the liveness endpoint exposed by the hardening sibling task
// (PRO-20 / pro-90). The concurrent-load test polls it to confirm the server
// stays green under demo load. Encoded as a single constant so it is a one-line
// update if the sibling lands the endpoint at a different path.
const healthPath = "/health"

// demoEnv is the shared, DB-backed fixture every demo-flow assertion reuses.
// It owns a user + ledger + entity/permission chain, a trialing subscription
// (so the MCP subscription gate passes), an OAuth bearer, and a *mcp.Server
// wired to a stub upstream API that returns deterministic transaction data.
type demoEnv struct {
	db          *db.DB
	cfg         *config.Config
	mcpServer   *Server
	accessToken string
	userID      uuid.UUID
	ledgerID    uuid.UUID
	upstream    *httptest.Server
}

// setupDemoEnv provisions a complete demo environment against the test Postgres
// database and registers cleanup so re-runs are deterministic. It skips (via
// testutil.ConnectTestDB) when TEST_DATABASE_URL is unset.
func setupDemoEnv(t *testing.T) *demoEnv {
	t.Helper()

	database := testutil.ConnectTestDB(t)
	t.Cleanup(database.Close)

	ctx := context.Background()

	users := models.NewUserStore(database.Pool)
	ledgers := models.NewLedgerStore(database.Pool)
	entities := models.NewEntityStore(database.Pool)
	permissions := models.NewPermissionStore(database.Pool)
	subscriptions := models.NewSubscriptionStore(database.Pool)

	tu := testutil.CreateUserAndLedger(t, users, ledgers, "mcp-demo")

	// Mirror the entity / UserEntityPermission / EntityLedger wiring from
	// internal/backup/backup_test.go so getCurrentLedger-style lookups resolve.
	personEntity := &models.Entity{
		ID:           uuid.New(),
		Type:         models.EntityTypePerson,
		Subtype:      "individual",
		Name:         tu.User.Email,
		UserVerified: true,
	}
	if err := entities.Create(ctx, personEntity); err != nil {
		t.Fatalf("failed to create person entity: %v", err)
	}
	perm := &models.UserEntityPermission{
		UserID:          tu.User.ID,
		EntityID:        personEntity.ID,
		PermissionLevel: models.PermissionLevelOwner,
		GrantedBy:       &tu.User.ID,
	}
	if err := permissions.CreateUserEntityPermission(ctx, perm); err != nil {
		t.Fatalf("failed to create user-entity permission: %v", err)
	}
	entityLedger := &models.EntityLedger{
		EntityID: personEntity.ID,
		LedgerID: tu.Ledger.ID,
		Role:     "owner",
	}
	if err := permissions.CreateEntityLedger(ctx, entityLedger); err != nil {
		t.Fatalf("failed to link ledger to entity: %v", err)
	}

	// A trialing subscription so processToolCall's subscription gate passes.
	sub := &models.Subscription{
		ID:       uuid.New(),
		UserID:   tu.User.ID,
		Status:   models.SubscriptionStatusTrialing,
		PlanType: models.PlanTypeMonthly,
	}
	if err := subscriptions.Create(ctx, sub); err != nil {
		t.Fatalf("failed to create trialing subscription: %v", err)
	}

	// Stub upstream API. search_transactions executes via APIClient.Get against
	// cfg.BaseURL + "/api/v1/transactions"; this stub returns one deterministic
	// transaction whose description references "groceries" so the demo flow is
	// hermetic and fast (no dependency on the full app router).
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":          uuid.New().String(),
					"description": "Weekly groceries",
					"entries": []map[string]interface{}{
						{"amount_cents": -4523.0},
					},
				},
			},
		})
	}))
	t.Cleanup(upstream.Close)

	cfg := &config.Config{BaseURL: upstream.URL}

	mcpServer, err := NewServer(cfg, database, nil)
	if err != nil {
		t.Fatalf("failed to create MCP server: %v", err)
	}

	// Mint the bearer the ContextHandler middleware validates. No OAuth dance:
	// the pre-minted token is exactly what Bearer auth checks.
	token, err := mcpServer.tokenStore.CreateToken(
		ctx,
		tu.User.ID,
		"demo-test-client",
		[]string{"read:transactions", "read:accounts", "read:financial"},
		3600,
	)
	if err != nil {
		t.Fatalf("failed to mint oauth token: %v", err)
	}
	if !strings.HasPrefix(token.AccessToken, "prob_oauth_") {
		t.Fatalf("unexpected token format: %q", token.AccessToken)
	}

	t.Cleanup(func() {
		_ = mcpServer.tokenStore.DeleteToken(context.Background(), token.AccessToken)
		_, _ = database.Pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", tu.User.ID)
	})

	return &demoEnv{
		db:          database,
		cfg:         cfg,
		mcpServer:   mcpServer,
		accessToken: token.AccessToken,
		userID:      tu.User.ID,
		ledgerID:    tu.Ledger.ID,
		upstream:    upstream,
	}
}

// jsonrpcClient is a thin JSON-RPC-over-HTTP client that POSTs to the protected
// MCP root (<baseURL>/mcp/) with the demo env's bearer token.
type jsonrpcClient struct {
	t          *testing.T
	httpClient *http.Client
	baseURL    string
	token      string
	nextID     int64
}

// newJSONRPCClient builds a client bound to a running httptest server URL.
func (e *demoEnv) newJSONRPCClient(t *testing.T, baseURL string) *jsonrpcClient {
	return &jsonrpcClient{
		t:          t,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
		token:      e.accessToken,
	}
}

// call issues a single JSON-RPC request and returns the parsed response.
// It fails the test on a non-2xx HTTP status; JSON-RPC-level errors are
// surfaced via the returned response's Error field for the caller to assert.
func (c *jsonrpcClient) call(method string, params any) (*jsonRPCResponse, error) {
	c.t.Helper()

	id := atomic.AddInt64(&c.nextID, 1)
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		reqBody["params"] = params
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/mcp/", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("non-2xx status for %s: %d", method, resp.StatusCode)
	}

	var rpcResp jsonRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &rpcResp, nil
}

// runDemoFlow drives the canonical Claude-to-Probably demo flow and fails the
// test via t.Fatalf on any error. Use this for serial callers.
//
// Concurrent callers (e.g. errgroup goroutines) MUST use runDemoFlowE instead:
// t.Fatalf calls runtime.Goexit on the calling goroutine, which with
// golang.org/x/sync does NOT propagate through errgroup.Wait, so a failing flow
// would silently pass. runDemoFlowE returns the error so Wait surfaces it.
func runDemoFlow(t *testing.T, c *jsonrpcClient) {
	t.Helper()
	if err := runDemoFlowE(c); err != nil {
		t.Fatalf("demo flow: %v", err)
	}
}

// runDemoFlowE drives the canonical Claude-to-Probably demo flow:
// initialize -> tools/list (asserts search_transactions present) ->
// tools/call(search_transactions, {query: "groceries", limit: 5}).
// It returns the first error encountered, making it safe to run concurrently.
func runDemoFlowE(c *jsonrpcClient) error {
	// 1. initialize
	initResp, err := c.call("initialize", map[string]interface{}{
		"protocolVersion": "2025-03-26",
		"clientInfo":      map[string]interface{}{"name": "demo-test", "version": "1.0.0"},
	})
	if err != nil {
		return fmt.Errorf("initialize call failed: %w", err)
	}
	if initResp.Error != nil {
		return fmt.Errorf("initialize returned JSON-RPC error: %+v", initResp.Error)
	}
	initResult, ok := initResp.Result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("initialize result has unexpected shape: %T", initResp.Result)
	}
	if got := initResult["protocolVersion"]; got != "2025-03-26" {
		return fmt.Errorf("initialize protocolVersion = %v, want 2025-03-26", got)
	}
	serverInfo, ok := initResult["serverInfo"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("initialize serverInfo missing or wrong type: %T", initResult["serverInfo"])
	}
	if got := serverInfo["name"]; got != "probably-mcp" {
		return fmt.Errorf("initialize serverInfo.name = %v, want probably-mcp", got)
	}

	// 2. tools/list — assert search_transactions is present by name.
	listResp, err := c.call("tools/list", nil)
	if err != nil {
		return fmt.Errorf("tools/list call failed: %w", err)
	}
	if listResp.Error != nil {
		return fmt.Errorf("tools/list returned JSON-RPC error: %+v", listResp.Error)
	}
	if !toolListContains(listResp.Result, "search_transactions") {
		return fmt.Errorf("tools/list did not contain search_transactions; got %+v", listResp.Result)
	}

	// 3. tools/call(search_transactions)
	callResp, err := c.call("tools/call", map[string]interface{}{
		"name": "search_transactions",
		"arguments": map[string]interface{}{
			"query": "groceries",
			"limit": 5,
		},
	})
	if err != nil {
		return fmt.Errorf("tools/call call failed: %w", err)
	}
	if callResp.Error != nil {
		return fmt.Errorf("tools/call returned JSON-RPC error: %+v", callResp.Error)
	}
	callResult, ok := callResp.Result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("tools/call result has unexpected shape: %T", callResp.Result)
	}
	content, ok := callResult["content"].([]interface{})
	if !ok || len(content) == 0 {
		return fmt.Errorf("tools/call result missing content block: %+v", callResult)
	}
	return nil
}

// toolListContains reports whether a tools/list result contains a tool with the
// given name in its "tools" array.
func toolListContains(result interface{}, name string) bool {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return false
	}
	tools, ok := resultMap["tools"].([]interface{})
	if !ok {
		return false
	}
	for _, raw := range tools {
		tool, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if tool["name"] == name {
			return true
		}
	}
	return false
}

// newStandaloneRouter mirrors cmd/mcp-server/main.go: a chi router that MCP owns
// entirely (the topology served on port 8081 in production).
func newStandaloneRouter(s *Server) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	s.RegisterRoutes(r, nil)
	return r
}

// newEmbeddedRouter mounts MCP alongside unrelated sibling routes, mirroring how
// MCP would coexist with the main app's router in cmd/server. The sibling routes
// exist to prove MCP does not collide with other handlers (the root POST inside
// RegisterRoutes is the most likely collision point).
func newEmbeddedRouter(s *Server) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Get("/api/v1/ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/some/app/route", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	s.RegisterRoutes(r, nil)
	return r
}

// TestDemoFlowHarness_Smoke proves the harness itself works end-to-end against a
// standalone router.
func TestDemoFlowHarness_Smoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB-backed integration test in short mode")
	}

	env := setupDemoEnv(t)
	srv := httptest.NewServer(newStandaloneRouter(env.mcpServer))
	t.Cleanup(srv.Close)

	runDemoFlow(t, env.newJSONRPCClient(t, srv.URL))
}

// TestDemoFlow_CrossMode runs the identical demo flow against both router
// topologies: standalone (MCP owns the router) and embedded (MCP alongside
// sibling routes). Both must pass the same runDemoFlow.
func TestDemoFlow_CrossMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB-backed integration test in short mode")
	}

	topologies := []struct {
		name  string
		build func(*Server) chi.Router
	}{
		{name: "standalone", build: newStandaloneRouter},
		{name: "embedded", build: newEmbeddedRouter},
	}

	for _, tc := range topologies {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			env := setupDemoEnv(t)
			srv := httptest.NewServer(tc.build(env.mcpServer))
			t.Cleanup(srv.Close)

			runDemoFlow(t, env.newJSONRPCClient(t, srv.URL))

			// In the embedded topology, confirm a sibling route still responds,
			// proving MCP routes coexist without colliding with app routes.
			if tc.name == "embedded" {
				resp, err := http.Get(srv.URL + "/api/v1/ping")
				if err != nil {
					t.Fatalf("sibling /api/v1/ping request failed: %v", err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("sibling /api/v1/ping status = %d, want 200", resp.StatusCode)
				}
			}
		})
	}
}

// TestDemoFlow_ConcurrentDemoLoad fires N parallel demo-flow invocations against
// one embedded-topology server while concurrently polling the /health endpoint
// (owned by the hardening sibling, PRO-20 / pro-90) to confirm it stays 200
// throughout the load window. This is the parent task's "expected demo load"
// gate: it measures the sibling hardening's effect, it does not implement it.
func TestDemoFlow_ConcurrentDemoLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB-backed load test in short mode")
	}

	const n = 20

	// One env builds the shared embedded router + server; the concurrent callers
	// each get their own env (and thus their own bearer) below.
	base := setupDemoEnv(t)
	srv := httptest.NewServer(newEmbeddedRouter(base.mcpServer))
	t.Cleanup(srv.Close)

	// Mint N independent envs so each goroutine drives its own OAuth token,
	// exercising the real concurrency surface rather than a single hot bearer.
	clients := make([]*jsonrpcClient, n)
	for i := 0; i < n; i++ {
		env := setupDemoEnv(t)
		clients[i] = env.newJSONRPCClient(t, srv.URL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Concurrent /health poller: counts non-200 responses and total polls across
	// the entire load window. Stops when the load goroutines finish.
	var healthPolls, healthFailures int64
	done := make(chan struct{})
	pollerDone := make(chan struct{})
	go func() {
		defer close(pollerDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		httpClient := &http.Client{Timeout: 5 * time.Second}
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				atomic.AddInt64(&healthPolls, 1)
				resp, err := httpClient.Get(srv.URL + healthPath)
				if err != nil {
					atomic.AddInt64(&healthFailures, 1)
					continue
				}
				if resp.StatusCode != http.StatusOK {
					atomic.AddInt64(&healthFailures, 1)
				}
				resp.Body.Close()
			}
		}
	}()

	// Fire N concurrent demo flows. runDemoFlowE returns errors (rather than
	// calling t.Fatalf in a goroutine) so a failure in any flow propagates
	// through errgroup.Wait and fails the test instead of silently passing.
	var g errgroup.Group
	for i := 0; i < n; i++ {
		c := clients[i]
		g.Go(func() error {
			return runDemoFlowE(c)
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("concurrent demo load failed: %v", err)
	}

	close(done)
	<-pollerDone

	if got := atomic.LoadInt64(&healthFailures); got != 0 {
		t.Fatalf("health endpoint returned %d non-200 responses during load window (path %q); "+
			"depends on PRO-20/pro-90 wiring %s", got, healthPath, healthPath)
	}
	// The poller must have run at least once during the load window. We don't
	// assert a higher floor: poll count scales with how long the load takes,
	// which is machine-dependent and would flake on fast hosts. The meaningful
	// invariant is healthFailures == 0 (asserted above).
	if got := atomic.LoadInt64(&healthPolls); got < 1 {
		t.Fatalf("health poller never issued a poll during the load window")
	}
}
