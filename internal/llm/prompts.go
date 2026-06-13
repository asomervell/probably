package llm

import (
	"fmt"
	"strings"
	"time"
)

// ConversationContext holds context about the current conversation
type ConversationContext struct {
	LedgerID       string
	LedgerName     string
	Currency       string
	AccountCount   int
	TransactionCount int
	RecentActivity string
	
	// Optional: current transaction being discussed
	CurrentTransactionID string
	CurrentTransactionDescription string
}

// Summary returns a summary of the context for the system prompt
func (c *ConversationContext) Summary() string {
	var parts []string
	
	if c.LedgerName != "" {
		parts = append(parts, fmt.Sprintf("- Ledger: %s", c.LedgerName))
	}
	if c.AccountCount > 0 {
		parts = append(parts, fmt.Sprintf("- %d accounts connected", c.AccountCount))
	}
	if c.TransactionCount > 0 {
		parts = append(parts, fmt.Sprintf("- %d total transactions", c.TransactionCount))
	}
	if c.RecentActivity != "" {
		parts = append(parts, fmt.Sprintf("- Recent activity: %s", c.RecentActivity))
	}
	if c.CurrentTransactionID != "" {
		parts = append(parts, fmt.Sprintf("- Currently discussing transaction: %s", c.CurrentTransactionDescription))
	}
	
	if len(parts) == 0 {
		return "No specific context available."
	}
	
	return strings.Join(parts, "\n")
}

// SystemPromptWithTools generates the system prompt for V2 chat with tool calling
func SystemPromptWithTools(context *ConversationContext) string {
	return fmt.Sprintf(`You are a helpful financial assistant for a personal finance application called Probably.

You help users understand their finances through natural conversation.

## Guidelines

- Be conversational and helpful
- Use exact dollar amounts when discussing money (e.g., $127.43, not "about $130")
- If you need more information, ask
- Keep responses concise but complete
- Use markdown for formatting: **bold** for emphasis, bullet lists, etc.

## Tool Usage - CRITICAL

**WORKFLOW: Call tool → Get result → ANSWER immediately**

1. **One tool call is usually enough.** Most questions need just get_spending_summary or search_transactions.
2. **Maximum 3 tool calls total.** If you've called 3 tools, you MUST answer with what you have.
3. **NEVER repeat a tool call.** If you already called get_spending_summary with group_by="tag", do NOT call it again with the same parameters.
4. **Answer after EVERY tool result.** Don't chain tool calls looking for "better" data - use what you got.

**Examples:**
- "How much did I spend on food?" → Call get_spending_summary once → Answer
- "What's my biggest expense?" → Call get_spending_summary once → Answer  
- "Show me Amazon purchases" → Call search_transactions once → Answer

**Anti-patterns to AVOID:**
- ❌ Calling get_spending_summary multiple times with different group_by values
- ❌ Calling search_transactions after get_spending_summary gave you the answer
- ❌ "Let me check one more thing" - NO, answer with what you have

## Current Context

%s

- Currency: %s
- Today: %s`,
		context.Summary(),
		context.Currency,
		time.Now().Format("January 2, 2006"),
	)
}
