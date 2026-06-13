# Multi-Provider Bank Connections

Probably supports connecting bank accounts through multiple secure providers, giving you flexibility and coverage across different banks and regions.

## Overview

The bank connection system uses a provider-agnostic architecture that supports:

- **Teller**: Secure bank account connection for US banks
- **Plaid**: Industry-leading financial data platform with 12,000+ institutions globally
- **Akahu**: Bank connections for New Zealand banks

All providers offer:
- Secure, read-only access to your bank accounts
- Automatic transaction import
- Scheduled balance syncing
- Account and institution information

## Features

### Unified Connection Interface

Connect accounts through any supported provider:

- **Provider selection**: Choose the best provider for your bank
- **Consistent experience**: Same interface regardless of provider
- **Multi-provider support**: Connect accounts from different providers simultaneously
- **Provider-agnostic storage**: All accounts stored uniformly in the system

### Automatic Transaction Import

When you connect a bank account:

- All accounts from the institution are automatically created
- Recent transactions are imported (typically last 90 days)
- Account balances are synced
- Institution information (name, logo) is populated
- Account subtypes (checking, savings, credit card) are detected

### Scheduled Syncing

Transactions are automatically synced in the background:

- **Background worker**: Runs every 15 minutes
- **Incremental updates**: Only new transactions are imported
- **Efficient**: Minimal API calls, only when needed
- **Automatic**: No manual action required
- **Multi-provider**: Syncs accounts from all connected providers

### Manual Sync

Trigger manual syncs when needed:

- **Sync single account**: Update one account immediately
- **Sync all accounts**: Update all connected accounts (across all providers)
- **Full resync**: Re-import all transactions (useful after fixing issues)
- **Provider-specific**: Sync all accounts from a specific provider

### Account Management

Manage connected accounts across all providers:

- **View all accounts**: See accounts from all providers in one place
- **Provider identification**: See which provider each account uses
- **Account details**: Institution name, type, balance, last synced time
- **Disconnect accounts**: Remove provider connection (account remains, stops syncing)
- **Reconnect**: Handle MFA-required reconnections
- **Link existing accounts**: Connect provider accounts to manually created accounts

### Transaction Processing

Imported transactions are automatically:

- **Enriched**: Merchant information, logos, descriptions
- **Categorized**: AI rules assign tags
- **Transfer detection**: Transfers between accounts are identified
- **Deduplication**: Prevents duplicate imports across providers

## Supported Providers

### Teller

**Status**: ✅ Fully implemented

- **Coverage**: US banks
- **Authentication**: OAuth-based, secure token exchange
- **Features**: Full account sync, transaction import, balance updates
- **Best for**: US-based users with supported banks

See [Teller Integration](teller-integration.md) for detailed Teller-specific information.

### Plaid

**Status**: ✅ Fully implemented

- **Coverage**: 12,000+ financial institutions globally
- **Authentication**: Plaid Link OAuth flow
- **Features**: Full account sync, transaction import, balance updates, institution logos, connection status monitoring
- **Connection Status**: Real-time monitoring with status banners for login required, pending expiration, and new accounts available
- **Best for**: Users needing broad bank coverage or international accounts

See [Plaid Integration](plaid-integration.md) for detailed Plaid-specific information.

### Akahu

**Status**: ✅ Fully implemented

- **Coverage**: New Zealand banks and financial institutions
- **Authentication**: Akahu OAuth 2.0 flow with automatic token refresh
- **Features**: Full account sync, transaction import, balance updates, NZD support
- **Webhooks**: Real-time updates via `TOKEN:DELETE` and `ACCOUNT:UPDATE` events
- **Best for**: New Zealand-based users

See [Akahu Integration](akahu-integration.md) for detailed Akahu-specific information.

## Security

All provider integrations are secure:

- **Bank-level encryption**: All data encrypted in transit
- **Read-only access**: Cannot move money or make changes
- **Token-based**: Uses access tokens, not stored credentials
- **Provider-specific security**: Each provider follows industry security standards
- **Disconnect anytime**: Remove access from settings
- **No credential storage**: Your bank credentials are never stored by Probably

## Setup

To connect your bank account:

1. Navigate to Accounts → Connect Bank Account
2. The system will show available providers based on your configuration
3. Select your provider (or it will be auto-selected based on your bank)
4. Complete the OAuth authentication flow
5. Authenticate with your bank (may require MFA)
6. Accounts and transactions are automatically imported

## Provider Selection

The system automatically selects the best provider:

- **Bank coverage**: Uses provider that supports your bank
- **Geographic location**: Regional providers (e.g., Akahu for NZ)
- **User preference**: Can manually select provider if multiple options exist
- **Fallback**: If one provider fails, can try another

## Use Cases

- Automatically track checking account transactions
- Monitor credit card spending across multiple cards
- Sync savings account balances
- Import investment account transactions
- Connect accounts from different banks using different providers
- Keep all accounts up-to-date without manual entry
- Support for international banking (when providers are available)

## Limitations

- **Provider availability**: Depends on which providers are configured
- **Bank coverage**: Varies by provider
- **Read-only**: Cannot initiate transfers or payments
- **Historical data**: Typically imports last 90 days initially
- **MFA required**: Some banks require periodic re-authentication
- **Regional restrictions**: Some providers are region-specific

## Troubleshooting

Common issues and solutions:

- **Sync failures**: Check provider connection status, may need to reconnect
- **Missing transactions**: Trigger a full resync
- **MFA required**: Use reconnect flow to re-authenticate
- **Account not found**: Bank may not be supported by the selected provider
- **Provider unavailable**: Check if provider is configured in system settings

## Migration

If you have existing Teller connections:

- **Automatic migration**: Existing Teller accounts are automatically migrated to the new system
- **Backward compatible**: Old Teller-specific features continue to work
- **No data loss**: All account and transaction data is preserved
- **Seamless transition**: No user action required

## Related Features

- **Accounts**: Providers create accounts automatically
- **Transactions**: Transactions are imported from providers
- **Transaction Enrichment**: Imported transactions are enriched
- **Transfer Matching**: Bank transfers are automatically detected
- **Backup**: Provider tokens are not included in backups (security)
- **Intelligence**: Account balances feed into financial reporting
