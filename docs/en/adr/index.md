---
title: ADR
description: Architecture Decision Records documenting key technical decisions in Specvital
---

# Architecture Decision Records (ADR)

> 🇰🇷 [한국어 버전](/ko/adr/)

Documentation of architectural decisions made in the Specvital project.

## What is an ADR?

An Architecture Decision Record (ADR) captures an important architectural decision made along with its context and consequences. ADRs help maintain decision history across multi-repository microservices.

## When to Write an ADR

| Category         | Examples                                                |
| ---------------- | ------------------------------------------------------- |
| Technology Stack | Framework selection, library adoption, version upgrades |
| Architecture     | Service boundaries, communication patterns, data flow   |
| API Design       | Endpoint structure, versioning strategy, error handling |
| Database         | Schema design, migration strategy, indexing approach    |
| Infrastructure   | Deployment platform, scaling strategy, monitoring       |
| Cross-cutting    | Security, performance optimization, observability       |

## Templates

| Template                     | Use Case                        |
| ---------------------------- | ------------------------------- |
| [template.md](./template.md) | Standard ADR for most decisions |

## Naming Convention

```
XX-brief-decision-title.md
```

- `XX`: Two-digit sequential number (01, 02, ...)
- Lowercase with hyphens
- Brief and descriptive titles

## Technical Areas

| Area           | Affected Repositories |
| -------------- | --------------------- |
| Parser         | core                  |
| API            | web                   |
| Worker         | collector             |
| Database       | infra                 |
| Infrastructure | infra                 |
| Cross-cutting  | multiple              |

## ADR Index

### Cross-cutting (All Repositories)

| #   | Title                                                                             | Area           | Date       |
| --- | --------------------------------------------------------------------------------- | -------------- | ---------- |
| 01  | [Static Analysis-Based Instant Analysis](./01-static-analysis-approach.md)        | Cross-cutting  | 2024-12-17 |
| 02  | [Competitive Differentiation Strategy](./02-competitive-differentiation.md)       | Cross-cutting  | 2024-12-17 |
| 03  | [API and Worker Service Separation](./03-api-worker-service-separation.md)        | Architecture   | 2024-12-17 |
| 04  | [Queue-Based Asynchronous Processing](./04-queue-based-async-processing.md)       | Architecture   | 2024-12-17 |
| 05  | [Polyrepo Repository Strategy](./05-repository-strategy.md)                       | Architecture   | 2024-12-17 |
| 06  | [PaaS-First Infrastructure Strategy](./06-paas-first-infrastructure.md)           | Infrastructure | 2024-12-17 |
| 07  | [Shared Infrastructure Strategy](./07-shared-infrastructure.md)                   | Infrastructure | 2024-12-17 |
| 08  | [External Repository ID-Based Data Integrity](./08-external-repo-id-integrity.md) | Data Integrity | 2024-12-22 |

### Core Repository

| #   | Title                                                                                          | Area    | Date       |
| --- | ---------------------------------------------------------------------------------------------- | ------- | ---------- |
| 01  | [Core Library Separation](./core/01-core-library-separation.md)                                | Core    | 2024-12-17 |
| 02  | [Dynamic Test Counting Policy](./core/02-dynamic-test-counting-policy.md)                      | Core    | 2024-12-22 |
| 03  | [Tree-sitter as AST Parsing Engine](./core/03-tree-sitter-ast-parsing-engine.md)               | Parser  | 2024-12-23 |
| 04  | [Early-Return Framework Detection](./core/04-early-return-framework-detection.md)              | Parser  | 2024-12-23 |
| 05  | [Parser Pooling Disabled](./core/05-parser-pooling-disabled.md)                                | Parser  | 2024-12-23 |
| 06  | [Unified Framework Definition](./core/06-unified-framework-definition.md)                      | Parser  | 2024-12-23 |
| 07  | [Source Abstraction Interface](./core/07-source-abstraction-interface.md)                      | Parser  | 2024-12-23 |
| 08  | [Shared Parser Modules](./core/08-shared-parser-modules.md)                                    | Parser  | 2024-12-23 |
| 09  | [Config Scope Resolution](./core/09-config-scope-resolution.md)                                | Config  | 2024-12-23 |
| 10  | [Standard Go Project Layout](./core/10-standard-go-project-layout.md)                          | Project | 2024-12-23 |
| 11  | [Integration Testing with Golden Snapshots](./core/11-integration-testing-golden-snapshots.md) | Testing | 2024-12-23 |
| 12  | [Parallel Scanning with Worker Pool](./core/12-parallel-scanning-worker-pool.md)               | Perf    | 2024-12-23 |
| 13  | [NaCl SecretBox Encryption](./core/13-nacl-secretbox-encryption.md)                            | Crypto  | 2024-12-23 |

### Collector Repository

| #   | Title                                                                                           | Area         | Date       |
| --- | ----------------------------------------------------------------------------------------------- | ------------ | ---------- |
| 01  | [Scheduled Re-collection Architecture](./collector/01-scheduled-recollection.md)                | Architecture | 2024-12-18 |
| 02  | [Clean Architecture Layer Introduction](./collector/02-clean-architecture-layers.md)            | Architecture | 2024-12-18 |
| 03  | [Graceful Shutdown and Context-Based Lifecycle Management](./collector/03-graceful-shutdown.md) | Architecture | 2024-12-18 |
| 04  | [OAuth Token Graceful Degradation](./collector/04-oauth-token-graceful-degradation.md)          | Reliability  | 2024-12-18 |
| 05  | [Worker-Scheduler Process Separation](./collector/05-worker-scheduler-separation.md)            | Architecture | 2024-12-18 |
| 06  | [Semaphore-Based Clone Concurrency Control](./collector/06-semaphore-clone-concurrency.md)      | Concurrency  | 2024-12-18 |
| 07  | [Repository Pattern Data Access Abstraction](./collector/07-repository-pattern.md)              | Architecture | 2024-12-18 |

### Web Repository

| #   | Title                                                     | Area       | Date       |
| --- | --------------------------------------------------------- | ---------- | ---------- |
| 01  | [Go as Backend Language](./web/01-go-backend-language.md) | Tech Stack | 2024-12-18 |

## Process

1. **Create**: Copy [template.md](./template.md) → `XX-title.md`
2. **Write**: Fill in all sections with finalized decision
3. **Localize**: Create Korean version in `kr/adr/`
4. **Review**: Submit PR for team review
5. **Merge**: Add to index after approval

## Related Repositories

- [specvital/core](https://github.com/specvital/core) - Parser engine
- [specvital/web](https://github.com/specvital/web) - Web platform
- [specvital/collector](https://github.com/specvital/collector) - Worker service
- [specvital/infra](https://github.com/specvital/infra) - Infrastructure
