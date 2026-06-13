# Akahu Bank Integration (New Zealand)

Akahu is one of the supported bank connection providers in Probably's multi-provider system. This document covers Akahu-specific features and details.

> **Note**: For information about the unified bank connection system that supports multiple providers (Teller, Akahu, Plaid), see [Multi-Provider Bank Connections](bank-connections.md).

Akahu integration connects your New Zealand bank accounts to Probably, automatically importing transactions and keeping your data up-to-date.

## Overview

Akahu provides secure, read-only access to New Zealand bank accounts through their open finance API. Probably uses Akahu to:

- Connect NZ bank accounts securely
- Import transactions automatically in NZD
- Sync account balances
- Keep data current with scheduled updates and webhooks

## Features

### Bank Connection

Connect accounts through Akahu OAuth:

- **Secure authentication**: OAuth 2.0 with automatic token refresh
- **Read-only access**: Akahu only reads data, never moves money
- **Multi-bank support**: Connect accounts from different NZ banks
- **Institution logos**: Bank logos automatically displayed
- **Account detection**: All accounts from connected institution are imported

### Automatic Transaction Import

When you connect a bank:

- All accounts are automatically created
- Recent transactions are imported
- Account balances are synced in NZD
- Institution information is populated
- NZ bank account numbers are parsed and stored

### Real-time Webhooks

Akahu provides real-time updates via webhooks:

- **TOKEN:DELETE**: Automatically clears credentials when user revokes access
- **ACCOUNT:UPDATE**: Triggers transaction sync when account changes
- **Signature verification**: HMAC-SHA256 verification for security

### Scheduled Syncing

Transactions are automatically synced:

- **Background worker**: Runs every 15 minutes
- **Incremental updates**: Only new transactions are imported
- **Efficient**: Cursor-based pagination for large datasets
- **Automatic**: No manual action required

### Manual Sync

Trigger manual syncs when needed:

- **Sync single account**: Update one account immediately
- **Sync all accounts**: Update all connected NZ accounts
- **Full resync**: Re-import all transactions

### Account Management

Manage connected accounts:

- **View connected accounts**: See all accounts from Akahu
- **Account details**: Institution name, type, balance in NZD
- **Disconnect accounts**: Remove Akahu connection (account remains, stops syncing)
- **Reconnect**: Handle connection issues with re-authentication
- **Account number matching**: Reconnect to existing accounts by NZ account number

### Transaction Processing

Imported transactions are automatically:

- **Pre-enriched by Akahu**: Merchant name, website, category, and NZBN come directly from Akahu's Genie enrichment service
- **Entity creation**: Merchants are automatically created as entities with their enriched data
- **Categorized**: Akahu's category + AI rules assign tags
- **Transfer detection**: Transfers between accounts are identified
- **Deduplication**: Prevents duplicate imports
- **NZD currency**: All amounts stored in NZD

### Built-in Enrichment

Akahu provides pre-enriched transaction data through their **Genie** service:

- **Merchant name**: Clean, human-readable business names
- **Category**: NZFCC (NZ Financial Consumer Categories) classification
- **Website**: Business website URL
- **NZBN**: New Zealand Business Number for verified businesses
- **Logo**: Merchant logo URL

This means NZ transactions from Akahu arrive already enriched - no additional API calls needed!

### Supported Transaction Types

Akahu provides detailed transaction types:

- **CREDIT**: Money received
- **DEBIT**: Money spent
- **EFTPOS**: Point of sale transactions
- **TRANSFER**: Bank transfers
- **DIRECT_DEBIT**: Automatic payments
- **DIRECT_CREDIT**: Direct credits
- **PAYMENT**: Bill payments
- **FEE**: Bank fees
- **INTEREST**: Interest earned/charged

## Security

Akahu integration is secure:

- **OAuth 2.0**: Industry-standard authorization
- **Token-based**: Uses access tokens, not stored credentials
- **Automatic refresh**: Tokens refreshed automatically
- **Read-only access**: Cannot move money or make changes
- **Webhook verification**: HMAC-SHA256 signature verification
- **Disconnect anytime**: Remove access from settings
- **No credential storage**: Your bank credentials are never stored by Probably

## Setup

### Personal App Setup (Single User)

1. Go to [my.akahu.nz](https://my.akahu.nz) and create an account
2. Navigate to the **Developers** page
3. Accept the Developer Terms and complete identity verification
4. Copy your **App ID Token** and **User Access Token**
5. Add to your `.env` file:
   ```bash
   AKAHU_APP_ID=app_xxx
   AKAHU_USER_TOKEN=user_token_xxx
   ```
6. In Probably, navigate to Accounts → Connect Bank
7. Click "🇳🇿 NZ Banks" - accounts will sync automatically

### Full App Setup (Multi-User)

1. Contact Akahu for a full app registration
2. Complete the Akahu accreditation process
3. Configure environment variables with your App ID and Secret
4. Users authenticate through OAuth flow when connecting

## Environment Variables

The following environment variables configure Akahu:

```bash
AKAHU_APP_ID=app_xxx          # Your Akahu application ID (App ID Token)
AKAHU_USER_TOKEN=user_xxx     # Personal app user token (for single-user apps)
AKAHU_APP_SECRET=xxx          # Your Akahu application secret (for full OAuth apps)
AKAHU_ENVIRONMENT=sandbox     # sandbox or production
AKAHU_WEBHOOK_SECRET=xxx      # For webhook signature verification
AKAHU_URL=https://api.akahu.io/v1  # API base URL (optional, defaults shown)
```

### Personal App vs Full App

Probably supports two modes of Akahu integration:

**Personal App Mode** (recommended for single-user):
- Set `AKAHU_APP_ID` and `AKAHU_USER_TOKEN`
- Get these from [my.akahu.nz](https://my.akahu.nz) → Developers page
- No OAuth flow required - accounts sync directly
- Free, limited to your own accounts

**Full App Mode** (for multi-user production):
- Set `AKAHU_APP_ID` and `AKAHU_APP_SECRET`
- Requires Akahu accreditation
- Uses OAuth flow for user authorization
- Supports webhooks and multiple users

## NZ Bank Account Numbers

Akahu provides formatted NZ bank account numbers:

- Format: `BB-bbbb-AAAAAAA-SS` (Bank-Branch-Account-Suffix)
- Example: `01-1234-1234567-12`
- Last four digits extracted for display

## Use Cases

- Automatically track NZ checking account transactions
- Monitor NZ credit card spending
- Sync savings account balances
- Track multiple accounts across NZ banks
- Keep all NZ accounts up-to-date without manual entry

## Supported Banks

Akahu supports major New Zealand banks including:

- ANZ New Zealand
- ASB Bank
- BNZ (Bank of New Zealand)
- Kiwibank
- Westpac New Zealand
- TSB Bank
- And many more

Check Akahu's website for the full list of supported institutions.

## Limitations

- **NZ banks only**: Akahu only supports New Zealand financial institutions
- **Read-only**: Cannot initiate transfers or payments
- **Historical data**: Amount of history depends on the bank
- **MFA required**: Most banks require MFA during connection

## Troubleshooting

Common issues and solutions:

- **Sync failures**: Check Akahu connection status, may need to reconnect
- **Missing transactions**: Trigger a full resync
- **Connection revoked**: Re-authenticate through the connect flow
- **Bank not found**: Check Akahu's supported bank list
- **Webhook issues**: Verify webhook secret is correctly configured

## Related Features

- **Accounts**: Akahu creates accounts automatically
- **Transactions**: Transactions are imported from Akahu
- **Transaction Enrichment**: Imported transactions are enriched
- **Transfer Matching**: Bank transfers are automatically detected
- **Backup**: Akahu tokens are not included in backups (security)
