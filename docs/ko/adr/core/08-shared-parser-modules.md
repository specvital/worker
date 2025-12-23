---
title: 공유 파서 모듈
description: 테스트 프레임워크 간 공유되는 언어 수준 AST 유틸리티 모듈 결정
---

# ADR-08: 공유 파서 모듈

> :us: [English Version](/en/adr/core/08-shared-parser-modules.md)

| 날짜       | 작성자       | 영향 리포지토리 |
| ---------- | ------------ | --------------- |
| 2025-12-23 | @KubrickCode | core            |

**상태**: 승인됨

## Context

### 문제 정의

같은 언어 계열의 테스트 프레임워크들은 공통 파싱 패턴을 공유함:

1. **JavaScript/TypeScript**: Jest, Vitest, Mocha, Cypress, Playwright 모두 `describe()`/`it()` 패턴 사용
2. **Java**: JUnit 5, TestNG가 어노테이션 기반 테스트 탐색 공유 (`@Test`, `@Disabled`)
3. **C#**: xUnit, NUnit, MSTest가 속성 기반 패턴 공유 (`[Fact]`, `[Test]`)

공유 유틸리티 없이는 각 프레임워크 파서가 다음을 중복 구현해야 함:

- AST 순회 로직
- 노드 추출 헬퍼
- 상태/수정자 파싱
- 문자열 언이스케이프 및 포매팅

### 요구사항

1. **코드 재사용**: 유사한 패턴을 가진 프레임워크 간 중복 제거
2. **일관된 동작**: 관련 프레임워크 간 동일한 파싱 로직 보장
3. **프레임워크 독립성**: 공유 모듈이 프레임워크별 동작을 강제하지 않아야 함
4. **유지보수성**: 공유 코드의 버그 수정이 모든 소비자에게 혜택이 되어야 함
5. **테스트 용이성**: 공유 유틸리티가 독립적으로 테스트 가능해야 함

### 전략적 질문

파싱 유틸리티를 재사용을 극대화하면서 프레임워크별 유연성을 유지하도록 어떻게 구성해야 하는가?

## Decision

**`pkg/parser/strategies/shared/` 아래에 언어 수준 공유 파서 모듈을 생성함.**

각 모듈은 해당 언어 계열 프레임워크를 위한 AST 순회 유틸리티와 공통 파싱 함수를 제공함. 프레임워크별 파서는 이러한 유틸리티를 자체 탐지 로직과 조합함.

## Options Considered

### Option A: 언어 수준 공유 모듈 (선택됨)

집중된 책임을 가진 언어 계열별 공유 코드 구성임.

**장점:**

- **명확한 경계**: 각 모듈이 하나의 언어 AST 패턴 처리
- **조합 가능**: 프레임워크가 필요한 유틸리티만 선택
- **테스트 가능**: 유틸리티를 격리 테스트
- **일관성**: 관련 프레임워크 간 동일한 파싱 동작

**단점:**

- 공유 모듈과 소비자 간의 긴밀한 결합
- 공유 코드 변경이 여러 프레임워크에 영향
- 프레임워크별 동작 누출에 주의 필요

### Option B: 프레임워크별 중복

각 프레임워크가 파싱을 처음부터 자체 구현함.

**장점:**

- 프레임워크 간 완전한 격리
- 변경에 조율 불필요
- 프레임워크가 특정 패턴에 최적화 가능

**단점:**

- **대규모 중복**: 같은 `describe()`/`it()` 파싱 로직이 Jest, Vitest, Mocha, Cypress, Playwright에서 반복됨
- **일관되지 않은 동작**: 버그 수정이 불균등하게 적용됨
- **유지보수 부담**: 같은 버그가 잠재적으로 여러 곳에서 수정됨

### Option C: 단일 범용 파서

모든 프레임워크를 설정으로 처리하는 하나의 파서임.

**장점:**

- 최대 코드 공유
- 모든 파싱 로직의 단일 위치

**단점:**

- **과도한 일반화**: 모든 프레임워크의 엣지 케이스 처리 강제
- **복잡한 설정**: 각 프레임워크에 광범위한 커스터마이징 필요
- **취약함**: 한 프레임워크를 위한 변경이 다른 것을 깨뜨릴 수 있음

## Architecture

### 모듈 구조

```
pkg/parser/strategies/shared/
├── jstest/           # JavaScript/TypeScript 테스트 프레임워크
│   ├── parser.go     # 메인 진입점: Parse()
│   ├── helpers.go    # AST 추출 유틸리티
│   └── constants.go  # 공유 상수 (함수명, 수정자)
├── javaast/          # Java 프레임워크
│   └── ast.go        # 어노테이션/메서드 유틸리티
├── dotnetast/        # C# 프레임워크
│   └── ast.go        # 속성/메서드 유틸리티
├── kotlinast/        # Kotlin 프레임워크
│   └── ast.go        # 어노테이션 유틸리티
├── pyast/            # Python 프레임워크
│   └── ast.go        # 데코레이터/함수 유틸리티
├── rubyast/          # Ruby 프레임워크
│   ├── ast.go        # 메서드 호출 유틸리티
│   └── helpers.go    # 블록 파싱
├── swiftast/         # Swift 프레임워크
│   └── ast.go        # 메서드 유틸리티
├── phpast/           # PHP 프레임워크
│   └── ast.go        # 어노테이션/메서드 유틸리티
└── configutil/       # 설정 파일 파싱
    └── strings.go    # 문자열 추출 유틸리티
```

### 책임 분리

| 계층                | 책임              | 예시                                             |
| ------------------- | ----------------- | ------------------------------------------------ |
| **공유 모듈**       | 언어 AST 패턴     | `jstest.ParseNode()`, `javaast.GetAnnotations()` |
| **프레임워크 파서** | 프레임워크별 탐지 | Jest의 `jest.fn()` 매처                          |
| **프레임워크 정의** | 등록 및 매처      | `framework.Register()`                           |

### jstest 모듈 (JavaScript/TypeScript)

여러 프레임워크를 지원하는 가장 복잡한 공유 모듈임:

**소비자**: Jest, Vitest, Mocha, Cypress, Playwright

**핵심 함수**:

```go
// 메인 진입점 - 전체 파일 파싱
func Parse(ctx context.Context, source []byte, filename string, framework string) (*domain.TestFile, error)

// 재귀적 AST 순회
func ParseNode(node *sitter.Node, source []byte, filename string, file *domain.TestFile, currentSuite *domain.TestSuite)

// 테스트/스위트 생성
func ProcessTest(callNode, args *sitter.Node, source []byte, filename string, file *domain.TestFile, parentSuite *domain.TestSuite, status domain.TestStatus, modifier string)
func ProcessSuite(callNode, args *sitter.Node, source []byte, filename string, file *domain.TestFile, parentSuite *domain.TestSuite, status domain.TestStatus, modifier string)

// .each() 파라미터화된 테스트
func ProcessEachTests(callNode *sitter.Node, testCases []string, nameTemplate string, ...)
func ProcessEachSuites(callNode *sitter.Node, testCases []string, nameTemplate string, callback *sitter.Node, ...)
```

**공유 상수**:

```go
const (
    FuncDescribe = "describe"
    FuncIt       = "it"
    FuncTest     = "test"
    FuncContext  = "context"    // Mocha TDD
    FuncSpecify  = "specify"    // Mocha TDD
    FuncSuite    = "suite"      // Mocha TDD
    FuncBench    = "bench"      // Vitest benchmark

    ModifierOnly = "only"
    ModifierSkip = "skip"
    ModifierTodo = "todo"
    ModifierEach = "each"
)
```

### javaast 모듈 (Java)

**소비자**: JUnit 5, TestNG

**핵심 함수**:

```go
// 어노테이션 추출
func GetAnnotations(modifiers *sitter.Node) []*sitter.Node
func GetAnnotationName(annotation *sitter.Node, source []byte) string
func HasAnnotation(modifiers *sitter.Node, source []byte, annotationName string) bool
func GetAnnotationArgument(annotation *sitter.Node, source []byte) string

// 클래스/메서드 유틸리티
func GetClassName(node *sitter.Node, source []byte) string
func GetMethodName(node *sitter.Node, source []byte) string
func GetClassBody(node *sitter.Node) *sitter.Node
func GetModifiers(node *sitter.Node) *sitter.Node
```

### dotnetast 모듈 (C#)

**소비자**: xUnit, NUnit, MSTest

**핵심 함수**:

```go
// 속성 추출
func GetAttributeLists(node *sitter.Node) []*sitter.Node
func GetAttributes(attributeLists []*sitter.Node) []*sitter.Node
func GetAttributeName(attribute *sitter.Node, source []byte) string
func HasAttribute(attributeLists []*sitter.Node, source []byte, attributeName string) bool

// 문자열 유틸리티
func ExtractStringContent(node *sitter.Node, source []byte) string
func ParseAssignmentExpression(argNode *sitter.Node, source []byte) (string, string)

// 파일 명명 규칙
func IsCSharpTestFileName(filename string) bool
```

## Usage Pattern

프레임워크 파서는 프레임워크별 동작을 추가하면서 공유 모듈에 위임함:

```go
// pkg/parser/strategies/jest/definition.go
type JestParser struct{}

func (p *JestParser) Parse(ctx context.Context, source []byte, filename string) (*domain.TestFile, error) {
    // 공유 모듈에 위임
    return jstest.Parse(ctx, source, filename, "jest")
}

// pkg/parser/strategies/vitest/definition.go
type VitestParser struct{}

func (p *VitestParser) Parse(ctx context.Context, source []byte, filename string) (*domain.TestFile, error) {
    // 같은 공유 모듈, 다른 프레임워크 이름
    return jstest.Parse(ctx, source, filename, "vitest")
}
```

변형이 더 많은 언어의 경우, 프레임워크가 유틸리티를 선택적으로 사용함:

```go
// pkg/parser/strategies/junit5/definition.go
func parseTestMethod(node *sitter.Node, source []byte, ...) *domain.Test {
    modifiers := javaast.GetModifiers(node)
    annotations := javaast.GetAnnotations(modifiers)

    // 프레임워크별 어노테이션 처리
    for _, ann := range annotations {
        name := javaast.GetAnnotationName(ann, source)
        switch name {
        case "Test", "ParameterizedTest", "RepeatedTest":
            isTest = true
        case "Disabled":
            status = domain.TestStatusSkipped
        }
    }
    // ...
}
```

## Consequences

### Positive

1. **상당한 코드 재사용**
   - jstest 모듈이 5개 이상의 프레임워크에서 공유됨
   - dotnetast 모듈이 3개 프레임워크에서 공유됨
   - javaast 모듈이 2개 프레임워크에서 공유됨

2. **일관된 파싱 동작**
   - `describe.skip()`이 Jest, Vitest, Mocha에서 동일하게 파싱됨
   - `@Disabled` 어노테이션이 JUnit 5, TestNG에서 일관되게 처리됨

3. **중앙 집중화된 버그 수정**
   - `jstest.ProcessEachTests()` 수정이 모든 JavaScript 프레임워크에 혜택
   - 문자열 언이스케이프가 한 번 수정되어 어디서나 적용됨

4. **명확한 계층화**
   - 공유 모듈: AST 패턴 (언어별)
   - 프레임워크 파서: 탐지 및 등록 (프레임워크별)

### Negative

1. **프레임워크 간 결합**
   - 공유 모듈의 버그가 여러 프레임워크에 영향
   - **완화**: 공유 모듈에 대한 포괄적인 테스트 커버리지

2. **잠재적 과도한 일반화**
   - 프레임워크별 코드를 공유 모듈에 추가할 위험
   - **완화**: 언어 전용 패턴을 강제하는 코드 리뷰

3. **암묵적 의존성**
   - 프레임워크 동작이 공유 모듈 구현에 의존함
   - **완화**: 공유 모듈 계약을 명확히 문서화

### 트레이드오프 요약

| 측면        | 공유 모듈 | 프레임워크별 | 범용 파서 |
| ----------- | --------- | ------------ | --------- |
| 코드 재사용 | 우수      | 없음         | 최대      |
| 격리        | 보통      | 완전         | 없음      |
| 유연성      | 높음      | 최대         | 낮음      |
| 유지보수    | 보통      | 높음         | 낮음      |
| 일관성      | 높음      | 가변적       | 최대      |

## Related ADRs

- [ADR-03: Tree-sitter AST 파싱 엔진](./03-tree-sitter-ast-parsing-engine.md) - 공유 AST 유틸리티의 기반
- [ADR-06: 통합 Framework Definition](./06-unified-framework-definition.md) - 프레임워크가 공유 모듈을 조합하는 방식

## References

- [DRY 원칙](https://en.wikipedia.org/wiki/Don%27t_repeat_yourself)
- [상속보다 조합](https://en.wikipedia.org/wiki/Composition_over_inheritance)
