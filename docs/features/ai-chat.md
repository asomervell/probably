# AI Chat

AI Chat is a conversational interface where you can ask questions about your finances in natural language. The AI understands your questions, generates SQL queries to analyze your data, and provides clear, natural language answers with visualizations.

## Overview

Instead of navigating complex reports or writing SQL queries, simply ask questions like:

- "How much did I spend on coffee last month?"
- "What's my biggest expense category this year?"
- "Show me all transactions over $500"
- "How does my spending this month compare to last month?"

The AI generates the appropriate SQL query, executes it securely on your data, and presents the results in an easy-to-understand format.

## Features

### Persistent Chat Threads

Conversations are automatically saved, allowing you to continue previous discussions:

- **Thread History**: All conversations are saved with auto-generated titles
- **Thread Sidebar**: View and navigate between previous conversations
- **Context Preservation**: Message history provides context for follow-up questions
- **Thread Management**: Create new threads, load existing ones, or delete old conversations

### Streaming AI Thoughts

Watch the AI think in real-time as it processes your questions:

- **Live Reasoning**: See the AI's thought process as it analyzes your question
- **Thinking Indicator**: Animated visual feedback shows when the AI is processing
- **Thought Bubbles**: Individual thoughts appear as they're generated
- **Transparent Process**: Understand how the AI arrives at its answers

### Semantic Similarity Search

Find transactions by meaning, not just keywords:

- **"Find transactions similar to Netflix"**: Uses AI embeddings to find semantically similar transactions
- **Smart Matching**: Even transactions without exact keyword matches can be found
- **Pattern Discovery**: Identify related transactions across different merchants

### Natural Language Queries

Ask questions in plain English:

- **Spending questions**: "How much did I spend?", "What did I spend on restaurants?"
- **Category analysis**: "What's my biggest expense category?", "Show me spending by category"
- **Time-based queries**: "How much did I spend this month?", "Compare this quarter to last quarter"
- **Transaction search**: "Show me all transactions over $100", "Find transactions with 'Amazon'"
- **Account-specific**: "How much did I spend from my checking account?"
- **Income questions**: "How much did I earn this year?", "What's my average monthly income?"

### Intelligent SQL Generation

The AI automatically:

- Generates valid PostgreSQL SQL queries
- Includes security filters (ledger_id) to ensure data isolation
- Handles complex joins across transactions, entries, accounts, and tags
- Optimizes queries for performance
- Validates SQL syntax before execution

### Results Presentation

Results are presented in multiple formats:

- **Tables**: Structured data with columns and rows
- **Charts**: Automatic visualization detection (bar charts, line charts, pie charts)
- **Natural language**: Clear explanations of what the data means
- **Formatted numbers**: Currency, percentages, and dates formatted for readability

### Conversation Context

The chat maintains persistent conversation context:

- **Follow-up questions**: Ask "What about last month?" after asking about this month
- **Context awareness**: The AI remembers all previous questions in the thread
- **Full history**: Complete conversation history is preserved in the database
- **Cross-session persistence**: Return to conversations anytime, even after closing the browser
- **Thread-based**: Each conversation thread maintains its own context

### Suggested Questions

Get started quickly with suggested questions:

- Pre-generated suggestions based on your data
- Time-based suggestions (this month, last month, etc.)
- Category-based suggestions (top spending categories)
- Comparison suggestions (this vs. last period)

### Security

All queries are executed securely:

- **SQL never exposed**: Users never see or can modify the SQL
- **Ledger isolation**: All queries automatically filter by ledger_id
- **Read-only**: Only SELECT queries are allowed
- **Validation**: SQL is validated before execution to prevent injection

### Query Caching

Frequently asked questions are cached:

- **Fast responses**: Cached queries return instantly
- **Automatic caching**: Results cached for 5 minutes
- **Cache invalidation**: Cache cleared when new transactions are added

## Use Cases

### Spending Analysis

- Track spending by category
- Compare spending across time periods
- Identify top spending categories
- Find unusual or large transactions

### Budget Monitoring

- Check spending against budgets
- See where money is going
- Identify areas for cost reduction
- Track progress toward financial goals

### Financial Reporting

- Generate custom reports
- Answer specific financial questions
- Get insights into spending patterns
- Understand income and expense trends

### Transaction Discovery

- Find specific transactions
- Search by merchant, amount, or date
- Filter by account or category
- Review transaction history

## Example Questions

### Spending Questions

- "How much did I spend this month?"
- "What's my total spending this year?"
- "How much did I spend on restaurants last month?"
- "Show me my top 5 spending categories"

### Category Analysis

- "What's my biggest expense category?"
- "How much did I spend on groceries vs. dining out?"
- "Show me spending by category for this month"
- "What percentage of my spending is on entertainment?"

### Time Comparisons

- "How does my spending this month compare to last month?"
- "Show me spending trends over the last 6 months"
- "What's my average monthly spending?"
- "Compare this quarter to last quarter"

### Transaction Search

- "Show me all transactions over $100"
- "Find transactions with 'Amazon' in the description"
- "What did I buy on [date]?"
- "Show me transactions from my credit card"

### Income Questions

- "How much did I earn this year?"
- "What's my average monthly income?"
- "Show me income by source"
- "Compare this month's income to last month"

## Understanding Results

### Tables

When results are presented as tables:

- **Columns**: Data fields (amount, category, date, etc.)
- **Rows**: Individual records
- **Sorting**: Click column headers to sort
- **Formatted values**: Currency, dates, and numbers are formatted

### Charts

Charts are automatically generated when appropriate:

- **Bar charts**: For category comparisons
- **Line charts**: For trends over time
- **Pie charts**: For proportional breakdowns
- **Interactive**: Hover to see exact values

### Natural Language Answers

The AI provides explanations:

- **Summary**: High-level answer to your question
- **Context**: Additional relevant information
- **Insights**: Observations about the data
- **Formatted**: Numbers and dates formatted for readability

## Tips for Better Results

### Be Specific

- ✅ "How much did I spend on restaurants this month?"
- ❌ "How much did I spend?" (too vague)

### Use Clear Time References

- ✅ "This month", "Last month", "This year"
- ✅ "From January 1 to March 31"
- ❌ "Recently" (too vague)

### Specify Categories When Needed

- ✅ "How much did I spend on groceries?"
- ✅ "Show me spending on entertainment"

### Ask Follow-up Questions

- After asking "How much did I spend this month?", you can ask:
  - "What about last month?"
  - "What's the biggest category?"
  - "Show me the transactions"

## Limitations

### What AI Can Do

- ✅ Answer questions about your financial data
- ✅ Generate SQL queries automatically
- ✅ Provide insights and analysis
- ✅ Compare time periods
- ✅ Search transactions

### What AI Cannot Do

- ❌ Modify or delete data
- ❌ Create accounts or tags
- ❌ Access data from other ledgers
- ❌ Execute non-SELECT queries
- ❌ Access data outside your ledger

### Data Scope

The AI can only access:

- Your transactions and entries
- Your accounts
- Your tags and categories
- Your entities (merchants)
- Data within your ledger only

## Privacy and Security

- **Data isolation**: Queries are automatically scoped to your ledger
- **No data sharing**: Your data is never shared with other users
- **Secure execution**: SQL is validated and executed server-side
- **No SQL exposure**: Users never see or can modify the SQL
- **Read-only access**: Only SELECT queries are allowed

## Troubleshooting

### No Results Found

- Check that you have transactions in the date range
- Verify your question is asking about data you have
- Try a broader time range

### Unexpected Results

- Be more specific in your question
- Check that categories/tags are correctly applied
- Verify account names and dates

### Slow Responses

- Complex queries may take a few seconds
- Try being more specific to narrow the scope
- Check your internet connection

## Advanced Usage

### Conversation Context

The chat maintains context across messages:

- Ask follow-up questions without repeating context
- Reference previous answers
- Build on previous questions

### Query Caching

Frequently asked questions are cached:

- Identical questions return cached results
- Cache expires after 5 minutes
- New transactions invalidate relevant caches

### Visualization Detection

The AI automatically detects when to show charts:

- Multiple categories → Bar chart
- Time series data → Line chart
- Proportional data → Pie chart
- Single values → Natural language answer

## Related Features

- **Intelligence**: Dashboard with financial overview
- **Insights**: AI-generated financial observations
- **Transactions**: View and manage individual transactions
- **Tags**: Categories used in analysis
- **Accounts**: Account data used in queries
