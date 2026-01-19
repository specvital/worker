---
title: Swift Testing 프레임워크 지원
description: "Apple Swift Testing 프레임워크(@Test, @Suite 속성) 지원 ADR"
---

# ADR-17: Swift Testing 프레임워크 지원

> 🇺🇸 [English Version](/en/adr/core/17-swift-testing-framework-support.md)

| 날짜       | 작성자     | 리포지토리 |
| ---------- | ---------- | ---------- |
| 2026-01-04 | @specvital | core       |

## 컨텍스트

### 문제 상황

주요 Swift 리포지토리(예: GitHub 스타 40K+ Alamofire) 검증 중 Swift Testing 테스트 미감지. 기존 XCTest 파서 인식 범위:

- `func testXxx()` 네이밍 컨벤션
- `XCTestCase` 상속 클래스

Apple WWDC 2024 (Swift 6 / Xcode 16) 발표 Swift Testing - 근본적으로 다른 패턴:

| 패턴          | XCTest                  | Swift Testing                          |
| ------------- | ----------------------- | -------------------------------------- |
| 테스트 선언   | `func testXxx()`        | `@Test` 속성                           |
| Suite 그룹    | `XCTestCase` 서브클래스 | `@Suite` (선택적, @Test 있으면 암시적) |
| Skip 메커니즘 | `XCTSkip()` 런타임      | `@Test(.disabled)` 컴파일타임 trait    |
| Assertion     | `XCTAssert*`            | `#expect()`, `#require()`              |
| 타입 지원     | 클래스만                | Struct, Actor, Class                   |

### 영향

- Alamofire 리포지토리 57개 테스트 미감지
- iOS/macOS 프로젝트 Swift Testing 채택 증가
- Apple 공식 권장 테스트 프레임워크

### 요구사항

1. `@Test` 및 `@Suite` 속성 기반 테스트 감지
2. `@Test(.disabled)` trait에서 skip 상태 인식
3. async 테스트 함수 지원
4. `swiftast` 모듈을 통한 XCTest와 AST 유틸리티 공유
5. 기존 XCTest 감지와의 하위 호환성 유지

## 결정

**Swift Testing을 `PrioritySpecialized` 감지 우선순위의 별도 프레임워크로 구현.**

`swifttesting` 프레임워크를 `xctest`와 독립적인 definition으로 등록, `swiftast` 모듈을 통한 공통 Swift AST 유틸리티 공유.

### 감지 전략

우선순위 기반 Early-Return (ADR-04):

1. **Import 감지** (최우선): `import Testing` → Swift Testing 파서 트리거
2. **속성 감지**: `@Test`, `@Suite` 존재 확인
3. **컨텐츠 패턴**: `#expect()`, `#require()` 보조 신호

### 파서 구현

```go
func NewDefinition() *framework.Definition {
    return &framework.Definition{
        Name:      "swifttesting",
        Languages: []domain.Language{domain.LanguageSwift},
        Matchers: []framework.Matcher{
            matchers.NewImportMatcher("Testing"),
            &SwiftTestingContentMatcher{}, // @Test, @Suite, #expect
        },
        Parser:   &SwiftTestingParser{},
        Priority: framework.PrioritySpecialized, // 200
    }
}
```

### Skip 감지

`@Test(.disabled)` trait → `TestStatusSkipped` 매핑:

```go
// "@Test(.disabled)" or "@Test(.disabled(\"reason\"))"
if hasDisabledTrait(annotation) {
    return domain.TestStatusSkipped
}
```

### Async 지원

함수 시그니처에서 `async` 키워드 컨텐츠 스캔:

```swift
@Test func fetchData() async throws { ... }
```

## 검토된 옵션

### 옵션 A: 별도 프레임워크 전략 (선택됨)

`PrioritySpecialized` 우선순위의 독립 `swifttesting` 프레임워크 정의.

**장점:**

- 프레임워크 격리로 독립적 진화 가능
- `import Testing` 통한 명확한 감지
- Swift Testing trait 네이티브 지원 (`@Test(.disabled)`)
- `swiftast` 모듈 통한 코드 재사용
- Unified Framework Definition 패턴 준수 (ADR-06)

**단점:**

- Swift 프레임워크 두 개 유지보수 필요
- 혼합 파일에서 감지 오버랩 가능성

### 옵션 B: 기존 XCTest 파서 확장

XCTest definition 내 Swift Testing 패턴 추가.

**장점:**

- 단일 Swift 프레임워크 정의
- 공유 유지보수 범위

**단점:**

- 단일 책임 원칙 위반
- 근본적으로 다른 패턴에 대한 복잡한 내부 분기
- 버그 수정 시 양쪽 프레임워크 영향
- Apple의 명시적 별도 프레임워크 포지셔닝

### 옵션 C: 통합 Swift 파서

단일 Swift 파서 + 서브 프레임워크 라우팅.

**장점:**

- 최대 코드 공유
- Swift 단일 진입점

**단점:**

- 과잉 일반화 위험
- 복잡한 내부 라우팅
- 프레임워크별 엣지 케이스 누출

### 옵션 D: 패턴 기반 감지만

AST 파싱 없이 정규식 기반 감지.

**장점:**

- 경량 구현
- 빠른 실행

**단점:**

- 파라미터화 테스트 이름 추출 불가
- async 함수 감지 불가
- 중첩 suite 파싱 불가
- 프로덕션 정확도 요구사항 충족 불가

## 결과

### 긍정적

1. **프레임워크 격리**
   - Swift Testing의 XCTest 독립적 진화
   - 한쪽 파서 업데이트 시 회귀 위험 없음
   - 프레임워크별 명확한 책임

2. **정확한 감지**
   - `import Testing` → 최고 신뢰도 감지 신호
   - `@Test(.disabled)` → skip 상태 네이티브 매핑
   - 컨텐츠 스캔 통한 async 함수 감지

3. **공유 모듈 통한 코드 재사용**
   - `swiftast` 모듈의 공통 AST 유틸리티 제공
   - `swiftast` 버그 수정 → 양쪽 파서 혜택
   - Shared Parser Modules 패턴 준수 (ADR-08)

4. **미래 지향 아키텍처**
   - 추가 trait 지원 확장 가능 (`@Test(.bug())`, `@Test(.tags())`)
   - 파라미터화 테스트 준비 (`@Test(arguments:)`)
   - Apple 프레임워크 방향성 정렬

5. **기존 ADR과의 일관성**
   - Unified Framework Definition (ADR-06)
   - Early-Return Framework Detection (ADR-04)
   - Shared Parser Modules (ADR-08)

### 부정적

1. **이중 프레임워크 유지보수**
   - Swift용 별도 definition 파일 두 개
   - **완화**: 공유 `swiftast` 모듈로 중복 최소화

2. **혼합 파일 감지**
   - XCTest + Swift Testing 동시 사용 파일 (Apple 지원 시나리오)
   - **완화**: `PrioritySpecialized`로 Swift Testing 우선 감지; 명시적 import 우선

3. **초기 개발 투자**
   - 새 definition.go, matchers, 파서 구현 필요
   - **완화**: 기존 `swiftast` 모듈 활용; 확립된 패턴 준수

## 참조

- [커밋 161b650](https://github.com/specvital/core/commit/161b650): feat(swift-testing): add Apple Swift Testing framework support
- [이슈 #95](https://github.com/specvital/core/issues/95): Add Apple Swift Testing framework support
- [ADR-04: Early-Return 프레임워크 탐지](/ko/adr/core/04-early-return-framework-detection.md)
- [ADR-06: 통합 Framework Definition](/ko/adr/core/06-unified-framework-definition.md)
- [ADR-08: 공유 파서 모듈](/ko/adr/core/08-shared-parser-modules.md)
- [Swift Testing - Apple Developer](https://developer.apple.com/xcode/swift-testing)
- [Meet Swift Testing - WWDC24](https://developer.apple.com/videos/play/wwdc2024/10179/)
