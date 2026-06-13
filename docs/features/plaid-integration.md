# Plaid Bank Integration

Plaid is one of the supported bank connection providers in Probably's multi-provider system. This document covers Plaid-specific features and details.

> **Note**: For information about the unified bank connection system that supports multiple providers (Teller, Plaid, Akahu), see [Multi-Provider Bank Connections](bank-connections.md).

Plaid integration connects your real bank accounts to Probably, automatically importing transactions and keeping your data up-to-date.

## Overview

Plaid provides secure, read-only access to your bank accounts through their API. Probably uses Plaid to:

- Connect bank accounts securely
- Import transactions automatically
- Sync account balances
- Keep data current with scheduled updates
- Monitor connection status in real-time

## Features

### Bank Connection

Connect accounts through Plaid Link:

- **Secure authentication**: Bank-level encryption and security
- **Read-only access**: Plaid only reads data, never moves money
- **Broad coverage**: 12,000+ financial institutions globally
- **Institution logos**: Bank logos automatically fetched and displayed
- **Account detection**: All accounts from connected institution are imported
- **Account selection**: Choose which accounts to add when connecting

### Connection Status Monitoring

Plaid provides real-time connection status tracking:

- **Status banners**: Visual indicators when action is required
- **Status types**:
  - **Login Required** (highest priority): Connection needs re-authentication (shown in red)
  - **Pending Disconnect**: Connection will be disconnected soon (shown in red)
  - **Pending Expiration**: Connection will expire soon (shown in yellow)
  - **New Accounts Available**: Additional accounts can be added (shown in blue)
- **Direct actions**: Status banners include buttons to reconnect or add accounts
- **Automatic updates**: Status is updated during sync operations

### Automatic Transaction Import

When you connect a bank:

- All selected accounts are automatically created
- **Full transaction history** is imported (up to 2 years of history)
- Account balances are synced
- Institution information (name, logo) is populated
- Merchant information and categories are included
- **Smart backfill**: If an account was recently connected or has limited history, the system automatically backfills to get the full 2 years of transactions

### Scheduled Syncing

Transactions are automatically synced:

- **Background worker**: Runs every 15 minutes
- **Incremental updates**: Only new transactions are imported
- **Efficient**: Minimal API calls, only when needed
- **Automatic**: No manual action required
- **Status-aware**: Sync operations update connection status

### Manual Sync

Trigger manual syncs when needed:

- **Sync single account**: Update one account immediately (fetches last 90 days)
- **Sync all accounts**: Update all connected accounts
- **Full resync**: Re-import all transactions with full 2-year history (useful after fixing issues or to get complete transaction history)
  - Available from the account detail page via the "Resync" button
  - Deletes existing transactions and re-imports with full history
- **Provider-specific**: Sync all accounts from Plaid

### Account Management

Manage connected accounts:

- **View connected accounts**: See all accounts from Plaid
- **Account details**: Institution name, type, balance, connection status
- **Disconnect accounts**: Remove Plaid connection (account remains, stops syncing)
- **Reconnect**: Handle re-authentication when login is required
- **Add new accounts**: Add additional accounts when they become available
- **Delete all accounts**: Bulk delete all accounts for an institution
- **Link existing accounts**: Connect Plaid accounts to manually created accounts

### Transaction Processing

Imported transactions are automatically:

- **Enriched**: Merchant information, logos, descriptions from Plaid
- **Categorized**: Plaid Personal Finance Categories (PFC) are imported
- **Entity linking**: Transactions are linked to merchant entities
- **Transfer detection**: Transfers between accounts are identified
- **Deduplication**: Prevents duplicate imports across providers

### Institution Logos

Plaid institution logos are automatically:

- **Fetched from Plaid**: Logos provided as base64-encoded images
- **Stored in CDN**: Downloaded and stored in cloud storage
- **Displayed consistently**: Shown on connected accounts page and transaction lists
- **Fallback support**: Generic icons when logos aren't available

## Security

Plaid integration is secure:

- **Bank-level encryption**: All data encrypted in transit
- **Read-only access**: Cannot move money or make changes
- **Token-based**: Uses access tokens, not stored credentials
- **OAuth 2.0**: Industry-standard authentication flow
- **Disconnect anytime**: Remove access from settings
- **No credential storage**: Your bank credentials are never stored by Probably

## Connection Status

Plaid connections can have different statuses that require attention:

### Login Required

- **Priority**: Highest (shown in red)
- **Meaning**: Connection needs re-authentication
- **Action**: Click "Reconnect" button in status banner
- **Impact**: Sync will fail until reconnected

### Pending Disconnect

- **Priority**: High (shown in red)
- **Meaning**: Connection will be disconnected soon
- **Action**: Reconnect to maintain access
- **Impact**: Connection will stop working if not addressed

### Pending Expiration

- **Priority**: Medium (shown in yellow)
- **Meaning**: Connection will expire soon
- **Action**: Reconnect before expiration
- **Impact**: Connection will stop working after expiration

### New Accounts Available

- **Priority**: Low (shown in blue)
- **Meaning**: Additional accounts can be added to connection
- **Action**: Click "Add Accounts" to select new accounts
- **Impact**: No impact on existing accounts

## Duplicate Connection Detection

Probably automatically detects if you try to connect the same Plaid Item multiple times:

- **Detection**: System checks for existing connections with same Item ID
- **Prevention**: Shows error message instead of creating duplicate
- **Solution**: Option to reconnect existing connection instead
- **Benefit**: Prevents data duplication and confusion

## Troubleshooting

Common issues and solutions:

### Connection Status Issues

- **Login Required**: Use reconnect flow to re-authenticate
- **Pending Expiration**: Reconnect before expiration date
- **Pending Disconnect**: Reconnect to maintain access

### Sync Failures

- **Check connection status**: May need to reconnect
- **Verify account access**: Bank may have changed access requirements
- **Try manual sync**: Trigger sync from account detail page

### Missing Transactions

- **Trigger full resync**: Re-import all transactions
- **Check date range**: Plaid typically provides last 90 days
- **Verify account selection**: Ensure all accounts are connected

### Duplicate Connections

- **System prevents duplicates**: Error message will appear
- **Use reconnect**: Reconnect existing connection instead
- **Delete and reconnect**: If needed, delete old connection first

### Institution Logo Not Showing

- **Automatic download**: Logos are fetched during account sync
- **May take time**: Logo download happens in background
- **Fallback**: Generic icon shown if logo unavailable

## Best Practices

- **Monitor connection status**: Check status banners regularly
- **Reconnect promptly**: Address login required status quickly
- **Select accounts carefully**: Only connect accounts you need
- **Use account selection**: Add new accounts when available
- **Keep connections active**: Reconnect before expiration

## Related Features

- **Multi-Provider Bank Connections**: Unified bank connection system
- **Accounts**: Account management and details
- **Transactions**: Transaction import and viewing
- **Transaction Enrichment**: Merchant information and logos
- **Transfer Matching**: Automatic transfer detection
- **Intelligence**: Account balances feed into financial reporting
