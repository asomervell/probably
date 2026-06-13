# Page Restructure Plan

## Overview

Rename and reorganize the main app pages to better reflect their purpose. Current "Intelligence" (accounting statements) becomes "Statements", current "Insights" (AI analysis) becomes "Intelligence", and a new "Pulse" page becomes the hero dashboard.

## Status: COMPLETE

## The Problem

Current naming doesn't match what the pages actually do:

| Current Name | What It Actually Is | User Expectation |
|--------------|---------------------|------------------|
| **Intelligence** | Balance Sheet + P&L (accounting reports) | Something smart/AI |
| **Insights** | AI-generated observations | Generic analytics |

The word "Intelligence" implies AI/smart analysis, but it's just financial statements. Meanwhile, the actual AI stuff is hidden under "Insights".

---

## The Fix

```
BEFORE                          AFTER
──────                          ─────
Intelligence ─────────────────► Statements
(Balance Sheet, P&L)            (same content, honest name)

Insights ─────────────────────► Intelligence  
(AI observations, reports)      (the AI stuff IS intelligence)

(new) ────────────────────────► Pulse
                                (Left to Spend, Upcoming Bills,
                                 Spending Pace, AI Chat)
```

---

## New Navigation Structure

```
┌─────────────────────────────────────────────────────────────┐
│  [Logo]  Pulse  Transactions  Accounts  Statements  Intelligence  │
└─────────────────────────────────────────────────────────────┘

Pulse         → Forward-looking dashboard (NEW hero page)
Transactions  → Transaction list (unchanged)
Accounts      → Account list (unchanged)  
Statements    → Balance Sheet + P&L (renamed from Intelligence)
Intelligence  → AI Insights + Reports (renamed from Insights)
```

### Mobile/Compact Nav

```
[Pulse] [Txns] [Accts] [More ▼]
                         └─► Statements
                             Intelligence
                             Settings
                             ...
```

---

## Page Purposes (Clarified)

### Pulse (NEW - Hero Page)

**Purpose**: "How am I doing right now?"

- Left to Spend widget
- Upcoming Bills (next 3-5)
- Spending Pace comparison
- AI Chat entry point
- Quick account balances

**URL**: `/pulse` or `/` (make it the default landing)

### Statements (renamed from Intelligence)

**Purpose**: "Show me the accounting reports"

- Balance Sheet (Assets vs Liabilities, Net Worth)
- Profit & Loss (Income vs Expenses by category)
- Date range selection
- Export options

**URL**: `/statements` (redirect `/intelligence` → `/statements`)

### Intelligence (renamed from Insights)  

**Purpose**: "What should I know about my finances?"

- AI-generated insights (spending alerts, trends, recommendations)
- Monthly/quarterly reports
- Subscription price alerts
- Anomaly detection

**URL**: `/intelligence` (redirect `/insights` → `/intelligence`)

---

## Implementation Steps

### Step 1: Create Pulse Page

- New handler: `internal/handlers/pulse.go`
- New route: `/pulse`
- Placeholder content initially (will be filled by other features)

### Step 2: Rename Intelligence → Statements

- Rename handler file: `intelligence.go` → `statements.go`
- Update handler function: `Intelligence()` → `Statements()`
- Update route: `/intelligence` → `/statements`
- Add redirect: `/intelligence` → `/statements` (preserve old links)
- Update page title and header text
- Update nav link

### Step 3: Rename Insights → Intelligence

- Rename handler file: `insights.go` → Leave as-is (avoid confusion)
- Update routes: `/insights/*` → `/intelligence/*`
- Add redirects for old URLs
- Update page titles and headers
- Update nav links

### Step 4: Update Navigation

- Reorder nav: Pulse first
- Update active states
- Mobile nav adjustments

### Step 5: Update Internal Links

- Any links to `/intelligence` → `/statements`
- Any links to `/insights` → `/intelligence`
- Homepage CTA buttons

### Step 6: Set Default Landing

- Logged-in users land on `/pulse` instead of `/intelligence`
- Update homepage "Go to Dashboard" link

---

## File Changes

### Renamed/New Files

```
internal/handlers/
├── pulse.go           # NEW - Pulse page handler
├── statements.go      # RENAMED from intelligence.go
└── insights.go        # KEEP name, but serves /intelligence routes

internal/views/
├── pulse/             # NEW - Pulse page components
├── statements/        # RENAMED from intelligence/
└── insights/          # KEEP name
```

### Modified Files

```
internal/handlers/handlers.go    # Update route registration
internal/views/layouts/base.go   # Update navigation
internal/handlers/home.go        # Update CTA links
```

---

## URL Redirects

```go
// In router setup
r.Get("/intelligence", redirectTo("/statements"))
r.Get("/insights", redirectTo("/intelligence"))
r.Get("/insights/*", redirectToPattern("/intelligence/*"))
```

Keep redirects permanently - users may have bookmarks, and search engines may have indexed old URLs.

---

## Database Changes

**None required.** This is purely a presentation/routing change.

---

## Todos

- [ ] Create new Pulse page handler with placeholder content
- [ ] Rename Intelligence handler/routes to Statements
- [ ] Update Insights routes to serve under /intelligence
- [ ] Add redirects from old URLs to new URLs
- [ ] Update navigation component with new labels and order
- [ ] Update all internal links (home CTAs, etc.)
- [ ] Set Pulse as default landing page for logged-in users
- [ ] Update documentation to reflect new page names
