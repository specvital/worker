---
title: Tech Radar
description: Technology adoption status and evaluation across the platform
---

# Tech Radar

> 🇰🇷 [한국어 버전](/ko/tech-radar.md)

Technology adoption status across the Specvital platform.

## Ring Definitions

| Ring       | Description                                    |
| ---------- | ---------------------------------------------- |
| **Adopt**  | Proven in production, recommended for new work |
| **Trial**  | Being tested in limited scope                  |
| **Assess** | Worth exploring, not yet in active use         |
| **Hold**   | Not recommended for new projects               |

## Quadrants

### Languages & Runtimes

| Technology | Ring  | Version | Notes           |
| ---------- | ----- | ------- | --------------- |
| Go         | Adopt | 1.24    | Primary backend |
| TypeScript | Adopt | 5.8     | Frontend        |
| Node.js    | Adopt | 22 LTS  | Tooling runtime |

### Backend

| Technology     | Ring  | Version | Notes                       |
| -------------- | ----- | ------- | --------------------------- |
| Chi            | Adopt | 5.2     | HTTP router                 |
| Tree-sitter    | Adopt | -       | Multi-language parser       |
| River          | Adopt | -       | PostgreSQL-based task queue |
| pgx            | Adopt | -       | PostgreSQL driver           |
| sqlc           | Adopt | -       | Type-safe SQL               |
| oapi-codegen   | Adopt | -       | OpenAPI code generation     |
| testcontainers | Adopt | -       | Integration testing         |

### Frontend

| Technology       | Ring  | Version | Notes               |
| ---------------- | ----- | ------- | ------------------- |
| Next.js          | Adopt | 16.0    | React framework     |
| React            | Adopt | 19.1    | UI library          |
| TanStack Query   | Adopt | -       | Server state        |
| TanStack Virtual | Adopt | -       | Virtual scrolling   |
| Tailwind CSS     | Adopt | 4.1     | Utility-first CSS   |
| Radix UI         | Adopt | -       | Headless components |
| Zod              | Adopt | -       | Schema validation   |
| Vitest           | Adopt | -       | Unit testing        |

### Platform & Infrastructure

| Technology     | Ring  | Notes                    |
| -------------- | ----- | ------------------------ |
| Railway        | Adopt | Application hosting      |
| NeonDB         | Adopt | Serverless PostgreSQL 16 |
| Docker         | Adopt | Containerization         |
| GitHub Actions | Adopt | CI/CD                    |
| Dev Containers | Adopt | Development environment  |

### Development Tools

| Technology       | Ring  | Notes              |
| ---------------- | ----- | ------------------ |
| pnpm             | Adopt | Package manager    |
| Just             | Adopt | Task runner        |
| Air              | Adopt | Go hot reload      |
| Atlas            | Adopt | Schema migration   |
| Prettier         | Adopt | Code formatter     |
| Husky            | Adopt | Git hooks          |
| semantic-release | Adopt | Automated versions |

### AI Tooling

| Technology  | Ring  | Notes                  |
| ----------- | ----- | ---------------------- |
| Claude Code | Adopt | AI coding assistant    |
| MCP         | Adopt | Model Context Protocol |
| Gemini      | Adopt | AI coding assistant    |

## Principles

1. **Type Safety**: Compile-time validation (Go, TypeScript, sqlc)
2. **Simplicity**: Well-maintained, focused libraries
3. **Avoid Lock-in**: Standard protocols, portable solutions
4. **DX Priority**: Hot reload, type generation

## Related Documents

- [Tech Stack (PRD)](./prd/06-tech-stack.md)
- [ADR Index](./adr/)
