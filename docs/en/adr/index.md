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
| Worker         | worker                |
| Database       | infra                 |
| Infrastructure | infra                 |
| Cross-cutting  | multiple              |

## ADR Index

### Cross-cutting (All Repositories)

| #   | Title                                                                                  | Area           | Date       |
| --- | -------------------------------------------------------------------------------------- | -------------- | ---------- |
| 01  | [Static Analysis-Based Instant Analysis](./01-static-analysis-approach.md)             | Cross-cutting  | 2024-12-17 |
| 02  | [Competitive Differentiation Strategy](./02-competitive-differentiation.md)            | Cross-cutting  | 2024-12-17 |
| 03  | [API and Worker Service Separation](./03-api-worker-service-separation.md)             | Architecture   | 2024-12-17 |
| 04  | [Queue-Based Asynchronous Processing](./04-queue-based-async-processing.md)            | Architecture   | 2024-12-17 |
| 05  | [Polyrepo Repository Strategy](./05-repository-strategy.md)                            | Architecture   | 2024-12-17 |
| 06  | [PaaS-First Infrastructure Strategy](./06-paas-first-infrastructure.md)                | Infrastructure | 2024-12-17 |
| 07  | [Shared Infrastructure Strategy](./07-shared-infrastructure.md)                        | Infrastructure | 2024-12-17 |
| 08  | [External Repository ID-Based Data Integrity](./08-external-repo-id-integrity.md)      | Data Integrity | 2024-12-22 |
| 09  | [GitHub App Integration Strategy](./09-github-app-integration.md)                      | Authentication | 2024-12-29 |
| 10  | [TestStatus Data Contract](./10-test-status-data-contract.md)                          | Data Integrity | 2024-12-29 |
| 11  | [Repository Visibility-Based Access Control](./11-community-private-repo-filtering.md) | Security       | 2026-01-03 |
| 12  | [Worker-Centric Analysis Lifecycle](./12-worker-centric-analysis-lifecycle.md)         | Architecture   | 2024-12-16 |
| 13  | [Billing and Quota Architecture](./13-billing-quota-architecture.md)                   | Billing        | 2026-01-18 |
| 14  | [AI-Based Spec Document Generation Pipeline](./14-ai-spec-generation-pipeline.md)      | AI/ML          | 2026-01-18 |
| 15  | [Parser Version Tracking for Re-analysis](./15-parser-version-tracking.md)             | Data Integrity | 2026-01-18 |
| 16  | [Multi-Queue Priority Routing Architecture](./16-multi-queue-priority-routing.md)      | Architecture   | 2026-01-19 |
| 17  | [Test File Schema Normalization](./17-test-file-schema-normalization.md)               | Database       | 2026-01-19 |
| 18  | [GitHub API Cache Tables](./18-github-api-cache-tables.md)                             | Database       | 2025-12-24 |
| 19  | [Hierarchical Spec Document Schema](./19-hierarchical-spec-document-schema.md)         | Database       | 2026-01-12 |
| 20  | [GitHub App Installation Schema](./20-github-app-installation-schema.md)               | Database       | 2026-01-19 |

### Core Repository

| #   | Title                                                                                                         | Area    | Date       |
| --- | ------------------------------------------------------------------------------------------------------------- | ------- | ---------- |
| 01  | [Core Library Separation](./core/01-core-library-separation.md)                                               | Core    | 2024-12-17 |
| 02  | [Dynamic Test Counting Policy](./core/02-dynamic-test-counting-policy.md)                                     | Core    | 2024-12-22 |
| 03  | [Tree-sitter as AST Parsing Engine](./core/03-tree-sitter-ast-parsing-engine.md)                              | Parser  | 2024-12-23 |
| 04  | [Early-Return Framework Detection](./core/04-early-return-framework-detection.md)                             | Parser  | 2024-12-23 |
| 05  | [Parser Pooling Disabled](./core/05-parser-pooling-disabled.md)                                               | Parser  | 2024-12-23 |
| 06  | [Unified Framework Definition](./core/06-unified-framework-definition.md)                                     | Parser  | 2024-12-23 |
| 07  | [Source Abstraction Interface](./core/07-source-abstraction-interface.md)                                     | Parser  | 2024-12-23 |
| 08  | [Shared Parser Modules](./core/08-shared-parser-modules.md)                                                   | Parser  | 2024-12-23 |
| 09  | [Config Scope Resolution](./core/09-config-scope-resolution.md)                                               | Config  | 2024-12-23 |
| 10  | [Standard Go Project Layout](./core/10-standard-go-project-layout.md)                                         | Project | 2024-12-23 |
| 11  | [Integration Testing with Golden Snapshots](./core/11-integration-testing-golden-snapshots.md)                | Testing | 2024-12-23 |
| 12  | [Parallel Scanning with Worker Pool](./core/12-parallel-scanning-worker-pool.md)                              | Perf    | 2024-12-23 |
| 13  | [NaCl SecretBox Encryption](./core/13-nacl-secretbox-encryption.md)                                           | Crypto  | 2024-12-23 |
| 14  | [Indirect Import Alias Detection Unsupported](./core/14-indirect-import-unsupported.md)                       | Parser  | 2025-12-29 |
| 15  | [C# Preprocessor Block Attribute Detection Limitation](./core/15-csharp-preprocessor-attribute-limitation.md) | Parser  | 2026-01-04 |
| 16  | [Domain Hints Extraction System](./core/16-domain-hints-extraction.md)                                        | AI/ML   | 2026-01-18 |
| 17  | [Swift Testing Framework Support](./core/17-swift-testing-framework-support.md)                               | Parser  | 2026-01-04 |
| 18  | [JUnit 4 Framework Separation](./core/18-junit4-framework-separation.md)                                      | Parser  | 2025-12-26 |
| 19  | [Vitest 4.0+ test.for/it.for API Support](./core/19-vitest-4-api-support.md)                                  | Parser  | 2026-01-03 |
| 20  | [Java 21+ Implicit Class Detection](./core/20-java21-implicit-class-detection.md)                             | Parser  | 2026-01-04 |
| 21  | [Rust Macro-Based Test Detection](./core/21-rust-macro-test-detection.md)                                     | Parser  | 2025-12-27 |

### Worker Repository

| #   | Title                                                                                        | Area         | Date       |
| --- | -------------------------------------------------------------------------------------------- | ------------ | ---------- |
| 01  | [Scheduled Re-analysis Architecture](./worker/01-scheduled-recollection.md)                  | Architecture | 2024-12-18 |
| 02  | [Clean Architecture Layer Introduction](./worker/02-clean-architecture-layers.md)            | Architecture | 2024-12-18 |
| 03  | [Graceful Shutdown and Context-Based Lifecycle Management](./worker/03-graceful-shutdown.md) | Architecture | 2024-12-18 |
| 04  | [OAuth Token Graceful Degradation](./worker/04-oauth-token-graceful-degradation.md)          | Reliability  | 2024-12-18 |
| 05  | [Analyzer-Scheduler Process Separation](./worker/05-worker-scheduler-separation.md)          | Architecture | 2024-12-18 |
| 06  | [Semaphore-Based Clone Concurrency Control](./worker/06-semaphore-clone-concurrency.md)      | Concurrency  | 2024-12-18 |
| 07  | [Repository Pattern Data Access Abstraction](./worker/07-repository-pattern.md)              | Architecture | 2024-12-18 |
| 08  | [SpecView Worker Binary Separation](./worker/08-specview-worker-separation.md)               | Architecture | 2026-01-13 |

### Web Repository

| #   | Title                                                                               | Area          | Date       |
| --- | ----------------------------------------------------------------------------------- | ------------- | ---------- |
| 01  | [Go as Backend Language](./web/01-go-backend-language.md)                           | Tech Stack    | 2024-12-18 |
| 02  | [Next.js 16 + React 19 Selection](./web/02-nextjs-react-selection.md)               | Tech Stack    | 2025-12-04 |
| 03  | [Chi Router Selection](./web/03-chi-router-selection.md)                            | Tech Stack    | 2025-01-03 |
| 04  | [TanStack Query Selection](./web/04-tanstack-query-selection.md)                    | Tech Stack    | 2025-01-03 |
| 05  | [shadcn/ui + Tailwind CSS Selection](./web/05-shadcn-tailwind-selection.md)         | Tech Stack    | 2025-01-03 |
| 06  | [SQLc Selection](./web/06-sqlc-selection.md)                                        | Tech Stack    | 2025-01-03 |
| 07  | [Next.js BFF Architecture](./web/07-nextjs-bff-architecture.md)                     | Architecture  | 2025-01-03 |
| 08  | [Clean Architecture Pattern](./web/08-clean-architecture-pattern.md)                | Architecture  | 2025-01-03 |
| 09  | [DI Container Pattern](./web/09-di-container-pattern.md)                            | Architecture  | 2025-01-03 |
| 10  | [StrictServerInterface Contract](./web/10-strict-server-interface-contract.md)      | API           | 2025-01-03 |
| 11  | [Feature-Based Module Organization](./web/11-feature-based-module-organization.md)  | Architecture  | 2025-01-03 |
| 12  | [APIHandlers Composition Pattern](./web/12-apihandlers-composition-pattern.md)      | Architecture  | 2025-01-03 |
| 13  | [Domain Error Handling Pattern](./web/13-domain-error-handling-pattern.md)          | Architecture  | 2025-01-03 |
| 14  | [slog Structured Logging](./web/14-slog-structured-logging.md)                      | Observability | 2025-01-03 |
| 15  | [React 19 use() Hook Pattern](./web/15-react-19-use-hook-pattern.md)                | Frontend      | 2025-01-03 |
| 16  | [nuqs URL State Management](./web/16-nuqs-url-state-management.md)                  | Frontend      | 2025-01-03 |
| 17  | [next-intl i18n Strategy](./web/17-next-intl-i18n-strategy.md)                      | Frontend      | 2025-01-03 |
| 18  | [next-themes Dark Mode](./web/18-next-themes-dark-mode.md)                          | Frontend      | 2025-01-03 |
| 19  | [CSS Variable Design Token System](./web/19-css-variable-design-token-system.md)    | Frontend      | 2025-01-03 |
| 20  | [Skeleton Loading Pattern](./web/20-skeleton-loading-pattern.md)                    | Frontend      | 2025-01-03 |
| 21  | [Anonymous User Rate Limiting](./web/21-anonymous-rate-limiting.md)                 | Security      | 2026-01-15 |
| 22  | [React Compiler Adoption](./web/22-react-compiler-adoption.md)                      | Frontend      | 2026-01-19 |
| 23  | [Window-Level Virtualization Pattern](./web/23-window-level-virtualization.md)      | Frontend      | 2026-01-19 |
| 24  | [Subscription Period Pro-rata Calculation](./web/24-subscription-period-prorata.md) | Billing       | 2026-01-16 |
| 25  | [OAuth Return URL Handling](./web/25-oauth-return-url-handling.md)                  | Security      | 2026-01-16 |

## Process

1. **Create**: Copy [template.md](./template.md) → `XX-title.md`
2. **Write**: Fill in all sections with finalized decision
3. **Localize**: Create Korean version in `kr/adr/`
4. **Review**: Submit PR for team review
5. **Merge**: Add to index after approval

## Related Repositories

- [specvital/core](https://github.com/specvital/core) - Parser engine
- [specvital/web](https://github.com/specvital/web) - Web platform
- [specvital/worker](https://github.com/specvital/worker) - Worker service
- [specvital/infra](https://github.com/specvital/infra) - Infrastructure
