---
title: Multi-Queue Priority Routing
description: ADR for tier-based queue routing architecture with configurable worker allocation
---

# ADR-16: Multi-Queue Priority Routing Architecture

> 🇰🇷 [한국어 버전](/ko/adr/16-multi-queue-priority-routing.md)

| Date       | Author       | Repos              |
| ---------- | ------------ | ------------------ |
| 2026-01-19 | @KubrickCode | web, worker, infra |

## Context

### Business Problem

The Specvital platform introduced tiered pricing (Free, Pro, Pro Plus, Enterprise) per [ADR-13](/en/adr/13-billing-quota-architecture.md) to monetize the test analysis service. However, the existing queue infrastructure treated all requests equally, creating three critical issues:

| Problem                     | Business Impact                                                   |
| --------------------------- | ----------------------------------------------------------------- |
| No priority differentiation | Paying customers waited behind free tier during peak load         |
| Scheduler contention        | Background re-analysis jobs competed with user-initiated requests |
| Worker routing errors       | Analyzer and SpecGenerator workers received mismatched job types  |

### Technical Evolution

The queue architecture evolved through three iterations:

**Phase 1: Single Shared Queue**

```
All requests → single FIFO queue → all workers
```

Result: "Job kind not registered" errors when workers received incompatible job types.

**Phase 2: Per-Service Dedicated Queues**

```
Analysis requests → analysis queue → Analyzer workers
SpecView requests → specview queue → SpecGen workers
```

Result: Job routing errors resolved, but no priority control for paid users.

**Phase 3: Tier-Based Multi-Queue (Current)**

```
Pro user analysis → analysis_priority → Analyzer (priority workers)
Free user analysis → analysis_default → Analyzer (default workers)
Scheduler jobs    → analysis_scheduled → Analyzer (scheduled workers)
```

### Constraints

| Constraint                | Origin                                                            | Impact                                                      |
| ------------------------- | ----------------------------------------------------------------- | ----------------------------------------------------------- |
| River queue naming        | Library validation                                                | Cannot use colons (`:`) in queue names                      |
| PostgreSQL-backed queue   | [ADR-04](/en/adr/04-queue-based-async-processing.md)              | Solution must integrate with existing River setup           |
| Worker process separation | [ADR-05 Worker](/en/adr/worker/05-worker-scheduler-separation.md) | Workers and Scheduler are separate deployments              |
| Billing tier integration  | [ADR-13](/en/adr/13-billing-quota-architecture.md)                | Queue selection must use tier information from subscription |

### Why Now

The billing tier system (ADR-13) established subscription tiers but lacked operational differentiation. Paying customers received no tangible processing advantage, undermining the value proposition of paid plans.

## Decision

**Adopt three-tier queue architecture per service with configurable worker allocation.**

### Queue Structure

Each worker service maintains three queues with tier-based routing:

```
{service}_priority   ← Pro / Pro Plus / Enterprise users
{service}_default    ← Free tier users
{service}_scheduled  ← Background scheduler jobs
```

Concrete queue names:

| Service       | Priority Queue      | Default Queue      | Scheduled Queue      |
| ------------- | ------------------- | ------------------ | -------------------- |
| Analyzer      | `analysis_priority` | `analysis_default` | `analysis_scheduled` |
| SpecGenerator | `specview_priority` | `specview_default` | `specview_scheduled` |

### Worker Allocation Strategy

Workers per queue are configurable via environment variables with sensible defaults:

**Analyzer Service:**

```
ANALYZER_QUEUE_PRIORITY_WORKERS=5   # 50% of worker capacity
ANALYZER_QUEUE_DEFAULT_WORKERS=3    # 30% of worker capacity
ANALYZER_QUEUE_SCHEDULED_WORKERS=2  # 20% of worker capacity
```

**SpecGenerator Service:**

```
SPECGEN_QUEUE_PRIORITY_WORKERS=3    # 50% of worker capacity
SPECGEN_QUEUE_DEFAULT_WORKERS=2     # 33% of worker capacity
SPECGEN_QUEUE_SCHEDULED_WORKERS=1   # 17% of worker capacity
```

### Queue Selection Logic

```go
func SelectQueue(baseQueue string, tier PlanTier, isScheduled bool) string {
    if isScheduled {
        return baseQueue + "_scheduled"
    }
    switch tier {
    case PlanTierPro, PlanTierProPlus, PlanTierEnterprise:
        return baseQueue + "_priority"
    default:
        return baseQueue + "_default"
    }
}
```

### Naming Convention

Queue names use underscores as separators to comply with River's validation:

- Allowed: letters, numbers, underscores, hyphens
- Forbidden: colons, spaces, special characters

Changed from original design `analysis:priority` to `analysis_priority`.

## Options Considered

### Option A: Tier-Based Multi-Queue with Configurable Worker Allocation (Selected)

**Description:**
Separate queues per tier per service, with worker counts configurable via environment variables.

```
analysis_priority   → Priority workers (5)
analysis_default    → Default workers (3)
analysis_scheduled  → Scheduled workers (2)
```

**Pros:**

| Benefit                | Explanation                                                          |
| ---------------------- | -------------------------------------------------------------------- |
| Clear SLA boundaries   | Priority queue depth indicates paid user experience                  |
| Independent scaling    | Adjust worker ratios without code changes                            |
| Scheduler isolation    | Background jobs cannot starve user requests                          |
| Monitoring granularity | Per-queue metrics enable tier-specific alerting                      |
| Graceful degradation   | If priority queue empty, workers remain idle (no priority inversion) |

**Cons:**

| Trade-off                | Mitigation                                   |
| ------------------------ | -------------------------------------------- |
| Configuration complexity | Sensible defaults; only tune when needed     |
| Worker underutilization  | Acceptable for SLA guarantees                |
| More queues to monitor   | Unified dashboard with queue-specific panels |

### Option B: Single Queue with Priority Field

**Description:**
Single queue per service with `priority` field on each job. Workers process higher-priority jobs first via `ORDER BY priority DESC`.

```sql
SELECT * FROM river_job
WHERE queue = 'analysis' AND state = 'available'
ORDER BY priority DESC, scheduled_at ASC
LIMIT 1 FOR UPDATE SKIP LOCKED;
```

**Pros:**

- Simpler configuration (one queue per service)
- Workers always have work (no idle capacity)
- Fewer infrastructure components

**Cons:**

| Issue                   | Severity                                        |
| ----------------------- | ----------------------------------------------- |
| Priority inversion risk | High - large batch of free jobs delays pro user |
| Query complexity        | Medium - `ORDER BY` on hot table                |
| No SLA guarantee        | High - cannot ensure priority processing time   |
| Monitoring difficulty   | Medium - single queue depth hides tier health   |

**Verdict:** Rejected. Priority inversion undermines the core business goal of differentiating paid tiers.

### Option C: Separate Worker Instances per Tier

**Description:**
Dedicated worker deployments for each tier:

```
analyzer-priority-worker  (Pro/Enterprise only)
analyzer-default-worker   (Free only)
analyzer-scheduled-worker (Scheduler only)
```

**Pros:**

- Complete resource isolation
- Independent scaling and deployment
- Clear security boundaries possible

**Cons:**

| Issue                   | Severity                                      |
| ----------------------- | --------------------------------------------- |
| Infrastructure cost     | High - 3x worker deployments per service      |
| Capacity waste          | High - cannot share idle workers across tiers |
| Operational complexity  | High - 6+ services to deploy/monitor          |
| Deployment coordination | Medium - schema changes affect all instances  |

**Verdict:** Rejected. Excessive infrastructure overhead for current scale. May revisit for enterprise-dedicated workers at larger scale.

## Consequences

### Positive

**Paid User Value Proposition:**

- Pro/Enterprise users experience 50% of worker capacity dedicated to their requests
- During peak load, paid users see consistent processing times while free tier degrades gracefully
- Clear justification for pricing tiers

**Operational Control:**

- Worker allocation tunable without deployment via environment variables
- Queue depth per tier enables proactive scaling decisions
- Scheduler jobs cannot impact user-facing latency

**Observability:**

| Metric                | Indicates                   |
| --------------------- | --------------------------- |
| Priority queue depth  | Paid user experience health |
| Default queue depth   | Free tier wait times        |
| Scheduled queue depth | Background job backlog      |

### Negative

**Configuration Overhead:**

- Six environment variables per worker service (18 total across 3 services)
- Incorrect ratios could waste capacity or degrade paid experience
- Requires documentation for operators

**Worker Idle Capacity:**

- Priority workers sit idle during low paid-user traffic
- Cannot dynamically rebalance across queues
- Acceptable trade-off for SLA guarantees

**Monitoring Complexity:**

- Three queues per service (6 total) vs. previous 2 queues
- Dashboard and alerting configuration increase
- Correlation across queues needed for debugging

### Technical Implications

| Aspect               | Implication                                                   |
| -------------------- | ------------------------------------------------------------- |
| River Configuration  | `QueueConfig` map with three entries per worker               |
| Queue Naming         | Underscore separators (`_priority`, `_default`, `_scheduled`) |
| Tier Lookup          | API layer queries subscription before enqueue                 |
| Graceful Degradation | Unknown tier routes to `_default` queue                       |
| Worker Health        | Each queue has independent worker pool health                 |
| Deployment           | Environment variables override default worker counts          |

## References

- [ADR-04: Queue-Based Async Processing](/en/adr/04-queue-based-async-processing.md) - Foundation for queue architecture
- [ADR-05 Worker: Worker-Scheduler Separation](/en/adr/worker/05-worker-scheduler-separation.md) - Scheduler isolation rationale
- [ADR-13: Billing and Quota Architecture](/en/adr/13-billing-quota-architecture.md) - Tier definitions and billing context
- [River Queue Documentation](https://riverqueue.com/docs) - Queue naming constraints
