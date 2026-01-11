---
title: Feature-Based Module Organization
description: ADR on adopting feature-based vertical slice architecture for both backend and frontend codebases
---

# ADR-11: Feature-Based Module Organization

> [Korean Version](/ko/adr/web/11-feature-based-module-organization.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2025-01-03 | @KubrickCode | web   |

## Context

### Initial Structure Challenges

Both backend and frontend codebases initially followed flat, layer-based organization:

**Backend (Before):**

```
src/backend/
├── analyzer/
│   ├── handler.go
│   ├── service.go
│   └── types.go
├── github/
└── health/
```

**Frontend (Before):**

```
src/frontend/
├── components/       # 16 files flat, mixed domains
└── types/
```

**Problems Identified:**

- **Low cohesion**: Unrelated files grouped by technical layer
- **High coupling**: Changes to one feature scattered across directories
- **Unclear boundaries**: Difficult to identify what belongs to which domain
- **Poor scalability**: Adding features required touching multiple unrelated directories
- **Navigation overhead**: Developers needed to jump between distant folders

### Alignment with Clean Architecture

This decision complements [ADR-08: Clean Architecture Pattern](/en/adr/web/08-clean-architecture-pattern.md). While Clean Architecture defines the layer structure within a module, Feature-Based Organization defines how modules are grouped at the top level.

## Decision

**Adopt Feature-Based Module Organization for both backend and frontend, with Clean Architecture layers within each module.**

### Backend Structure: `modules/{module}/`

Each domain module contains its own Clean Architecture layers:

```
modules/{module}/
├── domain/
│   ├── entity/      # Business entities
│   └── port/        # Interface definitions
├── usecase/         # Business logic
├── adapter/         # External implementations
└── handler/         # HTTP entry points
```

**Current modules:** `analyzer`, `auth`, `github`, `github-app`, `user`

### Frontend Structure: `features/{feature}/`

Each feature is a self-contained unit with its own internal organization:

```
features/{feature}/
├── components/      # UI components
├── hooks/           # React hooks
├── api/             # API calls
├── types/           # TypeScript types
└── index.ts         # Barrel export (public API)
```

**Current features:** `analysis`, `auth`, `dashboard`, `home`

### Shared Code Organization

Code that spans multiple modules is placed in dedicated shared directories:

| Location                | Purpose                                   |
| ----------------------- | ----------------------------------------- |
| Backend: `common/`      | Middleware, health checks, shared clients |
| Frontend: `components/` | Layout, theme, feedback components        |
| Frontend: `lib/`        | API client, utilities, styles             |

## Options Considered

### Option A: Feature-Based (Vertical Slice) Organization (Selected)

**How It Works:**

- Top-level directories represent business domains
- Each domain contains all technical layers internally
- Follows "Screaming Architecture" principle

**Pros:**

- **High cohesion**: Related code lives together
- **Minimal coupling**: Modules are largely independent
- **Easy deletion**: Remove a feature by deleting one directory
- **Clear ownership**: Teams can own entire features
- **Natural microservice boundaries**: Extraction is straightforward
- **AI-friendly**: Bounded context fits within LLM context windows

**Cons:**

- Shared code requires explicit decisions about placement
- Initial setup requires more structure

### Option B: Layer-Based (Horizontal) Organization

**How It Works:**

- Top-level directories represent technical layers
- All controllers in one folder, all services in another

**Example:**

```
src/backend/
├── controllers/
│   ├── analyzer.go
│   ├── auth.go
│   └── user.go
├── services/
│   ├── analyzer.go
│   ├── auth.go
│   └── user.go
└── repositories/
    └── ...
```

**Pros:**

- Simple initial structure
- Clear layer separation
- Common in traditional frameworks

**Cons:**

- **Low cohesion**: Unrelated files grouped by type
- **High coupling**: Feature changes span multiple directories
- **Difficult extraction**: Microservice migration requires major restructuring
- **Navigation overhead**: Developers must jump between distant folders
- **Large context**: AI tools struggle with scattered code

### Option C: Flat Component Organization

**How It Works:**

- All components in a single directory
- Minimal folder hierarchy

**Pros:**

- Zero organizational overhead
- Works for small projects

**Cons:**

- Becomes unmanageable at scale
- No clear domain boundaries
- Difficult to navigate with many files

## Implementation

### Barrel Export Pattern

Each feature exposes a public API via `index.ts`, enabling clean imports like `@/features/analysis`.

### Cross-Module Dependencies

| Stack    | Approach                               |
| -------- | -------------------------------------- |
| Backend  | Use port interfaces from other modules |
| Frontend | Import through barrel exports          |

## Consequences

### Positive

**Developer Experience:**

- Clear mental model: "Where is user bookmark logic?" → `modules/user/usecase/bookmark/`
- Faster navigation: All related code in one location
- Easier onboarding: New developers can focus on single feature

**Code Quality:**

- High cohesion within modules
- Low coupling between modules
- Natural boundaries for code reviews

**Scalability:**

- Adding features: Create new directory with standard structure
- Team scaling: Assign team ownership per module
- Microservice extraction: Module boundaries are already defined

**AI-Assisted Development:**

- Each feature fits within LLM context windows
- Clear boundaries for AI agents to work within
- Reduced cross-file dependency scanning

### Negative

**Shared Code Decisions:**

- Must decide whether code belongs to a feature or shared location
- **Mitigation**: Default to feature-specific, extract when reused 3+ times

**Initial Learning Curve:**

- New team members must understand the module structure
- **Mitigation**: CLAUDE.md documents the pattern; consistent structure across modules

**Potential Duplication:**

- Similar patterns may exist in multiple modules
- **Mitigation**: Extract to `common/` or `lib/` when duplication is identified

## References

- [Screaming Architecture - Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2011/09/30/Screaming-Architecture.html)
- [Vertical Slice Architecture - Jimmy Bogard](https://www.jimmybogard.com/vertical-slice-architecture/)
- [Colocation - Kent C. Dodds](https://kentcdodds.com/blog/colocation)
- [Practical DDD in Golang: Module](https://www.ompluscator.com/article/golang/practical-ddd-module/)
- [React Folder Structure - Robin Wieruch](https://www.robinwieruch.de/react-folder-structure/)
- [NestJS Modules Documentation](https://docs.nestjs.com/modules)
- [ADR-08: Clean Architecture Pattern](/en/adr/web/08-clean-architecture-pattern.md)
