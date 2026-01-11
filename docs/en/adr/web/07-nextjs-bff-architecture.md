---
title: Next.js BFF Architecture
description: ADR on using Next.js as a thin Backend-for-Frontend layer with all business logic in Go backend
---

# ADR-07: Next.js BFF Architecture

> [Korean Version](/ko/adr/web/07-nextjs-bff-architecture.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-01-03 | @KubrickCode | web   |

## Context

### The Frontend-Backend Communication Question

Modern web applications face a fundamental architectural question: how should the frontend communicate with backend services?

In the SpecVital ecosystem, we have:

- **Go Backend**: Business logic, database access, GitHub API integration, analysis orchestration
- **Frontend**: React-based UI for repository analysis visualization
- **External Services**: GitHub OAuth, GitHub API, River queue (shared with Worker)

### Challenges of Direct Client-Backend Communication

| Challenge                 | Impact                                                    |
| ------------------------- | --------------------------------------------------------- |
| Security Exposure         | Tokens and API keys visible in browser DevTools           |
| Multiple Network Requests | N+1 problem when aggregating data from multiple endpoints |
| No SSR/SSG                | SEO limitations, slower initial page load                 |
| CORS Complexity           | Cross-origin issues between frontend and backend domains  |
| Backend API Coupling      | Frontend tightly coupled to backend API structure         |

### Related Architectural Decisions

- [ADR-01: Go as Backend Language](/en/adr/web/01-go-backend-language.md) - Backend language choice
- [ADR-02: Next.js + React Selection](/en/adr/web/02-nextjs-react-selection.md) - Frontend framework choice
- [ADR-03: API and Worker Service Separation](/en/adr/03-api-worker-service-separation.md) - Service boundaries

## Decision

**Adopt Next.js as a thin Backend-for-Frontend (BFF) layer, with all business logic remaining in the Go backend.**

### Architecture

```
Browser <-> Next.js Server (BFF) <-> Go Backend API <-> Database
                                          |
                                          v
                                    Worker Service
```

### Core Principles

1. **Next.js is a Translation Layer**: Only client-specific logic (data shaping, SSR, caching)
2. **No Business Logic in BFF**: All domain logic resides in Go backend
3. **No Database Access**: Next.js never touches PostgreSQL directly
4. **API Proxy Pattern**: Frontend calls `/api/*`, Next.js rewrites to Go backend

### BFF Responsibilities

| Allowed                         | Forbidden                           |
| ------------------------------- | ----------------------------------- |
| Server-Side Rendering (SSR/SSG) | Business logic implementation       |
| API request aggregation         | Direct database queries             |
| Response caching                | Data validation beyond sanitization |
| Session/cookie management       | Domain entity definitions           |
| Data shape transformation       | Queue job creation                  |

## Options Considered

### Option A: Next.js as Thin BFF (Selected)

**How It Works:**

- Next.js Server Components fetch data from Go backend
- API proxy via `next.config.ts` rewrites (`/api/*` -> backend)
- Server Actions for mutations call backend endpoints
- Route Handlers only for external webhooks (OAuth callbacks)

**Pros:**

- **Security**: Tokens stay server-side in httpOnly cookies
- **Performance**: SSR eliminates client-side loading states, reduces TTFB
- **Aggregation**: Combine multiple backend calls into single frontend request
- **Caching**: Fine-grained control with Next.js cache directives
- **Type Safety**: Shared OpenAPI-generated types between BFF and frontend

**Cons:**

- Additional network hop (browser -> Next.js -> Go)
- Infrastructure complexity (two services to deploy)
- Potential single point of failure
- Team must understand both TypeScript and Go

### Option B: SPA with Direct API Calls

**How It Works:**

- React SPA calls Go backend directly via CORS
- All rendering happens client-side
- Tokens stored in localStorage or cookies

**Pros:**

- Simpler architecture (one less service)
- No additional network hop
- Lower infrastructure cost

**Cons:**

- **Security Risk**: Tokens exposed in browser (XSS vulnerability)
- **No SSR**: Poor SEO, slower perceived performance
- **CORS Complexity**: Must configure allowed origins
- **N+1 Requests**: Client makes multiple calls for aggregated views
- **Loading States**: User sees loading spinners, not content

### Option C: API Gateway + SPA

**How It Works:**

- API Gateway (Kong, AWS API Gateway) handles routing and auth
- SPA communicates through gateway
- Gateway proxies to Go backend

**Pros:**

- Centralized authentication and rate limiting
- Protocol translation capability
- Monitoring and logging at gateway level

**Cons:**

- No SSR capability
- Cannot aggregate/transform data for UI
- Generic API surface, not optimized for frontend needs
- Additional infrastructure component to manage

### Option D: Full-Stack Next.js (Business Logic in BFF)

**How It Works:**

- Next.js handles both UI and business logic
- Direct database access via Prisma or Drizzle
- Server Actions for all mutations

**Pros:**

- Single codebase, single deployment
- Simpler mental model
- No inter-service communication

**Cons:**

- **Violates Existing Architecture**: Conflicts with Go-based ecosystem
- **Cannot Share Core Library**: Parser, crypto utilities in Go
- **Queue Incompatibility**: River is PostgreSQL-based Go library
- **Duplicated Logic**: Must reimplement encryption, validation in TypeScript
- Harder to scale backend independently

## Implementation

### API Proxy Configuration

```typescript
// next.config.ts
const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8000";

const nextConfig: NextConfig = {
  rewrites: async () => [
    {
      destination: `${API_URL}/api/:path*`,
      source: "/api/:path*",
    },
  ],
};
```

### Server Component Data Fetching

```typescript
// Server Component - fetches from Go backend via proxy
export default async function Page() {
  const response = await fetch('/api/analyze/owner/repo', {
    cache: 'no-store'
  });
  const data = await response.json();
  return <AnalysisView data={data} />;
}
```

### Client Component Pattern

```typescript
// Client Component - uses BFF-proxied API
'use client';

export function AnalysisContent({ dataPromise }) {
  const data = use(dataPromise);  // React 19 use() hook
  return <Display data={data} />;
}
```

### Folder Structure

```
features/[name]/
├── components/     # UI components
├── hooks/          # React hooks (TanStack Query)
├── api/            # API client functions (call /api/* proxy)
└── index.ts        # Barrel export
```

## Consequences

### Positive

**Security:**

- Tokens and secrets never reach browser
- httpOnly cookies for session management
- CSP headers enforced at server level
- Reduced attack surface (single trusted application)

**Performance:**

- Server-Side Rendering eliminates client loading states
- Data aggregation reduces network round trips
- Built-in caching with revalidation control
- Streaming with React 19 Suspense

**Developer Experience:**

- Unified codebase for frontend and BFF
- Type safety from OpenAPI generation
- Simplified debugging with unified logging
- Consistent error handling patterns

**Scalability:**

- Frontend and backend scale independently
- CDN-friendly static pages where applicable
- Edge caching for common requests

### Negative

**Additional Network Hop:**

- Every request adds ~1-5ms latency
- **Mitigation**: Server Components reduce total round trips; caching minimizes backend calls

**Infrastructure Complexity:**

- Two services to deploy and monitor (Next.js + Go)
- **Mitigation**: Containerized deployment, unified CI/CD, shared monitoring

**Single Point of Failure:**

- BFF unavailability blocks all frontend access
- **Mitigation**: Health checks, multiple replicas, circuit breakers

**Learning Curve:**

- Team must understand both Next.js patterns and Go backend
- **Mitigation**: Clear documentation (CLAUDE.md, nextjs.md), code reviews

### Rules Enforced

From project's `nextjs.md`:

| Rule                         | Enforcement                         |
| ---------------------------- | ----------------------------------- |
| No database access           | Code review, no ORM dependencies    |
| No business logic            | Handler functions call backend only |
| Explicit cache declaration   | ESLint rules, PR checklist          |
| Server Components by default | `'use client'` only at leaf nodes   |

## References

- [Next.js BFF Guide](https://nextjs.org/docs/app/guides/backend-for-frontend)
- [Sam Newman - BFF Pattern](https://samnewman.io/patterns/architectural/bff/)
- [Microsoft Azure - BFF Pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/backends-for-frontends)
- [ADR-01: Go as Backend Language](/en/adr/web/01-go-backend-language.md)
- [ADR-02: Next.js + React Selection](/en/adr/web/02-nextjs-react-selection.md)
- [ADR-03: API and Worker Service Separation](/en/adr/03-api-worker-service-separation.md)
