---
title: Parser Pooling Disabled
description: ADR on disabling tree-sitter parser pooling due to cancellation flag bug
---

# ADR-05: Parser Pooling Disabled

> 🇰🇷 [한국어 버전](/ko/adr/core/05-parser-pooling-disabled.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-23 | @KubrickCode | core  |

**Status**: Accepted

## Context

### Problem Statement

Tree-sitter parsers were initially pooled using `sync.Pool` for performance optimization. However, this caused intermittent test failures that were difficult to reproduce and diagnose.

### Root Cause

When a context is cancelled during `ParseCtx()` execution:

1. Tree-sitter sets an internal cancellation flag
2. This flag is **not properly reset** when the parser is returned to the pool
3. Subsequent reuse of that parser fails with **"operation limit was hit"** error

### Impact

- Flaky tests in CI/CD pipelines
- Non-deterministic behavior in production
- Debugging complexity due to intermittent nature

### Strategic Question

How should we handle tree-sitter parser lifecycle to ensure reliability while maintaining acceptable performance?

## Decision

**Disable parser pooling. Create fresh parsers per-use while caching language grammars via `sync.Once`.**

This approach:

- Eliminates the cancellation flag bug completely
- Preserves the main performance optimization (grammar caching)
- Trades ~10µs per-parse overhead for guaranteed reliability

## Options Considered

### Option A: Fresh Parser Per-Use (Selected)

Create a new parser for each parse operation.

**Pros:**

- **Guaranteed reliability**: No state leakage between parse operations
- **Simple implementation**: No pool management complexity
- **Predictable behavior**: Each parse is independent

**Cons:**

- **Per-parse overhead**: ~10µs allocation cost per file
- **More GC pressure**: Fresh allocations increase garbage collection work

### Option B: Fix Tree-sitter Bug Upstream

Contribute a fix to the tree-sitter C library.

**Pros:**

- Addresses root cause
- Benefits entire tree-sitter ecosystem

**Cons:**

- **External dependency**: Fix timeline not under our control
- **Maintenance burden**: Must track upstream changes
- **Uncertain acceptance**: PR may not be accepted or may take months

### Option C: Manual Flag Reset

Implement workaround to reset parser state before reuse.

**Pros:**

- Preserves pooling performance benefits

**Cons:**

- **Fragile**: Depends on internal tree-sitter implementation details
- **Maintenance risk**: May break with tree-sitter updates
- **Incomplete**: May not address all edge cases

## Implementation Details

### Current Architecture

```
pkg/parser/tspool/
├── pool.go         # Parser creation, language grammar caching
└── pool_test.go    # Concurrency tests (race detection)
```

### Parser Creation

Fresh parser created per-use:

```go
func Get(lang domain.Language) *sitter.Parser {
    initLanguages()
    parser := sitter.NewParser()
    parser.SetLanguage(GetLanguage(lang))
    return parser
}
```

### Language Grammar Caching

Expensive grammar initialization is still cached via `sync.Once`:

```go
var (
    goLang   *sitter.Language
    jsLang   *sitter.Language
    // ... all supported languages
    langOnce sync.Once
)

func initLanguages() {
    langOnce.Do(func() {
        goLang = golang.GetLanguage()
        jsLang = javascript.GetLanguage()
        // ...
    })
}
```

**Rationale**: Grammar initialization involves C FFI calls and memory allocation. `sync.Once` ensures thread-safe single initialization while deferring the cost until first use.

### Parse Helper

The `Parse` function provides a clean API with guaranteed cleanup:

```go
func Parse(ctx context.Context, lang domain.Language, source []byte) (*sitter.Tree, error) {
    parser := Get(lang)
    defer parser.Close()

    tree, err := parser.ParseCtx(ctx, nil, source)
    if err != nil {
        return nil, fmt.Errorf("parse %s failed: %w", lang, err)
    }
    return tree, nil
}
```

### Performance Impact

| Operation             | Overhead      | Status      |
| --------------------- | ------------- | ----------- |
| Parser allocation     | ~10µs/parse   | Acceptable  |
| Language grammar init | ~1-5ms        | Cached once |
| Query compilation     | ~1-5ms        | Cached once |
| Query execution       | ~0.1-1ms/file | Optimized   |

**Net Impact**: Grammar and query caching provide 10-50x speedup for repeated operations. The ~10µs per-parse overhead is negligible compared to typical file I/O latency.

## Consequences

### Positive

1. **Test Stability**
   - No more flaky tests from parser state leakage
   - Deterministic CI/CD pipeline behavior

2. **Code Simplicity**
   - No pool management code to maintain
   - Clear ownership semantics (caller creates, caller closes)

3. **Debugging Ease**
   - Each parse operation is isolated
   - No cross-contamination between operations

### Negative

1. **Per-Parse Overhead**
   - ~10µs allocation per file
   - **Mitigation**: Acceptable for core library use case

2. **Increased GC Pressure**
   - More short-lived allocations
   - **Mitigation**: Grammar caching keeps most allocations long-lived

### Constraints on Future Changes

- **Cannot re-enable pooling** without upstream tree-sitter fix
- **Performance optimization efforts** must focus on query caching, not parser reuse

## Related ADRs

- [ADR-03: Tree-sitter as AST Parsing Engine](./03-tree-sitter-ast-parsing-engine.md) - Why tree-sitter was chosen

## References

- [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) - Go bindings used
- [Tree-sitter Documentation](https://tree-sitter.github.io/tree-sitter/) - Official documentation
