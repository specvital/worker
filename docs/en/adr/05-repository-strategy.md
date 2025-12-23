---
title: Repository Strategy
description: ADR on polyrepo strategy for independent service deployment and technology freedom
---

# ADR-05: Polyrepo Repository Strategy

> [한국어 버전](/ko/adr/05-repository-strategy.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-17 | @KubrickCode | All   |

## Context

### Problem Statement

Multi-service platforms require a fundamental decision on code organization strategy. This decision impacts:

1. **Development Workflow**: How developers work across service boundaries
2. **Release Management**: How services are versioned and deployed
3. **Team Collaboration**: How ownership and responsibilities are defined
4. **Technical Evolution**: How technology choices can evolve independently

### Repository Strategy Options

| Strategy | Description                                     | Typical Use Case                      |
| -------- | ----------------------------------------------- | ------------------------------------- |
| Monorepo | All services in a single repository             | Tightly coupled services, same stack  |
| Polyrepo | Each service in its own repository              | Independent services, diverse stacks  |
| Hybrid   | Core in monorepo, specific components separated | Gradual migration, mixed requirements |

### Key Decision Factors

- **Technology Stack Diversity**: Different languages/frameworks across services
- **Release Independence**: Need for independent deployment cycles
- **Team Structure**: Ownership boundaries and access control requirements
- **Code Sharing Patterns**: Frequency of shared code changes
- **Open Source Strategy**: Plans to open-source specific components
- **Platform Distribution Requirements**: Marketplace and registry constraints

### Platform Distribution Constraints

Certain distribution platforms require or strongly prefer independent repositories:

| Distribution Target | Repository Requirement                  | Monorepo Limitation                             |
| ------------------- | --------------------------------------- | ----------------------------------------------- |
| GitHub Action       | `action.yml` must be at repository root | **Impossible** - cannot register on Marketplace |
| VSCode Extension    | `package.json` at root, vsce packaging  | Complex workspace configuration required        |
| Go Module           | `go.mod` path becomes import path       | Submodule paths are awkward (`repo/pkg`)        |
| npm Package         | Independent package management          | Possible but requires workspace tooling         |
| Docker Hub          | Dockerfile context at root preferred    | Multi-context builds add complexity             |

This constraint is particularly critical when planning ecosystem expansion (IDE extensions, CI integrations, CLI tools).

## Decision

**Adopt Polyrepo strategy with repositories separated by deployment unit and technology boundary.**

Repository separation criteria:

- **Deployment Unit**: Services that deploy independently get separate repositories
- **Technology Boundary**: Different primary languages/frameworks warrant separation
- **Ownership Boundary**: Clear team ownership maps to repository boundary

## Options Considered

### Option A: Polyrepo Strategy (Selected)

Each service maintains its own repository with independent lifecycle.

**Pros:**

- **Independent Release Cycles**: Deploy any service without coordinating with others
- **Clear Ownership**: Repository boundary equals ownership boundary
- **Technology Freedom**: Each service can use optimal stack without constraints
- **Granular Access Control**: Repository-level permissions for sensitive code
- **Open Source Enablement**: Open-source specific components without exposing entire codebase
- **Focused CI/CD**: Faster builds, only affected service tested
- **Simplified Dependencies**: No build system complexity for cross-language support

**Cons:**

- **Cross-Repository Changes**: Multi-repo PRs require coordination
- **Dependency Synchronization**: Library versions must be tracked across repos
- **Code Duplication Risk**: Common utilities may be duplicated
- **Development Setup**: Local environment requires multiple repository clones

### Option B: Monorepo Strategy

All services in a single repository with shared tooling.

**Pros:**

- **Atomic Changes**: Single PR can modify multiple services
- **Code Reuse**: Easy sharing of common code and utilities
- **Consistent Tooling**: Unified build system and CI/CD
- **Simplified Refactoring**: Large-scale changes easier to execute
- **Single Source of Truth**: All code in one place

**Cons:**

- **Build System Complexity**: Multi-language support requires sophisticated tooling (Bazel, Nx)
- **Scaling Challenges**: Repository size impacts clone/checkout time
- **Coupled Releases**: Changes may trigger unnecessary rebuilds
- **Access Control Limitations**: Fine-grained permissions are complex
- **Technology Lock-in**: Pressure to standardize on single stack
- **Platform Distribution Blockers**: GitHub Actions cannot be registered on Marketplace; VSCode extensions require complex setup

### Option C: Hybrid Approach

Core services in monorepo, specific components (libraries, infra) in separate repos.

**Pros:**

- **Best of Both**: Combine monorepo benefits for related services
- **Selective Separation**: Isolate components with special requirements

**Cons:**

- **Dual Complexity**: Must maintain both patterns and tooling
- **Boundary Decisions**: Unclear criteria for what belongs where
- **Migration Friction**: Moving code between mono and poly sections is complex

## Consequences

### Positive

1. **Independent Release Cycles**
   - Services deploy on their own schedule
   - No release coordination overhead
   - Rollback scope limited to single service

2. **Clear Ownership Boundaries**
   - Repository ownership = service ownership
   - Code review scope well-defined
   - Accountability clearly established

3. **Technology Flexibility**
   - Each service uses optimal technology
   - Framework/library upgrades don't cascade
   - Build systems optimized per-language

4. **Access Control**
   - Sensitive code (infrastructure, secrets) isolated
   - Contributor permissions per-repository
   - Audit trails separated

5. **Open Source Capability**
   - Library components publishable independently
   - Separate licensing possible
   - Community contributions scoped

6. **Ecosystem Distribution**
   - GitHub Actions registrable on Marketplace (requires root `action.yml`)
   - VSCode extensions publishable without complex workspace setup
   - Each tool/extension has clean repository identity

### Negative

1. **Cross-Repository Changes**
   - Multi-service features require coordinated PRs
   - API contract changes need careful sequencing
   - **Mitigation**: Stable API contracts, versioning strategy, feature flags

2. **Dependency Synchronization**
   - Shared library updates across repositories
   - Breaking changes must be communicated
   - **Mitigation**: Automated dependency updates (Renovate/Dependabot), semantic versioning

3. **Code Duplication**
   - Utility code may be duplicated
   - Common patterns reimplemented
   - **Mitigation**: Extract shared code to dedicated library repository

4. **Development Environment**
   - Multiple repositories to clone and configure
   - E2E testing requires service orchestration
   - **Mitigation**: Docker Compose for local development, documentation

### Technical Implications

| Aspect                | Implication                                                          |
| --------------------- | -------------------------------------------------------------------- |
| **Dependency Mgmt**   | Go modules for Go services, npm for TypeScript                       |
| **Version Strategy**  | SemVer for libraries, independent versioning for services            |
| **CI/CD**             | Per-repository pipelines, integration tests in dedicated environment |
| **Code Sharing**      | Publish shared libraries as packages (Go modules, npm packages)      |
| **Local Development** | Docker Compose or development scripts for multi-service setup        |

### When to Reconsider

- Cross-repository changes become majority of development work
- Team consolidation makes shared ownership more practical
- Technology stack converges to single language/framework
- Monorepo tooling (Bazel, Nx) becomes compelling for the scale
