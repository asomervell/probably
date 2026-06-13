// Package llm provides tool definitions and execution for LLM-powered chat assistants.
package llm

import (
	"encoding/json"
)

// Tool represents an OpenAI-compatible function/tool definition
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function describes a callable function for the LLM
type Function struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall represents a tool call from the LLM
type ToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function FunctionCall    `json:"function"`
}

// FunctionCall contains the function name and arguments
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolResult represents the result of executing a tool
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"` // JSON string of result
	Error      string `json:"error,omitempty"`
}

// AllTools returns all available tool definitions
func AllTools() []Tool {
	return []Tool{
		// Read-only query tools
		getTransactionTool(),
		searchTransactionsTool(),
		getAccountsTool(),
		getTagsTool(),
		getSpendingSummaryTool(),
		getRecurringPatternsTool(),
		getEntitiesTool(),

		// Semantic similarity tools (embedding-based)
		findSimilarTransactionsTool(),
		findSimilarEntitiesTool(),

		// Transaction modification tools
		updateTransactionTool(),
		categorizeTransactionTool(),
		bulkCategorizeTool(),

		// Transfer matching tools
		findTransferCandidatesTool(),
		linkTransferTool(),
		unlinkTransferTool(),

		// Journal entry tools
		createJournalEntryTool(),
		splitTransactionTool(),

		// Entity tools
		createEntityTool(),
		setTransactionEntityTool(),
		createEntityRelationshipTool(),

		// Rule tools
		createRuleTool(),
		suggestRuleTool(),
	}
}

// =============================================================================
// Read-Only Query Tools
// =============================================================================

func getTransactionTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "get_transaction",
			Description: "Get full details of a single transaction including entries, tags, and entity information. Use this when the user asks about a specific transaction.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"transaction_id": {
						"type": "string",
						"format": "uuid",
						"description": "The UUID of the transaction to retrieve"
					}
				},
				"required": ["transaction_id"]
			}`),
		},
	}
}

func searchTransactionsTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "search_transactions",
			Description: "Search for transactions by various criteria like description, date range, amount range, account, tag, or entity. Use this to find transactions matching user's description.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"search": {
						"type": "string",
						"description": "Text to search in transaction descriptions"
					},
					"date_from": {
						"type": "string",
						"format": "date",
						"description": "Start date (YYYY-MM-DD) to filter transactions"
					},
					"date_to": {
						"type": "string",
						"format": "date",
						"description": "End date (YYYY-MM-DD) to filter transactions"
					},
					"amount_min_cents": {
						"type": "integer",
						"description": "Minimum amount in cents (use negative for credits/income)"
					},
					"amount_max_cents": {
						"type": "integer",
						"description": "Maximum amount in cents"
					},
					"account_id": {
						"type": "string",
						"format": "uuid",
						"description": "Filter by specific account UUID"
					},
					"tag_id": {
						"type": "string",
						"format": "uuid",
						"description": "Filter by specific tag/category UUID"
					},
					"entity_id": {
						"type": "string",
						"format": "uuid",
						"description": "Filter by specific entity UUID"
					},
					"is_uncategorized": {
						"type": "boolean",
						"description": "If true, only return transactions without tags"
					},
					"is_transfer": {
						"type": "boolean",
						"description": "If true/false, filter by transfer status"
					},
					"limit": {
						"type": "integer",
						"description": "Maximum number of results to return (default 20, max 100)"
					}
				},
				"required": []
			}`),
		},
	}
}

func getAccountsTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "get_accounts",
			Description: "Get all accounts with their current balances. Optionally filter by account type. Use this to understand what accounts the user has.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"type": {
						"type": "string",
						"enum": ["asset", "liability", "income", "expense", "equity"],
						"description": "Filter by account type"
					}
				},
				"required": []
			}`),
		},
	}
}

func getTagsTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "get_tags",
			Description: "Get all available tags/categories organized hierarchically. Use this to see what categories exist before categorizing transactions.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
	}
}

func getSpendingSummaryTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "get_spending_summary",
			Description: "Get aggregated spending totals instantly. Uses fast SQL aggregation to return category/entity/account totals. This is the best tool for spending questions - call it FIRST before searching individual transactions.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"start_date": {
						"type": "string",
						"format": "date",
						"description": "Start date (YYYY-MM-DD) for the summary period"
					},
					"end_date": {
						"type": "string",
						"format": "date",
						"description": "End date (YYYY-MM-DD) for the summary period"
					},
					"group_by": {
						"type": "string",
						"enum": ["tag", "entity", "account"],
						"description": "How to group the spending totals (default: tag)"
					}
				},
				"required": ["start_date", "end_date"]
			}`),
		},
	}
}

func getRecurringPatternsTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "get_recurring_patterns",
			Description: "Get detected recurring transactions like subscriptions and bills with their expected next dates and amounts.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
	}
}

// =============================================================================
// Transaction Modification Tools
// =============================================================================

func updateTransactionTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "update_transaction",
			Description: "Update a transaction's metadata like date, description, display title, or notes. Does NOT change the amounts or accounts. Always confirm with the user before calling.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"transaction_id": {
						"type": "string",
						"format": "uuid",
						"description": "The UUID of the transaction to update"
					},
					"date": {
						"type": "string",
						"format": "date",
						"description": "New date for the transaction (YYYY-MM-DD)"
					},
					"description": {
						"type": "string",
						"description": "New raw description"
					},
					"display_title": {
						"type": "string",
						"description": "Human-friendly display name"
					},
					"notes": {
						"type": "string",
						"description": "User notes about the transaction"
					}
				},
				"required": ["transaction_id"]
			}`),
		},
	}
}

func categorizeTransactionTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "categorize_transaction",
			Description: "Add or change the category tag on a transaction. This also updates the ledger entry to the appropriate expense/income account. Always confirm with the user before calling.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"transaction_id": {
						"type": "string",
						"format": "uuid",
						"description": "The UUID of the transaction to categorize"
					},
					"tag_id": {
						"type": "string",
						"format": "uuid",
						"description": "The UUID of the tag to apply"
					}
				},
				"required": ["transaction_id", "tag_id"]
			}`),
		},
	}
}

func bulkCategorizeTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "bulk_categorize",
			Description: "Apply the same category tag to multiple transactions at once. Use this when the user wants to categorize several similar transactions. Always confirm with the user before calling.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"transaction_ids": {
						"type": "array",
						"items": {
							"type": "string",
							"format": "uuid"
						},
						"description": "Array of transaction UUIDs to categorize"
					},
					"tag_id": {
						"type": "string",
						"format": "uuid",
						"description": "The UUID of the tag to apply to all transactions"
					}
				},
				"required": ["transaction_ids", "tag_id"]
			}`),
		},
	}
}

// =============================================================================
// Transfer Matching Tools
// =============================================================================

func findTransferCandidatesTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "find_transfer_candidates",
			Description: "Find potential matching transactions that could be the other side of a transfer. Returns candidates with confidence scores based on amount, date proximity, and account types.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"transaction_id": {
						"type": "string",
						"format": "uuid",
						"description": "The UUID of the transaction to find transfer matches for"
					}
				},
				"required": ["transaction_id"]
			}`),
		},
	}
}

func linkTransferTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "link_transfer",
			Description: "Link two transactions together as a transfer pair (e.g., money moving from checking to savings). This marks both as transfers and updates their entries appropriately. Always confirm with the user before calling.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"transaction_id_1": {
						"type": "string",
						"format": "uuid",
						"description": "The UUID of the first transaction"
					},
					"transaction_id_2": {
						"type": "string",
						"format": "uuid",
						"description": "The UUID of the second transaction"
					}
				},
				"required": ["transaction_id_1", "transaction_id_2"]
			}`),
		},
	}
}

func unlinkTransferTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "unlink_transfer",
			Description: "Unlink a transfer pair, converting both transactions back to regular transactions. Use this if a transfer was matched incorrectly. Always confirm with the user before calling.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"transaction_id": {
						"type": "string",
						"format": "uuid",
						"description": "The UUID of either transaction in the transfer pair"
					}
				},
				"required": ["transaction_id"]
			}`),
		},
	}
}

// =============================================================================
// Journal Entry Tools
// =============================================================================

func createJournalEntryTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "create_journal_entry",
			Description: `Create a manual double-entry transaction. Use this for recording cash transactions, loans, adjustments, or any transaction not imported from a bank.

IMPORTANT: Entries must sum to zero (double-entry bookkeeping). 
- Positive amounts = debit (increase assets/expenses, decrease liabilities/income)
- Negative amounts = credit (decrease assets/expenses, increase liabilities/income)

Examples:
- Loan to friend: Debit "Loans Receivable" +$200, Credit "Checking" -$200
- Pay credit card: Debit "Credit Card" +$500, Credit "Checking" -$500
- Cash expense: Debit "Groceries" +$50, Credit "Cash" -$50

Always explain the journal entry to the user and confirm before calling.`,
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"date": {
						"type": "string",
						"format": "date",
						"description": "Date of the transaction (YYYY-MM-DD)"
					},
					"description": {
						"type": "string",
						"description": "Description of the transaction"
					},
					"entries": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"account_id": {
									"type": "string",
									"format": "uuid",
									"description": "The account UUID"
								},
								"amount_cents": {
									"type": "integer",
									"description": "Amount in cents (positive=debit, negative=credit)"
								}
							},
							"required": ["account_id", "amount_cents"]
						},
						"minItems": 2,
						"description": "Array of entries that must sum to zero"
					},
					"notes": {
						"type": "string",
						"description": "Optional notes about the transaction"
					},
					"tag_ids": {
						"type": "array",
						"items": {
							"type": "string",
							"format": "uuid"
						},
						"description": "Optional tag UUIDs to apply"
					}
				},
				"required": ["date", "description", "entries"]
			}`),
		},
	}
}

func splitTransactionTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "split_transaction",
			Description: `Split an existing transaction into multiple categories. Use this when a single receipt covers multiple expense types (e.g., groceries + household items from Target).

The split amounts must sum to the original transaction amount. Always confirm with the user before calling.`,
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"transaction_id": {
						"type": "string",
						"format": "uuid",
						"description": "The UUID of the transaction to split"
					},
					"splits": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"tag_id": {
									"type": "string",
									"format": "uuid",
									"description": "The category tag UUID for this portion"
								},
								"amount_cents": {
									"type": "integer",
									"description": "Amount in cents for this split"
								},
								"notes": {
									"type": "string",
									"description": "Optional notes for this split"
								}
							},
							"required": ["tag_id", "amount_cents"]
						},
						"minItems": 2,
						"description": "Array of splits that must sum to original amount"
					}
				},
				"required": ["transaction_id", "splits"]
			}`),
		},
	}
}

// =============================================================================
// Rule/Automation Tools
// =============================================================================

func createRuleTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "create_rule",
			Description: "Create an automatic categorization rule. When new transactions match the rule's criteria, they'll be automatically tagged. Always confirm with the user before calling.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Human-readable name for the rule"
					},
					"match_pattern": {
						"type": "string",
						"description": "Text pattern to match in transaction descriptions"
					},
					"is_regex": {
						"type": "boolean",
						"description": "If true, match_pattern is treated as a regex"
					},
					"tag_id": {
						"type": "string",
						"format": "uuid",
						"description": "The tag to apply when rule matches"
					},
					"priority": {
						"type": "integer",
						"description": "Rule priority (higher = checked first, default 0)"
					}
				},
				"required": ["name", "match_pattern", "tag_id"]
			}`),
		},
	}
}

func suggestRuleTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "suggest_rule",
			Description: "Suggest a categorization rule based on a transaction the user just categorized. Returns a suggested rule that the user can confirm or modify.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"transaction_id": {
						"type": "string",
						"format": "uuid",
						"description": "The transaction that was just categorized"
					},
					"tag_id": {
						"type": "string",
						"format": "uuid",
						"description": "The tag that was applied"
					}
				},
				"required": ["transaction_id", "tag_id"]
			}`),
		},
	}
}

// =============================================================================
// Entity Tools
// =============================================================================

func getEntitiesTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "get_entities",
			Description: "Search for entities (people, businesses, government agencies, etc.) by name or type. Use this to find existing entities before creating new ones or to understand who the user transacts with.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"search": {
						"type": "string",
						"description": "Text to search in entity names"
					},
					"type": {
						"type": "string",
						"enum": ["person", "business", "trust", "partnership", "government"],
						"description": "Filter by entity type"
					},
					"limit": {
						"type": "integer",
						"description": "Maximum number of results (default 20, max 100)"
					}
				},
				"required": []
			}`),
		},
	}
}

// =============================================================================
// Semantic Similarity Tools (Embedding-based)
// =============================================================================

func findSimilarTransactionsTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "find_similar_transactions",
			Description: `Find transactions that are semantically similar to a reference transaction or search query. Uses AI embeddings for intelligent matching.

Use this when:
- User asks "find transactions like this one" or "similar to X"
- User asks "what else have I spent on streaming?" (find similar to Netflix)
- User wants to find related purchases or recurring patterns
- User asks "show me other [category] transactions"

Returns transactions ranked by similarity score (0-100%).`,
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"transaction_id": {
						"type": "string",
						"format": "uuid",
						"description": "Find transactions similar to this specific transaction"
					},
					"query": {
						"type": "string",
						"description": "Natural language description to find similar transactions (e.g., 'streaming subscriptions', 'coffee shops')"
					},
					"limit": {
						"type": "integer",
						"description": "Maximum number of results (default 10, max 50)"
					},
					"min_similarity": {
						"type": "number",
						"description": "Minimum similarity score 0-1 (default 0.7 = 70%)"
					}
				},
				"required": []
			}`),
		},
	}
}

func findSimilarEntitiesTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "find_similar_entities",
			Description: `Find entities (businesses, people) that are semantically similar to a reference entity or search query. Uses AI embeddings for intelligent matching.

Use this when:
- User asks "what companies like Netflix do I pay?" 
- User asks "find similar merchants to X"
- User wants to discover related businesses or services
- User asks about alternatives or competitors they might use

Returns entities ranked by similarity score (0-100%).`,
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"entity_id": {
						"type": "string",
						"format": "uuid",
						"description": "Find entities similar to this specific entity"
					},
					"entity_name": {
						"type": "string",
						"description": "Find entities similar to one with this name"
					},
					"query": {
						"type": "string",
						"description": "Natural language description to find similar entities (e.g., 'streaming services', 'fast food restaurants')"
					},
					"limit": {
						"type": "integer",
						"description": "Maximum number of results (default 10, max 50)"
					},
					"min_similarity": {
						"type": "number",
						"description": "Minimum similarity score 0-1 (default 0.7 = 70%)"
					}
				},
				"required": []
			}`),
		},
	}
}

func createEntityTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "create_entity",
			Description: `Create a new entity (person, business, government agency, etc.). Use this when the user mentions someone new they transact with.

Examples:
- "Julia is my sister" → Create person entity with relationship
- "I shop at Trader Joe's" → Create business entity
- "I paid the DMV" → Create government entity

Always confirm with the user before creating.`,
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"type": {
						"type": "string",
						"enum": ["person", "business", "trust", "partnership", "government"],
						"description": "The type of entity"
					},
					"name": {
						"type": "string",
						"description": "The name of the entity"
					},
					"subtype": {
						"type": "string",
						"description": "Subtype for more specific classification (e.g., 'financial_institution' for banks, 'retailer' for stores)"
					},
					"website": {
						"type": "string",
						"description": "Website URL if known"
					},
					"description": {
						"type": "string",
						"description": "Brief description of who this entity is"
					}
				},
				"required": ["type", "name"]
			}`),
		},
	}
}

func setTransactionEntityTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "set_transaction_entity",
			Description: `Link a transaction to an entity. Transactions can have three types of entity relationships:

1. entity_id - The business/person you transacted with (e.g., Starbucks for a coffee purchase)
2. counterparty_entity_id - For transfers, the person on the other side (e.g., your sister for a Venmo payment)
3. intermediary_entity_id - The service facilitating the transfer (e.g., Venmo, Zelle, ACH)

Always confirm with the user before setting.`,
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"transaction_id": {
						"type": "string",
						"format": "uuid",
						"description": "The UUID of the transaction"
					},
					"entity_id": {
						"type": "string",
						"format": "uuid",
						"description": "The primary entity (business) for this transaction"
					},
					"counterparty_entity_id": {
						"type": "string",
						"format": "uuid",
						"description": "For transfers: the person/org receiving or sending funds"
					},
					"intermediary_entity_id": {
						"type": "string",
						"format": "uuid",
						"description": "The financial service facilitating the transaction (Venmo, Zelle, etc.)"
					}
				},
				"required": ["transaction_id"]
			}`),
		},
	}
}

func createEntityRelationshipTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "create_entity_relationship",
			Description: `Define a relationship between two entities. Use this when the user tells you about relationships (e.g., "Julia is my sister", "I work at Acme Corp").

Relationship types:
- spouse: Married partner
- partner: Domestic/unmarried partner  
- family: Family member (parent, sibling, child, etc.)
- trustee: Manages a trust
- beneficiary: Benefits from a trust
- employer: Employment relationship
- self: Same person in different contexts

Always confirm with the user before creating.`,
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"entity_a_id": {
						"type": "string",
						"format": "uuid",
						"description": "The first entity in the relationship (usually the user's entity)"
					},
					"entity_b_id": {
						"type": "string",
						"format": "uuid",
						"description": "The second entity in the relationship"
					},
					"relationship_type": {
						"type": "string",
						"enum": ["spouse", "partner", "family", "trustee", "beneficiary", "employer", "self"],
						"description": "The type of relationship"
					}
				},
				"required": ["entity_a_id", "entity_b_id", "relationship_type"]
			}`),
		},
	}
}

// =============================================================================
// Tool Name Constants
// =============================================================================

const (
	ToolGetTransaction            = "get_transaction"
	ToolSearchTransactions        = "search_transactions"
	ToolGetAccounts               = "get_accounts"
	ToolGetTags                   = "get_tags"
	ToolGetSpendingSummary        = "get_spending_summary"
	ToolGetRecurringPatterns      = "get_recurring_patterns"
	ToolGetEntities               = "get_entities"
	ToolFindSimilarTransactions   = "find_similar_transactions"
	ToolFindSimilarEntities       = "find_similar_entities"
	ToolUpdateTransaction         = "update_transaction"
	ToolCategorizeTransaction     = "categorize_transaction"
	ToolBulkCategorize            = "bulk_categorize"
	ToolFindTransferCandidates    = "find_transfer_candidates"
	ToolLinkTransfer              = "link_transfer"
	ToolUnlinkTransfer            = "unlink_transfer"
	ToolCreateJournalEntry        = "create_journal_entry"
	ToolSplitTransaction          = "split_transaction"
	ToolCreateEntity              = "create_entity"
	ToolSetTransactionEntity      = "set_transaction_entity"
	ToolCreateEntityRelationship  = "create_entity_relationship"
	ToolCreateRule                = "create_rule"
	ToolSuggestRule               = "suggest_rule"
)

