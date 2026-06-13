# ChatGPT App Setup Guide

This guide explains how to set up and configure the **Probably ChatGPT App** - a consumer-facing financial app that integrates with ChatGPT using the OpenAI Apps SDK and MCP (Model Context Protocol) server.

## Overview

The **Probably ChatGPT App** is a consumer-facing financial application that integrates with ChatGPT. Users can:

- Ask questions about their finances in natural language
- View spending summaries, account balances, and financial trends
- Get insights about recurring patterns and subscriptions
- Search transactions and get financial overviews

The app uses:
- **OAuth 2.1 with PKCE** for secure authentication (required by ChatGPT Apps)
- **MCP Server** to expose financial tools to ChatGPT
- **OpenAI Apps SDK UI** for rich, interactive widgets
- **Existing Probably APIs** for data access

## Architecture

```
ChatGPT → MCP Server → Existing APIs (/api/v1/*) → Database
         ↓
    UI Resources (HTML + JS bundles)
```

## Prerequisites

1. **Probably server** running with API endpoints available
2. **User account** in Probably (users must be logged in)
3. **MCP server** configured and running
4. **ChatGPT App** configured in OpenAI's developer portal

## Configuration

### Environment Variables

Add these to your `.env` file:

```bash
# MCP Server Port (default: 8081, only used for standalone mode)
MCP_SERVER_PORT=8081

# OAuth Configuration
# NOTE: Client ID and Secret are NOT required!
# The MCP server uses Dynamic Client Registration - ChatGPT will register itself
# automatically via POST /mcp/register. Clients are stored in the database.
# These environment variables are currently unused (may be used for future features).
# MCP_OAUTH_CLIENT_ID=
# MCP_OAUTH_CLIENT_SECRET=

# Public URL for MCP server (for OAuth redirects and metadata)
# If not set, defaults to BASE_URL
# MCP_BASE_URL=https://mcp.probably.money

# CDN URL for UI resources
# MCP_UI_CDN_URL=https://cdn.probably.money/mcp-ui/

# Audit logging (default: true)
MCP_AUDIT_LOG_ENABLED=true

# Data retention period in days (default: 90)
MCP_DATA_RETENTION_DAYS=90
```

## Running the MCP Server

You have two options for running the MCP server:

### Option 1: Integrated into Main Server (Recommended)

The MCP server is **automatically integrated** into the main Probably server and available at `/mcp`:

```bash
# Start the main server (includes MCP at /mcp)
make dev-app

# Or build and run
make build-server
make server
```

The MCP endpoints will be available at:
- `http://localhost:8080/mcp/auth` - OAuth authorization
- `http://localhost:8080/mcp/callback` - Token exchange
- `http://localhost:8080/mcp/tools/list` - Tool discovery
- `http://localhost:8080/mcp/tools/call` - Tool invocation
- etc.

**Benefits:**
- Single server process
- Simpler deployment
- Shared authentication and database connections
- Recommended for production

### Option 2: Standalone Server (For Testing)

Run the MCP server as a separate process on port 8081:

```bash
# Using Make
make dev-mcp-standalone

# Or directly
go run cmd/mcp-server/main.go
```

The server will start on port 8081 (or the port specified in `MCP_SERVER_PORT`).

**Use cases:**
- Testing MCP server independently
- Development/debugging
- Running on a different port or host

## Authentication

The MCP server uses **OAuth 2.1 with PKCE** for secure authentication, as required by ChatGPT Apps.

### OAuth Flow

**Important**: You don't need to configure OAuth client credentials manually. ChatGPT will register itself automatically.

1. **Discovery**: ChatGPT reads the OAuth metadata from `/.well-known/oauth-protected-resource`
2. **Client Registration**: ChatGPT dynamically registers as an OAuth client via `POST /mcp/register`
   - The server generates a unique `client_id` (UUID) and stores it in the database
   - No `client_secret` is needed (public client with PKCE)
   - ChatGPT receives the `client_id` and uses it for subsequent requests
3. **Authorization**: 
   - User initiates OAuth flow in ChatGPT
   - ChatGPT redirects to `GET /mcp/auth` with OAuth parameters
   - If user is not logged into Probably, they're redirected to login
   - After login, user is redirected back to authorization endpoint
   - User authorizes the app (auto-approved for now, consent screen can be added)
   - Authorization code is generated and returned to ChatGPT
4. **Token Exchange**: ChatGPT exchanges the authorization code for an access token via `POST /mcp/callback` (with PKCE verification)
5. **Authenticated Requests**: ChatGPT includes the access token in the `Authorization` header:

```
Authorization: Bearer <oauth_access_token>
```

Behind the scenes the MCP server immediately exchanges that access token for a short-lived Probably API key and stores the mapping. Every internal `/api/v1/*` request still uses `Bearer prob_...` so we didn't need to loosen our existing API authentication rules.

**Note**: Users must be logged into Probably before authorizing the ChatGPT App. The OAuth flow integrates with Authboss session management.

### OAuth Endpoints

**Discovery Endpoints:**
- `GET /.well-known/oauth-authorization-server` - OAuth authorization server metadata (RFC 8414)
- `GET /.well-known/oauth-protected-resource` - OAuth protected resource metadata (RFC 7662)
- Both also available at `/mcp/.well-known/*` for compatibility

**OAuth Flow Endpoints:**
- `GET /mcp/auth` - Authorization endpoint (user login & consent)
- `POST /mcp/callback` - Token endpoint (exchange code for token)
- `POST /mcp/register` - Dynamic client registration

### Scopes

Available scopes:
- `read:transactions` - Read transaction data
- `read:accounts` - Read account information
- `read:financial` - Read financial summaries and overviews
- `read:patterns` - Read recurring patterns

Users must be logged into Probably before authorizing the ChatGPT App.

## Available Tools

### `get_spending_summary`
Get spending summary by category and time period.

**Parameters:**
- `period` (optional): "week", "month", "quarter", "year" (default: "month")
- `start_date` (optional): Start date (YYYY-MM-DD)
- `end_date` (optional): End date (YYYY-MM-DD)

### `get_account_balances`
Get current account balances including net worth, assets, and liabilities.

**Parameters:** None

### `ask_question`
Ask a complex question about financial data using AI.

**Parameters:**
- `question` (required): The user's question

### `get_spending_trends`
Get spending trends over time.

**Parameters:**
- `period` (optional): Time period (default: "month")
- `group_by` (optional): "day", "week", "month" (default: "day")

### `get_recurring_patterns`
Get detected recurring patterns (subscriptions, bills).

**Parameters:** None

### `search_transactions`
Search for specific transactions.

**Parameters:**
- `query` (optional): Search query
- `start_date` (optional): Start date (YYYY-MM-DD)
- `end_date` (optional): End date (YYYY-MM-DD)
- `limit` (optional): Max results (default: 50)

### `get_financial_overview`
Get comprehensive financial overview (dashboard).

**Parameters:** None

## MCP Transport & JSON-RPC Contract

ChatGPT Apps never calls our REST endpoints directly. Instead it speaks JSON-RPC 2.0 over an SSE connection to the root of the MCP server (e.g. `POST https://your-domain.com/`). The SDK always:

1. Opens an HTTP connection with `Accept: text/event-stream`.
2. Sends a JSON-RPC envelope in the request body.
3. Expects every server reply to be streamed back as an SSE `data:` frame that contains a JSON-RPC response object (`jsonrpc`, `id`, `result|error`).

### Required Methods

| Method | When ChatGPT calls it | Expected response |
| --- | --- | --- |
| `initialize` | First call on every connection | `{ \"protocolVersion\": \"2025-03-26\", \"capabilities\": { \"tools\": {} }, \"serverInfo\": { \"name\": \"probably-mcp\", \"version\": \"1.0.0\" } }` |
| `ping` | Periodic keep alive (optional from client) | Return `{}` to acknowledge |
| `tools/list` | When ChatGPT refreshes actions | `{ \"tools\": [...] }` with full tool definitions |
| `tools/call` | When the model invokes a tool | Tool-specific JSON payload |
| `resources/list` | During UI bootstrap | `{ \"resources\": [...] }` including `_meta` |
| `resources/read` | When the UI iframe loads a template | `{ \"contents\": [{ \"uri\", \"mimeType\", \"text\", \"_meta\" }] }` |

All other methods should be rejected with a JSON-RPC error `{ \"code\": -32601, \"message\": \"Method not found\" }`.

### SSE Response Helper

Always flush responses using a single helper so every handler writes:

```json
event: message
data: {
  "jsonrpc": "2.0",
  "id": 1,
  "result": { ... }
}
```

ChatGPT will hold the connection open until it receives the final `data:` frame, so do **not** fall back to plain JSON responses unless the client sent `Accept: application/json`.

## Legacy HTTP Endpoints (still useful for manual testing)

### List Tools
```
POST /mcp/tools/list
Authorization: Bearer <api_key>
```

Returns all available tools.

### Call Tool
```
POST /mcp/tools/call
Authorization: Bearer <api_key>
Content-Type: application/json

{
  "name": "get_spending_summary",
  "arguments": {
    "period": "month"
  }
}
```

### Get Resource
```
GET /mcp/resources/{uri}
Authorization: Bearer <api_key>
```

Returns HTML template for UI widget.

### List Resources
```
POST /mcp/resources/list
Authorization: Bearer <api_key>
```

Returns all registered UI resources.

## Response Format

Tool responses follow the MCP protocol with OpenAI Apps SDK extensions:

```json
{
  "structuredContent": {
    // Concise data for both widget and model
  },
  "content": "Optional narration text for model",
  "_meta": {
    // Large/sensitive data only for widget (never reaches model)
    "openai/outputTemplate": "ui://widget/spending-summary.html"
  }
}
```

## UI Resources

UI resources are HTML templates with `text/html+skybridge` MIME type that load React components.

### Widget Metadata Requirements

Each entry in `[internal/mcp/resources.go](internal/mcp/resources.go)` must expose `_meta` so ChatGPT can validate your sandbox:

- `openai/widgetDescription`: one sentence summary that appears in the Apps dashboard.
- `openai/widgetDomain`: Fully-qualified domain that serves the widget assets (derived from `MCP_BASE_URL` or `BASE_URL`).
- `openai/widgetCSP`: CSP object (`connect_domains`, `resource_domains`) that whitelists ChatGPT (`https://chatgpt.com`) and your API/CDN origins.
- `openai/widgetAccessible`: Set to `true` only if the widget calls `window.openai.callTool` on its own (e.g. ask-question, search-transactions).

Without these fields the Apps portal will block submission with “Widget CSP/domain not set” warnings (see screenshot in troubleshooting).

### Building UI Bundles

```bash
cd cmd/ui-bundles
npm install
npm run build
```

Bundles are output to `dist/` and can be:
- Served via CDN (set `MCP_UI_CDN_URL`)
- Embedded inline in HTML templates
- Referenced via script tags

### HTML Templates

Templates are in `static/mcp-ui/` and reference JS bundles:

```html
<div id="root"></div>
<script type="module" src="https://cdn.probably.money/mcp-ui/spending-summary.js"></script>
```

## ChatGPT App Configuration in OpenAI Portal

When adding your app in the ChatGPT Apps developer portal, you'll see an "Authentication" dropdown. Here's how to configure it:

### Step 1: Select "OAuth"

Choose **"OAuth"** from the Authentication dropdown (as shown in your screenshot).

### Step 2: Provide MCP Server Base URL

**Important**: Use the **root domain URL**, not the `/mcp` path!

ChatGPT Apps needs your server's base URL:

- **If integrated into main server**: `https://probably.money` (root domain, NOT `https://probably.money/mcp`)
- **If standalone**: `https://mcp.probably.money` (root domain, NOT `https://mcp.probably.money/mcp`)

**Example with ngrok:**
- ✅ Correct: `https://cheliform-sleetier-landen.ngrok-free.dev`
- ❌ Wrong: `https://cheliform-sleetier-landen.ngrok-free.dev/mcp`

ChatGPT will discover the MCP endpoints at `/mcp/*` automatically.

### Step 3: OAuth Discovery (Automatic)

ChatGPT will automatically discover OAuth endpoints from:
```
GET https://your-domain.com/.well-known/oauth-protected-resource
```

This returns all OAuth configuration including:
- `authorization_endpoint`: `/mcp/auth`
- `token_endpoint`: `/mcp/callback`
- `registration_endpoint`: `/mcp/register`

### Step 4: Client Credentials

**Important**: You don't need to provide OAuth client ID or secret!

The ChatGPT Apps UI might show fields for:
- **Client ID**: Leave empty or skip (ChatGPT will register itself automatically)
- **Client Secret**: Leave empty (not needed - we use PKCE for public clients)

ChatGPT will:
1. Call `POST /mcp/register` to register itself
2. Receive a `client_id` (UUID) from your server
3. Use that `client_id` for the OAuth flow

### What to Enter

If the UI asks for specific OAuth fields:
- **Authorization URL**: Will be auto-discovered from `/.well-known/oauth-protected-resource`
- **Token URL**: Will be auto-discovered
- **Client ID**: Leave empty (ChatGPT registers itself)
- **Client Secret**: Leave empty (not used with PKCE)

**The key is providing the base URL** - ChatGPT will discover everything else automatically.

## Testing with ChatGPT

1. **Start the MCP server**: `make dev-app` (integrated) or `make dev-mcp-standalone` (standalone)
2. **Configure ChatGPT App** in OpenAI's developer portal:
   - Set MCP server **root URL** (e.g., `https://probably.money` or your ngrok URL)
   - **Important**: Use the root domain, NOT `/mcp` path
   - Select "OAuth" authentication
   - OAuth endpoints will be discovered automatically from `/.well-known/oauth-protected-resource`
3. **Verify OAuth discovery**: ChatGPT should automatically discover:
   - Authorization endpoint: `https://your-domain.com/mcp/auth`
   - Token endpoint: `https://your-domain.com/mcp/callback`
   - Registration endpoint: `https://your-domain.com/mcp/register`
3. **User Flow**:
   - User opens ChatGPT and selects Probably app
   - ChatGPT initiates OAuth flow
   - User is redirected to Probably login (if not logged in)
   - User authorizes the app
   - ChatGPT receives access token
   - User can now ask questions about their finances
4. **Test tool calls** through ChatGPT conversation

## Security Considerations

- **Input Validation**: All tool inputs are validated server-side
- **Audit Logging**: All tool calls are logged (with PII redaction)
- **Scope Verification**: OAuth scopes will be verified (when implemented)
- **Rate Limiting**: Consider adding rate limiting for production
- **HTTPS**: Always use HTTPS in production

## Troubleshooting

### Error: "MCP server does not implement OAuth"

**Problem**: ChatGPT can't find the OAuth metadata endpoint.

**Solutions**:
1. **Check base URL**: Make sure you're using the **root domain**, not `/mcp` path:
   - ✅ Correct: `https://your-domain.com`
   - ❌ Wrong: `https://your-domain.com/mcp`

2. **Verify OAuth endpoint is accessible**:
   ```bash
   curl https://your-domain.com/.well-known/oauth-protected-resource
   ```
   Should return JSON with OAuth metadata.

3. **Check MCP_BASE_URL environment variable**:
   - If set, it should be the root domain (without `/mcp`)
   - If not set, it defaults to `BASE_URL`

4. **For ngrok**: Use the ngrok root URL:
   - ✅ `https://cheliform-sleetier-landen.ngrok-free.dev`
   - ❌ `https://cheliform-sleetier-landen.ngrok-free.dev/mcp`

### MCP Server Won't Start

- Check database connection
- Verify port is not in use
- Check environment variables

### Authentication Fails

- Verify API key is valid
- Check Authorization header format: `Bearer <key>`
- Ensure user has access to ledger

### Tools Return Errors

- Check API endpoints are accessible
- Verify ledger ID is correct
- Review audit logs for details

## Next Steps

- Implement OAuth 2.1 with PKCE
- Add more tools as needed
- Enhance UI components with Charts SDK
- Deploy UI bundles to CDN
- Add rate limiting and monitoring

## References

- [OpenAI Apps SDK Documentation](https://openai.github.io/apps-sdk-ui/)
- [MCP Protocol Specification](https://developers.openai.com/apps-sdk/build/mcp-server)
- [Apps SDK Security Guidelines](https://developers.openai.com/apps-sdk/guides/security-privacy)
