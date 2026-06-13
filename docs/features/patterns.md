# Pattern Detection

Probably automatically detects recurring financial patterns in your transactions, helping you understand your subscriptions, bills, salary, and other regular payments.

## How It Works

### Entity-First Analysis

Pattern detection uses an **entity-first approach** - analyzing all transactions from a single merchant or entity together. This gives the AI complete context to:

- Identify multiple distinct patterns per entity (e.g., Apple may have iCloud, Apple One, and App Store as separate subscriptions)
- Detect frequency accurately by seeing the full transaction history
- Calculate next expected payment dates
- Provide confidence scores based on consistency

### Supported Pattern Types

| Pattern Type | Description | Examples |
|-------------|-------------|----------|
| **Recurring Bill** | Regular payments for services | Netflix, Spotify, electricity, rent |
| **Salary** | Regular income deposits | Payroll, employer deposits |
| **Investment Contribution** | Regular investment transfers | 401k contributions, brokerage deposits |
| **Household Transfer** | Regular transfers between household members | Allowance, shared expenses |

### Detection Criteria

Patterns are detected based on:

- **Frequency**: Weekly, biweekly, monthly, quarterly, or annual
- **Amount Consistency**: Similar amounts across occurrences
- **Regularity**: Consistent timing between transactions
- **Occurrence Count**: At least 2+ transactions from the same entity

### Pattern Inheritance

When a new transaction arrives for an entity that already has established patterns, it **automatically inherits** that pattern:

- New transactions for known subscriptions (like Resend, Netflix) inherit existing pattern data immediately
- No waiting for multiple transactions to accumulate
- Patterns move from "Past" to "Current" as soon as a new charge arrives

This ensures subscriptions stay correctly classified even if you only get charged once per month.

### Business Type Context

Pattern detection uses **business type context** to make smarter decisions:

- **Subscription-likely businesses** (software, utilities, streaming): More likely to be detected as patterns
- **One-time purchase businesses** (cafes, restaurants, supermarkets): Require higher confidence to be marked as patterns

This reduces false positives - your random coffee purchases won't be flagged as "subscriptions."

## Viewing Patterns

### Patterns List

Navigate to **Patterns** in the main menu to see all detected patterns:

- **Current Patterns**: Active patterns with expected future payments
- **Past Patterns**: Patterns that haven't occurred recently (inactive)

Each pattern shows:
- Entity name and logo
- Frequency (monthly, annual, etc.)
- Current amount
- Monthly equivalent cost

### Pattern Details

Click any pattern to see:

- **Why It's a Pattern**: AI-generated reasoning explaining the detection
- **Transaction History**: All transactions included in the pattern
- **Statistics**: Confidence score, occurrence count, total spent
- **Timeline**: First occurrence, last occurrence, next expected

## Managing Patterns

### Refresh Detection

Click **Refresh Detection** to re-analyze all transactions:

- Queues transactions for pattern re-analysis
- Uses the latest entity-first detection approach
- Updates pattern metadata and reasoning

### Pattern Confidence

Confidence scores indicate how certain the system is about a pattern:

- **70%+**: High confidence - reliable pattern
- **50-70%**: Medium confidence - likely pattern
- **<50%**: Low confidence - possible pattern but uncertain

## Monthly Totals

The patterns page shows aggregated monthly totals:

- All recurring bills converted to monthly equivalent
- Weekly bills × 4.33
- Biweekly bills × 2.17
- Quarterly bills ÷ 3
- Annual bills ÷ 12

## Pulse Integration

Detected patterns power the **Upcoming Bills** section on the Pulse dashboard:

- Recurring bills with 50%+ confidence appear in upcoming bills
- Next expected dates calculated from last occurrence + frequency
- Bills sorted by expected date (soonest first)
- Shows whether each bill is covered by available balance

Patterns are computed dynamically, so changes are immediately reflected in Pulse.

## Technical Details

### Pattern Storage

Patterns are stored as metadata on each transaction:

- `pattern_type`: Type of recurring pattern detected
- `pattern_name`: Specific pattern identifier (e.g., "Apple iCloud Storage")
- `pattern_frequency`: Detected frequency
- `pattern_metadata`: JSON with confidence, reasoning, and details

### Entity Pattern Hints

Entities store learned pattern hints to improve future detection:

```json
{
  "pattern_hints": [
    {"type": "recurring_bill", "frequency": "monthly", "confidence": 85},
    {"type": "recurring_bill", "frequency": "annual", "confidence": 72}
  ]
}
```

## Related Features

- [Transactions](transactions.md) - View individual transactions in patterns
- [Merchants](merchants.md) - Entity information and logos
- [Intelligence](intelligence.md) - See how patterns affect your finances
- [AI Chat](ai-chat.md) - Ask questions about your recurring expenses
