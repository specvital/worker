---
title: APIHandlers Composition Pattern
description: ADR on composing multiple domain handlers into a single StrictServerInterface implementation
---

# ADR-12: APIHandlers Composition Pattern

> [Korean Version](/ko/adr/web/12-apihandlers-composition-pattern.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-01-03 | @KubrickCode | web   |

## Context

### The Single Interface Constraint

oapi-codegen's strict-server mode generates a single `StrictServerInterface` that must be implemented by one struct. This creates a tension with the Feature-Based Module Organization ([ADR-11](/en/adr/web/11-feature-based-module-organization.md)), where each domain module (analyzer, auth, user, github, etc.) maintains its own handler implementation within its Clean Architecture layers.

**The Problem:**

- `StrictServerInterface` defines all API endpoints in one interface (~20+ methods)
- Feature-based modules have separate handler packages (`modules/analyzer/handler/`, `modules/auth/handler/`, etc.)
- Each module's handler only knows about its own domain logic
- The server requires a single struct implementing all interface methods

**Without Composition:**

```go
// Impossible: Each handler only implements a subset of methods
var _ StrictServerInterface = (*AnalyzerHandler)(nil)  // Missing auth methods
var _ StrictServerInterface = (*AuthHandler)(nil)      // Missing analyzer methods
```

The initial implementation used a single handler struct with all methods. As the codebase grew and adopted Feature-Based Module Organization, a composition pattern became necessary to maintain domain separation while satisfying the single interface requirement.

## Decision

**Adopt the APIHandlers Composition Pattern to combine multiple domain handlers into a single StrictServerInterface implementation.**

### Pattern Structure

```go
// Domain-specific handler interfaces (handlers.go)
type AnalyzerHandlers interface {
    AnalyzeRepository(ctx context.Context, request AnalyzeRepositoryRequestObject) (AnalyzeRepositoryResponseObject, error)
    GetAnalysisStatus(ctx context.Context, request GetAnalysisStatusRequestObject) (GetAnalysisStatusResponseObject, error)
}

type AuthHandlers interface {
    AuthCallback(ctx context.Context, request AuthCallbackRequestObject) (AuthCallbackResponseObject, error)
    AuthLogin(ctx context.Context, request AuthLoginRequestObject) (AuthLoginResponseObject, error)
    // ...
}

// Composite struct implementing full interface
type APIHandlers struct {
    analyzer AnalyzerHandlers
    auth     AuthHandlers
    bookmark BookmarkHandlers
    // ...
}

var _ StrictServerInterface = (*APIHandlers)(nil)  // Compile-time check

// Delegation to domain handlers
func (h *APIHandlers) AnalyzeRepository(ctx context.Context, request AnalyzeRepositoryRequestObject) (AnalyzeRepositoryResponseObject, error) {
    return h.analyzer.AnalyzeRepository(ctx, request)
}
```

### Key Principles

1. **Domain-Specific Interfaces**: Each handler interface contains only methods relevant to its domain
2. **Single Composite Struct**: `APIHandlers` aggregates all domain handlers
3. **Delegation Pattern**: Each method delegates to the appropriate domain handler
4. **Compile-Time Verification**: `var _ StrictServerInterface = (*APIHandlers)(nil)` ensures completeness

## Options Considered

### Option A: APIHandlers Composition Pattern (Selected)

**How It Works:**

- Define domain-specific handler interfaces matching StrictServerInterface method subsets
- Create composite `APIHandlers` struct holding all domain handlers
- Implement StrictServerInterface by delegating to appropriate handlers
- Wire everything in `app.go` using dependency injection

**Pros:**

- **Domain Isolation**: Each handler only knows about its own domain
- **Independent Testing**: Domain handlers testable without other dependencies
- **Clear Ownership**: Each module owns its handler implementation
- **Compile-Time Safety**: Missing implementations caught at build time
- **Extensibility**: Adding new domains requires only new interface + handler

**Cons:**

- Additional boilerplate for delegation methods
- One more layer of indirection
- Interface definitions must stay synchronized with OpenAPI spec

### Option B: Single Monolithic Handler

**How It Works:**

- One large handler struct implementing all StrictServerInterface methods
- All UseCase dependencies injected into single struct
- Direct method implementations without delegation

**Pros:**

- Simpler structure with no delegation layer
- Fewer files to maintain
- Direct method implementations

**Cons:**

- **Violates Single Responsibility**: One struct handles all domains
- **Testing Complexity**: Requires mocking all dependencies for any test
- **Scalability Issues**: File grows unbounded as API expands
- **Poor Cohesion**: Unrelated business logic mixed in one file
- **Conflicts with Feature-Based Organization**: Undermines module boundaries

### Option C: Runtime Router Dispatch

**How It Works:**

- Register handlers dynamically by path prefix
- Router dispatches to appropriate handler at runtime
- Each handler implements partial interface

**Pros:**

- Maximum flexibility for handler registration
- No interface synchronization needed

**Cons:**

- **No Compile-Time Safety**: Missing handlers discovered only at runtime
- **Complex Registration Logic**: Error-prone handler wiring
- **Defeats StrictServerInterface Purpose**: Loses type safety benefits
- **Debugging Difficulty**: Dispatch errors hard to trace

### Option D: Code Generation for Composition

**How It Works:**

- Generate composition layer from handler interfaces
- Auto-generate delegation methods based on interface definitions

**Evaluation:**

- Additional tooling complexity
- Custom code generation maintenance burden
- Pattern is simple enough that manual implementation is acceptable
- **Rejected**: Overhead not justified for current scale

## Implementation

### Handler Interface Definition

Domain handler interfaces are defined in `internal/api/handlers.go`, adjacent to the generated `server.gen.go`:

```
internal/api/
├── handlers.go      # Domain handler interfaces + APIHandlers composite
└── server.gen.go    # Generated StrictServerInterface
```

### Wiring in Application

In `common/server/app.go`, handlers are created and composed:

```go
func initHandlers(container *infra.Container) (*Handlers, error) {
    // Create domain handlers
    analyzerHandler := analyzerhandler.NewHandler(...)
    authHandler := authhandler.NewHandler(...)
    userHandler := userhandler.NewHandler(...)
    githubHandler := githubhandler.NewHandler(...)

    // Compose into single interface
    apiHandlers := api.NewAPIHandlers(
        analyzerHandler,
        userHandler,      // AnalysisHistoryHandlers
        authHandler,
        userHandler,      // BookmarkHandlers
        githubHandler,
        githubAppHandler,
        analyzerHandler,  // RepositoryHandlers
        webhookHandler,
    )

    return &Handlers{API: apiHandlers}, nil
}
```

### Special Cases

**Optional Handlers:**

Some handlers may be conditionally available (e.g., GitHub App only when configured):

```go
func (h *APIHandlers) GetGitHubAppInstallURL(ctx context.Context, request GetGitHubAppInstallURLRequestObject) (GetGitHubAppInstallURLResponseObject, error) {
    if h.githubApp == nil {
        return GetGitHubAppInstallURL500ApplicationProblemPlusJSONResponse{
            InternalErrorApplicationProblemPlusJSONResponse: NewInternalError("GitHub App not configured"),
        }, nil
    }
    return h.githubApp.GetGitHubAppInstallURL(ctx, request)
}
```

**Raw HTTP Handlers:**

Endpoints requiring raw HTTP access (webhooks) use a separate interface:

```go
type WebhookHandlers interface {
    HandleGitHubAppWebhookRaw(w http.ResponseWriter, r *http.Request)
}

// Accessed via special accessor method
func (h *APIHandlers) WebhookHandler() WebhookHandlers {
    return h.webhook
}
```

## Consequences

### Positive

**Domain Separation:**

- Each domain handler lives within its module's `handler/` package
- Domain-specific logic isolated from other domains
- Changes to one domain don't affect others

**Testability:**

- Domain handlers tested independently with mocked dependencies
- No need to instantiate entire API layer for unit tests
- Integration tests can use real composite or partial mocks

**Extensibility:**

- Adding new domain: Define interface, implement handler, add to composite
- Adding new endpoint: Implement in appropriate domain handler
- Compile-time error if composite doesn't delegate new method

**Clean Architecture Alignment:**

- Handler layer clearly separated per [ADR-08](/en/adr/web/08-clean-architecture-pattern.md)
- Feature-Based modules maintained per [ADR-11](/en/adr/web/11-feature-based-module-organization.md)
- StrictServerInterface contract preserved per [ADR-10](/en/adr/web/10-strict-server-interface-contract.md)

### Negative

**Boilerplate:**

- Each StrictServerInterface method requires delegation method
- ~20+ one-liner methods in `handlers.go`
- **Mitigation**: Methods are trivial; IDE generates easily; rarely changes

**Interface Synchronization:**

- Domain interfaces must match StrictServerInterface signature subsets
- Adding API endpoints requires updating domain interface
- **Mitigation**: Compile-time error catches mismatches immediately

**Mental Model:**

- Developers must understand composition layer exists
- Debugging requires tracing through delegation
- **Mitigation**: Pattern is simple; documented in CLAUDE.md

## References

- [Composite Pattern - Design Patterns](https://refactoring.guru/design-patterns/composite)
- [Interface Segregation Principle - SOLID](https://en.wikipedia.org/wiki/Interface_segregation_principle)
- [oapi-codegen Strict Server](https://github.com/oapi-codegen/oapi-codegen)
- [ADR-08: Clean Architecture Pattern](/en/adr/web/08-clean-architecture-pattern.md)
- [ADR-10: StrictServerInterface Contract](/en/adr/web/10-strict-server-interface-contract.md)
- [ADR-11: Feature-Based Module Organization](/en/adr/web/11-feature-based-module-organization.md)
