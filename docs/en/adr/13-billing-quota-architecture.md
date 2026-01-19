---
title: Billing and Quota Architecture
description: ADR for event-based usage tracking, subscription tiers, and queue prioritization
---

# ADR-13: Billing and Quota Architecture

> [Korean Version](/ko/adr/13-billing-quota-architecture.md)

| Date       | Author       | Repos              |
| ---------- | ------------ | ------------------ |
| 2026-01-18 | @KubrickCode | web, worker, infra |

## Context

### Monetization Requirements

Specvital requires a billing and quota system to monetize the test analysis platform across four subscription tiers: free, pro, pro_plus, and enterprise.

**Key Requirements:**

| Requirement          | Description                                                |
| -------------------- | ---------------------------------------------------------- |
| Usage Tracking       | Track SpecView and Analysis operations accurately          |
| Cache-Aware Billing  | Cache hits should not consume quota                        |
| Audit Compliance     | Usage records must persist even when resources are deleted |
| Fair Quota Periods   | Users should receive full value regardless of signup date  |
| Tier Differentiation | Paying users must experience faster processing             |
| Enterprise Unlimited | Highest tier needs effectively unlimited usage             |

### Constraints

| Constraint         | Impact                                                           |
| ------------------ | ---------------------------------------------------------------- |
| PostgreSQL Backend | River queue uses PostgreSQL (ADR-04); solution must integrate    |
| Cache-First Model  | SpecView serves cached results freely; only misses consume quota |
| Anonymous Access   | Platform allows anonymous exploration with rate limits           |
| Multi-Repository   | Solution spans web, worker, and infra repositories               |

## Decision

**Adopt event-based usage tracking with rolling quota periods and tier-separated queue prioritization.**

### 1. Event-Based Usage Tracking

Record usage events at operation completion time in `usage_events` table:

```sql
table usage_events {
  id: uuid
  user_id -> users
  event_type: specview | analysis
  analysis_id -> analyses? (ON DELETE SET NULL)
  document_id -> spec_documents? (ON DELETE SET NULL)
  quota_amount: int
  created_at: timestamptz
}
```

**Key Characteristics:**

- Events recorded only on successful completion
- Cache hits generate no events
- `ON DELETE SET NULL` preserves audit trail
- Monthly aggregation index for efficient quota lookups
- `quota_amount` stores test case count for SpecView

### 2. Subscription Plan Architecture

Four-tier structure with NULL representing unlimited:

```sql
table subscription_plans {
  tier: free | pro | pro_plus | enterprise
  monthly_price: int?
  specview_monthly_limit: int?
  analysis_monthly_limit: int?
  retention_days: int?
}
```

```sql
table user_subscriptions {
  user_id -> users
  plan_id -> subscription_plans (ON DELETE RESTRICT)
  status: active | canceled | expired
  current_period_start: timestamptz
  current_period_end: timestamptz
}
```

**Constraints:**

- Partial unique index ensures one active subscription per user
- Auto-assign free plan on user signup
- Rolling period: exactly one month from activation date

### 3. Tier-Based Queue Prioritization

Three queues per service with tier-based routing:

| Queue        | Subscribers               | Purpose                  |
| ------------ | ------------------------- | ------------------------ |
| `_priority`  | Pro, Pro Plus, Enterprise | Fast processing for paid |
| `_default`   | Free                      | Standard processing      |
| `_scheduled` | System                    | Scheduled re-analysis    |

**Service-Specific Queue Names:**

```
Analysis Service:
  analysis_priority   → Pro/Enterprise users
  analysis_default    → Free tier users
  analysis_scheduled  → Scheduler/cron jobs

SpecView Service:
  specview_priority   → Pro/Enterprise users
  specview_default    → Free tier users
  specview_scheduled  → Scheduler/cron jobs
```

**Queue Selection Logic:**

```go
// common/queue/selector.go (web repository)
func SelectQueue(baseQueue string, tier PlanTier, isScheduled bool) string {
    if isScheduled {
        return baseQueue + SuffixScheduled  // "_scheduled"
    }
    switch tier {
    case PlanTierPro, PlanTierProPlus, PlanTierEnterprise:
        return baseQueue + SuffixPriority   // "_priority"
    default:
        return baseQueue + SuffixDefault    // "_default"
    }
}
```

**Request Flow:**

```
Handler → TierLookup → UseCase → QueueService → SelectQueue
   │           │           │           │              │
   │     GetUserTier()     │     Enqueue()      compute queue
   │           │           │           │              │
   └───────────┴───────────┴───────────┴──────────────┘
```

**Graceful Degradation:**

- Empty userID or nil tierLookup → routes to `_default`
- Database error on tier lookup → logs warning, routes to `_default`
- Missing subscription record → treats as empty tier, routes to `_default`

**Worker Allocation (configurable via environment):**

```
Analyzer:
  ANALYZER_QUEUE_PRIORITY_WORKERS=5   (default)
  ANALYZER_QUEUE_DEFAULT_WORKERS=3    (default)
  ANALYZER_QUEUE_SCHEDULED_WORKERS=2  (default)

SpecGen:
  SPECGEN_QUEUE_PRIORITY_WORKERS=3    (default)
  SPECGEN_QUEUE_DEFAULT_WORKERS=2     (default)
  SPECGEN_QUEUE_SCHEDULED_WORKERS=1   (default)
```

### 4. Rate Limiting for Anonymous Users

IP-based in-memory rate limiting with fixed window:

- 10 requests per minute per IP
- Only applies to anonymous users on analyze API
- In-memory storage (no external dependency)

## Options Considered

### A. Usage Tracking Strategy

| Option                                   | Verdict                                     |
| ---------------------------------------- | ------------------------------------------- |
| Event-based at completion **(Selected)** | Audit-friendly, cache-aligned, failure-safe |
| Real-time decrement                      | Race conditions, complex refund logic       |
| External metering (Stripe/Orb)           | External dependency, latency, cost at scale |

**Selection Rationale:**

Event-based tracking aligns with existing PostgreSQL-centric architecture and naturally fits the cache-first model where only misses consume quota.

### B. Quota Period Strategy

| Option                                    | Verdict                                |
| ----------------------------------------- | -------------------------------------- |
| Rolling period from signup **(Selected)** | Fair to all users, predictable renewal |
| Calendar month                            | Unfair to late-month signups           |
| Custom billing cycles                     | Maximum complexity, support burden     |

**Selection Rationale:**

Rolling periods prioritize user fairness. Users signing up on the 28th receive a full month of quota, not 3 days until calendar reset.

### C. Queue Priority Strategy

| Option                                  | Verdict                                            |
| --------------------------------------- | -------------------------------------------------- |
| Separate queues per tier **(Selected)** | Clear isolation, SLA-friendly, monitorable         |
| Single queue with priority field        | Priority inversion risk, complex queries           |
| Dedicated worker pools                  | Highest infrastructure cost, cannot share capacity |

**Selection Rationale:**

Separate queues provide cleaner SLA guarantees while allowing shared worker pools for efficiency.

## Cross-Service Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Billing & Quota Flow                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────┐     ┌─────────────────────┐     ┌──────────────┐  │
│  │   Web API   │────▶│  Quota Check API    │◀───▶│  PostgreSQL  │  │
│  │  (Go + Chi) │     │  POST /usage/check  │     │              │  │
│  └──────┬──────┘     │  GET /usage/current │     │  Tables:     │  │
│         │            └─────────────────────┘     │  - users     │  │
│         │                                        │  - subscript │  │
│         ▼                                        │    ion_plans │  │
│  ┌──────────────┐    ┌─────────────────────┐    │  - user_sub  │  │
│  │ Queue Select │───▶│  River Queue        │    │    scriptions│  │
│  │ (by tier)    │    │  - :priority        │    │  - usage_    │  │
│  └──────────────┘    │  - :default         │    │    events    │  │
│                      │  - :scheduled       │    └──────────────┘  │
│                      └──────────┬──────────┘                       │
│                                 │                                  │
│                                 ▼                                  │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                     Worker Service                           │  │
│  │  ┌───────────────┐     ┌───────────────┐                    │  │
│  │  │   Analyzer    │     │  SpecView Gen │                    │  │
│  │  │   Worker      │     │    Worker     │                    │  │
│  │  └───────┬───────┘     └───────┬───────┘                    │  │
│  │          │                     │                             │  │
│  │          ▼                     ▼                             │  │
│  │  Record usage_event    Record usage_event                   │  │
│  │  (type: analysis)      (type: specview)                     │  │
│  │                        (only on cache miss)                 │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Consequences

### Positive

| Area                    | Benefit                                              |
| ----------------------- | ---------------------------------------------------- |
| Billing Accuracy        | Only successful, non-cached operations consume quota |
| Audit Compliance        | `SET NULL` preserves complete usage history          |
| User Fairness           | Rolling periods ensure full subscription value       |
| Paid Tier Experience    | Queue separation guarantees processing priority      |
| Operational Flexibility | Worker allocation tunable via environment variables  |
| Enterprise Simplicity   | NULL values cleanly represent unlimited              |

### Negative

| Area                 | Trade-off                                                      |
| -------------------- | -------------------------------------------------------------- |
| Quota Visibility     | Users see usage update after job completion, not at submission |
| Reporting Complexity | Rolling periods complicate cohort analysis                     |
| Storage Growth       | Event-based tracking accumulates records over time             |
| Queue Monitoring     | Three queues per service increases observability config        |
| Proration Complexity | Mid-cycle plan changes require quota adjustment                |

### Technical Implications

| Aspect           | Implication                                                      |
| ---------------- | ---------------------------------------------------------------- |
| Database Schema  | `usage_events` table with monthly aggregation index              |
| Query Pattern    | Monthly usage via aggregate queries on indexed `created_at`      |
| Worker Config    | Environment variables control worker-to-queue ratios             |
| Rate Limiting    | Anonymous users via IP-based in-memory limits (10 req/min)       |
| Plan Transitions | Partial unique index ensures single active subscription per user |

## References

- [ADR-04: Queue-Based Async Processing](/en/adr/04-queue-based-async-processing.md)
- [ADR-12: Worker-Centric Analysis Lifecycle](/en/adr/12-worker-centric-analysis-lifecycle.md)
- [Related commits](https://github.com/specvital/infra/commits/main) - Database schema for subscription and usage tracking
