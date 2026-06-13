# LLM Tool Calls for Conversational Finance Assistant

## Overview

Design a comprehensive set of tool calls for an LLM assistant to help users manage their finances conversationally - including transaction review, journal entries, categorization, and insights.

## Status: Not Started

## Dependencies

- `05-ai-chat.md` - This extends the AI chat feature with action capabilities
- Entity Model Refactor (optional) - Some tools are entity-aware

## Relationship to AI Chat

The [AI Chat feature](05-ai-chat.md) focuses on **querying** financial data using SQL generation. This plan adds **action capabilities** - the ability for the LLM to modify data through structured tool calls.

Together they enable:
- **Read**: "How much did I spend on coffee?" → SQL query (from 05-ai-chat)
- **Write**: "Tag that as a business expense" → Tool call (this plan)

---

## Context

The assistant needs tools to help users with complex financial tasks like:

- Creating manual journal entries (e.g., "I lent my sister $200")
- Reviewing transactions conversationally ("What was this $45 charge?")
- Categorizing and tagging transactions
- Understanding spending patterns and insights

## Tool Categories

### 1. Read-Only Query Tools

These tools let the LLM gather context before taking action.

**`get_transaction`** - Get full details of a single transaction

```
Input: { transaction_id: UUID }
Output: Transaction with entries, tags, merchant/entity, counterparty
```

**`search_transactions`** - Find transactions by various criteria

```
Input: { 
  search?: string,           // Description search
  date_from?: date,
  date_to?: date,
  amount_min?: cents,
  amount_max?: cents,
  account_id?: UUID,
  tag_id?: UUID,
  entity_id?: UUID,          // Post-refactor: merchant/person
  is_uncategorized?: bool,
  is_transfer?: bool,
  limit?: int
}
Output: List of transactions with basic info
```

**`get_accounts`** - List all accounts with balances

```
Input: { type?: "asset"|"liability"|"income"|"expense"|"equity" }
Output: List of accounts with current balances
```

**`get_tags`** - Get available tags/categories

```
Input: {}
Output: Hierarchical tag list (Income > Salary, Expenses > Groceries, etc.)
```

**`get_entities`** - Search entities (post-refactor)

```
Input: { search?: string, type?: "person"|"business"|"government" }
Output: List of entities with logos, relationship to user
```

**`get_spending_summary`** - Spending by category for a period

```
Input: { start_date: date, end_date: date, group_by?: "tag"|"entity"|"account" }
Output: Aggregated spending totals
```

**`get_recurring_patterns`** - Get detected subscriptions/bills

```
Input: {}
Output: List of recurring patterns with next expected dates
```

### 2. Transaction Modification Tools

**`update_transaction`** - Edit transaction metadata

```
Input: {
  transaction_id: UUID,
  date?: date,
  description?: string,
  display_title?: string,    // Human-friendly name
  notes?: string
}
Output: Updated transaction
```

Use case: "That Starbucks charge was actually a birthday gift"

**`categorize_transaction`** - Add/change category tag

```
Input: {
  transaction_id: UUID,
  tag_id: UUID
}
Output: Updated transaction with new tag
```

Use case: "Tag this as Groceries"

**`bulk_categorize`** - Categorize multiple similar transactions

```
Input: {
  transaction_ids: UUID[],
  tag_id: UUID
}
Output: Count of updated transactions
```

Use case: "Tag all my Amazon purchases as Shopping"

**`set_entity`** - Link transaction to an entity (post-refactor)

```
Input: {
  transaction_id: UUID,
  entity_id: UUID,
  entity_role: "entity"|"counterparty"|"facilitator"
}
Output: Updated transaction
```

Use case: "This Zelle payment was to my sister Julia"

### 3. Transfer Matching Tools

**`find_transfer_candidates`** - Find potential matching transactions

```
Input: { transaction_id: UUID }
Output: List of candidate transactions with confidence scores
```

**`link_transfer`** - Match two transactions as a transfer

```
Input: {
  transaction_id_1: UUID,
  transaction_id_2: UUID
}
Output: Both transactions marked as transfer pair
```

Use case: "These two are the same transfer between my accounts"

**`unlink_transfer`** - Undo a transfer match

```
Input: { transaction_id: UUID }
Output: Both transactions unlinked
```

### 4. Journal Entry Tools (Complex)

**`create_journal_entry`** - Create a manual double-entry transaction

```
Input: {
  date: date,
  description: string,
  entries: [
    { account_id: UUID, amount_cents: int64 },
    { account_id: UUID, amount_cents: int64 },
    // ... more entries (must sum to zero)
  ],
  notes?: string,
  tags?: UUID[]
}
Output: Created transaction with entries
```

Use cases:

- "I lent my sister $200 from my checking account"
  - Debit: Loans to Family +$200
  - Credit: Checking -$200
- "Record that I paid off $500 of my credit card"
  - Debit: Credit Card +$500 (reduces liability)
  - Credit: Checking -$500

**`split_transaction`** - Split a single transaction into multiple categories

```
Input: {
  transaction_id: UUID,
  splits: [
    { tag_id: UUID, amount_cents: int64, notes?: string },
    { tag_id: UUID, amount_cents: int64, notes?: string }
  ]
}
Output: Original transaction updated with split entries
```

Use case: "That Target receipt was $50 groceries and $30 household items"

### 5. Entity Management Tools (Post-Refactor)

**`create_entity`** - Create a new person/business

```
Input: {
  type: "person"|"business"|"government"|"trust",
  name: string,
  metadata?: {
    is_household_member?: bool,
    relationship?: string,    // "spouse", "family", "friend"
    email?: string
  }
}
Output: Created entity
```

Use case: "Julia is my sister"

**`create_entity_relationship`** - Define how entities relate

```
Input: {
  entity_a_id: UUID,
  entity_b_id: UUID,
  relationship_type: "spouse"|"partner"|"family"|"employer"
}
Output: Created relationship
```

### 6. Rule/Automation Tools

**`create_rule`** - Create an auto-categorization rule

```
Input: {
  name: string,
  prompt: string,           // LLM instruction
  examples?: string,        // Transaction examples
  tag_id: UUID,
  priority: int
}
Output: Created rule
```

Use case: "Always tag Costco as Groceries"

**`suggest_rule`** - Suggest a rule based on user corrections

```
Input: { transaction_id: UUID, tag_id: UUID }
Output: Suggested rule (user confirms)
```

---

## Design Principles

### 1. Confirmation Before Writes

The LLM should always explain what it's about to do and ask for confirmation before modifying data:

> "I'll create a journal entry:
> - Debit 'Loans to Family': +$200
> - Credit 'Chase Checking': -$200
>
> Does this look right?"

### 2. Contextual Queries First

Before suggesting actions, query for context:

- "Let me look up that transaction..."
- "I see you have 3 similar uncategorized transactions..."

### 3. Entity-Aware (Post-Refactor)

Once entities are implemented, the LLM can understand:

- "Zelle" is a facilitator, not the counterparty
- "Julia" is a person (your sister), not a merchant
- Transfers to household members are different from P2P payments

### 4. Natural Language Mapping

The LLM prompt should map common phrases to tools:

| User Says | Tool | Parameters |
|-----------|------|------------|
| "What's this charge?" | get_transaction | current selection |
| "Tag it as groceries" | categorize_transaction | tag lookup |
| "I paid my sister back" | create_journal_entry | loan repayment |
| "This is a transfer" | find_transfer_candidates | then link_transfer |
| "Show my Amazon spending" | search_transactions + get_spending_summary | entity search |

---

## Implementation Location

New package: `internal/llm/`

```
internal/llm/
  tools.go          # Tool definitions and schemas
  executor.go       # Tool execution logic
  prompts.go        # System prompts
  handler.go        # HTTP handler for chat endpoint
```

Integration with existing handlers:

- Most tools wrap existing store methods
- Add permission checks per-ledger
- Audit logging for all writes

---

## Conversation Flow Example

**User**: "I lent my sister $200 last week"

**LLM** (internal):
1. `get_accounts` - Find checking account and create/find "Loans to Family"
2. `get_entities` or `create_entity` - Find or create "Sister" entity

**LLM** (to user):

> "I'll record a $200 loan to your sister from your Chase Checking account. This will:
> - Create a 'Loans to Family' asset account if it doesn't exist
> - Record the transfer on [date]
>
> Should I proceed?"

**User**: "Yes"

**LLM** (internal):
1. `create_journal_entry` with appropriate entries
2. `set_entity` to link to sister entity

**LLM** (to user):

> "Done! I've recorded the $200 loan. Your 'Loans to Family' balance is now $200."

---

## Todos

- [ ] Define JSON schemas for each tool (OpenAI function calling format)
- [ ] Implement read-only query tools (get_transaction, search_transactions, get_accounts, get_tags, get_spending_summary)
- [ ] Implement transaction modification tools (update_transaction, categorize_transaction, bulk_categorize)
- [ ] Implement complex journal entry tools (create_journal_entry, split_transaction)
- [ ] Implement transfer matching tools (find_transfer_candidates, link_transfer, unlink_transfer)
- [ ] Implement entity tools after refactor (create_entity, set_entity, create_entity_relationship)
- [ ] Create system prompt with tool descriptions, confirmation patterns, and natural language mappings
- [ ] Create chat API endpoint that processes messages and executes tool calls
