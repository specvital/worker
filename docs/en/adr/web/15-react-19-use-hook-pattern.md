---
title: React 19 use() Hook Pattern
description: ADR on adopting React 19 use() hook for Promise streaming from Server to Client Components
---

# ADR-15: React 19 use() Hook Pattern

> [한국어 버전](/ko/adr/web/15-react-19-use-hook-pattern.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-07 | @KubrickCode | web   |

## Context

### The Page Transition Delay Problem

The analyze page experienced noticeable delays when users clicked the analyze button from the home page. The issue stemmed from how data fetching blocked Server Component rendering.

**Observed Behavior:**

1. User clicks "Analyze" button on home page
2. Navigation stalls while waiting for `fetchAnalysis()` to complete
3. No visual feedback during the wait (button appears frozen)
4. Page finally renders after data arrives

**Root Cause:**

In the original implementation, the Server Component used `await` directly:

```typescript
// page.tsx (Server Component)
const AnalyzePage = async ({ params }) => {
  const result = await fetchAnalysis(owner, repo); // Blocks rendering
  return <AnalysisContent result={result} />;
};
```

This pattern blocks Server Component rendering until the Promise resolves, causing perceptible delays on slower networks or when the Go backend has cold starts.

### React 19's New Data Fetching Primitive

React 19 introduced the `use()` API, specifically designed to solve this problem. Unlike traditional hooks:

- Can be called inside loops and conditionals
- Works with both Promises and Context
- Integrates seamlessly with Suspense boundaries
- Enables Promise streaming from Server to Client Components

## Decision

**Adopt React 19 `use()` hook pattern for data fetching scenarios where immediate navigation feedback is critical, combined with `useTransition` for navigation state.**

Core principles:

1. **Promise Streaming**: Pass Promises from Server Component to Client Component as props
2. **Suspense Integration**: Wrap Client Components with `<Suspense>` for loading states
3. **Transition Feedback**: Use `useTransition` to show immediate loading indicators during navigation
4. **API Proxy**: Configure Next.js rewrites to enable environment-agnostic API calls

## Options Considered

### Option A: Server Component await (Traditional)

**How It Works:**

- Server Component calls `await fetchData()` directly
- Rendering blocked until data arrives
- Pass resolved data to children

**Evaluation:**

- **Pros**: Simple mental model, no Suspense boundaries needed
- **Cons**: Page appears frozen during fetch, no progressive rendering
- **Rejected**: Poor UX for network-bound operations

### Option B: React 19 use() Hook (Selected)

**How It Works:**

- Server Component creates Promise without awaiting
- Promise passed to Client Component as prop
- Client Component uses `use(promise)` to consume data
- Suspense boundary shows fallback during loading

**Pros:**

- **Non-Blocking Rendering**: Server Component renders immediately, streams Promise
- **Progressive Loading**: Page shell renders instantly, data streams in
- **Suspense Integration**: Native loading state handling
- **Stable Promises**: Promises from Server Components are stable across re-renders

**Cons:**

- Requires understanding of Suspense boundaries
- Need to handle error states with Error Boundaries
- Adds complexity for simple cases

### Option C: Client-Side Fetching Only

**How It Works:**

- Server Component renders immediately without data
- Client Component fetches data in `useEffect`
- Loading spinner while waiting

**Evaluation:**

- **Pros**: Simple, familiar pattern
- **Cons**: No server-side rendering benefits, waterfalls, extra roundtrip
- **Rejected**: Wastes Server Component capabilities

## Implementation Details

### Server Component (Promise Creation)

```typescript
// page.tsx (Server Component)
const AnalyzePage = async ({ params }) => {
  const { owner, repo } = await params;

  // Create Promise without awaiting
  const dataPromise = fetchAnalysis(owner, repo);

  return (
    <Suspense fallback={<Loading />}>
      <AnalysisContent dataPromise={dataPromise} />
    </Suspense>
  );
};
```

### Client Component (Promise Consumption)

```typescript
// analysis-content.tsx (Client Component)
"use client";

import { use } from "react";

type AnalysisContentProps = {
  dataPromise: Promise<AnalysisResult>;
};

export const AnalysisContent = ({ dataPromise }: AnalysisContentProps) => {
  const result = use(dataPromise);
  return <div>{/* Render result */}</div>;
};
```

### Navigation with Transition Feedback

```typescript
// url-input-form.tsx (Client Component)
"use client";

import { useTransition } from "react";

export const UrlInputForm = () => {
  const [isPending, startTransition] = useTransition();

  const handleSubmit = (e) => {
    e.preventDefault();
    // Wrap navigation in transition for immediate feedback
    startTransition(() => {
      router.push(`/analyze/${owner}/${repo}`);
    });
  };

  return (
    <Button disabled={isPending}>
      {isPending ? <Loader2 className="animate-spin" /> : "Analyze"}
    </Button>
  );
};
```

### API Proxy Configuration

```typescript
// next.config.ts
const nextConfig = {
  rewrites: async () => [
    {
      source: "/api/:path*",
      destination: `${API_URL}/api/:path*`,
    },
  ],
};
```

This enables environment-agnostic API calls (client-side uses relative paths, server-side uses full URLs).

## Consequences

### Positive

**Immediate Navigation Feedback:**

- Button shows loading state instantly via `useTransition`
- Page shell renders immediately
- Data streams in progressively

**Non-Blocking Server Rendering:**

- Server Component does not wait for fetch completion
- Promise is serialized and streamed to client
- Better Time to First Byte (TTFB)

**Native Suspense Integration:**

- Loading states handled declaratively
- Error boundaries catch fetch failures
- Consistent loading UI across the application

### Negative

**Added Complexity:**

- Requires understanding of Promise streaming
- Suspense boundaries must be placed correctly
- **Mitigation**: Clear patterns established in codebase

**Not Suitable for All Cases:**

- Polling and cache invalidation need TanStack Query
- Complex state management requires different patterns
- **Mitigation**: Documented when to use each pattern

**Error Handling:**

- Rejected Promises throw to nearest Error Boundary
- Requires `error.tsx` in each route segment
- **Mitigation**: Already part of Next.js App Router conventions

### Pattern Selection Guide

| Scenario                               | Recommended Pattern   |
| -------------------------------------- | --------------------- |
| One-time data fetch, immediate display | React 19 `use()` hook |
| Polling for status updates             | TanStack Query        |
| Cursor-based pagination                | TanStack Query        |
| Cache invalidation after mutation      | TanStack Query        |
| Form submission                        | Server Actions        |

## Evolution Note

This pattern was initially adopted but later migrated to TanStack Query when polling requirements emerged for tracking async analysis status (queued → analyzing → completed). The `use()` hook pattern remains valid for simpler data fetching scenarios without polling or complex cache management needs.

## References

### Internal

- [ADR-04: TanStack Query Selection](/en/adr/web/04-tanstack-query-selection.md) - Successor pattern for polling
- [ADR-02: Next.js 16 + React 19 Selection](/en/adr/web/02-nextjs-react-selection.md)

### External

- [React use() API Documentation](https://react.dev/reference/react/use)
- [React Server Components RFC](https://github.com/reactjs/rfcs/blob/main/text/0188-server-components.md)
- [Next.js Streaming with Suspense](https://nextjs.org/docs/app/building-your-application/routing/loading-ui-and-streaming)
