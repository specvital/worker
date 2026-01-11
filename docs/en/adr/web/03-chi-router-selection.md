---
title: Chi Router Selection
description: ADR on choosing Chi as the HTTP router for Go backend with focus on standard library compatibility
---

# ADR-03: Chi Router Selection

> [한국어 버전](/ko/adr/web/03-chi-router-selection.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-03 | @KubrickCode | web   |

## Context

### The Router Selection Question

With Go selected as the backend language (see [ADR-01](/en/adr/web/01-go-backend-language.md)), we need an HTTP router for the REST API. Key requirements:

1. **OpenAPI Compatibility**: Must integrate with `oapi-codegen` for type-safe API generation
2. **Standard Library Alignment**: Prefer `net/http` compatible solutions
3. **Middleware Composability**: Modular middleware chain for auth, CORS, logging, rate limiting
4. **Low Learning Curve**: Minimize friction for full-stack developers familiar with JavaScript frameworks
5. **Minimal Dependencies**: Keep the dependency tree lean

### Existing Architecture Constraints

- **OpenAPI-First**: `openapi.yaml` → `oapi-codegen` → Go server handlers
- **Clean Architecture**: Handlers, usecases, adapters, domain separation
- **BFF Pattern**: Next.js frontend calls Go backend; Go handles all business logic
- **Deployment**: Railway (Go server) + Vercel (Next.js) + Neon (PostgreSQL)

### Candidates Evaluated

1. **Chi**: Lightweight router built on `net/http`, modular middleware
2. **Gin**: High-performance framework with custom context
3. **Echo**: Full-featured framework with own middleware stack
4. **Fiber**: Express-inspired framework on `fasthttp`

## Decision

**Adopt Chi (go-chi/chi v5) as the HTTP router for its strict adherence to Go's standard library interfaces.**

Core principles:

1. **net/http Native**: All handlers use `http.Handler` and `http.HandlerFunc`
2. **Zero Framework Lock-in**: Any `net/http` middleware works without adaptation
3. **oapi-codegen Integration**: Native support via `HandlerFromMux()`
4. **Composition over Configuration**: Build exactly what you need

## Options Considered

### Option A: Chi (Selected)

**How It Works:**

- Thin wrapper around `net/http`
- Middleware uses standard `func(http.Handler) http.Handler` signature
- Router implements `http.Handler` interface
- Context values via `context.WithValue()` (Go standard)

**Pros:**

- **Standard Library Compatible**: All existing `net/http` middleware works directly
- **oapi-codegen Native**: First-class integration via `chi-server` generation target
- **Minimal API Surface**: Router + middleware pattern; nothing more
- **Testing Simplicity**: Standard `httptest` package works without adaptation
- **Low Dependency Count**: Only depends on `net/http` and `context`

**Cons:**

- Less "batteries included" than full frameworks
- No built-in request binding/validation (handled by oapi-codegen)
- Manual response helpers (handled by generated code)

### Option B: Gin

**How It Works:**

- Custom `*gin.Context` wraps request/response
- Middleware uses `gin.HandlerFunc` signature
- High-performance radix tree router
- Built-in JSON binding, validation, response helpers

**Evaluation:**

- **Context Coupling**: `*gin.Context` creates framework dependency in all handlers
- **Middleware Incompatibility**: `net/http` middleware requires `WrapH()` adapter
- **oapi-codegen Support**: Works but handlers receive `*gin.Context`, not standard interfaces
- **Overhead**: Features we don't need (binding, validation) already provided by oapi-codegen
- **Rejected**: Framework lock-in outweighs convenience features

### Option C: Echo

**How It Works:**

- Custom `echo.Context` interface
- Closure-based middleware `echo.MiddlewareFunc`
- Built-in routing, binding, validation
- Good performance with own context pooling

**Evaluation:**

- **Context Coupling**: Similar to Gin, custom context throughout
- **Middleware Adaptation**: `net/http` middleware requires `echo.WrapHandler()`
- **Feature Overlap**: Binding/validation redundant with oapi-codegen
- **Rejected**: Same lock-in issues as Gin

### Option D: Fiber

**How It Works:**

- Built on `fasthttp` instead of `net/http`
- Express.js-inspired API
- Highest raw performance benchmarks
- Custom `*fiber.Ctx` for all operations

**Evaluation:**

- **API Incompatibility**: `fasthttp` signature differs from `net/http` entirely
- **Ecosystem Isolation**: Cannot use any `net/http` middleware
- **HTTP/2 & HTTP/3**: Not supported (fasthttp limitation)
- **Memory Management**: Manual lifecycle control required; risk of leaks
- **oapi-codegen**: Works but with adaptation layer overhead
- **Initially Considered**: Was the original recommendation in early project setup
- **Rejected**: Ecosystem incompatibility; HTTP/2 gap; memory management complexity

## Implementation Details

### Router Setup

```go
// cmd/server/main.go
func newRouter(...) *chi.Mux {
    r := chi.NewRouter()

    // Standard middleware chain
    r.Use(chimiddleware.RequestID)
    r.Use(chimiddleware.RealIP)
    r.Use(middleware.Logger())
    r.Use(chimiddleware.Recoverer)
    r.Use(middleware.SecurityHeaders())
    r.Use(middleware.CORS(origins))
    r.Use(chimiddleware.Timeout(apiTimeout))
    r.Use(middleware.Compress())
    r.Use(authMiddleware.OptionalAuth)

    // Rate limiting for auth endpoints
    authLimiter := middleware.NewIPRateLimiter(authRateLimit)
    r.Route("/api/auth", func(authRouter chi.Router) {
        authRouter.Use(middleware.RateLimit(authLimiter))
    })

    // oapi-codegen generated handlers
    strictHandler := api.NewStrictHandler(apiHandler, nil)
    api.HandlerFromMux(strictHandler, r)

    return r
}
```

### Middleware Architecture

| Middleware      | Source         | Purpose                          |
| --------------- | -------------- | -------------------------------- |
| RequestID       | chi/middleware | Request correlation across logs  |
| RealIP          | chi/middleware | Extract client IP behind proxies |
| Logger          | custom         | Structured logging with slog     |
| Recoverer       | chi/middleware | Panic recovery to 500 response   |
| SecurityHeaders | custom         | HSTS, X-Frame-Options, CSP       |
| CORS            | go-chi/cors    | Cross-origin resource sharing    |
| Timeout         | chi/middleware | Request timeout enforcement      |
| Compress        | custom         | gzip response compression        |
| OptionalAuth    | custom         | JWT validation when present      |
| RateLimit       | custom         | IP-based rate limiting           |

### oapi-codegen Integration

```yaml
# oapi-codegen.yaml
generate:
  chi-server: true
  strict-server: true
  models: true
output: internal/api/server.gen.go
```

Generated code provides:

- `StrictServerInterface`: Type-safe handler signatures
- `HandlerFromMux()`: Register all routes on Chi router
- Request/response validation at compile time

### RouteRegistrar Pattern

```go
// common/server/registrar.go
type RouteRegistrar interface {
    RegisterRoutes(r chi.Router)
}

// Enables modular route registration
func (h *HealthHandler) RegisterRoutes(r chi.Router) {
    r.Get("/health", h.Check)
}
```

## Consequences

### Positive

**Standard Library Alignment:**

- Handlers portable to any `net/http` compatible router
- All Go HTTP testing patterns work unchanged
- Middleware from the entire Go ecosystem compatible

**OpenAPI Integration:**

- oapi-codegen Chi support is mature and well-documented
- StrictServerInterface ensures compile-time API contract validation
- Zero boilerplate for route registration

**Developer Experience:**

- Minimal API surface reduces learning curve
- Standard library knowledge directly applicable
- Clear separation between routing and business logic

**Maintainability:**

- Small dependency footprint (chi v5.2.0, go-chi/cors v1.2.1)
- Active maintenance with semantic versioning
- No custom abstractions to understand

### Negative

**Manual Response Helpers:**

- No built-in response formatting (JSON, XML)
- **Mitigation**: oapi-codegen generates all response types

**Less "Batteries Included":**

- No built-in request binding, validation, templating
- **Mitigation**: oapi-codegen handles binding/validation; templates not needed for API

**Middleware Discovery:**

- Must find compatible `net/http` middleware packages
- **Mitigation**: go-chi ecosystem provides common middleware (cors, httplog)

### Evolution Path

| Scenario            | Approach                                        |
| ------------------- | ----------------------------------------------- |
| Need GraphQL        | Add graphql-go with Chi mount; no router change |
| Need gRPC           | Add grpc-gateway; Chi continues serving REST    |
| Performance Issues  | Profile first; Chi overhead is negligible       |
| Framework Migration | Handlers portable; only router setup changes    |

## References

### Internal

- [ADR-01: Go as Backend Language](/en/adr/web/01-go-backend-language.md)
- [ADR-02: Next.js 16 + React 19 Selection](/en/adr/web/02-nextjs-react-selection.md)
- [Tech Radar](/en/tech-radar.md)
- [PRD: Tech Stack](/en/prd/06-tech-stack.md)

### External

- [go-chi/chi GitHub](https://github.com/go-chi/chi)
- [oapi-codegen Chi Integration](https://github.com/oapi-codegen/oapi-codegen)
- [Chi vs Gin vs Echo vs Fiber Comparison](https://www.linkedin.com/pulse/comparing-go-frameworks-chi-vs-gin-fiber-httprouter-echo-parasuraman-uj0bc)
- [Fiber fasthttp Limitations](https://wnjoon.github.io/2025/11/11/comparison-go-http-lib-en/)
- [Understanding Routing in Gin, Echo, and Chi](https://leapcell.io/blog/understanding-routing-and-middleware-in-gin-echo-and-chi)
