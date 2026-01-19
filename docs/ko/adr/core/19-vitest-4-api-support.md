---
title: Vitest 4.0+ test.for/it.for API 지원
description: jstest 공유 모듈을 확장하여 Vitest test.for/it.for 매개변수화 테스트 API 지원에 대한 ADR
---

# ADR-19: Vitest 4.0+ test.for/it.for API 지원

> 🇺🇸 [English Version](/en/adr/core/19-vitest-4-api-support)

| 날짜       | 작성자     | 레포 |
| ---------- | ---------- | ---- |
| 2026-01-03 | @specvital | core |

## 컨텍스트

### 문제 상황

SpecVital Core 파서의 정적 AST 분석이 Vitest의 `test.for`/`it.for` API를 인식하지 못해 최신 Vitest 코드베이스 파싱 시 정확도 저하 발생.

### 발견 경위

`vitejs/vite` v7.3.0 검증 결과:

| 지표                 | 값            |
| -------------------- | ------------- |
| 실제 테스트 수 (CLI) | 703개         |
| 파서 탐지 결과       | 583개         |
| 차이                 | -120 (-17.1%) |

원인: `test.for` API를 사용하는 120개 테스트 미탐지.

### 배경

Vitest 2.0에서 도입된 `test.for`/`it.for`는 `test.each`의 대안으로 주요 차이점 존재:

| 항목             | `test.each` | `test.for`            |
| ---------------- | ----------- | --------------------- |
| 인자 전개        | 배열 전개   | 전개 없음             |
| TestContext 접근 | 불가        | 2번째 매개변수로 가능 |
| 동시성 스냅샷    | 미지원      | 지원                  |
| Jest 호환성      | 예          | Vitest 전용           |

**문법 차이:**

```typescript
// test.each는 배열 인자를 전개
test.each([
  [1, 1, 2],
  [1, 2, 3],
])("add(%i, %i) -> %i", (a, b, expected) => {
  expect(a + b).toBe(expected);
});

// test.for는 전개하지 않음 - 구조 분해 필요
test.for([
  [1, 1, 2],
  [1, 2, 3],
])("add(%i, %i) -> %i", ([a, b, expected]) => {
  expect(a + b).toBe(expected);
});
```

`.for` API는 TestContext와 픽스처 관련 `test.each` 제한 해결, 동시성 스냅샷 테스트 활성화:

```typescript
test.concurrent.for([
  [1, 1],
  [1, 2],
])("add(%i, %i)", ([a, b], { expect }) => {
  expect(a + b).matchSnapshot();
});
```

### 요구사항

1. Vitest 파일에서 `test.for`/`it.for`/`describe.for` 패턴 탐지
2. `.each()`와 동일한 카운팅 정책 적용 (ADR-02)
3. 체인 수정자 지원: `test.concurrent.for`, `test.skip.for`, `test.only.for`
4. `jstest` 공유 모듈 변경 최소화 (ADR-08)

## 결정

**기존 `.each()` 인프라를 확장하여 `.for` 수정자 지원.**

`jstest` 공유 모듈의 매개변수화 테스트 처리 로직을 확장하여 `.for`를 `.each`와 함께 추가 수정자로 인식. 기존 동적 테스트 카운팅 정책(ADR-02) 적용: 매개변수화 테스트는 런타임 반복 횟수와 무관하게 1로 카운트.

### 구현

`pkg/parser/strategies/shared/jstest/` 내 3개 파일 최소 변경:

```go
// constants.go
const (
    ModifierFor  = "for"    // 신규
    ModifierEach = "each"
    // ... 기존 수정자들
)

// helpers.go - .for 수정자 탐지 포함
func ParseSimpleMemberExpression(node *sitter.Node, source []byte) string {
    // "test.for", "describe.for" 등 반환
}

// parser.go - .for를 기존 처리 로직으로 라우팅
switch funcName {
case FuncDescribe + "." + ModifierEach, FuncDescribe + "." + ModifierFor:
    ProcessEachSuites(...)
case FuncIt + "." + ModifierEach, FuncIt + "." + ModifierFor:
    ProcessEachTests(...)
}
```

### 지원 패턴

| 패턴                                     | 설명                     |
| ---------------------------------------- | ------------------------ |
| `test.for([...])('name', cb)`            | 기본 매개변수화 테스트   |
| `it.for([...])('name', cb)`              | test.for 별칭            |
| `describe.for([...])('name', cb)`        | 매개변수화 스위트        |
| `test.concurrent.for([...])('name', cb)` | 동시성 매개변수화 테스트 |
| `test.skip.for([...])('name', cb)`       | 스킵 매개변수화 테스트   |
| `test.only.for([...])('name', cb)`       | 포커스 매개변수화 테스트 |

### 카운팅 정책

ADR-02에 따라 모든 매개변수화 테스트 패턴은 1로 카운트:

| 패턴                         | 파서 카운트 | 근거                                  |
| ---------------------------- | ----------- | ------------------------------------- |
| `test.for([a,b,c])`          | 1           | 정적 분석으로 런타임 카운트 평가 불가 |
| `test.each([a,b,c])`         | 1           | 일관성을 위해 동일 정책               |
| `test.concurrent.for([...])` | 1           | 수정자 체인이 정책 변경하지 않음      |

## 고려한 옵션

### 옵션 A: 기존 .each() 인프라 확장 (선택됨)

기존 매개변수화 테스트 처리에 `.for`를 추가 수정자로 추가.

**장점:**

- 최소 코드 변경 (3개 파일)
- 검증된 `.each()` 인프라 활용
- 동적 테스트 카운팅 정책(ADR-02)과 일관성 유지
- 공유 모듈(ADR-08)을 통해 모든 JavaScript 프레임워크 혜택
- 매개변수화 테스트의 단일 코드 경로

**단점:**

- `.for`와 `.each` 간 의미적 차이 무시
- 향후 `.for` 전용 기능 시 분리 필요 가능성

### 옵션 B: 별도 test.for 파서 생성

`test.for`/`it.for` 패턴을 위한 독립적 파싱 로직 구현.

**장점:**

- 깔끔한 관심사 분리
- `.each()` 회귀 위험 없음
- `.for` 전용 최적화 가능

**단점:**

- `.each()` 처리와 약 70% 코드 중복
- DRY 원칙 및 공유 파서 모듈 패턴(ADR-08) 위반
- 버그 수정을 각 경로에 별도 적용 필요
- 유지보수 부담 증가

### 옵션 C: test.for 별도 카운팅 정책

배열 인자를 파싱하여 `.for` 패턴의 실제 반복 횟수 카운트 시도.

**장점:**

- 단순 리터럴 케이스에 대해 더 정확한 카운트
- 사용자 기대와 더 나은 부합

**단점:**

- 기존 동적 테스트 카운팅 정책(ADR-02) 위반
- 유사한 API 간 일관성 없는 동작
- 변수 참조 처리 불가 (동일한 제한)
- 서로 다른 카운팅 규칙으로 인한 사용자 혼란

### 옵션 D: Vitest API를 통한 런타임 탐지

Vitest 테스트 수집 API를 사용하여 정확한 런타임 카운트 확보.

**장점:**

- 모든 패턴에 대해 100% 정확도
- 정적 분석 제한 없음

**단점:**

- 코어의 정적 전용 아키텍처 근본적 변경
- 테스트 환경 설정 필요
- 코드 실행으로 인한 보안 문제
- 성능 영향
- 코어 아키텍처 원칙(ADR-03) 위반

## 결과

### 긍정적

1. **정확도 복원**
   - vitejs/vite 탐지 정확도 허용 범위 내로 복원
   - 최신 Vitest 코드베이스 올바르게 파싱

2. **일관성**
   - `.each()`와 `.for()` 패턴에 대해 동일한 카운팅 동작
   - 매개변수화 API 전반에 걸쳐 예측 가능한 동작 경험

3. **공유 모듈 혜택**
   - 모든 `jstest` 소비자(Jest, Vitest, Mocha, Cypress, Playwright) 기능 획득
   - 다른 프레임워크의 잠재적 `.for` 도입에 대비

4. **유지보수성**
   - 모든 매개변수화 테스트 처리를 위한 단일 코드 경로
   - 공유 로직의 버그 수정이 모든 패턴에 혜택

### 부정적

1. **의미적 단순화**
   - `.for`와 `.each` 간 인자 전개 차이 무시
   - 완화책: 카운팅 정책이 둘 다 1로 처리하므로 의미적 차이가 카운트 정확도에 무관

2. **향후 분리 가능성**
   - `.for`가 다른 처리를 요구하는 기능 획득 시 리팩토링 필요 가능
   - 완화책: 현재 접근 방식이 필요시 향후 옵션 B로 분리 배제하지 않음

## 참조

- [이슈 #96: vitest - add test.for/it.for API support](https://github.com/specvital/core/issues/96)
- [커밋 5c7c8fa: feat(vitest): add test.for/it.for API support](https://github.com/specvital/core/commit/5c7c8fa)
- [ADR-02: 동적 테스트 카운팅 정책](/ko/adr/core/02-dynamic-test-counting-policy)
- [ADR-08: 공유 파서 모듈](/ko/adr/core/08-shared-parser-modules)
- [Vitest test.for 문서](https://vitest.dev/api/)
