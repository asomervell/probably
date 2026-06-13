# MCP UI Bundles

React/TypeScript components for ChatGPT App widgets using OpenAI Apps SDK UI.

## Setup

```bash
cd cmd/ui-bundles
npm install
```

## Development

```bash
npm run dev
```

## Build

```bash
npm run build
```

This will output ES modules to `dist/` that can be:
- Served via CDN
- Embedded inline in HTML templates
- Referenced via script tags in HTML templates

## Components

Each component reads data from `window.openai.toolOutput` (provided by ChatGPT runtime) and renders using Apps SDK UI components.

- `SpendingSummary.tsx` - Spending breakdown by category
- `AccountBalances.tsx` - Net worth and account balances
- `AskQuestion.tsx` - AI-powered financial insights
- `SpendingTrends.tsx` - Time series spending trends
- `RecurringPatterns.tsx` - Subscriptions and recurring bills
- `SearchTransactions.tsx` - Transaction search results
- `FinancialOverview.tsx` - Dashboard overview

## Usage in HTML Templates

HTML templates in `static/mcp-ui/` can reference these bundles:

```html
<div id="root"></div>
<script type="module" src="https://cdn.probably.money/mcp-ui/spending-summary.js"></script>
```

Or embed inline (after building):

```html
<div id="root"></div>
<script type="module">
  // Inline JS bundle content here
</script>
```
