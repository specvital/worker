---
title: Repository Visibility-Based Access Control
description: ADR on visibility detection via git ls-remote to isolate private repository data from non-owners
---

# ADR-11: Repository Visibility-Based Access Control

> 🇰🇷 [한국어 버전](/ko/adr/11-community-private-repo-filtering)

| Date       | Author       | Repos              |
| ---------- | ------------ | ------------------ |
| 2026-01-03 | @KubrickCode | infra, worker, web |

## Context

### Security Issue Discovery

While implementing features to share analysis data with other users, a security issue was discovered:

**Problem**: Private repository data analyzed by users can be exposed to non-owners

- Private repository names/owners visible
- Test statistics (counts, pass/fail rates) exposed

**Core Principle**: Users who are not repository owners/analysis requesters must not access private repository data

### Why `is_private` Was Not Stored

Intentional omission of `is_private` in the `codebases` table:

- Repository visibility can change at any time (public ↔ private)
- Stored values can become stale
- GitHub doesn't provide visibility change webhooks (without App installation)

### Risk Assessment

| Scenario                             | Frequency | Risk Level                                |
| ------------------------------------ | --------- | ----------------------------------------- |
| Analyze initially private repo       | Common    | **High** - Must never appear in community |
| public → analyze → switch to private | Rare      | **Medium** - Edge case                    |
| private → analyze → switch to public | Rare      | **Low** - Acceptable to show              |

## Decision

**Determine visibility via git ls-remote result and store `is_private`**

### Core Idea

During analysis, the worker fetches the latest commit via `git ls-remote`. Leverage this existing logic:

1. **Try without token first** → Success means **public**
2. **On failure, try with user token** → Success means **private**

This allows natural visibility detection without additional GitHub API calls.

### Key Principles

1. **Token-less first**: Always attempt public access first
2. **Token only when needed**: Use user token only on failure
3. **Capture at analysis time**: Store result as `is_private`
4. **Filter in queries**: Exclude `is_private = true` from Community view

## Options Considered

### Option A: "Share to Community" Checkbox (Rejected)

User explicitly consents to public sharing during analysis.

**Pros**: Perfect privacy, consent-based

**Cons**:

- UX friction reduces content (opt-in rate ~5-15%)
- Hinders community growth for new platform

**Decision**: Excluded from initial implementation. Can add as opt-out later.

### Option B: Real-time GitHub API Check (Rejected)

Check current visibility via GitHub API on every request.

**Pros**: Always accurate information

**Cons**:

- Rate limit (5000/hour)
- Page load latency
- Increased complexity

**Decision**: Rejected. Unrealistic at scale.

### Option C: git ls-remote Based Detection (Selected)

Use existing git ls-remote call to determine visibility.

**Pros**:

- No additional API calls
- Accurate at analysis time
- Simple implementation

**Cons**:

- Stale if visibility changes post-analysis

## Implementation

### Database Schema (infra)

```sql
-- Add is_private to codebases table
ALTER TABLE codebases
ADD COLUMN is_private BOOLEAN NOT NULL DEFAULT false;

-- Partial index for efficient filtering
CREATE INDEX idx_codebases_is_private
ON codebases(is_private)
WHERE is_private = false;
```

### git ls-remote Logic Change (worker)

**File**: `src/internal/adapter/vcs/git.go`

Current logic:

```go
// Try with token first if available
if token != nil {
    sha, err := GetHeadCommit(ctx, url, token)
    if err == nil { return sha, nil }
}
// Fall back to tokenless
return GetHeadCommit(ctx, url, nil)
```

After change:

```go
// 1. Try without token first (public check)
sha, err := GetHeadCommit(ctx, url, nil)
if err == nil {
    return &CommitInfo{SHA: sha, IsPrivate: false}, nil
}

// 2. On failure, try with token (private repo)
if token != nil {
    sha, err = GetHeadCommit(ctx, url, token)
    if err == nil {
        return &CommitInfo{SHA: sha, IsPrivate: true}, nil
    }
}
```

### Community Query Filter (web backend)

**File**: `queries/analysis.sql`

```sql
-- Add to Community view WHERE clause
AND (
    sqlc.arg(view_filter)::text = 'community'
    AND c.is_private = false  -- public repos only
    AND NOT EXISTS(...)
)
```

### User Notice (web frontend)

```tsx
// explore-content.tsx
<p className="text-sm text-muted-foreground">{t("community.visibilityDisclosure")}</p>
```

```json
// messages/en.json
{
  "explore": {
    "community": {
      "visibilityDisclosure": "Only public repositories at the time of analysis are shown."
    }
  }
}
```

### Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Analysis Flow                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  User requests analysis                                              │
│         │                                                            │
│         ▼                                                            │
│  ┌─────────────┐                                                    │
│  │     web     │  Register job in queue                             │
│  └──────┬──────┘                                                    │
│         │                                                            │
│         ▼                                                            │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                      worker                               │    │
│  │                                                              │    │
│  │  git ls-remote (without token)                               │    │
│  │  ├─ Success → isPrivate = false (public)                     │    │
│  │  └─ Failure → git ls-remote (with token)                     │    │
│  │            └─ Success → isPrivate = true (private)           │    │
│  │                                                              │    │
│  │  git clone → analyze → save (including is_private)           │    │
│  └─────────────────────────────────────────────────────────────┘    │
│         │                                                            │
│         ▼                                                            │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                      PostgreSQL                              │    │
│  │  codebases: { id, owner, name, is_private, ... }            │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                       Community View Flow                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Unauthenticated user visits /explore                               │
│         │                                                            │
│         ▼                                                            │
│  ┌─────────────┐                                                    │
│  │   web API   │                                                    │
│  └──────┬──────┘                                                    │
│         │  SELECT ... WHERE is_private = false                      │
│         ▼                                                            │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                      PostgreSQL                              │    │
│  │  Returns only public repositories                            │    │
│  └─────────────────────────────────────────────────────────────┘    │
│         │                                                            │
│         ▼                                                            │
│  User sees only public repos in community tab                       │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Consequences

### Positive

**Security**:

- Private repository metadata no longer exposed to public
- Covers majority of use cases (initially private repos)
- No additional API overhead

**Simplicity**:

- Leverages existing git ls-remote call
- Single boolean column addition
- Query-level filtering

### Negative

**No Real-time Sync**:

- Visibility changes not reflected until re-analysis
- However, if public at analysis time, that data was publicly accessible then, so continued exposure is justified

## Security Assessment

| Criterion           | Score | Notes             |
| ------------------- | ----- | ----------------- |
| Post-implementation | 8/10  | Covers most cases |

### Design Principles

- **Analysis-time determination**: Visibility is fixed at analysis time
- **Past public data legitimacy**: If public at analysis time, that data was publicly accessible then, so continued exposure is valid
- **private → public transition**: Remains hidden until re-analysis (conservative approach)

## References

- [GitHub REST API - Repositories](https://docs.github.com/en/rest/repos/repos)
- [git ls-remote documentation](https://git-scm.com/docs/git-ls-remote)
