---
title: shadcn/ui + Tailwind CSS Selection
description: ADR on choosing shadcn/ui with Tailwind CSS for UI component library
---

# ADR-05: shadcn/ui + Tailwind CSS Selection

> [한국어 버전](/ko/adr/web/05-shadcn-tailwind-selection.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-12-04 | @KubrickCode | web   |

## Context

### The UI Component Library Question

The web platform requires a UI component library for building accessible, consistent, and maintainable interfaces. Key requirements:

1. **AI-Assisted Development**: Optimized for Claude, Cursor, v0, and AI coding workflows
2. **React 19 Compatibility**: Full support for latest React features
3. **Server Components**: Work seamlessly with Next.js 16 App Router RSC patterns
4. **Accessibility**: WCAG-compliant components out of the box
5. **Customization**: Full control over styling and behavior
6. **Dark Mode**: System preference detection with manual toggle

### Architectural Constraints

- **BFF Pattern**: Thin presentation layer; UI library should not add backend complexity
- **TypeScript**: Strong typing for component props and variants
- **Tailwind CSS**: Already selected for utility-first styling approach
- **Vercel Deployment**: Edge-optimized bundle sizes matter

### Candidates Evaluated

1. **shadcn/ui + Tailwind CSS**: Copy-paste components built on Radix UI
2. **MUI (Material UI)**: Google's Material Design implementation
3. **Chakra UI v3**: Zero-runtime CSS-in-JS with semantic tokens
4. **Ant Design v5**: Enterprise-focused component library
5. **Headless UI**: Tailwind Labs' unstyled accessible components

## Decision

**Adopt shadcn/ui with Tailwind CSS as the AI-native UI component library for maximum AI-assisted development productivity.**

Core principles:

1. **AI-Ready Architecture**: Open code for LLMs to read, understand, and improve
2. **Code Ownership**: Copy components into project; no external runtime dependency
3. **Radix Foundation**: Leverage battle-tested accessibility primitives
4. **CSS Variables Theming**: OKLCH-based design tokens in `globals.css`
5. **Server Component First**: Components compatible with RSC patterns

## Options Considered

### Option A: shadcn/ui + Tailwind CSS (Selected)

**How It Works:**

- Components copied into project via CLI (`pnpm dlx shadcn@latest add [component]`)
- Built on Radix UI primitives for accessibility
- Styled with Tailwind CSS utility classes
- Theming via CSS variables and `class-variance-authority` (cva)

**Pros:**

- **AI-Native Stack**: Vercel v0 generates exclusively shadcn/ui + Tailwind; industry-standard for AI coding
- **LLM-Optimized Styling**: Tailwind's inline utilities = complete context in single file; no hidden CSS cascade
- **Full Code Ownership**: Components in your repo; AI can read, understand, and modify freely
- **Radix Accessibility**: WAI-ARIA compliant, keyboard navigation, screen reader tested
- **React 19 Native**: First-class support; uses `data-slot` attributes, no deprecated APIs
- **Tailwind v4 Ready**: CSS variables, `@theme` directive, OKLCH color support
- **RSC Compatible**: Components work in Server Component patterns
- **Zero Bundle Overhead**: No library runtime; only ship code you use (2.3 KB initial vs 80+ KB for MUI)

**Cons:**

- Manual updates required (copy new versions selectively)
- Fewer pre-built complex components than enterprise libraries
- Team must understand Tailwind CSS paradigm

### Option B: MUI (Material UI v6)

**How It Works:**

- Install as npm dependency with Emotion CSS-in-JS
- 50+ pre-built Material Design components
- Theme object configuration with `createTheme()`

**Evaluation:**

- **Bundle Size**: 80-90 KB gzipped core; single Button adds 91.7 KB initial JS
- **RSC Incompatible**: Emotion uses React Context; all components must be Client Components
- **AI-Unfriendly**: CSS-in-JS requires AI to track styles across separate runtime context
- **Material Design Lock-in**: Opinionated aesthetic difficult to escape
- **Barrel Import Issues**: Development performance degradation without path imports
- **Rejected**: AI workflow friction; RSC incompatibility; design system lock-in

### Option C: Chakra UI v3

**How It Works:**

- Zero-runtime CSS-in-JS (migrated from runtime in v3)
- Semantic tokens with automatic dark/light mode
- Recipes system inspired by Panda CSS

**Evaluation:**

- **Bundle Size**: ~50 KB gzipped (improved from v2)
- **v3 Breaking Changes**: Major migration from v2; ecosystem still stabilizing
- **RSC**: Components can be imported but hydrate as Client Components
- **next-themes Required**: Dark mode moved from built-in to external
- **Rejected**: Migration instability; smaller ecosystem than shadcn/ui

### Option D: Ant Design v5

**How It Works:**

- 60+ enterprise-focused components
- Design tokens with `@ant-design/cssinjs`
- ConfigProvider for theming and i18n

**Evaluation:**

- **Bundle Size**: 126 KB+ for single component; ConfigProvider prevents tree-shaking
- **Enterprise Features**: Pro components (ProTable, ProForm) available
- **Built-in i18n**: 50+ locales out of the box
- **Opinionated Design**: Strong Ant aesthetic difficult to customize
- **RSC Incompatible**: CSS-in-JS architecture requires Client Components
- **Rejected**: Massive bundle; limited customization; RSC incompatibility

### Option E: Headless UI + Tailwind

**How It Works:**

- 16 unstyled accessible components from Tailwind Labs
- Style with Tailwind utility classes
- Framework support for React and Vue

**Evaluation:**

- **Component Gap**: Only 16 components vs Radix's 32+
- **Missing**: Accordion, Context Menu, Toast, Slider, Tooltip, Progress
- **Less Advanced**: Focus trapping and scroll locking less robust than Radix
- **Rejected**: Insufficient component coverage; Radix superiority for accessibility

## Implementation Details

### Component Adoption

| Component                   | Purpose                     |
| --------------------------- | --------------------------- |
| Button, Input               | Initial form implementation |
| Card, Badge, Tooltip, Alert | Dashboard UI foundation     |
| Dialog, Dropdown, Tabs      | Filter and navigation       |
| Scroll Area                 | Horizontal scroll UI        |
| Sheet, Command              | Mobile navigation           |

### Theming System

- CSS variables defined in `globals.css` with OKLCH color space
- Dark mode via `next-themes` with system detection
- Type-safe variants using `class-variance-authority` (cva)

### Radix UI Dependencies

Current primitives in use:

| Package                         | Component     |
| ------------------------------- | ------------- |
| `@radix-ui/react-checkbox`      | Checkbox      |
| `@radix-ui/react-dialog`        | Dialog, Sheet |
| `@radix-ui/react-dropdown-menu` | Dropdown Menu |
| `@radix-ui/react-popover`       | Popover       |
| `@radix-ui/react-scroll-area`   | Scroll Area   |
| `@radix-ui/react-tabs`          | Tabs          |
| `@radix-ui/react-toggle`        | Toggle        |
| `@radix-ui/react-tooltip`       | Tooltip       |

## AI-Assisted Development Synergy

### Why This Stack Is AI-Native

The combination of shadcn/ui + Tailwind CSS represents the emerging industry standard for AI-assisted development. This is the **primary strategic reason** for this technology choice.

### Vercel v0 Validation

Vercel's flagship AI product v0 generates **exclusively** shadcn/ui + Tailwind code:

> "v0 is trained on best practices for React, Tailwind and shadcn/ui. Every component v0 generates uses React, Next.js, Tailwind CSS, and shadcn/ui."
> — [Vercel Official Blog](https://vercel.com/blog/announcing-v0-generative-ui)

Teams using v0 report **3x faster** design-to-implementation when their design systems are built on shadcn/ui.

### Tailwind: "Ugly" for Humans, Perfect for AI

Traditional CSS requires AI to track relationships across multiple files:

| Semantic CSS Challenge       | Tailwind Solution        | AI Benefit                   |
| ---------------------------- | ------------------------ | ---------------------------- |
| CSS cascade side effects     | Self-contained utilities | No unexpected inheritance    |
| Separate file dependencies   | Inline declarations      | Complete context in one file |
| Creative class naming needed | Standard vocabulary      | Consistent generation        |
| Hidden style relationships   | Explicit per-element     | Predictable modifications    |

> "TailwindCSS's utility-first philosophy is like a playground for AI. Instead of crafting intricate CSS rules from scratch, AI can tap into Tailwind's vast library of pre-defined classes."
> — [DEV Community](https://dev.to/brolag/tailwindcss-a-game-changer-for-ai-driven-code-generation-and-design-systems-18m7)

> "Tailwind serves as a really effective styling mechanism that AIs are actually really good at using."
> — [Glide Blog](https://www.glideapps.com/blog/tailwind-css)

### shadcn/ui: Designed for LLMs

From shadcn/ui's official design principles:

> "AI-Ready: Open code for LLMs to read, understand, and improve."
> — [shadcn.io](https://www.shadcn.io/)

**Why copy-paste works for AI:**

- **Full Context Visibility**: AI sees complete component source, not abstracted API
- **No Library Hallucination**: No guessing props that don't exist
- **Unlimited Customization**: AI can modify any line without library constraints
- **MCP Integration**: Official shadcn MCP server provides real-time component specs to Claude, Cursor, VS Code

### Comparison with CSS-in-JS for AI

| Approach              | AI Code Generation              | Context Requirements     |
| --------------------- | ------------------------------- | ------------------------ |
| **Tailwind CSS**      | Excellent - inline, predictable | Single file              |
| **styled-components** | Poor - runtime context needed   | Multiple files + runtime |
| **CSS Modules**       | Moderate - separate files       | CSS + JSX files          |

> "CSS Modules are less AI-optimized in suggestions."
> — [Superflex AI Blog](https://www.superflex.ai/blog/css-modules-vs-styled-components-vs-tailwind)

### AI Productivity Metrics

| Metric                    | shadcn/ui + Tailwind   | Traditional Libraries     |
| ------------------------- | ---------------------- | ------------------------- |
| Design-to-implementation  | 3x faster with v0      | Baseline                  |
| AI suggestion accuracy    | 95%+ for Tailwind      | 60-70% for CSS-in-JS      |
| Component customization   | Immediate (code owned) | Wrapper/override patterns |
| Context window efficiency | Single file            | Multi-file tracking       |

## Consequences

### Positive

**AI Development Productivity:**

- 3x faster design-to-implementation with AI tools (v0, Claude, Cursor)
- Industry-standard stack for AI coding; extensive training data
- AI can freely modify owned code without library API constraints
- Tailwind's explicitness eliminates AI hallucination about hidden styles

**Customization Freedom:**

- Full source code ownership; modify any component behavior
- No waiting for library updates to fix issues
- Tailwind utility classes for rapid style iteration
- AI excels at Tailwind customization due to predictable class patterns

**Bundle Efficiency:**

- 2.3 KB initial JS vs 80+ KB for MUI or 126 KB+ for Ant Design
- Only used components are bundled; no library runtime
- Faster First Contentful Paint (0.8s vs 1.6s with heavy libraries)

**Accessibility by Default:**

- Radix UI handles ARIA attributes, focus management, keyboard navigation
- WAI-ARIA compliant without accessibility expertise
- Screen reader tested primitives

**React 19 Alignment:**

- No deprecated `forwardRef` usage
- `data-slot` attributes for styling hooks
- Compatible with React Compiler optimization

### Negative

**Update Overhead:**

- Must manually copy updated components when upstream changes
- No automatic security patches from npm update
- **Mitigation**: Pin Radix versions; selective component updates; monitor shadcn/ui releases

**Component Coverage:**

- Fewer pre-built complex components than Ant Design
- May need to build custom components for advanced use cases
- **Mitigation**: Extend with additional Radix primitives; community contributions

**Team Learning:**

- Requires Tailwind CSS proficiency
- Understanding of variant patterns with cva
- **Mitigation**: Tailwind documentation; pair programming; component documentation

### Bundle Size Comparison

| Library       | Initial Bundle | Notes                              |
| ------------- | -------------- | ---------------------------------- |
| shadcn/ui     | 2.3 KB         | Only copied components             |
| Chakra UI v3  | ~50 KB         | Zero-runtime improvement           |
| MUI v6        | ~80-90 KB      | Core + Emotion                     |
| Ant Design v5 | 126 KB+        | ConfigProvider blocks tree-shaking |

### Component Availability

| Feature         | shadcn/ui    | MUI      | Chakra      | Ant Design     |
| --------------- | ------------ | -------- | ----------- | -------------- |
| Component Count | 40+          | 50+      | 30+         | 60+            |
| Accessibility   | Radix (WCAG) | WCAG     | WCAG        | WCAG           |
| Dark Mode       | next-themes  | Built-in | next-themes | ConfigProvider |
| i18n            | External     | External | External    | Built-in       |
| RSC Compatible  | Yes          | No       | Partial     | No             |

## References

### Internal

- [ADR-02: Next.js 16 + React 19 Selection](/en/adr/web/02-nextjs-react-selection.md)
- [ADR-06: PaaS-First Infrastructure](/en/adr/06-paas-first-infrastructure.md)

### External

- [shadcn/ui Documentation](https://ui.shadcn.com/)
- [shadcn/ui Tailwind v4 Support](https://ui.shadcn.com/docs/tailwind-v4)
- [Radix UI Primitives](https://www.radix-ui.com/primitives)
- [Tailwind CSS v4](https://tailwindcss.com/)
- [class-variance-authority](https://cva.style/docs)
- [next-themes](https://github.com/pacocoursey/next-themes)

### AI Development Resources

- [Announcing v0: Generative UI - Vercel](https://vercel.com/blog/announcing-v0-generative-ui)
- [TailwindCSS: A Game-Changer for AI-Driven Code Generation - DEV Community](https://dev.to/brolag/tailwindcss-a-game-changer-for-ai-driven-code-generation-and-design-systems-18m7)
- [What is Tailwind CSS, and why is it important for AI coding? - Glide](https://www.glideapps.com/blog/tailwind-css)
- [shadcn MCP Documentation](https://ui.shadcn.com/docs/mcp)
- [The AI-Native shadcn/ui Component Library](https://www.shadcn.io/)
