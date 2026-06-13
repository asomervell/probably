# Feature Plans

This directory contains detailed implementation plans for major features to deliver the "Intelligence" vision for Probably.

## Background

The marketing site promises **"Know where you stand. See where you're going."** but the app currently only delivers the first half. These plans bridge the gap by adding forward-looking, conversational financial intelligence.

## Plans

| # | Plan | Description | Status |
|---|------|-------------|--------|
| 0 | [Overview](./00-overview-vision-gap.md) | Vision gap analysis and roadmap | Reference |
| 1 | [Page Restructure](./01-page-restructure.md) | Rename pages, create Pulse | Not Started |
| 2 | [Subscription Tracker](./02-subscription-tracker.md) | Recurring detection + price alerts | Not Started |
| 3 | [Left to Spend](./03-left-to-spend.md) | Hero metric with upcoming bills | Not Started |
| 4 | [Spending Pace](./04-spending-pace.md) | This month vs last month | Not Started |
| 5 | [AI Chat](./05-ai-chat.md) | Conversational queries with sql.js | Not Started |

## Implementation Order

```
1. Page Restructure       ← Do first (creates Pulse placeholder, fixes naming)
2. Subscription Tracker   ← Builds recurring detection engine
3. Left to Spend          ← Uses recurring detection, fills Pulse hero
4. Spending Pace          ← Independent, adds to Pulse
5. AI Chat                ← Most complex, adds to Pulse
```

## Key Architectural Decisions

- **Recurring detection** is built as part of Subscription Tracker, then reused by Left to Spend
- **Spendable accounts**: Checking = yes by default, Savings = no by default, user can override
- **AI Chat uses sql.js**: LLM generates SQL that runs client-side in WASM SQLite
- **Family-friendly language**: No "burn rate" or corporate terminology

## Design Principles

1. **Forward-looking**: Answer "what's coming" not just "what happened"
2. **Zero setup**: Infer recurring bills automatically, let users correct
3. **Conversational**: Let users ask questions in natural language
4. **Privacy-first**: SQL runs client-side, data doesn't leave the browser
