---
title: 통합 Framework Definition
description: 프레임워크 구성요소를 단일 Definition 타입으로 통합하는 결정
---

# ADR-06: 통합 Framework Definition 시스템

> :us: [English Version](/en/adr/core/06-unified-framework-definition.md)

| 날짜       | 작성자       | 영향 리포지토리 |
| ---------- | ------------ | --------------- |
| 2025-12-23 | @KubrickCode | core            |

**상태**: 승인됨

## Context

### 문제 정의

기존 아키텍처는 프레임워크 구성요소가 분리된 레지스트리에 나뉘어 있는 **이중 레지스트리 패턴**을 사용했음:

1. **Matchers Registry**: 프레임워크 탐지 규칙 저장
2. **Strategies Registry**: 테스트 파일 파서 저장

이 분리로 인해 여러 문제가 발생함:

- **동기화 부담**: 새 프레임워크 추가 시 여러 곳을 수정해야 함
- **등록 취약성**: 대응하는 파서 없이 매처만 등록하기 쉬움 (또는 그 반대)
- **분산된 정의**: 프레임워크 동작이 여러 파일과 패키지에 흩어짐
- **테스트 복잡성**: 모킹 시 여러 레지스트리 조율 필요

### 요구사항

1. **단일 등록 지점**: 프레임워크의 모든 것을 정의하는 한 곳 필요
2. **자기 완결적 정의**: 모든 프레임워크 구성요소가 함께 묶여야 함
3. **타입 안전성**: 완전한 프레임워크 정의를 컴파일 타임에 검증해야 함
4. **확장성**: 최소한의 보일러플레이트로 새 프레임워크를 쉽게 추가해야 함

### 전략적 질문

프레임워크 구성요소(탐지, 설정 파싱, 테스트 파싱)를 결합도와 유지보수 부담을 최소화하도록 어떻게 구성해야 하는가?

## Decision

**모든 프레임워크 구성요소를 단일 `framework.Definition` 타입과 통합 레지스트리로 통합함.**

각 프레임워크는 다음을 묶는 하나의 `Definition` 구조체를 제공함:

- 프레임워크 식별자 (이름, 지원 언어)
- 탐지 규칙 (매처)
- 설정 파서
- 테스트 파일 파서
- 탐지 순서를 위한 우선순위

## Options Considered

### Option A: 통합 Definition (선택됨)

모든 프레임워크 구성요소를 포함하는 단일 구조체 타입임.

**장점:**

- **프레임워크당 단일 파일**: 완전한 프레임워크 정의가 하나의 `definition.go`에 있음
- **자기 문서화**: 모든 프레임워크 동작이 한 곳에서 보임
- **컴파일 타임 완전성**: 누락된 구성요소가 컴파일 에러를 발생시킴
- **간단한 등록**: `init()`에서 단일 `framework.Register()` 호출
- **쉬운 테스트**: 하나의 구조체로 전체 프레임워크 모킹

**단점:**

- 더 큰 구조체 크기 (모든 구성요소 포함)
- 모든 구성요소를 정의해야 함 (부분 프레임워크 불가)

### Option B: 이중 레지스트리 (기존)

매처와 파서를 위한 분리된 레지스트리임.

**장점:**

- 개별 구성요소에 대한 세밀한 제어
- 레지스트리당 잠재적으로 작은 메모리 사용

**단점:**

- **동기화 필요**: 각 프레임워크마다 두 레지스트리를 모두 업데이트해야 함
- **잊기 쉬움**: 한 레지스트리에만 등록하고 다른 것은 빠뜨림
- **분산된 코드**: 프레임워크 로직이 패키지 전체에 흩어짐
- **테스트 어려움**: 여러 모의 레지스트리 조율 필요

### Option C: 플러그인 시스템

런타임에 동적 플러그인 로드 방식임.

**장점:**

- 프레임워크 추가에 최대 유연성
- 새 프레임워크에 재컴파일 불필요

**단점:**

- **복잡성**: 플러그인 탐색, 로드, 생명주기 관리
- **타입 안전성 상실**: 컴파일 타임 대신 런타임 에러
- **배포 부담**: 별도 플러그인 바이너리 관리
- **과잉 설계**: 현재 요구에는 정적 등록으로 충분함

## Implementation Details

### Definition 구조

```go
type Definition struct {
    // 프레임워크 식별
    Name      string
    Languages []domain.Language

    // 탐지 구성요소
    Matchers []Matcher

    // 설정 파싱 (선택적)
    ConfigParser ConfigParser

    // 테스트 파일 파싱
    Parser Parser

    // 탐지 순서를 위한 우선순위
    Priority int
}
```

### 핵심 인터페이스

```go
// Matcher는 탐지 시그널을 평가함
type Matcher interface {
    Match(ctx context.Context, signal Signal) MatchResult
}

// ConfigParser는 프레임워크 설정 파일에서 설정을 추출함
type ConfigParser interface {
    Parse(ctx context.Context, configPath string, content []byte) (*ConfigScope, error)
}

// Parser는 소스 코드에서 테스트 정의를 추출함
type Parser interface {
    Parse(ctx context.Context, source []byte, filename string) (*domain.TestFile, error)
}
```

### 등록 패턴

각 프레임워크는 `init()`을 통해 등록함:

```go
// pkg/parser/strategies/jest/definition.go
func init() {
    framework.Register(NewDefinition())
}

func NewDefinition() *framework.Definition {
    return &framework.Definition{
        Name:      "jest",
        Languages: []domain.Language{domain.LanguageTypeScript, domain.LanguageJavaScript},
        Matchers: []framework.Matcher{
            matchers.NewImportMatcher("@jest/globals", "@jest/", "jest"),
            matchers.NewConfigMatcher("jest.config.js", "jest.config.ts"),
            &JestContentMatcher{},
        },
        ConfigParser: &JestConfigParser{},
        Parser:       &JestParser{},
        Priority:     framework.PriorityGeneric,
    }
}
```

**중요**: `init()` 트리거를 위해 blank import가 필수임:

```go
import (
    _ "github.com/specvital/core/pkg/parser/strategies/jest"
)
```

### 레지스트리 아키텍처

```
pkg/parser/
├── framework/
│   ├── definition.go     # Definition 타입과 인터페이스
│   ├── registry.go       # 단일 통합 레지스트리
│   ├── scope.go          # 설정 파일 처리용 ConfigScope
│   ├── constants.go      # 우선순위 레벨, 프레임워크 이름
│   └── matchers/         # 재사용 가능한 매처 구현
│       ├── import.go     # import 문 매처
│       ├── config.go     # 설정 파일 매처
│       └── content.go    # 콘텐츠 패턴 매처
└── strategies/
    ├── jest/definition.go
    ├── vitest/definition.go
    ├── playwright/definition.go
    └── gotesting/definition.go
```

### 우선순위 시스템

```go
const (
    PriorityGeneric     = 100  // Jest, Go testing
    PriorityE2E         = 150  // Playwright, Cypress
    PrioritySpecialized = 200  // Vitest (명시적 import 탐지 필요)
)
```

높은 우선순위의 프레임워크가 탐지 시 먼저 평가됨.

## Consequences

### Positive

1. **단일 진실의 원천**
   - 모든 프레임워크 동작이 하나의 파일에 정의됨
   - 파일 간 조율 불필요
   - 명확한 소유권과 책임

2. **보일러플레이트 감소**
   - 새 프레임워크는 `definition.go` 파일 하나만 필요
   - 재사용 가능한 매처 컴포넌트 (`matchers/` 패키지)
   - 공유 파싱 유틸리티 (`shared/jstest/` 등)

3. **향상된 유지보수성**
   - 프레임워크 변경이 단일 파일에 국한됨
   - 컴파일 타임에 완전성 검증
   - 자기 문서화 구조

4. **향상된 테스트 용이성**
   - 단일 구조체로 전체 프레임워크 모킹
   - 탐지, 설정 파싱, 테스트 파싱을 함께 테스트
   - 프레임워크별 격리된 단위 테스트

### Negative

1. **Blank Import 필요**
   - 소비자가 각 프레임워크 패키지를 명시적으로 import해야 함
   - 누락된 import는 조용히 프레임워크를 제외시킴
   - **완화**: README에 필수 import 문서화; 레지스트리 검증 고려

2. **전역 상태 의존**
   - `defaultRegistry`가 패키지 수준 변수임
   - 등록에 `init()` 순서가 영향을 줌
   - **완화**: Go는 init()이 main() 전에 실행됨을 보장; 레지스트리는 스레드 안전함

3. **전부 아니면 전무 정의**
   - 부분 프레임워크 등록 불가 (예: 매처만)
   - **완화**: 이는 의도적임; 부분 프레임워크는 이중 레지스트리에서 버그를 유발함

### 트레이드오프 요약

| 측면          | 통합 Definition  | 이중 레지스트리 |
| ------------- | ---------------- | --------------- |
| 등록          | 단일 호출        | 다중 호출       |
| 파일 구성     | 프레임워크당 1개 | 다중 파일       |
| 구성요소 결합 | 높음 (의도적)    | 낮음            |
| 유지보수 부담 | 낮음             | 높음            |
| 타입 안전성   | 컴파일 타임      | 잠재적 런타임   |

## Related ADRs

- [ADR-03: Tree-sitter AST 파싱 엔진](./03-tree-sitter-ast-parsing-engine.md) - 파서 구현
- [ADR-04: Early-Return 프레임워크 탐지](./04-early-return-framework-detection.md) - 매처를 사용하는 탐지 알고리즘

## References

- [Go `init()` 함수](https://go.dev/doc/effective_go#init) - init 함수 공식 문서
- [Accept Interfaces, Return Structs](https://bryanftan.medium.com/accept-interfaces-return-structs-in-go-d4cab29a301b) - Go 인터페이스 설계 원칙
