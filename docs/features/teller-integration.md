# Teller Bank Integration

Teller is one of the supported bank connection providers in Probably's multi-provider system. This document covers Teller-specific features and details.

> **Note**: For information about the unified bank connection system that supports multiple providers (Teller, Plaid, Akahu), see [Multi-Provider Bank Connections](bank-connections.md).

Teller integration connects your real bank accounts to Probably, automatically importing transactions and keeping your data up-to-date.

## Overview

Teller provides secure, read-only access to your bank accounts through their API. Probably uses Teller to:

- Connect bank accounts securely
- Import transactions automatically
- Sync account balances
- Keep data current with scheduled updates

## Features

### Bank Connection

Connect accounts through Teller Connect:

- **Secure authentication**: Bank-level encryption and security
- **Read-only access**: Teller only reads data, never moves money
- **Multi-bank support**: Connect accounts from different banks
- **Institution logos**: Bank logos automatically displayed
- **Account detection**: All accounts from connected institution are imported

### Automatic Transaction Import

When you connect a bank:

- All accounts are automatically created
- Recent transactions are imported (typically last 90 days)
- Account balances are synced
- Institution information is populated

### Scheduled Syncing

Transactions are automatically synced:

- **Background worker**: Runs every 15 minutes
- **Incremental updates**: Only new transactions are imported
- **Efficient**: Minimal API calls, only when needed
- **Automatic**: No manual action required

### Manual Sync

Trigger manual syncs when needed:

- **Sync single account**: Update one account immediately
- **Sync all accounts**: Update all connected accounts
- **Full resync**: Re-import all transactions (useful after fixing issues)
- **Multi-account sync**: Sync multiple accounts from grouped institutions

### Account Management

Manage connected accounts:

- **View connected accounts**: See all accounts from Teller
- **Account details**: Institution name, type, balance
- **Disconnect accounts**: Remove Teller connection (account remains, stops syncing)
- **Reconnect**: Handle MFA-required reconnections
- **Link existing accounts**: Connect Teller accounts to manually created accounts

### Transaction Processing

Imported transactions are automatically:

- **Enriched**: Merchant information, logos, descriptions
- **Categorized**: AI rules assign tags
- **Transfer detection**: Transfers between accounts are identified
- **Deduplication**: Prevents duplicate imports

## Security

Teller integration is secure:

- **Bank-level encryption**: All data encrypted in transit
- **Read-only access**: Cannot move money or make changes
- **Token-based**: Uses access tokens, not stored credentials
- **Disconnect anytime**: Remove access from settings
- **No credential storage**: Your bank credentials are never stored by Probably

## Setup

To connect your bank:

1. Navigate to Accounts → Connect Bank
2. Click "Connect Bank Account"
3. Select your bank from Teller's interface
4. Authenticate with your bank (may require MFA)
5. Accounts and transactions are automatically imported

## Use Cases

- Automatically track checking account transactions
- Monitor credit card spending
- Sync savings account balances
- Import investment account transactions
- Keep all accounts up-to-date without manual entry

## Limitations

- **Supported banks**: Depends on Teller's bank coverage
- **Read-only**: Cannot initiate transfers or payments
- **Historical data**: Typically imports last 90 days initially
- **MFA required**: Some banks require periodic re-authentication

## Troubleshooting

Common issues and solutions:

- **Sync failures**: Check Teller connection status, may need to reconnect. The system will automatically redirect you to reconnect if your access token is missing or expired.
- **Missing transactions**: Trigger a full resync from the account settings
- **MFA required**: Use the reconnect flow to re-authenticate. You'll be automatically redirected when MFA is required.
- **"Unable to process" errors**: This may indicate a temporary Teller API issue or that your account needs to be reconnected. Try again in a few minutes, or reconnect your account.
- **Account not found**: Some banks may not be supported by Teller
- **Account reconnection**: When reconnecting, your existing accounts will be automatically matched by Teller account ID, so you won't lose transaction history

## Related Features

- **Accounts**: Teller creates accounts automatically
- **Transactions**: Transactions are imported from Teller
- **Transaction Enrichment**: Imported transactions are enriched
- **Transfer Matching**: Bank transfers are automatically detected
- **Backup**: Teller tokens are not included in backups (security)
