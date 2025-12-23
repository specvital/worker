---
title: Semaphore-Based Clone Concurrency
description: ADR on weighted semaphore for limiting concurrent git clone operations
---

# ADR-06: Semaphore-Based Clone Concurrency Control

> 🇰🇷 [한국어 버전](/ko/adr/collector/06-semaphore-clone-concurrency.md)

| Date       | Author       | Repos     |
| ---------- | ------------ | --------- |
| 2024-12-18 | @KubrickCode | collector |

## Context

### Problem

Git clone operations are resource-intensive:

- **Network I/O**: Downloads entire repository history
- **Disk I/O**: Writes to filesystem (code + .git directory)
- **Memory**: Large repositories can consume hundreds of MBs

Without concurrency control, unbounded parallel clones cause:

- Out-of-memory (OOM) errors on constrained environments
- Network bandwidth exhaustion
- Degraded performance for all concurrent tasks

### Constraints

- **Deployment Target**: Small VMs (512MB-2GB RAM)
- **Queue Architecture**: River worker with configurable concurrency (default: 5)
- **Workload**: Variable repository sizes (small libs to large monorepos)

### Goals

1. Prevent OOM from concurrent clone operations
2. Maximize throughput within resource limits
3. Respect context cancellation and timeouts
4. Allow runtime configuration per deployment

## Decision

**Apply weighted semaphore at UseCase level to limit concurrent clone operations.**

### Implementation

```go
type AnalyzeUseCase struct {
    cloneSem *semaphore.Weighted
    // ... other dependencies
}

func NewAnalyzeUseCase(..., opts ...Option) *AnalyzeUseCase {
    return &AnalyzeUseCase{
        cloneSem: semaphore.NewWeighted(cfg.MaxConcurrentClones),
    }
}

func (uc *AnalyzeUseCase) cloneWithSemaphore(ctx context.Context, url string, token *string) (Source, error) {
    if err := uc.cloneSem.Acquire(ctx, 1); err != nil {
        return nil, err
    }
    defer uc.cloneSem.Release(1)

    return uc.vcs.Clone(ctx, url, token)
}
```

### Key Characteristics

| Aspect           | Value                                      |
| ---------------- | ------------------------------------------ |
| Library          | `golang.org/x/sync/semaphore`              |
| Default Limit    | 2 concurrent clones                        |
| Location         | UseCase layer (not Adapter)                |
| Configuration    | `WithMaxConcurrentClones(n)` option        |
| Context Handling | Automatic timeout/cancellation propagation |

## Options Considered

### Option A: Weighted Semaphore at UseCase (Selected)

**Description:**

Use `golang.org/x/sync/semaphore.Weighted` in UseCase to wrap clone calls.

**Pros:**

- Declarative intent: explicitly communicates "limit N concurrent operations"
- Context-aware: built-in timeout/cancellation handling
- FIFO ordering prevents starvation
- Configurable per UseCase instance
- Battle-tested stdlib extension

**Cons:**

- Per-instance limit (not cluster-wide)
- Static limit (can't dynamically adjust based on available memory)

### Option B: Semaphore at Git Adapter Level

**Description:**

Move concurrency control to the VCS adapter.

**Pros:**

- All VCS operations automatically throttled
- Single point of control

**Cons:**

- Wrong abstraction layer: resource management is business policy, not I/O detail
- Global limit: can't have different limits for different usecases
- Adapter becomes stateful, violating single responsibility
- Harder to test UseCase concurrency behavior

### Option C: Global Rate Limiter

**Description:**

Use `golang.org/x/time/rate` to limit clone request rate.

**Pros:**

- Simple API
- Well-understood pattern

**Cons:**

- Controls requests per time, not concurrent operations
- Doesn't prevent N clones starting simultaneously if N tokens available
- Wrong abstraction for resource exhaustion problem

### Option D: Channel-Based Worker Pool

**Description:**

Create dedicated clone worker pool with buffered channel.

**Pros:**

- Fine-grained control over worker lifecycle
- Can implement custom scheduling logic

**Cons:**

- Over-engineering: River already provides worker pool
- Nested worker pools complicate observability
- Requires manual context handling (`select` statement)
- More boilerplate than semaphore

## Implementation Principles

### Why UseCase Level

Concurrency control is a **business policy decision**:

```
┌─────────────────────────────────────┐
│   UseCase (AnalyzeUseCase)          │
│   ┌─────────────────────────────┐   │ ← Semaphore: Business decision
│   │ Semaphore Control           │   │   "Allow max N concurrent clones"
│   │  • Acquire before clone     │   │
│   │  • Release after clone      │   │
│   └─────────────────────────────┘   │
│              │                       │
│              ▼                       │
│     vcs.Clone(ctx, url, token)      │ ← Adapter call (thin wrapper)
└─────────────────────────────────────┘
```

- **UseCase knows execution context**: Aware of River worker concurrency, memory constraints
- **Adapter stays stateless**: Pure I/O, no resource management
- **Configuration flexibility**: Different usecases can have different limits

### Why Default = 2

| Limit | Memory (estimated) | Network       | Assessment                  |
| ----- | ------------------ | ------------- | --------------------------- |
| 1     | ~500MB             | Underutilized | Too conservative            |
| **2** | **~1GB**           | **Balanced**  | **Safe for 2GB instances**  |
| 3     | ~1.5GB             | High          | Risk OOM                    |
| 5     | ~2.5GB             | Maximum       | Guaranteed OOM on small VMs |

Assumptions:

- Average repository clone: ~500MB (code + .git history)
- Target deployment: 512MB-2GB RAM instances
- Need headroom for parser (tree-sitter), DB connections, OS

### Context Propagation

```go
// Execute sets 15-minute timeout
timeoutCtx, cancel := context.WithTimeout(ctx, uc.timeout)
defer cancel()

// Semaphore Acquire respects context
if err := uc.cloneSem.Acquire(ctx, 1); err != nil {
    return nil, err  // context.DeadlineExceeded if timeout
}
```

Benefits:

- **Timeout propagation**: Tasks don't hang waiting for semaphore
- **Graceful shutdown**: Worker shutdown cancels context, releases waiters
- **No goroutine leaks**: Automatic cleanup on cancellation

## Consequences

### Positive

**Memory Safety:**

- Maximum 2 concurrent clones limits peak memory usage
- Prevents OOM on constrained environments

**Predictable Behavior:**

- FIFO queue ordering: no starvation
- Deterministic throughput under load

**Context Integration:**

- Automatic timeout handling
- Clean cancellation propagation
- No manual cleanup required

**Operational Simplicity:**

- Single configuration option
- No external dependencies
- Observable via standard logging

### Negative

**Queue Wait Time:**

- During burst traffic, tasks wait for semaphore
- Mitigation: River queue depth monitoring

**Per-Instance Limit:**

- Not a cluster-wide limit
- 3 workers × 2 clones = 6 total concurrent clones
- Acceptable for current scale

**Static Configuration:**

- Can't dynamically adjust based on runtime memory
- Future improvement: integrate with resource monitoring

## Scaling Guidelines

| Instance Size     | Recommended Limit | Notes                       |
| ----------------- | ----------------- | --------------------------- |
| Small (512MB)     | 1                 | Conservative for free tiers |
| Medium (2GB)      | 2                 | Default configuration       |
| Large (8GB)       | 4                 | Higher throughput           |
| Dedicated (32GB+) | 8                 | Maximum parallel I/O        |

```go
// Configuration example
uc := NewAnalyzeUseCase(
    repo, vcs, parser, tokenLookup,
    WithMaxConcurrentClones(4),
)
```

## References

- [golang.org/x/sync/semaphore](https://pkg.go.dev/golang.org/x/sync/semaphore)
- [ADR-02: Clean Architecture Layers](./02-clean-architecture-layers.md) - Layer placement rationale
- [ADR-03: Graceful Shutdown](./03-graceful-shutdown.md) - Context propagation pattern
