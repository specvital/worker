---
title: Unified Framework Definition
description: ADR on consolidating framework components into a single Definition type
---

# ADR-06: Unified Framework Definition System

> :kr: [한국어 버전](/ko/adr/core/06-unified-framework-definition.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-23 | @KubrickCode | core  |

**Status**: Accepted

## Context

### Problem Statement

The original architecture used a **dual registry pattern** where framework components were split across separate registries:

1. **Matchers Registry**: Stored framework detection rules
2. **Strategies Registry**: Stored test file parsers

This separation caused several issues:

- **Synchronization burden**: Adding a new framework required modifications in multiple places
- **Registration fragility**: Easy to register a matcher without a corresponding parser (or vice versa)
- **Scattered definitions**: Framework behavior was spread across multiple files and packages
- **Testing complexity**: Mocking required coordinating multiple registries

### Requirements

1. **Single registration point**: One place to define everything about a framework
2. **Self-contained definition**: All framework components bundled together
3. **Type safety**: Compile-time verification of complete framework definitions
4. **Extensibility**: Easy to add new frameworks with minimal boilerplate

### Strategic Question

How should framework components (detection, configuration parsing, test parsing) be organized to minimize coupling and maintenance burden?

## Decision

**Consolidate all framework components into a single `framework.Definition` type with a unified registry.**

Each framework provides one `Definition` struct that bundles:

- Framework identity (name, supported languages)
- Detection rules (matchers)
- Configuration parser
- Test file parser
- Priority for detection ordering

## Options Considered

### Option A: Unified Definition (Selected)

Single struct type containing all framework components.

**Pros:**

- **Single file per framework**: Complete framework definition in one `definition.go`
- **Self-documenting**: All framework behavior visible in one place
- **Compile-time complete**: Missing components cause compilation errors
- **Simple registration**: Single `framework.Register()` call in `init()`
- **Easy testing**: Mock entire framework with one struct

**Cons:**

- Larger struct size (contains all components)
- All components must be defined (no partial frameworks)

### Option B: Dual Registry (Original)

Separate registries for matchers and parsers.

**Pros:**

- Fine-grained control over individual components
- Potentially smaller memory footprint per registry

**Cons:**

- **Synchronization required**: Must update both registries for each framework
- **Easy to forget**: Registration in one registry without the other
- **Scattered code**: Framework logic spread across packages
- **Harder to test**: Must coordinate multiple mock registries

### Option C: Plugin System

Dynamic plugin loading at runtime.

**Pros:**

- Maximum flexibility for adding frameworks
- No recompilation needed for new frameworks

**Cons:**

- **Complexity**: Plugin discovery, loading, and lifecycle management
- **Type safety loss**: Runtime errors instead of compile-time
- **Deployment burden**: Manage separate plugin binaries
- **Overkill**: Static registration sufficient for current needs

## Implementation Details

### Definition Structure

```go
type Definition struct {
    // Framework identity
    Name      string
    Languages []domain.Language

    // Detection components
    Matchers []Matcher

    // Configuration parsing (optional)
    ConfigParser ConfigParser

    // Test file parsing
    Parser Parser

    // Priority for detection ordering
    Priority int
}
```

### Core Interfaces

```go
// Matcher evaluates detection signals
type Matcher interface {
    Match(ctx context.Context, signal Signal) MatchResult
}

// ConfigParser extracts settings from framework config files
type ConfigParser interface {
    Parse(ctx context.Context, configPath string, content []byte) (*ConfigScope, error)
}

// Parser extracts test definitions from source code
type Parser interface {
    Parse(ctx context.Context, source []byte, filename string) (*domain.TestFile, error)
}
```

### Registration Pattern

Each framework registers via `init()`:

```go
// pkg/parser/strategies/jest/definition.go
func init() {
    framework.Register(NewDefinition())
}

func NewDefinition() *framework.Definition {
    return &framework.Definition{
        Name:      "jest",
        Languages: []domain.Language{domain.LanguageTypeScript, domain.LanguageJavaScript},
        Matchers: []framework.Matcher{
            matchers.NewImportMatcher("@jest/globals", "@jest/", "jest"),
            matchers.NewConfigMatcher("jest.config.js", "jest.config.ts"),
            &JestContentMatcher{},
        },
        ConfigParser: &JestConfigParser{},
        Parser:       &JestParser{},
        Priority:     framework.PriorityGeneric,
    }
}
```

**Critical**: Blank import is required to trigger `init()`:

```go
import (
    _ "github.com/specvital/core/pkg/parser/strategies/jest"
)
```

### Registry Architecture

```
pkg/parser/
├── framework/
│   ├── definition.go     # Definition type and interfaces
│   ├── registry.go       # Single unified registry
│   ├── scope.go          # ConfigScope for config file handling
│   ├── constants.go      # Priority levels, framework names
│   └── matchers/         # Reusable matcher implementations
│       ├── import.go     # Import statement matcher
│       ├── config.go     # Config file matcher
│       └── content.go    # Content pattern matcher
└── strategies/
    ├── jest/definition.go
    ├── vitest/definition.go
    ├── playwright/definition.go
    └── gotesting/definition.go
```

### Priority System

```go
const (
    PriorityGeneric     = 100  // Jest, Go testing
    PriorityE2E         = 150  // Playwright, Cypress
    PrioritySpecialized = 200  // Vitest (needs explicit import detection)
)
```

Higher priority frameworks are evaluated first during detection.

## Consequences

### Positive

1. **Single Source of Truth**
   - All framework behavior defined in one file
   - No cross-file coordination required
   - Clear ownership and responsibility

2. **Reduced Boilerplate**
   - New framework requires only one `definition.go` file
   - Reusable matcher components (`matchers/` package)
   - Shared parsing utilities (`shared/jstest/`, etc.)

3. **Improved Maintainability**
   - Framework changes localized to single file
   - Compile-time verification of completeness
   - Self-documenting structure

4. **Better Testability**
   - Mock entire framework with single struct
   - Test detection, config parsing, and test parsing together
   - Isolated unit tests per framework

### Negative

1. **Blank Import Requirement**
   - Consumers must explicitly import each framework package
   - Missing import causes silent framework exclusion
   - **Mitigation**: Document required imports in README; consider registry validation

2. **Global State Dependency**
   - `defaultRegistry` is a package-level variable
   - `init()` order matters for registration
   - **Mitigation**: Go guarantees init() runs before main(); registry is thread-safe

3. **All-or-Nothing Definition**
   - Cannot register partial framework (e.g., matcher only)
   - **Mitigation**: This is intentional; partial frameworks cause bugs in dual registry

### Trade-off Summary

| Aspect             | Unified Definition | Dual Registry     |
| ------------------ | ------------------ | ----------------- |
| Registration       | Single call        | Multiple calls    |
| File organization  | One file/framework | Multiple files    |
| Component coupling | High (intentional) | Low               |
| Maintenance burden | Low                | High              |
| Type safety        | Compile-time       | Runtime potential |

## Related ADRs

- [ADR-03: Tree-sitter as AST Parsing Engine](./03-tree-sitter-ast-parsing-engine.md) - Parser implementation
- [ADR-04: Early-Return Framework Detection](./04-early-return-framework-detection.md) - Detection algorithm using matchers

## References

- [Go `init()` Function](https://go.dev/doc/effective_go#init) - Official documentation on init functions
- [Accept Interfaces, Return Structs](https://bryanftan.medium.com/accept-interfaces-return-structs-in-go-d4cab29a301b) - Go interface design principle
