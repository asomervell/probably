# AI Categorization Rules

> **⚠️ Deprecated in v1.16.0**: The Rules page has been removed from the UI. AI categorization now uses the [Pattern Detection](patterns.md) system and contextual information from [My Life](my-life.md) relationships. Existing rules functionality may still be available via API.

Rules are natural language instructions that guide Probably's AI to automatically categorize your transactions. They make it easy to teach the system how to handle recurring merchants and transaction patterns.

## How Rules Work

Rules use AI (xAI Grok or Groq) to understand transaction descriptions and automatically assign tags:

1. **You write a rule** in plain English describing which transactions should match
2. **The AI processes** new transactions and checks them against your rules
3. **Matching transactions** are automatically tagged with the specified category
4. **Rules have priority** - higher priority rules are checked first

## Features

### Rule Creation

Create rules with:

- **Rule name**: A descriptive name (e.g., "Pet Store Purchases")
- **AI instruction**: Natural language description of what to match
  - Example: "Pet stores like Petstock, PetSmart, or pet supply shops"
- **Target category**: The tag to apply when matched
- **Examples** (optional): Specific transaction descriptions that should match
- **Priority**: Higher numbers are checked first
- **Active status**: Enable/disable rules without deleting

### Rule Examples

Good rule instructions:

- "Coffee shops including Starbucks, local cafes, or any coffee purchase"
- "Subscription services such as Netflix, Spotify, or streaming platforms"
- "Gas stations and fuel purchases"
- "Online retailers like Amazon, eBay, or e-commerce purchases"

### Rule Priority

- Rules are processed in priority order (highest first)
- First matching rule wins
- Use priority to handle edge cases and overrides
- Default priority is 0

### Re-categorization

Apply updated rules to existing transactions:

- **Search and re-categorize**: Find transactions by merchant/description and re-process
- **Re-categorize all**: Re-process every transaction with current rules
- Useful when you add new rules or improve existing ones

### Rule Management

- View all rules with their instructions and target categories
- Edit rules to refine matching logic
- Enable/disable rules without deleting
- Delete rules (doesn't affect already-tagged transactions)

## Use Cases

- Automatically tag recurring merchants (e.g., "Starbucks → Coffee")
- Handle merchant name variations (e.g., "PETSTOCK", "Petstock", "Pet Stock")
- Create category rules for spending patterns
- Override default categorizations with custom rules
- Teach the system your specific categorization preferences

## Best Practices

- Start with specific rules for your most common merchants
- Use clear, descriptive instructions
- Add examples for merchants with unusual names
- Review and refine rules based on categorization results
- Use priority to handle conflicts (e.g., "Amazon → Shopping" vs "Amazon Prime → Subscriptions")

## Related Features

- **Tags**: Rules assign tags to transactions
- **Transactions**: Rules process and categorize transactions
- **Processing Worker**: Background worker applies rules automatically
- **Merchants**: Rules can match by merchant name
