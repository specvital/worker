---
title: Shared Parser Modules
description: ADR on language-level AST utility modules shared across test frameworks
---

# ADR-08: Shared Parser Modules

> :kr: [한국어 버전](/ko/adr/core/08-shared-parser-modules.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-23 | @KubrickCode | core  |

**Status**: Accepted

## Context

### Problem Statement

Test frameworks within the same language family share common parsing patterns:

1. **JavaScript/TypeScript**: Jest, Vitest, Mocha, Cypress, Playwright all use `describe()`/`it()` patterns
2. **Java**: JUnit 5, TestNG share annotation-based test discovery (`@Test`, `@Disabled`)
3. **C#**: xUnit, NUnit, MSTest share attribute-based patterns (`[Fact]`, `[Test]`)

Without shared utilities, each framework parser would duplicate:

- AST traversal logic
- Node extraction helpers
- Status/modifier parsing
- String unquoting and formatting

### Requirements

1. **Code Reuse**: Eliminate duplication across frameworks with similar patterns
2. **Consistent Behavior**: Ensure identical parsing logic across related frameworks
3. **Framework Independence**: Shared modules must not impose framework-specific behavior
4. **Maintainability**: Bug fixes in shared code should benefit all consumers
5. **Testability**: Shared utilities should be independently testable

### Strategic Question

How should parsing utilities be organized to maximize reuse while preserving framework-specific flexibility?

## Decision

**Create language-level shared parser modules under `pkg/parser/strategies/shared/`.**

Each module provides AST traversal utilities and common parsing functions for frameworks of that language family. Framework-specific parsers compose these utilities with their own detection logic.

## Options Considered

### Option A: Language-Level Shared Modules (Selected)

Organize shared code by language family with focused responsibilities.

**Pros:**

- **Clear boundaries**: Each module handles one language's AST patterns
- **Composable**: Frameworks pick utilities they need
- **Testable**: Utilities tested in isolation
- **Consistent**: Same parsing behavior across related frameworks

**Cons:**

- Tight coupling between shared module and its consumers
- Changes to shared code affect multiple frameworks
- Must be careful not to leak framework-specific behavior

### Option B: Per-Framework Duplication

Each framework implements its own parsing from scratch.

**Pros:**

- Complete isolation between frameworks
- No coordination needed for changes
- Framework can optimize for its specific patterns

**Cons:**

- **Massive duplication**: Same `describe()`/`it()` parsing logic repeated across Jest, Vitest, Mocha, Cypress, Playwright
- **Inconsistent behavior**: Bug fixes applied unevenly
- **Maintenance burden**: Same bug potentially fixed in multiple places

### Option C: Single Universal Parser

One parser handles all frameworks with configuration.

**Pros:**

- Maximum code sharing
- Single place for all parsing logic

**Cons:**

- **Over-generalization**: Forced to handle every framework's edge cases
- **Complex configuration**: Each framework needs extensive customization
- **Fragile**: Changes for one framework can break others

## Architecture

### Module Structure

```
pkg/parser/strategies/shared/
├── jstest/           # JavaScript/TypeScript test frameworks
│   ├── parser.go     # Main entry point: Parse()
│   ├── helpers.go    # AST extraction utilities
│   └── constants.go  # Shared constants (function names, modifiers)
├── javaast/          # Java frameworks
│   └── ast.go        # Annotation/method utilities
├── dotnetast/        # C# frameworks
│   └── ast.go        # Attribute/method utilities
├── kotlinast/        # Kotlin frameworks
│   └── ast.go        # Annotation utilities
├── pyast/            # Python frameworks
│   └── ast.go        # Decorator/function utilities
├── rubyast/          # Ruby frameworks
│   ├── ast.go        # Method call utilities
│   └── helpers.go    # Block parsing
├── swiftast/         # Swift frameworks
│   └── ast.go        # Method utilities
├── phpast/           # PHP frameworks
│   └── ast.go        # Annotation/method utilities
└── configutil/       # Configuration file parsing
    └── strings.go    # String extraction utilities
```

### Responsibility Separation

| Layer                    | Responsibility               | Example                                          |
| ------------------------ | ---------------------------- | ------------------------------------------------ |
| **Shared Module**        | Language AST patterns        | `jstest.ParseNode()`, `javaast.GetAnnotations()` |
| **Framework Parser**     | Framework-specific detection | Jest's `jest.fn()` matcher                       |
| **Framework Definition** | Registration and matchers    | `framework.Register()`                           |

### jstest Module (JavaScript/TypeScript)

The most complex shared module, supporting multiple frameworks:

**Consumers**: Jest, Vitest, Mocha, Cypress, Playwright

**Key Functions**:

```go
// Main entry point - parses entire file
func Parse(ctx context.Context, source []byte, filename string, framework string) (*domain.TestFile, error)

// Recursive AST traversal
func ParseNode(node *sitter.Node, source []byte, filename string, file *domain.TestFile, currentSuite *domain.TestSuite)

// Test/suite creation
func ProcessTest(callNode, args *sitter.Node, source []byte, filename string, file *domain.TestFile, parentSuite *domain.TestSuite, status domain.TestStatus, modifier string)
func ProcessSuite(callNode, args *sitter.Node, source []byte, filename string, file *domain.TestFile, parentSuite *domain.TestSuite, status domain.TestStatus, modifier string)

// .each() parameterized tests
func ProcessEachTests(callNode *sitter.Node, testCases []string, nameTemplate string, ...)
func ProcessEachSuites(callNode *sitter.Node, testCases []string, nameTemplate string, callback *sitter.Node, ...)
```

**Shared Constants**:

```go
const (
    FuncDescribe = "describe"
    FuncIt       = "it"
    FuncTest     = "test"
    FuncContext  = "context"    // Mocha TDD
    FuncSpecify  = "specify"    // Mocha TDD
    FuncSuite    = "suite"      // Mocha TDD
    FuncBench    = "bench"      // Vitest benchmark

    ModifierOnly = "only"
    ModifierSkip = "skip"
    ModifierTodo = "todo"
    ModifierEach = "each"
)
```

### javaast Module (Java)

**Consumers**: JUnit 5, TestNG

**Key Functions**:

```go
// Annotation extraction
func GetAnnotations(modifiers *sitter.Node) []*sitter.Node
func GetAnnotationName(annotation *sitter.Node, source []byte) string
func HasAnnotation(modifiers *sitter.Node, source []byte, annotationName string) bool
func GetAnnotationArgument(annotation *sitter.Node, source []byte) string

// Class/method utilities
func GetClassName(node *sitter.Node, source []byte) string
func GetMethodName(node *sitter.Node, source []byte) string
func GetClassBody(node *sitter.Node) *sitter.Node
func GetModifiers(node *sitter.Node) *sitter.Node
```

### dotnetast Module (C#)

**Consumers**: xUnit, NUnit, MSTest

**Key Functions**:

```go
// Attribute extraction
func GetAttributeLists(node *sitter.Node) []*sitter.Node
func GetAttributes(attributeLists []*sitter.Node) []*sitter.Node
func GetAttributeName(attribute *sitter.Node, source []byte) string
func HasAttribute(attributeLists []*sitter.Node, source []byte, attributeName string) bool

// String utilities
func ExtractStringContent(node *sitter.Node, source []byte) string
func ParseAssignmentExpression(argNode *sitter.Node, source []byte) (string, string)

// File naming conventions
func IsCSharpTestFileName(filename string) bool
```

## Usage Pattern

Framework parsers delegate to shared modules while adding framework-specific behavior:

```go
// pkg/parser/strategies/jest/definition.go
type JestParser struct{}

func (p *JestParser) Parse(ctx context.Context, source []byte, filename string) (*domain.TestFile, error) {
    // Delegate to shared module
    return jstest.Parse(ctx, source, filename, "jest")
}

// pkg/parser/strategies/vitest/definition.go
type VitestParser struct{}

func (p *VitestParser) Parse(ctx context.Context, source []byte, filename string) (*domain.TestFile, error) {
    // Same shared module, different framework name
    return jstest.Parse(ctx, source, filename, "vitest")
}
```

For languages with more variation, frameworks use utilities selectively:

```go
// pkg/parser/strategies/junit5/definition.go
func parseTestMethod(node *sitter.Node, source []byte, ...) *domain.Test {
    modifiers := javaast.GetModifiers(node)
    annotations := javaast.GetAnnotations(modifiers)

    // Framework-specific annotation handling
    for _, ann := range annotations {
        name := javaast.GetAnnotationName(ann, source)
        switch name {
        case "Test", "ParameterizedTest", "RepeatedTest":
            isTest = true
        case "Disabled":
            status = domain.TestStatusSkipped
        }
    }
    // ...
}
```

## Consequences

### Positive

1. **Significant Code Reuse**
   - jstest module shared across 5+ frameworks
   - dotnetast module shared across 3 frameworks
   - javaast module shared across 2 frameworks

2. **Consistent Parsing Behavior**
   - `describe.skip()` parsed identically in Jest, Vitest, Mocha
   - `@Disabled` annotation handled consistently in JUnit 5, TestNG

3. **Centralized Bug Fixes**
   - Fix in `jstest.ProcessEachTests()` benefits all JavaScript frameworks
   - String unquoting fixed once, applied everywhere

4. **Clear Layering**
   - Shared modules: AST patterns (language-specific)
   - Framework parsers: Detection and registration (framework-specific)

### Negative

1. **Coupling Between Frameworks**
   - Bug in shared module affects multiple frameworks
   - **Mitigation**: Comprehensive test coverage for shared modules

2. **Potential Over-Generalization**
   - Risk of adding framework-specific code to shared modules
   - **Mitigation**: Code review to enforce language-only patterns

3. **Implicit Dependencies**
   - Framework behavior depends on shared module implementation
   - **Mitigation**: Document shared module contracts clearly

### Trade-off Summary

| Aspect      | Shared Modules | Per-Framework | Universal Parser |
| ----------- | -------------- | ------------- | ---------------- |
| Code reuse  | Excellent      | None          | Maximum          |
| Isolation   | Moderate       | Complete      | None             |
| Flexibility | High           | Maximum       | Low              |
| Maintenance | Moderate       | High          | Low              |
| Consistency | High           | Variable      | Maximum          |

## Related ADRs

- [ADR-03: Tree-sitter as AST Parsing Engine](./03-tree-sitter-ast-parsing-engine.md) - Foundation for shared AST utilities
- [ADR-06: Unified Framework Definition](./06-unified-framework-definition.md) - How frameworks compose shared modules

## References

- [DRY Principle](https://en.wikipedia.org/wiki/Don%27t_repeat_yourself)
- [Composition over Inheritance](https://en.wikipedia.org/wiki/Composition_over_inheritance)
