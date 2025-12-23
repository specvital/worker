---
title: Config Scope Resolution
description: ADR on hierarchical config file resolution for monorepo support
---

# ADR-09: Config Scope Resolution

> :kr: [한국어 버전](/ko/adr/core/09-config-scope-resolution.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-23 | @KubrickCode | core  |

**Status**: Accepted

## Context

### Problem Statement

Modern codebases frequently use monorepo structures with multiple test framework configurations:

```
monorepo/
├── jest.config.js           # Root-level Jest config
├── packages/
│   ├── web/
│   │   └── vitest.config.ts # Vitest for web package
│   └── api/
│       └── jest.config.ts   # Jest for api package
└── e2e/
    └── playwright.config.ts # Playwright for E2E tests
```

When detecting which framework a test file belongs to, the parser must:

1. **Handle multiple overlapping scopes**: A file in `packages/web/` could match both root Jest and local Vitest configs
2. **Respect hierarchy**: Nested (more specific) configs should take precedence over parent configs
3. **Ensure determinism**: Same file must always resolve to same config across runs
4. **Support framework-specific config features**: Jest's `roots`, Vitest's `root`, include/exclude patterns

### Strategic Question

How should the parser resolve which config file governs a given test file when multiple configs could apply?

## Decision

**Use depth-based resolution with deterministic tie-breaking: deeper (more specific) configs win, with lexicographic ordering as final tie-breaker.**

### Resolution Algorithm

```
For a test file at path P:
1. Filter configs by language compatibility
2. Find all configs whose scope contains P
3. Select by depth (deeper = more specific)
4. Tie-breaker 1: Longer config path
5. Tie-breaker 2: Lexicographic order (deterministic)
```

### ConfigScope Structure

```go
type ConfigScope struct {
    ConfigPath      string              // Path to config file
    BaseDir         string              // Effective root directory
    Include         []string            // Glob patterns for inclusion
    Exclude         []string            // Glob patterns for exclusion
    Roots           []string            // Multiple root directories (Jest)
    Framework       string              // Framework name
    GlobalsMode     bool                // Whether globals are available
}
```

## Options Considered

### Option A: Depth-Based Resolution (Selected)

Deeper config files take precedence over shallower ones.

**Pros:**

- **Intuitive behavior**: More specific config naturally wins
- **Monorepo-friendly**: Package-level configs override workspace root
- **Deterministic**: Clear hierarchy with tie-breakers

**Cons:**

- Config path structure affects precedence
- Deeply nested configs always win regardless of explicit intent

### Option B: Explicit Priority in Config Files

Framework configs declare explicit priority values.

**Pros:**

- Full control over resolution order
- Can override depth-based defaults

**Cons:**

- **Requires config modification**: Users must add priority fields
- **Non-standard**: Not part of native framework configs
- **Maintenance burden**: Priority values need coordination

### Option C: First-Match Resolution

Use first config discovered during filesystem walk.

**Pros:**

- Simple implementation
- Fast (stops at first match)

**Cons:**

- **Non-deterministic**: Walk order varies by filesystem
- **Unpredictable**: Results depend on discovery order

## Implementation Details

### Contains Check

The `ConfigScope.Contains()` method determines if a file is within scope:

```go
func (s *ConfigScope) Contains(filePath string) bool {
    roots := s.effectiveRoots()
    for _, root := range roots {
        relPath := computeRelativePath(root, filePath)
        if isOutsideRoot(relPath) {
            continue
        }
        if !matchesIncludePatterns(relPath, s.Include) {
            continue
        }
        if matchesExcludePatterns(relPath, s.Exclude) {
            continue
        }
        return true
    }
    return false
}
```

### Depth Calculation

Depth is calculated from BaseDir path structure:

```go
func (s *ConfigScope) Depth() int {
    return strings.Count(filepath.ToSlash(s.BaseDir), "/")
}
```

| Config Path                         | BaseDir            | Depth |
| ----------------------------------- | ------------------ | ----- |
| `jest.config.js`                    | `.`                | 0     |
| `packages/web/vitest.config.ts`     | `packages/web`     | 1     |
| `packages/web/src/vitest.config.ts` | `packages/web/src` | 2     |

### Multi-Root Support

Jest's `roots` config allows multiple root directories:

```javascript
// jest.config.js
module.exports = {
  roots: ["<rootDir>/packages/next/src", "<rootDir>/packages/font/src"],
};
```

The parser resolves these relative to the config directory and checks file containment against all roots.

### Deterministic Selection

When multiple configs match with equal depth:

```go
// Tie-breaker 1: prefer longer config path (more specific)
if len(m.path) > len(best.path) {
    best = m
}
// Tie-breaker 2: lexicographic order for determinism
if m.path < best.path {
    best = m
}
```

This ensures consistent behavior across:

- Multiple CI runs
- Different filesystem implementations
- Map iteration order variance

## Consequences

### Positive

1. **Monorepo Support**
   - Package-specific configs naturally take precedence
   - Works with any nesting depth
   - No special configuration required

2. **Deterministic Results**
   - Same file always maps to same config
   - Consistent across CI environments
   - Reproducible detection results

3. **Framework Compatibility**
   - Respects native config semantics (Jest roots, Vitest root)
   - Supports include/exclude patterns
   - Handles globals mode detection

4. **Zero Configuration**
   - Works with standard framework config conventions
   - No additional metadata required
   - Drop-in support for existing projects

### Negative

1. **Implicit Precedence**
   - Config hierarchy determined by path structure
   - **Mitigation**: Document resolution order; depth-based is intuitive

2. **No Override Mechanism**
   - Cannot force shallow config to win over deep config
   - **Mitigation**: Restructure config files if explicit override needed

3. **Performance Cost**
   - Must check all matching configs before selecting
   - **Mitigation**: Config count typically small; linear search acceptable

### Design Principles

- **Proximity wins**: Config closer to the test file is more relevant
- **Convention over configuration**: Standard layouts work without extra setup
- **Predictable behavior**: Same input always produces same output
- **Language-aware**: Only consider configs compatible with file's language

## Related ADRs

- [ADR-04: Early-Return Framework Detection](./04-early-return-framework-detection.md) - Uses scope resolution in detection hierarchy
- [ADR-06: Unified Framework Definition](./06-unified-framework-definition.md) - ConfigParser interface for scope creation

## References

- `pkg/parser/framework/scope.go` - ConfigScope implementation
- `pkg/parser/detection/detector.go` - detectFromScope resolution logic
- `pkg/parser/strategies/jest/definition.go` - Jest config parsing with roots support
- `pkg/parser/strategies/vitest/definition.go` - Vitest config parsing with root support
