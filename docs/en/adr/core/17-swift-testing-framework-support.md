---
title: Swift Testing Framework Support
description: "ADR for Apple Swift Testing framework support (@Test, @Suite attributes)"
---

# ADR-17: Swift Testing Framework Support

> 🇰🇷 [한국어 버전](/ko/adr/core/17-swift-testing-framework-support.md)

| Date       | Author     | Repos |
| ---------- | ---------- | ----- |
| 2026-01-04 | @specvital | core  |

## Context

### Problem Statement

During validation of major Swift repositories (e.g., Alamofire with 40K+ GitHub stars), Swift Testing tests were not being detected. The existing XCTest parser only recognizes:

- `func testXxx()` naming convention
- Classes extending `XCTestCase`

Apple introduced Swift Testing at WWDC 2024 (Swift 6 / Xcode 16) with fundamentally different patterns:

| Pattern          | XCTest                | Swift Testing                           |
| ---------------- | --------------------- | --------------------------------------- |
| Test declaration | `func testXxx()`      | `@Test` attribute                       |
| Suite grouping   | `XCTestCase` subclass | `@Suite` (optional, implicit for @Test) |
| Skip mechanism   | `XCTSkip()` runtime   | `@Test(.disabled)` compile-time trait   |
| Assertions       | `XCTAssert*`          | `#expect()`, `#require()`               |
| Type support     | Classes only          | Struct, Actor, Class                    |

### Impact

- 57 tests undetected in Alamofire repository
- Growing adoption of Swift Testing in iOS/macOS projects
- Apple's official recommendation for new Swift tests

### Requirements

1. Detect Swift Testing tests via `@Test` and `@Suite` attributes
2. Recognize skip status from `@Test(.disabled)` trait
3. Support async test functions
4. Share AST utilities with XCTest via `swiftast` module
5. Maintain backward compatibility with existing XCTest detection

## Decision

**Implement Swift Testing as a separate framework with `PrioritySpecialized` detection.**

The `swifttesting` framework is registered as an independent definition alongside `xctest`, sharing the `swiftast` module for common Swift AST utilities.

### Detection Strategy

Priority-based early-return (per ADR-04):

1. **Import detection** (highest): `import Testing` triggers Swift Testing parser
2. **Attribute detection**: `@Test`, `@Suite` presence confirms framework
3. **Content patterns**: `#expect()`, `#require()` as supporting signals

### Parser Implementation

```go
func NewDefinition() *framework.Definition {
    return &framework.Definition{
        Name:      "swifttesting",
        Languages: []domain.Language{domain.LanguageSwift},
        Matchers: []framework.Matcher{
            matchers.NewImportMatcher("Testing"),
            &SwiftTestingContentMatcher{}, // @Test, @Suite, #expect
        },
        Parser:   &SwiftTestingParser{},
        Priority: framework.PrioritySpecialized, // 200
    }
}
```

### Skip Detection

`@Test(.disabled)` trait maps to `TestStatusSkipped`:

```go
// "@Test(.disabled)" or "@Test(.disabled(\"reason\"))"
if hasDisabledTrait(annotation) {
    return domain.TestStatusSkipped
}
```

### Async Support

Content scan for `async` keyword in function signature:

```swift
@Test func fetchData() async throws { ... }
```

## Options Considered

### Option A: Separate Framework Strategy (Selected)

Create distinct `swifttesting` framework definition with `PrioritySpecialized`.

**Pros:**

- Framework isolation enables independent evolution
- Clean detection via `import Testing`
- Native support for Swift Testing traits (`@Test(.disabled)`)
- Shares `swiftast` module for code reuse
- Follows Unified Framework Definition pattern (ADR-06)

**Cons:**

- Two Swift framework definitions to maintain
- Potential detection overlap in mixed files

### Option B: Extend Existing XCTest Parser

Add Swift Testing patterns within XCTest definition.

**Pros:**

- Single Swift framework definition
- Shared maintenance scope

**Cons:**

- Violates single-responsibility principle
- Complex internal branching for fundamentally different patterns
- Bug fixes could cascade across both frameworks
- Apple explicitly positions these as distinct frameworks

### Option C: Universal Swift Parser

Single Swift parser with sub-framework routing.

**Pros:**

- Maximum code sharing
- Single entry point for Swift

**Cons:**

- Over-generalization risk
- Complex internal routing
- Framework-specific edge cases leak across boundaries

### Option D: Pattern-Based Detection Only

Regex-based detection without AST parsing.

**Pros:**

- Lightweight implementation
- Fast execution

**Cons:**

- Cannot extract parameterized test names
- Cannot detect async functions properly
- Cannot parse nested suites
- Insufficient for production accuracy requirements

## Consequences

### Positive

1. **Framework Isolation**
   - Swift Testing evolves independently from XCTest
   - No regression risk when updating one parser
   - Clear ownership and responsibility per framework

2. **Accurate Detection**
   - `import Testing` provides highest-reliability detection signal
   - `@Test(.disabled)` natively maps to skip status
   - Async function detection via content scan

3. **Code Reuse via Shared Modules**
   - `swiftast` module provides common AST utilities
   - Bug fixes in `swiftast` benefit both parsers
   - Follows Shared Parser Modules pattern (ADR-08)

4. **Future-Proof Architecture**
   - Extensible to support additional traits (`@Test(.bug())`, `@Test(.tags())`)
   - Ready for parameterized tests (`@Test(arguments:)`)
   - Aligned with Apple's framework direction

5. **Consistent with Existing ADRs**
   - Unified Framework Definition (ADR-06)
   - Early-Return Framework Detection (ADR-04)
   - Shared Parser Modules (ADR-08)

### Negative

1. **Dual Framework Maintenance**
   - Two separate definition files for Swift
   - **Mitigation**: Shared `swiftast` module minimizes duplication

2. **Mixed File Detection**
   - Files using both XCTest and Swift Testing (Apple-supported scenario)
   - **Mitigation**: `PrioritySpecialized` ensures Swift Testing detected first; explicit import wins

3. **Initial Development Investment**
   - New definition.go, matchers, and parser implementation
   - **Mitigation**: Leverage existing `swiftast` module; follow established patterns

## References

- [Commit 161b650](https://github.com/specvital/core/commit/161b650): feat(swift-testing): add Apple Swift Testing framework support
- [Issue #95](https://github.com/specvital/core/issues/95): Add Apple Swift Testing framework support
- [ADR-04: Early-Return Framework Detection](/en/adr/core/04-early-return-framework-detection.md)
- [ADR-06: Unified Framework Definition](/en/adr/core/06-unified-framework-definition.md)
- [ADR-08: Shared Parser Modules](/en/adr/core/08-shared-parser-modules.md)
- [Swift Testing - Apple Developer](https://developer.apple.com/xcode/swift-testing)
- [Meet Swift Testing - WWDC24](https://developer.apple.com/videos/play/wwdc2024/10179/)
