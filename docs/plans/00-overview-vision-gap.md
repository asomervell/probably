# Intelligence Vision Gap Analysis

## Overview

Analysis of the gap between Probably's marketing promise and current implementation, with a roadmap to build the forward-looking, conversational financial intelligence that differentiates it from traditional banking apps.

## The Core Problem

Your website promises **"Know where you stand. See where you're going."** but the app only delivers the first half.

**Current Intelligence page** = Traditional accounting statements (Balance Sheet + P&L)

**Current Insights page** = AI-generated observations (which is closer to "intelligence")

The user is right: these should swap. But more importantly, you're missing the features that would truly differentiate Probably from "a banking app of the last 5 years."

---

## The Vision vs Reality Gap

### What You Promise

From `docs/features/benefits.md` and `internal/handlers/home.go`:

- "See where you're going" - forward-looking guidance
- "Calculates your real-time position, giving you the confidence that comes from knowing exactly where you stand"
- "Tells you if a subscription has quietly increased in price"
- "Can I afford the vacation?"
- "No more 3am wondering... you know exactly where you stand—net worth, burn rate, upcoming expenses"
- AI that "acts as a proactive analyst rather than a passive ledger"

### What You've Built

- **Intelligence**: Balance sheet + P&L by date range (backward-looking accounting)
- **Insights**: Batch AI reports with spending alerts, trends, recommendations

### What's Missing (The Differentiators)

See individual feature plans for details.

---

## Feature Plans

| # | Plan | Description | Dependencies |
|---|------|-------------|--------------|
| 1 | [Page Restructure](./01-page-restructure.md) | Rename pages, create Pulse placeholder | None |
| 2 | [Subscription Tracker](./02-subscription-tracker.md) | Recurring detection + price change alerts | None |
| 3 | [Left to Spend](./03-left-to-spend.md) | Hero metric with upcoming bills | Subscription Tracker |
| 4 | [Spending Pace](./04-spending-pace.md) | This month vs last month comparison | None |
| 5 | [AI Chat](./05-ai-chat.md) | Conversational queries with sql.js | None |

---

## Implementation Order

Based on dependencies and value delivery:

```
1. Page Restructure          ← Do first (creates foundation)
   └─► Creates Pulse placeholder
   └─► Renames pages correctly
   
2. Subscription Tracker      ← Foundation for recurring detection
   └─► Builds recurring detection engine
   └─► Adds price change alerts to Intelligence
   
3. Left to Spend + Upcoming Bills  
   └─► Reuses recurring detection
   └─► Fills Pulse page with hero metric
   
4. Spending Pace             ← Independent, quick win
   └─► Adds to Pulse page
   
5. AI Chat                   ← Most complex, high impact
   └─► Adds to Pulse page (and floating)
```

---

## Summary

The current app is excellent at **looking backward** (what happened). The vision promises **looking forward** (what's coming, what can I do). The missing piece is the proactive, forward-looking layer that transforms Probably from "a fancier mint" to "a financial co-pilot."

The "Left to Spend" concept is the unlock—it shifts the user's mental model from accounting to planning.
