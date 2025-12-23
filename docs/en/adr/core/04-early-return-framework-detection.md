---
title: Early-Return Framework Detection
description: ADR on using priority-based early-return for test framework detection
---

# ADR-04: Early-Return Framework Detection

> 🇰🇷 [한국어 버전](/ko/adr/core/04-early-return-framework-detection.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-23 | @KubrickCode | core  |

**Status**: Accepted

## Context

### Problem Statement

SpecVital Core detects which test framework a test file belongs to across multiple languages and frameworks. This detection must handle:

1. **Similar frameworks sharing patterns**: Jest and Vitest both use `describe`/`it` syntax
2. **Multiple valid signals per file**: Import statements, config file presence, content patterns
3. **Deterministic requirement**: Same input must always produce same output
4. **Monorepo complexity**: Nested config files with different scopes

### Detection Signals

Test files can be identified through several signals:

| Signal       | Example                            | Reliability |
| ------------ | ---------------------------------- | ----------- |
| Import       | `import { test } from 'vitest'`    | Highest     |
| Config Scope | File within `jest.config.js` scope | High        |
| Content      | `jest.fn()`, `vi.mock()` patterns  | Medium      |
| Filename     | `*.test.ts`, `*_test.go`           | Low         |

### Strategic Question

How should the detector combine multiple signals to produce a single framework result?

## Decision

**Use priority-based early-return: first match wins based on signal reliability.**

Detection follows a strict priority order:

1. **Import** → Return immediately if framework-specific import found
2. **Config Scope** → Return immediately if file is within a config's scope
3. **Content Pattern** → Return immediately if framework-specific pattern found
4. **Unknown** → Return if no signals matched

The first successful match at any priority level immediately returns without checking lower priorities.

### Detection Source Tracking

Each result includes how the framework was detected:

```go
type DetectionSource string

const (
    SourceImport         DetectionSource = "import"
    SourceConfigScope    DetectionSource = "config-scope"
    SourceContentPattern DetectionSource = "content-pattern"
    SourceUnknown        DetectionSource = "unknown"
)
```

## Options Considered

### Option A: Priority-Based Early-Return (Selected)

First match wins based on signal reliability hierarchy.

**Pros:**

- **Fast execution**: Stops at first match, no unnecessary processing
- **Predictable behavior**: Same input always produces same output
- **Easy debugging**: Clear "which signal matched" tracking
- **Simple implementation**: No complex scoring logic

**Cons:**

- **Lower-priority signals ignored**: Even if stronger, later signals are not evaluated
- **Import extraction sensitivity**: Incorrect import parsing leads to wrong results

### Option B: Score Accumulation

Assign confidence points to each signal, sum them, return highest-scoring framework.

**Example scoring:**
| Signal | Points |
|--------|--------|
| Scope | 80 |
| Import | 60 |
| Content | 40 |
| Filename | 20 |

**Pros:**

- Multiple signals can reinforce each other
- Stronger overall signal could override weaker early match

**Cons:**

- **Debugging difficulty**: "Why did this framework win?" requires analyzing all scores
- **Tuning complexity**: Point values are arbitrary and hard to calibrate
- **Non-determinism risk**: Score ties require additional tie-breaking rules
- **Performance overhead**: Must evaluate all signals before deciding

### Option C: Hybrid Approach

Score accumulation with early-exit threshold (e.g., return immediately if score exceeds 100).

**Pros:**

- Balances speed and signal combination

**Cons:**

- Inherits complexity of scoring
- Threshold value is arbitrary
- Still requires full scoring logic

## Implementation Details

### Config Scope Resolution

When multiple config files could apply to a file, the detector selects the most specific:

1. **Depth-based selection**: Deeper config paths take precedence (more specific)
2. **Tie-breaker 1**: Longer config path (more specific path)
3. **Tie-breaker 2**: Lexicographic order (deterministic)

```
project/
├── jest.config.js          # depth 0
├── packages/
│   └── web/
│       └── jest.config.js  # depth 2 (wins for files in packages/web/)
```

### Language-Specific Handling

Go test files use naming convention (`*_test.go`) rather than import detection, handled as a special case before the general detection flow.

### Negative Matching

Frameworks can declare "definitely not this framework" signals:

```go
type MatchResult struct {
    Confidence int
    Evidence   []string
    Negative   bool  // If true, exclude this framework
}
```

This prevents false positives when similar frameworks share patterns.

## Consequences

### Positive

1. **Performance**
   - Fast execution: exits at first match
   - No wasted computation on lower-priority signals
   - O(1) best case, O(frameworks × signals) worst case

2. **Maintainability**
   - Simple control flow (priority checks in sequence)
   - Clear responsibility per detection stage
   - Easy to add new frameworks or signals

3. **Debuggability**
   - Result includes detection source
   - Easy to trace "why this framework"
   - No complex score calculation to audit

4. **Determinism**
   - Same file always produces same result
   - Config scope tie-breaking is fully deterministic
   - No random or timing-dependent behavior

### Negative

1. **Signal Hierarchy is Fixed**
   - Cannot adjust priority order per-project
   - **Mitigation**: Priority order based on real-world reliability analysis

2. **Import Parsing Sensitivity**
   - Incorrect import extraction causes wrong results
   - **Mitigation**: Language-specific extractors with comprehensive test coverage

3. **Later Signals Ignored**
   - Content patterns not checked if import matched
   - **Mitigation**: Import is most reliable; ignoring later signals is intentional

### Design Principles

- **Explicit developer intent wins**: Import statements represent conscious framework choice
- **Project configuration is authoritative**: Config files define project-level decisions
- **Content patterns are fallback**: Used only when explicit signals absent
- **Filename patterns are unreliable**: Removed from detection (too many false positives)

## Related ADRs

- [ADR-03: Tree-sitter as AST Parsing Engine](./03-tree-sitter-ast-parsing-engine.md) - Parsing infrastructure used for import extraction

## References

- `pkg/parser/detection/detector.go` - Core detection implementation
- `pkg/parser/detection/result.go` - Result types with DetectionSource
- `pkg/parser/framework/scope.go` - Config scope resolution logic
