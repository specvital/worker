---
title: 동적 테스트 카운팅 정책
description: 정적 AST 분석에서 동적 생성 테스트 패턴 처리 정책
---

# ADR-02: 동적 테스트 카운팅 정책

> 🇺🇸 [English Version](/en/adr/core/02-dynamic-test-counting-policy.md)

| 날짜       | 작성자       | 영향 리포지토리 |
| ---------- | ------------ | --------------- |
| 2025-12-22 | @KubrickCode | core            |

**상태**: 승인됨
**구현**: ✅ Phase 1 완료 (2025-12-22)

## Context

SpecVital Core 파서는 정적 AST 분석 기반 테스트 카운트 수행. 많은 테스트 프레임워크가 런타임 실행 없이는 정확한 카운트 불가능한 동적 테스트 생성 패턴 지원.

### 발견 경위

`github-project-status-viewer` 대상 검증 결과:

- 실제값 (CLI): 236 테스트
- 파서 결과: 229 테스트
- 차이: -7 (2.97%)

근본 원인: 동적 테스트 패턴 미지원.

## Decision

### 정책: 동적 테스트를 1로 카운트

모든 동적 생성 테스트 패턴은 실제 런타임 카운트와 관계없이 **1개 테스트**로 카운트.

### 근거

1. **정적 분석의 한계**: 런타임 값 평가 불가
2. **일관성**: 20개 프레임워크 전체에서 동일한 동작
3. **복잡도 vs 가치**: 배열 리터럴 파싱은 한계 이익만 제공
4. **탐지 우선순위**: 테스트 존재 탐지 > 정확한 카운트

## Options Considered

### Option A: 동적 테스트를 1로 카운트 (선택됨)

모든 동적 패턴을 단일 테스트로 균일하게 처리.

**장점:**

- 프레임워크 간 일관된 동작
- 단순한 구현
- 정확도에 대한 거짓 약속 없음
- 한계에 대한 명확한 문서화

**단점:**

- 파서 카운트와 CLI 카운트 차이 가능
- 정확한 카운트를 위한 CLI 필요

### Option B: 배열 리터럴 파싱

`it.each([1,2,3])` 같은 정적 패턴에서 배열 요소 카운트 시도.

**장점:**

- 단순한 케이스에서 더 정확

**단점:**

- 비일관적 (리터럴은 됨, 변수는 안됨)
- 복잡한 구현
- 미미한 정확도 향상

### Option C: 런타임 실행 필요

테스트를 실행하여 정확한 카운트 획득.

**장점:**

- 100% 정확도

**단점:**

- 코어의 정적 분석 접근 방식의 근본적 변경
- 테스트 환경 설정 필요
- 느린 실행
- 보안 우려

## 프레임워크 분석

### 프레임워크별 동적 테스트 패턴

| 프레임워크                | 동적 패턴                   | 현재 지원 | 정책                  |
| ------------------------- | --------------------------- | --------- | --------------------- |
| **JavaScript/TypeScript** |                             |           |                       |
| Jest                      | `it.each([...])`            | 부분      | 1 + `(dynamic cases)` |
| Jest                      | `forEach` + `it`            | ❌ 버그   | 1                     |
| Vitest                    | `it.each([...])`            | 부분      | 1 + `(dynamic cases)` |
| Vitest                    | `forEach` + `it`            | ❌ 버그   | 1                     |
| Mocha                     | `forEach` + `it`            | ❌ 버그   | 1                     |
| Cypress                   | `forEach` + `it`            | ❌ 버그   | 1                     |
| Playwright                | loop + `test`               | ❌        | 1                     |
| **Python**                |                             |           |                       |
| pytest                    | `@pytest.mark.parametrize`  | ❌        | 1                     |
| unittest                  | `subTest`                   | ❌        | 1                     |
| **Java**                  |                             |           |                       |
| JUnit5                    | `@ParameterizedTest`        | ❌        | 1                     |
| JUnit5                    | `@RepeatedTest`             | ❌        | 1                     |
| TestNG                    | `@DataProvider`             | ❌        | 1                     |
| **Kotlin**                |                             |           |                       |
| Kotest                    | `forAll`, data-driven       | ❌        | 1                     |
| **C#**                    |                             |           |                       |
| NUnit                     | `[TestCase]` 복수           | ✅        | N (attribute 카운트)  |
| NUnit                     | `[TestCaseSource]`          | ❌        | 1                     |
| xUnit                     | `[Theory]` + `[InlineData]` | ✅        | N (attribute 카운트)  |
| xUnit                     | `[MemberData]`              | ❌        | 1                     |
| MSTest                    | `[DataRow]` 복수            | ✅        | N (attribute 카운트)  |
| MSTest                    | `[DynamicData]`             | ❌        | 1                     |
| **Ruby**                  |                             |           |                       |
| RSpec                     | `shared_examples`           | ❌        | 1                     |
| Minitest                  | loop + `def test_`          | ❌        | 1                     |
| **Go**                    |                             |           |                       |
| go-testing                | `t.Run` in loop             | ✅        | N (감지된 subtest)    |
| go-testing                | table-driven (변수)         | 부분      | 감지된 row만          |
| **Rust**                  |                             |           |                       |
| cargo-test                | `#[test_case]`              | ❌        | 1                     |
| **C++**                   |                             |           |                       |
| GoogleTest                | `INSTANTIATE_TEST_SUITE_P`  | ❌        | 1                     |
| **Swift**                 |                             |           |                       |
| XCTest                    | 네이티브 parametrized 없음  | N/A       | -                     |
| **PHP**                   |                             |           |                       |
| PHPUnit                   | `@dataProvider`             | ❌        | 1                     |

### 범례

- ✅ 지원: 실제 케이스 카운트
- 부분: 패턴 감지하지만 모든 케이스 카운트 못함
- ❌ 미지원: 1로 카운트
- ❌ 버그: 감지해야 하지만 현재 안됨

## Consequences

### Positive

- 프레임워크 간 일관된 동작
- 단순한 구현
- 정확도에 대한 거짓 약속 없음
- 한계에 대한 명확한 문서화

### Negative

- 파서 카운트와 CLI 카운트 차이 가능
- 정확한 카운트를 위한 CLI 필요

### Neutral

- 실제값 검증 시 동적 테스트 고려 필요

## Implementation

### Phase 1: 버그 수정 (필수)

테스트 감지 필요하나 현재 0 반환하는 패턴 수정:

1. **JS/TS**: `forEach`/`map` 콜백 내 `it`/`test`
2. **JS/TS**: 객체 배열이 있는 `it.each([{...}])` (현재 0, 1이어야 함)

### Phase 2: 개선 (선택)

카운트가 정적으로 결정 가능한 attribute 기반 parametrized 테스트 카운트 고려:

- C#의 `[TestCase(...)]` × N
- 리터럴 배열이 있는 `@pytest.mark.parametrize("x", [1,2,3])`
