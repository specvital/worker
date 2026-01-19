---
title: JUnit 4 Framework Separation
description: ADR for separating JUnit 4 as a distinct framework from JUnit 5
---

# ADR-18: JUnit 4 Framework Separation

> 🇰🇷 [한국어 버전](/ko/adr/core/18-junit4-framework-separation.md)

| Date       | Author     | Repos |
| ---------- | ---------- | ----- |
| 2025-12-26 | @specvital | core  |

## Context

### Problem Statement

The JUnit framework parser exhibited a critical detection flaw: JUnit 4 test files using `org.junit.Test` were incorrectly classified as JUnit 5 tests. The existing matcher only checked for the `@Test` annotation presence without distinguishing import packages.

**Quantified Impact:**

- **testcontainers-java**: 250 JUnit 4 tests misclassified as JUnit 5
- **junit5-samples**: Mixed JUnit 4/5 examples incorrectly categorized
- **playwright-java**: JUnit 4 integration tests misattributed

### Why Decision Required

1. **Data Integrity**: Framework attribution affects test statistics and framework adoption metrics
2. **User Trust**: Incorrect framework detection undermines analysis credibility
3. **Enterprise Reality**: Many enterprise Java projects maintain hybrid JUnit 4/5 codebases during multi-year migrations
4. **Semantic Difference**: JUnit 4 and JUnit 5 have fundamentally different architectures

### Detection Patterns

| Version | Import Pattern                  | Key Annotations                                                       |
| ------- | ------------------------------- | --------------------------------------------------------------------- |
| JUnit 4 | `org.junit.Test`, `org.junit.*` | `@Test`, `@Before`, `@After`, `@Ignore`, `@RunWith`                   |
| JUnit 5 | `org.junit.jupiter.api.Test`    | `@Test`, `@ParameterizedTest`, `@Nested`, `@Disabled`, `@DisplayName` |

## Decision

**Adopt a separate `junit4` framework definition alongside `junit5`, with import-based mutual exclusion.**

### Core Principles

1. **Framework Isolation**: JUnit 4 and JUnit 5 are distinct frameworks with separate `Definition` structs following ADR-06 patterns
2. **Import-Based Detection**: Framework version determined by import package, not annotation name
3. **Shared AST Module**: Both frameworks reuse `javaast` utilities per ADR-08
4. **Explicit Mutual Exclusion**: Import patterns designed to be non-overlapping

### Detection Rules

| Version | Import Pattern                                      | Excludes              |
| ------- | --------------------------------------------------- | --------------------- |
| JUnit 4 | `org.junit.Test`, `org.junit.*` (no jupiter)        | `org.junit.jupiter.*` |
| JUnit 5 | `org.junit.jupiter.api.Test`, `org.junit.jupiter.*` | n/a                   |

### Implementation

```go
// junit4/definition.go
var JUnit4ImportPattern = regexp.MustCompile(`import\s+(?:static\s+)?org\.junit\.(?:\*|[A-Z])`)
var JUnit5ImportPattern = regexp.MustCompile(`import\s+(?:static\s+)?org\.junit\.jupiter`)

func (m *JUnit4ContentMatcher) Matches(content []byte) bool {
    // Exclude files with JUnit 5 imports
    if JUnit5ImportPattern.Match(content) {
        return false
    }
    // Require JUnit 4 imports
    return JUnit4ImportPattern.Match(content)
}
```

## Options Considered

### Option A: Separate Framework Strategy (Selected)

Create independent `junit4` and `junit5` framework definitions, each with its own matchers, parsers, and registration.

**Pros:**

- Clean separation with clear ownership per framework
- ADR-06 compliant (unified definition pattern)
- Independent evolution (JUnit 4 Rules vs JUnit 5 Extensions)
- Accurate framework adoption statistics
- Each framework tested in isolation

**Cons:**

- Two definition files instead of one
- Slight code duplication for common annotation handling
- Registry contains two Java test framework entries

### Option B: Single Parser with Version Detection

One `junit` framework that internally detects and reports version.

**Pros:**

- Single framework registration
- Unified JUnit handling

**Cons:**

- Violates ADR-06 (framework identity becomes runtime-determined)
- Complex branching for two different annotation sets
- Statistics ambiguity ("junit" loses version granularity)

### Option C: Import-Based Routing in Unified Parser

Single framework definition that routes to version-specific sub-parsers based on imports.

**Pros:**

- Single definition point
- Internal routing preserves separation

**Cons:**

- Hidden complexity (external view is one framework, internal is two)
- Matcher mismatch (definition must accept both versions)
- Statistics still reported as single "junit" framework

### Option D: Annotation-Only Detection (Ignore Imports)

Detect framework purely by annotation names without considering imports.

**Pros:**

- Simplest implementation
- No import parsing needed

**Cons:**

- Root cause of the bug (current broken approach)
- Cannot distinguish versions (`@Test` exists in both)
- False positives from other frameworks using `@Test`

## Consequences

### Positive

1. **Accurate Framework Attribution**
   - JUnit 4 tests correctly identified and reported
   - testcontainers-java: 250 tests now correctly attributed
   - Framework adoption metrics reflect actual codebase state

2. **Enterprise Codebase Support**
   - Hybrid JUnit 4/5 projects analyzed correctly
   - Migration progress trackable (JUnit 4 count decreasing over time)

3. **Architecture Alignment**
   - Follows ADR-06 unified definition pattern
   - Reuses ADR-08 `javaast` shared module
   - Consistent with existing framework separation (Jest/Vitest)

4. **Clear Ownership**
   - JUnit 4-specific handling (`@RunWith`, `@Rule`) isolated
   - JUnit 5-specific handling (`@Nested`, `@ParameterizedTest`) isolated

5. **Nested Class Detection**
   - Fix for nested static classes (testcontainers-java pattern)
   - Recursive AST traversal properly handles inner test classes

### Negative

1. **Increased Framework Count**
   - Registry now has two Java unit test frameworks
   - **Mitigation**: Add framework family grouping if needed for simplified views

2. **Slight Code Duplication**
   - Common annotation extraction logic (`@Test` parsing)
   - **Mitigation**: Extract to `javaast` shared module per ADR-08

3. **Edge Case: Both Imports Present**
   - File importing both `org.junit.Test` and `org.junit.jupiter.api.Test`
   - **Resolution**: JUnit 5 takes precedence (more specific import wins)

## References

- [Commit 7b96c63](https://github.com/specvital/core/commit/7b96c63): feat(junit4): add JUnit 4 framework support
- [Commit 02aaed1](https://github.com/specvital/core/commit/02aaed1): fix(junit5): exclude JUnit4 test files from JUnit5 detection
- [Commit 5673d83](https://github.com/specvital/core/commit/5673d83): fix(junit4): detect tests inside nested static classes
- [Issue #67](https://github.com/specvital/core/issues/67): add JUnit 4 framework support
- [ADR-06: Unified Framework Definition](/en/adr/core/06-unified-framework-definition.md)
- [ADR-08: Shared Parser Modules](/en/adr/core/08-shared-parser-modules.md)
