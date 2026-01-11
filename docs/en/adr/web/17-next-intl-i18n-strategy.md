---
title: next-intl i18n Strategy
description: ADR on choosing next-intl for internationalization with URL-based routing
---

# ADR-17: next-intl i18n Strategy

> [한국어 버전](/ko/adr/web/17-next-intl-i18n-strategy.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-06 | @KubrickCode | web   |

## Context

### The Internationalization Requirement

SpecVital targets both Korean and English-speaking developer communities. A proper i18n solution was needed to:

1. Support Korean and English languages initially
2. Provide URL-based routing for SEO benefits (`/ko/...`, `/en/...`)
3. Integrate seamlessly with Next.js App Router and Server Components
4. Enable browser language detection for first-time visitors
5. Maintain type safety for translation keys

### App Router Challenges

Next.js 13+ App Router removed the built-in `i18n` configuration that Pages Router provided. This created new requirements:

- Manual locale routing setup via `[locale]` dynamic segments
- Server Component compatibility (no `useEffect` or client-side detection)
- Proper message loading without shipping all translations to the client
- Hydration safety for time-sensitive content (relative dates)

## Decision

**Adopt next-intl as the internationalization library for Next.js frontend.**

Core implementation:

1. **URL-Based Routing**: `/[locale]/` prefix for all routes
2. **Server-First**: `getTranslations()` for Server Components
3. **ICU Message Format**: Pluralization and interpolation support
4. **Browser Detection**: Automatic redirect based on `Accept-Language`

## Options Considered

### Option A: next-intl (Selected)

**How It Works:**

- Purpose-built for Next.js App Router
- Native Server Component support via `getTranslations()`
- Client Component hooks via `useTranslations()`
- Middleware handles locale detection and routing

**Pros:**

- **App Router Native**: Designed specifically for RSC and App Router
- **Type Safety**: TypeScript autocompletion for translation keys
- **Bundle Optimization**: Only loads messages for current locale on server
- **Simple API**: Identical hook API for both Server and Client Components
- **Active Maintenance**: Regular updates following Next.js releases

**Cons:**

- Smaller ecosystem compared to i18next
- Static rendering requires explicit `setRequestLocale()` calls
- Less documentation compared to react-i18next

### Option B: react-i18next + next-i18next

**How It Works:**

- Mature i18next ecosystem with React bindings
- Requires additional setup for App Router compatibility
- Plugin-based architecture for features

**Pros:**

- Large ecosystem and community
- Extensive feature set (namespaces, backends, plugins)
- Battle-tested in production at scale

**Cons:**

- **App Router Friction**: Originally designed for Pages Router
- Complex configuration for Server Components
- Larger bundle size with full i18next core
- Requires workarounds for RSC compatibility

### Option C: next-translate

**How It Works:**

- File-based translations with automatic code-splitting
- Simpler feature set than i18next

**Evaluation:**

- Limited App Router support at decision time
- Fewer updates compared to next-intl
- Missing some RSC-specific optimizations
- **Rejected**: Insufficient App Router integration

### Option D: Built-in Next.js + Custom Solution

**How It Works:**

- Manual implementation using Next.js middleware
- Custom translation loading and hooks

**Evaluation:**

- Maximum flexibility but high maintenance cost
- Must implement pluralization, interpolation manually
- Risk of subtle hydration mismatches
- **Rejected**: Reinventing well-solved problems

## Implementation Details

### File Structure

```
src/frontend/
├── i18n/
│   ├── config.ts      # Locale definitions
│   ├── navigation.ts  # Localized Link, useRouter
│   ├── request.ts     # Server-side message loading
│   └── routing.ts     # Routing configuration
├── messages/
│   ├── en.json        # English translations
│   └── ko.json        # Korean translations
├── middleware.ts      # Locale detection, redirects
└── app/[locale]/      # Locale-prefixed routes
```

### Locale Configuration

Two locales supported with English as default:

- `en` (default): English
- `ko`: Korean (한국어)

### Middleware Behavior

1. Check URL for locale prefix
2. If missing, detect from `Accept-Language` header
3. Redirect to appropriate locale path
4. Set response cookies for subsequent requests

### Translation Usage Patterns

**Server Components:**

```tsx
const t = await getTranslations("namespace");
return <h1>{t("key")}</h1>;
```

**Client Components:**

```tsx
const t = useTranslations("namespace");
return <button>{t("action")}</button>;
```

**ICU Pluralization:**

```json
{
  "tests": "{count, plural, =0 {No tests} =1 {1 test} other {# tests}}"
}
```

### Hydration Safety

For time-sensitive content like relative dates, use `useNow()` hook:

```tsx
const now = useNow({ updateInterval: 60000 });
const formatted = formatRelativeTime(date, now);
```

This prevents hydration mismatches between server and client render times.

## Consequences

### Positive

**Developer Experience:**

- TypeScript autocompletion for translation keys
- Consistent API across Server and Client Components
- Clear separation of translation files by locale

**Performance:**

- Server-side message resolution (no client bundle bloat)
- Per-route translation loading
- Lazy loading for client components if needed

**SEO Benefits:**

- Locale-prefixed URLs for search engines
- Proper `lang` attribute on `<html>`
- `hreflang` alternates in metadata

**User Experience:**

- Automatic language detection on first visit
- Shareable localized URLs
- Language switcher in header

### Negative

**Static Rendering Limitation:**

- Currently requires `setRequestLocale()` for static rendering
- Addressed in each layout and page component
- **Mitigation**: Future next-intl versions aim to remove this requirement

**Learning Curve:**

- Team must learn ICU message format for pluralization
- Different APIs for async vs sync contexts
- **Mitigation**: Documented patterns in CLAUDE.md

**Message Management:**

- Manual synchronization between language files
- Risk of missing translations in one locale
- **Mitigation**: CI checks for missing keys (future enhancement)

## References

- [next-intl Documentation](https://next-intl.dev/)
- [ICU Message Format](https://unicode-org.github.io/icu/userguide/format_parse/messages/)
