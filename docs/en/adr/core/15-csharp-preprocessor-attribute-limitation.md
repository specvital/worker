---
title: C# Preprocessor Block Attribute Detection Limitation
description: ADR for attribute detection limitation inside conditional compilation blocks due to tree-sitter-c-sharp grammar
---

# ADR-15: C# Preprocessor Block Attribute Detection Limitation

> 🇰🇷 [Korean Version](/ko/adr/core/15-csharp-preprocessor-attribute-limitation.md)

| Date       | Author       | Repository |
| ---------- | ------------ | ---------- |
| 2026-01-04 | @KubrickCode | core       |

**Status**: Accepted

## Background

### Problem Definition

The tree-sitter-c-sharp grammar parses preprocessor directives (`#if`, `#else`, `#elif`) between attributes as `ERROR` nodes instead of `preproc_if` nodes.

### Discovery

Validation of `fluentassertions/fluentassertions` repository:

- Ground Truth (AI Manual Analysis): 5,995 tests
- Parser Result: 6,009 tests
- Delta: +14 (+0.23%)

The delta is positive because GT analysis errors outnumber parser bugs. The actual parser bug caused 2 tests to be missed in `AssertionExtensionsSpecs.cs`.

### Technical Analysis

```csharp
// InlineData(2) is not detected in this pattern
[Theory]
[InlineData(1)]
#if NET6_0_OR_GREATER
[InlineData(2)]  // ← Parser misses this
#endif
public void Test(int x) { }
```

Actual tree-sitter parsing result:

```
method_declaration
├── attribute_list [Theory]
├── attribute_list [InlineData(1)]
├── ERROR                          ← Not preproc_if!
│   └── #if NET6_0_OR_GREATER
│       └── (InlineData(2) incorrectly parsed)
└── public void Test()
```

**Note**: Class-level `#if` (wrapping entire methods) works correctly:

```csharp
// This pattern is detected correctly
#if NET6_0_OR_GREATER
[Fact]
public void Net6OnlyTest() { }
#endif
```

## Decision

**Test attribute detection inside preprocessor blocks between attributes is not supported.**

This is a tree-sitter-c-sharp grammar-level issue that cannot be fixed in the SpecVital Core parser.

### Rationale

1. **Grammar-level limitation**: tree-sitter-c-sharp generates incorrect AST, making parser-level workaround impossible
2. **Upstream dependency**: Fix requires modifying the tree-sitter-c-sharp grammar itself
3. **Limited impact**: Most C# projects don't use preprocessors between attributes

## Options Considered

### Option A: Accept Limitation and Document (Selected)

Document the limitation and verify behavior with tests.

**Pros:**

- Honest representation of limitations
- Automatically resolved if tree-sitter-c-sharp is fixed

**Cons:**

- Test under-count in certain codebases

### Option B: Text-based Preprocessor Expansion

Process preprocessor directives at text level before AST parsing.

**Pros:**

- Can detect attributes inside preprocessor blocks

**Cons:**

- **Complexity explosion**: Requires condition evaluation, nesting handling, multiple branch processing
- **Accuracy degradation**: Cannot know which conditions are active
- **Architecture violation**: Conflicts with tree-sitter-based parsing principles

### Option C: Fork tree-sitter-c-sharp

Directly modify the grammar to support preprocessors between attributes.

**Pros:**

- Fundamental solution

**Cons:**

- **Maintenance burden**: Must continuously merge upstream changes
- **Scope creep**: Forking entire grammar for single issue
- **Uncertainty**: Difficult to predict side effects of grammar modification

## Consequences

### Positive

1. **Architecture integrity**: Maintains tree-sitter-based parsing model
2. **Clear limitations**: Documented in code comments and tests
3. **Maintainability**: No complex workarounds

### Negative

1. **Accuracy gap**: Projects using preprocessors between attributes will have under-counted tests
2. **FluentAssertions impact**: Under-count due to `[InlineData]` usage in `#if` blocks

### Mitigation

1. **Minimal impact**: Most projects don't use this pattern
2. **Class-level works**: `#if` wrapping entire methods works correctly
3. **Documentation**: Limitation documented in `GetAttributeLists()` function comment

## Framework Impact

| Framework | Affected Pattern        | Severity |
| --------- | ----------------------- | -------- |
| xUnit     | `[InlineData]` in `#if` | Low      |
| NUnit     | `[TestCase]` in `#if`   | Low      |
| MSTest    | `[DataRow]` in `#if`    | Low      |

Most C# test projects use class-level or method-level conditional compilation. The pattern of inserting preprocessors between attributes is rare.

## Related ADRs

- [ADR-02: Dynamic Test Counting Policy](./02-dynamic-test-counting-policy.md) - Another accuracy limitation
- [ADR-03: Tree-sitter AST Parsing Engine](./03-tree-sitter-ast-parsing-engine.md) - Tree-sitter-based parsing principles
- [ADR-14: Indirect Import Alias Detection Unsupported](./14-indirect-import-unsupported.md) - Similar limitation documentation pattern

## References

- [tree-sitter-c-sharp GitHub](https://github.com/tree-sitter/tree-sitter-c-sharp)
- Validation Report: `realworld-test-report.md`
- Limitation Test: `pkg/parser/strategies/shared/dotnetast/ast_test.go:TestGetAttributeLists_PreprocessorLimitation`
