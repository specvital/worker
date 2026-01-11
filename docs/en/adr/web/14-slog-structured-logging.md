---
title: slog-Based Structured Logging
description: ADR on choosing Go standard library slog for structured logging with context-aware request ID injection
---

# ADR-14: slog-Based Structured Logging

> [한국어 버전](/ko/adr/web/14-slog-structured-logging.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-13 | @KubrickCode | web   |

## Context

### The Logging Library Question

The Go backend requires a structured logging solution for:

1. **Observability**: Machine-readable logs for aggregation tools (Datadog, CloudWatch, Splunk)
2. **Request Tracing**: Consistent `request_id` across all log entries for a single request
3. **Structured Data**: Key-value pairs for filtering and analysis
4. **Dependency Management**: Alignment with project's minimal dependency strategy

### Initial Implementation

The initial backend skeleton used `go-chi/httplog/v2`:

```go
logger := httplog.NewLogger(serviceName, httplog.Options{
    LogLevel:       slog.LevelInfo,
    Concise:        true,
    RequestHeaders: true,
})
```

While functional, this introduced an external dependency for functionality that Go 1.21+ provides natively.

### Go 1.21 slog Introduction

Go 1.21 (August 2023) introduced `log/slog` as a standard library package, providing structured logging without external dependencies. This created an opportunity to reduce the dependency footprint while maintaining feature parity.

## Decision

**Migrate from httplog/v2 to Go standard library slog with a context-aware Logger wrapper.**

Implementation principles:

1. **Standard Library First**: Use `log/slog` directly for all structured logging
2. **Context Propagation**: Automatically inject `request_id` from Chi middleware context
3. **Field Chaining**: Support `With()` method for adding contextual fields (owner, repo, etc.)
4. **DI Integration**: Inject Logger via dependency injection for testability

## Options Considered

### Option A: slog (Standard Library) - Selected

**How It Works:**

- Direct use of `log/slog` package
- Custom `Logger` wrapper for context-aware `request_id` injection
- Custom `responseWriter` wrapper for HTTP status/size capture

**Pros:**

- **Zero External Dependencies**: Part of Go standard library since 1.21
- **Long-term Stability**: Maintained by Go team; guaranteed backward compatibility
- **Ecosystem Unification**: Other logging libraries can use slog as a backend
- **Sufficient Performance**: 40 B/op memory allocation; adequate for web applications
- **Future-proof**: All future Go tooling will integrate with slog

**Cons:**

- Slightly slower than zerolog/zap for extreme high-throughput scenarios
- Less feature-rich than specialized libraries

### Option B: zerolog

**How It Works:**

- Zero-allocation JSON logger
- Chainable API: `log.Info().Str("key", "val").Msg("message")`
- External dependency: `github.com/rs/zerolog`

**Evaluation:**

- Fastest logging library in Go ecosystem
- Zero allocations for common operations
- Requires external dependency
- **Rejected**: Performance gains unnecessary for web application scale; adds dependency

### Option C: zap

**How It Works:**

- Uber's high-performance structured logger
- Field-based API with `zap.String()`, `zap.Int()` helpers
- External dependency: `go.uber.org/zap`

**Evaluation:**

- Extensive customization options
- Strong production track record at Uber scale
- Higher memory allocation (168 B/op vs 40 B/op for slog)
- **Rejected**: Overkill for requirements; unnecessary external dependency

### Option D: logrus

**How It Works:**

- Structured logging with hooks system
- JSON and text formatters
- External dependency: `github.com/sirupsen/logrus`

**Evaluation:**

- Feature-rich with extensive hook ecosystem
- No longer actively developed (maintenance mode)
- Higher overhead than modern alternatives
- **Rejected**: Legacy status; not recommended for new projects

### Option E: httplog/v2 (Status Quo)

**How It Works:**

- Chi-specific logging middleware wrapper
- Built on top of slog
- External dependency: `github.com/go-chi/httplog/v2`

**Evaluation:**

- Convenient Chi integration
- Adds dependency layer over what slog provides natively
- Limited customization for context injection
- **Rejected**: Unnecessary abstraction; direct slog provides more control

## Implementation Details

### HTTP Request Logger Middleware

Custom `responseWriter` wrapper captures HTTP status and response size:

```go
type responseWriter struct {
    http.ResponseWriter
    status int
    size   int
}

func Logger() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
            next.ServeHTTP(rw, r)

            slog.Info("http request",
                "request_id", middleware.GetReqID(r.Context()),
                "method", r.Method,
                "path", r.URL.Path,
                "status", rw.status,
                "size", rw.size,
                "duration", time.Since(start).String(),
            )
        })
    }
}
```

### Context-Aware Logger Wrapper

The `Logger` struct automatically includes `request_id` from context:

```go
type Logger struct {
    base  *slog.Logger
    attrs []any
}

func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
    l.logger(ctx).Info(msg, args...)
}

func (l *Logger) logger(ctx context.Context) *slog.Logger {
    allAttrs := make([]any, 0, len(l.attrs)+2)
    allAttrs = append(allAttrs, "request_id", middleware.GetReqID(ctx))
    allAttrs = append(allAttrs, l.attrs...)
    return l.base.With(allAttrs...)
}
```

### Usage Pattern

```go
// In handler/service
logger := logger.New().With("owner", owner, "repo", repo)
logger.Info(ctx, "analysis started")
// Output: {"level":"INFO","msg":"analysis started","request_id":"abc123","owner":"foo","repo":"bar"}
```

## Consequences

### Positive

**Reduced Dependencies:**

- Removed `github.com/go-chi/httplog/v2` from `go.mod`
- Aligns with project's minimal dependency philosophy

**Long-term Stability:**

- Standard library guarantees backward compatibility
- No version conflicts with other packages using slog
- Automatic benefit from Go runtime improvements

**Ecosystem Integration:**

- Can use zerolog/zap as slog backends if needed in future
- Consistent logging interface across Go ecosystem
- Log aggregation tools (Datadog, CloudWatch) natively parse slog JSON output

**Improved Testability:**

- Logger injected via DI pattern
- Easy to mock for unit tests
- Context-based design matches Go idioms

### Negative

**Performance Trade-off:**

- ~10-15% slower than zerolog for extreme throughput
- **Mitigation**: Negligible for web application scale; not a bottleneck

**Custom Wrapper Maintenance:**

- Must maintain `Logger` wrapper code
- **Mitigation**: Minimal code (~47 lines); stable requirements

**Limited Advanced Features:**

- No built-in log rotation or hook system
- **Mitigation**: Use external log aggregation (CloudWatch, Datadog) instead of local files

## References

### Internal

- [ADR-01: Go as Backend Language](/en/adr/web/01-go-backend-language.md)

### External

- [Structured Logging with slog - Go Blog](https://go.dev/blog/slog)
- [slog Package Documentation](https://pkg.go.dev/log/slog)
- [Go Logging Benchmarks](https://github.com/betterstack-community/go-logging-benchmarks)
