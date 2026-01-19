---
title: Java 21+ 암시적 클래스 감지
description: JUnit5 파서를 확장하여 Java 21+ 암시적 클래스(JEP 445/JEP 463) 지원에 대한 ADR
---

# ADR-20: Java 21+ 암시적 클래스 감지

> 🇺🇸 [English Version](/en/adr/core/20-java21-implicit-class-detection)

| 날짜       | 작성자     | 레포 |
| ---------- | ---------- | ---- |
| 2026-01-04 | @specvital | core |

## 컨텍스트

### 문제 상황

SpecVital Core의 JUnit5 파서가 `@Test` 어노테이션 메서드를 찾기 위해 `class_declaration` 노드만 순회. Java 21에서 도입된 암시적 클래스(JEP 445, Java 22에서 JEP 463으로 최종화)는 명시적 클래스 래퍼 없이 파일 레벨에 메서드 존재 가능.

### 배경

Java 21+ 암시적 선언 클래스(JEP 445/JEP 463)의 특징:

- 명시적 클래스 선언 없는 소스 파일
- 파일 레벨에 직접 메서드 선언
- 컴파일 시점에 컴파일러가 자동으로 무명 최상위 클래스로 래핑

이 기능은 초보자 친화적 Java 프로그램을 목표로 하지만 테스트 파일 구성도 간결하게 가능:

**전통적 Java (지원됨):**

```java
public class HelloTests {
    @Test
    void testHello() {
        // ...
    }
}
```

**Java 21+ 암시적 클래스 (이전에 미지원):**

```java
// HelloTests.java - 클래스 선언 없음
import org.junit.jupiter.api.Test;

@Test
void testHello() {
    // ...
}
```

| 패턴          | AST 구조                                               | 파서 상태     |
| ------------- | ------------------------------------------------------ | ------------- |
| 전통적 클래스 | `program` → `class_declaration` → `method_declaration` | 지원됨        |
| 암시적 클래스 | `program` → `method_declaration`                       | 이전에 미지원 |

### 기술적 분석

tree-sitter Java 문법이 암시적 클래스를 올바르게 파싱하여 `program` 노드 직접 하위에 `method_declaration` 노드 생성. 제한은 순전히 파서 순회 로직에 있었으며 문법 지원 문제 아님.

```
전통적:
program
└── class_declaration ("HelloTests")
    └── class_body
        └── method_declaration ("testHello")
            └── modifiers
                └── marker_annotation ("@Test")

암시적 클래스:
program
└── method_declaration ("testHello")
    └── modifiers
        └── marker_annotation ("@Test")
```

### 요구사항

1. `program` 노드 하위의 `@Test` 메서드 탐지 (파일 레벨 메서드)
2. 파일명 기반 합성 `TestSuite` 생성
3. 혼합 시나리오 처리 (명시적 클래스 + 파일 레벨 메서드)
4. 전통적 클래스 패턴과의 하위 호환성 유지
5. 전통적 파일에 대한 성능 저하 없음

## 결정

**JUnit5 파서를 확장하여 `program` 노드 하위의 `method_declaration` 노드 순회.**

`parseTestClasses()` 함수 확장:

1. `program` 루트 노드의 직접 자식인 `method_declaration` 확인
2. 파일명을 사용하여 합성 `TestSuite` 생성 (예: `HelloTests.java` → `HelloTests`)
3. 기존 인프라를 사용하여 어노테이션 처리 및 테스트 메서드 추출

### 구현

```go
// pkg/parser/strategies/junit5/definition.go

func parseTestClasses(root *sitter.Node, source []byte, filename string) []domain.TestSuite {
    var suites []domain.TestSuite
    var implicitClassTests []domain.Test

    parser.WalkTree(root, func(node *sitter.Node) bool {
        switch node.Type() {
        case javaast.NodeClassDeclaration:
            // 전통적: class_declaration 노드
            if suite := parseTestClassWithDepth(node, source, filename, 0); suite != nil {
                suites = append(suites, *suite)
            }
            return false

        case javaast.NodeMethodDeclaration:
            // 신규: Java 21+ 암시적 클래스 처리
            if node.Parent() != nil && node.Parent().Type() == "program" {
                if test := parseTestMethod(node, source, filename, domain.TestStatusActive, ""); test != nil {
                    implicitClassTests = append(implicitClassTests, *test)
                }
            }
        }
        return true
    })

    // 암시적 클래스 테스트를 위한 합성 스위트 생성
    if len(implicitClassTests) > 0 {
        suites = append(suites, domain.TestSuite{
            Name:     getImplicitClassName(filename),
            Status:   domain.TestStatusActive,
            Location: parser.GetLocation(root, filename),
            Tests:    implicitClassTests,
        })
    }

    return suites
}
```

### 스위트 명명 전략

| 파일명                                | 합성 스위트명      |
| ------------------------------------- | ------------------ |
| `HelloTests.java`                     | `HelloTests`       |
| `UserServiceTest.java`                | `UserServiceTest`  |
| `src/test/java/IntegrationTests.java` | `IntegrationTests` |

컴파일러의 암시적 클래스 명명 동작과 일치.

## 고려한 옵션

### 옵션 A: 기존 파서 확장 (선택됨)

기존 `parseTestClasses()` 함수에 프로그램 레벨 메서드 순회 추가.

**장점:**

- 최소 코드 변경 (단일 파일에서 약 50줄)
- 기존 어노테이션 파싱 및 메서드 추출 재사용
- 전통적 파일에 대한 성능 영향 없음
- 모든 JUnit5 패턴에서 일관된 동작
- 공유 파서 모듈 패턴(ADR-08) 준수

**단점:**

- 합성 스위트 명명이 사용자 기대와 다를 수 있음
- 혼합 파일 처리(명시적 + 암시적)로 엣지 케이스 복잡성 추가

### 옵션 B: 별도 암시적 클래스 파서

독립적인 `implicit_class_parser.go` 모듈 생성.

**장점:**

- 깔끔한 관심사 분리
- 암시적 클래스 처리의 독립적 진화
- 전통적 파싱에 대한 회귀 위험 없음

**단점:**

- 기존 파서와 약 80% 코드 중복
- DRY 원칙 위반
- 버그 수정을 양쪽 코드 경로에 적용 필요
- 유지보수 부담 증가
- 기존 패턴(ADR-08) 위반

### 옵션 C: 미지원 (명시적 클래스 요구)

제한 사항 문서화 및 사용자에게 명시적 클래스로 테스트 래핑 요구.

**장점:**

- 구현 노력 제로
- 코드 변경 없음

**단점:**

- 유효한 Java 21+ 언어 기능 무시
- 최신 코드베이스에 대한 사용자 불편
- Java 21+ 지원 도구 대비 경쟁 열위
- 파서가 구식으로 인식

### 옵션 D: 2패스 파일 탐지

첫 번째 패스에서 파일 유형 탐지, 두 번째 패스에서 특화된 파싱 적용.

**장점:**

- 명시적 파일 유형 결정
- 유형별 최적화 가능

**단점:**

- 2배 파싱 오버헤드
- 불필요한 복잡성
- 단순한 문제에 대한 과잉 엔지니어링

## 결과

### 긍정적

1. **Java 21+ 호환성**
   - 암시적 클래스 패턴 완전 지원
   - 파서가 언어 진화와 함께 최신 상태 유지
   - 최신 Java 코드베이스에 대한 사용자 불편 없음

2. **최소 구현 위험**
   - 단일 함수 수정
   - 기존 테스트 인프라 재사용
   - 전통적 파일 처리 변경 없음

3. **아키텍처 일관성**
   - 공유 파서 모듈 패턴(ADR-08) 준수
   - JUnit5 테스트 추출을 위한 단일 코드 경로
   - 버그 수정이 양쪽 패턴에 자동 적용

4. **직관적 동작**
   - 파일명 기반 스위트 명명이 컴파일러 동작과 일치
   - 사용자가 파일명으로 출력 예측 가능
   - "파일 = 테스트 스위트" 정신 모델과 일관성

### 부정적

1. **합성 스위트 명명**
   - 스위트 이름이 명시적 선언이 아닌 파일명에서 파생
   - 완화책: Java 컴파일러의 암시적 클래스 명명과 일치; 사용자에게 직관적

2. **혼합 파일 엣지 케이스**
   - 명시적 클래스와 파일 레벨 메서드가 모두 있는 파일은 신중한 처리 필요
   - 완화책: 명시적 클래스 먼저 처리; 파일 레벨 메서드는 별도 그룹; 실제로 드문 패턴

3. **Java 버전 탐지 부재**
   - 파서가 Java 버전 호환성 검증하지 않음
   - 완화책: tree-sitter가 버전과 무관하게 문법 파싱; 런타임 검증은 사용자 책임

## 참조

- [이슈 #101: junit5 - add Java 21+ implicit class test detection](https://github.com/specvital/core/issues/101)
- [커밋 d7c1218: feat(junit5): add Java 21+ implicit class test detection](https://github.com/specvital/core/commit/d7c1218)
- [JEP 445: Unnamed Classes and Instance Main Methods](https://openjdk.org/jeps/445)
- [JEP 463: Implicitly Declared Classes and Instance Main Methods](https://openjdk.org/jeps/463)
- [ADR-03: Tree-sitter AST 파싱 엔진](/ko/adr/core/03-tree-sitter-ast-parsing-engine)
- [ADR-08: 공유 파서 모듈](/ko/adr/core/08-shared-parser-modules)
