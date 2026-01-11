---
title: Domain Error Handling Pattern
description: ADR on sentinel error pattern for decoupling domain errors from HTTP status codes
---

# ADR-13: Domain Error Handling Pattern

> [Korean Version](/ko/adr/web/13-domain-error-handling-pattern.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-01-03 | @KubrickCode | web   |

## Context

### Initial Architecture Problem

The initial service layer directly returned HTTP status codes alongside business results:

```go
// Before: Service layer aware of HTTP
func (s *Service) GetAnalysis(...) (api.Response, int, error) {
    if analysis == nil {
        return nil, http.StatusNotFound, errors.New("not found")
    }
    return response, http.StatusOK, nil
}
```

This violated Clean Architecture's dependency rule—inner layers (domain/service) should not know about outer layers (HTTP transport).

### Problems with HTTP Coupling

**Separation of Concerns Violation:**

- Service layer mixed business logic with transport concerns
- Switching from REST to gRPC would require rewriting service layer
- Business rules became entangled with status code decisions

**Testing Complexity:**

- Tests had to verify both business logic and HTTP status codes
- Mock setup required understanding of HTTP semantics
- Status code assertions cluttered business logic tests

**Multi-Handler Inconsistency:**

- Different handlers might map the same error to different status codes
- No single source of truth for domain error semantics

## Decision

**Adopt sentinel error pattern with handler-layer HTTP mapping.**

### Core Principles

1. **Sentinel Errors in Domain**: Each module defines `var ErrXxx = errors.New(...)` in `domain/errors.go`
2. **Error Checking with `errors.Is()`**: Handlers use `errors.Is(err, domain.ErrXxx)` for classification
3. **HTTP Mapping in Handler Only**: Only handler layer knows about HTTP status codes
4. **Contextual Wrapping**: Use `fmt.Errorf("context: %w", err)` to add context while preserving error type

### Sentinel Error Categories

| Category       | Examples                                                 | HTTP Mapping |
| -------------- | -------------------------------------------------------- | ------------ |
| **Not Found**  | `ErrNotFound`, `ErrUserNotFound`, `ErrCodebaseNotFound`  | 404          |
| **Validation** | `ErrInvalidCursor`, `ErrInvalidState`                    | 400          |
| **Auth**       | `ErrUnauthorized`, `ErrTokenExpired`, `ErrNoGitHubToken` | 401          |
| **Permission** | `ErrAccessDenied`, `ErrInsufficientScope`                | 401/403      |
| **Rate Limit** | `RateLimitError` (custom type)                           | 429          |
| **Conflict**   | `ErrAlreadyQueued`                                       | 409          |

### Custom Error Types

For errors requiring additional data, use custom error types:

```go
type RateLimitError struct {
    Limit     int
    Remaining int
    ResetAt   time.Time
}

func (e *RateLimitError) Error() string { ... }

// Check with errors.As()
func IsRateLimitError(err error) bool {
    var rateLimitErr *RateLimitError
    return errors.As(err, &rateLimitErr)
}
```

## Options Considered

### Option A: Sentinel Errors with Handler Mapping (Selected)

**How It Works:**

- Domain layer defines `var ErrXxx = errors.New("...")`
- UseCase returns domain errors or wrapped domain errors
- Handler uses `errors.Is()` to classify and map to HTTP status

**Pros:**

- Domain layer has no transport dependencies
- `errors.Is()` provides type-safe error checking
- Error wrapping preserves context (`fmt.Errorf("%w", err)`)
- Consistent pattern across all modules
- Aligns with Go 1.13+ error handling idioms

**Cons:**

- Requires explicit mapping in each handler
- Risk of error proliferation if not managed
- Handler code becomes verbose with many error cases

### Option B: HTTP Status Codes in Domain/Service

**How It Works:**

- Service returns `(result, httpStatus, error)` tuple
- Handler directly uses returned status code

**Pros:**

- Simpler handler code
- Direct status code from business logic

**Cons:**

- Violates Clean Architecture dependency rule
- Domain layer tied to HTTP transport
- Cannot reuse domain logic for non-HTTP transports (gRPC, CLI)
- Testing requires HTTP knowledge in service tests

### Option C: Error Codes Enum

**How It Works:**

- Define numeric/string error codes in domain
- Map error codes to HTTP status in handler

**Pros:**

- Explicit error catalog
- Easy to document

**Cons:**

- Less Go-idiomatic than sentinel errors
- Requires additional mapping layer
- Error type information lost in code values

### Option D: Exception-Style with panic/recover

**How It Works:**

- Panic with typed error in domain
- Recover and map in middleware

**Pros:**

- Cleaner happy path code
- Automatic propagation

**Cons:**

- Not idiomatic Go
- Stack unwinding overhead
- Difficult to control recovery scope
- Unexpected behavior for callers

## Implementation

### Module Error Definition

Each module maintains its own `domain/errors.go`:

```
modules/
├── analyzer/domain/errors.go    # ErrNotFound, ErrInvalidCursor
├── auth/domain/errors.go        # ErrUserNotFound, ErrTokenExpired, ...
├── github/domain/errors.go      # ErrUnauthorized, RateLimitError
├── github-app/domain/errors.go  # ErrInstallationNotFound, ...
└── user/domain/errors.go        # ErrCodebaseNotFound, ErrInvalidCursor
```

### UseCase Error Propagation

UseCases return domain errors or wrap with context:

```go
func (uc *GetAnalysisUseCase) Execute(ctx context.Context, input Input) (*Result, error) {
    analysis, err := uc.repo.GetByOwnerRepo(ctx, input.Owner, input.Repo)
    if err != nil {
        if errors.Is(err, domain.ErrNotFound) {
            return nil, err  // Propagate as-is
        }
        return nil, fmt.Errorf("get analysis: %w", err)  // Wrap unexpected
    }
    return &Result{Analysis: analysis}, nil
}
```

### Handler Error Mapping

Handler maps domain errors to HTTP responses:

```go
func (h *Handler) GetAnalysis(ctx context.Context, req Request) (Response, error) {
    result, err := h.getAnalysis.Execute(ctx, input)
    if err != nil {
        switch {
        case errors.Is(err, domain.ErrNotFound):
            return api.GetAnalysis404JSONResponse{...}, nil
        case errors.Is(err, domain.ErrInvalidCursor):
            return api.GetAnalysis400JSONResponse{...}, nil
        default:
            h.logger.Error(ctx, "unexpected error", "error", err)
            return api.GetAnalysis500JSONResponse{...}, nil
        }
    }
    return api.GetAnalysis200JSONResponse{...}, nil
}
```

### Cross-Module Error Handling

When UseCase depends on another module's port, handle cross-module errors:

```go
// analyzer/usecase/helper.go
token, err := uc.tokenProvider.GetGitHubToken(ctx, userID)
if err != nil {
    // Handle auth module errors in analyzer context
    if errors.Is(err, authdomain.ErrUserNotFound) ||
       errors.Is(err, authdomain.ErrNoGitHubToken) {
        return nil, domain.ErrNoGitHubToken  // Translate to local domain error
    }
    return nil, fmt.Errorf("get github token: %w", err)
}
```

## Consequences

### Positive

**Domain Independence:**

- Domain layer has zero transport dependencies
- Can reuse domain logic for gRPC, CLI, or other transports
- Clean separation of business rules from delivery mechanism

**Type-Safe Error Handling:**

- `errors.Is()` and `errors.As()` provide compile-time safety
- No string comparison for error classification
- Error wrapping preserves full context chain

**Consistency Across Modules:**

- All modules follow same pattern: `domain/errors.go` + handler mapping
- Predictable error handling code structure
- Easy to add new error types following established pattern

**Testability:**

- UseCase tests verify domain error returns, not HTTP codes
- Handler tests focus on error-to-status mapping
- Clear boundary between business logic and transport tests

### Negative

**Handler Verbosity:**

- Each handler must implement error classification switch
- Multiple error cases lead to repetitive code
- **Mitigation**: Extract common error mapping to helper functions

**Error Proliferation Risk:**

- Easy to add new sentinel errors without governance
- Too many errors reduce semantic clarity
- **Mitigation**: Limit to 5-7 errors per module; review new errors carefully

**Cross-Module Complexity:**

- Handlers may need to check errors from multiple modules
- Error translation between modules adds code
- **Mitigation**: Define clear port interfaces with documented error contracts

## References

- [Go Blog: Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors)
- [ADR-08: Clean Architecture Pattern](/en/adr/web/08-clean-architecture-pattern.md)
- [Worker ADR-02: Clean Architecture Layers](/en/adr/worker/02-clean-architecture-layers.md)
