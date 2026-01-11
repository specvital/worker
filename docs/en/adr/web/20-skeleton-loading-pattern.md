---
title: Skeleton Loading Pattern
description: ADR on using content-aware skeleton screens over spinners for improved perceived performance
---

# ADR-20: Skeleton Loading Pattern

> [한국어 버전](/ko/adr/web/20-skeleton-loading-pattern.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-20 | @KubrickCode | web   |

## Context

### Problem Statement

The web platform requires visual feedback during data loading:

- Analysis pages: 2-10 seconds (async processing with queue)
- Dashboard: Pagination and infinite scroll operations
- Initial page loads: Server Component data fetching

Users need immediate visual feedback to understand system state and maintain engagement during wait times.

### Initial Approach Issues

The initial loading implementation used generic placeholder divs:

```
<div className="h-4 w-full animate-pulse rounded bg-muted" />
```

This approach had limitations:

1. **No layout preview**: Users couldn't anticipate content structure
2. **Layout shift**: Content popped in causing visual instability
3. **Passive waiting**: No sense of progress or structure

### UX Research Context

Research from Nielsen Norman Group and Viget shows:

- Skeleton screens are perceived 20-30% faster than spinners
- Active waiting (seeing structure) feels faster than passive waiting (watching spinner)
- Layout stability reduces cognitive load

## Decision

**Adopt content-aware skeleton loading pattern over spinners for all data loading states.**

Key principles:

1. **Layout fidelity**: Skeleton must match final content dimensions and structure
2. **Feature-specific**: Each feature has dedicated skeleton components
3. **Status awareness**: Long-running operations show contextual status banners
4. **Accessibility**: ARIA attributes for screen reader compatibility

## Options Considered

### Option A: Spinner/Loading Indicator

**How It Works:**

- Central spinner animation during loading
- Simple implementation with existing icons

**Pros:**

- Minimal development effort
- Universal recognition
- Clear "something is happening" signal

**Cons:**

- Passive waiting experience
- No content preview
- Layout shift when content loads
- Research shows lower perceived performance

### Option B: Progress Bar

**How It Works:**

- Linear progress indicator showing completion percentage
- Common in file uploads and downloads

**Pros:**

- Shows completion progress
- Good for known durations
- Familiar pattern

**Cons:**

- Requires known operation duration
- Not suitable for API calls with unknown response times
- No content structure preview
- Inappropriate for content loading use cases

### Option C: Skeleton Screens (Selected)

**How It Works:**

- Placeholder UI matching final content layout
- Pulse animation indicates activity
- Progressive content reveal as data loads

**Pros:**

- Active waiting experience (20-30% perceived faster)
- Layout stability (no content shift)
- Mental model building for users
- Accessibility-compliant with proper ARIA attributes

**Cons:**

- More development effort per feature
- Maintenance when layouts change
- **Mitigation**: Centralized skeleton components with shared base

### Option D: No Loading State

**How It Works:**

- Show nothing until content is ready
- Appropriate only for very fast operations (<1s)

**Evaluation:**

- Acceptable for instant operations
- Problematic for 2-10 second waits
- **Rejected**: Analysis operations exceed 1 second threshold

## Implementation

### Component Hierarchy

```
Skeleton (shadcn/ui base)
├── StatsCardSkeleton (analysis stats)
├── TestListSkeleton (test accordion items)
├── AnalysisSkeleton (full analysis page)
└── RepositorySkeleton (dashboard card)
```

### Design Patterns

**1. Layout Matching**

Skeleton dimensions match actual content:

- Repository name: `h-5 w-32`
- Stats card: Preserved grid layout
- Test list: 6 accordion-style items

**2. Status-Aware Banners**

For long-running operations:

| Status    | Color  | Use Case                    |
| --------- | ------ | --------------------------- |
| loading   | gray   | Initial page load           |
| queued    | blue   | Analysis waiting in queue   |
| analyzing | orange | Active analysis in progress |

**3. Accessibility**

- `aria-busy="true"` on loading containers
- `aria-live="polite"` for status updates
- `role="status"` for live regions
- `aria-label` on skeleton components

## Consequences

### Positive

**1. Improved Perceived Performance**

- Research-backed 20-30% improvement in perceived speed
- Active waiting reduces frustration

**2. Layout Stability**

- No content shift when data loads
- Consistent visual experience

**3. Mental Model Building**

- Users understand content structure before it loads
- Reduced cognitive load on content reveal

**4. Accessibility Compliance**

- Screen reader support via ARIA attributes
- Inclusive design for all users

### Negative

**1. Development Overhead**

- Each feature requires dedicated skeleton component
- **Mitigation**: Shared base component, consistent patterns

**2. Maintenance Burden**

- Layout changes require skeleton updates
- **Mitigation**: Skeleton components co-located with feature components

**3. Potential for Mismatch**

- Skeleton layout may drift from actual content
- **Mitigation**: Code review checklist, visual regression testing

## References

- [Nielsen Norman Group: Skeleton Screens 101](https://www.nngroup.com/articles/skeleton-screens/)
- [LogRocket: Skeleton Loading Screen Design](https://blog.logrocket.com/ux-design/skeleton-loading-screen-design/)
