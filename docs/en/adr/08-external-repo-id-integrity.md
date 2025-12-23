---
title: External Repo ID Integrity
description: ADR on external repository ID-based data integrity verification
---

# ADR-08: External Repository ID-Based Data Integrity

> 🇰🇷 [한국어 버전](/ko/adr/08-external-repo-id-integrity.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-22 | @KubrickCode | all   |

## Context

### Problem

Current repository identification uses `UNIQUE (host, owner, name)` constraint. This fails in several scenarios:

**Scenario: Delete and Recreate**

```
1. Repo A: alice/my-repo (external_repo_id: 100) → analyzed
2. Repo A deleted
3. Repo B created: alice/my-repo (external_repo_id: 200)
4. Scheduler requests re-analysis of alice/my-repo
5. Clone succeeds (Repo B)
6. Analysis results saved to Repo A's row
   → Data corruption!
```

**Additional Scenarios**

| Scenario                         | Problem                                           |
| -------------------------------- | ------------------------------------------------- |
| Rename (alice/old → alice/new)   | History disconnected on new name                  |
| Transfer (alice/repo → bob/repo) | History disconnected on owner change              |
| Delete and recreate              | Different repo data contaminates existing history |

### Goals

1. **Data Integrity**: Prevent analysis results from wrong repository being saved
2. **History Continuity**: Maintain analysis history across rename/transfer
3. **API Efficiency**: Minimize VCS API calls (rate limit concerns)

## Decision

**Adopt dual verification mechanism: `external_repo_id` for identity + `git fetch SHA` for integrity verification.**

### Core Principle

> **Re-analysis verifies integrity via `git fetch <last_commit_sha>` without API calls**

### Mechanism Combination

| Mechanism                      | Purpose                         | API Required       |
| ------------------------------ | ------------------------------- | ------------------ |
| `external_repo_id`             | Link history on rename/transfer | Yes (new analysis) |
| `git fetch <sha>` verification | Confirm same repository         | No                 |

### git fetch SHA Verification

```bash
# Check if last analyzed commit exists in current repo
git fetch --depth 1 origin <last_commit_sha>

# Result
# - Success: Same repo (commit exists)
# - Failure: Different repo (delete+recreate) or force push
```

**Error message** (when commit doesn't exist):

```
fatal: remote error: upload-pack: not our ref <sha>
```

## Options Considered

### Option A: Always Call VCS API (Rejected)

**Description:** Call API on every analysis to get repository ID and verify.

**Pros:**

- Simple implementation
- Always accurate

**Cons:**

- Rate limit exhaustion (5000/hr for GitHub)
- Increased latency
- Not scalable for frequent re-analysis

### Option B: git fetch SHA Only (Rejected)

**Description:** Use only git fetch verification without external_repo_id.

**Pros:**

- Zero API calls
- Simple

**Cons:**

- Cannot detect rename/transfer (history disconnection)
- Force push indistinguishable from delete+recreate

### Option C: Dual Mechanism (Selected)

**Description:** Combine external_repo_id storage with git fetch SHA verification.

**Pros:**

- API calls only when necessary (new analysis, verification failure)
- Rename/transfer detection via external_repo_id
- Force push vs delete+recreate differentiation
- Scalable (most re-analyses need zero API calls)

**Cons:**

- More complex implementation
- Requires schema changes

## Implementation

### Case Classification

| Case | Condition                             | Result                              |
| ---- | ------------------------------------- | ----------------------------------- |
| A    | Not in DB, external_repo_id not found | Create new codebase                 |
| B    | In DB, git fetch success              | Re-analyze existing codebase        |
| D    | Not in DB, external_repo_id exists    | Update owner/name (rename/transfer) |
| E    | In DB, git fetch fail, ID differs     | Mark stale + create new             |
| F    | In DB, git fetch fail, ID same        | Force push, re-analyze existing     |

### Flow

```
[Analysis Request: owner/repo]
      │
      ├─ 1. Clone
      │
      ├─ 2. DB lookup (owner, name)
      │      │
      │      ├─ Not found ─────────────────────────┐
      │      │                                     │
      │      └─ Found                              │
      │           │                               │
      │           ├─ 3. git fetch <last_sha>      │
      │           │      │                        │
      │           │      ├─ Success               │
      │           │      │    → Proceed           │
      │           │      │                        │
      │           │      └─ Failure               │
      │           │           │                   │
      │           │           ▼                   │
      │           └───────────┴───────────────────┤
      │                                           │
      │                       ┌───────────────────┘
      │                       │
      │                       ▼
      │              4. VCS API call
      │                 → external_repo_id
      │                       │
      │                       ▼
      │              5. DB lookup (external_repo_id)
      │                       │
      │              ┌────────┴────────┐
      │              │                 │
      │           Found             Not found
      │              │                 │
      │              ▼                 ▼
      │        Update              Create new
      │        owner/name          codebase
      │              │                 │
      │              └────────┬────────┘
      │                       │
      │                       ▼
      └──────────────→ 6. Analyze & Save
```

### Schema Changes

```sql
-- Add columns
ALTER TABLE codebases ADD COLUMN external_repo_id VARCHAR(64);
ALTER TABLE codebases ADD COLUMN is_stale BOOLEAN DEFAULT false;

-- Partial unique index for owner/name (excludes stale)
CREATE UNIQUE INDEX idx_codebases_owner_name
ON codebases(host, owner, name)
WHERE is_stale = false;

-- Unique index for external_repo_id
CREATE UNIQUE INDEX idx_codebases_external_repo_id
ON codebases(host, external_repo_id);
```

**VARCHAR(64) Rationale:**

| Platform  | Type    | Example                                  |
| --------- | ------- | ---------------------------------------- |
| GitHub    | BIGINT  | `123456789`                              |
| GitLab    | INTEGER | `12345678`                               |
| Bitbucket | UUID    | `{550e8400-e29b-41d4-a716-446655440000}` |

All types stored as strings for uniformity.

### Race Condition Handling

**Clone-Rename Race:**

```
T1: Worker clones alice/old-repo
T2: User renames alice/old-repo → alice/new-repo
T3: Worker completes clone (old-repo code)
T4: Worker calls API → external_repo_id: 100
T5: DB lookup id=100 → shows alice/new-repo
T6: Worker saves old-repo code to new-repo
    → Data corruption!
```

**Solution:** Compare clone-time owner/name with API result

```go
if existingCodebase.Owner != req.Owner || existingCodebase.Name != req.Name {
    return ErrRaceConditionDetected // Trigger retry
}
```

**Concurrent Analysis:**

- Use `(host, external_repo_id)` unique constraint to prevent duplicate creation
- Application layer case-by-case handling (no blind UPSERT)

### Stale Policy

| Item        | Value                         |
| ----------- | ----------------------------- |
| Retention   | 30 days                       |
| UI Display  | "Repository no longer exists" |
| Auto-delete | After 30 days                 |

## Consequences

### Positive

**Data Integrity:**

- Delete+recreate scenario correctly handled
- Force push distinguished from identity change
- History preserved across rename/transfer

**Efficiency:**

- Most re-analyses require zero API calls
- Rate limit burden minimized
- Scalable to millions of repositories

**Competitive Advantage:**

- Unlike Codecov/Coveralls, automatic history linking on rename
- No manual reconfiguration needed

### Negative

**Complexity:**

- 6 case classifications to implement
- Schema migration required
- Race condition handling needed

**Migration:**

- Existing codebases need external_repo_id backfill
- Phased deployment required (nullable → backfill → NOT NULL)

**Platform Dependency:**

- Bitbucket Cloud git fetch SHA support uncertain
- GitLab self-hosted requires `uploadpack.allowReachableSHA1InWant`

## Platform Support

| Platform         | git fetch SHA     | Tested        |
| ---------------- | ----------------- | ------------- |
| GitHub           | Supported         | Direct test   |
| GitLab           | Supported         | Direct test   |
| Bitbucket Server | Supported (v5.5+) | Docs verified |
| Bitbucket Cloud  | Uncertain         | Needs testing |

## API Call Frequency

| Case                 | API Calls | Frequency |
| -------------------- | --------- | --------- |
| New analysis         | 1         | Low       |
| Re-analysis (normal) | 0         | High      |
| Scheduler (normal)   | 0         | High      |
| Delete+recreate      | 1         | Very low  |
| Force push           | 1         | Very low  |
| Rename/Transfer      | 1         | Very low  |

**Most cases require no API calls** → Rate limit burden minimized

## References

- [ADR-05: Repository Pattern](./collector/07-repository-pattern.md) - Data access abstraction
- [ADR-03: Worker-Scheduler Separation](./collector/05-worker-scheduler-separation.md) - Process architecture
- [GitHub API Rate Limits](https://docs.github.com/en/rest/rate-limit)
