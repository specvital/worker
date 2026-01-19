---
title: Anonymous User Rate Limiting
description: ADR on IP-based in-memory rate limiting for anonymous users on the analyzer API
---

# ADR-21: Anonymous User Rate Limiting

> [한국어 버전](/ko/adr/web/21-anonymous-rate-limiting.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2026-01-15 | @KubrickCode | web   |

## Context

### Problem Statement

The analyzer API is publicly accessible without authentication, enabling anonymous users to explore the platform. This creates abuse risk vectors:

| Risk                | Impact                        | Likelihood |
| ------------------- | ----------------------------- | ---------- |
| API abuse/scraping  | Service degradation           | Medium     |
| Resource exhaustion | Outage affecting paying users | Low-Medium |
| Cost amplification  | Increased compute costs       | Medium     |

### Existing Architecture

The platform has a two-tier user experience:

- **Authenticated users**: Quota system managed via billing service ([ADR-13](/en/adr/13-billing-quota-architecture.md))
- **Anonymous users**: No protection mechanism prior to this decision

Anonymous users need throttling while still allowing platform exploration for potential conversions.

### Constraints

| Constraint                      | Source | Implication                             |
| ------------------------------- | ------ | --------------------------------------- |
| Single-instance deployment      | Infra  | No distributed state needed             |
| PostgreSQL-centric              | ADR-04 | Adding Redis introduces new dependency  |
| PaaS-first strategy             | ADR-06 | Prefer simple, self-contained solutions |
| Authenticated users have quotas | ADR-13 | Rate limiting only for anonymous        |

## Decision

**Implement IP-based in-memory rate limiting for anonymous users on the analyzer API.**

Configuration:

| Parameter | Value          |
| --------- | -------------- |
| Algorithm | Fixed window   |
| Limit     | 10 requests    |
| Window    | 1 minute       |
| Key       | Client IP      |
| Scope     | Analyzer API   |
| Target    | Anonymous only |

Implementation pattern:

```go
userID := middleware.GetUserID(ctx)

if userID == "" && h.anonymousRateLimiter != nil {
    clientIP := middleware.GetClientIP(ctx)
    if !h.anonymousRateLimiter.Allow(clientIP) {
        return 429 Response
    }
}
// Authenticated users bypass rate limiting
```

Response format follows RFC 7807 Problem Details:

```json
{
  "status": 429,
  "title": "Too Many Requests",
  "detail": "Rate limit exceeded. Please sign in for higher limits or try again later."
}
```

## Options Considered

### Option A: In-Memory Fixed Window (Selected)

**How It Works:**

- Per-IP request counter resets each minute window
- Stored in Go map with background cleanup goroutine
- Zero external dependencies

**Pros:**

- Zero infrastructure dependency
- Simple implementation, predictable behavior
- Immediate availability without deployment changes

**Cons:**

- Single-instance only (not suitable for horizontal scaling)
- State lost on restart (limits reset on deployment)
- Fixed window boundary burst (worst case: 20 requests at window boundary)

### Option B: Redis-Based Distributed

**How It Works:**

- Centralized counter stored in Redis
- Atomic increment with TTL for window expiration

**Pros:**

- Multi-instance support for horizontal scaling
- State persistence across restarts
- Proven at scale

**Cons:**

- New infrastructure dependency (violates ADR-06)
- Network latency overhead per request
- Operational complexity (Redis monitoring, failover)

**Decision:** Rejected for current single-instance deployment. Reconsider when scaling to multiple instances.

### Option C: Cloud WAF / API Gateway

**How It Works:**

- Configure rate limiting at Cloudflare or API Gateway level
- Edge enforcement before reaching application

**Pros:**

- Zero application code changes
- Global edge enforcement
- DDoS protection included

**Cons:**

- Cannot differentiate authenticated vs. anonymous users at edge
- Coarser granularity than application-level control
- External dependency, cost per request

**Decision:** Retain as defense-in-depth layer, not primary solution.

### Option D: No Application-Level Limiting

**How It Works:**

- Rely solely on infrastructure protection (Cloudflare)
- No application-level throttling

**Pros:**

- Simplest approach, already deployed

**Cons:**

- Cannot understand application semantics (user types)
- Treats all users equally, conflicts with two-tier experience

**Decision:** Rejected as it cannot provide user-type-specific behavior.

## Implementation

### Rate Limiter Component

```
src/backend/
├── common/
│   ├── ratelimit/
│   │   └── limiter.go       # Fixed window IPRateLimiter
│   ├── middleware/
│   │   └── ratelimit.go     # Token bucket middleware (alternative)
│   └── httputil/
│       └── client_ip.go     # IP extraction
└── modules/analyzer/
    └── handler/http.go      # Rate limiter integration
```

### IP Extraction Priority

1. `X-Forwarded-For` header (first IP in list)
2. `X-Real-IP` header
3. `RemoteAddr` (fallback)

Trusts proxy headers assuming deployment behind trusted reverse proxy (Railway, Cloudflare).

### Initialization

```go
// app.go
anonymousRateLimiter := ratelimit.NewIPRateLimiter(10, time.Minute)
closers = append(closers, anonymousRateLimiter) // Graceful shutdown
```

## Consequences

### Positive

**1. Zero External Dependency**

- No Redis or external service required
- Aligns with ADR-06 PaaS-first strategy
- Simplified deployment and operations

**2. Abuse Prevention**

- Protects platform resources from anonymous abuse
- Ensures fair access for authenticated users
- Reduces risk of cost amplification

**3. Clear User Experience**

- Anonymous users understand limits exist
- Error message guides toward authentication
- Authenticated users bypass limits entirely

### Negative

**1. Single-Instance Limitation**

- Does not support horizontal scaling
- **Migration path:** Implement Redis-based solution when multi-instance deployment is needed

**2. State Not Persistent**

- Limits reset on application restart
- **Impact:** Acceptable for 1-minute windows; limits recover quickly

**3. IP-Based Identification Limitations**

- False positives on shared IPs (NAT, corporate networks)
- **Impact:** Low limit (10/min) rarely impacts legitimate exploration
- **Mitigation:** Users can authenticate for higher limits

### Trade-off Summary

| Trade-off                           | Decision              | Rationale                                           |
| ----------------------------------- | --------------------- | --------------------------------------------------- |
| Simplicity vs. Scalability          | Favor simplicity      | Single instance today; revisit when scaling         |
| IP accuracy vs. Implementation cost | Accept IP limitations | NAT false positives acceptable for exploration tier |
| Memory vs. External dependency      | Favor memory          | In-memory acceptable for anonymous user count       |

## References

- [ADR-13: Billing and Quota Architecture](/en/adr/13-billing-quota-architecture.md) - Quota system for authenticated users
- [ADR-13 (Web): Domain Error Handling Pattern](/en/adr/web/13-domain-error-handling-pattern.md) - RateLimitError custom type
- [GitHub Issue #207](https://github.com/specvital/web/issues/207) - Implementation tracking
- [Commit 107f387](https://github.com/specvital/web/commit/107f387) - Implementation commit
