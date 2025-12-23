---
title: Scheduled Re-collection
description: ADR on scheduler-based re-collection with adaptive decay for data freshness
---

# ADR-01: Scheduled Re-collection Architecture

> 🇰🇷 [한국어 버전](/ko/adr/collector/01-scheduled-recollection.md)

| Date       | Author       | Repos     |
| ---------- | ------------ | --------- |
| 2024-12-18 | @KubrickCode | collector |

## Context

### The Response Time Problem

ADR-05 established queue-based asynchronous processing for initial analysis requests. While this solves the long-running task problem, it introduces latency: users must wait for queue processing even when requesting analysis of a previously analyzed repository.

**If repositories are pre-analyzed and kept fresh, users get instant responses without waiting for analysis.**

### User Experience Impact

| Scenario             | On-Demand Only        | With Pre-collection         |
| -------------------- | --------------------- | --------------------------- |
| First visit          | Queue wait (expected) | Queue wait (expected)       |
| Return visit (fresh) | Instant from cache    | Instant from cache          |
| Return visit (stale) | Queue wait again      | Instant (pre-refreshed)     |
| Popular repository   | Queue wait            | Instant (likely pre-cached) |

The key insight: **most user requests are for previously analyzed repositories**. Pre-collection eliminates queue wait time for the majority of requests.

### Secondary Benefit: Data Freshness

Beyond response time, pre-collection also solves data staleness (dependency updates, security patches, code refactoring).

### Key Requirements

1. **Automatic Updates**: Re-analyze previously collected repositories periodically
2. **Resource Efficiency**: Avoid unnecessary re-collection of inactive repositories
3. **Distributed Safety**: Prevent duplicate executions in multi-instance deployments
4. **Graceful Degradation**: Handle failures without cascading effects

## Decision

**Adopt a scheduler-based re-collection system with adaptive decay logic.**

Core principles:

1. **Adaptive Refresh Intervals**: Decay algorithm based on user activity
2. **Distributed Locking**: PostgreSQL-based lock for single execution guarantee
3. **Service Separation**: Scheduler runs independently from Worker
4. **Circuit Breaker**: Automatic halt on consecutive failures

## Options Considered

### Option A: Scheduler with Adaptive Decay (Selected)

**How It Works:**

- Cron job triggers periodically
- Scheduler acquires distributed lock to prevent duplicate execution
- Query candidates: repositories viewed within a configured window
- Apply decay algorithm: more recent activity → more frequent refresh
- Enqueue eligible repositories to task queue

**Decay Algorithm Concept:**

- Recently viewed repositories refresh more frequently
- As idle time increases, refresh interval lengthens
- Beyond a threshold, repositories are considered idle and excluded

**Pros:**

- Optimizes resource usage based on actual user activity
- Prevents stale data for active repositories
- Automatically stops refreshing abandoned repositories
- Failure isolation via consecutive failure tracking

**Cons:**

- Complex logic for interval calculation
- Requires tracking user view timestamps
- Cutoff threshold may be too aggressive for some use cases

### Option B: Fixed Interval Refresh

- Refresh all repositories every N hours regardless of activity
- Simple but wastes resources on inactive repositories

### Option C: Event-Driven Refresh

- Trigger re-collection on external events (GitHub webhooks)
- Real-time but requires webhook infrastructure and access

## Implementation Considerations

### Service Architecture

| Component | Scaling Strategy                        |
| --------- | --------------------------------------- |
| Worker    | Horizontal scaling based on queue depth |
| Scheduler | Single active instance (lock-protected) |

**Separation Rationale:**

- Worker scaling doesn't spawn redundant schedulers
- Scheduler changes don't require Worker redeployment
- Blue-green deployments remain safe via distributed lock

### Private Repository Handling

**Design Decision: Runtime Validation, Not Schema-Level Filtering**

Repository visibility is not stored in the database because it can change at any time (public↔private). Instead, visibility is validated at runtime during clone.

**Why This Approach:**

| Concern            | Solution                                             |
| ------------------ | ---------------------------------------------------- |
| Token management   | Scheduler operates without user tokens               |
| Visibility changes | No stale visibility flag to maintain                 |
| Security           | No background access to private code without consent |
| Simplicity         | No additional schema or sync logic                   |

**Behavior:**

- Scheduler enqueues all eligible candidates (no visibility filter)
- Worker attempts unauthenticated clone
- Private repositories fail naturally
- Consecutive failures accumulate → eventually excluded

**Note:** Technically possible to re-collect private repos using stored user tokens. Intentionally excluded due to:

- Token expiration/revocation handling complexity
- User may have lost repository access (left organization, permissions revoked)
- Privacy concerns: background access without explicit user action
- Rate limit consumption against user's GitHub quota

### Error Handling Strategy

**Circuit Breaker Pattern:**

- Scheduler level: Consecutive enqueue failures halt the batch
- Repository level: Consecutive analysis failures exclude from auto-refresh
- Recovery: Next cycle starts fresh; manual re-analysis resets counters

### Deduplication

- Unique window prevents duplicate enqueues within a configured period
- Handles cron jitter and manual enqueue overlap

## Consequences

### Positive

**Resource Efficiency:**

- Active repositories get frequent updates
- Inactive repositories consume zero resources
- Decay algorithm naturally limits batch sizes

**System Reliability:**

- Distributed lock guarantees single scheduler execution
- Individual repository failures don't affect others
- Transient failures retry via queue mechanism

**Operational Simplicity:**

- Single scheduler instance to monitor
- Clear failure signals via failure counters

### Negative

**Complexity:**

- Decay algorithm requires careful tuning
- Distributed lock adds operational dependency on PostgreSQL
- Multiple failure counters to track and understand

**Limitations:**

- Minimum granularity limited by cron interval
- Hard cutoff may miss long-term inactive users returning
- Lock TTL limits maximum batch processing time

## References

- [ADR-04: Queue-Based Asynchronous Processing](/en/adr/04-queue-based-async-processing.md)
