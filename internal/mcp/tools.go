package mcp

// ToolRegistry manages MCP tool definitions
type ToolRegistry struct {
	tools map[string]*ToolDefinition
}

// ToolDefinition represents an MCP tool with OpenAI Apps SDK metadata
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Meta        map[string]interface{} `json:"_meta,omitempty"`
}

// NewToolRegistry creates a new tool registry with all available tools
func NewToolRegistry() (*ToolRegistry, error) {
	registry := &ToolRegistry{
		tools: make(map[string]*ToolDefinition),
	}

	// Register all tools
	tools := []*ToolDefinition{
		getSpendingSummaryTool(),
		getAccountBalancesTool(),
		askQuestionTool(),
		getSpendingTrendsTool(),
		getRecurringPatternsTool(),
		searchTransactionsTool(),
		getFinancialOverviewTool(),
	}

	for _, tool := range tools {
		registry.tools[tool.Name] = tool
	}

	return registry, nil
}

// GetTool returns a tool definition by name
func (r *ToolRegistry) GetTool(name string) *ToolDefinition {
	return r.tools[name]
}

// HasTool checks if a tool exists
func (r *ToolRegistry) HasTool(name string) bool {
	_, exists := r.tools[name]
	return exists
}

// GetAllTools returns all tool definitions
func (r *ToolRegistry) GetAllTools() []*ToolDefinition {
	tools := make([]*ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Tool definitions

func getSpendingSummaryTool() *ToolDefinition {
	return &ToolDefinition{
		Name:        "get_spending_summary",
		Description: "Get a summary of spending by category and time period. Returns total spending, breakdown by category, and period information. Use this when the user asks about spending, expenses, or what they spent money on.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"period": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"week", "month", "quarter", "year"},
					"description": "Time period for the summary",
					"default":     "month",
				},
				"start_date": map[string]interface{}{
					"type":        "string",
					"format":      "date",
					"description": "Optional start date (YYYY-MM-DD). If not provided, uses period relative to today.",
				},
				"end_date": map[string]interface{}{
					"type":        "string",
					"format":      "date",
					"description": "Optional end date (YYYY-MM-DD). If not provided, uses period relative to today.",
				},
			},
		},
		Meta: map[string]interface{}{
			"openai/outputTemplate": "ui://widget/spending-summary.html",
			"openai/visibility":     "public",
		},
	}
}

func getAccountBalancesTool() *ToolDefinition {
	return &ToolDefinition{
		Name:        "get_account_balances",
		Description: "Get current account balances including net worth, total assets, and total liabilities. Returns a financial overview with account details. Use this when the user asks about their balance, net worth, assets, liabilities, or account status.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Meta: map[string]interface{}{
			"openai/outputTemplate": "ui://widget/account-balances.html",
			"openai/visibility":     "public",
		},
	}
}

func askQuestionTool() *ToolDefinition {
	return &ToolDefinition{
		Name:        "ask_question",
		Description: "Ask a complex question about financial data. This tool uses AI to analyze transactions, accounts, patterns, and provide insights. Use this for questions that require reasoning, analysis, or complex queries that can't be answered by simple data retrieval.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"question": map[string]interface{}{
					"type":        "string",
					"description": "The user's question about their financial data",
				},
			},
			"required": []string{"question"},
		},
		Meta: map[string]interface{}{
			"openai/outputTemplate": "ui://widget/ask-question.html",
			"openai/visibility":     "public",
		},
	}
}

func getSpendingTrendsTool() *ToolDefinition {
	return &ToolDefinition{
		Name:        "get_spending_trends",
		Description: "Get spending trends over time. Returns time series data showing spending patterns, trends, and changes. Use this when the user asks about spending over time, trends, or changes in spending patterns.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"period": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"week", "month", "quarter", "year"},
					"description": "Time period for trend analysis",
					"default":     "month",
				},
			},
		},
		Meta: map[string]interface{}{
			"openai/outputTemplate": "ui://widget/spending-trends.html",
			"openai/visibility":     "public",
		},
	}
}

func getRecurringPatternsTool() *ToolDefinition {
	return &ToolDefinition{
		Name:        "get_recurring_patterns",
		Description: "Get detected recurring patterns like subscriptions and bills. Returns a list of recurring charges with amounts, frequencies, and details. Use this when the user asks about subscriptions, recurring bills, or monthly/annual charges.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Meta: map[string]interface{}{
			"openai/outputTemplate": "ui://widget/recurring-patterns.html",
			"openai/visibility":     "public",
		},
	}
}

func searchTransactionsTool() *ToolDefinition {
	return &ToolDefinition{
		Name:        "search_transactions",
		Description: "Search for specific transactions by description, amount, date range, or other criteria. Returns a list of matching transactions. Use this when the user asks to find specific transactions or search for something.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query (searches description, merchant name, etc.)",
				},
				"start_date": map[string]interface{}{
					"type":        "string",
					"format":      "date",
					"description": "Optional start date (YYYY-MM-DD)",
				},
				"end_date": map[string]interface{}{
					"type":        "string",
					"format":      "date",
					"description": "Optional end date (YYYY-MM-DD)",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results (default: 50)",
					"default":     50,
				},
			},
		},
		Meta: map[string]interface{}{
			"openai/outputTemplate": "ui://widget/search-transactions.html",
			"openai/visibility":     "public",
		},
	}
}

func getFinancialOverviewTool() *ToolDefinition {
	return &ToolDefinition{
		Name:        "get_financial_overview",
		Description: "Get a comprehensive financial overview including net worth, account balances, recent spending summary, and key financial metrics. This is a dashboard-style summary. Use this when the user asks for a general overview, dashboard, or summary of their finances.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Meta: map[string]interface{}{
			"openai/outputTemplate": "ui://widget/financial-overview.html",
			"openai/visibility":     "public",
		},
	}
}
