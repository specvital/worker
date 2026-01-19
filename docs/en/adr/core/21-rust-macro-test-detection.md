---
title: Rust Macro-Based Test Detection
description: ADR for implementing two-phase macro detection strategy for Rust test files
---

# ADR-21: Rust Macro-Based Test Detection

> 🇰🇷 [한국어 버전](/ko/adr/core/21-rust-macro-test-detection)

| Date       | Author     | Repos |
| ---------- | ---------- | ----- |
| 2025-12-27 | @specvital | core  |

## Context

### Problem Statement

SpecVital Core's Rust parser only detected functions annotated with the standard `#[test]` attribute. This approach missed tests defined through macro-based patterns, which are common in mature Rust codebases.

### Discovery

Validation against the `BurntSushi/ripgrep` repository revealed a significant detection gap:

| Metric                      | Value                 |
| --------------------------- | --------------------- |
| Ground Truth (`cargo test`) | 1,111 tests           |
| Parser Detection            | 436 tests             |
| **Delta**                   | **-675 (61% missed)** |

Root cause analysis identified two categories of undetected tests:

**Category 1: Name-Based Macros** (~330 tests)

Macros with "test" in the name that generate test functions:

```rust
// ripgrep/tests/integration.rs
rgtest!(f1, |dir: Dir, mut cmd: TestCommand| {
    // test body
});
```

**Category 2: Definition-Based Macros** (~264 tests)

Macros whose definitions expand to `#[test]` internally:

```rust
// Definition in same file
macro_rules! syntax {
    ($name:ident, $re:expr, $hay:expr) => {
        #[test]
        fn $name() {
            // ...
        }
    };
}

// Usage
syntax!(test_literal, r"foo", "foo");
```

### Technical Constraints

1. **Single-file parsing constraint** (ADR-14): Core parser operates on files in isolation without cross-file dependency resolution
2. **Tree-sitter limitation**: AST parsing provides structure but no macro expansion
3. **Rust macro types**:
   - **Declarative macros** (`macro_rules!`): Definitions analyzable if in same file
   - **Procedural attribute macros** (`rstest`, `test_case`): External crate implementations, opaque without compiler expansion

### Strategic Imperative

61% detection miss undermines platform credibility for Rust ecosystem. A solution must balance accuracy improvement against architectural constraints.

## Decision

**Implement a two-phase macro-based test detection strategy that analyzes both macro names and same-file macro definitions.**

### Phase 1: Name-Based Heuristic

Detect macro invocations where the macro name contains "test" (case-insensitive):

```rust
// Detected: macro name contains "test"
rgtest!(test_name, |...| { ... });
test_case!(name, input, expected);
```

### Phase 2: Definition Analysis

For `macro_rules!` definitions in the same file, analyze whether the macro body contains `#[test]` attribute expansion:

```rust
// Step 1: Collect macro definitions
macro_rules! syntax {  // <- definition found
    ($name:ident, $re:expr, $hay:expr) => {
        #[test]  // <- expands to #[test]
        fn $name() { ... }
    };
}

// Step 2: Count invocations of test-generating macros
syntax!(test_one, ...);   // <- counted as test
syntax!(test_two, ...);   // <- counted as test
```

### Implementation

Two-pass AST analysis in `pkg/parser/strategies/cargotest/definition.go`:

```go
func parseRustAST(root *sitter.Node, source []byte) []domain.TestSuite {
    // Pass 1: Collect macro definitions that generate #[test]
    macroRegistry := collectTestMacroDefinitions(root, source)

    // Pass 2: Traverse all nodes using registry + name heuristic
    var tests []domain.Test
    walkTree(root, func(node *sitter.Node) {
        switch node.Type() {
        case "function_item":
            if hasTestAttribute(node) {
                tests = append(tests, extractAttributeTest(node))
            }
        case "macro_invocation":
            if isTestMacro(node, macroRegistry) {
                tests = append(tests, extractMacroTest(node))
            }
        }
    })
    return tests
}
```

Key functions:

- `collectTestMacroDefinitions()`: First pass to build registry of test-generating macros
- `tokenTreeHasTestAttribute()`: Recursively searches `macro_rules!` body for `#[test]`
- `isTestMacro()`: Checks registry first, falls back to name heuristic

### External Macro Fallback

Procedural attribute macros from external crates (`rstest`, `test_case`) use name-based heuristic fallback since their implementations cannot be analyzed without crate resolution.

## Options Considered

### Option A: Two-Phase Detection (Selected)

Combine name-based heuristic with same-file `macro_rules!` definition analysis.

**Pros:**

- High accuracy for common patterns: Covers both naming-convention and definition-based macros
- Maintains single-file constraint: No cross-file resolution needed
- Deterministic: Same-file analysis is fully deterministic
- Reasonable external macro handling: Name heuristic catches `rstest`, `test_case` invocations

**Cons:**

- Definition analysis scope limited: Only works for `macro_rules!` in same file
- Heuristic can miss edge cases: Macros without "test" in name and defined in different file
- Two-pass overhead: Requires two AST traversals

### Option B: Name-Only Heuristic

Detect only macro invocations with "test" in the macro name.

**Pros:**

- Simple implementation: Single-pass, straightforward pattern matching
- Works across all macro types: Name heuristic applies to both declarative and procedural macros

**Cons:**

- Misses definition-based macros: `syntax!`, `matches!` etc. not detected (~264 tests in ripgrep)
- Higher false negative rate: Only catches ~50% of macro-based tests
- Naming convention dependency: Assumes projects follow "test" naming convention

### Option C: Full Macro Expansion via Compiler

Invoke `rustc` or `cargo expand` to get fully expanded source code.

**Pros:**

- 100% accuracy: Compiler expansion handles all macro types correctly
- No heuristics needed: Ground truth from compiler

**Cons:**

- Requires compilation: Must resolve dependencies, download crates, build
- Performance impact: Seconds to minutes per crate vs milliseconds for static parsing
- Environment dependency: Requires Rust toolchain, may fail on incomplete projects
- Violates static analysis principle (ADR-01): Moves from static to dynamic analysis

### Option D: External Crate Resolution

Build dependency graph to resolve `macro_rules!` definitions from external files and crates.

**Pros:**

- Higher coverage than Option A: Catches macros defined in other files within same crate

**Cons:**

- Violates single-file constraint (ADR-14): Requires multi-file coordination
- Complexity explosion: Module resolution, `use` statements, re-exports
- Significant architecture change: Fundamentally different from current file-by-file approach

## Consequences

### Positive

1. **Dramatic accuracy improvement**
   - ripgrep: 436 → ~1,030 detected tests (61% miss → ~7% miss)
   - Covers majority of real-world Rust test patterns

2. **Architecture preserved**
   - Single-file parsing constraint maintained
   - No external dependencies or compilation required
   - Consistent with ADR-14 boundary decisions

3. **Incremental enhancement**
   - Two phases can be enabled/tuned independently
   - Name heuristic provides baseline; definition analysis adds precision

4. **Framework-agnostic**
   - Works with custom project macros (ripgrep's `rgtest!`)
   - Works with external frameworks (`rstest`, `test_case`) via name heuristic

### Negative

1. **Same-file scope limitation**
   - `macro_rules!` definitions in separate files not analyzed
   - Projects with centralized test utilities may have gaps
   - Mitigation: Name heuristic provides fallback for common patterns

2. **Procedural macro opacity**
   - `rstest`, `test_case` expansion logic cannot be analyzed
   - Relies on naming convention for detection
   - Mitigation: These frameworks follow "test" naming convention; false negatives rare

3. **Two-pass performance cost**
   - Additional AST traversal for definition collection
   - Mitigation: Overhead minimal (~10-20% for Rust files)

### Detection Coverage Matrix

| Macro Type           | Same File Definition | Different File | External Crate |
| -------------------- | -------------------- | -------------- | -------------- |
| Name contains "test" | **Detected**         | **Detected**   | **Detected**   |
| Expands to `#[test]` | **Detected**         | Not detected   | Not detected   |
| Neither              | Not detected         | Not detected   | Not detected   |

## References

- [Issue #73: cargo-test: add macro-based test detection for Rust](https://github.com/specvital/core/issues/73)
- [Issue #89: cargotest - detect test macros by analyzing same-file macro_rules! definitions](https://github.com/specvital/core/issues/89)
- [Commit caa4d1b: fix(cargo-test): add macro-based test detection for Rust](https://github.com/specvital/core/commit/caa4d1b)
- [Commit 4f3d697: feat(cargotest): detect test macros by analyzing same-file macro_rules! definitions](https://github.com/specvital/core/commit/4f3d697)
- [ADR-14: Indirect Import Alias Detection Unsupported](/en/adr/core/14-indirect-import-unsupported)
- [ADR-03: Tree-sitter as AST Parsing Engine](/en/adr/core/03-tree-sitter-ast-parsing-engine)
- [Rust Test Documentation](https://doc.rust-lang.org/book/ch11-01-writing-tests.html)
