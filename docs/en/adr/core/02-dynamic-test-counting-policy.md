---
title: Dynamic Test Counting Policy
description: ADR on counting dynamically generated tests as 1 due to static analysis limitations
---

# ADR-02: Dynamic Test Counting Policy

> 🇰🇷 [한국어 버전](/ko/adr/core/02-dynamic-test-counting-policy.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-22 | @KubrickCode | core  |

**Status**: Accepted
**Implementation**: ✅ Phase 1 Complete (2025-12-22)

## Context

SpecVital Core parser uses static AST analysis to count tests. Many test frameworks support dynamic test generation patterns that cannot be accurately counted without runtime execution.

### Discovery

Validation against `github-project-status-viewer` revealed:

- Ground Truth (CLI): 236 tests
- Parser Result: 229 tests
- Delta: -7 (2.97%)

Root cause: Dynamic test patterns not fully supported.

## Decision

### Policy: Count Dynamic Tests as 1

All dynamically generated test patterns will be counted as **1 test** regardless of actual runtime count.

### Rationale

1. **Static analysis limitation**: Cannot evaluate runtime values
2. **Consistency**: Same behavior across all 20 frameworks
3. **Complexity vs value**: Parsing array literals provides marginal benefit
4. **Detection priority**: Detecting test existence > exact count

## Options Considered

### Option A: Count Dynamic Tests as 1 (Selected)

Treat all dynamic patterns uniformly as a single test.

**Pros:**

- Consistent behavior across frameworks
- Simpler implementation
- No false promises about accuracy
- Clear documentation of limitations

**Cons:**

- Parser count may differ from CLI count
- Users need CLI for exact counts

### Option B: Parse Array Literals

Attempt to count array elements in static patterns like `it.each([1,2,3])`.

**Pros:**

- More accurate for simple cases

**Cons:**

- Inconsistent (works for literals, fails for variables)
- Complex implementation
- Marginal accuracy improvement

### Option C: Require Runtime Execution

Execute tests to get exact counts.

**Pros:**

- 100% accuracy

**Cons:**

- Fundamentally changes core's static analysis approach
- Requires test environment setup
- Slow execution
- Security concerns

## Framework Analysis

### Dynamic Test Patterns by Framework

| Framework                 | Dynamic Pattern             | Current Support | Policy                |
| ------------------------- | --------------------------- | --------------- | --------------------- |
| **JavaScript/TypeScript** |                             |                 |                       |
| Jest                      | `it.each([...])`            | Partial         | 1 + `(dynamic cases)` |
| Jest                      | `forEach` + `it`            | ❌ Bug          | 1                     |
| Vitest                    | `it.each([...])`            | Partial         | 1 + `(dynamic cases)` |
| Vitest                    | `forEach` + `it`            | ❌ Bug          | 1                     |
| Mocha                     | `forEach` + `it`            | ❌ Bug          | 1                     |
| Cypress                   | `forEach` + `it`            | ❌ Bug          | 1                     |
| Playwright                | loop + `test`               | ❌              | 1                     |
| **Python**                |                             |                 |                       |
| pytest                    | `@pytest.mark.parametrize`  | ❌              | 1                     |
| unittest                  | `subTest`                   | ❌              | 1                     |
| **Java**                  |                             |                 |                       |
| JUnit5                    | `@ParameterizedTest`        | ❌              | 1                     |
| JUnit5                    | `@RepeatedTest`             | ❌              | 1                     |
| TestNG                    | `@DataProvider`             | ❌              | 1                     |
| **Kotlin**                |                             |                 |                       |
| Kotest                    | `forAll`, data-driven       | ❌              | 1                     |
| **C#**                    |                             |                 |                       |
| NUnit                     | `[TestCase]` multiple       | ✅              | N (attribute count)   |
| NUnit                     | `[TestCaseSource]`          | ❌              | 1                     |
| xUnit                     | `[Theory]` + `[InlineData]` | ✅              | N (attribute count)   |
| xUnit                     | `[MemberData]`              | ❌              | 1                     |
| MSTest                    | `[DataRow]` multiple        | ✅              | N (attribute count)   |
| MSTest                    | `[DynamicData]`             | ❌              | 1                     |
| **Ruby**                  |                             |                 |                       |
| RSpec                     | `shared_examples`           | ❌              | 1                     |
| Minitest                  | loop + `def test_`          | ❌              | 1                     |
| **Go**                    |                             |                 |                       |
| go-testing                | `t.Run` in loop             | ✅              | N (detected subtests) |
| go-testing                | table-driven (variable)     | Partial         | Detected rows only    |
| **Rust**                  |                             |                 |                       |
| cargo-test                | `#[test_case]`              | ❌              | 1                     |
| **C++**                   |                             |                 |                       |
| GoogleTest                | `INSTANTIATE_TEST_SUITE_P`  | ❌              | 1                     |
| **Swift**                 |                             |                 |                       |
| XCTest                    | No native parametrized      | N/A             | -                     |
| **PHP**                   |                             |                 |                       |
| PHPUnit                   | `@dataProvider`             | ❌              | 1                     |

### Legend

- ✅ Supported: Counts actual cases
- Partial: Detects pattern but may not count all cases
- ❌ Not supported: Counts as 1
- ❌ Bug: Should detect but currently doesn't

## Consequences

### Positive

- Consistent behavior across frameworks
- Simpler implementation
- No false promises about accuracy
- Clear documentation of limitations

### Negative

- Parser count may differ from CLI count
- Users need CLI for exact counts

### Neutral

- Ground truth validation must account for dynamic tests

## Implementation

### Phase 1: Bug Fixes (Required)

Fix patterns that should detect tests but currently return 0:

1. **JS/TS**: `forEach`/`map` callback containing `it`/`test`
2. **JS/TS**: `it.each([{...}])` with object array (currently 0, should be 1)

### Phase 2: Enhancement (Optional)

Consider counting attribute-based parametrized tests where count is statically determinable:

- `[TestCase(...)]` × N in C#
- `@pytest.mark.parametrize("x", [1,2,3])` with literal array
