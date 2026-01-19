---
title: React Compiler Adoption
description: ADR for enabling React Compiler and removing manual memoization
---

# ADR-22: React Compiler Adoption

> [Korean Version](/ko/adr/web/22-react-compiler-adoption.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2026-01-19 | @KubrickCode | web   |

## Context

### The Manual Memoization Burden

The web application accumulated significant manual memoization overhead across the codebase:

- **27 files** containing explicit `useMemo` and `useCallback` calls
- Cognitive load on developers to determine when memoization is necessary
- Risk of over-memoization (unnecessary complexity) or under-memoization (performance issues)
- Maintenance burden when refactoring components with memoization dependencies

### React Compiler Opportunity

React 19, adopted via [ADR-02: Next.js 16 + React 19 Selection](/en/adr/web/02-nextjs-react-selection.md), includes the React Compiler (formerly React Forget). The compiler automatically applies memoization optimizations at build time, eliminating the need for manual `useMemo`, `useCallback`, and `React.memo` in most cases.

**ADR-02 Performance Projection:** "React Compiler automatic memoization (25-40% fewer re-renders)"

### Post-Deployment Discovery

After enabling React Compiler, two components using TanStack Virtual exhibited broken behavior:

- `test-list.tsx` - virtualized test result list
- `tree-view.tsx` - virtualized file tree navigation

**Root Cause:** React Compiler's aggressive memoization cached `virtualizer.getVirtualItems()` results. Since the virtualizer reference never changes, the virtual items were never updated, causing empty or stale lists.

This is a known compatibility issue between React Compiler and TanStack Virtual's ref-based measurement pattern using `measureElement` and `ResizeObserver`.

## Decision

**Adopt React Compiler globally with `"use no memo"` escape hatch for incompatible third-party library patterns.**

### Core Principles

1. **Compiler-First**: Enable React Compiler for all components by default
2. **Explicit Opt-Out**: Use `"use no memo"` directive only for documented incompatibilities
3. **Remove Manual Memoization**: Delete `useMemo`/`useCallback` from compiler-optimized components
4. **Document Exceptions**: Maintain a list of opt-out components with rationale

### Configuration

```typescript
// next.config.ts
const nextConfig = {
  reactCompiler: true,
  // ...
};
```

### Dependency

```json
{
  "devDependencies": {
    "babel-plugin-react-compiler": "1.0.0"
  }
}
```

### Escape Hatch Usage

```tsx
"use no memo";

export function VirtualizedList() {
  // Excluded from React Compiler optimization
  const virtualizer = useVirtualizer({ ... });
  return ...;
}
```

## Options Considered

### Option A: Full React Compiler Adoption with Escape Hatch (Selected)

Enable `reactCompiler: true` globally, remove manual memoization, apply `"use no memo"` for incompatibilities.

| Pros                                      | Cons                                      |
| ----------------------------------------- | ----------------------------------------- |
| Code simplification (27 files cleaned)    | Library compatibility overhead            |
| Consistent optimization strategy          | Debugging requires compiler understanding |
| Future-proof, aligns with React direction | Risk of escape hatch proliferation        |
| No developer memoization decisions        |                                           |

### Option B: Continue Manual Memoization

Do not enable React Compiler, maintain existing patterns.

| Pros                   | Cons                                 |
| ---------------------- | ------------------------------------ |
| No compatibility risks | Ongoing cognitive burden             |
| Familiar pattern       | Inconsistent application             |
|                        | Counter to React ecosystem direction |

**Rejected**: ADR-02 explicitly adopted React 19 for compiler benefits.

### Option C: Selective/Gradual Adoption

Enable React Compiler in opt-in mode with `"use memo"` directive.

| Pros                              | Cons                                             |
| --------------------------------- | ------------------------------------------------ |
| Safer rollout                     | Doubles effort (manual decisions still required) |
| Component-by-component validation | Partial benefits, inconsistent state             |

**Rejected**: Escape hatch pattern achieves safer rollout with less overhead.

## Consequences

### Positive

| Area                    | Benefit                                                  |
| ----------------------- | -------------------------------------------------------- |
| Code Simplification     | Removed manual memoization from 27 files                 |
| Performance Consistency | Compiler applies optimal memoization via static analysis |
| Developer Experience    | No mental overhead deciding when to memoize              |
| Ecosystem Alignment     | Positions codebase for future React optimizations        |

### Negative

| Area                    | Trade-off                                 | Mitigation                             |
| ----------------------- | ----------------------------------------- | -------------------------------------- |
| Library Compatibility   | TanStack Virtual requires `"use no memo"` | Documented in component files          |
| Escape Hatch Governance | Risk of directive proliferation           | Code review policy requiring rationale |
| Debugging Complexity    | Understanding compiler transformation     | React DevTools Compiler badge          |

### Affected Components

| Component       | Issue                        | Solution        | Status                       |
| --------------- | ---------------------------- | --------------- | ---------------------------- |
| `test-list.tsx` | TanStack Virtual ref caching | `"use no memo"` | Temporary until TanStack fix |
| `tree-view.tsx` | TanStack Virtual ref caching | `"use no memo"` | Temporary until TanStack fix |

### Coding Guidelines

Added to `CLAUDE.md`:

```markdown
## React Compiler

React Compiler is enabled (`next.config.ts`: `reactCompiler: true`)

### Prohibited

- **NEVER** use `useMemo`, `useCallback`, `React.memo`
- React Compiler handles memoization automatically at build time

### Escape Hatch

Use `"use no memo"` directive only when compiler causes issues
```

## References

- [ADR-02: Next.js 16 + React 19 Selection](/en/adr/web/02-nextjs-react-selection.md)
- [Commit 482d080e](https://github.com/specvital/web/commit/482d080e) - Enable React Compiler
- [Commit 21a7fb83](https://github.com/specvital/web/commit/21a7fb83) - Fix accordion overlap
- [React Compiler "use no memo" directive](https://react.dev/reference/react-compiler/directives/use-no-memo)
- [TanStack Virtual Issue #736](https://github.com/TanStack/virtual/issues/736) - React Compiler compatibility
