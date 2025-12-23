---
title: 홈
description: Specvital 문서 허브 - PRD, ADR, 아키텍처, API 레퍼런스 제공
---

# Specvital 문서

> 🇺🇸 [English Version](/en/)

Specvital은 코드 리뷰 프로세스를 개선하기 위해 설계된 오픈소스 테스트 커버리지 인사이트 도구.

## 문서 구조

### [PRD (Product Requirements Document)](./prd/)

Specvital 플랫폼의 제품 사양 및 요구사항 문서.

- [제품 개요](./prd/00-overview.md) - 제품 비전, 타겟 사용자, GTM 전략
- [아키텍처](./prd/01-architecture.md) - 시스템 아키텍처 및 서비스 구성
- [코어 엔진](./prd/02-core-engine.md) - 테스트 파서 라이브러리 설계
- [웹 플랫폼](./prd/03-web-platform.md) - 웹 대시보드 및 REST API
- [컬렉터 서비스](./prd/04-collector-service.md) - 백그라운드 분석 워커
- [데이터베이스 설계](./prd/05-database-design.md) - 데이터베이스 스키마 및 설계
- [기술 스택](./prd/06-tech-stack.md) - 기술 선택 및 근거

### [ADR (Architecture Decision Records)](./adr/)

Specvital 개발 중 내린 아키텍처 결정에 대한 문서.

**공통**

- [ADR 개요](./adr/) - 아키텍처 결정 기록 소개
- [정적 분석 접근법](./adr/01-static-analysis-approach.md)
- [경쟁 차별화](./adr/02-competitive-differentiation.md)
- [API 워커 서비스 분리](./adr/03-api-worker-service-separation.md)
- [큐 기반 비동기 처리](./adr/04-queue-based-async-processing.md)
- [리포지토리 전략](./adr/05-repository-strategy.md)
- [PaaS 우선 인프라](./adr/06-paas-first-infrastructure.md)
- [공유 인프라](./adr/07-shared-infrastructure.md)
- [External Repo ID 무결성](./adr/08-external-repo-id-integrity.md)

**[Core](./adr/core/)**

- [코어 라이브러리 분리](./adr/core/01-core-library-separation.md)
- [동적 테스트 카운팅 정책](./adr/core/02-dynamic-test-counting-policy.md)
- [Tree-sitter AST 파싱 엔진](./adr/core/03-tree-sitter-ast-parsing-engine.md)
- [Early-Return 프레임워크 탐지](./adr/core/04-early-return-framework-detection.md)
- [파서 풀링 비활성화](./adr/core/05-parser-pooling-disabled.md)
- [통합 Framework Definition](./adr/core/06-unified-framework-definition.md)
- [Source 추상화 인터페이스](./adr/core/07-source-abstraction-interface.md)
- [공유 파서 모듈](./adr/core/08-shared-parser-modules.md)
- [Config 스코프 해석](./adr/core/09-config-scope-resolution.md)
- [표준 Go 프로젝트 레이아웃](./adr/core/10-standard-go-project-layout.md)
- [골든 스냅샷 통합 테스트](./adr/core/11-integration-testing-golden-snapshots.md)
- [Worker Pool 병렬 스캔](./adr/core/12-parallel-scanning-worker-pool.md)
- [NaCl SecretBox 암호화](./adr/core/13-nacl-secretbox-encryption.md)

**[Collector](./adr/collector/)**

- [스케줄 기반 재수집](./adr/collector/01-scheduled-recollection.md)
- [Clean Architecture 레이어](./adr/collector/02-clean-architecture-layers.md)
- [Graceful Shutdown](./adr/collector/03-graceful-shutdown.md)
- [OAuth 토큰 Degradation](./adr/collector/04-oauth-token-graceful-degradation.md)
- [Worker-Scheduler 분리](./adr/collector/05-worker-scheduler-separation.md)
- [Semaphore Clone 동시성](./adr/collector/06-semaphore-clone-concurrency.md)
- [Repository 패턴](./adr/collector/07-repository-pattern.md)

**[Web](./adr/web/)**

- [백엔드 언어로 Go 선택](./adr/web/01-go-backend-language.md)

### [기술 레이더](./tech-radar.md)

플랫폼 전반의 기술 채택 현황 및 평가 기준.

### [릴리즈 노트](./releases.md)

전체 서비스 릴리즈 히스토리 (Core, Collector, Web, Infra).

### [용어집](./glossary.md)

플랫폼 전반에서 사용되는 도메인 용어.

### [아키텍처 개요](./architecture.md)

상위 수준의 시스템 아키텍처 문서.

## 관련 리포지토리

Specvital 플랫폼은 여러 리포지토리로 구성됨:

- [specvital/core](https://github.com/specvital/core) - 파서 엔진
- [specvital/web](https://github.com/specvital/web) - 웹 플랫폼
- [specvital/collector](https://github.com/specvital/collector) - 워커 서비스
- [specvital/infra](https://github.com/specvital/infra) - 인프라 및 스키마

## 기여하기

Specvital의 메인 문서 리포지토리. 기여 가이드라인은 각 리포지토리의 CONTRIBUTING.md 파일 참조.

## 라이선스

자세한 내용은 [LICENSE](https://github.com/specvital/.github/blob/main/LICENSE) 참조.
