---
title: Vitest 4.0+ test.for/it.for API Support
description: ADR for extending the jstest shared module to support Vitest's test.for/it.for parameterized test API
---

# ADR-19: Vitest 4.0+ test.for/it.for API Support

> 🇰🇷 [한국어 버전](/ko/adr/core/19-vitest-4-api-support)

| Date       | Author     | Repos |
| ---------- | ---------- | ----- |
| 2026-01-03 | @specvital | core  |

## Context

### Problem Statement

SpecVital Core parser's static AST analysis did not recognize Vitest's `test.for`/`it.for` API, causing significant accuracy degradation when parsing modern Vitest codebases.

### Discovery

Validation against `vitejs/vite` v7.3.0 revealed:

| Metric             | Value         |
| ------------------ | ------------- |
| Ground Truth (CLI) | 703 tests     |
| Parser Result      | 583 tests     |
| Delta              | -120 (-17.1%) |

Root cause: 120 tests using `test.for` API were undetected.

### Background

Vitest 2.0 introduced `test.for`/`it.for` as an alternative to `test.each` with key differences:

| Aspect               | `test.each`    | `test.for`             |
| -------------------- | -------------- | ---------------------- |
| Argument spreading   | Spreads arrays | No spreading           |
| TestContext access   | Not available  | Available as 2nd param |
| Concurrent snapshots | Not supported  | Supported              |
| Jest compatibility   | Yes            | Vitest-only            |

**Syntax difference:**

```typescript
// test.each spreads array arguments
test.each([
  [1, 1, 2],
  [1, 2, 3],
])("add(%i, %i) -> %i", (a, b, expected) => {
  expect(a + b).toBe(expected);
});

// test.for does NOT spread - requires destructuring
test.for([
  [1, 1, 2],
  [1, 2, 3],
])("add(%i, %i) -> %i", ([a, b, expected]) => {
  expect(a + b).toBe(expected);
});
```

The `.for` API addresses `test.each` limitations around TestContext and fixtures, enabling concurrent snapshot testing:

```typescript
test.concurrent.for([
  [1, 1],
  [1, 2],
])("add(%i, %i)", ([a, b], { expect }) => {
  expect(a + b).matchSnapshot();
});
```

### Requirements

1. Detect `test.for`/`it.for`/`describe.for` patterns in Vitest files
2. Apply same counting policy as `.each()` (ADR-02)
3. Support chained modifiers: `test.concurrent.for`, `test.skip.for`, `test.only.for`
4. Minimize changes to shared `jstest` module (ADR-08)

## Decision

**Extend existing `.each()` infrastructure to support `.for` modifier.**

The `jstest` shared module's parameterized test handling is extended to recognize `.for` as an additional modifier alongside `.each`. This applies the established Dynamic Test Counting Policy (ADR-02): parameterized tests count as 1 regardless of runtime iterations.

### Implementation

Minimal changes to three files in `pkg/parser/strategies/shared/jstest/`:

```go
// constants.go
const (
    ModifierFor  = "for"    // New
    ModifierEach = "each"
    // ... existing modifiers
)

// helpers.go - Include .for in modifier detection
func ParseSimpleMemberExpression(node *sitter.Node, source []byte) string {
    // Returns "test.for", "describe.for", etc.
}

// parser.go - Route .for through existing processing
switch funcName {
case FuncDescribe + "." + ModifierEach, FuncDescribe + "." + ModifierFor:
    ProcessEachSuites(...)
case FuncIt + "." + ModifierEach, FuncIt + "." + ModifierFor:
    ProcessEachTests(...)
}
```

### Supported Patterns

| Pattern                                  | Description                   |
| ---------------------------------------- | ----------------------------- |
| `test.for([...])('name', cb)`            | Basic parameterized test      |
| `it.for([...])('name', cb)`              | Alias for test.for            |
| `describe.for([...])('name', cb)`        | Parameterized suite           |
| `test.concurrent.for([...])('name', cb)` | Concurrent parameterized test |
| `test.skip.for([...])('name', cb)`       | Skipped parameterized test    |
| `test.only.for([...])('name', cb)`       | Focused parameterized test    |

### Counting Policy

Per ADR-02, all parameterized test patterns count as 1:

| Pattern                      | Parser Count | Rationale                                     |
| ---------------------------- | ------------ | --------------------------------------------- |
| `test.for([a,b,c])`          | 1            | Static analysis cannot evaluate runtime count |
| `test.each([a,b,c])`         | 1            | Same policy for consistency                   |
| `test.concurrent.for([...])` | 1            | Modifier chain does not change policy         |

## Options Considered

### Option A: Extend Existing .each() Infrastructure (Selected)

Add `.for` as an additional modifier in the existing parameterized test handling.

**Pros:**

- Minimal code changes (3 files)
- Leverages battle-tested `.each()` infrastructure
- Consistent with Dynamic Test Counting Policy (ADR-02)
- All JavaScript frameworks benefit via shared module (ADR-08)
- Single code path for parameterized tests

**Cons:**

- Ignores semantic differences between `.for` and `.each`
- Future `.for`-specific features may require divergence

### Option B: Create Separate test.for Parser

Implement independent parsing logic for `test.for`/`it.for` patterns.

**Pros:**

- Clean separation of concerns
- No risk of `.each()` regression
- Can implement `.for`-specific optimizations

**Cons:**

- ~70% code duplication with `.each()` handling
- Violates DRY principle and Shared Parser Modules pattern (ADR-08)
- Bug fixes must be applied separately to each path
- Maintenance burden increase

### Option C: Different Counting Policy for test.for

Attempt to count actual iterations for `.for` patterns by parsing array arguments.

**Pros:**

- More accurate counts for simple literal cases
- Better alignment with user expectations

**Cons:**

- Violates established Dynamic Test Counting Policy (ADR-02)
- Inconsistent behavior between similar APIs
- Cannot handle variable references (same limitation)
- User confusion from different counting rules

### Option D: Runtime Detection via Vitest API

Use Vitest's test collection API for accurate runtime counts.

**Pros:**

- 100% accuracy for all patterns
- No static analysis limitations

**Cons:**

- Fundamentally changes core's static-only architecture
- Requires test environment setup
- Security implications from code execution
- Performance impact
- Violates core architectural principle (ADR-03)

## Consequences

### Positive

1. **Accuracy Restoration**
   - vitejs/vite detection accuracy returns to acceptable range
   - Modern Vitest codebases correctly parsed

2. **Consistency**
   - Same counting behavior for `.each()` and `.for()` patterns
   - Users experience predictable behavior across parameterized APIs

3. **Shared Module Benefit**
   - All `jstest` consumers (Jest, Vitest, Mocha, Cypress, Playwright) gain capability
   - Prepared for potential `.for` adoption by other frameworks

4. **Maintainability**
   - Single code path for all parameterized test handling
   - Bug fixes in shared logic benefit all patterns

### Negative

1. **Semantic Simplification**
   - Argument spreading differences between `.for` and `.each` are ignored
   - Mitigation: Counting policy treats both as 1, making semantic difference irrelevant for count accuracy

2. **Potential Future Divergence**
   - If `.for` gains features requiring different handling, refactoring may be needed
   - Mitigation: Current approach does not preclude future separation into Option B if warranted

## References

- [Issue #96: vitest - add test.for/it.for API support](https://github.com/specvital/core/issues/96)
- [Commit 5c7c8fa: feat(vitest): add test.for/it.for API support](https://github.com/specvital/core/commit/5c7c8fa)
- [ADR-02: Dynamic Test Counting Policy](/en/adr/core/02-dynamic-test-counting-policy)
- [ADR-08: Shared Parser Modules](/en/adr/core/08-shared-parser-modules)
- [Vitest test.for Documentation](https://vitest.dev/api/)
