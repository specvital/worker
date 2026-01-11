---
title: OKLCH Design Token System with Cloud Dancer Theme
description: ADR on adopting OKLCH color space with Pantone Cloud Dancer-inspired warm tones
---

# ADR-19: OKLCH Design Token System with Cloud Dancer Theme

> 🇰🇷 [한국어 버전](/ko/adr/web/19-css-variable-design-token-system.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-20 | @KubrickCode | web   |

## Context

### Default Theme Limitations

The shadcn/ui default theme uses neutral grayscale colors with zero chroma, resulting in a cold, clinical appearance that lacks visual warmth and personality.

### Dark/Light Mode Inconsistency

Hardcoded Tailwind color classes (e.g., `text-green-600`, `bg-yellow-500`) created inconsistent appearances between light and dark modes. Colors that worked well in light mode appeared too bright or washed out in dark mode.

### Color Space Limitations

Traditional color spaces (RGB, HSL) have fundamental limitations:

- **RGB/Hex**: Difficult to manipulate programmatically for palette generation
- **HSL**: Perceptually non-uniform - equal lightness values appear differently across hues

### Test Status Visualization

The application displays test results with multiple status states (active, focused, skipped, todo, xfail), requiring a systematic approach to status-specific colors that work consistently across themes.

## Decision

**Adopt OKLCH color space with CSS variables as the design token system for consistent, accessible theming.**

Core principles:

1. **OKLCH Color Space**: Use perceptually uniform color representation
2. **Warm Palette**: Apply Pantone 2026 Cloud Dancer-inspired warm undertones
3. **Semantic Variables**: Define purpose-based color tokens instead of spectrum-based
4. **Tailwind Integration**: Expose tokens via @theme directive for utility class generation

### Token Categories

| Category   | Variables                                                                | Purpose                      |
| ---------- | ------------------------------------------------------------------------ | ---------------------------- |
| **Core**   | background, foreground, card, popover, primary, secondary, muted, accent | Base UI surfaces and text    |
| **State**  | destructive, border, input, ring                                         | Interactive element states   |
| **Chart**  | chart-1 through chart-5                                                  | Data visualization           |
| **Status** | status-active, status-focused, status-skipped, status-todo, status-xfail | Test result indicators       |
| **Custom** | input-bg, hero-gradient-center, hero-gradient-edge                       | Component-specific overrides |

### Color System Parameters

| Parameter     | Light Mode    | Dark Mode     | Purpose                   |
| ------------- | ------------- | ------------- | ------------------------- |
| **Lightness** | 0.885 - 0.975 | 0.185 - 0.320 | Base brightness level     |
| **Chroma**    | 0.004 - 0.010 | 0.010 - 0.014 | Subtle warmth saturation  |
| **Hue**       | 95° - 98°     | 95° - 98°     | Warm beige/sand direction |

## Options Considered

### Option A: OKLCH Color Space (Selected)

OKLCH uses Lightness, Chroma, and Hue with perceptual uniformity based on the Oklab color model.

**Pros:**

- **Perceptual Uniformity**: Equal numerical steps produce equal visual changes
- **Consistent Lightness**: Same L value appears equally bright across all hues
- **Wide Gamut Support**: Native Display P3 and beyond color representation
- **Predictable Accessibility**: Reliable WCAG contrast ratio calculations
- **Better Gradients**: No muddy intermediate colors in color interpolation
- **Browser Support**: 92%+ global support as of 2025

**Cons:**

- Learning curve for developers unfamiliar with OKLCH
- Out-of-gamut values require clipping awareness
- Legacy browser fallback needed (<8% usage)

### Option B: HSL Variables

**How It Works:**

CSS variables with HSL values, leveraging its intuitive Hue-Saturation-Lightness model.

**Evaluation:**

- More intuitive hue selection (0-360° color wheel)
- **Rejected**: Perceptually non-uniform - yellow appears brighter than blue at same L value
- Inconsistent palette generation results

### Option C: RGB/Hex with Tailwind Palette

**How It Works:**

Use standard Tailwind color palette (slate, gray, zinc) with hardcoded hex values.

**Evaluation:**

- Zero configuration, works out of box
- **Rejected**: No theme customization, cold neutral appearance
- Difficult to maintain dark/light mode parity

## Implementation

### @theme Directive Integration

Tokens are exposed to Tailwind via the @theme inline directive:

```css
@theme inline {
  --color-background: var(--background);
  --color-status-active: var(--status-active);
  /* ... */
}
```

This enables utility class generation (`bg-background`, `text-status-active`) while maintaining single source of truth in CSS variables.

### Theme Mode Strategy

Light and dark modes share identical hue angles (95°-98°) but invert lightness and adjust chroma:

- **Light**: High lightness (0.9+), low chroma (0.004-0.010)
- **Dark**: Low lightness (0.2-0.3), slightly higher chroma (0.010-0.014) for visibility

### Status Color Mapping

| Status  | Light Mode Hue | Dark Mode L Adjustment | Semantic Meaning       |
| ------- | -------------- | ---------------------- | ---------------------- |
| active  | 145° (green)   | +0.10                  | Passing/successful     |
| focused | 310° (magenta) | +0.10                  | Currently selected     |
| skipped | 85° (yellow)   | +0.07                  | Intentionally bypassed |
| todo    | 240° (blue)    | +0.10                  | Pending implementation |
| xfail   | 25° (orange)   | +0.08                  | Expected failure       |

## Consequences

### Positive

**Visual Consistency:**

- Unified warm appearance across all components
- Seamless light/dark mode transitions
- Professional, approachable aesthetic

**Developer Experience:**

- Semantic naming reduces cognitive load (`text-status-active` vs `text-green-600`)
- Single source of truth for color modifications
- Tailwind utility classes auto-generated from tokens

**Accessibility:**

- Predictable contrast ratios due to perceptual uniformity
- Status colors maintain distinction in both modes
- Consistent reading experience across themes

**Future-Proofing:**

- Wide gamut ready for HDR displays
- Tailwind v4 native OKLCH alignment

### Negative

**Learning Curve:**

- OKLCH less familiar than HSL/RGB
- **Mitigation**: Document common values and provide conversion tools

**Browser Compatibility:**

- ~8% browsers lack OKLCH support
- **Mitigation**: Acceptable for target audience (developers using modern browsers)

**Gamut Clipping:**

- Some OKLCH values exceed sRGB display capabilities
- **Mitigation**: All production values tested within sRGB gamut

## References

- [Pantone Color of the Year 2026: Cloud Dancer](https://www.pantone.com/articles/press-releases/pantone-announces-color-of-the-year-2026-cloud-dancer)
- [OKLCH in CSS: Why We Moved from RGB and HSL](https://evilmartians.com/chronicles/oklch-in-css-why-quit-rgb-hsl)
- [Tailwind CSS v4.0 Release](https://tailwindcss.com/blog/tailwindcss-v4)
- [MDN: oklch()](https://developer.mozilla.org/en-US/docs/Web/CSS/color_value/oklch)
