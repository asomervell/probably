# Subscription Tracker Feature Plan

## Overview

Subscription management feature that shows all detected recurring subscriptions, tracks price changes over time, and alerts users when a subscription silently increases in price.

## Status: Not Started

## What We're Building

A dedicated view for managing subscriptions that:
- Lists all detected fixed-amount recurring charges
- **Detects and alerts on price increases** ("Netflix went from $15.99 to $22.99")
- Shows monthly/annual subscription totals
- Lets users cancel-track or snooze subscriptions

---

## Key Differentiator: Price Change Detection

Most finance apps show subscriptions. We go further by **detecting when companies silently raise prices**.

```
┌─────────────────────────────────────────────────────────────┐
│  ⚠️  PRICE INCREASE DETECTED                                │
│                                                             │
│  [🎬] Netflix                                               │
│  Was: $15.99/month → Now: $22.99/month                      │
│  +$7.00/month (+44%)                                        │
│  First noticed: Dec 15, 2024                                │
│                                                             │
│  [Acknowledge]  [Cancel Subscription]                       │
└─────────────────────────────────────────────────────────────┘
```

---

## Subscription vs Bill Distinction

From the recurring detection engine:

| Type | Amount | Examples | Show in Tracker? |
|------|--------|----------|------------------|
| **Subscription** | Fixed | Netflix, Spotify, Gym | ✅ Yes |
| **Bill** | Variable | Electric, Water, Phone | ❌ No (show in Upcoming Bills) |
| **Rent/Mortgage** | Fixed but large | Rent, Mortgage | ❌ No (it's a bill, not subscription) |

**Classification logic:**
```go
func isSubscription(pattern DetectedRecurring) bool {
    // Fixed amount (low variance)
    isFixedAmount := pattern.AmountVarianceCents < 100 // <$1 variance
    
    // Reasonable subscription size ($5-$500/month)
    isReasonableSize := pattern.AvgAmountCents >= 500 && 
                        pattern.AvgAmountCents <= 50000
    
    // Monthly or annual frequency
    isSubscriptionFreq := pattern.Frequency == "monthly" || 
                          pattern.Frequency == "annual"
    
    // Not rent/mortgage (large fixed monthly)
    isNotRent := pattern.AvgAmountCents < 100000 || // <$1000
                 pattern.BillType != "rent"
    
    return isFixedAmount && isReasonableSize && isSubscriptionFreq && isNotRent
}
```

---

## Recurring Detection Engine

This feature BUILDS the recurring detection engine that Left to Spend will reuse.

### Detection Algorithm

```
For each merchant with 2+ transactions in last 12 months:
  1. Calculate intervals between consecutive transactions
  2. Detect dominant frequency:
     - Weekly: avg interval 5-9 days
     - Biweekly: avg interval 12-16 days  
     - Monthly: avg interval 25-35 days
     - Quarterly: avg interval 80-100 days
     - Annual: avg interval 350-380 days
  3. Calculate amount stats:
     - Average amount
     - Variance (fixed vs variable)
  4. Score confidence (0-100):
     - High: consistent interval + consistent amount
     - Medium: consistent interval + variable amount (utility bill)
     - Low: irregular interval
  5. Predict next occurrence:
     - last_transaction_date + detected_interval
```

### Frequency-Dependent Thresholds

- **Monthly or longer**: 2 occurrences to detect
- **Weekly/Biweekly**: 3 occurrences (need more data)

---

## Price Change Detection Algorithm

### Track Amount History

```sql
CREATE TABLE subscription_amount_history (
    id UUID PRIMARY KEY,
    recurring_id UUID REFERENCES detected_recurring(id),
    amount_cents BIGINT NOT NULL,
    first_seen_at DATE NOT NULL,
    last_seen_at DATE NOT NULL,
    occurrence_count INT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Detection Flow

```
On new transaction matched to recurring pattern:
  1. Check if amount matches current expected amount (within $0.50)
  2. If YES: update last_seen_at, increment count
  3. If NO (amount changed):
     a. Create new amount_history record
     b. If new amount > old amount:
        - Create price_increase_alert
        - Mark as unacknowledged
     c. Update pattern's avg_amount to new amount
```

---

## Database Schema

```sql
-- Core recurring detection table
CREATE TABLE detected_recurring (
  id UUID PRIMARY KEY,
  ledger_id UUID NOT NULL REFERENCES ledgers(id),
  merchant_id UUID REFERENCES merchants(id),
  description_pattern TEXT,
  
  -- Detection results
  frequency TEXT,            -- weekly, biweekly, monthly, quarterly, annual
  interval_days INT,
  avg_amount_cents BIGINT,
  amount_variance_cents BIGINT,
  confidence_score INT,      -- 0-100
  
  -- Prediction
  last_occurrence_date DATE,
  next_expected_date DATE,
  next_expected_amount_cents BIGINT,
  
  -- User overrides
  is_active BOOLEAN DEFAULT true,
  user_adjusted_amount_cents BIGINT,
  user_adjusted_date DATE,
  
  -- Classification
  bill_type TEXT,  -- subscription, utility, rent, insurance, other
  is_subscription BOOLEAN DEFAULT false,
  
  -- Price change tracking
  previous_amount_cents BIGINT,
  amount_changed_at DATE,
  price_change_acknowledged BOOLEAN DEFAULT false,
  cancelled_at DATE,
  tracking_paused BOOLEAN DEFAULT false,
  
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ
);

-- Price history for subscriptions
CREATE TABLE subscription_price_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recurring_id UUID NOT NULL REFERENCES detected_recurring(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL,
    effective_from DATE NOT NULL,
    effective_to DATE,  -- NULL = current
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## UI Design

### Subscriptions List View

```
┌─────────────────────────────────────────────────────────────┐
│  SUBSCRIPTIONS                          Total: $127.94/mo   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ⚠️ PRICE CHANGES (1)                                       │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ [🎬] Netflix     $15.99 → $22.99  +$7.00  [Acknowledge] ││
│  └─────────────────────────────────────────────────────────┘│
│                                                             │
│  ACTIVE SUBSCRIPTIONS                                       │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ [🎵] Spotify           $10.99/mo    Next: Jan 15        ││
│  │ [☁️] iCloud            $2.99/mo     Next: Jan 3         ││
│  │ [🎮] Xbox Game Pass    $16.99/mo    Next: Jan 22        ││
│  │ [📰] NYT Digital       $17.00/mo    Next: Jan 8         ││
│  │ [💪] Planet Fitness    $24.99/mo    Next: Jan 1         ││
│  │ [🎬] Netflix           $22.99/mo    Next: Jan 15        ││
│  │ [📦] Amazon Prime      $139/yr      Next: Mar 15        ││
│  └─────────────────────────────────────────────────────────┘│
│                                                             │
│  Monthly: $96.95 · Annual: $139.00 ($11.58/mo)              │
│  Total: $108.53/month equivalent                            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Files to Create/Modify

### New Files
- `internal/db/migrations/033_detected_recurring.sql`
- `internal/db/migrations/034_subscription_tracking.sql`
- `internal/recurring/detector.go` - detection algorithm
- `internal/recurring/detector_test.go`
- `internal/subscriptions/tracker.go`
- `internal/subscriptions/classifier.go`
- `internal/handlers/subscriptions.go`
- `internal/views/subscriptions/` - UI components

### Modified Files
- Transaction import pipeline - hook detection + price change

---

## Edge Cases

1. **Subscription with tiers**: User upgrades Netflix plan → Always alert, let user acknowledge
2. **Annual subscriptions**: Only see price change once a year → Track by amount, alert even if 12 months apart
3. **Prorated charges**: First charge is different → Ignore first charge, establish baseline on second
4. **Free trials ending**: $0 → $9.99 → Don't alert on $0 → any amount (trial ending)
5. **Currency/tax variations**: Same subscription, slightly different amounts → Allow $0.50 tolerance

---

## Todos

- [ ] Create migration for detected_recurring table
- [ ] Create migration for subscription tracking fields and price history table
- [ ] Build recurring detection algorithm in internal/recurring/detector.go
- [ ] Write tests for detection algorithm with various patterns
- [ ] Build subscription classifier to distinguish subscriptions from bills
- [ ] Implement price change detection algorithm
- [ ] Hook detection into transaction import pipeline
- [ ] Create subscriptions handler with list/detail/actions
- [ ] Build subscriptions list UI with price change alerts
- [ ] Build subscription detail view with price history
- [ ] Implement acknowledge/cancel/pause actions
