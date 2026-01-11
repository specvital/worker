---
title: Indirect Import Alias Detection Unsupported
description: ADR on not supporting test detection via indirect import chains due to single-file parsing limitations
---

# ADR-14: Indirect Import Alias Detection Unsupported

> 🇰🇷 [한국어 버전](/ko/adr/core/14-indirect-import-unsupported.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-29 | @KubrickCode | core  |

**Status**: Accepted

## Context

### Problem Statement

SpecVital Core parser operates on a **single-file basis** using static AST analysis. This creates a fundamental limitation when projects use indirect import patterns to re-export test utilities.

### Discovery

Validation against `microsoft/playwright` repository revealed:

- Ground Truth (CLI): 4,332 tests
- Parser Result: 3,598 tests
- Delta: -734 (16.9%)

Root cause analysis identified 22 files with 0 tests detected despite containing valid test code. These files used indirect import patterns:

```typescript
// tests/page/browsercontext-add-cookies.spec.ts
import type { Cookie } from "@playwright/test";
import { contextTest as it, expect } from "../config/browserTest";

it("should work @smoke", async ({ context, page, server }) => {
  // test code
});
```

The parser cannot trace that `contextTest` from `../config/browserTest` ultimately re-exports from `@playwright/test`.

### Technical Analysis

```
Direct Import (Supported):
  file.spec.ts → @playwright/test
  ✅ Parser detects "test" function

Indirect Import (Unsupported):
  file.spec.ts → ../config/browserTest → @playwright/test
  ❌ Parser cannot follow import chain
```

## Decision

**Indirect import alias detection is explicitly unsupported.**

The parser will only recognize test functions imported directly from the framework's canonical import path (e.g., `@playwright/test` for Playwright, `@jest/globals` for Jest).

### Rationale

1. **Single-file parsing constraint**: Core's architecture parses files in isolation for performance and simplicity
2. **Multi-file analysis complexity**: Following import chains requires building a dependency graph, fundamentally changing the parsing approach
3. **Heuristics are unreliable**: Alternative approaches (naming convention matching) introduce false positives and framework-specific knowledge into generic parsing logic
4. **Detection still works**: Files are correctly detected as belonging to the framework via config scope; only test extraction fails

## Options Considered

### Option A: Accept Limitation (Selected)

Document that indirect imports are unsupported. Users relying on re-export patterns will see lower test counts.

**Pros:**

- Maintains single-file parsing simplicity
- No framework-specific heuristics in parser
- Clear, documented limitation
- Detection layer still correctly identifies files

**Cons:**

- Some tests not counted in certain codebases
- Parser count may significantly differ from CLI count for projects using re-exports

### Option B: Multi-File Import Resolution

Build dependency graph and follow import chains to resolve aliases.

**Pros:**

- Would correctly handle indirect imports
- 100% accuracy for static imports

**Cons:**

- **Fundamental architecture change**: Requires full project analysis, not file-by-file
- **Performance impact**: Must parse and cache all imported files
- **Complexity explosion**: Circular imports, conditional exports, re-exports
- **Scope creep**: Approaches full TypeScript/JavaScript type resolver

### Option C: Naming Convention Heuristics

Detect Playwright-specific fixture names (`contextTest`, `browserTest`, etc.) from any import source.

**Pros:**

- Works for microsoft/playwright and similar codebases
- No multi-file analysis needed

**Cons:**

- **Parser becomes framework-aware**: Hardcoding fixture names violates separation of concerns
- **Maintenance burden**: New fixture names require parser updates
- **False positives**: Names might collide in other projects
- **Not generalizable**: Different heuristic needed per framework
- **Wrong layer**: This is detection logic, not parsing logic

### Option D: User Configuration

Allow users to specify custom aliases in configuration.

**Pros:**

- Flexible, user-controlled
- No heuristics needed

**Cons:**

- Adds configuration complexity
- Users must understand internal parser behavior
- Easy to misconfigure

## Consequences

### Positive

1. **Architecture integrity**: Single-file parsing model preserved
2. **Clear boundaries**: Parser handles parsing, detection handles framework matching
3. **Documented limitation**: Users understand expected behavior
4. **Maintainability**: No framework-specific knowledge in shared parsing code

### Negative

1. **Accuracy gap**: Projects using re-export patterns will have undercounted tests
2. **microsoft/playwright specifically**: ~17% tests not detected

### Mitigation

1. **Config scope detection**: Files are still correctly identified as belonging to the framework
2. **CLI for accuracy**: Users needing exact counts should use framework's native CLI
3. **Common patterns work**: Direct `@playwright/test` imports (recommended pattern) work correctly

## Framework Impact

| Framework  | Canonical Import   | Re-export Pattern  | Impact                    |
| ---------- | ------------------ | ------------------ | ------------------------- |
| Playwright | `@playwright/test` | Fixture re-exports | microsoft/playwright ~17% |
| Jest       | `@jest/globals`    | Rare               | Minimal                   |
| Vitest     | `vitest`           | Rare               | Minimal                   |
| Mocha      | `mocha`            | Rare               | Minimal                   |
| Cypress    | `cypress`          | Rare               | Minimal                   |

Most frameworks encourage direct imports from canonical paths, making this limitation primarily relevant to projects with custom test infrastructure like microsoft/playwright's internal test utilities.

## Related ADRs

- [ADR-02: Dynamic Test Counting Policy](./02-dynamic-test-counting-policy.md) - Another accuracy limitation
- [ADR-03: Tree-sitter as AST Parsing Engine](./03-tree-sitter-ast-parsing-engine.md) - Single-file parsing foundation
- [ADR-08: Shared Parser Modules](./08-shared-parser-modules.md) - Language-level parsing utilities

## References

- [microsoft/playwright test infrastructure](https://github.com/microsoft/playwright/tree/main/tests/config)
- Validation report: `realworld-test-report.md`
