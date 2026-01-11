---
title: Clean Architecture Layers
description: ADR on six-layer Clean Architecture for separation of concerns and testability
---

# ADR-02: Clean Architecture Layer Introduction

> 🇰🇷 [한국어 버전](/ko/adr/worker/02-clean-architecture-layers.md)

| Date       | Author       | Repos  |
| ---------- | ------------ | ------ |
| 2024-12-18 | @KubrickCode | worker |

## Context

### Initial Architecture Challenges

The initial monolithic structure presented several challenges as the codebase grew:

**Domain-Infrastructure Coupling:**

- Business logic was intertwined with database queries and queue operations
- Changes to infrastructure details (e.g., PostgreSQL queries, River task handling) required modifications to core business logic

**Limited Testability:**

- Unit testing was difficult due to direct dependencies on concrete implementations
- Mock injection was not possible without significant refactoring
- Integration tests were required even for simple business rule verification

**Change Impact:**

- Switching external libraries (e.g., different queue system) would require rewriting business logic
- No clear boundaries between concerns made code navigation difficult

### Goals

1. **Separation of Concerns**: Isolate business logic from infrastructure details
2. **Testability**: Enable unit testing with mock dependencies
3. **Flexibility**: Allow infrastructure changes without affecting business rules
4. **Scalability**: Support multiple entry points (Worker, Scheduler, CLI) with shared business logic
5. **Code Readability**: Improve code navigation and review efficiency through clear layer boundaries
6. **AI-Assisted Development**: Limit context scope by confining work to specific layers, enabling more effective AI-based coding with bounded context windows

## Decision

**Adopt a Clean Architecture layer structure with six distinct layers.**

### Layer Structure

| Layer          | Responsibility                                        |
| -------------- | ----------------------------------------------------- |
| Domain         | Business logic and interface definitions              |
| UseCase        | Business workflow orchestration                       |
| Adapter        | Interface implementations (Repository, VCS, Parser)   |
| Handler        | Entry point adapters (Queue handlers, Scheduler jobs) |
| Infrastructure | Technical components (DB pool, Queue client, Config)  |
| Application    | Dependency injection containers                       |

### Dependency Rule

Dependencies flow inward only:

```
Command (main) → Application → Handler → UseCase → Domain
                      ↓            ↓         ↓
                Infrastructure   Adapter   (no deps)
```

- **Domain Layer** has no external dependencies
- **UseCase Layer** depends only on Domain interfaces
- **Adapter Layer** implements Domain interfaces using Infrastructure
- **Handler Layer** translates external requests to UseCase calls
- **Application Layer** wires dependencies together

### Layer Details

**Domain Layer:**

- Defines interfaces: `Repository`, `VCS`, `Parser`, `TokenLookup`
- Contains business models and value objects
- Defines domain-specific errors
- Zero external package imports

**UseCase Layer:**

- Orchestrates business workflows (e.g., Clone → Parse → Save)
- Depends only on Domain interfaces (injected at construction)
- Manages cross-cutting concerns like concurrency limits and timeouts

**Adapter Layer:**

- Implements Domain interfaces with specific technologies
- Examples: PostgreSQL repository, Git VCS adapter, Core parser adapter
- Maps between domain models and external data formats

**Handler Layer:**

- Entry point for external triggers (River tasks, Cron jobs)
- Extracts request parameters and invokes UseCase
- Handles framework-specific concerns (payload unmarshaling, error codes)

**Infrastructure Layer:**

- Database connection pool management
- Queue client/server configuration
- Configuration loading
- Distributed lock implementation

**Application Layer:**

- DI container definitions
- Dependency wiring per entry point
- Lifecycle management (startup/shutdown)

## Options Considered

### Option A: Clean Architecture (Selected)

**Description:**

Six-layer structure with strict dependency rules. Domain at the center with no external dependencies.

**Pros:**

- Clear separation of concerns
- High testability through interface injection
- Technology changes isolated to adapter/infrastructure layers
- Supports multiple entry points with shared business logic

**Cons:**

- Initial setup complexity
- More files and packages to manage
- Learning curve for team members unfamiliar with the pattern

### Option B: Keep Monolithic Structure

**Description:**

Maintain single-package structure with direct dependencies.

**Pros:**

- Simpler initial structure
- Fewer indirections
- Faster to implement small features

**Cons:**

- Testing requires integration setup
- Changes cascade across concerns
- Difficult to scale with multiple entry points

### Option C: Hexagonal Architecture

**Description:**

Ports and Adapters pattern with less prescriptive internal structure.

**Pros:**

- Flexible internal organization
- Focus on ports (interfaces) and adapters (implementations)
- Well-documented pattern

**Cons:**

- Less guidance on internal layer structure
- "Application hexagon" remains undefined
- Clean Architecture provides more actionable structure for this project's scale

## Implementation Principles

### Interface Definition Location

Interfaces are defined in the Domain layer, not alongside implementations:

| Interface   | Location          |
| ----------- | ----------------- |
| Repository  | `domain/` package |
| VCS         | `domain/` package |
| Parser      | `domain/` package |
| TokenLookup | `domain/` package |

This ensures:

- Domain layer has zero infrastructure dependencies
- Implementations can change without domain modifications
- Clear contracts for all adapters

### DI Container Strategy

Separate containers for different entry points:

| Container          | Purpose               | Special Dependencies           |
| ------------------ | --------------------- | ------------------------------ |
| WorkerContainer    | Queue task processing | Encryption key (token decrypt) |
| SchedulerContainer | Cron job scheduling   | Distributed lock               |

**Rationale:**

- Different entry points have different dependency requirements
- Scheduler doesn't need encryption (no private repo access)
- Worker doesn't need distributed lock (queue handles concurrency)

### Concurrency Control Location

Business-level concurrency decisions (e.g., max concurrent clones) belong in UseCase layer, not Adapter:

- Semaphore-based throttling is a **business decision** about resource allocation
- Adapter layer focuses on **technical execution**, not policy

## Consequences

### Positive

**Testability:**

- Domain logic testable without any mocks
- UseCase testable with simple interface mocks
- No database/queue required for business rule verification

**Maintainability:**

- Clear boundaries reduce cognitive load
- Changes to one layer rarely affect others
- Easier onboarding with well-defined responsibilities

**Flexibility:**

- Database migration: only adapter layer changes
- Queue system switch: only infrastructure/handler layers change
- New entry point (e.g., HTTP API): add handler, reuse UseCase

**Scalability:**

- Worker and Scheduler scale independently
- Shared UseCase logic ensures consistency
- Container separation allows deployment flexibility

### Negative

**Initial Complexity:**

- More packages and files than monolithic approach
- Understanding dependency flow requires documentation

**Indirection:**

- More layers between entry point and business logic
- Debugging may require tracing through multiple packages

**Overhead for Simple Operations:**

- Even simple CRUD requires full layer traversal
- May feel excessive for straightforward features

## References

- [Clean Architecture by Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [ADR-01: Core Library Separation](/en/adr/core/01-core-library-separation.md) (Core)
