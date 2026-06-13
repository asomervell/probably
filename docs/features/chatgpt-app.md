# ChatGPT App Integration

Probably integrates with ChatGPT Apps, allowing users to access their financial data through ChatGPT's conversational interface. This integration uses the Model Context Protocol (MCP) to expose financial tools and provides rich, interactive widgets for visualizing financial data.

## Overview

The ChatGPT App integration enables users to:

- Ask questions about their finances in natural language through ChatGPT
- View spending summaries, account balances, and financial trends
- Get insights about recurring patterns and subscriptions
- Search transactions and get financial overviews
- Access all financial data through ChatGPT's conversational interface

## Architecture

The integration uses:

- **MCP Server** - Model Context Protocol server that exposes financial tools to ChatGPT
- **OAuth 2.1 with PKCE** - Secure authentication required by ChatGPT Apps
- **OpenAI Apps SDK UI** - Rich, interactive React widgets for financial visualizations
- **Existing Probably APIs** - All tools use the existing `/api/v1/*` endpoints

```
ChatGPT → MCP Server → Existing APIs (/api/v1/*) → Database
         ↓
    UI Resources (HTML + JS bundles)
```

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

## Authentication

The integration uses **OAuth 2.1 with PKCE** for secure authentication:

1. **User Authorization**: Users must be logged into Probably first
2. **OAuth Flow**: ChatGPT redirects to Probably for authorization
3. **Token Exchange**: ChatGPT receives an access token after authorization
4. **API Access**: All tool calls use the access token to authenticate API requests

**Security Features:**
- PKCE (Proof Key for Code Exchange) prevents authorization code interception
- Dynamic client registration - no manual client configuration needed
- OAuth tokens are mapped to short-lived API keys for internal API access
- Audit logging tracks all MCP operations

## UI Widgets

Each tool can return interactive UI widgets built with React:

- **Spending Summary** - Category breakdown with visualizations
- **Account Balances** - Net worth, assets, and liabilities cards
- **Financial Overview** - Comprehensive dashboard view
- **Recurring Patterns** - Subscription and bill tracking
- **Spending Trends** - Time series charts and analysis
- **Transaction Search** - Search results with transaction details
- **Ask Question** - AI-powered Q&A interface

All widgets support light/dark themes and are responsive.

## Configuration

The MCP server requires environment variables for configuration:

- `MCP_BASE_URL` - Public URL for MCP server (required)
- `MCP_UI_CDN_URL` or `CDN_DOMAIN` - CDN URL for UI resources (required)
- `MCP_SERVER_PORT` - Port for standalone mode (default: 8081)
- `MCP_AUDIT_LOG_ENABLED` - Enable audit logging (default: true)
- `MCP_DATA_RETENTION_DAYS` - Data retention period (default: 90)

See [ChatGPT App Setup Guide](../../docs/chatgpt-app-setup.md) for detailed configuration instructions.

## Subscription Requirements

Users can connect ChatGPT Apps without a subscription, but tool execution requires an active subscription or trial. This allows users to connect and see the integration, with subscription requirements enforced when actually using features.

## Related Documentation

- [ChatGPT App Setup Guide](../../docs/chatgpt-app-setup.md) - Complete setup and configuration guide
- [AI Chat](ai-chat.md) - Web-based AI chat interface
- [Multi-Provider Bank Connections](bank-connections.md) - Bank account connections
