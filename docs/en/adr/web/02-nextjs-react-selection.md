---
title: Next.js 16 + React 19 Selection
description: ADR on choosing Next.js 16 with React 19 as the frontend framework for BFF architecture
---

# ADR-02: Next.js 16 + React 19 Selection

> [한국어 버전](/ko/adr/web/02-nextjs-react-selection.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-04 | @KubrickCode | web   |

## Context

### The Framework Selection Question

The web platform requires a frontend framework for the BFF (Backend-for-Frontend) architecture. Key requirements:

1. **SSR/SSG Support**: Server-side rendering for performance and SEO
2. **React Ecosystem**: Leverage existing component libraries and developer expertise
3. **BFF Pattern**: Thin presentation layer with Go backend handling business logic
4. **i18n**: Korean and English language support
5. **Real-time Updates**: Polling-based dashboard with status tracking

### Existing Architecture Constraints

- **Go Backend**: REST API with OpenAPI specification
- **Type Generation**: `openapi.yaml` → TypeScript types via `openapi-typescript`
- **Deployment**: Vercel (frontend) + Railway (backend) + Neon (PostgreSQL)
- **Authentication**: GitHub OAuth with JWT tokens

### Candidates Evaluated

1. **Next.js 16 + React 19**: App Router, Server Components, Turbopack
2. **Remix**: React Router v7, Server-first architecture
3. **SvelteKit**: Svelte 5 runes, smallest bundle sizes
4. **Astro**: Islands architecture, content-focused
5. **Nuxt 4**: Vue-based, Nitro engine

## Decision

**Adopt Next.js 16 with React 19 as the frontend framework for BFF architecture.**

Core principles:

1. **Server Components First**: Default to Server Components; Client Components only at leaf nodes
2. **Explicit Rendering Strategy**: Every page declares `force-static`, `force-dynamic`, or `revalidate`
3. **No Direct Database Access**: All data operations via Go backend API
4. **Type Safety**: OpenAPI → TypeScript generation chain

## Options Considered

### Option A: Next.js 16 + React 19 (Selected)

**How It Works:**

- App Router with React Server Components
- Turbopack for development (2-5x faster builds)
- Server Actions for mutations, API routes for webhooks only
- React 19 `use()` hook for data streaming

**Pros:**

- **Market Dominance**: 78% of new React apps use Next.js; 42% frontend market share
- **React 19 Features**: `use()` hook, View Transitions, React Compiler optimization
- **Ecosystem Maturity**: TanStack Query, shadcn/ui, next-intl, next-themes all optimized
- **Vercel Synergy**: Zero-config deployment, Edge Functions, preview deployments
- **Talent Pool**: Largest developer community; hiring advantage

**Cons:**

- Vercel deployment optimization bias
- Complexity for simple applications
- Bundle size larger than Svelte alternatives

### Option B: Remix

**How It Works:**

- React Router v7 with Server Components (preview)
- Loader/Action pattern for data fetching
- Progressive enhancement focus

**Evaluation:**

- RSC support in preview only (July 2025)
- Smaller ecosystem than Next.js
- Limited i18n tooling compared to next-intl
- **Rejected**: RSC instability; ecosystem gaps

### Option C: SvelteKit

**How It Works:**

- Svelte 5 with runes reactivity
- 1.6 KB runtime vs React's 44 KB
- Native SSR with Vercel adapter

**Evaluation:**

- 72.8% developer satisfaction (highest rated)
- 122:1 React vs Svelte job ratio disadvantage
- Requires complete codebase rewrite (~50%)
- Different component paradigm (SFCs vs JSX)
- **Rejected**: Migration cost; hiring challenges

### Option D: Astro

**How It Works:**

- Islands architecture for partial hydration
- Multi-framework support (React, Vue, Svelte)
- Content-first static generation

**Evaluation:**

- Excellent for blogs and marketing sites
- Not suitable for interactive dashboards
- Real-time polling challenges
- No built-in global state management
- **Rejected**: Architecture mismatch for dashboard UX

### Option E: Nuxt 4

**How It Works:**

- Vue 3 Composition API
- Nitro engine with multi-runtime support
- NuxtLabs acquired by Vercel (July 2025)

**Evaluation:**

- Feature parity with Next.js for SSR/SSG/ISR
- Vue-based; requires complete rewrite (~50%)
- Different template syntax and paradigm
- **Rejected**: Migration cost; paradigm shift

## Implementation Details

### Framework Features Adopted

| Feature          | Implementation                                     |
| ---------------- | -------------------------------------------------- |
| App Router       | `app/[locale]/` structure with layouts             |
| React 19 `use()` | Promise streaming from Server to Client Components |
| next-intl        | URL-based i18n with `/en`, `/ko` prefixes          |
| next-themes      | System preference detection + manual toggle        |
| TanStack Query   | Polling-based analysis status, cursor pagination   |
| shadcn/ui        | Radix-based accessible components                  |
| nuqs             | Type-safe URL query state management               |

### BFF Architecture Implementation

```
Browser ↔ Next.js Server (Vercel) ↔ Go Backend (Railway) ↔ PostgreSQL (Neon)
              │
              └─→ GitHub API (OAuth tokens)
```

**Boundaries Enforced:**

- Next.js: SSR/SSG, API aggregation, session management, caching
- Go Backend: Business logic, database operations, external API calls

### Component Strategy

| Type             | Use When                            | Example                     |
| ---------------- | ----------------------------------- | --------------------------- |
| Server Component | Data fetching, no interactivity     | Page layouts, data displays |
| Client Component | useState, useEffect, event handlers | Forms, modals, toggles      |

```tsx
// Server Component (default)
export default async function Page() {
  const data = await fetchData();
  return <Display data={data} />;
}

// Client Component (explicit)
("use client");
export function InteractiveForm() {
  const [state, setState] = useState();
}
```

### Rendering Strategy

```typescript
// Every page MUST declare one:
export const dynamic = "force-static"; // SSG
export const dynamic = "force-dynamic"; // SSR
export const revalidate = 3600; // ISR (seconds)
```

## Consequences

### Positive

**Development Velocity:**

- Zero-config Vercel deployment
- Turbopack reduces dev server startup by 2-5x
- Extensive pre-built component ecosystem (shadcn/ui, Radix)

**Performance:**

- React Compiler automatic memoization (25-40% fewer re-renders)
- Streaming SSR with React 19 `use()` hook
- Edge deployment for global dashboard performance

**Maintainability:**

- Largest developer talent pool reduces hiring friction
- Extensive documentation and community support
- Clear upgrade paths with codemods

### Negative

**Vercel Coupling:**

- Deep platform optimization may create switching friction
- **Mitigation**: Standard Node.js deployment possible on Railway, AWS, Cloudflare

**Bundle Size:**

- Larger than SvelteKit alternatives (~44 KB React runtime)
- **Mitigation**: Acceptable for dashboard application; offset by ecosystem benefits

**Complexity:**

- Server Components mental model requires team training
- **Mitigation**: BFF pattern isolates frontend complexity; explicit rendering declarations

### Migration Path

If future migration becomes necessary:

| Target    | Effort      | Notes                                              |
| --------- | ----------- | -------------------------------------------------- |
| Remix     | Medium      | Same React ecosystem; route conventions differ     |
| Astro     | Medium-High | Static-first; architecture mismatch for dashboards |
| SvelteKit | High        | Full rewrite required                              |

BFF architecture ensures backend remains unchanged during any frontend migration.

## References

### Internal

- [ADR-01: Go as Backend Language](/en/adr/web/01-go-backend-language.md)
- [PRD: Tech Stack](/en/prd/06-tech-stack.md)
- [Tech Radar](/en/tech-radar.md)

### External

- [Next.js 16 Release Blog](https://nextjs.org/blog/next-16)
- [React 19 Documentation](https://react.dev/)
- [Next.js App Router](https://nextjs.org/docs/app)
- [next-intl Documentation](https://next-intl.dev/)
- [TanStack Query](https://tanstack.com/query)
- [shadcn/ui](https://ui.shadcn.com/)
