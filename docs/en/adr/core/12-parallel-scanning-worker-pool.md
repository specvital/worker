---
title: Parallel Scanning
description: ADR on bounded parallel processing for large-scale test file scanning
---

# ADR-12: Parallel Scanning with Worker Pool

> [한국어 버전](/ko/adr/core/12-parallel-scanning-worker-pool.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-23 | @KubrickCode | core  |

## Context

### Problem Statement

Large repositories contain thousands of test files. Sequential parsing creates unacceptable latency:

1. **Scale**: Monorepos with 5,000+ test files
2. **User Experience**: Synchronous API calls must respond within reasonable time
3. **Resource Efficiency**: Modern machines have multiple cores

### Technical Requirements

- **Bounded Concurrency**: Prevent resource exhaustion from unbounded parallelism
- **Context Propagation**: Support timeout and cancellation
- **Error Resilience**: Partial failures should not abort entire scan
- **Deterministic Output**: Results must be reproducible regardless of goroutine scheduling

## Decision

**Use errgroup + semaphore pattern for bounded parallel processing with deterministic output ordering.**

```go
sem := semaphore.NewWeighted(int64(workers))
g, gCtx := errgroup.WithContext(ctx)

for _, file := range files {
    g.Go(func() error {
        if err := sem.Acquire(gCtx, 1); err != nil {
            return nil
        }
        defer sem.Release(1)
        // parse file...
        return nil
    })
}
_ = g.Wait()
sort.Slice(results, ...)  // deterministic ordering
```

## Options Considered

### Option A: errgroup + semaphore (Selected)

Combine errgroup for goroutine lifecycle with semaphore for bounded concurrency.

**Pros:**

- **Bounded Concurrency**: Configurable worker count prevents resource exhaustion
- **Context Integration**: Native context cancellation and timeout support
- **Error Propagation**: errgroup collects first error automatically
- **Standard Library**: Uses `golang.org/x/sync` (quasi-standard)

**Cons:**

- Semaphore acquisition adds slight overhead
- Two synchronization primitives to understand

### Option B: Worker Pool with Channels

Classic producer-consumer pattern with fixed worker goroutines.

```go
jobs := make(chan string, len(files))
results := make(chan TestFile)

for w := 0; w < workers; w++ {
    go worker(jobs, results)
}
```

**Pros:**

- Familiar pattern
- No external dependencies

**Cons:**

- More boilerplate code
- Complex channel coordination
- Error handling requires additional channels
- Context cancellation requires manual propagation

### Option C: Unbounded Parallelism

Launch goroutine per file without concurrency control.

**Pros:**

- Simplest implementation
- Maximum theoretical throughput

**Cons:**

- **Resource Exhaustion**: Thousands of concurrent goroutines
- **Memory Pressure**: Each parser holds AST in memory
- **File Descriptor Limits**: OS limits on open files
- Unpredictable performance under load

## Consequences

### Positive

1. **Configurable Performance**
   - Default: GOMAXPROCS (CPU-bound optimization)
   - Configurable via `WithWorkers(n)` option
   - Upper bound (MaxWorkers) prevents misconfiguration

2. **Graceful Degradation**
   - Context cancellation stops accepting new work
   - In-progress files complete or timeout
   - Partial results returned with error list

3. **Deterministic Results**
   - Post-sort by path ensures reproducible output
   - Critical for testing and caching strategies

4. **Resource Safety**
   - Memory usage bounded by worker count
   - File descriptors managed per-worker
   - CPU utilization predictable

### Negative

1. **Sorting Overhead**
   - Results sorted after parallel collection
   - **Mitigation**: O(n log n) acceptable for typical file counts

2. **Semaphore Contention**
   - High worker counts may see acquisition delays
   - **Mitigation**: Default GOMAXPROCS balances throughput vs contention

### Configuration

| Option  | Default    | Max  | Description             |
| ------- | ---------- | ---- | ----------------------- |
| Workers | GOMAXPROCS | 1024 | Concurrent file parsers |
| Timeout | 5min       | -    | Total scan timeout      |

### Usage

```go
result, err := parser.Scan(ctx, src,
    parser.WithWorkers(8),
    parser.WithTimeout(2*time.Minute),
)
```

## References

- `pkg/parser/scanner.go` - parseFilesParallel implementation
- `pkg/parser/options.go` - Functional options
- `golang.org/x/sync/errgroup` - Goroutine group management
- `golang.org/x/sync/semaphore` - Weighted semaphore
