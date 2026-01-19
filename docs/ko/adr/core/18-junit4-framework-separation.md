---
title: JUnit 4 프레임워크 분리
description: JUnit 5와 구분되는 별도 프레임워크로 JUnit 4 분리 ADR
---

# ADR-18: JUnit 4 프레임워크 분리

> 🇺🇸 [English Version](/en/adr/core/18-junit4-framework-separation.md)

| 날짜       | 작성자     | 리포지토리 |
| ---------- | ---------- | ---------- |
| 2025-12-26 | @specvital | core       |

## 컨텍스트

### 문제 상황

JUnit 프레임워크 파서의 치명적 감지 결함: `org.junit.Test`를 사용하는 JUnit 4 테스트 파일이 JUnit 5로 잘못 분류. 기존 matcher는 import 패키지 구분 없이 `@Test` 어노테이션 존재 여부만 확인.

**정량적 영향:**

- **testcontainers-java**: 250개 JUnit 4 테스트가 JUnit 5로 오분류
- **junit5-samples**: 혼합 JUnit 4/5 예제 잘못 분류
- **playwright-java**: JUnit 4 통합 테스트 잘못 귀속

### 결정 필요 이유

1. **데이터 무결성**: 프레임워크 귀속이 테스트 통계 및 채택 지표에 영향
2. **사용자 신뢰**: 부정확한 프레임워크 감지로 분석 신뢰도 저하
3. **엔터프라이즈 현실**: 다년간 마이그레이션 중 JUnit 4/5 하이브리드 코드베이스 유지
4. **의미적 차이**: JUnit 4와 JUnit 5의 근본적으로 다른 아키텍처

### 감지 패턴

| 버전    | Import 패턴                     | 주요 어노테이션                                                       |
| ------- | ------------------------------- | --------------------------------------------------------------------- |
| JUnit 4 | `org.junit.Test`, `org.junit.*` | `@Test`, `@Before`, `@After`, `@Ignore`, `@RunWith`                   |
| JUnit 5 | `org.junit.jupiter.api.Test`    | `@Test`, `@ParameterizedTest`, `@Nested`, `@Disabled`, `@DisplayName` |

## 결정

**Import 기반 상호 배제를 통한 `junit4` 프레임워크 정의를 `junit5`와 별도로 도입.**

### 핵심 원칙

1. **프레임워크 격리**: JUnit 4와 JUnit 5는 ADR-06 패턴을 따르는 별도 `Definition` 구조체를 가진 독립 프레임워크
2. **Import 기반 감지**: 어노테이션명이 아닌 import 패키지로 프레임워크 버전 결정
3. **공유 AST 모듈**: ADR-08에 따라 양쪽 프레임워크가 `javaast` 유틸리티 재사용
4. **명시적 상호 배제**: 겹치지 않도록 설계된 import 패턴

### 감지 규칙

| 버전    | Import 패턴                                         | 제외                  |
| ------- | --------------------------------------------------- | --------------------- |
| JUnit 4 | `org.junit.Test`, `org.junit.*` (jupiter 제외)      | `org.junit.jupiter.*` |
| JUnit 5 | `org.junit.jupiter.api.Test`, `org.junit.jupiter.*` | 해당 없음             |

### 구현

```go
// junit4/definition.go
var JUnit4ImportPattern = regexp.MustCompile(`import\s+(?:static\s+)?org\.junit\.(?:\*|[A-Z])`)
var JUnit5ImportPattern = regexp.MustCompile(`import\s+(?:static\s+)?org\.junit\.jupiter`)

func (m *JUnit4ContentMatcher) Matches(content []byte) bool {
    // JUnit 5 import 포함 파일 제외
    if JUnit5ImportPattern.Match(content) {
        return false
    }
    // JUnit 4 import 필요
    return JUnit4ImportPattern.Match(content)
}
```

## 검토된 옵션

### 옵션 A: 별도 프레임워크 전략 (선택됨)

독립적인 `junit4`와 `junit5` 프레임워크 정의 생성, 각각의 matcher, 파서, 등록 보유.

**장점:**

- 프레임워크별 명확한 책임의 깔끔한 분리
- ADR-06 준수 (통합 정의 패턴)
- 독립적 진화 (JUnit 4 Rules vs JUnit 5 Extensions)
- 정확한 프레임워크 채택 통계
- 각 프레임워크 격리 테스트 가능

**단점:**

- 하나 대신 두 개의 definition 파일
- 공통 어노테이션 처리의 약간의 코드 중복
- 레지스트리에 두 개의 Java 테스트 프레임워크 항목

### 옵션 B: 버전 감지를 포함한 단일 파서

버전을 내부적으로 감지하고 보고하는 하나의 `junit` 프레임워크.

**장점:**

- 단일 프레임워크 등록
- 통합 JUnit 처리

**단점:**

- ADR-06 위반 (프레임워크 정체성이 런타임에 결정)
- 두 가지 다른 어노테이션 세트에 대한 복잡한 분기
- 통계 모호성 ("junit"은 버전 세분성 상실)

### 옵션 C: 통합 파서 내 Import 기반 라우팅

Import 기반으로 버전별 하위 파서로 라우팅하는 단일 프레임워크 정의.

**장점:**

- 단일 정의 지점
- 내부 라우팅으로 분리 유지

**단점:**

- 숨겨진 복잡성 (외부 뷰는 하나, 내부는 둘)
- Matcher 불일치 (정의가 양쪽 버전 수용 필요)
- 통계는 여전히 단일 "junit" 프레임워크로 보고

### 옵션 D: 어노테이션만 감지 (Import 무시)

Import 고려 없이 어노테이션명만으로 프레임워크 감지.

**장점:**

- 가장 단순한 구현
- Import 파싱 불필요

**단점:**

- 버그의 근본 원인 (현재 깨진 접근법)
- 버전 구분 불가 (`@Test`가 양쪽에 존재)
- `@Test` 사용하는 다른 프레임워크로 인한 오탐

## 결과

### 긍정적

1. **정확한 프레임워크 귀속**
   - JUnit 4 테스트 정확히 식별 및 보고
   - testcontainers-java: 250개 테스트 올바르게 귀속
   - 실제 코드베이스 상태를 반영하는 프레임워크 채택 지표

2. **엔터프라이즈 코드베이스 지원**
   - 하이브리드 JUnit 4/5 프로젝트 올바르게 분석
   - 마이그레이션 진행 추적 가능 (시간에 따른 JUnit 4 감소)

3. **아키텍처 정렬**
   - ADR-06 통합 정의 패턴 준수
   - ADR-08 `javaast` 공유 모듈 재사용
   - 기존 프레임워크 분리와 일관성 (Jest/Vitest)

4. **명확한 책임**
   - JUnit 4 전용 처리 (`@RunWith`, `@Rule`) 격리
   - JUnit 5 전용 처리 (`@Nested`, `@ParameterizedTest`) 격리

5. **중첩 클래스 감지**
   - 중첩 static 클래스 수정 (testcontainers-java 패턴)
   - 재귀적 AST 순회로 내부 테스트 클래스 적절히 처리

### 부정적

1. **프레임워크 수 증가**
   - 레지스트리에 두 개의 Java 단위 테스트 프레임워크
   - **완화**: 필요시 단순화된 뷰를 위한 프레임워크 패밀리 그룹화 추가

2. **약간의 코드 중복**
   - 공통 어노테이션 추출 로직 (`@Test` 파싱)
   - **완화**: ADR-08에 따라 `javaast` 공유 모듈로 추출

3. **엣지 케이스: 양쪽 Import 존재**
   - `org.junit.Test`와 `org.junit.jupiter.api.Test` 둘 다 import하는 파일
   - **해결**: JUnit 5 우선 (더 구체적인 import 승리)

## 참조

- [커밋 7b96c63](https://github.com/specvital/core/commit/7b96c63): feat(junit4): add JUnit 4 framework support
- [커밋 02aaed1](https://github.com/specvital/core/commit/02aaed1): fix(junit5): exclude JUnit4 test files from JUnit5 detection
- [커밋 5673d83](https://github.com/specvital/core/commit/5673d83): fix(junit4): detect tests inside nested static classes
- [이슈 #67](https://github.com/specvital/core/issues/67): add JUnit 4 framework support
- [ADR-06: 통합 Framework Definition](/ko/adr/core/06-unified-framework-definition.md)
- [ADR-08: 공유 파서 모듈](/ko/adr/core/08-shared-parser-modules.md)
