# Left to Spend Feature Plan

## Overview

Complete implementation plan for the "Left to Spend" feature - the hero metric showing how much money is safe to spend after accounting for upcoming detected bills. Depends on the recurring detection engine built in the Subscription Tracker plan.

## Status: Not Started

## Dependencies

- **Subscription Tracker** (Plan 02) - provides the recurring detection engine

## What We're Building

A forward-looking financial metric that answers: **"How much can I safely spend right now?"**

```
Left to Spend = Checking Account Balances - Upcoming Bills (next occurrence)
```

**Example:**
- Checking accounts: $3,200
- Upcoming in next 30 days: Netflix ($23), Electric (~$140), Rent ($1,500), Insurance ($180)
- **Left to Spend: $1,357**

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│                     LEFT TO SPEND                            │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────────┐    ┌─────────────────────────────────┐  │
│  │ Spendable       │    │ Recurring Detection Engine      │  │
│  │ Accounts        │    │ (from Subscription Tracker)     │  │
│  │                 │    │                                 │  │
│  │ - Checking: YES │    │ - Analyze transaction history   │  │
│  │ - Savings: NO   │    │ - Detect patterns by merchant   │  │
│  │ - User override │    │ - Predict next occurrence       │  │
│  └────────┬────────┘    └───────────────┬─────────────────┘  │
│           │                             │                    │
│           ▼                             ▼                    │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              Left to Spend Calculation                  │ │
│  │                                                         │ │
│  │  SUM(spendable_accounts.balance)                        │ │
│  │  - SUM(upcoming_bills.expected_amount)                  │ │
│  │  = LEFT_TO_SPEND                                        │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

---

## Component 1: Spendable Account Designation

### Smart Default Logic

```go
func isSpendableByDefault(account *Account) bool {
    // Checking accounts are spendable
    if account.TellerSubtype == "checking" {
        return true
    }
    // Savings are not spendable by default
    if account.TellerSubtype == "savings" {
        return false
    }
    // Credit cards are not "spendable" (they're credit)
    if account.Type == AccountTypeLiability {
        return false
    }
    // Manual asset accounts default to spendable
    if account.Type == AccountTypeAsset {
        return true
    }
    return false
}
```

### User Override

Add to accounts table:

```sql
ALTER TABLE accounts ADD COLUMN include_in_left_to_spend BOOLEAN;
-- NULL = use smart default, TRUE/FALSE = user override
```

---

## Component 2: Left to Spend Calculation

### Core Query

```sql
-- Get spendable account total
WITH spendable AS (
  SELECT SUM(e.amount_cents) as total
  FROM accounts a
  JOIN entries e ON a.id = e.account_id
  WHERE a.ledger_id = $1
    AND a.is_active = true
    AND (
      a.include_in_left_to_spend = true
      OR (a.include_in_left_to_spend IS NULL AND a.teller_subtype = 'checking')
      OR (a.include_in_left_to_spend IS NULL AND a.type = 'asset' AND a.teller_subtype IS NULL)
    )
),
-- Get upcoming bills (from detected_recurring table)
upcoming AS (
  SELECT SUM(COALESCE(user_adjusted_amount_cents, next_expected_amount_cents)) as total
  FROM detected_recurring
  WHERE ledger_id = $1
    AND is_active = true
    AND next_expected_date <= CURRENT_DATE + INTERVAL '30 days'
)
SELECT 
  COALESCE(spendable.total, 0) as available,
  COALESCE(upcoming.total, 0) as upcoming_bills,
  COALESCE(spendable.total, 0) - COALESCE(upcoming.total, 0) as left_to_spend
FROM spendable, upcoming;
```

---

## Component 3: Upcoming Bills List

### Data Structure

```go
type UpcomingBill struct {
    ID               uuid.UUID
    MerchantName     string
    MerchantLogo     string
    ExpectedDate     time.Time
    ExpectedAmount   int64
    IsCovered        bool      // available_at_date >= amount
    DaysUntil        int
    BillType         string    // subscription, utility, etc.
    Confidence       int       // detection confidence
}
```

### "Covered" Logic

```go
func isCovered(bill UpcomingBill, runningBalance int64) bool {
    // Simple: do we have enough right now?
    return runningBalance >= bill.ExpectedAmount
}
```

Show next 5 upcoming bills, sorted by date.

---

## UI Design

### Pulse Page Hero Section

```
┌─────────────────────────────────────────────────────────────┐
│  LEFT TO SPEND                                              │
│                                                             │
│  $1,357                                                     │
│  ████████████████████░░░░░░░░░░                             │
│                                                             │
│  $3,200 available · $1,843 in upcoming bills                │
└─────────────────────────────────────────────────────────────┘
```

### Upcoming Bills Widget

```
┌─────────────────────────────────────────────────────────────┐
│  UPCOMING BILLS                                             │
├─────────────────────────────────────────────────────────────┤
│  [🏠] Rent              Jan 1    $1,500    [COVERED]        │
│  [⚡] Electric Company   Jan 5    ~$140     [COVERED]        │
│  [🎬] Netflix           Jan 15   $22.99    [COVERED]        │
│  [🚗] Car Insurance     Jan 20   $180      [COVERED]        │
│  [💳] Amex Payment      Jan 25   ~$800     [SHORT $200]     │
└─────────────────────────────────────────────────────────────┘
```

---

## Implementation Phases

### Phase 1: Spendable Account Logic
- Add `include_in_left_to_spend` column to accounts
- Implement smart default logic
- UI to toggle accounts (settings or inline)

### Phase 2: Left to Spend Calculation
- Create `internal/pulse/calculator.go`
- SQL queries for available balance + upcoming bills
- Uses `detected_recurring` from Subscription Tracker

### Phase 3: Pulse Page UI
- New handler `internal/handlers/pulse.go`
- Left to Spend hero component
- Upcoming Bills list component
- Wire up to navigation

### Phase 4: User Editing
- Edit predicted bills (adjust amount, date, disable)
- Add manual bills not detected
- Mark bills as "paid" or "skip this month"

---

## Files to Create/Modify

### New Files
- `internal/pulse/calculator.go` - left to spend math
- `internal/views/pulse/left_to_spend.go` - hero component
- `internal/views/pulse/upcoming_bills.go` - bills list component

### Modified Files
- `internal/db/migrations/` - add accounts.include_in_left_to_spend column
- `internal/models/account.go` - add IncludeInLeftToSpend field
- `internal/handlers/pulse.go` - add left to spend data
- Navigation - ensure Pulse link is present

---

## Open Questions

1. **Time horizon for "upcoming"**: Show all predicted bills, or cap at 30 days?
2. **Credit card payments**: Auto-detect minimum payment due? Or let user set?
3. **Variable bills**: Show average, or last amount, or user-set estimate?
4. **Overdraft warning**: Alert if upcoming bill will overdraw account?

---

## Todos

- [ ] Add migration for accounts.include_in_left_to_spend column
- [ ] Implement smart default logic for spendable accounts
- [ ] Build Left to Spend calculator in internal/pulse/calculator.go
- [ ] Create Pulse page handler with Left to Spend data
- [ ] Build Left to Spend hero UI component
- [ ] Build Upcoming Bills list UI component
- [ ] Add ability to edit/disable detected bills
- [ ] Add ability to manually add bills
