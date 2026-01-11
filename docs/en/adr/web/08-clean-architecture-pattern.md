---
title: Clean Architecture Pattern
description: ADR on adopting 5-layer Clean Architecture for separation of concerns, testability, and AI-assisted development
---

# ADR-08: Clean Architecture Pattern

> [Korean Version](/ko/adr/web/08-clean-architecture-pattern.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-01-03 | @KubrickCode | web   |

## Context

### Initial Architecture Challenges

The initial service-oriented structure presented several challenges as the codebase grew:

**Domain-Infrastructure Coupling:**

- Business logic was intertwined with database queries and HTTP handling
- Changes to infrastructure details (PostgreSQL, River queue) required modifications to service layer
- HTTP status codes were directly returned from service layer, violating separation of concerns

**Limited Testability:**

- Unit testing was difficult due to direct dependencies on concrete implementations
- Integration tests were required even for simple business rule verification
- Mock injection was not possible without significant refactoring

**AI-Assisted Development Constraints:**

- Large service files exceeded AI context windows, reducing LLM coding effectiveness
- Cross-cutting concerns made it difficult for AI tools to understand modification scope
- No clear boundaries for AI agents to work within bounded contexts

### Evolution Timeline

| Change                                          | Motivation                                 |
| ----------------------------------------------- | ------------------------------------------ |
| Introduce Service layer + StrictServerInterface | Extract business logic from handler        |
| Decouple HTTP status codes from service         | Service layer was returning HTTP codes     |
| Introduce domain layer with errors + models     | Centralize domain definitions              |
| Apply Clean Architecture domain layer           | entity/ + port/ separation                 |
| Apply Clean Architecture usecase layer          | Feature-specific use cases                 |
| Apply Clean Architecture adapter layer          | Repository, Queue, Client implementations  |
| Apply Clean Architecture handler layer          | Complete handler -> usecase -> domain flow |

### Alignment with Worker Service

The Worker service already adopted a 6-layer Clean Architecture ([Worker ADR-02](/en/adr/worker/02-clean-architecture-layers.md)). Adopting a similar structure for Web backend ensures:

- Consistent mental model across repositories
- Reusable patterns for team members
- Shared testing strategies

## Decision

**Adopt a 5-layer Clean Architecture for the Web backend.**

### Layer Structure

| Layer       | Location         | Responsibility                               |
| ----------- | ---------------- | -------------------------------------------- |
| **Entity**  | `domain/entity/` | Pure business models, value objects          |
| **Port**    | `domain/port/`   | Interface definitions (DIP contracts)        |
| **UseCase** | `usecase/`       | Business logic, feature orchestration        |
| **Adapter** | `adapter/`       | External implementations (DB, API, Queue)    |
| **Handler** | `handler/`       | HTTP entry points, request/response handling |

### Dependency Direction

```
handler -> usecase -> domain <- adapter
                        ^
                (implements port)
```

- **Domain Layer** has no external dependencies
- **UseCase Layer** depends only on Domain interfaces
- **Adapter Layer** implements Domain port interfaces
- **Handler Layer** injects UseCases directly

### Why 5 Layers Instead of 6?

Worker uses 6 layers including separate Application and Infrastructure layers. Web simplifies this:

| Worker (6-Layer) | Web (5-Layer)         | Rationale                         |
| ---------------- | --------------------- | --------------------------------- |
| Application      | (merged into Handler) | Web has single entry point (HTTP) |
| Infrastructure   | (merged into Adapter) | Simpler DI wiring in Web context  |

Web backend's simpler requirements (HTTP-only entry point, smaller team) don't warrant the additional Infrastructure/Application separation.

## Options Considered

### Option A: 5-Layer Clean Architecture (Selected)

**How It Works:**

- Domain layer defines pure entities and port interfaces
- UseCase layer orchestrates business logic using ports
- Adapter layer implements ports with specific technologies
- Handler layer maps HTTP requests to UseCases

**Pros:**

- **Testability**: UseCase testable with simple mock ports
- **Maintainability**: Clear boundaries reduce cognitive load
- **AI-Friendliness**: Isolated files fit within LLM context windows
- **Flexibility**: Technology changes isolated to adapter layer
- **Consistency**: Aligns with Worker architecture pattern

**Cons:**

- More files and packages than monolithic approach
- Understanding dependency flow requires documentation
- Overhead for simple CRUD operations

### Option B: Traditional Layered Architecture

**How It Works:**

- Handler -> Service -> Repository pattern
- Service layer contains all business logic
- Repository handles database access

**Pros:**

- Simpler initial structure
- Fewer indirections
- Common pattern, widely understood

**Cons:**

- Service files become bloated as features grow
- Testing requires mocking concrete classes
- HTTP concerns leak into service layer
- Technology coupling in service layer

### Option C: Hexagonal Architecture

**How It Works:**

- Ports and Adapters pattern
- Less prescriptive internal structure
- Inbound/Outbound adapter distinction

**Pros:**

- Flexible internal organization
- Well-documented pattern
- Clear boundary concept

**Cons:**

- Less guidance on internal layer structure
- "Application hexagon" remains undefined
- Clean Architecture provides more actionable structure

### Option D: Keep Service-Oriented Structure

**How It Works:**

- Continue with Handler -> Service pattern
- Gradual refactoring when needed

**Evaluation:**

- HTTP status codes in service layer violates separation
- Testing complexity increases over time
- AI agents struggle with large service files

## Implementation

### Port Interface Pattern

Interfaces are defined in Domain layer, not alongside implementations:

```
modules/{module}/
├── domain/
│   ├── entity/        # Pure Go models
│   │   └── analysis.go
│   └── port/          # Interface definitions
│       └── repository.go
├── usecase/           # One file per feature
│   └── get_analysis.go
├── adapter/           # External implementations
│   ├── repository_postgres.go
│   └── mapper/
│       └── response.go
└── handler/
    └── http.go        # StrictServerInterface impl
```

### Error Handling Pattern

Domain errors are mapped to HTTP status codes in Handler layer:

| Domain Error       | HTTP Status | Purpose             |
| ------------------ | ----------- | ------------------- |
| `ErrNotFound`      | 404         | Analysis not found  |
| `ErrAlreadyQueued` | 409         | Duplicate request   |
| `ErrRateLimited`   | 429         | Rate limit exceeded |
| (unexpected)       | 500         | Internal error      |

### UseCase Pattern

Each use case is a focused struct with port dependencies:

```go
type GetAnalysisUseCase struct {
    queue      port.QueueService
    repository port.Repository
}

type GetAnalysisInput struct {
    Owner string
    Repo  string
}

func (uc *GetAnalysisUseCase) Execute(ctx context.Context, input GetAnalysisInput) (*AnalyzeResult, error)
```

### Import Rules (Enforced by depguard)

| Layer         | Allowed Imports             |
| ------------- | --------------------------- |
| domain/entity | No external dependencies    |
| domain/port   | Only entity                 |
| usecase       | Only domain (entity + port) |
| adapter       | domain + external libraries |
| handler       | usecase + adapter/mapper    |

## Consequences

### Positive

**Testability:**

- Domain logic testable without any mocks
- UseCase testable with simple port mocks
- No database/queue required for business rule verification
- 90%+ coverage achievable with unit tests

**Maintainability:**

- Clear boundaries reduce cognitive load
- Changes to one layer rarely affect others
- Easier onboarding with well-defined responsibilities
- Code navigation follows predictable patterns

**AI-Assisted Development:**

- Each file is self-contained within LLM context windows
- AI agents can understand and regenerate entire modules
- Explicit interfaces reduce cross-file dependency scanning
- Bounded contexts enable effective AI-based refactoring

**Flexibility:**

- Database migration: only adapter layer changes
- Queue system switch: only adapter layer changes
- New use case: add usecase file, wire in handler

### Negative

**Initial Complexity:**

- More packages and files than service-oriented approach
- Understanding dependency flow requires documentation
- **Mitigation**: CLAUDE.md documents layer structure; depguard enforces rules

**Indirection:**

- More layers between HTTP request and business logic
- Debugging may require tracing through multiple packages
- **Mitigation**: Structured logging with context; clear naming conventions

**Overhead for Simple Operations:**

- Even simple CRUD requires full layer traversal
- May feel excessive for straightforward features
- **Mitigation**: Accept overhead as investment in long-term maintainability

### Migration Considerations

Existing modules migrated incrementally:

1. **analyzer**: First module migrated
2. **auth**: Full migration with 5 port interfaces
3. **github**: Service layer replaced with usecases
4. **user**: Bookmark and history features restructured

Each migration followed the same pattern: domain/entity -> domain/port -> usecase -> adapter -> handler.

## References

- [The Clean Architecture - Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Clean Architecture in Go - Three Dots Labs](https://threedots.tech/post/introducing-clean-architecture/)
- [AI-Optimizing Codebase Architecture for AI Coding Tools](https://medium.com/@richardhightower/ai-optimizing-codebase-architecture-for-ai-coding-tools-ff6bb6fdc497)
- [Worker ADR-02: Clean Architecture Layers](/en/adr/worker/02-clean-architecture-layers.md)
- [Worker ADR-07: Repository Pattern](/en/adr/worker/07-repository-pattern.md)
