# Internal Feature Catalog (Commit-Audited)

This document is an internal, customer-outcome-first feature catalog for Probably.

Scope:
- Commit history reviewed: `229` commits (`git log --reverse`)
- Focus: customer benefits, user workflows, and practical "how to use" guidance
- Includes active features, evolving features, and deprecated/removed features

## How This Should Be Used Internally

- Use this as the source of truth for "what value users get" from each feature.
- Use the "How users use it" sections for onboarding flows, release notes, and support docs.
- Use "Commit landmarks" when tracing why a behavior exists.
- Update this file when major product behavior changes (not just code refactors).

## Core Ledger Foundations

### 1) Double-Entry Ledger, Accounts, and Transactions
- **Customer benefit:** Users get trustworthy numbers. Net worth and spending reports stay grounded in accounting reality instead of guesswork.
- **Who benefits most:** People tracking multiple accounts (checking, cards, loans, assets) who want accurate month-end answers.
- **How users use it:**
  1. Create or connect accounts.
  2. Import or add transactions.
  3. Review account balances and transaction entries.
  4. Use reports/chat knowing entries are balanced.
- **Commit landmarks:** `1b104e7`, `dfa760c`, `71dc0fd`, `9c6cacc`

### 2) Transfers and P2P Handling
- **Customer benefit:** Money moved between your own accounts does not inflate spending or income.
- **Who benefits most:** Users paying credit cards, moving savings, reimbursing friends/family, or splitting bills.
- **How users use it:**
  1. Import normal account activity.
  2. Let transfer/P2P matching run automatically.
  3. Confirm linked movements in transaction views.
  4. Trust that reports exclude internal movement noise.
- **Commit landmarks:** `74e2d2c`, `0795a01`, `33b9d1b`, `9c6cacc`

## Bank Connectivity

### 3) Multi-Provider Bank Connections (Teller, Plaid, Akahu)
- **Customer benefit:** Users can connect more institutions and reduce manual entry.
- **Who benefits most:** Users with cross-country or mixed-institution banking needs.
- **How users use it:**
  1. Go to connected accounts.
  2. Select provider and authenticate.
  3. Choose accounts to import.
  4. Monitor connection health/status banners.
- **Commit landmarks:** `0ac1d6f`, `c770f93`, `6799783`, `5afca98`, `90c80de`

### 4) Teller Reliability Improvements
- **Customer benefit:** Fewer sync breakages, clearer failure handling, better recovery when credentials/config are wrong.
- **How users use it:** Reconnect when prompted, then let scheduled sync resume.
- **Commit landmarks:** `1800096`, `eaffe8d`, `0a65f45`, `c770f93`

### 5) Plaid Improvements and UX
- **Customer benefit:** Better first-time linking success, clearer errors, less confusion around duplicates and connection state.
- **How users use it:**
  1. Start Plaid Link from connected accounts.
  2. Resolve any surfaced toast/status errors.
  3. Use account status banners to fix broken links quickly.
- **Commit landmarks:** `d2a757b`, `6799783`, `5afca98`, `fc6ff02`, `c5f4140`

### 6) Akahu Integration (NZ) + Personal App Support
- **Customer benefit:** NZ users can connect banks in-region; personal app path reduces setup friction.
- **How users use it:** Connect via Akahu flow, then review imported accounts/transactions like other providers.
- **Commit landmarks:** `e2edf85`, `518ee05`

## Data Quality and Transaction Understanding

### 7) Enrichment and Entity Model (Merchant Evolution)
- **Customer benefit:** Messy raw statements become readable transactions with recognizable entities/logos/context.
- **Who benefits most:** Users who want "what was this charge?" answered quickly.
- **How users use it:**
  1. Sync/import data.
  2. Let enrichment run in background.
  3. Scan cleaner names/logos/entities in transaction lists.
  4. Use search and filters with normalized entity names.
- **Commit landmarks:** `c0cf85d`, `e3063cd`, `0cdb1c1`, `8d70991`, `35efd45`

### 8) Pattern Detection (Recurring Behaviors)
- **Customer benefit:** Users get proactive visibility into subscriptions, recurring payments, and behavior patterns.
- **How users use it:**
  1. Keep syncing transactions.
  2. Review detected patterns and confidence/reasoning.
  3. Use output to catch waste, forecast bills, and tune cash flow.
- **Commit landmarks:** `76831af`, `70d5d19`, `e424600`, `fcf64ca`

### 9) My Life Context for Better AI Decisions
- **Customer benefit:** Categorization and transfer interpretation become more personal and accurate (e.g., family transfers, employer payments, asset-related spend).
- **How users use it:**
  1. Define people/relationships/assets in My Life.
  2. Sync new transactions.
  3. Let AI use context for better categorization and interpretation.
- **Commit landmarks:** `f345266`, `70c2e39`

## Intelligence, Insights, and AI Interfaces

### 10) Intelligence/Pulse Dashboard + Spending Pace
- **Customer benefit:** Faster understanding of financial position and burn pace without spreadsheet work.
- **How users use it:**
  1. Open Pulse/Intelligence views.
  2. Set relevant date ranges.
  3. Track assets, liabilities, and spending pace trends.
  4. Use trends for near-term decision making.
- **Commit landmarks:** `cb87e4d`, `7ebcb11`, `13a96a5`

### 11) AI Insights
- **Customer benefit:** Users get narrative analysis (signals, changes, possible anomalies) instead of only raw numbers.
- **How users use it:** Open insights, review highlights, then jump into underlying transactions for validation/action.
- **Commit landmarks:** `e3063cd`, `2575245` (release/doc reinforcement)

### 12) AI Chat (v1 to v2)
- **Customer benefit:** Natural-language access to personal financial data with follow-up context, charts, and faster retrieval.
- **How users use it:**
  1. Ask plain-language financial questions.
  2. Use suggested questions to explore.
  3. Continue threads for deeper follow-ups.
  4. Validate with tables/charts and act.
- **Commit landmarks (major):**
  - Foundation and UI: `3ff527a`, `47afbdf`, `e1b04c8`, `fbbff34`
  - Follow-ups and performance: `9922326`, `a713cb2`, `0dfa88b`, `09552a3`, `43ab0ba`
  - Chat v2 persistence/embeddings/streaming: `74f2151`
  - Markdown/voice quality fixes: `f1ed4fd`, `0f372bc`

### 13) ChatGPT App Integration (MCP + OAuth)
- **Customer benefit:** Users can query their financial data from ChatGPT and interact through richer surfaces.
- **How users use it:**
  1. Complete setup/auth flow.
  2. Connect Probably in ChatGPT.
  3. Ask spending, balance, and trend questions from ChatGPT.
  4. Open widgets/details as needed.
- **Commit landmarks:** `34d9f7e`, `62127bd`, `c449955`, `ec98444`

## Access, Identity, and Multi-Entity Controls

### 14) Passkey Authentication
- **Customer benefit:** Faster and more secure sign-in with reduced password friction.
- **How users use it:** Enroll a passkey from account/security settings, then authenticate with device biometrics.
- **Commit landmarks:** `377cc05`

### 15) RBAC and Multi-Entity Permissions
- **Customer benefit:** Better control over who can view/act on specific financial entities.
- **How users use it:** Assign roles/permissions per entity and validate access boundaries by account context.
- **Commit landmarks:** `d987e0b`

### 16) Legal, Policy, and Trust Surface
- **Customer benefit:** Clearer transparency around policies and product terms.
- **How users use it:** Review legal links/policy pages from footer and docs before onboarding or billing decisions.
- **Commit landmarks:** `2ebf365`, `6ca74ca`

## Billing and Commercial Model

### 17) Subscriptions + 45-Day Free Trial
- **Customer benefit:** Users can evaluate the full product before committing, then manage billing predictably.
- **How users use it:**
  1. Start trial.
  2. Use full product during trial window.
  3. Upgrade/manage subscription from settings.
- **Commit landmarks:** `734f8a5`, `6cf8c0e`, `b66c0fa`, `49d0fcd`

## UX, Performance, and Quality-of-Life

### 18) Navigation, Homepage, and Product Positioning
- **Customer benefit:** Clearer orientation for first-time users and easier movement through core workflows.
- **How users use it:** Enter via homepage, follow clearer nav to pulse/statements/settings/blog.
- **Commit landmarks:** `70ea40f`, `a0e80f8`, `cb87e4d`, `f345266`

### 19) HTMX/CSS-Only and Lazy Loading Improvements
- **Customer benefit:** Faster-feeling pages, less UI fragility, smoother interaction on heavier screens.
- **How users use it:** Browse transactions and connected-account screens with reduced latency and cleaner interactions.
- **Commit landmarks:** `eeefb27`, `84d7999`

### 20) Theming and Visual Clarity
- **Customer benefit:** Improved readability and aesthetic coherence (including brick-basil redesign).
- **How users use it:** Continue normal workflows with clearer, more consistent visual hierarchy.
- **Commit landmarks:** `5fb4d39`, `70d535f`

## Feature Lifecycle Notes (Important for Messaging)

- **Bud integration was added, then removed.**
  - Added: `c0cf85d`
  - Removed (breaking): `605ec36`
  - Internal guidance: do not position Bud as active functionality.

- **Rules-based categorization is legacy/deprecated in docs.**
  - Product direction shifted toward pattern detection + My Life context + entity-first logic.
  - Internal guidance: when speaking with users, frame this as an evolution toward higher-quality automation.

## User Journey Playbook (Cross-Feature)

Recommended end-user onboarding sequence:
1. Create account and sign in (passkey if available).
2. Connect banks (provider best suited to region/institution).
3. Verify imported accounts and recent transactions.
4. Add My Life context for personalization.
5. Review Pulse/Intelligence baseline.
6. Use AI Chat for immediate Q&A.
7. Check pattern detections and recurring costs.
8. Configure billing after trial if continuing.

## Commit Audit Notes

- I reviewed the full commit timeline and synthesized customer-facing impact even for non-feature commits.
- Infra/reliability commits (deployment, logging, linting, error handling, migrations, CI) were interpreted for user impact as:
  - higher uptime,
  - faster issue recovery,
  - safer releases,
  - more predictable feature behavior.
- Documentation-only commits informed feature framing but did not by themselves create net-new capability.

## Maintainer Checklist for Updating This File

When adding a new feature section:
1. State user outcome first (one sentence).
2. Add "How users use it" in concrete steps.
3. Include 1-5 commit landmarks.
4. Note whether feature is active, changed, deprecated, or removed.
5. Update onboarding flow if the first-run path changes.
