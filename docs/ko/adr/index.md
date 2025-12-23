---
title: ADR (한국어)
description: Specvital 프로젝트의 핵심 기술 의사결정 기록 문서
---

# 아키텍처 의사결정 기록 (ADR)

> 🇺🇸 [English Version](/en/adr/)

Specvital 프로젝트의 아키텍처 의사결정 문서화

## ADR이란?

ADR(Architecture Decision Record)은 중요한 아키텍처 결정을 그 배경 및 결과와 함께 기록하는 문서. 멀티-리포지토리 마이크로서비스 환경에서 의사결정 히스토리를 유지하는 데 도움이 됨.

## ADR 작성 시점

| 카테고리     | 예시                                              |
| ------------ | ------------------------------------------------- |
| 기술 스택    | 프레임워크 선택, 라이브러리 도입, 버전 업그레이드 |
| 아키텍처     | 서비스 경계, 통신 패턴, 데이터 흐름               |
| API 설계     | 엔드포인트 구조, 버저닝 전략, 에러 처리           |
| 데이터베이스 | 스키마 설계, 마이그레이션 전략, 인덱싱 방식       |
| 인프라       | 배포 플랫폼, 스케일링 전략, 모니터링              |
| 공통 관심사  | 보안, 성능 최적화, 옵저버빌리티                   |

## 템플릿

| 템플릿                       | 용도                                  |
| ---------------------------- | ------------------------------------- |
| [template.md](./template.md) | 대부분의 의사결정에 사용하는 표준 ADR |

## 명명 규칙

```
XX-brief-decision-title.md
```

- `XX`: 두 자리 순차 번호 (01, 02, ...)
- 소문자와 하이픈 사용
- 간결하고 명확한 제목

## 기술 영역

| 영역           | 영향받는 리포지토리 |
| -------------- | ------------------- |
| Parser         | core                |
| API            | web                 |
| Worker         | collector           |
| Database       | infra               |
| Infrastructure | infra               |
| Cross-cutting  | 복수                |

## ADR 목록

### 공통 (전체 리포지토리)

| #   | 제목                                                                            | 영역           | 날짜       |
| --- | ------------------------------------------------------------------------------- | -------------- | ---------- |
| 01  | [정적 분석 기반 즉시 분석](./01-static-analysis-approach.md)                    | Cross-cutting  | 2024-12-17 |
| 02  | [경쟁 차별화 전략](./02-competitive-differentiation.md)                         | Cross-cutting  | 2024-12-17 |
| 03  | [API와 Worker 서비스 분리](./03-api-worker-service-separation.md)               | Architecture   | 2024-12-17 |
| 04  | [큐 기반 비동기 처리](./04-queue-based-async-processing.md)                     | Architecture   | 2024-12-17 |
| 05  | [Polyrepo 리포지토리 전략](./05-repository-strategy.md)                         | Architecture   | 2024-12-17 |
| 06  | [PaaS 우선 인프라 전략](./06-paas-first-infrastructure.md)                      | Infrastructure | 2024-12-17 |
| 07  | [공유 인프라 전략](./07-shared-infrastructure.md)                               | Infrastructure | 2024-12-17 |
| 08  | [External Repository ID 기반 데이터 무결성](./08-external-repo-id-integrity.md) | Data Integrity | 2024-12-22 |

### Core 리포지토리

| #   | 제목                                                                          | 영역    | 날짜       |
| --- | ----------------------------------------------------------------------------- | ------- | ---------- |
| 01  | [코어 라이브러리 분리](./core/01-core-library-separation.md)                  | Core    | 2024-12-17 |
| 02  | [동적 테스트 카운팅 정책](./core/02-dynamic-test-counting-policy.md)          | Core    | 2024-12-22 |
| 03  | [Tree-sitter AST 파싱 엔진](./core/03-tree-sitter-ast-parsing-engine.md)      | Parser  | 2024-12-23 |
| 04  | [Early-Return 프레임워크 탐지](./core/04-early-return-framework-detection.md) | Parser  | 2024-12-23 |
| 05  | [파서 풀링 비활성화](./core/05-parser-pooling-disabled.md)                    | Parser  | 2024-12-23 |
| 06  | [통합 Framework Definition](./core/06-unified-framework-definition.md)        | Parser  | 2024-12-23 |
| 07  | [Source 추상화 인터페이스](./core/07-source-abstraction-interface.md)         | Parser  | 2024-12-23 |
| 08  | [공유 파서 모듈](./core/08-shared-parser-modules.md)                          | Parser  | 2024-12-23 |
| 09  | [Config 스코프 해석](./core/09-config-scope-resolution.md)                    | Config  | 2024-12-23 |
| 10  | [표준 Go 프로젝트 레이아웃](./core/10-standard-go-project-layout.md)          | Project | 2024-12-23 |
| 11  | [골든 스냅샷 통합 테스트](./core/11-integration-testing-golden-snapshots.md)  | Testing | 2024-12-23 |
| 12  | [Worker Pool 병렬 스캔](./core/12-parallel-scanning-worker-pool.md)           | Perf    | 2024-12-23 |
| 13  | [NaCl SecretBox 암호화](./core/13-nacl-secretbox-encryption.md)               | Crypto  | 2024-12-23 |

### Collector 리포지토리

| #   | 제목                                                                                   | 영역         | 날짜       |
| --- | -------------------------------------------------------------------------------------- | ------------ | ---------- |
| 01  | [스케줄 기반 재수집 아키텍처](./collector/01-scheduled-recollection.md)                | Architecture | 2024-12-18 |
| 02  | [Clean Architecture 레이어 도입](./collector/02-clean-architecture-layers.md)          | Architecture | 2024-12-18 |
| 03  | [Graceful Shutdown 및 Context 기반 생명주기 관리](./collector/03-graceful-shutdown.md) | Architecture | 2024-12-18 |
| 04  | [OAuth 토큰 Graceful Degradation](./collector/04-oauth-token-graceful-degradation.md)  | Reliability  | 2024-12-18 |
| 05  | [Worker-Scheduler 프로세스 분리](./collector/05-worker-scheduler-separation.md)        | Architecture | 2024-12-18 |
| 06  | [Semaphore 기반 Clone 동시성 제어](./collector/06-semaphore-clone-concurrency.md)      | Concurrency  | 2024-12-18 |
| 07  | [Repository 패턴 데이터 접근 추상화](./collector/07-repository-pattern.md)             | Architecture | 2024-12-18 |

### Web 리포지토리

| #   | 제목                                                     | 영역       | 날짜       |
| --- | -------------------------------------------------------- | ---------- | ---------- |
| 01  | [백엔드 언어로 Go 선택](./web/01-go-backend-language.md) | Tech Stack | 2024-12-18 |

## 프로세스

1. **생성**: [template.md](./template.md) 복사 → `XX-title.md`
2. **작성**: 확정된 의사결정 내용으로 모든 섹션 작성
3. **현지화**: `adr/`에 영어 버전 생성
4. **리뷰**: 팀 리뷰를 위해 PR 제출
5. **병합**: 승인 후 목록에 추가

## 관련 리포지토리

- [specvital/core](https://github.com/specvital/core) - 파서 엔진
- [specvital/web](https://github.com/specvital/web) - 웹 플랫폼
- [specvital/collector](https://github.com/specvital/collector) - 워커 서비스
- [specvital/infra](https://github.com/specvital/infra) - 인프라
