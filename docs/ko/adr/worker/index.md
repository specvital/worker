---
title: Worker ADR
description: 워커 서비스(백그라운드 분석 워커) 아키텍처 의사결정 기록
---

# Worker 리포지토리 ADR

> 🇺🇸 [English Version](/en/adr/worker/)

[specvital/worker](https://github.com/specvital/worker) 리포지토리 (워커 서비스)의 아키텍처 의사결정 기록.

## ADR 목록

| #   | 제목                                                                         | 날짜       |
| --- | ---------------------------------------------------------------------------- | ---------- |
| 01  | [스케줄 기반 재분석 아키텍처](./01-scheduled-recollection.md)                | 2024-12-18 |
| 02  | [Clean Architecture 레이어 도입](./02-clean-architecture-layers.md)          | 2024-12-18 |
| 03  | [Graceful Shutdown 및 Context 기반 생명주기 관리](./03-graceful-shutdown.md) | 2024-12-18 |
| 04  | [OAuth 토큰 Graceful Degradation](./04-oauth-token-graceful-degradation.md)  | 2024-12-18 |
| 05  | [Analyzer-Scheduler 프로세스 분리](./05-worker-scheduler-separation.md)      | 2024-12-18 |
| 06  | [Semaphore 기반 Clone 동시성 제어](./06-semaphore-clone-concurrency.md)      | 2024-12-18 |
| 07  | [Repository 패턴 데이터 접근 추상화](./07-repository-pattern.md)             | 2024-12-18 |

## 관련 문서

- [전체 ADR](/ko/adr/)
- [Worker PRD](/ko/prd/04-worker-service.md)
