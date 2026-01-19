---
title: OAuth Return URL Handling
description: ADR for cookie-based return URL preservation during OAuth authentication flow
---

# ADR-25: OAuth Return URL Handling

> 🇰🇷 [한국어 버전](/ko/adr/web/25-oauth-return-url-handling.md)

| Date       | Author     | Repos |
| ---------- | ---------- | ----- |
| 2026-01-16 | @specvital | web   |

## Context

Users logging in via GitHub OAuth from pages other than the homepage were redirected to the dashboard instead of returning to their original page.

| Entry Point          | Expected Behavior       | Actual Behavior             |
| -------------------- | ----------------------- | --------------------------- |
| `/en/pricing`        | Return to `/en/pricing` | Redirect to `/en/dashboard` |
| `/en/explore`        | Return to `/en/explore` | Redirect to `/en/dashboard` |
| `/en/{owner}/{repo}` | Return to analysis page | Redirect to `/en/dashboard` |

### Root Causes

**1. Conflicting Redirect Logic**

The `AuthenticatedRedirect` component on the homepage affected redirect behavior globally.

**2. Client-Server Storage Mismatch**

```
Client (Browser)                    Server (Route Handler)
┌─────────────────┐                ┌──────────────────────┐
│ sessionStorage  │ ─────────X──── │ OAuth callback       │
│ (returnTo URL)  │  Not accessible│ (code exchange)      │
└─────────────────┘                └──────────────────────┘
```

The initial approach stored `returnTo` in `sessionStorage`, but Next.js App Router OAuth callbacks execute as server-side Route Handlers, which cannot access browser `sessionStorage`.

### Constraints

| Constraint                 | Source                                              | Implication                           |
| -------------------------- | --------------------------------------------------- | ------------------------------------- |
| Server-side OAuth callback | Next.js App Router                                  | Cannot use client-only storage        |
| Locale preservation        | [ADR-17](/en/adr/web/17-next-intl-i18n-strategy.md) | Return URL must include locale prefix |
| Security requirements      | OAuth best practices                                | Must prevent open redirect attacks    |

## Decision

**Cookie-based return URL storage with server-side redirect validation.**

```typescript
// Before OAuth redirect (client-side)
const returnTo = window.location.pathname;
document.cookie = `returnTo=${encodeURIComponent(returnTo)}; path=/; max-age=300; SameSite=Lax`;
```

```typescript
// OAuth callback (server-side Route Handler)
const returnTo = cookies().get("returnTo")?.value;
const safeReturnTo = validateReturnUrl(returnTo);
cookies().delete("returnTo");
redirect(safeReturnTo || "/dashboard");
```

### Cookie Configuration

| Parameter | Value        | Rationale                                |
| --------- | ------------ | ---------------------------------------- |
| Storage   | HTTP Cookie  | Accessible server-side                   |
| Max-Age   | 300s (5 min) | Covers OAuth flow, limits stale URL risk |
| SameSite  | Lax          | Prevents cross-site request attacks      |
| Path      | /            | Available to all routes                  |

## Options Considered

### Option A: sessionStorage-based (Initial, Failed)

- Store `returnTo` in browser `sessionStorage`

**Pros:**

- Simple, no cookie management
- Scoped to browser tab

**Cons:**

- Server-side Route Handler cannot access `sessionStorage`
- Architectural incompatibility with Next.js App Router

**Decision:** Rejected.

### Option B: Cookie-based Storage (Selected)

- Store `returnTo` in HTTP cookie before OAuth redirect
- Server reads, validates, and clears cookie on callback

**Pros:**

- Works with server-side Route Handlers
- Short-lived (5 min)
- SameSite=Lax provides CSRF protection

**Cons:**

- Requires cookie management
- Open redirect vector (mitigated by validation)

**Decision:** Selected.

### Option C: OAuth State Parameter

- Encode `returnTo` within OAuth state parameter

**Pros:**

- Stateless, built into OAuth spec

**Cons:**

- GitHub state parameter size limits
- Complicates state handling if used for CSRF protection

**Decision:** Rejected.

### Option D: Database Session Storage

- Store `returnTo` in session table

**Pros:**

- Reliable server-side storage

**Cons:**

- Adds database latency
- Over-engineered for redirect URL storage
- Violates PaaS-first simplicity ([ADR-06](/en/adr/06-paas-first-infrastructure.md))

**Decision:** Rejected.

## Implementation

### Return URL Flow

```
1. User on /en/pricing clicks "Sign in with GitHub"
   └─> Set cookie: returnTo=/en/pricing; max-age=300

2. Redirect to GitHub OAuth
   └─> User authenticates on github.com

3. GitHub redirects to /api/auth/callback/github
   └─> Server reads returnTo cookie
   └─> Validate: starts with "/" and not "//"
   └─> Delete cookie
   └─> Redirect to /en/pricing

4. User arrives back at /en/pricing (authenticated)
```

### URL Validation Logic

```typescript
function validateReturnUrl(url: string | undefined): string | null {
  if (!url) return null;

  // Must start with single slash (relative path)
  if (!url.startsWith("/")) return null;

  // Reject protocol-relative URLs (open redirect vector)
  if (url.startsWith("//")) return null;

  // Reject URLs with embedded credentials
  if (url.includes("@") || url.includes("\\")) return null;

  return url;
}
```

## Consequences

**Positive:**

- Users return to original page after OAuth login
- Locale/i18n context preserved
- Works with Next.js App Router server-side handlers
- Short cookie expiry (5 min) limits attack window

**Negative:**

- Cookie overhead (minimal, <100 bytes)
- Open redirect attack surface (mitigated by validation)
- Falls back to dashboard if cookies disabled

## Security Considerations

### Open Redirect Prevention

| Attack Vector          | Mitigation           |
| ---------------------- | -------------------- |
| Absolute URL injection | Require `/` prefix   |
| Protocol-relative URL  | Reject `//` prefix   |
| URL with credentials   | Reject `@` character |
| Backslash bypass       | Reject `\` character |

### Cookie Security Attributes

| Attribute | Value | Benefit                     |
| --------- | ----- | --------------------------- |
| SameSite  | Lax   | Prevents cross-site attacks |
| Max-Age   | 300   | Limits stale URL risk       |
| Path      | /     | Application-scoped          |

## References

- [Commit 9a961ed](https://github.com/specvital/web/commit/9a961ed) - OAuth return URL fix
- [ADR-17: next-intl i18n Strategy](/en/adr/web/17-next-intl-i18n-strategy.md)
- [Next.js Cookies API](https://nextjs.org/docs/app/api-reference/functions/cookies)
