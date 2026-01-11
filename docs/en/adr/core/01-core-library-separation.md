---
title: Core Library
description: ADR on extracting core library as independent Go module for reuse
---

# ADR-01: Core Library as Independent Repository

> 🇰🇷 [한국어 버전](/ko/adr/core/01-core-library-separation.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-17 | @KubrickCode | core  |

## Context

### Problem Statement

The core library is a shared capability that multiple services need to consume:

1. **Worker Service (worker)**: Processes analysis jobs from queue
2. **API Service (web)**: May need direct access for sync operations
3. **CLI Tool**: Developers want to run locally
4. **Docker Image**: CI/CD pipelines need containerized execution

### Strategic Question

Should the core library be embedded within a service or extracted as an independent, reusable library?

## Decision

**Extract the core library as an independent Go library in a separate repository.**

The core is published as a Go module that can be:

- Imported by any Go service (`go get`)
- Distributed as a CLI binary
- Packaged as a Docker image

## Options Considered

### Option A: Independent Library (Selected)

Separate repository with the core as a reusable Go module.

**Pros:**

- **Multiple Deployment Modes**: Single codebase serves library, CLI, and Docker use cases
- **Independent Release Cycle**: Core fixes don't require service redeployment
- **Open Source Enablement**: Community can use and contribute to the core
- **Clear API Contract**: Forces well-defined boundaries between core and consumers
- **Ecosystem Value**: Provides standalone value beyond the platform

**Cons:**

- Version coordination across consuming services
- API stability commitment required
- Separate CI/CD pipeline maintenance

### Option B: Service-Internal Module

Core code lives inside a consuming service (e.g., worker).

**Pros:**

- Simpler initial setup
- No cross-repository coordination
- Faster iteration without versioning concerns

**Cons:**

- **Code Duplication**: Other services needing core must duplicate code
- **No Standalone Use**: Cannot offer CLI or Docker without additional work
- **Tight Coupling**: Core changes tied to service release cycle
- **Limited Reuse**: External parties cannot consume the core

### Option C: Shared Source (Git Submodule)

Share core code via Git submodule across repositories.

**Pros:**

- Source-level sharing without publishing

**Cons:**

- Complex Git workflows
- No independent versioning
- Poor tooling support
- Not publishable as library

## Consequences

### Positive

1. **Open Source Strategy**
   - Core can be MIT licensed separately
   - Builds trust through transparency
   - Enables community contributions for new frameworks

2. **Flexible Consumption**
   - Services import as Go module
   - Developers run CLI locally
   - CI/CD uses Docker image
   - All from single source

3. **Independent Evolution**
   - Core team can release bug fixes immediately
   - Consumers upgrade at their own pace
   - Breaking changes communicated via semver

4. **Ecosystem Contribution**
   - Standalone tool has value beyond platform
   - Potential for adoption outside organization

### Negative

1. **Coordination Overhead**
   - Must track which service uses which core version
   - **Mitigation**: Automated dependency updates (Dependabot)

2. **API Stability Burden**
   - Public API changes require careful planning
   - **Mitigation**: Conservative API surface, extension points

3. **Maintenance Cost**
   - Separate repository, CI/CD, documentation
   - **Mitigation**: Standardized tooling across repositories

### Affected Repositories

| Repository | Role         | Impact                         |
| ---------- | ------------ | ------------------------------ |
| **core**   | Core library | Primary - defines public API   |
| **worker** | Consumer     | Imports core as dependency     |
| **web**    | Consumer     | May import for sync operations |
