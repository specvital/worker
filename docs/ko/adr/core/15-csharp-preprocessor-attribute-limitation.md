---
title: C# 전처리기 블록 내 Attribute 감지 한계
description: tree-sitter-c-sharp 그래머 한계로 인한 조건부 컴파일 블록 내 attribute 미감지에 대한 ADR
---

# ADR-15: C# 전처리기 블록 내 Attribute 감지 한계

> 🇺🇸 [English Version](/en/adr/core/15-csharp-preprocessor-attribute-limitation.md)

| 날짜       | 작성자       | 저장소 |
| ---------- | ------------ | ------ |
| 2026-01-04 | @KubrickCode | core   |

**상태**: 승인됨

## 배경

### 문제 정의

tree-sitter-c-sharp 그래머가 attribute 사이에 위치한 전처리기 지시문(`#if`, `#else`, `#elif`)을 `preproc_if` 노드가 아닌 `ERROR` 노드로 파싱.

### 발견 경위

`fluentassertions/fluentassertions` 저장소 검증 결과:

- Ground Truth (AI 수동 분석): 5,995개 테스트
- 파서 결과: 6,009개 테스트
- 차이: +14개 (+0.23%)

Delta가 양수인 이유는 GT 분석 오류가 파서 버그보다 더 많기 때문. 실제 파서 버그로 인해 `AssertionExtensionsSpecs.cs`에서 2개 테스트 미감지.

### 기술 분석

```csharp
// 이 패턴에서 InlineData(2)가 감지되지 않음
[Theory]
[InlineData(1)]
#if NET6_0_OR_GREATER
[InlineData(2)]  // ← 파서가 놓침
#endif
public void Test(int x) { }
```

실제 tree-sitter 파싱 결과:

```
method_declaration
├── attribute_list [Theory]
├── attribute_list [InlineData(1)]
├── ERROR                          ← preproc_if가 아님!
│   └── #if NET6_0_OR_GREATER
│       └── (InlineData(2) 잘못 파싱됨)
└── public void Test()
```

**참고**: 클래스 레벨 `#if`(전체 메소드를 감싸는 경우)는 정상 작동:

```csharp
// 이 패턴은 정상 감지됨
#if NET6_0_OR_GREATER
[Fact]
public void Net6OnlyTest() { }
#endif
```

## 결정

**attribute 사이의 전처리기 블록 내 테스트 attribute 감지 미지원.**

tree-sitter-c-sharp 그래머 레벨 이슈로, SpecVital Core 파서에서 수정 불가.

### 근거

1. **그래머 레벨 한계**: tree-sitter-c-sharp가 AST를 잘못 생성하므로 파서에서 우회 불가
2. **upstream 의존성**: 수정하려면 tree-sitter-c-sharp 그래머 자체를 수정해야 함
3. **영향 범위 제한**: 대부분의 C# 프로젝트는 attribute 사이에 전처리기를 사용하지 않음

## 검토된 옵션

### 옵션 A: 한계 수용 및 문서화 (선택됨)

한계를 문서화하고 테스트로 동작을 검증.

**장점:**

- 정직한 한계 표현
- 향후 tree-sitter-c-sharp가 수정되면 자동으로 해결

**단점:**

- 특정 코드베이스에서 테스트 under-count 발생

### 옵션 B: 텍스트 기반 전처리기 확장

AST 파싱 전에 전처리기 지시문을 텍스트 레벨에서 처리.

**장점:**

- 전처리기 블록 내 attribute 감지 가능

**단점:**

- **복잡도 폭발**: 조건 평가, 중첩 처리, 여러 분기 처리 필요
- **정확도 저하**: 어떤 조건이 활성화되는지 알 수 없음
- **아키텍처 위반**: tree-sitter 기반 파싱 원칙과 충돌

### 옵션 C: tree-sitter-c-sharp 포크

그래머를 직접 수정하여 attribute 사이 전처리기를 지원.

**장점:**

- 근본적 해결

**단점:**

- **유지보수 부담**: upstream 변경사항을 지속적으로 병합해야 함
- **범위 확대**: 단일 이슈를 위해 전체 그래머를 포크
- **불확실성**: 그래머 수정의 side effect 예측 어려움

## 결과

### 긍정적

1. **아키텍처 무결성**: tree-sitter 기반 파싱 모델 유지
2. **명확한 한계**: 코드 주석과 테스트로 문서화
3. **유지보수성**: 복잡한 workaround 없음

### 부정적

1. **정확도 차이**: attribute 사이 전처리기를 사용하는 프로젝트는 테스트가 적게 카운트됨
2. **FluentAssertions 영향**: `#if` 블록 내 `[InlineData]` 사용으로 인한 under-count

### 완화 방안

1. **영향 최소**: 대부분의 프로젝트는 이 패턴을 사용하지 않음
2. **클래스 레벨 동작**: 전체 메소드를 감싸는 `#if`는 정상 작동
3. **문서화**: `GetAttributeLists()` 함수 주석에 한계 명시

## 프레임워크 영향

| 프레임워크 | 영향 패턴               | 심각도 |
| ---------- | ----------------------- | ------ |
| xUnit      | `[InlineData]` in `#if` | 낮음   |
| NUnit      | `[TestCase]` in `#if`   | 낮음   |
| MSTest     | `[DataRow]` in `#if`    | 낮음   |

대부분의 C# 테스트 프로젝트는 클래스 레벨 또는 메소드 레벨 조건부 컴파일 사용. attribute 사이에 전처리기를 삽입하는 패턴은 드묾.

## 관련 ADR

- [ADR-02: 동적 테스트 카운팅 정책](./02-dynamic-test-counting-policy.md) - 또 다른 정확도 한계
- [ADR-03: Tree-sitter AST 파싱 엔진](./03-tree-sitter-ast-parsing-engine.md) - tree-sitter 기반 파싱 원칙
- [ADR-14: 간접 Import Alias 감지 미지원](./14-indirect-import-unsupported.md) - 유사한 한계 문서화 패턴

## 참고 자료

- [tree-sitter-c-sharp GitHub](https://github.com/tree-sitter/tree-sitter-c-sharp)
- 검증 리포트: `realworld-test-report.md`
- 한계 테스트: `pkg/parser/strategies/shared/dotnetast/ast_test.go:TestGetAttributeLists_PreprocessorLimitation`
