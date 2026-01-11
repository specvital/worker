---
title: next-themes Dark Mode
description: ADR on choosing next-themes for dark mode implementation with system theme detection
---

# ADR-18: next-themes Dark Mode

> 🇰🇷 [한국어 버전](/ko/adr/web/18-next-themes-dark-mode.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-06 | @KubrickCode | web   |

## Context

### The Dark Mode Challenge

Modern web applications require dark mode support for:

1. **User Preference**: Users expect theme customization
2. **Accessibility**: Reduced eye strain in low-light environments
3. **System Integration**: Respect OS-level theme preferences
4. **Professional Appearance**: Industry-standard feature for developer tools

### Technical Challenges in SSR

Dark mode in Server-Side Rendered applications faces unique challenges:

- **Hydration Mismatch**: Server renders one theme, client prefers another
- **Flash of Incorrect Theme (FOIT)**: Brief flash of wrong theme during page load
- **State Persistence**: Remember user preference across sessions
- **System Detection**: Detect and respond to OS preference changes

### Existing Architecture

The project already uses:

- **Next.js 16 App Router**: Server Components with SSR/SSG
- **Tailwind CSS v4**: Utility-first styling with `dark:` variant
- **shadcn/ui**: Component library recommending next-themes
- **OKLCH Color Space**: CSS variables for theming (ADR-05)

## Decision

**Adopt next-themes as the dark mode solution for SSR-safe theme management with system preference detection.**

Core principles:

1. **Hydration Safety**: Prevent theme flash with injected script
2. **System Integration**: Auto-detect `prefers-color-scheme`
3. **User Override**: Allow manual light/dark/system selection
4. **Tailwind Compatibility**: Class-based dark mode for `dark:` utilities
5. **Persistence**: LocalStorage-backed preference retention

## Options Considered

### Option A: next-themes (Selected)

**How It Works:**

- ThemeProvider wraps application, injects blocking script in `<head>`
- Script reads localStorage/system preference before paint
- Sets `class="dark"` on `<html>` element synchronously
- Prevents FOIT by resolving theme before first render

**Pros:**

- **SSR Flash Prevention**: Synchronous script injection solves the hard problem
- **shadcn/ui Standard**: Official recommendation for dark mode
- **Tailwind Integration**: Native `attribute="class"` for `dark:` variants
- **Zero Config**: Works out of the box with sensible defaults
- **Lightweight**: ~2KB gzipped, minimal runtime overhead
- **Tab Sync**: Automatic synchronization across browser tabs
- **Industry Proven**: 4.8M+ weekly downloads, 6.1k GitHub stars

**Cons:**

- Single maintainer (pacocoursey)
- Requires `suppressHydrationWarning` on `<html>` element

### Option B: CSS-Only with prefers-color-scheme

**How It Works:**

- Use CSS media query `@media (prefers-color-scheme: dark)`
- Tailwind config: `darkMode: 'media'`
- No JavaScript required

**Evaluation:**

- **No User Toggle**: Cannot override system preference
- **No Persistence**: User choice not remembered
- **No Hybrid Mode**: Cannot offer light/dark/system options
- **Rejected**: Insufficient feature set for user expectations

### Option C: Custom Context + zustand

**How It Works:**

- Create ThemeContext with zustand for state management
- Manually implement localStorage persistence
- Add blocking script in `_document.tsx` or layout

**Evaluation:**

- **Reinventing the Wheel**: 50+ lines to replicate next-themes functionality
- **Maintenance Burden**: Must handle edge cases (SSR, hydration, tab sync)
- **Error Prone**: Easy to introduce subtle hydration mismatches
- **Rejected**: No benefit over battle-tested library

### Option D: usehooks-ts useDarkMode

**How It Works:**

- Import `useDarkMode` hook from usehooks-ts library
- Provides `isDarkMode`, `toggle`, `enable`, `disable` API

**Evaluation:**

- **No SSR Solution**: Does not address hydration flash
- **No System Detection**: Requires `useTernaryDarkMode` for system preference
- **Library Overhead**: ~10KB for single-feature dependency
- **Rejected**: Incomplete SSR handling; still needs custom script

## Implementation Details

### ThemeProvider Configuration

Provider wraps the application in root layout with specific options:

- `attribute="class"`: Sets `.dark` class on `<html>` for Tailwind
- `defaultTheme="system"`: Respects OS preference by default
- `enableSystem`: Activates `prefers-color-scheme` detection
- `disableTransitionOnChange`: Prevents jarring color transitions during switch

### Hydration Safety Pattern

The layout applies `suppressHydrationWarning` to the `<html>` element:

```
html[suppressHydrationWarning] → ThemeProvider → App
```

This suppresses React warnings about server/client class mismatch since next-themes intentionally modifies the class before hydration.

### Toggle Component Pattern

The ThemeToggle component uses a mounted state pattern:

1. Server renders placeholder (static icon)
2. `useEffect` sets `mounted=true` on client
3. Only then renders interactive toggle with current theme
4. Prevents hydration mismatch in toggle UI

### Tailwind v4 Integration

Tailwind v4 defaults to `prefers-color-scheme` media query. For class-based dark mode with next-themes:

```css
@custom-variant dark (&:where(.dark, .dark *));
```

This enables `dark:` utilities to respond to the `.dark` class applied by next-themes.

### CSS Variables Theming

Light and dark themes are defined in `globals.css` using OKLCH color space:

| Mode  | Background   | Foreground   |
| ----- | ------------ | ------------ |
| Light | oklch(0.952) | oklch(0.25)  |
| Dark  | oklch(0.185) | oklch(0.950) |

Full color palette includes semantic tokens: primary, secondary, muted, accent, destructive, and status colors.

## Consequences

### Positive

**User Experience:**

- No flash of incorrect theme on page load
- Seamless system preference detection
- Persistent user choice across sessions
- Smooth theme transitions (when enabled)

**Developer Experience:**

- Two-line setup in ThemeProvider
- Standard `useTheme()` hook API
- No custom SSR handling required
- Tailwind `dark:` utilities work directly

**Ecosystem Alignment:**

- shadcn/ui official recommendation
- Vercel/Next.js community standard
- Extensive documentation and examples

### Negative

**Single Maintainer:**

- Library maintained by one developer
- **Mitigation**: Stable API, minimal updates needed; trivial to fork if abandoned

**Hydration Warning Suppression:**

- Must add `suppressHydrationWarning` to `<html>`
- **Mitigation**: Well-documented pattern; no actual hydration issues

**Animation Complexity:**

- Theme toggle animations require careful coordination with `setTheme`
- **Mitigation**: Delay theme change until animation completes

## References

### Internal

- [ADR-02: Next.js 16 + React 19 Selection](/en/adr/web/02-nextjs-react-selection.md)
- [ADR-05: shadcn/ui + Tailwind CSS Selection](/en/adr/web/05-shadcn-tailwind-selection.md)

### External

- [next-themes GitHub Repository](https://github.com/pacocoursey/next-themes)
- [next-themes npm Package](https://www.npmjs.com/package/next-themes)
- [shadcn/ui Dark Mode Documentation](https://ui.shadcn.com/docs/dark-mode/next)
- [Tailwind CSS Dark Mode](https://tailwindcss.com/docs/dark-mode)
