# AI Chat User Guide

## What is AI Chat?

AI Chat is a conversational interface that lets you ask questions about your finances in natural language. Instead of writing SQL queries or navigating complex reports, you can simply ask questions like "How much did I spend this month?" and get instant answers.

The AI understands your financial data and generates the appropriate queries to answer your questions, then presents the results in an easy-to-understand format with tables, charts, and natural language summaries.

## Getting Started

1. **Navigate to AI Chat**: Click on "AI Chat" in the sidebar or mobile navigation
2. **Type your question**: Enter your question in the input box at the bottom of the chat
3. **Review the results**: The AI will generate a response with:
   - A natural language answer
   - Tables showing detailed data (if applicable)
   - Charts visualizing trends (if applicable)
   - Suggested follow-up questions

## Example Questions

### Spending Questions
- "How much did I spend this month?"
- "What are my top 5 expense categories?"
- "Show me all Amazon purchases over $100"
- "How much did I spend on groceries last month?"
- "What's my total spending for the year?"
- "How much did I spend yesterday?"

### Category Analysis
- "What categories am I spending the most on?"
- "Compare my spending this month vs last month"
- "Show me spending by category for the last 3 months"
- "Which category has the highest average transaction amount?"
- "Show me all transactions in the 'Dining' category"
- "What percentage of my spending goes to groceries?"

### Transaction Queries
- "Show me all transactions over $500"
- "What did I spend on restaurants this month?"
- "Find all transactions with 'Amazon' in the description"
- "Show me my 10 largest expenses this month"
- "What transactions happened on weekends?"
- "Find all recurring subscription payments"

### Time-based Analysis
- "What's my average monthly spending?"
- "Show me spending trends over the last 6 months"
- "How much did I spend in January vs February?"
- "What's my spending pace for this month?"
- "Compare this quarter to last quarter"
- "Show me daily spending for the last week"

### Account-specific Queries
- "What's the balance of my checking account?"
- "Show me all credit card transactions"
- "How much did I spend from my savings account?"
- "What transactions came from my Chase account?"
- "Show me all transfers between accounts"

### Income Questions
- "How much income did I receive this month?"
- "What are my income sources?"
- "Show me all salary deposits"
- "Compare my income this month vs last month"

### Budget and Goals
- "Am I over budget for groceries this month?"
- "How much do I have left to spend this month?"
- "What's my spending rate compared to my income?"
- "Show me categories where I'm spending more than average"

### Follow-up Examples

**Example 1: Spending Analysis**
- You: "How much did I spend this month?"
- AI: "You spent $2,345.67 this month across 45 transactions"
- You: "What's the biggest expense?"
- AI: "Your biggest expense this month was $456.78 for Groceries"
- You: "Show me all grocery transactions"
- AI: [Shows table of grocery transactions]

**Example 2: Category Comparison**
- You: "Compare my spending this month vs last month"
- AI: [Shows comparison chart]
- You: "Which categories increased the most?"
- AI: "Dining increased by $123.45 (25%) compared to last month"
- You: "Show me those dining transactions"
- AI: [Shows dining transactions]

**Example 3: Trend Analysis**
- You: "Show me spending trends over the last 6 months"
- AI: [Shows line chart]
- You: "What's the average?"
- AI: "Your average monthly spending over the last 6 months is $2,100.50"
- You: "Is that higher or lower than this month?"
- AI: "This month's spending ($2,345.67) is 11.7% higher than your 6-month average"

## Understanding Results

### Tables
When the AI returns tabular data, you'll see:
- **Formatted numbers**: Currency values are automatically formatted (e.g., $1,234.56)
- **Dates**: Transaction dates in readable format
- **Categories**: Transaction categories and tags
- **Sortable columns**: Click column headers to sort (if applicable)

### Charts
The AI automatically detects when data should be visualized:
- **Line charts**: For trends over time (e.g., monthly spending)
- **Bar charts**: For comparisons (e.g., spending by category)
- **Pie charts**: For proportions (e.g., category breakdown)

### Natural Language Answers
The AI provides a summary sentence explaining the results:
- "You spent $2,345.67 this month across 45 transactions"
- "Your top expense category is Groceries with $456.78"
- "You have 12 transactions over $100 this month"

## Follow-up Questions

The AI remembers the context of your conversation, so you can ask follow-up questions:

**Example conversation:**
- You: "How much did I spend this month?"
- AI: "You spent $2,345.67 this month..."
- You: "What's the biggest expense?"
- AI: "Your biggest expense this month was..."

The AI understands references like "this month", "that category", "those transactions" based on your previous questions.

## Suggested Questions

When you first open the chat, you'll see suggested questions based on your data:
- Top spending categories
- Time-based comparisons
- General search queries

Click any suggestion to ask that question instantly. Suggestions disappear when you start typing, but you can reload them by clearing your input.

## Tips for Better Results

1. **Be specific**: "How much did I spend on restaurants in January?" is better than "restaurants"
2. **Use natural language**: Write questions as you would ask a person
3. **Reference previous questions**: "Show me more details about that" works after asking a summary question
4. **Include timeframes**: "This month", "last 3 months", "in 2024"
5. **Specify amounts**: "Over $100", "between $50 and $200"

## Limitations

### What the AI Can Do
- Answer questions about your transaction history
- Analyze spending patterns and trends
- Compare data across time periods
- Generate visualizations for appropriate data
- Remember conversation context for follow-up questions

### What the AI Cannot Do
- Modify transactions or accounts (read-only)
- Access data from other users' ledgers
- Answer questions about future transactions (only historical data)
- Perform complex financial calculations beyond basic aggregations
- Export data (use the main app for exports)

### Data Scope
- The AI only has access to **your ledger's data**
- It cannot see data from other ledgers or accounts you don't have access to
- All queries are automatically scoped to your ledger for security

## Privacy and Security

- **Server-side execution**: All SQL queries are generated and executed on the server
- **No SQL exposure**: You never see or interact with SQL directly
- **Ledger isolation**: Queries are automatically filtered to your ledger only
- **Read-only access**: The AI can only read data, never modify it
- **Secure validation**: All queries are validated for safety before execution

## Troubleshooting

### "I'm not getting the results I expected"
- Try rephrasing your question more specifically
- Check that you're asking about data that exists (e.g., don't ask about "next month" if it hasn't happened)
- Use the suggested questions as examples of what works well

### "The AI didn't understand my question"
- Be more explicit about what you want
- Break complex questions into simpler ones
- Use the suggested questions as a starting point

### "The response is taking too long"
- Complex queries with large date ranges may take longer
- Try narrowing your question (e.g., "this month" instead of "all time")
- The AI automatically limits results to prevent very slow queries

### "I want to ask about something else"
- Start a new conversation by refreshing the page
- Or simply ask a completely different question - the AI will adapt

## Getting Help

If you encounter issues or have questions:
1. Check this guide for common solutions
2. Try rephrasing your question
3. Use the suggested questions to see what works
4. Contact support if problems persist

## Advanced Usage

### Conversation Context
The AI remembers your last 10 messages in the conversation. This allows for natural follow-up questions. After 1 hour of inactivity, the conversation context resets.

### Query Caching
Repeated queries are cached for 5 minutes to improve performance. If you ask the same question twice within 5 minutes, you'll get the cached result instantly.

### Visualization Detection
The AI automatically detects when to show charts:
- **Line charts**: When results include dates and numeric values
- **Bar charts**: When comparing categories or groups
- **Pie charts**: When showing proportions or percentages

You don't need to ask for charts explicitly - the AI will add them when appropriate.

---

**Note**: AI Chat is designed to make financial data more accessible. For complex analysis or bulk operations, use the main app's transaction and reporting features.
