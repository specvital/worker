---
title: Home
description: Specvital documentation hub with PRD, ADR, architecture, and API references
---

# Specvital Documentation

> 🇰🇷 [한국어 버전](/ko/)

Welcome to the Specvital documentation. Specvital is an open-source test coverage insights tool designed to enhance code review processes.

## Documentation Structure

### [PRD (Product Requirements Document)](./prd/)

Product specifications and requirements documentation for the Specvital platform.

- [Product Overview](./prd/00-overview.md) - Product vision, target users, and GTM strategy
- [Architecture](./prd/01-architecture.md) - System architecture and service composition
- [Core Engine](./prd/02-core-engine.md) - Test parser library design
- [Web Platform](./prd/03-web-platform.md) - Web dashboard and REST API
- [Collector Service](./prd/04-collector-service.md) - Background analysis worker
- [Database Design](./prd/05-database-design.md) - Database schema and design
- [Tech Stack](./prd/06-tech-stack.md) - Technology choices and rationale

### [ADR (Architecture Decision Records)](./adr/)

Documentation of architectural decisions made during the development of Specvital.

**Cross-cutting**

- [ADR Overview](./adr/) - Introduction to architecture decision records
- [Static Analysis Approach](./adr/01-static-analysis-approach.md)
- [Competitive Differentiation](./adr/02-competitive-differentiation.md)
- [API Worker Service Separation](./adr/03-api-worker-service-separation.md)
- [Queue-Based Async Processing](./adr/04-queue-based-async-processing.md)
- [Repository Strategy](./adr/05-repository-strategy.md)
- [PaaS-First Infrastructure](./adr/06-paas-first-infrastructure.md)
- [Shared Infrastructure](./adr/07-shared-infrastructure.md)
- [External Repo ID Integrity](./adr/08-external-repo-id-integrity.md)

**[Core](./adr/core/)**

- [Core Library Separation](./adr/core/01-core-library-separation.md)
- [Dynamic Test Counting Policy](./adr/core/02-dynamic-test-counting-policy.md)
- [Tree-sitter AST Parsing Engine](./adr/core/03-tree-sitter-ast-parsing-engine.md)
- [Early-Return Framework Detection](./adr/core/04-early-return-framework-detection.md)
- [Parser Pooling Disabled](./adr/core/05-parser-pooling-disabled.md)
- [Unified Framework Definition](./adr/core/06-unified-framework-definition.md)
- [Source Abstraction Interface](./adr/core/07-source-abstraction-interface.md)
- [Shared Parser Modules](./adr/core/08-shared-parser-modules.md)
- [Config Scope Resolution](./adr/core/09-config-scope-resolution.md)
- [Standard Go Project Layout](./adr/core/10-standard-go-project-layout.md)
- [Integration Testing with Golden Snapshots](./adr/core/11-integration-testing-golden-snapshots.md)
- [Parallel Scanning with Worker Pool](./adr/core/12-parallel-scanning-worker-pool.md)
- [NaCl SecretBox Encryption](./adr/core/13-nacl-secretbox-encryption.md)

**[Collector](./adr/collector/)**

- [Scheduled Re-collection](./adr/collector/01-scheduled-recollection.md)
- [Clean Architecture Layers](./adr/collector/02-clean-architecture-layers.md)
- [Graceful Shutdown](./adr/collector/03-graceful-shutdown.md)
- [OAuth Token Degradation](./adr/collector/04-oauth-token-graceful-degradation.md)
- [Worker-Scheduler Separation](./adr/collector/05-worker-scheduler-separation.md)
- [Semaphore Clone Concurrency](./adr/collector/06-semaphore-clone-concurrency.md)
- [Repository Pattern](./adr/collector/07-repository-pattern.md)

**[Web](./adr/web/)**

- [Go as Backend Language](./adr/web/01-go-backend-language.md)

### [Tech Radar](./tech-radar.md)

Technology adoption status and evaluation criteria across the platform.

### [Release Notes](./releases.md)

Release history for all services (Core, Collector, Web, Infra).

### [Glossary](./glossary.md)

Domain terminology used across the platform.

### [Architecture Overview](./architecture.md)

High-level system architecture documentation.

## Related Repositories

The Specvital platform is composed of multiple repositories:

- [specvital/core](https://github.com/specvital/core) - Parser engine
- [specvital/web](https://github.com/specvital/web) - Web platform
- [specvital/collector](https://github.com/specvital/collector) - Worker service
- [specvital/infra](https://github.com/specvital/infra) - Infrastructure and schema

## Contributing

This is the main documentation repository for Specvital. For contribution guidelines, please refer to each repository's CONTRIBUTING.md file.

## License

See [LICENSE](https://github.com/specvital/.github/blob/main/LICENSE) for details.
