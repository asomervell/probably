# Transfer Matching

Transfer matching automatically detects when money moves between your accounts and links those transactions together. This prevents transfers from appearing as income or expenses in your financial reports.

## What Are Transfers?

Transfers are money movements between your own accounts:

- Moving money from checking to savings
- Paying a credit card from a checking account
- Transferring funds between investment accounts
- Any transaction where money moves but doesn't leave your control

## Features

### Automatic Detection

The system automatically detects potential transfers by:

- **Matching amounts**: Finding transactions with opposite amounts
- **Date proximity**: Looking within a 3-day window
- **Account types**: Only matching between asset/liability accounts
- **Description patterns**: Recognizing transfer-related keywords
- **Confidence scoring**: Calculating match confidence based on multiple factors

### Confidence Levels

Transfers are matched with confidence scores:

- **High confidence (85%+)**: Automatically linked without review
- **Medium confidence (50-85%)**: Queued for manual review
- **Low confidence (<50%)**: Not matched, remain as separate transactions

### Match Reasons

Each match includes reasons explaining why it was detected:

- "Opposite amounts"
- "Same date"
- "Transfer keywords in description"
- "Account relationship"
- Multiple reasons increase confidence

### Manual Review

Review pending matches to confirm or reject:

- **View match details**: See both transactions side-by-side
- **Confidence indicator**: Visual badge showing match confidence
- **Match reasons**: Understand why the system thinks it's a transfer
- **Confirm**: Accept the match and link the transactions
- **Reject**: Mark as not a transfer (they remain separate)

### Manual Matching

Manually link transactions as transfers:

- Select two transactions to link
- Useful for transfers the system missed
- Overrides automatic detection

### Re-matching

Re-run transfer matching on all transactions:

- Useful after fixing matching logic
- Re-processes previously unmatched transactions
- Shows statistics: auto-linked vs pending review

### Unlinking

Remove transfer links when needed:

- Unlink incorrectly matched transfers
- Transactions become independent again
- Can be re-matched later if needed

## How It Works

1. **New transaction arrives**: When a transaction is imported or created
2. **Search for candidates**: System looks for matching transactions in other accounts
3. **Score matches**: Calculates confidence based on amount, date, description
4. **Auto-link or queue**: High confidence = auto-link, medium = review queue
5. **User review**: You confirm or reject pending matches
6. **Mark as transfer**: Confirmed matches are marked, excluded from P&L

## Use Cases

- Automatically handle bank-to-bank transfers
- Link credit card payments to checking account withdrawals
- Connect investment account transfers
- Prevent transfers from appearing as expenses
- Keep balance sheet accurate while excluding internal movements

## Benefits

- **Accurate reporting**: Transfers don't inflate income/expense totals
- **Time savings**: Most transfers are automatically detected
- **Flexibility**: Manual review ensures accuracy
- **Transparency**: See why matches were made

## Related Features

- **Transactions**: Transfers are special types of transactions
- **Accounts**: Transfers link transactions between accounts
- **Intelligence**: Transfers are excluded from P&L calculations
- **Teller Integration**: Bank transfers are automatically detected
