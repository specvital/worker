---
title: TanStack Query Selection
description: ADR on choosing TanStack Query v5 as the data fetching library for client-side state management
---

# ADR-04: TanStack Query Selection

> [한국어 버전](/ko/adr/web/04-tanstack-query-selection.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-09 | @KubrickCode | web   |

## Context

### The Data Fetching Challenge

The web platform requires client-side data fetching for:

1. **Polling-Based Status Tracking**: Analysis jobs run asynchronously (queued → analyzing → completed/failed). The frontend must poll for status updates until completion.
2. **Cursor-Based Pagination**: Dashboard lists use infinite scroll with cursor-based pagination from the Go backend.
3. **Mutation with Cache Sync**: Actions like bookmarks and reanalysis must update cached data automatically.
4. **REST API Optimization**: All data comes from REST endpoints defined in `openapi.yaml`; no GraphQL.

### Existing Architecture Constraints

- **Next.js 16 + React 19**: App Router with Server Components; data fetching hooks run in Client Components
- **BFF Pattern**: Next.js as thin presentation layer; Go backend handles all business logic
- **OpenAPI Type Generation**: TypeScript types generated from `openapi.yaml` via `openapi-typescript`
- **No Global State Library**: No Redux, Zustand, or similar global state management

### Candidates Evaluated

1. **TanStack Query v5**: Feature-rich data fetching library with polling, infinite queries, mutations
2. **SWR**: Vercel's lightweight data fetching library
3. **RTK Query**: Redux Toolkit's data fetching solution
4. **Apollo Client**: GraphQL-focused but adaptable for REST

## Decision

**Adopt TanStack Query v5 as the primary data fetching library for its polling capabilities, infinite query support, and mature mutation handling.**

Core principles:

1. **Query Key Factories**: Centralized query key definitions per feature domain
2. **Conditional Polling**: Use `refetchInterval` function for status-dependent polling
3. **Cache Invalidation**: Use `invalidateQueries` after mutations for automatic data sync
4. **Type Safety**: Leverage OpenAPI-generated types in query functions

## Options Considered

### Option A: TanStack Query v5 (Selected)

**How It Works:**

- `QueryClient` with customized defaults (`staleTime`, error handlers)
- `useQuery` for data fetching with automatic caching
- `useInfiniteQuery` for cursor-based pagination
- `useMutation` with `onSuccess` cache invalidation
- `refetchInterval` with function support for conditional polling

**Pros:**

- **Polling Excellence**: `refetchInterval` supports functions for conditional polling with backoff
- **Infinite Queries**: Native `useInfiniteQuery` with `getNextPageParam` for cursor pagination
- **Garbage Collection**: Automatic cleanup of unused queries (default 5 minutes)
- **DevTools**: Official DevTools package for debugging cache states
- **React 19 Support**: Uses `useSyncExternalStore`, fully compatible
- **Market Dominance**: 60-70% market share, extensive documentation, community support

**Cons:**

- Larger bundle than SWR (~11-13 KB vs ~4.2 KB gzipped)
- Learning curve for advanced patterns
- HydrationBoundary boilerplate for SSR prefetching

### Option B: SWR

**How It Works:**

- `useSWR` for data fetching with stale-while-revalidate strategy
- `useSWRInfinite` for pagination
- `useSWRMutation` for mutations (added in v2.0)

**Evaluation:**

- **Missing Garbage Collection**: No automatic cleanup of unused queries; memory leaks with dynamic queries
- **Weaker Infinite Queries**: `useSWRInfinite` less intuitive than TanStack's `useInfiniteQuery`
- **No Official DevTools**: Community-built alternatives only
- **No staleTime Equivalent**: Less control over when data is considered fresh
- **Rejected**: Insufficient for polling complexity and pagination requirements

### Option C: RTK Query

**How It Works:**

- API slice definition with endpoints
- Generated hooks (`useGetXQuery`, `useLazyGetXQuery`)
- Tag-based cache invalidation

**Evaluation:**

- **Redux Dependency**: Requires Redux Toolkit adoption
- **Infinite Queries Are New**: Added February 2025, less battle-tested
- **Overhead**: Heavier setup for non-Redux applications
- **Limited Next.js App Router Docs**: Less documented for App Router patterns
- **Rejected**: Unnecessary Redux adoption for current architecture

### Option D: Apollo Client

**How It Works:**

- GraphQL-first design with normalized cache
- `apollo-link-rest` adapter for REST APIs
- Polling via `pollInterval` option

**Evaluation:**

- **REST Is Second-Class**: Requires `apollo-link-rest` adapter
- **Bundle Size**: ~30 KB gzipped, 3x larger than TanStack Query
- **Normalized Cache Overhead**: Complexity not needed for REST APIs
- **GraphQL Concepts**: Fragments, links, resolvers are GraphQL-specific
- **Rejected**: Significant overhead for REST-only application

## Implementation Details

### QueryClient Configuration

```typescript
// lib/query/client.ts
export const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: false,
        retry: false,
        staleTime: 1000 * 60, // 1 minute
      },
    },
    mutationCache: new MutationCache({
      onError: (error, _variables, _context, mutation) => {
        if (isUnauthorizedError(error) && isAuthQuery(mutation.options.mutationKey)) {
          handleUnauthorizedError(queryClient);
        }
      },
    }),
    queryCache: new QueryCache({
      onError: (error, query) => {
        if (isUnauthorizedError(error) && isAuthQuery(query.queryKey)) {
          handleUnauthorizedError(queryClient);
        }
      },
    }),
  });
```

### Polling with Exponential Backoff

```typescript
// features/analysis/hooks/use-analysis.ts
const INITIAL_INTERVAL_MS = 1000;
const MAX_INTERVAL_MS = 5000;
const BACKOFF_MULTIPLIER = 1.5;

const query = useQuery({
  queryKey: analysisKeys.detail(owner, repo),
  queryFn: () => fetchAnalysis(owner, repo),
  refetchInterval: (query) => {
    const response = query.state.data;
    if (response && isTerminalStatus(response)) {
      return false; // Stop polling
    }
    const interval = intervalRef.current;
    intervalRef.current = Math.min(interval * BACKOFF_MULTIPLIER, MAX_INTERVAL_MS);
    return interval;
  },
});
```

### Cursor-Based Infinite Query

```typescript
// features/dashboard/hooks/use-paginated-repositories.ts
export const usePaginatedRepositories = (options: PaginatedRepositoriesOptions) => {
  const query = useInfiniteQuery({
    queryKey: paginatedRepositoriesKeys.list({ limit, sortBy, sortOrder, view }),
    queryFn: ({ pageParam }) =>
      fetchPaginatedRepositories({
        cursor: pageParam,
        limit,
        sortBy,
        sortOrder,
        view,
      }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => (lastPage.hasNext ? lastPage.nextCursor : undefined),
    staleTime: 30 * 1000,
  });

  const data = query.data?.pages.flatMap((page) => page.data) ?? [];
  return { data, hasNextPage: query.hasNextPage, fetchNextPage: query.fetchNextPage };
};
```

### Mutation with Cache Invalidation

```typescript
// features/dashboard/hooks/use-bookmark-mutation.ts
export const useAddBookmark = () => {
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: ({ owner, repo }) => addBookmark(owner, repo),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: paginatedRepositoriesKeys.all });
      toast.success("Bookmark added");
    },
    onError: (error) => toast.error("Failed to add bookmark", { description: error.message }),
  });

  return { addBookmark: mutation.mutate, isPending: mutation.isPending };
};
```

### Query Key Factory Pattern

```typescript
// Centralized query key definitions per feature
export const analysisKeys = {
  all: ["analysis"] as const,
  detail: (owner: string, repo: string) => [...analysisKeys.all, owner, repo] as const,
};

export const paginatedRepositoriesKeys = {
  all: ["paginatedRepositories"] as const,
  list: (options: PaginatedRepositoriesOptions) =>
    [...paginatedRepositoriesKeys.all, "list", options] as const,
};
```

## Consequences

### Positive

**Polling Flexibility:**

- Conditional polling with function-based `refetchInterval`
- Exponential backoff prevents server overload
- Automatic cleanup when polling stops

**Pagination UX:**

- Native infinite query support with cursor handling
- Lagged query data for smooth transitions
- Intersection Observer integration for auto-load

**Developer Experience:**

- Query key factories enable precise cache invalidation
- DevTools for debugging cache states in development
- Type-safe integration with OpenAPI-generated types

**Memory Management:**

- Automatic garbage collection of unused queries
- Configurable `gcTime` (formerly `cacheTime`) prevents memory leaks
- No manual cleanup required for dynamic queries

### Negative

**Bundle Size:**

- ~11-13 KB gzipped vs SWR's ~4.2 KB
- **Mitigation**: Acceptable for dashboard application; DevTools are dev-only

**SSR Complexity:**

- Requires `QueryClientProvider` wrapper in Client Component
- `HydrationBoundary` needed for SSR prefetching
- **Mitigation**: BFF pattern minimizes SSR data requirements

**Learning Curve:**

- Advanced patterns (staleTime, gcTime, structural sharing) require study
- **Mitigation**: Established patterns in codebase; internal documentation

### Usage Patterns Established

| Pattern        | Implementation                          | File                            |
| -------------- | --------------------------------------- | ------------------------------- |
| Polling        | `refetchInterval` with function         | `use-analysis.ts`               |
| Infinite Query | `useInfiniteQuery` + `getNextPageParam` | `use-paginated-repositories.ts` |
| Mutation       | `useMutation` + `invalidateQueries`     | `use-bookmark-mutation.ts`      |
| Data Fetching  | `useQuery` + query key factory          | `use-my-repositories.ts`        |

## References

### Internal

- [ADR-02: Next.js 16 + React 19 Selection](/en/adr/web/02-nextjs-react-selection.md)

### External

- [TanStack Query Official Documentation](https://tanstack.com/query/latest)
- [TanStack Query Comparison Page](https://tanstack.com/query/latest/docs/framework/react/comparison)
- [Infinite Queries Guide](https://tanstack.com/query/v5/docs/framework/react/guides/infinite-queries)
- [React Query vs SWR Comparison](https://tanstack.com/query/latest/docs/framework/react/comparison)
