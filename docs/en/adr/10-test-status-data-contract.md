---
title: TestStatus Data Contract
description: ADR on cross-service TestStatus enum alignment for data integrity
---

# ADR-10: TestStatus Data Contract

> 🇰🇷 [한국어 버전](/ko/adr/10-test-status-data-contract)

| Date       | Author       | Repos             |
| ---------- | ------------ | ----------------- |
| 2024-12-29 | @KubrickCode | core, worker, web |

## Context

### The Data Flow Problem

Specvital processes test metadata through a multi-service pipeline:

```
Core Parser → Worker → Database → Web API → Frontend
```

Each service defines its own `TestStatus` type, creating potential for:

- **Data loss**: Status values dropped during transformation
- **Semantic drift**: Same enum value meaning different things
- **Silent failures**: Mapping errors going unnoticed

### The Incident

During development, a critical data integrity issue was discovered:

```
Core defines:    active, focused, skipped, todo, xfail (5 statuses)
Worker had:   active, skipped, todo (3 statuses)
```

**Impact**:

- `focused` tests were incorrectly mapped to `active`
- `xfail` tests were incorrectly mapped to `todo`
- Users saw inaccurate test counts and missing status indicators

### Why This Matters

| Status  | Semantic Meaning                | User Impact if Lost            |
| ------- | ------------------------------- | ------------------------------ |
| active  | Normal test, will run           | Baseline, no impact            |
| focused | Debug-only test (.only, fit)    | CI warning not triggered       |
| skipped | Intentionally excluded          | Wrong skip count               |
| todo    | Placeholder, not implemented    | Missing TODO tracking          |
| xfail   | Expected to fail (pytest xfail) | Incorrect failure expectations |

## Decision

**Enforce 1:1 TestStatus mapping across all services with no lossy transformations.**

### Canonical Status Definition

All services MUST support exactly these 5 statuses:

```go
// Canonical TestStatus enum (source of truth: core)
type TestStatus string

const (
    TestStatusActive  TestStatus = "active"   // Normal test
    TestStatusFocused TestStatus = "focused"  // .only, fit - debug mode
    TestStatusSkipped TestStatus = "skipped"  // .skip, xit - excluded
    TestStatusTodo    TestStatus = "todo"     // Placeholder test
    TestStatusXfail   TestStatus = "xfail"    // Expected failure
)
```

### Service Alignment

| Service  | Location                                | Status                |
| -------- | --------------------------------------- | --------------------- |
| Core     | `pkg/domain/status.go`                  | Source of truth       |
| Worker   | `internal/domain/analysis/inventory.go` | 1:1 mapping from Core |
| Database | `test_status` ENUM in schema.sql        | 1:1 mapping           |
| Web API  | OpenAPI `TestStatus` schema             | 1:1 mapping           |

## Options Considered

### Option A: String Pass-Through (Rejected)

- Pass status as raw string without enum validation
- **Rejected**: No compile-time safety, typos cause silent failures

### Option B: Subset Mapping (Previous State)

- Worker uses simplified 3-status model
- Map `focused → active`, `xfail → todo`
- **Rejected**: Data loss, semantic corruption

### Option C: Strict 1:1 Mapping (Selected)

- Every service defines identical enum values
- Explicit switch statement with all cases
- Unknown values panic/error (fail-fast)

## Implementation

### Core (Source of Truth)

```go
// pkg/domain/status.go
type TestStatus string

const (
    TestStatusActive  TestStatus = "active"
    TestStatusSkipped TestStatus = "skipped"
    TestStatusTodo    TestStatus = "todo"
    TestStatusFocused TestStatus = "focused"
    TestStatusXfail   TestStatus = "xfail"
)
```

### Worker (Consumer)

```go
// internal/domain/analysis/inventory.go
type TestStatus string

const (
    TestStatusActive  TestStatus = "active"
    TestStatusFocused TestStatus = "focused"
    TestStatusSkipped TestStatus = "skipped"
    TestStatusTodo    TestStatus = "todo"
    TestStatusXfail   TestStatus = "xfail"
)
```

### Mapping Layer

```go
// internal/adapter/mapping/core_domain.go
func convertCoreTestStatus(coreStatus domain.TestStatus) analysis.TestStatus {
    switch coreStatus {
    case domain.TestStatusFocused:
        return analysis.TestStatusFocused
    case domain.TestStatusSkipped:
        return analysis.TestStatusSkipped
    case domain.TestStatusTodo:
        return analysis.TestStatusTodo
    case domain.TestStatusXfail:
        return analysis.TestStatusXfail
    default:
        return analysis.TestStatusActive
    }
}
```

### Database Schema

```sql
CREATE TYPE public.test_status AS ENUM (
    'active',
    'skipped',
    'todo',
    'focused',
    'xfail'
);
```

### Web API (OpenAPI)

```yaml
TestStatus:
  type: string
  enum:
    - active
    - focused
    - skipped
    - todo
    - xfail
  description: |
    Test status indicator:
    - active: Normal test that will run
    - focused: Test marked to run exclusively (e.g., it.only)
    - skipped: Test marked to be skipped (e.g., it.skip)
    - todo: Placeholder test to be implemented
    - xfail: Expected to fail (pytest xfail)
```

## Consequences

### Positive

**Data Integrity**:

- No information loss in the pipeline
- Accurate test counts for all status types
- Reliable CI warnings for focused tests

**Type Safety**:

- Compile-time validation with typed enums
- Explicit mapping prevents silent failures
- IDE autocomplete for status values

**API Clarity**:

- Frontend receives accurate status information
- Consistent behavior across all endpoints
- Self-documenting enum values

### Negative

**Coordination Overhead**:

- Adding new status requires changes in all 4 locations:
  - Core: `pkg/domain/status.go`
  - Worker: `internal/domain/analysis/inventory.go`
  - Database: Migration for ENUM alteration
  - Web: OpenAPI schema update
- Risk of version mismatch during deployment

**Schema Evolution**:

- PostgreSQL ENUM alteration requires migration
- Cannot easily remove status values (only deprecate)
- Order of values in ENUM affects storage

## Guidelines for Future Changes

### Adding a New Status

1. Add to Core `pkg/domain/status.go` first
2. Add to Worker domain and mapping layer
3. Create database migration for ENUM addition
4. Update OpenAPI schema
5. Deploy in order: Database → Worker → Web → Core

### Deprecating a Status

1. Mark as deprecated in documentation
2. Map deprecated status to replacement in Worker
3. Update parsers to stop emitting deprecated status
4. Remove from OpenAPI after migration period

## References

- [PostgreSQL ENUM Types](https://www.postgresql.org/docs/current/datatype-enum.html)
- [OpenAPI Enum Best Practices](https://swagger.io/docs/specification/data-models/enums/)
