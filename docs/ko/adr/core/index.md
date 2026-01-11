---
title: 코어 ADR
description: 코어 라이브러리(테스트 파서 엔진) 아키텍처 의사결정 기록
---

# 코어 리포지토리 ADR

> 🇺🇸 [English Version](/en/adr/core/)

[specvital/core](https://github.com/specvital/core) 리포지토리의 아키텍처 의사결정 기록.

## ADR 목록

| #   | 제목                                                                                        | 날짜       |
| --- | ------------------------------------------------------------------------------------------- | ---------- |
| 01  | [코어 라이브러리 분리](./01-core-library-separation.md)                                     | 2024-12-17 |
| 02  | [동적 테스트 카운팅 정책](./02-dynamic-test-counting-policy.md)                             | 2024-12-22 |
| 03  | [Tree-sitter AST 파싱 엔진](./03-tree-sitter-ast-parsing-engine.md)                         | 2024-12-23 |
| 04  | [Early-Return 프레임워크 탐지](./04-early-return-framework-detection.md)                    | 2024-12-23 |
| 05  | [파서 풀링 비활성화](./05-parser-pooling-disabled.md)                                       | 2024-12-23 |
| 06  | [통합 Framework Definition](./06-unified-framework-definition.md)                           | 2024-12-23 |
| 07  | [Source 추상화 인터페이스](./07-source-abstraction-interface.md)                            | 2024-12-23 |
| 08  | [공유 파서 모듈](./08-shared-parser-modules.md)                                             | 2024-12-23 |
| 09  | [Config 스코프 해석](./09-config-scope-resolution.md)                                       | 2024-12-23 |
| 10  | [표준 Go 프로젝트 레이아웃](./10-standard-go-project-layout.md)                             | 2024-12-23 |
| 11  | [골든 스냅샷 통합 테스트](./11-integration-testing-golden-snapshots.md)                     | 2024-12-23 |
| 12  | [Worker Pool 병렬 스캔](./12-parallel-scanning-worker-pool.md)                              | 2024-12-23 |
| 13  | [NaCl SecretBox 암호화](./13-nacl-secretbox-encryption.md)                                  | 2024-12-23 |
| 14  | [간접 Import Alias 감지 미지원](./14-indirect-import-unsupported.md)                        | 2025-12-29 |
| 15  | [C# 전처리기 블록 내 Attribute 감지 한계](./15-csharp-preprocessor-attribute-limitation.md) | 2026-01-04 |

## 관련 문서

- [전체 ADR](/ko/adr/)
- [Core PRD](/ko/prd/02-core-engine.md)
