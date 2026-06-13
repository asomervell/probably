# Transactions

Transactions are the core of your financial record-keeping. Every financial event in Probably is recorded as a transaction using double-entry bookkeeping.

## Double-Entry Bookkeeping

Each transaction has at least two entries that must balance to zero:

- **Debit entries**: Money going out of an account (positive amounts)
- **Credit entries**: Money coming into an account (negative amounts)
- **Balance**: All entries in a transaction must sum to zero

For example, a $100 grocery purchase:
- Debit $100 to "Groceries" expense account
- Credit $100 from "Checking" asset account

## Features

### Manual Transaction Entry

Create transactions manually:

- Enter description, date, and amount
- Select accounts for debit and credit entries
- Add tags for categorization
- Attach notes or receipts

### Automatic Transaction Import

Transactions are automatically imported when you:

- Connect bank accounts via Teller
- Sync existing connected accounts
- The system processes new transactions in the background

### Transaction Management

- **View all transactions** with filtering and search
- **Filter by**: account, tag, date range, uncategorized, needs review
- **Search** by description or merchant name
- **Edit** transaction details (description, date, amount, accounts)
- **Delete** transactions (with safety checks)
- **View transaction details** including all entries and tags

### Transaction Filtering

Quick filters help you focus on what matters:

- **All**: View every transaction
- **Uncategorized**: Transactions without tags
- **Needs Review**: Transactions flagged for manual review
- **Transfers**: Money movement between accounts

### Tagging

- Add or remove tags to categorize transactions
- Multiple tags per transaction supported
- Tags link to your category hierarchy
- Used for reporting and insights

### Transaction Review

- Flag transactions that need manual review
- Review queue for uncertain categorizations
- Mark transactions as reviewed when resolved

### Transfer Detection

- Automatic detection of transfers between accounts
- Manual linking of transfer pairs
- Transfer transactions are marked and excluded from P&L

## Transaction Enrichment

Transactions are automatically enriched with:

- **Merchant information**: Name, logo, website, description
- **Category suggestions**: AI-powered categorization
- **Logo display**: Visual merchant logos in transaction lists
- **Metadata**: Additional context from enrichment services

## Use Cases

- Record daily expenses and income
- Track bank transactions automatically
- Categorize spending for budgeting
- Review and correct automatic categorizations
- Monitor transfers between accounts
- Search transaction history

## Related Features

- **Tags**: Categorize transactions for reporting
- **Rules**: Automatic categorization rules
- **Accounts**: All transactions link to accounts
- **Intelligence**: Transaction data powers financial reports
- **Transfers**: Automatic transfer detection and matching
