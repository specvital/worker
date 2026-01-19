---
title: Domain Hints Extraction
description: ADR for AI-consumable metadata extraction from test files
---

# Core ADR-16: Domain Hints Extraction System

> [Korean Version](/ko/adr/core/16-domain-hints-extraction.md)

| Date       | Author       | Repo |
| ---------- | ------------ | ---- |
| 2026-01-18 | @KubrickCode | core |

## Context

### The Domain Classification Problem

The AI-based SpecView generation pipeline (ADR-14) requires domain classification of tests to group them into business domains (Authentication, Payment, UserManagement, etc.). Without context about what code is being tested, AI models cannot categorize tests meaningfully.

**Challenge**: Provide AI with semantic context without:

- Sending entire source files (excessive token consumption)
- Including noise that dilutes classification signal
- Building separate extractors for each of 12+ supported languages

### Requirements

| Requirement      | Description                                                |
| ---------------- | ---------------------------------------------------------- |
| Signal Density   | High ratio of meaningful domain indicators to tokens       |
| Token Efficiency | Minimize AI input tokens while preserving quality          |
| Cross-Language   | Unified extraction approach across all languages           |
| Noise Immunity   | Filter universal and language-specific non-domain patterns |
| AST Accuracy     | Distinguish code from comments and strings                 |

### Constraints

| Constraint             | Impact                                            |
| ---------------------- | ------------------------------------------------- |
| Tree-sitter Dependency | Must integrate with existing AST infrastructure   |
| Parallel Scanning      | Extraction must not block worker pool performance |
| Memory Budget          | Cannot load full ASTs for large repositories      |

## Decision

**Adopt a dual-field extraction model (Imports + Calls) with aggressive noise filtering and 2-segment call normalization.**

```go
type DomainHints struct {
    Imports []string  // Deduplicated import paths
    Calls   []string  // Normalized to 2 segments (a.b.c() → a.b)
}
```

### Key Design Choices

1. **Imports**: Direct indicators of external dependencies and their domains
2. **Calls**: Reveal interaction patterns with domain entities
3. **2-segment normalization**: `stripe.customers.create()` → `stripe.customers`
4. **No Variables field**: Removed after empirical testing showed no classification improvement

### Token Impact

| Metric         | Before   | After      | Improvement |
| -------------- | -------- | ---------- | ----------- |
| Token Volume   | 600K     | 90K        | 85% ↓       |
| Classification | Baseline | Equivalent | Maintained  |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Domain Hints Extraction                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Test File → Tree-sitter Parse → Language Extractor             │
│                                       │                          │
│              ┌────────────────────────┼────────────────────────┐│
│              │                        │                        ││
│              ▼                        ▼                        ▼│
│         Import Extraction       Call Extraction          Noise Filter│
│         - ES6 import            - Method calls           - Universal │
│         - CommonJS require      - 2-segment norm         - Per-lang  │
│         - Go import             - Chain flatten                     │
│         - Python import                                             │
│              │                        │                             │
│              └────────────┬───────────┘                             │
│                           ▼                                         │
│                    DomainHints{Imports, Calls}                     │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Language-Specific Extractors

| Language      | Import Patterns                  | Call Extraction         |
| ------------- | -------------------------------- | ----------------------- |
| Go            | `import "pkg"`, `import ("pkg")` | Function calls, methods |
| JavaScript/TS | `import x from`, `require()`     | Method chains           |
| Python        | `import x`, `from x import y`    | Function/method calls   |
| Java/Kotlin   | `import package.Class`           | Static/instance methods |
| C#            | `using Namespace;`               | Static/instance methods |
| Ruby          | `require`, `require_relative`    | Method calls            |
| PHP           | `use Namespace\Class`            | Function/method calls   |
| Rust          | `use crate::module`              | Function/method calls   |
| Swift         | `import Module`                  | Function/method calls   |
| C++           | `#include <header>`              | Function/method calls   |

### Call Normalization

```
Input:  stripe.customers.subscriptions.create()
Output: stripe.customers

Input:  authService.validateToken()
Output: authService.validateToken

Input:  db.query()
Output: db.query
```

Rationale: 2 segments preserve domain entity relationships while preventing token explosion.

### Noise Filtering

**Universal Filters:**

| Pattern           | Example        | Reason                    |
| ----------------- | -------------- | ------------------------- |
| Empty strings     | `""`           | No signal                 |
| Leading brackets  | `[item`        | Spread array artifacts    |
| URLs              | `http://...`   | Test fixtures             |
| Inline comments   | `// comment`   | Parser leakage            |
| Short identifiers | `a`, `fn`, `x` | Generic, no domain signal |

**Language-Specific Filters:**

| Language   | Filtered Patterns                              |
| ---------- | ---------------------------------------------- |
| Go         | `fmt`, `os`, `io`, `context`, `make`, `append` |
| Rust       | `Ok`, `Err`, `Some`, `None`, `unwrap`          |
| Java       | `toString`, `equals`, `hashCode`, `getClass`   |
| Kotlin     | `listOf`, `mapOf`, `emptyList`, `setOf`        |
| C#         | `System.*`, `nameof`                           |
| JavaScript | `console.*`, `JSON.*`                          |

## Options Considered

### Option A: Dual-Field Extraction (Selected)

Extract import statements and function calls only, with aggressive filtering.

| Aspect         | Assessment                                           |
| -------------- | ---------------------------------------------------- |
| Signal Density | Highest - imports and calls are strongest indicators |
| Token Cost     | 85% reduction vs full extraction                     |
| Implementation | 12 language extractors with shared filter            |
| Trade-off      | Some context loss from normalization                 |

### Option B: Full AST Extraction

Extract all identifiers, variables, string literals, and comments.

| Aspect         | Assessment                                         |
| -------------- | -------------------------------------------------- |
| Context        | Maximum information captured                       |
| Token Cost     | 6-7x more tokens than Option A                     |
| Signal Quality | Degraded by noise (loop variables, generics)       |
| Rejection      | Variables field test showed no classification gain |

### Option C: Regex-Based Pattern Matching

Use regular expressions to extract import/require statements.

| Aspect      | Assessment                                    |
| ----------- | --------------------------------------------- |
| Simplicity  | No tree-sitter dependency                     |
| Accuracy    | False positives from comments/strings         |
| Maintenance | Regex per language per pattern                |
| Rejection   | Contradicts unified AST architecture (ADR-03) |

## Consequences

### Positive

| Area             | Benefit                                              |
| ---------------- | ---------------------------------------------------- |
| Token Efficiency | 85% reduction enables cost-effective AI at scale     |
| Classification   | High signal-to-noise ratio improves AI accuracy      |
| Cross-Language   | Unified pattern across 12 languages                  |
| Integration      | Clean interface with tree-sitter infrastructure      |
| Evolution        | Variables removal demonstrates data-driven iteration |

### Negative

| Area          | Trade-off                                  |
| ------------- | ------------------------------------------ |
| Context Loss  | String literals and comments not captured  |
| Normalization | Deep call chains lose specificity          |
| Maintenance   | 12 extractors need grammar version updates |
| Tree-sitter   | No lightweight extraction fallback         |

### Technical Implications

| Aspect      | Implication                                           |
| ----------- | ----------------------------------------------------- |
| API Surface | `DomainHints` is public API; changes affect Worker    |
| Testing     | Golden snapshots per language for extraction behavior |
| Performance | ~1-2ms per file; negligible in parallel context       |
| Future      | Additional fields can be added if AI requires them    |

## References

- [ADR-03: Tree-sitter AST Parsing Engine](./03-tree-sitter-ast-parsing-engine.md)
- [ADR-12: Parallel Scanning with Worker Pool](./12-parallel-scanning-worker-pool.md)
- [ADR-14: AI-Based Spec Document Generation Pipeline](/en/adr/14-ai-spec-generation-pipeline.md)
- Commit `8b83b95`: Variables field removal after empirical testing
