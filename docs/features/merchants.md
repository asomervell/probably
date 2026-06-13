# Merchants

Merchants represent the businesses and entities you transact with. The system automatically creates merchant records from your transactions and enriches them with logos, descriptions, and other metadata.

## Overview

Every transaction is associated with a merchant. Merchants are automatically created and managed, making it easy to:

- See all transactions from a specific merchant
- View merchant information and logos
- Search transactions by merchant
- Understand your spending patterns by merchant

## Features

### Automatic Merchant Creation

Merchants are created automatically when:

- Transactions are imported from banks
- You manually create transactions
- The system identifies a new merchant name

### Merchant Identification

The system identifies merchants from transaction descriptions:

- **Name extraction**: Parses merchant names from payment processor descriptions
- **Standardization**: Normalizes names (handles variations like "PETSTOCK" vs "Petstock")
- **Fuzzy matching**: Links similar merchant names to the same record
- **Institution merchants**: Creates merchants for financial institutions

### Merchant Enrichment

Merchants are automatically enriched with:

- **Logos**: Visual logos downloaded from enrichment services
- **Descriptions**: What the merchant does (e.g., "Pet supply retailer")
- **Websites**: Official merchant websites
- **Metadata**: Additional information from enrichment APIs

### Merchant Search

Find transactions by merchant:

- Search by merchant name
- See all transactions from a merchant
- Filter transactions by merchant
- View merchant spending totals

### Merchant Verification

You can verify merchant assignments:

- **User verification**: Mark merchants as verified
- **Prevents override**: Verified merchants won't be changed by automatic enrichment
- **Manual correction**: Fix incorrect merchant assignments

### Merchant Display

Merchants appear throughout the app:

- **Transaction lists**: Merchant logos and names
- **Transaction details**: Full merchant information
- **Merchant pages**: View all transactions for a merchant
- **Intelligence reports**: Merchant logos in financial overviews

## Merchant Records

Each merchant has:

- **Display name**: The name shown in the UI
- **Slug**: URL-friendly identifier
- **Logo URL**: Path to merchant logo (stored in cloud storage)
- **Website**: Official merchant website
- **Description**: What the merchant does
- **User verified**: Whether you've verified this merchant
- **Transaction count**: How many transactions use this merchant

## Use Cases

- **Spending analysis**: See how much you spend at each merchant
- **Merchant lookup**: Find all transactions from a specific business
- **Visual identification**: Merchant logos make transactions easier to recognize
- **Spending patterns**: Understand which merchants you frequent most
- **Budget tracking**: Monitor spending at specific merchants

## Merchant Management

While merchants are mostly automatic, you can:

- **Search merchants**: Find merchants by name
- **View merchant details**: See all information about a merchant
- **Verify merchants**: Mark merchants as correct
- **View transactions**: See all transactions for a merchant

## Best Practices

- **Verify important merchants**: Mark frequently-used merchants as verified
- **Review merchant assignments**: Check that merchants are correctly identified
- **Use merchant search**: Find transactions quickly by merchant name
- **Monitor merchant spending**: Use merchant data for budgeting

## Related Features

- **Transactions**: Every transaction links to a merchant
- **Transaction Enrichment**: Merchants are enriched with logos and info
- **Rules**: Merchant names help with automatic categorization
- **Intelligence**: Merchant logos appear in financial reports
- **Search**: Find transactions by merchant name
