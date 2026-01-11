---
title: Repository Pattern
description: ADR on Repository pattern for data access abstraction and testability
---

# ADR-07: Repository Pattern Data Access Abstraction

> 🇰🇷 [한국어 버전](/ko/adr/worker/07-repository-pattern.md)

| Date       | Author       | Repos  |
| ---------- | ------------ | ------ |
| 2024-12-18 | @KubrickCode | worker |

## Context

### Problem

Direct database queries scattered across UseCase layer create several issues:

**Tight Coupling:**

- UseCase directly depends on PostgreSQL-specific code (pgx, pgtype)
- Changing database vendor requires modifying business logic
- SQL queries mixed with business workflow orchestration

**Testing Difficulty:**

- Unit testing requires database connection or complex mocking
- Can't test business logic in isolation
- Integration tests needed even for simple rule verification

**Code Organization:**

- No clear boundary between data access and business logic
- Query optimization concerns leak into UseCase
- Duplicated query patterns across different usecases

### Goals

1. **Abstraction**: Hide database implementation details from UseCase
2. **Testability**: Enable unit testing with simple mock implementations
3. **Maintainability**: Centralize data access logic in dedicated layer
4. **Domain Alignment**: Express data operations in domain terms

## Decision

**Adopt Repository pattern with domain-centric interfaces defined in Domain layer.**

### Interface Design

```go
// domain/analysis/repository.go
type Repository interface {
    CreateAnalysisRecord(ctx context.Context, params CreateAnalysisRecordParams) (UUID, error)
    RecordFailure(ctx context.Context, analysisID UUID, errMessage string) error
    SaveAnalysisInventory(ctx context.Context, params SaveAnalysisInventoryParams) error
}
```

### Key Characteristics

| Aspect                  | Decision                                       |
| ----------------------- | ---------------------------------------------- |
| Interface Location      | Domain layer (`domain/analysis/repository.go`) |
| Implementation Location | Adapter layer (`adapter/repository/postgres/`) |
| Transaction Scope       | Per-method (each method is atomic)             |
| Parameter Style         | Value Objects with validation                  |
| Error Handling          | Domain errors + wrapped infrastructure errors  |

## Options Considered

### Option A: Repository Pattern (Selected)

**Description:**

Define interfaces in Domain layer, implement in Adapter layer. Each method represents a complete, atomic operation.

**Pros:**

- Clear separation between domain logic and persistence
- UseCase depends only on abstractions
- Easy to mock for unit testing
- Implementation can change without affecting business logic

**Cons:**

- Additional abstraction layer
- Risk of "repository bloat" with many methods
- May need to balance between fine-grained and coarse-grained operations

### Option B: Query Object Pattern

**Description:**

Create query objects that encapsulate specific queries, passed to a generic executor.

**Pros:**

- Very flexible query composition
- Reusable query fragments

**Cons:**

- More complex API surface
- Query objects may leak persistence details
- Harder to understand data flow

### Option C: Active Record Pattern

**Description:**

Domain objects contain their own persistence methods.

**Pros:**

- Simple and intuitive for CRUD operations
- Less ceremony for small domains

**Cons:**

- Domain objects become heavy with persistence logic
- Violates Single Responsibility Principle
- Tight coupling between domain and infrastructure
- Difficult to test domain logic in isolation

## Implementation Principles

### Interface Definition in Domain Layer

Interfaces are defined where they are **used**, not where they are implemented:

```
domain/
  analysis/
    repository.go      ← Interface definition (Repository)
    autorefresh.go     ← Extended interface (AutoRefreshRepository)

adapter/
  repository/
    postgres/
      analysis.go      ← PostgreSQL implementation
```

**Rationale:**

- Domain layer remains free of infrastructure dependencies
- Dependency Inversion: high-level modules define contracts
- Implementation details contained in Adapter layer

### Value Object Parameters

Instead of primitive parameters, use validated Value Objects:

```go
type CreateAnalysisRecordParams struct {
    AnalysisID *UUID    // Optional: use provided ID or generate new
    Branch     string
    CommitSHA  string
    Owner      string
    Repo       string
}

func (p CreateAnalysisRecordParams) Validate() error {
    if p.Owner == "" {
        return fmt.Errorf("%w: owner is required", ErrInvalidInput)
    }
    // ... validation logic
}
```

**Benefits:**

- Self-documenting method signatures
- Validation logic co-located with data
- Easy to extend without breaking API
- Clear distinction between required and optional fields

### Per-Method Transaction Scope

Each repository method is a complete, atomic operation:

```go
func (r *AnalysisRepository) CreateAnalysisRecord(ctx context.Context, params ...) (UUID, error) {
    tx, err := r.pool.Begin(ctx)
    if err != nil {
        return NilUUID, fmt.Errorf("begin transaction: %w", err)
    }
    defer tx.Rollback(ctx)  // Safe: no-op if committed

    // ... operations within transaction

    if err := tx.Commit(ctx); err != nil {
        return NilUUID, fmt.Errorf("commit transaction: %w", err)
    }
    return result, nil
}
```

**Rationale:**

- Simpler mental model: each method either succeeds or fails completely
- No transaction leakage across method boundaries
- UseCase doesn't need to manage transaction lifecycle
- Context cancellation automatically handled

### Error Message Truncation

Long error messages are truncated before storage:

```go
const maxErrorMessageLength = 1000

func truncateErrorMessage(msg string) string {
    if len(msg) <= maxErrorMessageLength {
        return msg
    }
    return msg[:maxErrorMessageLength-15] + "... (truncated)"
}
```

**Rationale:**

- Database column has size limit
- Prevents query failures from oversized data
- Preserves useful portion of error message
- Indicates truncation occurred

### External Analysis ID Support

Repository supports optional externally-provided IDs:

```go
type CreateAnalysisRecordParams struct {
    AnalysisID *UUID  // If nil, generate new UUID; if provided, use it
    // ...
}

// Implementation
analysisID := analysis.NewUUID()
if params.AnalysisID != nil {
    analysisID = *params.AnalysisID
}
```

**Use Case:**

- Web service creates Analysis record with known ID
- Worker receives this ID in task payload
- Worker uses same ID when saving results
- Enables correlation between systems

## Consequences

### Positive

**Testability:**

```go
// Easy to mock for unit tests
type MockRepository struct {
    CreateAnalysisRecordFn func(...) (UUID, error)
}

func (m *MockRepository) CreateAnalysisRecord(...) (UUID, error) {
    return m.CreateAnalysisRecordFn(...)
}
```

**Flexibility:**

- Can swap PostgreSQL for another database
- Implementation changes don't affect UseCase tests
- Can add caching layer transparently

**Maintainability:**

- All SQL queries in one location
- Query optimization isolated to Adapter layer
- Clear responsibility boundaries

### Negative

**Abstraction Overhead:**

- Additional interface and implementation files
- Some duplication between params and DB structs
- Mapping required between domain and persistence models

**Method Proliferation:**

- New data access patterns require new methods
- Risk of repository becoming a "god object"
- May need to split into specialized repositories

**Transaction Limitations:**

- Cross-method transactions not supported by interface
- Complex workflows may need coordination in Adapter layer

## References

- [ADR-02: Clean Architecture Layers](./02-clean-architecture-layers.md) - Overall layer structure
- [Repository Pattern by Martin Fowler](https://martinfowler.com/eaaCatalog/repository.html)
- [Domain-Driven Design by Eric Evans](https://www.domainlanguage.com/ddd/) - Repository pattern origin
