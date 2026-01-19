---
title: Subscription Period Pro-rata Calculation
description: ADR for rolling period-based subscription billing to ensure fair quota allocation
---

# ADR-24: Subscription Period Pro-rata Calculation

> 🇰🇷 [한국어 버전](/ko/adr/web/24-subscription-period-prorata)

| Date       | Author     | Repos |
| ---------- | ---------- | ----- |
| 2026-01-16 | @specvital | web   |

## Context

Calendar month-based billing periods created unfair quota allocation for users who signed up late in the month. For example, a user signing up on January 29th would receive a full month's quota but have it reset on February 1st—effectively getting 2 months worth of quota for only 3 days of actual usage.

This issue was identified as a consequence of the billing architecture and required a fair period calculation strategy.

### Problem Scenario

| Signup Date  | Calendar Reset | Days of Actual Usage | Quota Received |
| ------------ | -------------- | -------------------- | -------------- |
| January 1st  | February 1st   | 31 days              | 1 month        |
| January 29th | February 1st   | 3 days               | 1 month        |

Users signing up on the 29th receive the same quota as users signing up on the 1st, despite having only 10% of the usage period.

### Relationship to ADR-13

This ADR extends [ADR-13: Billing and Quota Architecture](/en/adr/13-billing-quota-architecture.md), which identified "Rolling period from signup" as the selected strategy but deferred implementation specifics. ADR-13 also noted "Proration Complexity: Mid-cycle plan changes require quota adjustment" as a negative consequence that remains future work.

## Decision

**Implement rolling period calculation based on each user's signup timestamp.**

Each user's billing period starts from their actual signup date and ends exactly one calendar month later, ensuring every user receives a full month of quota regardless of when they sign up.

### Implementation

```go
// src/backend/modules/subscription/usecase/assign_default_plan.go
now := time.Now().UTC()
periodStart := now                                       // Signup timestamp
periodEnd := now.AddDate(0, 1, 0).Add(-time.Nanosecond)  // Exactly 1 month later
```

### Key Design Decisions

| Decision                 | Rationale                                                                 |
| ------------------------ | ------------------------------------------------------------------------- |
| UTC normalization        | Prevents timezone-related edge cases                                      |
| `AddDate(0, 1, 0)`       | Uses Go's calendar month semantics (handles Feb 28, leap years correctly) |
| Nanosecond subtraction   | Ensures exclusive upper bound for SQL range queries                       |
| Per-user period tracking | Period boundaries stored in `user_subscriptions` table                    |

### Usage Query Pattern

```sql
SELECT COALESCE(SUM(quota_amount), 0)::bigint AS total
FROM usage_events
WHERE user_id = $1
    AND event_type = $2
    AND created_at >= $3    -- current_period_start (inclusive)
    AND created_at < $4     -- current_period_end (exclusive)
```

## Options Considered

### Option A: Rolling Period from Signup Date (Selected)

Period starts from user's actual signup timestamp; ends exactly 1 calendar month later.

| Pros                                          | Cons                             |
| --------------------------------------------- | -------------------------------- |
| Fair to all users regardless of signup timing | Different renewal dates per user |
| Predictable renewal dates (user-specific)     | Complicates batch reporting      |
| No late-signup penalty                        | No centralized processing window |
| Cannot game system by signing up on month-end |                                  |

### Option B: Calendar Month Period

Period always runs from 1st to last day of the month for all users.

| Pros                      | Cons                                                |
| ------------------------- | --------------------------------------------------- |
| Simple implementation     | Unfair to late-month signups (up to 97% value loss) |
| Easy batch processing     | Gaming potential (signup on 28th, reset on 1st)     |
| Cohort analysis alignment | User complaints about unfairness                    |

**Rejected**: Unacceptable user fairness trade-off.

### Option C: Prorated Calendar Month

Calendar month alignment with prorated quota for partial first month.

| Pros                         | Cons                         |
| ---------------------------- | ---------------------------- |
| Maintains calendar alignment | Complex partial calculations |
| Mathematically fair          | Hard to communicate to users |
|                              | Fractional quota UX friction |

**Rejected**: Complexity outweighs calendar alignment benefits.

### Option D: Fixed Day Anchor (1st or 15th)

Round signup to nearest anchor date for predictable batch windows.

| Pros                                 | Cons                                     |
| ------------------------------------ | ---------------------------------------- |
| Predictable batch processing windows | Up to 14 days unfairness                 |
| Simpler reporting                    | Arbitrary cutoffs feel unfair            |
|                                      | Users may delay signup to maximize value |

**Rejected**: Arbitrary unfairness creates user dissatisfaction.

## Consequences

### Positive

| Area                | Benefit                                                                  |
| ------------------- | ------------------------------------------------------------------------ |
| User Fairness       | Every user receives exactly 1 month of quota regardless of signup timing |
| Predictable Renewal | User knows exact renewal date (visible in UI as specific calendar date)  |
| Abuse Prevention    | Cannot game system by signing up on month-end for immediate reset        |
| Clear Expectations  | UI shows "Resets on January 16" rather than ambiguous "monthly"          |
| ADR-13 Alignment    | Fulfills selected strategy documented in parent ADR                      |

### Negative

| Area                  | Trade-off                                        | Mitigation                                            |
| --------------------- | ------------------------------------------------ | ----------------------------------------------------- |
| No Batch Window       | Different users reset on different days          | Acceptable for current scale                          |
| Invoice Complexity    | Payment integration needs per-user billing dates | Future consideration when payment processing required |
| Month Length Variance | Jan 31 rolls to Feb 28 (loses 3 days)            | Go's `AddDate` handles correctly; acceptable variance |
| Mid-cycle Changes     | Plan upgrades/downgrades need proration logic    | Explicitly deferred as future work                    |
| Reporting Complexity  | Rolling periods complicate cohort analysis       | Accept for user fairness prioritization               |

### Edge Cases Handled

| Edge Case                        | Handling                                   |
| -------------------------------- | ------------------------------------------ |
| Month boundary (Jan 31 → Feb 28) | Go `AddDate` calendar semantics            |
| Leap year (Feb 29)               | Go standard library handles correctly      |
| Timezone variance                | UTC normalization at storage layer         |
| Boundary precision               | Nanosecond-precision exclusive upper bound |

## Deferred Decisions

The following mid-cycle scenarios remain unimplemented per ADR-13's noted consequences:

| Scenario            | Proposed Approach                                                            | Status      |
| ------------------- | ---------------------------------------------------------------------------- | ----------- |
| Upgrade mid-cycle   | Add prorated difference: `(newQuota - oldQuota) * remainingDays / totalDays` | Future work |
| Downgrade mid-cycle | Options: carry over unused OR reset to new limit                             | Future work |
| Plan expiration     | Graceful degradation to free tier                                            | Future work |

## References

- [ADR-13: Billing and Quota Architecture](/en/adr/13-billing-quota-architecture.md)
- [Commit ec64e42](https://github.com/specvital/web/commit/ec64e42) - fix(subscription): fix users getting 2 months quota when signing up late in month
- [Commit b364f51](https://github.com/specvital/web/commit/b364f51) - fix(account): fix awkward grammar in usage reset date display
