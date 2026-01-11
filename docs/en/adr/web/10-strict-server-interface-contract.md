---
title: StrictServerInterface Contract
description: ADR on using oapi-codegen strict-server mode for compile-time API contract enforcement
---

# ADR-10: StrictServerInterface Contract

> [Korean Version](/ko/adr/web/10-strict-server-interface-contract.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-01-03 | @KubrickCode | web   |

## Context

### The API Contract Problem

API development involves a fundamental tension between specification and implementation. When OpenAPI specifications and handler implementations evolve independently, several problems emerge:

**Manual Synchronization:**

- Handler function signatures must match API specification manually
- Missing or incorrect parameters discovered only at runtime
- Response type mismatches silently produce invalid JSON

**Runtime vs Compile-Time Errors:**

- Traditional handlers use raw `http.ResponseWriter` and `*http.Request`
- Type errors only surface during API testing or production
- No compiler assistance for parameter extraction or response formatting

**Handler-Specification Drift:**

- Adding API parameters requires updating both OpenAPI spec and handler code
- Renaming operations breaks the connection silently
- Response status codes not enforced by type system

### OpenAPI-First Type Generation

The project adopted OpenAPI-first development with oapi-codegen:

| Change                                       | Motivation                           |
| -------------------------------------------- | ------------------------------------ |
| Setup OpenAPI-based type generation pipeline | Single source of truth for API types |
| Enable strict-server mode                    | Compile-time API contract validation |
| Introduce APIHandlers composition pattern    | Multi-domain handler management      |

The initial type generation produced types but still required manual handler wiring. The strict-server enhancement introduced compile-time enforcement.

## Decision

**Adopt oapi-codegen strict-server mode for compile-time API contract enforcement.**

Configuration in `api/oapi-codegen.yaml`:

```yaml
package: api
output: internal/api/server.gen.go
generate:
  models: true
  chi-server: true
  strict-server: true
```

The strict-server option generates `StrictServerInterface` with typed request/response objects, providing compile-time verification that all API endpoints are implemented correctly.

### Handler Implementation Pattern

All HTTP handlers implement the generated `StrictServerInterface`:

```go
// Generated interface (server.gen.go)
type StrictServerInterface interface {
    GetAnalysis(ctx context.Context, request GetAnalysisRequestObject) (GetAnalysisResponseObject, error)
    // ... all endpoints
}

// Implementation check (handlers.go)
var _ StrictServerInterface = (*APIHandlers)(nil)
```

This compile-time assertion ensures the implementation matches the OpenAPI specification exactly.

## Options Considered

### Option A: StrictServerInterface (Selected)

**How It Works:**

- oapi-codegen generates `StrictServerInterface` with typed signatures
- Each endpoint receives a strongly-typed `RequestObject` and returns a `ResponseObject`
- Handler wrapper translates between HTTP and typed interfaces
- Compiler enforces interface implementation completeness

**Function Signature Comparison:**

```diff
// ServerInterface (non-strict)
-GetAnalysis(w http.ResponseWriter, r *http.Request, owner string, repo string)

// StrictServerInterface (strict)
+GetAnalysis(ctx context.Context, request GetAnalysisRequestObject) (GetAnalysisResponseObject, error)
```

**Pros:**

- **Compile-Time Enforcement**: Missing endpoints cause build failure
- **Type Safety**: Request parameters and response bodies are typed
- **Explicit Error Handling**: Error return requires explicit handling
- **Context Propagation**: `context.Context` passed explicitly for cancellation/timeout
- **HTTP Abstraction**: No direct `http.ResponseWriter` manipulation in business logic

**Cons:**

- Generated code dependency (must run `just gen-api` after OpenAPI changes)
- Additional abstraction layer between HTTP and handler
- Learning curve for generated request/response types

### Option B: Standard ServerInterface

**How It Works:**

- oapi-codegen generates `ServerInterface` with raw HTTP handlers
- Parameters extracted from `*http.Request` by generated code
- Response writing handled manually via `http.ResponseWriter`

**Function Signature:**

```go
GetAnalysis(w http.ResponseWriter, r *http.Request, owner string, repo string)
```

**Pros:**

- Direct HTTP control for advanced use cases
- Familiar Go HTTP handler pattern
- Slightly less generated code

**Cons:**

- Response types not enforced by compiler
- Manual JSON serialization with error-prone status codes
- HTTP concerns mixed with business logic
- No compile-time check for response type correctness

### Option C: Manual Handler Implementation

**How It Works:**

- Write handlers without code generation
- Manually extract parameters and validate request
- Manually construct and write responses

**Pros:**

- Full control over all aspects
- No code generation dependency
- No learning curve for generated types

**Cons:**

- No compile-time contract enforcement
- Specification-implementation drift inevitable
- Duplicate type definitions across OpenAPI and Go
- Manual parameter extraction is error-prone

## Implementation

### Request Object Pattern

Each endpoint's request is encapsulated in a generated struct:

```go
// Generated request object
type GetAnalysisRequestObject struct {
    Owner string
    Repo  string
}

// Handler receives strongly-typed request
func (h *Handler) GetAnalysis(ctx context.Context, request GetAnalysisRequestObject) (GetAnalysisResponseObject, error) {
    result, err := h.usecase.Execute(ctx, usecase.GetAnalysisInput{
        Owner: request.Owner,
        Repo:  request.Repo,
    })
    // ...
}
```

### Response Object Pattern

Responses use a union type pattern with specific response types:

```go
// Generated response interface
type GetAnalysisResponseObject interface {
    VisitGetAnalysisResponse(w http.ResponseWriter) error
}

// Concrete response types
type GetAnalysis200JSONResponse AnalysisResult
type GetAnalysis404ApplicationProblemPlusJSONResponse ProblemDetail

// Handler returns specific response type
func (h *Handler) GetAnalysis(ctx context.Context, request GetAnalysisRequestObject) (GetAnalysisResponseObject, error) {
    result, err := h.usecase.Execute(ctx, input)

    switch {
    case errors.Is(err, domain.ErrNotFound):
        return GetAnalysis404ApplicationProblemPlusJSONResponse{
            Status: 404,
            Title:  "Not Found",
            Detail: "Analysis not found",
        }, nil
    case err != nil:
        return nil, err
    }

    return GetAnalysis200JSONResponse(*result), nil
}
```

### APIHandlers Composition

Multiple domain handlers are composed into a single `StrictServerInterface` implementation:

```go
type APIHandlers struct {
    analyzer        AnalyzerHandlers
    auth            AuthHandlers
    bookmark        BookmarkHandlers
    // ...
}

var _ StrictServerInterface = (*APIHandlers)(nil)

func (h *APIHandlers) GetAnalysis(ctx context.Context, request GetAnalysisRequestObject) (GetAnalysisResponseObject, error) {
    return h.analyzer.GetAnalysis(ctx, request)
}
```

This pattern allows domain-specific handlers while maintaining a single interface for the HTTP server.

## Consequences

### Positive

**Compile-Time API Contract:**

- Adding new endpoints without implementation fails compilation
- Changing request parameters forces handler signature update
- Response type mismatches caught at build time
- No runtime surprises from handler-specification drift

**Type Safety:**

- Request parameters are extracted and typed by generated code
- Response bodies match OpenAPI schema definitions
- Error responses use consistent ProblemDetail structure
- No manual JSON marshaling in handlers

**Clean Architecture Alignment:**

- Handlers focus on request/response mapping, not HTTP details
- Business logic in UseCase layer receives typed inputs
- Domain errors mapped to typed HTTP responses
- Clear separation between API contract and implementation

**Developer Experience:**

- IDE autocomplete for request/response types
- Compiler errors guide API implementation
- Consistent handler patterns across all endpoints

### Negative

**Code Generation Dependency:**

- Must run `just gen-api` after OpenAPI changes
- CI must verify generated code is up-to-date
- **Mitigation**: pre-commit hook or CI check for generated file freshness

**Generated Code Volume:**

- `server.gen.go` is ~3000+ lines of generated code
- Request/Response types increase binary size slightly
- **Mitigation**: Accept as cost of type safety; exclude from code reviews

**Learning Curve:**

- Team must understand generated type patterns
- Response union types require learning pattern
- **Mitigation**: Document patterns in CLAUDE.md; provide examples

**Webhook Exception:**

- Some endpoints (GitHub webhooks) require raw HTTP access
- Cannot fit all use cases into strict interface pattern
- **Mitigation**: `WebhookHandlers` interface with separate raw handler

## References

- [oapi-codegen GitHub Repository](https://github.com/oapi-codegen/oapi-codegen)
- [oapi-codegen Strict Server Documentation](https://github.com/oapi-codegen/oapi-codegen/blob/main/README.md#strict-server)
- [ADR-08: Clean Architecture Pattern](/en/adr/web/08-clean-architecture-pattern.md)
