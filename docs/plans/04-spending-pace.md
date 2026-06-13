# Spending Pace Feature Plan

## Overview

Visual spending pace comparison showing how fast you're spending this month vs last month, with projected end-of-month totals. Helps users understand if they're on track without corporate "burn rate" language.

## Status: Not Started

## Dependencies

None - this is an independent feature.

## What We're Building

A visual comparison answering: **"Am I spending faster or slower than usual?"**

This is NOT "burn rate" (corporate language). It's a friendly visualization showing:

- How much you've spent so far this month
- How that compares to the same point last month
- Where you'll likely end up if the pace continues

---

## The Core Insight

On January 15th:

- "You've spent $1,847 so far this month"
- "By this point last month, you'd spent $1,523"
- "You're spending 21% faster than last month"
- "At this pace, you'll spend ~$3,694 by month end (vs $3,046 last month)"

---

## UI Design

### Spending Pace Card (Pulse Page)

```
┌─────────────────────────────────────────────────────────────┐
│  SPENDING PACE                                              │
│                                                             │
│  $1,847 spent so far                                        │
│                                                             │
│  ──────────────────●─────────────────  Jan 15               │
│  This month       ████████████░░░░░░░░░░░░░░  $1,847        │
│  Last month       █████████░░░░░░░░░░░░░░░░░  $1,523        │
│                                                             │
│  📈 21% faster than last month                              │
│                                                             │
│  Projected month end: ~$3,694                               │
│  (Last month total: $3,046)                                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Visual Variants

**On track (within 10%):**
```
  ✓ On pace with last month
  Projected: ~$3,100 (Last month: $3,046)
```

**Spending faster (>10% ahead):**
```
  📈 Spending 21% faster than last month  
  Projected: ~$3,694 (Last month: $3,046)
```

**Spending slower (>10% behind):**
```
  📉 Spending 15% slower than last month
  Projected: ~$2,589 (Last month: $3,046)
```

---

## Calculation Logic

### Core Metrics

```go
type SpendingPace struct {
    // Current month progress
    CurrentMonthSpent     int64     // Total expenses so far this month
    DayOfMonth            int       // e.g., 15
    DaysInMonth           int       // e.g., 31
    PercentOfMonthElapsed float64   // e.g., 0.48 (15/31)
    
    // Last month comparison
    LastMonthSamePoint    int64     // Expenses through day 15 of last month
    LastMonthTotal        int64     // Full last month expenses
    
    // Derived
    PacePercentage        float64   // e.g., 1.21 (21% faster)
    ProjectedMonthEnd     int64     // Extrapolated total
    
    // Status
    Status                string    // "on_track", "faster", "slower"
}
```

### Calculation

```go
func CalculateSpendingPace(ctx context.Context, ledgerID uuid.UUID) (*SpendingPace, error) {
    now := time.Now()
    dayOfMonth := now.Day()
    
    // Current month: expenses from 1st to today
    currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
    currentMonthSpent := getExpenses(ledgerID, currentMonthStart, now)
    
    // Last month: expenses from 1st to same day of month
    lastMonthStart := currentMonthStart.AddDate(0, -1, 0)
    lastMonthSameDay := time.Date(lastMonthStart.Year(), lastMonthStart.Month(), dayOfMonth, 23, 59, 59, 0, now.Location())
    lastMonthEnd := currentMonthStart.AddDate(0, 0, -1)
    
    lastMonthSamePoint := getExpenses(ledgerID, lastMonthStart, lastMonthSameDay)
    lastMonthTotal := getExpenses(ledgerID, lastMonthStart, lastMonthEnd)
    
    // Calculate pace
    var pacePercentage float64
    if lastMonthSamePoint > 0 {
        pacePercentage = float64(currentMonthSpent) / float64(lastMonthSamePoint)
    }
    
    // Project month end (simple linear extrapolation)
    daysInMonth := daysInMonth(now)
    projectedMonthEnd := int64(float64(currentMonthSpent) / float64(dayOfMonth) * float64(daysInMonth))
    
    // Determine status
    var status string
    switch {
    case pacePercentage > 1.10:
        status = "faster"
    case pacePercentage < 0.90:
        status = "slower"
    default:
        status = "on_track"
    }
    
    return &SpendingPace{...}, nil
}
```

### What Counts as "Expenses"

```sql
-- Expenses = debits to expense accounts, excluding transfers
SELECT COALESCE(SUM(e.amount_cents), 0)
FROM entries e
JOIN transactions t ON e.transaction_id = t.id
JOIN accounts a ON e.account_id = a.id
WHERE t.ledger_id = $1
  AND t.date >= $2 AND t.date <= $3
  AND a.type = 'expense'
  AND t.is_transfer = false
  AND a.name NOT IN ('Internal Transfer')
```

---

## Edge Cases

### Early in Month (Day 1-3)

Not enough data for meaningful comparison. Show:
```
  "Check back in a few days for spending pace comparison"
```

### First Month of Use

No last month data. Show:
```
  "$1,847 spent so far this month"
  "We'll show pace comparison next month"
```

### Month with Unusual Expenses

User bought a car last month. Pace looks artificially slow.
- **Don't try to be too smart** - just show the data
- User understands context

### Different Days in Month

February (28) vs January (31):
- Normalize by percentage of month, not day number
- "50% through the month" works regardless of month length

---

## Files to Create

```
internal/pulse/spending_pace.go       # Calculation logic
internal/views/pulse/pace_card.go     # UI component
```

---

## Complexity Assessment

**This is a relatively simple feature:**

- Single SQL query (expenses by date range)
- Basic math (ratios, projections)
- UI component (progress bars)
- No new tables needed
- No background jobs
- Stateless calculation

**Estimate: 1-2 days implementation**

---

## Todos

- [ ] Build spending pace calculator in internal/pulse/spending_pace.go
- [ ] Write SQL query for expenses by date range (excluding transfers)
- [ ] Create spending pace card component with dual progress bars
- [ ] Integrate pace card into Pulse page
- [ ] Handle edge cases: early month, first month, no data
