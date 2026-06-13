package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/oauth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// defaultRequestTimeout bounds every JSON-RPC method call when
// MCP_REQUEST_TIMEOUT is unset. It is a local constant rather than a shared
// config field to avoid widening internal/config (owned elsewhere).
const defaultRequestTimeout = 30 * time.Second

// Server implements the MCP protocol for ChatGPT Apps
type Server struct {
	db                *db.DB
	ab                interface{} // *authboss.Authboss (passed as interface to avoid import in server.go)
	tools             *ToolRegistry
	resources         *ResourceRegistry
	apiClient         *APIClient
	auth              *AuthHandler
	context           *ContextHandler
	validation        *ValidationHandler
	audit             *AuditLogger
	tokenStore        *oauth.TokenStore
	subscriptionStore *models.SubscriptionStore

	// requestTimeout bounds every JSON-RPC method call dispatched via
	// handleMCPRoot. Defaults to defaultRequestTimeout; overridable via the
	// MCP_REQUEST_TIMEOUT environment variable (Go duration syntax).
	requestTimeout time.Duration

	// execTool runs a single tool attempt. It defaults to s.executeTool and is
	// a field purely to give tests a seam for injecting fake/transient-erroring
	// tools without touching any executeGet*/executeAsk*/executeSearch* body.
	execTool func(ctx context.Context, userCtx *UserContext, name string, arguments json.RawMessage) (map[string]interface{}, error)

	// checkAccess reports whether a user may invoke tools. It defaults to
	// subscriptionStore.HasActiveSubscriptionOrTrial and exists as a field so
	// dispatch/retry/timeout logic can be unit-tested without a live database.
	checkAccess func(ctx context.Context, userID uuid.UUID) (bool, error)
}

func textContentBlock(text string) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type": "text",
			"text": text,
		},
	}
}

// NewServer creates a new MCP server instance
func NewServer(cfg *config.Config, database *db.DB, ab interface{}) (*Server, error) {
	// Create tool registry
	tools, err := NewToolRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to create tool registry: %w", err)
	}

	// Create resource registry
	// Use MCP_BASE_URL if set, otherwise use BASE_URL
	baseURL := cfg.MCPBaseURL
	if baseURL == "" {
		baseURL = cfg.BaseURL
	}
	// Use CDN_DOMAIN for widget CSP (preferred), fallback to MCP_UI_CDN_URL
	cdnDomain := cfg.CDNDomain
	if cdnDomain == "" {
		cdnDomain = cfg.MCPUICDNURL
	}
	resources, err := NewResourceRegistry(baseURL, cdnDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource registry: %w", err)
	}

	// Create API client
	apiClient := NewAPIClient(cfg)

	apiKeyStore := models.NewAPIKeyStore(database.Pool)
	tokenStore := oauth.NewTokenStore(database.Pool, apiKeyStore)

	// Create auth handler (type assert ab to *authboss.Authboss)
	var authBoss *authboss.Authboss
	if ab != nil {
		if abVal, ok := ab.(*authboss.Authboss); ok {
			authBoss = abVal
		}
	}

	auth, err := NewAuthHandler(cfg, database, authBoss)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth handler: %w", err)
	}

	// Create context handler
	contextHandler := NewContextHandler(database, tokenStore)

	// Create validation handler
	validation := NewValidationHandler()

	// Create audit logger
	audit := NewAuditLogger()

	// Create subscription store for checking subscription status
	subscriptionStore := models.NewSubscriptionStore(database.Pool)

	s := &Server{
		db:                database,
		ab:                ab,
		tools:             tools,
		resources:         resources,
		apiClient:         apiClient,
		auth:              auth,
		context:           contextHandler,
		validation:        validation,
		audit:             audit,
		tokenStore:        tokenStore,
		subscriptionStore: subscriptionStore,
		requestTimeout:    requestTimeoutFromEnv(),
	}
	s.execTool = s.executeTool
	s.checkAccess = subscriptionStore.HasActiveSubscriptionOrTrial
	return s, nil
}

// requestTimeoutFromEnv reads the per-request timeout from MCP_REQUEST_TIMEOUT
// (Go duration syntax). It falls back to defaultRequestTimeout when the variable
// is unset, empty, or unparseable, logging a warning in the malformed case.
func requestTimeoutFromEnv() time.Duration {
	raw := os.Getenv("MCP_REQUEST_TIMEOUT")
	if raw == "" {
		return defaultRequestTimeout
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		slog.Warn("mcp: invalid MCP_REQUEST_TIMEOUT, using default", "value", raw, "default", defaultRequestTimeout)
		return defaultRequestTimeout
	}
	return d
}

// RegisterRoutes sets up MCP protocol routes
// abLoadClientStateMiddleware should be Authboss's LoadClientStateMiddleware
func (s *Server) RegisterRoutes(r chi.Router, abLoadClientStateMiddleware func(http.Handler) http.Handler) {
	// Unauthenticated health check (registered first, outside any auth group)
	// so it is reachable identically in standalone (cmd/mcp-server, :8081) and
	// embedded (cmd/server) modes for demo monitoring and the validation suite.
	r.Get("/health", s.handleHealth)

	// OAuth metadata endpoints (public, for discovery)
	// Register at both root and /mcp path for compatibility
	// RFC 8414: OAuth Authorization Server Metadata
	r.Get("/.well-known/oauth-authorization-server", s.auth.HandleAuthorizationServerMetadata)
	// RFC 7662: OAuth Protected Resource Metadata
	r.Get("/.well-known/oauth-protected-resource", s.auth.HandleMetadata)

	// Claude Code/Desktop client config (public, for one-paste discovery).
	// Mirrors the OAuth metadata registration so it is reachable without auth in
	// both standalone and embedded modes.
	r.Get("/.well-known/mcp-client-config", s.handleClientConfig)

	// ChatGPT Apps POST / directly; apply OAuth middleware then delegate to MCP root handler
	r.With(s.context.Middleware).Post("/", func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(r.Context(), "POST / → MCP root")
		s.handleMCPRoot(w, r)
	})

	// MCP protocol endpoints
	r.Route("/mcp", func(r chi.Router) {

		// Also register OAuth metadata under /mcp for ChatGPT Apps discovery
		r.Get("/.well-known/oauth-authorization-server", s.auth.HandleAuthorizationServerMetadata)
		r.Get("/.well-known/oauth-protected-resource", s.auth.HandleMetadata)
		// And the Claude client config, public and unauthenticated like above.
		r.Get("/.well-known/mcp-client-config", s.handleClientConfig)
		// OAuth endpoints
		// Authorization endpoint needs Authboss session middleware
		r.Group(func(r chi.Router) {
			// Load Authboss client state (session cookies) first
			if abLoadClientStateMiddleware != nil {
				r.Use(abLoadClientStateMiddleware)
			}
			// Then load user from session
			r.Use(s.loadAuthbossUserMiddleware())
			r.Get("/auth", s.auth.HandleAuthorization)
		})

		// Token exchange and client registration (public, no session needed)
		r.Post("/callback", s.auth.HandleCallback)           // Token exchange
		r.Post("/register", s.auth.HandleClientRegistration) // Dynamic client registration

		// Protected MCP endpoints (require OAuth token)
		r.Group(func(r chi.Router) {
			r.Use(s.context.Middleware) // Extract user from OAuth token

			// Catch-all handler for /mcp root (ChatGPT Apps refresh calls this directly)
			// Must be first to catch root requests before more specific routes
			// Support all HTTP methods to avoid 405 errors
			r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				slog.InfoContext(r.Context(), "request to /mcp", "method", r.Method)
				if r.Method == http.MethodOptions {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
					w.WriteHeader(http.StatusOK)
					return
				}
				s.handleMCPRoot(w, r)
			})

			// Tool discovery (legacy HTTP form for manual testing)
			r.HandleFunc("/tools/list", func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodOptions {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
					w.WriteHeader(http.StatusOK)
					return
				}
				s.handleListTools(w, r)
			})

			// Tool invocation
			r.Post("/tools/call", s.handleCallTool)

			// Resource retrieval
			r.Get("/resources/{uri:.*}", s.handleGetResource)
			r.Post("/resources/list", s.handleListResources)
		})
	})
}

// handleHealth is an unauthenticated liveness/readiness probe. It pings the
// database with a 2s budget and confirms at least one tool is registered.
// Healthy -> 200 {"status":"ok","tools":<n>,"db":"ok"}; otherwise -> 503
// {"status":"unavailable","db":"<msg>"}. The response is intentionally cheap so
// it can be polled frequently under demo load.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.Pool.Ping(ctx); err != nil {
		slog.WarnContext(ctx, "mcp: health check db ping failed", "err", err)
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status": "unavailable",
			"db":     err.Error(),
		})
		return
	}

	toolCount := len(s.tools.GetAllTools())
	if toolCount == 0 {
		slog.WarnContext(ctx, "mcp: health check found no registered tools")
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status": "unavailable",
			"db":     "ok",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"tools":  toolCount,
		"db":     "ok",
	})
}

// handleClientConfig serves a public, unauthenticated Claude Code/Desktop client
// config document so a user can connect in one paste. It reflects the live base
// URL and the actual registered tool list, so it can never drift from what the
// server really serves. Discovery must be unauthenticated, mirroring the OAuth
// metadata endpoints; the config only describes existing tools and auth.
func (s *Server) handleClientConfig(w http.ResponseWriter, r *http.Request) {
	baseURL := s.auth.cfg.MCPBaseURL
	if baseURL == "" {
		baseURL = s.auth.cfg.BaseURL
	}
	mcpURL := fmt.Sprintf("%s/mcp", baseURL)

	allTools := s.tools.GetAllTools()
	tools := make([]map[string]interface{}, 0, len(allTools))
	for _, tool := range allTools {
		tools = append(tools, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
		})
	}

	config := map[string]interface{}{
		"claude_desktop": map[string]interface{}{
			"mcpServers": map[string]interface{}{
				"probably": map[string]interface{}{
					"url": mcpURL,
				},
			},
		},
		"claude_code_cli": fmt.Sprintf("claude mcp add probably %s", mcpURL),
		"auth": map[string]interface{}{
			"type":          "oauth2",
			"discovery_url": fmt.Sprintf("%s/.well-known/oauth-authorization-server", baseURL),
			"scopes":        supportedScopes(),
		},
		"server": map[string]interface{}{
			"name":    "probably-mcp",
			"version": "1.0.0",
			"url":     mcpURL,
		},
		"tools": tools,
	}

	respondJSON(w, http.StatusOK, config)
}

// MCP Request/Response types

type MCPResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
	ID     interface{} `json:"id,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

func newRPCError(code int, message string, data interface{}) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func rpcErrorStatus(err *MCPError) int {
	switch err.Code {
	case -32001:
		return http.StatusUnauthorized
	case -32002:
		return http.StatusForbidden
	case -32003:
		return http.StatusPaymentRequired
	case -32600, -32602:
		return http.StatusBadRequest
	case -32601:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func (s *Server) writeJSONRPCResponse(w http.ResponseWriter, resp jsonRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) writeSSEJSONRPCResponse(w http.ResponseWriter, resp jsonRPCResponse, ctx context.Context) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	payload, err := json.Marshal(resp)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to marshal SSE payload", "err", err)
		return
	}

	if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", payload); err != nil {
		slog.ErrorContext(ctx, "Failed to write SSE frame", "err", err)
		return
	}

	rc := http.NewResponseController(w)
	if err := rc.Flush(); err != nil {
		slog.ErrorContext(ctx, "SSE flush error", "err", err)
		return
	}

	<-ctx.Done()
}

// handleListTools handles tool discovery (list_tools)
func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	result, rpcErr := s.rpcListTools(ctx)
	if rpcErr != nil {
		respondMCPError(w, rpcErrorStatus(rpcErr), rpcErr.Message, rpcErr.Data)
		return
	}

	respondJSON(w, http.StatusOK, MCPResponse{
		Result: result,
	})
}

// handleMCPRoot handles requests to /mcp (root MCP endpoint)
// ChatGPT Apps refresh calls this endpoint directly
// This handler is in the protected group, so middleware has already validated the token
func (s *Server) handleMCPRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
		w.WriteHeader(http.StatusOK)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Error:   newRPCError(-32600, "failed to read request body", err.Error()),
		}
		s.writeJSONRPCResponse(w, resp)
		return
	}

	var rpcReq jsonRPCRequest
	if err := json.Unmarshal(bodyBytes, &rpcReq); err != nil {
		// Malformed JSON (including a syntactically invalid params field, which
		// fails the whole-document decode) is a JSON-RPC parse error (-32700).
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Error:   newRPCError(-32700, "parse error", err.Error()),
		}
		s.writeJSONRPCResponse(w, resp)
		return
	}

	if envErr := validateRPCEnvelope(rpcReq); envErr != nil {
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error:   envErr,
		}
		s.writeJSONRPCResponse(w, resp)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()

	result, rpcErr := s.dispatchJSONRPC(ctx, rpcReq)

	// A bounded-context deadline takes precedence over whatever the tool
	// returned, so the client sees the real cause (a slow/hanging call) rather
	// than a generic execution failure or a stalled request.
	if ctx.Err() == context.DeadlineExceeded {
		rpcErr = newRPCError(-32000, "request timeout", map[string]interface{}{
			"timeout_ms": s.requestTimeout.Milliseconds(),
		})
		result = nil
	}

	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      rpcReq.ID,
	}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		resp.Result = result
	}

	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		s.writeSSEJSONRPCResponse(w, resp, r.Context())
		return
	}

	s.writeJSONRPCResponse(w, resp)
}

// validateRPCEnvelope enforces JSON-RPC 2.0 envelope invariants before dispatch.
// It is pure and package-private: it returns nil for a valid envelope, a -32600
// invalid-request error for a missing/wrong jsonrpc version or empty method, and a
// -32700 parse error when params is present but not valid JSON. Per-tool argument
// validation is intentionally NOT performed here — that remains owned by
// ValidationHandler.ValidateToolArguments in processToolCall.
func validateRPCEnvelope(req jsonRPCRequest) *MCPError {
	if req.JSONRPC != "2.0" {
		return newRPCError(-32600, "invalid request", "jsonrpc must be \"2.0\"")
	}
	if req.Method == "" {
		return newRPCError(-32600, "invalid request", "method is required")
	}
	if len(req.Params) > 0 && !json.Valid(req.Params) {
		return newRPCError(-32700, "parse error", "params is not valid JSON")
	}
	return nil
}

func (s *Server) dispatchJSONRPC(ctx context.Context, req jsonRPCRequest) (interface{}, *MCPError) {
	switch req.Method {
	case "initialize":
		return map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "probably-mcp",
				"version": "1.0.0",
			},
		}, nil
	case "ping":
		return map[string]interface{}{}, nil
	case "tools/list":
		return s.rpcListTools(ctx)
	case "resources/list":
		return s.rpcListResources(ctx)
	case "resources/read":
		return s.rpcReadResource(ctx, req.Params)
	case "tools/call":
		return s.rpcCallTool(ctx, req.Params)
	default:
		return nil, newRPCError(-32601, "method not found", map[string]interface{}{"method": req.Method})
	}
}

func (s *Server) rpcListTools(ctx context.Context) (map[string]interface{}, *MCPError) {
	userCtx := s.context.GetContext(ctx)
	if userCtx == nil {
		return nil, newRPCError(-32001, "unauthorized", nil)
	}

	s.audit.LogToolDiscovery(ctx, userCtx.UserID, userCtx.LedgerID)

	return map[string]interface{}{
		"tools": s.tools.GetAllTools(),
	}, nil
}

func (s *Server) rpcListResources(ctx context.Context) (map[string]interface{}, *MCPError) {
	userCtx := s.context.GetContext(ctx)
	if userCtx == nil {
		return nil, newRPCError(-32001, "unauthorized", nil)
	}

	return map[string]interface{}{
		"resources": s.resources.ListResources(),
	}, nil
}

func (s *Server) rpcReadResource(ctx context.Context, params json.RawMessage) (map[string]interface{}, *MCPError) {
	userCtx := s.context.GetContext(ctx)
	if userCtx == nil {
		return nil, newRPCError(-32001, "unauthorized", nil)
	}

	var req struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &req); err != nil || req.URI == "" {
		return nil, newRPCError(-32602, "invalid params", "uri is required")
	}

	resource, err := s.resources.GetResource(req.URI)
	if err != nil {
		return nil, newRPCError(-32602, "resource not found", req.URI)
	}

	content := map[string]interface{}{
		"uri":      resource.URI,
		"mimeType": resource.MimeType,
		"text":     resource.Content,
	}
	if len(resource.Meta) > 0 {
		content["_meta"] = resource.Meta
	}

	return map[string]interface{}{
		"contents": []map[string]interface{}{content},
	}, nil
}

func (s *Server) rpcCallTool(ctx context.Context, params json.RawMessage) (map[string]interface{}, *MCPError) {
	var req struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, newRPCError(-32602, "invalid params", err.Error())
	}
	return s.processToolCall(ctx, req.Name, req.Arguments)
}

func (s *Server) processToolCall(ctx context.Context, name string, arguments json.RawMessage) (map[string]interface{}, *MCPError) {
	userCtx := s.context.GetContext(ctx)
	if userCtx == nil {
		slog.WarnContext(ctx, "mcp: unauthorized tool call", "tool", name)
		return nil, newRPCError(-32001, "unauthorized", nil)
	}

	slog.InfoContext(ctx, "mcp: tool call", "tool", name, "user_id", userCtx.UserID, "ledger_id", userCtx.LedgerID, "scopes", userCtx.Scopes)

	if name == "" {
		return nil, newRPCError(-32602, "missing tool name", nil)
	}

	if !s.tools.HasTool(name) {
		slog.WarnContext(ctx, "mcp: tool not found", "tool", name)
		return nil, newRPCError(-32601, fmt.Sprintf("tool not found: %s", name), nil)
	}

	tool := s.tools.GetTool(name)
	if err := s.validation.ValidateToolArguments(arguments, tool); err != nil {
		slog.WarnContext(ctx, "mcp: invalid tool arguments", "tool", name, "err", err)
		return nil, newRPCError(-32602, "invalid arguments", err.Error())
	}

	s.audit.LogToolCall(ctx, userCtx.UserID, userCtx.LedgerID, name, arguments)

	if !s.verifyScopes(userCtx.Scopes, name) {
		slog.ErrorContext(ctx, "mcp: scope verification failed", "tool", name, "scopes", userCtx.Scopes)
		return nil, newRPCError(-32002, "insufficient_scope", map[string]interface{}{
			"message":         fmt.Sprintf("Tool '%s' requires one of these scopes, but user has: %v", name, userCtx.Scopes),
			"required_scopes": s.getRequiredScopes(name),
			"user_scopes":     userCtx.Scopes,
		})
	}

	if hasAccess, err := s.checkAccess(ctx, userCtx.UserID); err == nil {
		if !hasAccess {
			slog.ErrorContext(ctx, "mcp: subscription required", "tool", name, "user_id", userCtx.UserID)
			return nil, newRPCError(-32003, "subscription_required", map[string]interface{}{
				"message": "A subscription or trial is required to use this feature. Please subscribe to continue.",
				"content": textContentBlock("You need an active subscription or trial to access financial data through ChatGPT. Start with a 45-day free trial to see how Probably can help you understand your finances."),
			})
		}
		slog.DebugContext(ctx, "mcp: subscription check passed", "tool", name, "user_id", userCtx.UserID)
	} else {
		slog.ErrorContext(ctx, "mcp: subscription check error", "tool", name, "user_id", userCtx.UserID, "err", err)
		s.audit.LogToolError(ctx, userCtx.UserID, userCtx.LedgerID, name, fmt.Errorf("subscription check failed: %w", err))
		// Don't block on subscription check errors - allow the tool to proceed
	}

	// Wrap the single tool attempt in a bounded retry so genuinely transient
	// failures (slow-LLM cancellation, momentary DB/network hiccups, 5xx/429)
	// get a second chance. The retry shares ctx, so it never runs past the
	// per-request deadline, and audit logging stays outside the loop to fire
	// once per overall request. Tool bodies are untouched — this is a wrapper.
	var result map[string]interface{}
	attempts := 0
	err := retryWithBackoff(ctx, maxToolRetries, func() (bool, error) {
		attempts++
		var execErr error
		result, execErr = s.execTool(ctx, userCtx, name, arguments)
		if execErr == nil {
			return false, nil
		}
		retry := isTransientToolError(ctx, execErr)
		if retry {
			slog.WarnContext(ctx, "mcp: transient tool failure, retrying", "tool", name, "attempt", attempts, "user_id", userCtx.UserID, "err", execErr)
		}
		return retry, execErr
	})
	if err != nil {
		slog.ErrorContext(ctx, "mcp: tool execution failed", "tool", name, "user_id", userCtx.UserID, "ledger_id", userCtx.LedgerID, "attempts", attempts, "err", err)
		s.audit.LogToolError(ctx, userCtx.UserID, userCtx.LedgerID, name, err)
		return nil, newRPCError(-32603, "tool execution failed", err.Error())
	}

	if resultJSON, err := json.Marshal(result); err == nil {
		slog.DebugContext(ctx, "mcp: tool response", "tool", name, "user_id", userCtx.UserID, "response", string(resultJSON))
	} else {
		slog.ErrorContext(ctx, "mcp: tool response serialization failed", "tool", name, "user_id", userCtx.UserID, "err", err)
	}

	slog.InfoContext(ctx, "mcp: tool executed", "tool", name, "user_id", userCtx.UserID, "ledger_id", userCtx.LedgerID)
	s.audit.LogToolSuccess(ctx, userCtx.UserID, userCtx.LedgerID, name)
	return result, nil
}

// handleCallTool handles tool invocation (call_tool)
func (s *Server) handleCallTool(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	var req struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondMCPError(w, http.StatusBadRequest, "invalid request", err)
		return
	}

	result, rpcErr := s.processToolCall(ctx, req.Name, req.Arguments)
	if rpcErr != nil {
		respondMCPError(w, rpcErrorStatus(rpcErr), rpcErr.Message, rpcErr.Data)
		return
	}

	// Return result
	respondJSON(w, http.StatusOK, MCPResponse{
		Result: result,
	})
}

// handleGetResource handles resource retrieval (get_resource)
func (s *Server) handleGetResource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user context
	userCtx := s.context.GetContext(ctx)
	if userCtx == nil {
		slog.WarnContext(r.Context(), "mcp: unauthorized resource request", "remote_addr", r.RemoteAddr)
		respondMCPError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	// Get URI from path
	uri := chi.URLParam(r, "uri")
	if uri == "" {
		slog.WarnContext(r.Context(), "mcp: missing resource URI", "remote_addr", r.RemoteAddr)
		respondMCPError(w, http.StatusBadRequest, "missing resource URI", nil)
		return
	}

	slog.InfoContext(r.Context(), "mcp: resource request", "uri", uri, "user_id", userCtx.UserID, "ledger_id", userCtx.LedgerID)

	// Get resource
	resource, err := s.resources.GetResource(uri)
	if err != nil {
		slog.WarnContext(r.Context(), "mcp: resource not found", "uri", uri, "err", err)
		respondMCPError(w, http.StatusNotFound, "resource not found", err)
		return
	}

	slog.InfoContext(r.Context(), "mcp: serving resource", "uri", uri, "content_length", len(resource.Content), "mime", resource.MimeType)

	// Set appropriate MIME type
	w.Header().Set("Content-Type", resource.MimeType)

	// Return resource content
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(resource.Content))
}

// handleListResources handles resource listing (list_resources)
func (s *Server) handleListResources(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	result, rpcErr := s.rpcListResources(ctx)
	if rpcErr != nil {
		respondMCPError(w, rpcErrorStatus(rpcErr), rpcErr.Message, rpcErr.Data)
		return
	}

	respondJSON(w, http.StatusOK, MCPResponse{
		Result: result,
	})
}

// executeTool executes a tool with the given arguments
func (s *Server) executeTool(ctx context.Context, userCtx *UserContext, toolName string, arguments json.RawMessage) (map[string]interface{}, error) {
	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Execute tool based on name
	switch toolName {
	case "get_spending_summary":
		return s.executeGetSpendingSummary(ctx, userCtx, args)
	case "get_account_balances":
		return s.executeGetAccountBalances(ctx, userCtx, args)
	case "ask_question":
		return s.executeAskQuestion(ctx, userCtx, args)
	case "get_spending_trends":
		return s.executeGetSpendingTrends(ctx, userCtx, args)
	case "get_recurring_patterns":
		return s.executeGetRecurringPatterns(ctx, userCtx, args)
	case "search_transactions":
		return s.executeSearchTransactions(ctx, userCtx, args)
	case "get_financial_overview":
		return s.executeGetFinancialOverview(ctx, userCtx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Tool execution implementations

func (s *Server) executeGetSpendingSummary(ctx context.Context, userCtx *UserContext, args map[string]interface{}) (map[string]interface{}, error) {
	// Build query params
	period := "month"
	if p, ok := args["period"].(string); ok {
		period = p
	}

	// Call API
	path := fmt.Sprintf("/transactions?period=%s", period)
	data, err := s.apiClient.Get(ctx, path, userCtx.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	// Process transactions to calculate spending summary
	transactions, ok := data["data"].([]interface{})
	if !ok {
		transactions = []interface{}{}
	}

	// Calculate totals by category
	totalCents := int64(0)
	byCategory := make(map[string]int64)

	for _, txn := range transactions {
		txnMap, ok := txn.(map[string]interface{})
		if !ok {
			continue
		}

		// Get entries (expenses have negative amounts)
		entries, ok := txnMap["entries"].([]interface{})
		if !ok {
			continue
		}

		for _, entry := range entries {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}

			amountCents, ok := entryMap["amount_cents"].(float64)
			if !ok {
				continue
			}

			// Only count expenses (negative amounts)
			if amountCents < 0 {
				totalCents += int64(-amountCents)

				// Get category from tags or description
				category := "Uncategorized"
				if tags, ok := txnMap["tags"].([]interface{}); ok && len(tags) > 0 {
					if tag, ok := tags[0].(map[string]interface{}); ok {
						if name, ok := tag["name"].(string); ok {
							category = name
						}
					}
				}

				byCategory[category] += int64(-amountCents)
			}
		}
	}

	// Convert to array
	categoryBreakdown := make([]map[string]interface{}, 0, len(byCategory))
	for cat, amount := range byCategory {
		categoryBreakdown = append(categoryBreakdown, map[string]interface{}{
			"category":     cat,
			"amount_cents": amount,
			"amount":       fmt.Sprintf("$%.2f", float64(amount)/100),
		})
	}

	// Return MCP response format
	return map[string]interface{}{
		"structuredContent": map[string]interface{}{
			"total_cents": totalCents,
			"total":       fmt.Sprintf("$%.2f", float64(totalCents)/100),
			"period":      period,
			"by_category": categoryBreakdown,
		},
		"content": textContentBlock(fmt.Sprintf("Total spending for %s: $%.2f", period, float64(totalCents)/100)),
		"_meta": map[string]interface{}{
			"full_transactions": transactions,
			"chart_config": map[string]interface{}{
				"type": "bar",
				"data": categoryBreakdown,
			},
			"openai/outputTemplate": "ui://widget/spending-summary.html",
		},
	}, nil
}

func (s *Server) executeGetAccountBalances(ctx context.Context, userCtx *UserContext, args map[string]interface{}) (map[string]interface{}, error) {
	// Call dashboard API
	data, err := s.apiClient.Get(ctx, "/dashboard", userCtx.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard: %w", err)
	}

	getCents := func(key string) int64 {
		val, ok := data[key]
		if !ok {
			return 0
		}

		switch v := val.(type) {
		case float64:
			return int64(math.Round(v))
		case float32:
			return int64(math.Round(float64(v)))
		case int64:
			return v
		case int:
			return int64(v)
		case json.Number:
			parsed, err := v.Int64()
			if err == nil {
				return parsed
			}
			floatVal, err := v.Float64()
			if err == nil {
				return int64(math.Round(floatVal))
			}
		}
		return 0
	}

	netWorthCents := getCents("net_worth")
	totalAssetsCents := getCents("total_assets")
	totalLiabilitiesCents := getCents("total_liabilities")
	if totalLiabilitiesCents < 0 {
		totalLiabilitiesCents = -totalLiabilitiesCents
	}

	netWorthDisplay := models.FormatCents(netWorthCents)
	totalAssetsDisplay := models.FormatCents(totalAssetsCents)
	totalLiabilitiesDisplay := models.FormatCents(totalLiabilitiesCents)

	// Return MCP response format
	return map[string]interface{}{
		"structuredContent": map[string]interface{}{
			"net_worth":         netWorthDisplay,
			"total_assets":      totalAssetsDisplay,
			"total_liabilities": totalLiabilitiesDisplay,
		},
		"content": textContentBlock(fmt.Sprintf(
			"Net worth: %s (Assets: %s, Liabilities: %s)",
			netWorthDisplay,
			totalAssetsDisplay,
			totalLiabilitiesDisplay,
		)),
		"_meta": map[string]interface{}{
			"full_data": data,
			"raw_values": map[string]interface{}{
				"net_worth_cents":         netWorthCents,
				"total_assets_cents":      totalAssetsCents,
				"total_liabilities_cents": totalLiabilitiesCents,
			},
			"openai/outputTemplate": "ui://widget/account-balances.html",
		},
	}, nil
}

func (s *Server) executeAskQuestion(ctx context.Context, userCtx *UserContext, args map[string]interface{}) (map[string]interface{}, error) {
	question, ok := args["question"].(string)
	if !ok || question == "" {
		return nil, fmt.Errorf("question is required")
	}

	// Call Chat API to get AI-powered answer
	// Use POST /api/v1/chat/ask with JSON response (non-streaming)
	// Note: Chat API expects Accept: application/json (not text/event-stream)
	requestBody := map[string]interface{}{
		"question":  question,
		"ledger_id": userCtx.LedgerID.String(),
		// Don't include thread_id - let it create a new thread for each question
	}

	data, err := s.apiClient.Post(ctx, "/chat/ask", userCtx.APIKey, requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat answer: %w", err)
	}

	// Extract answer from ChatResponse structure
	answer := ""
	if ans, ok := data["answer"].(string); ok && ans != "" {
		answer = ans
	} else if ans, ok := data["response"].(string); ok && ans != "" {
		answer = ans
	}

	// If no answer found, try to construct from other fields
	if answer == "" {
		// Try to build answer from summary and data
		summary := ""
		if s, ok := data["summary"].(string); ok {
			summary = s
		}

		count := 0
		if c, ok := data["count"].(float64); ok {
			count = int(c)
		} else if c, ok := data["count"].(int); ok {
			count = c
		}

		if summary != "" {
			answer = summary
			if count > 0 {
				answer = fmt.Sprintf("%s\n\nFound %d result(s).", answer, count)
			}
		} else if count > 0 {
			answer = fmt.Sprintf("Found %d result(s) for your query.", count)
		} else {
			answer = "I couldn't generate an answer to that question. Please try rephrasing it."
		}
	}

	// Return MCP response format
	return map[string]interface{}{
		"structuredContent": map[string]interface{}{
			"answer":   answer,
			"question": question,
		},
		"content": textContentBlock(answer),
		"_meta": map[string]interface{}{
			"openai/outputTemplate": "ui://widget/ask-question.html",
			"full_response":         data,
		},
	}, nil
}

func (s *Server) executeGetSpendingTrends(ctx context.Context, userCtx *UserContext, args map[string]interface{}) (map[string]interface{}, error) {
	period := "month"
	if p, ok := args["period"].(string); ok {
		period = p
	}

	// Call transactions API
	path := fmt.Sprintf("/transactions?period=%s", period)
	data, err := s.apiClient.Get(ctx, path, userCtx.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	// Process for trends (simplified - would need proper date grouping)
	transactions, _ := data["data"].([]interface{})

	// Calculate trend
	totalCents := int64(0)
	for _, txn := range transactions {
		txnMap, ok := txn.(map[string]interface{})
		if !ok {
			continue
		}
		entries, ok := txnMap["entries"].([]interface{})
		if !ok {
			continue
		}
		for _, entry := range entries {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			if amountCents, ok := entryMap["amount_cents"].(float64); ok && amountCents < 0 {
				totalCents += int64(-amountCents)
			}
		}
	}

	return map[string]interface{}{
		"structuredContent": map[string]interface{}{
			"trend":  "up", // Simplified
			"change": 12.5, // Simplified
			"period": period,
		},
		"content": textContentBlock(fmt.Sprintf("Spending trend for %s period", period)),
		"_meta": map[string]interface{}{
			"time_series_data": transactions,
			"chart_config": map[string]interface{}{
				"type": "line",
				"data": transactions,
			},
			"openai/outputTemplate": "ui://widget/spending-trends.html",
		},
	}, nil
}

func (s *Server) executeGetRecurringPatterns(ctx context.Context, userCtx *UserContext, args map[string]interface{}) (map[string]interface{}, error) {
	// Call patterns API
	// Note: The API returns an array directly, but our API client wraps it in a map
	data, err := s.apiClient.Get(ctx, "/patterns", userCtx.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get patterns: %w", err)
	}

	// API returns array directly (respondJSON sends array, but Get() wraps in map)
	// Try to extract from common response formats
	var patterns []interface{}
	if patternsArray, ok := data["data"].([]interface{}); ok {
		patterns = patternsArray
	} else {
		// If API returns array directly, it might be in the response
		// For now, default to empty array
		patterns = []interface{}{}
	}

	// Calculate totals
	count := len(patterns)
	totalMonthlyCents := int64(0)
	for _, p := range patterns {
		pMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if avgAmount, ok := pMap["avg_amount_cents"].(float64); ok {
			totalMonthlyCents += int64(avgAmount)
		}
	}

	return map[string]interface{}{
		"structuredContent": map[string]interface{}{
			"count":               count,
			"total_monthly_cents": totalMonthlyCents,
			"total_monthly":       fmt.Sprintf("$%.2f", float64(totalMonthlyCents)/100),
		},
		"content": textContentBlock(fmt.Sprintf("Found %d recurring patterns totaling $%.2f/month", count, float64(totalMonthlyCents)/100)),
		"_meta": map[string]interface{}{
			"patterns":              patterns,
			"openai/outputTemplate": "ui://widget/recurring-patterns.html",
		},
	}, nil
}

func (s *Server) executeSearchTransactions(ctx context.Context, userCtx *UserContext, args map[string]interface{}) (map[string]interface{}, error) {
	query, _ := args["query"].(string)
	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	// Build path
	path := fmt.Sprintf("/transactions?limit=%d", limit)
	if query != "" {
		path += "&search=" + query
	}

	// Call API
	data, err := s.apiClient.Get(ctx, path, userCtx.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to search transactions: %w", err)
	}

	transactions, _ := data["data"].([]interface{})

	return map[string]interface{}{
		"structuredContent": map[string]interface{}{
			"count": len(transactions),
			"query": query,
		},
		"content": textContentBlock(fmt.Sprintf("Found %d transactions matching '%s'", len(transactions), query)),
		"_meta": map[string]interface{}{
			"transactions":          transactions,
			"openai/outputTemplate": "ui://widget/search-transactions.html",
		},
	}, nil
}

func (s *Server) executeGetFinancialOverview(ctx context.Context, userCtx *UserContext, args map[string]interface{}) (map[string]interface{}, error) {
	// Get dashboard data
	dashboard, err := s.apiClient.Get(ctx, "/dashboard", userCtx.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard: %w", err)
	}

	// Get recent transactions
	transactions, err := s.apiClient.Get(ctx, "/transactions?limit=10", userCtx.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	netWorth, _ := dashboard["net_worth"].(float64)
	totalAssets, _ := dashboard["total_assets"].(float64)
	totalLiabilities, _ := dashboard["total_liabilities"].(float64)

	return map[string]interface{}{
		"structuredContent": map[string]interface{}{
			"net_worth":         netWorth,
			"total_assets":      totalAssets,
			"total_liabilities": totalLiabilities,
		},
		"content": textContentBlock(fmt.Sprintf("Financial Overview: Net Worth $%.2f", netWorth/100)),
		"_meta": map[string]interface{}{
			"dashboard":             dashboard,
			"recent_transactions":   transactions,
			"openai/outputTemplate": "ui://widget/financial-overview.html",
		},
	}, nil
}

// Helper functions

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondMCPError(w http.ResponseWriter, status int, message string, data interface{}) {
	respondJSON(w, status, MCPResponse{
		Error: &MCPError{
			Code:    status,
			Message: message,
			Data:    data,
		},
	})
}

// Context keys for Authboss (matching authboss package constants)
const (
	authbossPIDKey  contextKey = "authboss_pid"
	authbossUserKey contextKey = "authboss_user"
)

// loadAuthbossUserMiddleware loads user from Authboss session into context
// This is needed for the OAuth authorization endpoint
func (s *Server) loadAuthbossUserMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use Authboss to get current user ID
			// We use type assertion since ab is stored as interface{}
			type AuthbossInterface interface {
				CurrentUserID(r *http.Request) (string, error)
			}

			if ab, ok := s.ab.(AuthbossInterface); ok {
				pid, err := ab.CurrentUserID(r)
				if err == nil && pid != "" {
					// Load full user
					userStore := models.NewUserStore(s.db.Pool)
					user, err := userStore.GetByEmail(r.Context(), pid)
					if err == nil && user != nil {
						// Add to context using typed context keys
						// This matches how auth.LoadUserMiddleware works
						ctx := r.Context()
						ctx = context.WithValue(ctx, authbossPIDKey, pid)
						ctx = context.WithValue(ctx, authbossUserKey, user)
						r = r.WithContext(ctx)
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getRequiredScopes returns the required scopes for a tool (for error messages)
func (s *Server) getRequiredScopes(toolName string) []string {
	toolScopes := map[string][]string{
		"get_spending_summary":   {"read:transactions", "read:financial"},
		"get_account_balances":   {"read:accounts", "read:financial"},
		"ask_question":           {"read:transactions", "read:accounts", "read:financial"},
		"get_spending_trends":    {"read:transactions", "read:financial"},
		"get_recurring_patterns": {"read:patterns", "read:transactions"},
		"search_transactions":    {"read:transactions"},
		"get_financial_overview": {"read:transactions", "read:accounts", "read:financial"},
	}
	if scopes, ok := toolScopes[toolName]; ok {
		return scopes
	}
	return []string{}
}

// verifyScopes checks if the user's scopes are sufficient for the tool
func (s *Server) verifyScopes(userScopes []string, toolName string) bool {
	required := s.getRequiredScopes(toolName)
	if len(required) == 0 {
		// Unknown tool - allow for now
		return true
	}

	// Check if user has at least one required scope
	scopeMap := make(map[string]bool)
	for _, scope := range userScopes {
		scopeMap[scope] = true
	}

	for _, requiredScope := range required {
		if scopeMap[requiredScope] {
			return true
		}
	}

	return false
}
