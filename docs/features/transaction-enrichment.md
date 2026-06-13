# Transaction Enrichment

Transaction enrichment automatically adds rich metadata to your transactions, including merchant logos, descriptions, websites, and standardized merchant names. This makes your transaction list more visual and informative.

## Overview

When transactions are imported or created, the enrichment system automatically:

- Identifies merchants from transaction descriptions
- Downloads merchant logos
- Fetches merchant descriptions and websites
- Standardizes merchant names
- Links transactions to merchant records

## Features

### Merchant Identification

The system identifies merchants from transaction descriptions:

- **Pattern matching**: Recognizes merchant names in various formats
- **Name parsing**: Extracts merchant names from payment processor descriptions
- **Standardization**: Normalizes names (e.g., "PETSTOCK", "Petstock", "Pet Stock" → "Petstock")
- **Fuzzy matching**: Handles typos and variations

### Logo Enrichment

Merchant logos are automatically downloaded and stored:

- **Multiple sources**: Tries Firecrawl first, then logo.dev
- **Local storage**: Logos stored in cloud storage (GCS/S3)
- **CDN delivery**: Logos served via CDN for fast loading
- **Fallback handling**: Gracefully handles missing logos
- **Institution logos**: Bank/institution logos from Teller

### Merchant Information

Rich merchant data is fetched:

- **Descriptions**: What the merchant does (e.g., "Pet supply retailer")
- **Websites**: Official merchant websites
- **Categories**: Merchant category information
- **Metadata**: Additional context from enrichment services

### Enrichment Sources

The system uses multiple enrichment services:

1. **Firecrawl**: Web scraping for brand information and logos
   - Best for detailed merchant information
   - Extracts from merchant websites
   - Country-aware searching

2. **logo.dev**: Logo search and retrieval
   - Can search by merchant name (no website required)
   - Large logo database
   - Fast logo retrieval

3. **Bud API**: Transaction enrichment (if configured)
   - Merchant name resolution
   - Category suggestions
   - Additional metadata

### Fast-Path Enrichment

Optimization for identical transactions:

- If a transaction description was seen before, reuse enrichment data
- Speeds up processing for recurring merchants
- Reduces API calls to enrichment services

### Merchant Management

- **Merchant records**: Each unique merchant has a record
- **User verification**: You can verify merchant assignments
- **Merchant search**: Find transactions by merchant
- **Merchant details**: View all transactions for a merchant

## Use Cases

- **Visual transaction lists**: See merchant logos in transaction views
- **Merchant identification**: Understand who you're paying
- **Spending analysis**: Group spending by merchant
- **Merchant search**: Find all transactions from a specific merchant
- **Better categorization**: Merchant info helps with automatic categorization

## How It Works

1. **Transaction arrives**: New transaction is created or imported
2. **Merchant extraction**: System identifies merchant from description
3. **Enrichment lookup**: Checks if merchant already exists
4. **Data fetching**: If new, fetches logo, description, website
5. **Storage**: Stores enrichment data in merchant record
6. **Display**: Logo and info appear in transaction lists

## Enrichment Priority

The system tries enrichment sources in order:

1. **Existing merchant**: If merchant exists, use stored data
2. **Firecrawl**: Best for comprehensive merchant info
3. **logo.dev**: Fallback for logo if Firecrawl didn't find one
4. **Bud API**: Additional metadata if configured
5. **Institution**: For bank-related transactions, use institution info

## Configuration

Enrichment requires API keys:

- **Firecrawl API key**: For web scraping and merchant info
- **logo.dev API key**: For logo retrieval
- **Bud API key** (optional): For additional enrichment

Without API keys, enrichment is skipped but transactions still work.

## Related Features

- **Transactions**: Enrichment data appears in transaction lists
- **Merchants**: Enrichment creates and updates merchant records
- **Rules**: Merchant information helps with categorization
- **Intelligence**: Merchant logos enhance financial reports
- **Processing Worker**: Background worker handles enrichment
