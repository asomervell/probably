# Probably Features

This directory contains documentation for all customer-facing features in Probably, a personal finance application.

For a high-level overview of why these features matter, see **[The Benefits of Probably](benefits.md)**.

## Financial Intelligence

### [Intelligence Dashboard](intelligence.md)
Comprehensive financial overview with balance sheet (assets/liabilities) and profit & loss (income/expenses) reporting. View by date range with category breakdowns.

### [AI Financial Insights](insights.md)
AI-generated observations about your finances - spending alerts, trends, recommendations, and anomalies. Monthly and quarterly reports with actionable insights.

### [AI Chat](ai-chat.md)
Conversational interface for asking questions about your finances in natural language. Get instant answers with SQL queries generated automatically, presented as tables, charts, and natural language summaries.

### [ChatGPT App Integration](chatgpt-app.md)
Access your financial data through ChatGPT's conversational interface. Ask questions, view spending summaries, account balances, and financial trends using natural language. Includes rich, interactive widgets for visualizing financial data.

## Automation

### [Pattern Detection](patterns.md)
Automatic detection of recurring financial patterns - subscriptions, bills, salary, and regular payments. Uses entity-first AI analysis to identify multiple patterns per merchant with confidence scores and reasoning.

### [Transfer Matching](transfers.md)
Automatic detection and linking of money transfers between accounts. Prevents transfers from appearing as income/expenses in reports.

### [Multi-Provider Bank Connections](bank-connections.md)
Secure, read-only bank account connections via multiple providers (Teller, Plaid, Akahu). Automatic transaction import and scheduled syncing to keep data current. Unified interface for connecting accounts from different providers.

### [Teller Bank Integration](teller-integration.md)
Teller-specific integration details. See [Multi-Provider Bank Connections](bank-connections.md) for the unified bank connection system.

### [Plaid Bank Integration](plaid-integration.md)
Plaid-specific integration details including connection status monitoring, institution logos, and account management. See [Multi-Provider Bank Connections](bank-connections.md) for the unified bank connection system.

### [Transaction Enrichment](transaction-enrichment.md)
Automatic enrichment of transactions with merchant logos, descriptions, websites, and standardized names. Makes transaction lists visual and informative.

### [Merchants](merchants.md)
Automatic merchant identification and management. Search transactions by merchant, view merchant information, and track spending patterns.

## Core Features

### [Accounts](accounts.md)
Manage your financial accounts - checking, savings, credit cards, loans, and more. Supports both manual account creation and automatic bank account integration.

### [Transactions](transactions.md)
Record and manage all financial transactions using double-entry bookkeeping. Automatic import from banks, manual entry, tagging, and transaction review.

### [Tags](tags.md)
Hierarchical categorization system for organizing transactions. Create custom categories, use default tags, and organize spending with colors and parent-child relationships.

### [My Life](my-life.md)
Define the important people, work relationships, and assets in your life. This context helps the AI better understand and categorize your transactions - identifying transfers to family, salary from employers, and expenses related to vehicles or pets.

### [AI Categorization Rules](rules.md) *(Deprecated)*
Natural language rules that automatically categorize transactions using AI. This feature has been superseded by Pattern Detection and My Life relationships.

### [Multi-Model Orchestrator](multi-model-orchestrator.md)
Advanced AI system that intelligently routes and executes LLM tasks using multiple models and execution strategies. Provides cost optimization, quality assurance, and flexible execution patterns for AI-powered features.

## Data Management

### [Backup & Restore](backup-restore.md)
Export all financial data to a portable ZIP file. Import backups to restore data or migrate between instances. Essential for data portability and disaster recovery.

## Account & Billing

### [Subscriptions & Free Trial](subscriptions.md)
45-day free trial to experience all features. Flexible subscription plans with automatic renewal. Manage your subscription and billing from settings.

## Feature Overview

| Feature | Purpose | Automation Level |
|---------|---------|------------------|
| Accounts | Track where your money lives | Manual + Automatic (Bank Connections) |
| Transactions | Record financial events | Manual + Automatic (Bank Connections) |
| Tags | Categorize transactions | Manual + Automatic (Rules) |
| My Life | Personal context | Manual (user-defined) |
| Rules | Auto-categorization | Automatic (AI) *(Deprecated)* |
| Intelligence | Financial reporting | Automatic (calculated) |
| Insights | Financial observations | Automatic (AI) |
| AI Chat | Natural language queries | Automatic (AI) |
| ChatGPT App | ChatGPT integration | Automatic (AI) |
| Orchestrator | AI task execution | Automatic (AI) |
| Patterns | Recurring expense detection | Automatic (AI) |
| Transfers | Link account movements | Automatic (detection) |
| Bank Connections | Multi-provider bank syncing | Automatic (scheduled) |
| Teller | Teller provider integration | Automatic (scheduled) |
| Enrichment | Add merchant metadata | Automatic (background) |
| Merchants | Manage business entities | Automatic (creation) |
| Backup | Data portability | Manual (on-demand) |
| Subscriptions | Account access & billing | Manual (trial + subscription) |

## Getting Started

New users should start with:

1. **Accounts**: Create or connect your bank accounts
2. **Tags**: Add default tags or create your own categories
3. **My Life**: Define your important people, work, and assets for better AI context
4. **Transactions**: Let bank connections import transactions automatically, or enter manually
5. **Intelligence**: Review your financial overview
6. **AI Chat**: Ask questions about your finances in natural language

## Feature Dependencies

- **Transactions** require **Accounts**
- **Tags** are applied to **Transactions**
- **My Life** provides context for **AI Categorization** and **Transfers**
- **Rules** assign **Tags** to **Transactions** *(Deprecated)*
- **Intelligence** uses **Transactions** and **Tags**
- **Insights** analyze **Transactions** and **Tags**
- **AI Chat** queries **Transactions**, **Accounts**, and **Tags**
- **ChatGPT App** uses **AI Chat** tools and **Transactions**, **Accounts**, and **Tags**
- **Orchestrator** powers **Rules**, **Insights**, **AI Chat**, and categorization
- **Transfers** link **Transactions** between **Accounts**
- **Enrichment** enhances **Transactions** with **Merchant** data
- **Bank Connections** (Teller, Plaid, Akahu) create **Accounts** and **Transactions**

## Related Documentation

- [Release Notes](../releases/README.md) - See what's new in each version
- [Project README](../../README.md) - Setup and installation
