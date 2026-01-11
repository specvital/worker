---
title: Worker ADR
description: Architecture Decision Records for the Worker background worker service
---

# Worker Repository ADR

> 🇰🇷 [한국어 버전](/ko/adr/worker/)

Architecture Decision Records for the [specvital/worker](https://github.com/specvital/worker) repository (Worker Service).

## ADR Index

| #   | Title                                                                                 | Date       |
| --- | ------------------------------------------------------------------------------------- | ---------- |
| 01  | [Scheduled Re-analysis Architecture](./01-scheduled-recollection.md)                  | 2024-12-18 |
| 02  | [Clean Architecture Layer Introduction](./02-clean-architecture-layers.md)            | 2024-12-18 |
| 03  | [Graceful Shutdown and Context-Based Lifecycle Management](./03-graceful-shutdown.md) | 2024-12-18 |
| 04  | [OAuth Token Graceful Degradation](./04-oauth-token-graceful-degradation.md)          | 2024-12-18 |
| 05  | [Analyzer-Scheduler Process Separation](./05-worker-scheduler-separation.md)          | 2024-12-18 |
| 06  | [Semaphore-Based Clone Concurrency Control](./06-semaphore-clone-concurrency.md)      | 2024-12-18 |
| 07  | [Repository Pattern Data Access Abstraction](./07-repository-pattern.md)              | 2024-12-18 |

## Related

- [All ADRs](/en/adr/)
- [Worker PRD](/en/prd/04-worker-service.md)
