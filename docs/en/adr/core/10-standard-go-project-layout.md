---
title: Standard Go Project Layout
description: ADR on adopting standard Go project layout for external library consumption
---

# ADR-10: Standard Go Project Layout

> 🇰🇷 [한국어 버전](/ko/adr/core/10-standard-go-project-layout.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-23 | @KubrickCode | core  |

## Context

### Problem Statement

As an independent Go library (see [ADR-01: Core Library Separation](./01-core-library-separation.md)), the core must be consumable by external projects via `go get`. A non-standard directory structure creates friction for consumers and ecosystem tools.

### Technical Challenge

When `go.mod` is nested inside subdirectories (e.g., `src/pkg/go.mod`), external consumers cannot import the module directly. They must use `replace` directives in their own `go.mod`, which:

- Breaks standard `go get` workflow
- Requires manual coordination for dependency updates
- Incompatible with Go module proxy and checksum database
- Confuses ecosystem tools (goimports, gopls, etc.)

## Decision

**Adopt standard Go project layout with root-level `go.mod` and `pkg/` directory for public packages.**

```
specvital/core/
├── go.mod              # Module definition at root
├── go.sum
├── pkg/                # Public packages
│   ├── domain/         # Domain models
│   ├── parser/         # Parser engine
│   ├── source/         # Source abstraction
│   └── crypto/         # Cryptography utilities
└── ...
```

Consumers import packages directly:

```go
import "github.com/specvital/core/pkg/parser"
```

## Options Considered

### Option A: Root go.mod + pkg/ Directory (Selected)

Standard Go project layout with public packages under `pkg/`.

**Pros:**

- Direct import without `replace` directives
- Compatible with Go module proxy and checksum database
- Works seamlessly with ecosystem tools
- Familiar structure for Go developers
- Clear separation of public (`pkg/`) vs internal (`internal/`) packages

**Cons:**

- All packages in `pkg/` are public API
- Requires discipline to maintain API stability
- No compile-time enforcement of internal packages (unless using `internal/`)

### Option B: Nested go.mod (e.g., src/pkg/go.mod)

Module definition in a subdirectory.

**Pros:**

- Allows different directory organization preferences
- Can co-exist with other languages in monorepo

**Cons:**

- Requires `replace` directives for external consumers
- Breaks standard Go tooling workflow
- Incompatible with module proxy caching
- Non-standard and confusing for contributors

### Option C: internal/ Only

All packages under `internal/` directory.

**Pros:**

- Compile-time enforcement: external packages cannot import
- Full freedom to refactor without breaking consumers

**Cons:**

- Contradicts the library's purpose: designed for external consumption
- No public API exposure
- Not suitable for reusable library

## Consequences

### Positive

1. **Frictionless Consumption**
   - External projects use standard `go get github.com/specvital/core`
   - No manual `replace` directives required
   - Dependency updates work via standard tools (Dependabot, Renovate)

2. **Ecosystem Compatibility**
   - Go module proxy caches the module
   - Checksum database provides integrity verification
   - goimports, gopls work correctly

3. **Developer Experience**
   - Standard layout familiar to Go developers
   - Reduces onboarding time for contributors
   - Clear public API surface in `pkg/`

### Negative

1. **API Stability Commitment**
   - All packages under `pkg/` are public API
   - Breaking changes require major version bump
   - **Mitigation**: Use `internal/` for implementation details that should not be exposed

2. **Refactoring Constraints**
   - Cannot freely rename/move packages in `pkg/`
   - **Mitigation**: Conservative API design, extension points over modifications

### Package Visibility

| Directory   | Visibility | Usage                              |
| ----------- | ---------- | ---------------------------------- |
| `pkg/`      | Public     | Exported API for external projects |
| `internal/` | Private    | Implementation details (if needed) |

## References

- [ADR-01: Core Library Separation](./01-core-library-separation.md)
- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
