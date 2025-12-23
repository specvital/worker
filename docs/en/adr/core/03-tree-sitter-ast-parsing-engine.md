---
title: Tree-sitter as AST Parsing Engine
description: ADR on choosing tree-sitter for multi-language test file parsing
---

# ADR-03: Tree-sitter as AST Parsing Engine

> 🇰🇷 [한국어 버전](/ko/adr/core/03-tree-sitter-ast-parsing-engine.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-03 | @KubrickCode | core  |

**Status**: Accepted

## Context

### Problem Statement

SpecVital Core requires parsing test files across:

- **Multiple testing frameworks** (Jest, Vitest, Playwright, JUnit, pytest, RSpec, etc.)
- **Multiple programming languages** (JavaScript, TypeScript, Python, Go, Java, C#, Ruby, etc.)
- **Real-world constraints**: Incomplete code, syntax errors, concurrent parsing, production reliability

### Requirements

1. **Multi-language support**: Single unified approach for all target languages
2. **Error recovery**: Parse incomplete or syntactically invalid code gracefully
3. **Accuracy**: Full AST access for precise test detection (not regex-level approximation)
4. **Performance**: Efficient for large repositories with thousands of test files
5. **Maintainability**: Minimize custom parser code per framework

### Strategic Question

Which parsing approach provides the best trade-off between accuracy, maintainability, and performance for multi-language test file analysis?

## Decision

**Use tree-sitter as the AST parsing engine via `smacker/go-tree-sitter` bindings.**

Tree-sitter provides:

- Unified C API across all 40+ supported languages
- GLR-based incremental parser with robust error recovery
- Language grammars maintained by active community
- Production-proven in VSCode, Neovim, GitHub Semantic

## Options Considered

### Option A: Tree-sitter (Selected)

Incremental parser generator with language-specific grammars.

**Pros:**

- **Single unified API**: Same `Node`, `Tree`, `Query` structures across all supported languages
- **Error recovery**: Parses incomplete code and returns usable AST
- **Community grammars**: 40+ languages with active maintenance
- **Production adoption**: VSCode, Neovim, Zed, GitHub Semantic
- **Performance**: O(n) time complexity, proven fast enough for real-time editor use

**Cons:**

- **C dependency**: Requires CGO for Go bindings
- **Grammar quality variance**: Community-maintained grammars have varying quality
- **Parser pooling issues**: Cancellation flag bug prevents parser reuse (see ADR-05)

### Option B: ANTLR4

ALL(\*) parser generator with extended BNF grammars.

**Pros:**

- Mature ecosystem with extensive grammar repository
- Built-in code completion engine
- Production-proven in compiler toolchains

**Cons:**

- **Performance**: 40x slower than hand-written parsers in benchmarks
- **No incremental parsing**: Must re-parse entire file on changes
- **Error recovery**: Less robust than tree-sitter for incomplete code
- **Go runtime overhead**: Performance penalties in Go target

### Option C: Regex Matching

Pattern matching on raw text.

**Pros:**

- Simple implementation, no external dependencies
- Very fast for basic patterns
- Works on any input (no syntax requirements)

**Cons:**

- **False positives**: Cannot distinguish code from comments or strings
- **No structure understanding**: Fails on nested constructs (describe/it blocks)
- **Maintenance nightmare**: Each framework needs custom patterns per language
- **Fragile**: Breaks with code style variations

### Option D: Custom Parsers per Language

Hand-written recursive descent parsers.

**Pros:**

- Maximum performance (40x faster than ANTLR possible)
- Full control over error handling and recovery
- Deep semantic analysis integration possible

**Cons:**

- **Development cost**: Multiple languages × multiple frameworks = unsustainable scope
- **Expertise required**: Language-specific parsing knowledge for each target
- **Maintenance burden**: Language changes require manual updates
- **Time to market**: Months/years to reach feature parity

## Implementation Details

### Architecture

```
pkg/parser/
├── tspool/              # Tree-sitter parser lifecycle management
│   └── pool.go          # Parser creation, language grammar caching
├── treesitter.go        # High-level utilities (GetNodeText, WalkTree)
├── parser_pool.go       # Query compilation caching
└── strategies/
    ├── jest/            # Jest framework parser (tree-sitter queries)
    ├── vitest/          # Vitest framework parser
    ├── playwright/      # Playwright framework parser
    └── shared/
        ├── jstest/      # Shared JS/TS parsing utilities
        ├── javaast/     # Shared Java parsing utilities
        └── dotnetast/   # Shared C# parsing utilities
```

### Language Grammar Initialization

Language grammars are initialized once via `sync.Once`:

```go
var (
    goLang    *sitter.Language
    jsLang    *sitter.Language
    // ... all supported languages
    langOnce  sync.Once
)

func initLanguages() {
    langOnce.Do(func() {
        goLang = golang.GetLanguage()
        jsLang = javascript.GetLanguage()
        // ...
    })
}
```

**Rationale**: Grammar initialization is expensive (C FFI calls, memory allocation). `sync.Once` ensures thread-safe single initialization while deferring the cost until first use.

### Parser Lifecycle

Fresh parser created per-use (see ADR-05 for pooling decision):

```go
func Parse(ctx context.Context, lang domain.Language, source []byte) (*sitter.Tree, error) {
    parser := Get(lang)        // Fresh parser
    defer parser.Close()       // Guaranteed cleanup

    tree, err := parser.ParseCtx(ctx, nil, source)
    if err != nil {
        return nil, fmt.Errorf("parse %s failed: %w", lang, err)
    }
    return tree, nil
}
```

### Query Caching

Tree-sitter queries are cached for performance:

```go
var queryCache sync.Map  // Concurrent map for compiled queries

func QueryWithCache(root *sitter.Node, source []byte, lang domain.Language, queryStr string) ([]QueryResult, error) {
    query, err := getCachedQuery(lang, queryStr)  // Compile once
    if err != nil {
        return nil, err
    }
    cursor := sitter.NewQueryCursor()
    defer cursor.Close()
    cursor.Exec(query, root)
    // ...
}
```

**Impact**: Query compilation: ~1-5ms (one-time) vs query execution: ~0.1-1ms (per file). 10-50x speedup for frameworks with many files.

## Consequences

### Positive

1. **Unified Multi-Language Support**
   - Single API for all supported languages
   - Shared utilities across similar frameworks (`jstest` for Jest/Vitest/Mocha)
   - New frameworks added without new parsing infrastructure

2. **Robust Error Handling**
   - Incomplete test files parsed without crashes
   - Defensive programming for C binding edge cases
   - Graceful degradation when AST extraction fails

3. **Production-Grade Performance**
   - Language grammar caching via `sync.Once`
   - Query compilation caching via `sync.Map`
   - Parallel parsing with worker pools (default: GOMAXPROCS)

4. **Community Leverage**
   - Grammar improvements benefit all users
   - Active ecosystem (40+ languages maintained)
   - Proven in production (GitHub, VSCode, Neovim)

### Negative

1. **CGO Dependency**
   - Complicates cross-compilation
   - Build time overhead
   - **Mitigation**: Acceptable for core library; pure-Go alternatives exist as fallback

2. **Parser Pooling Disabled**
   - Fresh parser allocation per file (~10µs overhead)
   - **Mitigation**: Grammar caching preserves main optimization; see ADR-05

3. **Limited Semantic Analysis**
   - Tree-sitter provides structure, not full semantics
   - No type resolution or symbol tables
   - **Mitigation**: Test detection needs structure only; semantic analysis out of scope

4. **Grammar Maintenance Risk**
   - Dependent on community grammar quality
   - **Mitigation**: Use popular grammars (JS, Python, Java) with active maintainers

### Trade-off Summary

| Aspect               | Tree-sitter         | Alternatives        |
| -------------------- | ------------------- | ------------------- |
| Multi-language       | Excellent (40+)     | Poor (per-language) |
| Error recovery       | Excellent           | Variable            |
| Development velocity | Fast (use grammars) | Slow (build custom) |
| Maintenance cost     | Low (community)     | High (internal)     |
| Performance          | Good (O(n), cached) | Variable            |

## Related ADRs

- [ADR-05: Parser Pooling Disabled](./05-parser-pooling-disabled.md) - Details on tree-sitter cancellation bug

## References

- [Tree-sitter Documentation](https://tree-sitter.github.io/tree-sitter/)
- [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) - Go bindings used
- [Why Tree-sitter - GitHub Semantic](https://github.com/github/semantic/blob/main/docs/why-tree-sitter.md)
- [Tree-sitter Performance Analysis - Symflower](https://symflower.com/en/company/blog/2023/parsing-code-with-tree-sitter/)
