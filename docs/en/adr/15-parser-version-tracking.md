---
title: Parser Version Tracking
description: ADR for automatic re-analysis trigger based on parser version changes
---

# ADR-15: Parser Version Tracking for Re-analysis

> [Korean Version](/ko/adr/15-parser-version-tracking.md)

| Date       | Author       | Repos         |
| ---------- | ------------ | ------------- |
| 2026-01-18 | @KubrickCode | infra, worker |

## Context

### The Parser Upgrade Blind Spot

The scheduled re-collection system (Worker ADR-01) monitors repository changes via commit SHA comparison. However, this creates a blind spot: when specvital/core is upgraded with parser improvements, existing analyses remain unchanged because the repository commit hasn't changed.

**Problem Scenario:**

```
1. User analyzes repository at commit `abc123` → Analysis stored with test count 50
2. Core v1.2.0 releases with improved Jest detection
3. Same repository, same commit `abc123` would now report 55 tests
4. User sees stale count (50) until repository makes a new commit
```

**Impact:**

| Issue                 | Consequence                          |
| --------------------- | ------------------------------------ |
| Parser bug fixes      | Invisible to existing analyses       |
| New framework support | Unavailable until manual re-analysis |
| Accuracy improvements | Users unaware their data is stale    |

### Requirements

| Requirement              | Description                                        |
| ------------------------ | -------------------------------------------------- |
| Automatic Detection      | Identify when parser version differs from analysis |
| Zero Manual Intervention | No deployment-time configuration required          |
| Backward Compatibility   | Handle existing analyses without version info      |
| Integration              | Work with existing auto-refresh infrastructure     |

## Decision

**Implement runtime parser version extraction using Go's `debug.ReadBuildInfo()` with database-tracked version comparison.**

### 1. Version Registration

Worker extracts core module version at startup, UPSERTs to `system_config` table:

```go
// buildinfo/version.go
func ExtractCoreVersion() string {
    info, ok := debug.ReadBuildInfo()
    if !ok {
        return "unknown"
    }
    for _, dep := range info.Deps {
        if dep.Path == "github.com/specvital/core" {
            return dep.Version  // e.g., "v1.2.3"
        }
    }
    return "unknown"
}
```

### 2. Version Recording

Each analysis record stores the parser version that produced it:

```sql
ALTER TABLE analyses ADD COLUMN parser_version VARCHAR(100) DEFAULT 'legacy';
```

### 3. Version Comparison

Auto-refresh logic compares analysis parser version against current system version:

```go
func (uc *AutoRefreshUseCase) shouldEnqueueRefresh(
    codebase CodebaseRefreshInfo,
    headCommitSHA string,
    currentParserVersion string,
) bool {
    // Trigger 1: New commit detected
    if codebase.LastCommitSHA != headCommitSHA {
        return true
    }
    // Trigger 2: Parser version changed
    if currentParserVersion != "" &&
       codebase.LastParserVersion != currentParserVersion {
        return true
    }
    return false
}
```

### 4. Constraint Update

Unique constraint includes parser version, allowing same commit to have multiple analyses:

```sql
-- Old: (codebase_id, commit_sha)
-- New: (codebase_id, commit_sha, parser_version)
CREATE UNIQUE INDEX uq_analyses_completed_commit_version
    ON analyses (codebase_id, commit_sha, parser_version)
    WHERE status = 'completed';
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Worker Startup                               │
├─────────────────────────────────────────────────────────────────┤
│  1. Extract: runtime/debug.ReadBuildInfo()                       │
│  2. Find: specvital/core module version                          │
│  3. UPSERT: system_config {key: "parser_version", value: X}      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Analysis Creation                             │
├─────────────────────────────────────────────────────────────────┤
│  analyses.parser_version = injected version from ContainerConfig │
│  → Enables historical tracking per analysis                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Scheduled Auto-Refresh                         │
├─────────────────────────────────────────────────────────────────┤
│  1. Query: system_config.parser_version (current)                │
│  2. Query: codebases with last_parser_version (from analyses)    │
│  3. Compare: if different → enqueue re-analysis                  │
│  4. Graceful degradation: skip version check on query failure    │
└─────────────────────────────────────────────────────────────────┘
```

## Options Considered

### Option A: Runtime Version Extraction (Selected)

Extract specvital/core module version from Go build info at Worker startup.

| Aspect     | Assessment                                     |
| ---------- | ---------------------------------------------- |
| Automation | Automatic detection from compiled binary       |
| Accuracy   | Reflects actual module version in use          |
| Debugging  | Semantic versions correlate with release notes |
| Limitation | Go-specific, `(devel)` in local builds         |

### Option B: Manual Version Configuration

Set parser version via environment variable or config file.

| Aspect      | Assessment                       |
| ----------- | -------------------------------- |
| Portability | Language agnostic                |
| Control     | Explicit version string          |
| Risk        | Easy to forget during deployment |
| Friction    | Additional step per release      |

### Option C: Content-Based Hashing

Hash parser outputs on reference codebases to detect behavioral changes.

| Aspect      | Assessment                         |
| ----------- | ---------------------------------- |
| Detection   | Actual behavioral changes          |
| Overhead    | Compute intensive                  |
| Debugging   | Hash doesn't indicate what changed |
| Reliability | Non-deterministic edge cases       |

## Consequences

### Positive

| Area           | Benefit                                       |
| -------------- | --------------------------------------------- |
| Parser Upgrade | Users automatically receive improved analysis |
| Operations     | Zero deployment-time configuration            |
| Audit Trail    | Each analysis linked to producing version     |
| Data Model     | Multiple analyses per commit supported        |

### Negative

| Area        | Trade-off                                   |
| ----------- | ------------------------------------------- |
| Portability | Go-specific `runtime/debug` dependency      |
| Development | `(devel)` version requires fallback logic   |
| Storage     | Additional ~20 bytes per analysis row       |
| Migration   | Legacy analyses default to `'legacy'` value |

### Technical Implications

| Aspect               | Implication                                           |
| -------------------- | ----------------------------------------------------- |
| Schema Migration     | Existing analyses require default value handling      |
| Build Requirements   | `CGO_ENABLED=0` to retain module info                 |
| Graceful Degradation | Version check skipped if query fails                  |
| A/B Testing          | Same commit analyzable with different parser versions |

## References

- [Worker ADR-01: Scheduled Re-collection Architecture](/en/adr/worker/01-scheduled-recollection.md)
- [Go debug.ReadBuildInfo Documentation](https://pkg.go.dev/runtime/debug)
- Commits: `a681e0d0` (infra), `290a5efa`, `aa47dab0`, `4cc6cc43` (worker)
