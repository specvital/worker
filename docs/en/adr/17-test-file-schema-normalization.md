---
title: Test File Schema Normalization
description: ADR for normalizing the test data schema with test_files table for file-level metadata
---

# ADR-17: Test File Schema Normalization

> 🇰🇷 [한국어 버전](/ko/adr/17-test-file-schema-normalization.md)

| Date       | Author       | Repos              |
| ---------- | ------------ | ------------------ |
| 2026-01-19 | @KubrickCode | infra, worker, web |

## Context

### The File-Level Metadata Problem

The existing test data schema uses a 3-tier hierarchy:

```
analyses → test_suites (file_path, framework) → test_cases
```

This structure has two deficiencies:

| Issue          | Description                                                                                                        |
| -------------- | ------------------------------------------------------------------------------------------------------------------ |
| Redundant Data | `file_path` and `framework` stored per test_suite, causing duplication when a single file contains multiple suites |
| Missing Entity | No logical attachment point for file-level metadata                                                                |

### DomainHints Requirement

The AI-based SpecView generation pipeline ([ADR-14](/en/adr/14-ai-spec-generation-pipeline.md)) requires domain classification using `DomainHints` extracted from test files ([Core ADR-16](/en/adr/core/16-domain-hints-extraction.md)). These hints are inherently file-level data:

```go
type DomainHints struct {
    Imports []string  // Per-file import statements
    Calls   []string  // Per-file function calls
}
```

Without schema normalization, storing `domain_hints` in `test_suites` would:

- Duplicate JSONB data for each suite in a file
- Create update anomalies when hints change
- Waste storage proportional to suite count

### Constraints

| Constraint             | Impact                                                         |
| ---------------------- | -------------------------------------------------------------- |
| Backward Compatibility | All existing analyses must migrate without data loss           |
| CASCADE DELETE         | Entire hierarchy must clean up via FK relationships            |
| Query Performance      | Web API queries must not degrade significantly                 |
| Storage Flow           | Worker must insert files before suites (sequential dependency) |

## Decision

**Normalize from 3-tier to 4-tier schema by introducing `test_files` table between `analyses` and `test_suites`.**

### Schema Design

**New test_files table:**

```sql
CREATE TABLE test_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id UUID NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    framework TEXT NOT NULL,
    domain_hints JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (analysis_id, file_path)
);

CREATE INDEX idx_test_files_analysis_id ON test_files(analysis_id);
```

**Modified test_suites:**

```sql
-- After migration:
ALTER TABLE test_suites
    ADD COLUMN file_id UUID REFERENCES test_files(id) ON DELETE CASCADE,
    DROP COLUMN analysis_id,
    DROP COLUMN file_path,
    DROP COLUMN framework;
```

### New Hierarchy

```
analyses
    └── test_files (file_path, framework, domain_hints)
            └── test_suites (suite_name)
                    └── test_cases (test_name, status)
```

### Migration Strategy

| Phase | Action                                    | Risk                |
| ----- | ----------------------------------------- | ------------------- |
| 1     | Create `test_files` table                 | None                |
| 2     | Populate: `INSERT FROM SELECT DISTINCT`   | Low - idempotent    |
| 3     | Add `file_id` FK to test_suites           | Medium              |
| 4     | Verify all test_suites have valid file_id | None                |
| 5     | Drop redundant columns from test_suites   | High - irreversible |
| 6     | Add NOT NULL constraint on file_id        | None                |

**Rollback Strategy**: Before step 5, rollback is trivial. After step 5, requires data reconstruction.

## Options Considered

### Option A: test_files Normalization Layer (Selected)

Introduce intermediate `test_files` table to normalize file-level data.

**Pros:**

| Benefit              | Description                                                  |
| -------------------- | ------------------------------------------------------------ |
| Data Integrity       | Single source of truth for file metadata                     |
| Storage Efficiency   | Eliminates duplication of file_path, framework, domain_hints |
| FK Hierarchy         | Clean CASCADE DELETE chain                                   |
| Future Extensibility | File-level metrics (coverage, complexity) have natural home  |

**Cons:**

| Trade-off           | Mitigation                                    |
| ------------------- | --------------------------------------------- |
| Query Complexity    | One additional JOIN; acceptable for integrity |
| Migration Effort    | One-time lossless migration                   |
| Storage Flow Change | Worker inserts files before suites            |

### Option B: Store domain_hints in test_suites

Add `domain_hints` column directly to existing test_suites table.

**Pros:**

- No migration needed
- No query changes required

**Cons:**

| Issue            | Severity                                        |
| ---------------- | ----------------------------------------------- |
| Data duplication | High - hints repeated per suite                 |
| Update anomalies | High - changing hints requires multiple updates |
| Storage waste    | Medium - JSONB duplicated                       |
| 3NF violation    | Architectural debt                              |

**Verdict:** Rejected. Violates 3NF; creates update anomalies and storage waste.

### Option C: Separate file_domain_hints Table

Create parallel table for domain hints only, without modifying test_suites.

**Pros:**

- Hints normalized separately
- Additive change only

**Cons:**

| Issue                            | Severity                                |
| -------------------------------- | --------------------------------------- |
| Parallel structures              | High - file_path in two tables          |
| No referential integrity         | Medium - hints disconnected from suites |
| Existing duplication unaddressed | High - file_path still redundant        |

**Verdict:** Rejected. Does not address existing duplication; creates architectural inconsistency.

## Consequences

### Positive

**Data Integrity:**

- Single source of truth for `file_path`, `framework`, `domain_hints`
- UNIQUE constraint on `(analysis_id, file_path)` prevents duplicates
- Clean CASCADE DELETE: analyses → test_files → test_suites → test_cases

**AI Pipeline Integration:**

- `domain_hints` has natural home at file level
- Aligns with Core ADR-16's file-level extraction model
- Enables per-file caching in AI pipeline

**Future Extensibility:**

- File-level coverage metrics have attachment point
- File complexity scores can be added
- Per-file analysis status possible

### Negative

**Query Complexity:**

- All test queries require additional JOIN through test_files
- Example change:

```sql
-- Before:
SELECT ts.file_path FROM test_suites ts
WHERE ts.analysis_id = $1

-- After:
SELECT tf.file_path FROM test_suites ts
JOIN test_files tf ON ts.file_id = tf.id
WHERE tf.analysis_id = $1
```

**Migration Effort:**

- One-time orchestrated migration required
- Both worker and web services need interface updates
- Repository pattern implementations change

**Worker Storage Flow:**

- Must insert files before suites (sequential dependency)
- Two-phase write: `saveFiles()` then `saveSuitesBatch()`
- Storage method signature changes

### Technical Implications

| Aspect           | Implication                                                  |
| ---------------- | ------------------------------------------------------------ |
| JSONB Storage    | domain_hints uses PostgreSQL JSONB type                      |
| Index Strategy   | Primary lookup by analysis_id                                |
| Query Pattern    | JOIN chain: test_cases → test_suites → test_files → analyses |
| Worker Interface | `saveSuitesBatch()` now takes file_id instead of analysis_id |

## References

- [ADR-14: AI-Based Spec Document Generation Pipeline](/en/adr/14-ai-spec-generation-pipeline.md) - Motivates DomainHints requirement
- [Core ADR-16: Domain Hints Extraction System](/en/adr/core/16-domain-hints-extraction.md) - Defines DomainHints structure
- [Worker ADR-07: Repository Pattern](/en/adr/worker/07-repository-pattern.md) - Affects storage implementation
