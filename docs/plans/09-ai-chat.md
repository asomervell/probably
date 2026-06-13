---
name: AI Chat with sql.js
overview: Conversational AI interface where users can ask questions about their finances in natural language. The LLM generates SQL queries that run client-side via sql.js (SQLite in WebAssembly), keeping data processing in the browser.
todos: []
---

# AI Chat Feature Plan

## What We're Building

A conversational interface where users can **talk to their money**:

- "How much did I spend on coffee last month?"
- "What's my biggest expense category this year?"
- "Am I spending more on dining out than last quarter?"
- "Show me all transactions over $500"

The magic: LLM generates SQL, which runs **client-side in the browser** via sql.js (SQLite compiled to WebAssembly).

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         BROWSER                                     │
│                                                                     │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐      │
│  │  User    │───▶│  Send    │───▶│  Server  │───▶│  LLM     │      │
│  │  asks    │    │  to API  │    │  (Go)    │    │  (Grok)  │      │
│  │  question│    │          │    │          │    │          │      │
│  └──────────┘    └──────────┘    └──────────┘    └────┬─────┘      │
│                                                       │             │
│                                         ┌─────────────┘             │
│                                         ▼                           │
│  ┌──────────┐    ┌──────────┐    ┌──────────────┐                   │
│  │  Display │◀───│  Execute │◀───│  Generated   │                   │
│  │  Results │    │  in      │    │  SQL + API   │                   │
│  │          │    │  sql.js  │    │  calls       │                   │
│  └──────────┘    └──────────┘    └──────────────┘                   │
│                       ▲                                             │
│                       │                                             │
│              ┌────────┴────────┐                                    │
│              │  sql.js         │                                    │
│              │  (SQLite WASM)  │                                    │
│              │  ~1MB           │                                    │
│              └─────────────────┘                                    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## User Flow

### 1. User Opens Chat

```
┌─────────────────────────────────────────────────────────────┐
│  💬 Ask about your finances                                 │
│                                                             │
│  Try:                                                       │
│  • "How much did I spend on groceries this month?"          │
│  • "What are my top 5 merchants?"                           │
│  • "Show transactions from Amazon over $50"                 │
│                                                             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Ask anything...                                    [➤] ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

### 2. User Asks Question

```
User: "How much did I spend on coffee shops in December?"
```

### 3. LLM Generates Response

The LLM returns a structured response:

```json
{
  "thought": "User wants coffee shop spending for December 2024. I need to query transactions with coffee-related merchants or tags.",
  "data_needed": ["transactions", "merchants", "tags"],
  "api_calls": [
    {
      "endpoint": "/api/chat/data",
      "params": {
        "tables": ["transactions", "merchants", "tags"],
        "date_start": "2024-12-01",
        "date_end": "2024-12-31"
      }
    }
  ],
  "sql": "SELECT SUM(ABS(t.amount_cents)) / 100.0 as total_dollars, COUNT(*) as transaction_count FROM transactions t LEFT JOIN merchants m ON t.merchant_id = m.id LEFT JOIN transaction_tags tt ON t.id = tt.transaction_id LEFT JOIN tags tg ON tt.tag_id = tg.id WHERE t.date >= '2024-12-01' AND t.date <= '2024-12-31' AND (LOWER(m.display_name) LIKE '%coffee%' OR LOWER(m.category) LIKE '%coffee%' OR LOWER(tg.name) LIKE '%coffee%')",
  "answer_template": "You spent **${total_dollars}** on coffee shops in December across ${transaction_count} transactions."
}
```

### 4. Browser Executes

1. Fetch data via API call
2. Load into sql.js
3. Execute SQL query
4. Format answer with results

### 5. Display Answer

```
┌─────────────────────────────────────────────────────────────┐
│  You: How much did I spend on coffee shops in December?     │
│                                                             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ You spent **$127.45** on coffee shops in December       ││
│  │ across 23 transactions.                                 ││
│  │                                                         ││
│  │ Top merchants:                                          ││
│  │ • Starbucks: $67.20 (12 visits)                         ││
│  │ • Blue Bottle: $42.50 (7 visits)                        ││
│  │ • Local Coffee: $17.75 (4 visits)                       ││
│  └─────────────────────────────────────────────────────────┘│
│                                                             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Ask anything...                                    [➤] ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

---

## Technical Components

### 1. Data Export API

Server endpoint that exports ledger data for sql.js:

```go
// POST /api/chat/data
type ChatDataRequest struct {
    Tables    []string `json:"tables"`     // ["transactions", "merchants", "tags"]
    DateStart string   `json:"date_start"` // Optional filter
    DateEnd   string   `json:"date_end"`   // Optional filter
}

type ChatDataResponse struct {
    Schema map[string][]Column `json:"schema"`
    Data   map[string][]Row    `json:"data"`
}
```

**Security**: Only export data for the authenticated user's ledger.

### 2. sql.js Setup (Browser)

```javascript
// Initialize sql.js once on page load
const SQL = await initSqlJs({
  locateFile: file => `/static/sql-wasm.wasm`
});

// Create in-memory database
const db = new SQL.Database();

// Load data from API response
function loadData(response) {
  // Create tables
  for (const [table, columns] of Object.entries(response.schema)) {
    const columnDefs = columns.map(c => `${c.name} ${c.type}`).join(', ');
    db.run(`CREATE TABLE ${table} (${columnDefs})`);
  }
  
  // Insert data
  for (const [table, rows] of Object.entries(response.data)) {
    for (const row of rows) {
      // ... insert row
    }
  }
}
```

### 3. LLM Prompt Engineering

System prompt for the LLM:

```
You are a financial assistant that helps users understand their spending.
You have access to the user's financial data via SQL queries.

Available tables:
- transactions (id, date, description, amount_cents, merchant_id, is_transfer)
- merchants (id, display_name, category, logo_url)
- tags (id, name, color)
- transaction_tags (transaction_id, tag_id)
- accounts (id, name, type, balance)

Rules:
1. Generate valid SQLite SQL
2. Always use amount_cents (divide by 100 for dollars)
3. Exclude transfers (is_transfer = false) for spending queries
4. Use ABS() for expense amounts (they're positive in our system)
5. Be concise in answers
6. If unsure, ask clarifying questions

Respond in JSON format:
{
  "thought": "your reasoning",
  "api_calls": [...],
  "sql": "SELECT ...",
  "answer_template": "The answer is ${variable}"
}
```

### 4. Query Execution Flow

```javascript
async function askQuestion(question) {
  // 1. Send question to server
  const response = await fetch('/api/chat/ask', {
    method: 'POST',
    body: JSON.stringify({ question })
  });
  
  const { api_calls, sql, answer_template } = await response.json();
  
  // 2. Fetch required data
  for (const call of api_calls) {
    const data = await fetch(call.endpoint, {
      method: 'POST',
      body: JSON.stringify(call.params)
    });
    loadData(await data.json());
  }
  
  // 3. Execute SQL in browser
  const results = db.exec(sql);
  
  // 4. Format answer
  const answer = formatAnswer(answer_template, results);
  
  // 5. Display
  displayMessage(answer);
}
```

---

## Schema for sql.js

Simplified schema optimized for queries:

```sql
-- Transactions (denormalized for easy querying)
CREATE TABLE transactions (
  id TEXT PRIMARY KEY,
  date TEXT,
  description TEXT,
  display_title TEXT,
  amount_cents INTEGER,
  merchant_id TEXT,
  merchant_name TEXT,
  merchant_category TEXT,
  is_transfer INTEGER,
  account_id TEXT,
  account_name TEXT
);

-- Tags (for category queries)
CREATE TABLE tags (
  id TEXT PRIMARY KEY,
  name TEXT,
  color TEXT,
  parent_id TEXT
);

-- Transaction-Tag mapping
CREATE TABLE transaction_tags (
  transaction_id TEXT,
  tag_id TEXT
);

-- Accounts (for balance queries)
CREATE TABLE accounts (
  id TEXT PRIMARY KEY,
  name TEXT,
  type TEXT,
  balance_cents INTEGER
);
```

---

## Example Queries the LLM Should Generate

### "How much did I spend this month?"

```sql
SELECT SUM(amount_cents) / 100.0 as total
FROM transactions
WHERE date >= date('now', 'start of month')
  AND is_transfer = 0
  AND amount_cents > 0;
```

### "What are my top 5 expense categories?"

```sql
SELECT t.name as category, SUM(tr.amount_cents) / 100.0 as total
FROM transactions tr
JOIN transaction_tags tt ON tr.id = tt.transaction_id
JOIN tags t ON tt.tag_id = t.id
WHERE tr.is_transfer = 0 AND tr.amount_cents > 0
GROUP BY t.name
ORDER BY total DESC
LIMIT 5;
```

### "Show me transactions over $100 from Amazon"

```sql
SELECT date, description, amount_cents / 100.0 as amount
FROM transactions
WHERE merchant_name LIKE '%Amazon%'
  AND ABS(amount_cents) > 10000
ORDER BY date DESC;
```

### "Am I spending more on dining this month vs last?"

```sql
SELECT 
  CASE WHEN date >= date('now', 'start of month') THEN 'this_month' ELSE 'last_month' END as period,
  SUM(amount_cents) / 100.0 as total
FROM transactions tr
JOIN transaction_tags tt ON tr.id = tt.transaction_id
JOIN tags t ON tt.tag_id = t.id
WHERE t.name LIKE '%Dining%' OR t.name LIKE '%Restaurant%'
  AND date >= date('now', 'start of month', '-1 month')
GROUP BY period;
```

---

## Security Considerations

### Data Exposure

- **Only export user's own ledger data**
- **Filter by date range** to limit data transfer
- **No sensitive fields** (no access tokens, no password hashes)

### SQL Injection

- SQL runs in **isolated browser sandbox** (sql.js)
- Cannot affect server database
- Worst case: user sees garbled results

### Rate Limiting

- Limit chat requests per minute
- Limit data export size
- Cache repeated data fetches

---

## UI Components

### Chat Container

```
┌─────────────────────────────────────────────────────────────┐
│  FINANCIAL ASSISTANT                              [Clear]   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  [Message history scrollable area]                          │
│                                                             │
│  User: How much did I spend on groceries?                   │
│                                                             │
│  Assistant: You spent $542.30 on groceries this month.      │
│  That's 12% more than last month ($484.20).                 │
│                                                             │
│  User: Break it down by store                               │
│                                                             │
│  Assistant:                                                 │
│  • Whole Foods: $234.50 (43%)                               │
│  • Trader Joe's: $187.20 (35%)                              │
│  • Safeway: $120.60 (22%)                                   │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Ask anything about your finances...               [➤]  ││
│  └─────────────────────────────────────────────────────────┘│
│                                                             │
│  💡 Try: "What's my biggest expense?" • "Coffee spending"   │
└─────────────────────────────────────────────────────────────┘
```

### Loading State

```
┌─────────────────────────────────────────────────────────────┐
│  User: How much on coffee?                                  │
│                                                             │
│  Assistant:                                                 │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  ⟳ Thinking...                                          ││
│  │  [████████░░░░░░░░░░░░] Fetching data                   ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

### Error State

```
┌─────────────────────────────────────────────────────────────┐
│  Assistant:                                                 │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  ❌ I couldn't understand that query.                   ││
│  │  Try asking in a different way, like:                   ││
│  │  "How much did I spend on [category] in [time period]?" ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

---

## Implementation Phases

### Phase 1: Basic Infrastructure
- Add sql.js to frontend (copy WASM file to static)
- Create data export API endpoint
- Basic chat UI shell

### Phase 2: LLM Integration
- Design prompt template with schema
- Create `/api/chat/ask` endpoint
- Parse LLM JSON response

### Phase 3: Query Execution
- JavaScript to load data into sql.js
- Execute generated SQL
- Format results into answer

### Phase 4: Polish
- Conversation history
- Suggested questions
- Error handling
- Loading states

### Phase 5: Advanced Features (Future)
- Follow-up questions (context)
- Charts/visualizations in responses
- Export results
- Save favorite queries

---

## Files to Create

```
# Backend
internal/handlers/chat.go           # /api/chat/ask, /api/chat/data
internal/chat/prompt.go             # LLM prompt templates
internal/chat/schema.go             # Schema definitions for export

# Frontend
static/js/chat.js                   # Chat UI and sql.js integration
static/sql-wasm.wasm                # sql.js WASM binary (~1MB)
static/sql-wasm.js                  # sql.js JavaScript

# Views
internal/views/chat/                
