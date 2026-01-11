---
title: nuqs URL State Management
description: ADR on choosing nuqs for type-safe URL query parameter state management in React/Next.js
---

# ADR-16: nuqs URL State Management

> [한국어 버전](/ko/adr/web/16-nuqs-url-state-management.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-25 | @KubrickCode | web   |

## Context

### The URL State Challenge

The dashboard requires state synchronization between UI and URL for:

1. **Shareable Filter State**: Users sharing dashboard links should see identical filter configurations
2. **Browser History Support**: Back/forward navigation should restore previous filter states
3. **Bookmarkable Views**: Specific filter combinations must be bookmarkable
4. **Type Safety**: Query parameters are strings but need type-safe parsing

### Specific Use Cases

| Feature               | URL Parameter             | Type          |
| --------------------- | ------------------------- | ------------- |
| Dashboard view filter | `?view=starred`           | Literal union |
| Test search           | `?q=auth`                 | String        |
| Status filter         | `?statuses=skipped,todo`  | String array  |
| Framework filter      | `?frameworks=vitest,jest` | String array  |

### Existing Architecture Constraints

- **Next.js 16 App Router**: Server Components with client-side interactivity at leaf nodes
- **React 19**: Modern hooks API with `use()` for data streaming
- **TanStack Query**: Already handles server state; URL state is separate concern
- **TypeScript**: Strong typing required across the codebase

### Candidates Evaluated

1. **nuqs**: Type-safe URL query state manager for React
2. **Manual URLSearchParams**: Native browser API with custom hooks
3. **query-string**: Low-level parsing utility
4. **use-query-params**: Older React library for query params

## Decision

**Adopt nuqs for URL query state management due to its type-safe parsers, useState-like API, and native Next.js App Router support.**

Core principles:

1. **NuqsAdapter at Root**: Wrap application in `NuqsAdapter` for App Router integration
2. **Type-Safe Parsers**: Use `parseAsString`, `parseAsStringLiteral`, `parseAsArrayOf` for type guarantees
3. **Default Values**: Always provide `.withDefault()` to prevent null states
4. **Colocation**: Place `useQueryState` hooks in feature-specific custom hooks

## Options Considered

### Option A: nuqs (Selected)

**How It Works:**

- `useQueryState` hook mirrors `useState` API but persists to URL
- Built-in parsers handle serialization/deserialization
- `NuqsAdapter` integrates with Next.js App Router
- Batched updates prevent History API overload

**Pros:**

- **useState-like API**: Minimal learning curve for React developers
- **Type Safety**: `parseAsStringLiteral` enforces literal union types at compile time
- **App Router Native**: First-class Next.js 13+ support with `NuqsAdapter`
- **Lightweight**: ~5.5 KB gzipped with no external dependencies
- **History Integration**: Automatic browser back/forward support
- **Throttled Updates**: Prevents History API crashes from rapid state changes

**Cons:**

- Additional dependency (~5.5 KB)
- Requires `NuqsAdapter` wrapper at layout level
- Learning curve for parser composition

### Option B: Manual URLSearchParams

**How It Works:**

- Use `useSearchParams()` from `next/navigation`
- Create custom hooks for each query parameter
- Manual serialization/deserialization logic

**Evaluation:**

- **Boilerplate Heavy**: Each parameter needs manual parsing logic
- **No Type Safety**: String parsing without compile-time validation
- **History Edge Cases**: Manual handling of browser navigation
- **Rejected**: Excessive boilerplate; error-prone type coercion

### Option C: query-string

**How It Works:**

- Low-level utility for parsing/stringifying query strings
- No React integration; requires wrapper hooks

**Evaluation:**

- **No React Hooks**: Must build custom hook layer
- **No Type Safety**: Returns `string | string[] | null`
- **Not SSR-Aware**: No Server Component considerations
- **Rejected**: Too low-level; requires significant wrapper code

### Option D: use-query-params

**How It Works:**

- Older React library with similar goals to nuqs
- Uses React Context for query state

**Evaluation:**

- **Outdated**: Last major update predates Next.js App Router
- **RSC Uncertainty**: Unknown Server Components compatibility
- **Larger Bundle**: More dependencies than nuqs
- **Rejected**: nuqs is the modern successor with better App Router support

## Implementation Details

### Root Layout Configuration

```tsx
// app/[locale]/layout.tsx
import { NuqsAdapter } from "nuqs/adapters/next/app";

const LocaleLayout = ({ children }) => (
  <NuqsAdapter>
    <QueryProvider>{children}</QueryProvider>
  </NuqsAdapter>
);
```

### String Literal Parser (Union Types)

```typescript
// features/dashboard/hooks/use-view-filter.ts
import { parseAsStringLiteral, useQueryState } from "nuqs";

export type ViewFilter = "all" | "mine" | "starred" | "community";

const VIEW_FILTER_OPTIONS: ViewFilter[] = ["all", "mine", "starred", "community"];
const viewFilterParser = parseAsStringLiteral(VIEW_FILTER_OPTIONS).withDefault("all");

export const useViewFilter = () => {
  const [viewFilter, setViewFilter] = useQueryState("view", viewFilterParser);
  return { setViewFilter, viewFilter } as const;
};
```

### Array Parser (Multi-Select Filters)

```typescript
// features/analysis/hooks/use-filter-state.ts
import { parseAsArrayOf, parseAsString, useQueryState } from "nuqs";

const arrayParser = parseAsArrayOf(parseAsString, ",").withDefault([]);

export const useFilterState = () => {
  const [frameworks, setFrameworks] = useQueryState("frameworks", arrayParser);
  const [statuses, setStatuses] = useQueryState("statuses", arrayParser);

  return { frameworks, setFrameworks, statuses, setStatuses } as const;
};
```

### String Parser (Search Query)

```typescript
// features/analysis/hooks/use-filter-state.ts
const queryParser = parseAsString.withDefault("");

export const useFilterState = () => {
  const [query, setQuery] = useQueryState("q", queryParser);
  return { query, setQuery } as const;
};
```

## Consequences

### Positive

**Shareable State:**

- Filter URLs like `/dashboard?view=starred&q=auth` are shareable
- Recipients see exact same filter configuration
- Enables support debugging ("send me your current dashboard URL")

**Browser History Integration:**

- Back button restores previous filter states
- Forward button re-applies filters
- Natural browser UX without custom history management

**Type Safety:**

- `parseAsStringLiteral` prevents invalid values at compile time
- `withDefault()` eliminates null checks in consuming components
- IntelliSense support for filter options

**Developer Experience:**

- Same API as `useState`: `const [value, setValue] = useQueryState(...)`
- No boilerplate for serialization/deserialization
- Composable parsers for complex types

### Negative

**Bundle Size:**

- Adds ~5.5 KB gzipped to client bundle
- **Mitigation**: Acceptable for dashboard application; enables significant UX improvements

**Adapter Requirement:**

- Must wrap app in `NuqsAdapter` at root layout
- **Mitigation**: One-time setup; already done in codebase

**Learning Curve:**

- Team must understand parser composition
- **Mitigation**: Established patterns in feature hooks; consistent usage across codebase

### Usage Patterns Established

| Pattern       | Parser                               | Example URL              |
| ------------- | ------------------------------------ | ------------------------ |
| Literal union | `parseAsStringLiteral`               | `?view=starred`          |
| Search string | `parseAsString.withDefault("")`      | `?q=auth`                |
| Multi-select  | `parseAsArrayOf(parseAsString, ",")` | `?statuses=skipped,todo` |

## References

### Internal

- [ADR-02: Next.js 16 + React 19 Selection](/en/adr/web/02-nextjs-react-selection.md) - Framework context
- [ADR-04: TanStack Query Selection](/en/adr/web/04-tanstack-query-selection.md) - Complementary server state

### External

- [nuqs Official Documentation](https://nuqs.dev)
- [nuqs GitHub Repository](https://github.com/47ng/nuqs)
- [Managing search parameters in Next.js with nuqs](https://blog.logrocket.com/managing-search-parameters-next-js-nuqs/)
