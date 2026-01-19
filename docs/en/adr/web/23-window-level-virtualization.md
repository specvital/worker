---
title: Window-Level Virtualization Pattern
description: ADR for window-level virtualization pattern to handle large document rendering performance
---

# ADR-23: Window-Level Virtualization Pattern

> 🇰🇷 [한국어 버전](/ko/adr/web/23-window-level-virtualization.md)

| Date       | Author     | Repos |
| ---------- | ---------- | ----- |
| 2026-01-19 | @specvital | web   |

## Context

The spec-view mode experienced severe performance degradation with 1000+ behaviors:

| Issue                             | Description                                                                                               |
| --------------------------------- | --------------------------------------------------------------------------------------------------------- |
| Computational Bottleneck          | `computeFilteredDocument()` exhibited O(n^3) complexity, recalculating on every render cycle              |
| Insufficient Virtualization Scope | Only behavior-level virtualization at 100+ threshold; no Domain/Feature level virtualization              |
| Nested Structure Challenge        | Deep 4-level hierarchy (Document -> Domain -> Feature -> Behavior) complicated traditional virtualization |

### Previous Approach Failures

Container-based virtualization using `useVirtualizer` created critical UX issues:

- **Inconsistent Scroll Experience**: Separate scroll containers for each virtualized section broke the unified page scroll pattern
- **CSS Layout Breakdown**: `position: absolute` layout (required for container virtualization) disrupted normal CSS spacing, card structures, and visual grouping

### Constraints

| Constraint            | Description                                                                              |
| --------------------- | ---------------------------------------------------------------------------------------- |
| React Compiler        | Must remain compatible with React Compiler; no patterns that break automatic memoization |
| UX Consistency        | Single page scroll; no nested scroll containers                                          |
| Visual Grouping       | Domain/Feature card structure must be preserved                                          |
| Threshold Sensitivity | Must handle both small (< 30 items) and large (1000+) datasets                           |

## Decision

**Adopt window-level virtualization using TanStack Virtual's `useWindowVirtualizer` with flat array conversion and deferred rendering.**

### Core Implementation

1. **Flat Array Conversion**: `flattenSpecDocument()` transforms nested hierarchy into a single array with type discriminators
2. **Window-Level Virtualization**: `useWindowVirtualizer` uses the browser window as scroll container, eliminating nested scroll issues
3. **Deferred Rendering**: React 18's `useDeferredValue` prevents UI blocking during filter/search operations
4. **Lowered Threshold**: Reduced from 100 to 30 items for earlier optimization activation
5. **CSS-Based Grouping**: `isLastInDomain` flag enables visual separation without breaking layout

### Library Selection

**@tanstack/react-virtual 3.13.6**

Selected over alternatives because:

- Native `useWindowVirtualizer` hook for page-level scrolling
- Already in dependency tree (used in other components)
- React 19 compatible (uses `useSyncExternalStore`)
- Known React Compiler compatibility issue documented and mitigated via `"use no memo"` ([ADR-22](/en/adr/web/22-react-compiler-adoption.md))

## Options Considered

### Option A: Window-Level Virtualization with Flat Array (Selected)

`flattenSpecDocument()` converts `Document -> Domain[] -> Feature[] -> Behavior[]` into `FlatItem[]`. Each `FlatItem` carries type discriminator (`domain-header`, `feature-header`, `behavior-row`). `useWindowVirtualizer` virtualizes the flat array using window scroll position.

```typescript
type FlatSpecItem = FlatSpecDomainItem | FlatSpecFeatureItem | FlatSpecBehaviorItem;

const flatItems = useMemo(() => flattenSpecDocument(document), [document]);
const virtualizer = useWindowVirtualizer({
  count: flatItems.length,
  estimateSize: (index) => getItemHeight(flatItems[index].type),
  overscan: 5,
});
const deferredItems = useDeferredValue(virtualizer.getVirtualItems());
```

| Pros                                          | Cons                                      |
| --------------------------------------------- | ----------------------------------------- |
| Single scroll container (window)              | Flat array transformation adds complexity |
| Preserves normal CSS layout                   | Type checking overhead in render function |
| React Compiler compatible (with escape hatch) | Requires `estimateSize` calibration       |
| `useDeferredValue` keeps UI responsive        | Memory overhead from flat array           |

### Option B: Container-Level Virtualization

Multiple `useVirtualizer` instances, one per Domain/Feature section. Each section has its own scroll container.

| Pros                            | Cons                                          |
| ------------------------------- | --------------------------------------------- |
| Simpler per-section logic       | Multiple scroll containers (UX fragmentation) |
| No flat array conversion needed | `position: absolute` breaks CSS spacing       |
|                                 | Card structure collapse                       |

**Rejected**: UX regression outweighed implementation simplicity. Commit `2c45796` explicitly migrated away from this pattern.

### Option C: No Virtualization with Pagination

Paginate Domains/Features (e.g., 10 domains per page). Render all items within current page without virtualization.

| Pros                       | Cons                                   |
| -------------------------- | -------------------------------------- |
| Simplest implementation    | Breaks continuous exploration UX       |
| No virtualization overhead | Page navigation friction               |
| Predictable memory usage   | Doesn't solve O(n^3) computation issue |

**Rejected**: Spec documents are explored as continuous hierarchies; pagination fragments the user experience.

### Option D: Server-Side Rendering with Streaming

Server renders visible portion initially. Stream additional content as user scrolls (React Server Components + Suspense).

| Pros                          | Cons                              |
| ----------------------------- | --------------------------------- |
| Eliminates client computation | Requires server-side scroll state |
| Faster initial paint          | Network latency on scroll         |
|                               | Complex SSR/client hydration      |

**Rejected**: Over-architected for the problem; client-side virtualization is sufficient.

## Consequences

### Positive

| Area               | Benefit                                                    |
| ------------------ | ---------------------------------------------------------- |
| Performance        | 1000+ behaviors render smoothly; only visible items in DOM |
| UX Consistency     | Single page scroll; unified scroll position                |
| CSS Integrity      | Normal flow layout; card structures preserved              |
| Responsiveness     | `useDeferredValue` prevents input lag during filtering     |
| Threshold Coverage | 30-item threshold catches more real-world cases            |

### Negative

| Area                | Trade-off                             | Mitigation                                                        |
| ------------------- | ------------------------------------- | ----------------------------------------------------------------- |
| Complexity          | Flat array transformation logic       | Isolated in `flattenSpecDocument()` utility                       |
| React Compiler      | Requires `"use no memo"` directive    | Documented in [ADR-22](/en/adr/web/22-react-compiler-adoption.md) |
| Type Safety         | Runtime type discrimination in render | Exhaustive switch with TypeScript narrowing                       |
| Estimation Accuracy | `estimateSize` mismatch causes jitter | Measure actual heights; add padding buffer                        |

### Technical Implications

- **Memory Profile**: Flat array duplicates structural metadata but reduces DOM nodes from n to ~20 (viewport size)
- **Scroll Restoration**: Window scroll position automatically persists across navigation (browser native behavior)
- **Search/Filter Integration**: Filter operations update flat array; `useDeferredValue` defers re-virtualization

## Implementation Details

### Files Affected

| File                            | Purpose                                                                  |
| ------------------------------- | ------------------------------------------------------------------------ |
| `virtualized-document-view.tsx` | Main window-level virtualized view                                       |
| `virtualized-behavior-list.tsx` | Container-level virtualized behavior list (older approach, threshold 30) |
| `flatten-spec-document.ts`      | Hierarchy to flat array conversion                                       |
| `flat-spec-item.ts`             | Discriminated union types for flat items                                 |
| `use-document-filter.ts`        | Document filtering with `useDeferredValue`                               |

### Height Estimation Pattern

```typescript
const getItemHeight = (item: FlatSpecItem): number => {
  const baseHeight = item.type === "domain-header" ? 80 : item.type === "feature-header" ? 56 : 72;
  return baseHeight + (item.isLastInDomain ? DOMAIN_GAP : 0);
};
```

### ScrollMargin Handling

```typescript
const [scrollMargin, setScrollMargin] = useState(0);
useLayoutEffect(() => {
  setScrollMargin(listRef.current?.offsetTop ?? 0);
}, []);
```

## References

- [71fce34](https://github.com/specvital/web/commit/71fce34): Main window virtualization implementation
- [a155369](https://github.com/specvital/web/commit/a155369): Fix missing gaps between cards
- [9c41475](https://github.com/specvital/web/commit/9c41475): Restore domain card structure
- [2c45796](https://github.com/specvital/web/commit/2c45796): Container to window scroll migration
- [ADR-22: React Compiler Adoption](/en/adr/web/22-react-compiler-adoption.md) - `"use no memo"` escape hatch pattern
- [ADR-04: TanStack Query Selection](/en/adr/web/04-tanstack-query-selection.md) - TanStack ecosystem
- [TanStack Virtual Documentation](https://tanstack.com/virtual/latest/docs/api/virtualizer)
- [React useDeferredValue](https://react.dev/reference/react/useDeferredValue)
