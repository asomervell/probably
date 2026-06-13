# Backup & Restore

Backup and restore allows you to export all your financial data and import it elsewhere. This is essential for data portability, disaster recovery, and migrating between instances.

## Overview

The backup system exports all your financial data into a single ZIP file that can be imported into any Probably instance. This gives you complete control over your data.

## Features

### Data Export

Export all your financial data:

- **Accounts**: All account information (names, types, balances, institution data)
- **Transactions**: All transactions with entries, dates, amounts, descriptions
- **Tags**: Complete tag hierarchy with colors and relationships
- **Rules**: All categorization rules with instructions and priorities
- **Merchants**: Merchant information and associations
- **Transfer matches**: Confirmed transfer links between transactions

### Export Format

Backups are ZIP files containing:

- **JSON files**: One file per data type (accounts.json, transactions.json, etc.)
- **Structured data**: Easy to read and parse
- **Complete**: All relationships and references preserved
- **Portable**: Can be imported into any Probably instance

### Data Import

Import backups to restore or migrate data:

- **Full restore**: Replaces all data in the target ledger
- **New ledger creation**: Import creates a new ledger with the data
- **Data validation**: System validates data integrity during import
- **Progress tracking**: See what's being imported
- **Error handling**: Clear error messages if import fails

### Security

Backups exclude sensitive information:

- **Teller tokens**: Bank connection tokens are NOT included (security)
- **User credentials**: Authentication data is not exported
- **You'll need to reconnect**: Bank accounts must be reconnected after import

### Import Statistics

After import, see what was restored:

- Number of accounts imported
- Number of transactions imported
- Number of tags imported
- Other data counts

## Use Cases

### Data Portability

- Export your data to move to a different Probably instance
- Keep a local copy of all your financial data
- Migrate between self-hosted and cloud instances

### Disaster Recovery

- Regular backups protect against data loss
- Restore from backup if something goes wrong
- Peace of mind knowing your data is safe

### Testing & Development

- Export production data for testing
- Create test ledgers with real data structure
- Safely experiment without affecting production data

### Data Migration

- Move data between accounts
- Consolidate multiple ledgers
- Transfer data to a new user account

## Best Practices

### Regular Backups

- Export backups monthly or quarterly
- Store backups in multiple locations (cloud + local)
- Keep backups for at least one year
- Name backups with dates for easy identification

### Before Major Changes

- Export backup before bulk operations
- Backup before deleting large amounts of data
- Export before major system updates

### After Import

- Verify data integrity after import
- Check account balances match expectations
- Review transaction counts
- Reconnect bank accounts if needed

## Import Process

When importing a backup:

1. **Upload backup file**: Select the ZIP file to import
2. **Validation**: System checks file format and structure
3. **Data import**: All data is imported into a new ledger
4. **Verification**: Import statistics are shown
5. **Review**: Check that everything imported correctly

## Limitations

- **Teller connections**: Must be reconnected after import (tokens not included)
- **File size**: Large backups may take time to import
- **One-way**: Import replaces existing data (no merge)
- **User accounts**: Import creates data for the importing user

## Related Features

- **Accounts**: All accounts are included in backups
- **Transactions**: Complete transaction history is exported
- **Tags**: Tag hierarchy is preserved
- **Rules**: Categorization rules are included
- **Settings**: Some settings may need to be reconfigured
