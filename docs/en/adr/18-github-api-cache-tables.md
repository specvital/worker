---
title: GitHub API Cache Tables
description: ADR for database-backed cache tables for GitHub API responses
---

# ADR-18: GitHub API Cache Tables

> 🇰🇷 [한국어 버전](/ko/adr/18-github-api-cache-tables.md)

| Date       | Author     | Repos      |
| ---------- | ---------- | ---------- |
| 2025-12-24 | @specvital | infra, web |

## Context

GitHub API imposes strict rate limits: 5,000 requests per hour for authenticated users. This creates UX and reliability issues for the dashboard repository selection feature.

| Issue                    | Impact                                        |
| ------------------------ | --------------------------------------------- |
| Rate limit exhaustion    | Dashboard unusable when limit reached         |
| Repeated API calls       | Every dashboard visit fetches repository list |
| Latency variability      | 100-500ms GitHub API response times           |
| Concurrent request waste | Multiple browser tabs trigger duplicate calls |

### UX Problems

- Slow initial load (200-500ms wait for repository list)
- Power users with many repositories consume rate limit quickly
- No visibility into data freshness

This extends [ADR-09: GitHub App Integration Strategy](/en/adr/09-github-app-integration.md) which established the 5,000/hr rate limit constraint.

## Decision

**Database-backed cache with hybrid normalization and user-controlled refresh.**

### Schema

```sql
-- Shared organization metadata (global deduplication)
CREATE TABLE github_organizations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  github_org_id BIGINT NOT NULL UNIQUE,
  login VARCHAR(255) NOT NULL,
  avatar_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- User-Org N:N relationship
CREATE TABLE user_github_org_memberships (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  github_org_id UUID NOT NULL REFERENCES github_organizations(id) ON DELETE CASCADE,
  CONSTRAINT uq_user_org UNIQUE (user_id, github_org_id)
);

-- Unified repository cache per user
CREATE TABLE user_github_repositories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  github_repo_id BIGINT NOT NULL,
  full_name VARCHAR(511) NOT NULL,
  source_type VARCHAR(50) NOT NULL, -- 'personal' | 'organization'
  source_org_id UUID REFERENCES github_organizations(id),
  CONSTRAINT uq_user_repo UNIQUE (user_id, github_repo_id)
);
```

### Access Pattern

```
User Request → Check Cache → Hit? → Return cached data (instant)
                    ↓ No
              Singleflight Group
                    ↓
              GitHub API Call → Store → Return
```

### Refresh Strategy

- **Cache-First**: Check database before GitHub API
- **User-Controlled**: `?refresh=true` forces invalidation
- **No TTL**: Cache persists until explicit refresh
- **Singleflight**: Prevents duplicate concurrent requests

## Options Considered

### Option A: No Caching (Direct API)

| Pros         | Cons                             |
| ------------ | -------------------------------- |
| Always fresh | Rapid rate limit consumption     |
| Simple       | 200-500ms latency per request    |
|              | Cascading failures on exhaustion |

**Rejected**: Unacceptable UX and rate limit risk.

### Option B: TTL-Based Cache

Cache with automatic time-based expiration (e.g., 15-minute TTL).

| Pros                  | Cons                           |
| --------------------- | ------------------------------ |
| Automatic freshness   | TTL tuning complexity          |
| Predictable staleness | Cache stampede at TTL boundary |
| Industry-standard     | Background refresh jobs needed |

**Rejected**: TTL boundaries create stampede risk; arbitrary values don't align with actual change frequency.

### Option C: User-Controlled Refresh (Selected)

Cache persists until user explicitly refreshes.

| Pros                    | Cons                              |
| ----------------------- | --------------------------------- |
| Instant loads (0ms hit) | Data may become stale             |
| User agency             | Users must know to refresh        |
| Minimal API consumption | New repos invisible until refresh |
| No TTL complexity       | Storage grows with users          |

**Selected**: Optimal UX for primary use case (selecting existing repos).

### Option D: Redis Cache

External Redis for repository data.

| Pros                    | Cons                      |
| ----------------------- | ------------------------- |
| Sub-millisecond reads   | Additional infrastructure |
| Built-in TTL            | Data loss on restart      |
| Shared across instances | Operational complexity    |

**Rejected**: Over-engineering; PostgreSQL already sufficient.

## Consequences

**Positive:**

- Repository list displays instantly from cache
- API calls only on explicit refresh or first visit
- User control over data freshness
- Hybrid normalization shares org data across users
- Singleflight prevents duplicate in-flight requests
- Cache serves requests during GitHub outages

**Negative:**

- New repos invisible until refresh (mitigated by refresh button)
- Storage cost per user (pruned on user deletion)
- Users must learn refresh pattern (onboarding tooltip)

## Schema Design Rationale

### Hybrid Normalization

| Table                               | Rationale                                                        |
| ----------------------------------- | ---------------------------------------------------------------- |
| Global `github_organizations`       | Orgs are shared; 1000 users in same org don't duplicate metadata |
| Per-user `user_github_repositories` | Visibility is user-specific; permissions vary                    |
| Junction table                      | Clean N:N for "which orgs does user belong to" queries           |

### UNIQUE (user_id, github_repo_id)

- Prevents duplicate cache entries
- Enables upsert (`INSERT ... ON CONFLICT`)
- Maintains referential integrity

### source_type Column

- UI grouping: "Personal" vs "Organization" sections
- Query filtering for org-only views
- Future permission logic differentiation

### No TTL Column

- User-controlled refresh eliminates automatic expiration
- Simpler queries without TTL checks
- No background jobs for enforcement

## References

- [Commit 16056864](https://github.com/specvital/infra/commit/16056864) - Cache tables migration
- [Commit a4db76e8](https://github.com/specvital/web/commit/a4db76e8) - GitHub API module
- [ADR-09: GitHub App Integration](/en/adr/09-github-app-integration.md)
