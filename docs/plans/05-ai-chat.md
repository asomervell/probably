# AI Chat Feature Plan

## Overview

Conversational AI interface that lets users ask questions about their finances in natural language. The LLM generates SQL queries that execute client-side via sql.js (SQLite in WASM), keeping data local and enabling complex analysis without server-side code execution.

## Status: Not Started

## Dependencies

None - this is an independent feature (though it will live on the Pulse page).

## What We're Building

A conversational interface where users can ask financial questions in natural language:

- "How much did I spend on coffee last quarter?"
- "What's my biggest expense category this year?"
- "Am I spending more on dining out than last month?"
- "Show me all transactions over $500"

The LLM generates SQL queries that run **client-side** via sql.js (SQLite compiled to WASM).

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         AI CHAT FLOW                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  User: "How much did I spend on coffee this year?"                  │
│                           │                                         │
│                           ▼                                         │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  1. CONTEXT PREPARATION (Server)                            │    │
│  │     - Fetch schema description                              │    │
│  │     - Fetch relevant context (merchant list, tag list)      │    │
│  │     - Build prompt with user question + context             │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                           │                                         │
│                           ▼                                         │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  2. LLM GENERATES RESPONSE (Server)                         │    │
│  │     - Natural language explanation                          │    │
│  │     - SQL query to fetch data                               │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                           │                                         │
│                           ▼                                         │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  3. DATA FETCH (Client → Server API)                        │    │
│  │     Browser calls: GET /api/chat/transactions               │    │
│  │     Server returns: JSON data                               │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                           │                                         │
│                           ▼                                         │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  4. SQL EXECUTION (Client - sql.js WASM)                    │    │
│  │     - Load data into sql.js SQLite database                 │    │
│  │     - Execute LLM-generated SQL query                       │    │
│  │     - Return results                                        │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                           │                                         │
│                           ▼                                         │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  5. RENDER RESPONSE (Client)                                │    │
│  │     - Display natural language answer                       │    │
│  │     - Show data table if applicable                         │    │
│  │     - Show SQL query (collapsible)                          │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Why sql.js Client-Side?

1. **Security**: No server-side code execution needed
2. **Speed**: Data already in browser, queries are instant
3. **Privacy**: Financial data stays client-side for analysis
4. **LLM Strength**: LLMs are excellent at generating SQL
5. **Lightweight**: sql.js is ~1MB WASM bundle

---

## Schema for sql.js

```sql
-- Loaded into sql.js when chat opens
CREATE TABLE transactions (
    id TEXT PRIMARY KEY,
    date TEXT,              -- ISO date string
    description TEXT,
    display_title TEXT,
    amount_cents INTEGER,   -- Positive = expense, Negative = income
    merchant_name TEXT,
    merchant_logo TEXT,
    category TEXT,          -- Primary tag name
    account_name TEXT,
    account_type TEXT,      -- checking, savings, credit_card
    is_transfer INTEGER     -- 0 or 1
);

CREATE TABLE merchants (
    id TEXT PRIMARY KEY,
    name TEXT,
    logo_url TEXT,
    category TEXT
);

CREATE TABLE categories (
    name TEXT PRIMARY KEY,
    parent TEXT,
    color TEXT
);

CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    name TEXT,
    type TEXT,
    institution TEXT,
    balance_cents INTEGER
);
```

---

## LLM Prompt Design

### System Prompt

```
You are a financial assistant helping users understand their spending.
You have access to their transaction data via SQL queries.

SCHEMA:
{schema_description}

AVAILABLE DATA:
- Transactions from {earliest_date} to {latest_date}
- {transaction_count} total transactions
- Categories: {category_list}
- Merchants: {top_merchants}
- Accounts: {account_list}

When answering questions:
1. First, write a brief natural language response
2. If data is needed, provide a SQL query wrapped in ```sql blocks
3. Keep queries simple and efficient
4. Use appropriate aggregations (SUM, COUNT, AVG, GROUP BY)
5. Format money as dollars (divide cents by 100)
6. Always filter out transfers (is_transfer = 0) for spending questions

IMPORTANT: The SQL runs client-side in SQLite. Use SQLite-compatible syntax.
```

---

## Data API Endpoints

### GET /api/chat/context

Returns context for the LLM prompt:
```json
{
    "schema": "CREATE TABLE transactions...",
    "date_range": {
        "earliest": "2023-01-15",
        "latest": "2024-12-28"
    },
    "transaction_count": 1847,
    "categories": ["Groceries", "Dining", "Transportation"],
    "top_merchants": ["Amazon", "Costco", "Starbucks"],
    "accounts": [
        {"name": "Chase Checking", "type": "checking"},
        {"name": "Amex Gold", "type": "credit_card"}
    ]
}
```

### GET /api/chat/transactions

Returns transactions for sql.js loading:
```json
{
    "transactions": [
        {
            "id": "uuid",
            "date": "2024-12-15",
            "description": "STARBUCKS #1234",
            "display_title": "Starbucks",
            "amount_cents": 567,
            "merchant_name": "Starbucks",
            "category": "Coffee & Tea",
            "account_name": "Chase Checking",
            "is_transfer": 0
        }
    ]
}
```

---

## UI Design

### Chat Interface

```
┌─────────────────────────────────────────────────────────────┐
│  💬 Ask about your finances                            [×]  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ You: How much did I spend on coffee this year?      │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ 🤖 You've spent $847.32 on coffee in 2024 across    │    │
│  │    127 transactions.                                │    │
│  │                                                     │    │
│  │    ┌────────────────────────────────────────────┐   │    │
│  │    │ Merchant          │ Amount    │ Count      │   │    │
│  │    ├───────────────────┼───────────┼────────────┤   │    │
│  │    │ Starbucks         │ $423.15   │ 67         │   │    │
│  │    │ Blue Bottle       │ $312.40   │ 42         │   │    │
│  │    │ Local Coffee      │ $111.77   │ 18         │   │    │
│  │    └────────────────────────────────────────────┘   │    │
│  │                                                     │    │
│  │    [Show SQL ▼]                                     │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  [Type your question...]                          [Send →]  │
└─────────────────────────────────────────────────────────────┘
```

### Entry Points

1. **Pulse page**: "Ask about your finances" button
2. **Floating button**: Always-available chat icon
3. **Command palette**: Keyboard shortcut (Cmd+K)

---

## Security Considerations

### SQL Injection (Client-Side)

Since SQL runs client-side on the user's own data, SQL injection is not a security risk. The user can only query their own data.

### Prompt Injection

Mitigations:
- Clear separation of system prompt and user input
- Input sanitization
- Response validation

### Data Loading

- Only load user's own data (authenticated API)
- Don't persist data in browser storage (memory only)
- Clear sql.js database when chat closes

---

## Implementation Phases

### Phase 1: Basic Chat (MVP)
- Chat UI component
- Server-side LLM calls
- Transaction data API
- sql.js integration
- Basic SQL execution

### Phase 2: Polish
- Conversation history (in-memory)
- Suggested questions
- Result visualization (tables)
- Error handling

### Phase 3: Advanced
- Client-side LLM option (user's API key)
- More complex queries
- Export results
- Save favorite queries

---

## Files to Create

```
internal/handlers/chat.go           # API endpoints
internal/chat/context.go            # Context preparation
internal/chat/prompts.go            # System prompts

static/js/chat.js                   # Chat UI logic
static/js/sqljs-worker.js           # sql.js web worker
static/sql.js/                      # sql.js WASM files

internal/views/chat/                # Chat UI components
```

---

## Example Queries the LLM Should Handle

| Question | SQL Pattern |
|----------|-------------|
| "Total spending this month" | `SUM WHERE date >= month_start` |
| "Biggest expense categories" | `GROUP BY category ORDER BY SUM DESC` |
| "Spending at Amazon" | `WHERE merchant_name LIKE '%Amazon%'` |
| "Transactions over $100" | `WHERE amount_cents > 10000` |
| "Average grocery bill" | `AVG WHERE category = 'Groceries'` |
| "Monthly spending trend" | `GROUP BY strftime('%Y-%m', date)` |
| "Most frequent merchants" | `COUNT GROUP BY merchant_name` |

---

## Todos

- [ ] Create /api/chat/context endpoint for LLM prompt context
- [ ] Create /api/chat/transactions endpoint for sql.js data loading
- [ ] Set up sql.js WASM bundle and initialization
- [ ] Design and test LLM system prompts for SQL generation
- [ ] Build chat interface component
- [ ] Implement client-side SQL execution with result parsing
- [ ] Build result rendering (tables, formatted answers)
- [ ] Integrate chat into Pulse page and navigation
