---
title: Rust 매크로 기반 테스트 감지
description: Rust 테스트 파일을 위한 2단계 매크로 감지 전략 구현에 대한 ADR
---

# ADR-21: Rust 매크로 기반 테스트 감지

> 🇺🇸 [English Version](/en/adr/core/21-rust-macro-test-detection)

| 날짜       | 작성자     | 레포 |
| ---------- | ---------- | ---- |
| 2025-12-27 | @specvital | core |

## 컨텍스트

### 문제 상황

SpecVital Core의 Rust 파서가 표준 `#[test]` 속성 함수만 감지. 성숙한 Rust 코드베이스에서 일반적인 매크로 기반 패턴으로 정의된 테스트 미감지.

### 발견 경위

`BurntSushi/ripgrep` 리포지토리 검증 결과 상당한 탐지 격차 확인:

| 지표                          | 값                    |
| ----------------------------- | --------------------- |
| 실제 테스트 수 (`cargo test`) | 1,111개               |
| 파서 탐지 결과                | 436개                 |
| **차이**                      | **-675 (61% 미탐지)** |

근본 원인 분석으로 두 가지 미탐지 테스트 범주 식별:

**범주 1: 이름 기반 매크로** (~330개 테스트)

이름에 "test"가 포함된 테스트 함수 생성 매크로:

```rust
// ripgrep/tests/integration.rs
rgtest!(f1, |dir: Dir, mut cmd: TestCommand| {
    // 테스트 본문
});
```

**범주 2: 정의 기반 매크로** (~264개 테스트)

내부적으로 `#[test]`로 확장되는 정의를 가진 매크로:

```rust
// 동일 파일 내 정의
macro_rules! syntax {
    ($name:ident, $re:expr, $hay:expr) => {
        #[test]
        fn $name() {
            // ...
        }
    };
}

// 사용
syntax!(test_literal, r"foo", "foo");
```

### 기술적 제약

1. **단일 파일 파싱 제약** (ADR-14): Core 파서가 크로스 파일 의존성 해석 없이 파일별 독립 작동
2. **Tree-sitter 한계**: AST 파싱이 구조 제공하지만 매크로 확장 불가
3. **Rust 매크로 유형**:
   - **선언적 매크로** (`macro_rules!`): 동일 파일 내 정의 분석 가능
   - **절차적 속성 매크로** (`rstest`, `test_case`): 외부 크레이트 구현, 컴파일러 확장 없이 불투명

### 전략적 필요성

61% 탐지 실패는 Rust 생태계에 대한 플랫폼 신뢰도 훼손. 솔루션은 아키텍처 제약 내에서 정확도 개선 균형 필요.

## 결정

**매크로 이름과 동일 파일 매크로 정의를 모두 분석하는 2단계 매크로 기반 테스트 감지 전략 구현.**

### 1단계: 이름 기반 휴리스틱

매크로 이름에 "test"가 포함된 매크로 호출 탐지 (대소문자 무시):

```rust
// 탐지됨: 매크로 이름에 "test" 포함
rgtest!(test_name, |...| { ... });
test_case!(name, input, expected);
```

### 2단계: 정의 분석

동일 파일 내 `macro_rules!` 정의에서 매크로 본문에 `#[test]` 속성 확장 포함 여부 분석:

```rust
// 1단계: 매크로 정의 수집
macro_rules! syntax {  // <- 정의 발견
    ($name:ident, $re:expr, $hay:expr) => {
        #[test]  // <- #[test]로 확장
        fn $name() { ... }
    };
}

// 2단계: 테스트 생성 매크로 호출 카운트
syntax!(test_one, ...);   // <- 테스트로 카운트
syntax!(test_two, ...);   // <- 테스트로 카운트
```

### 구현

`pkg/parser/strategies/cargotest/definition.go`에서 2패스 AST 분석:

```go
func parseRustAST(root *sitter.Node, source []byte) []domain.TestSuite {
    // 패스 1: #[test] 생성 매크로 정의 수집
    macroRegistry := collectTestMacroDefinitions(root, source)

    // 패스 2: 레지스트리 + 이름 휴리스틱으로 모든 노드 순회
    var tests []domain.Test
    walkTree(root, func(node *sitter.Node) {
        switch node.Type() {
        case "function_item":
            if hasTestAttribute(node) {
                tests = append(tests, extractAttributeTest(node))
            }
        case "macro_invocation":
            if isTestMacro(node, macroRegistry) {
                tests = append(tests, extractMacroTest(node))
            }
        }
    })
    return tests
}
```

주요 함수:

- `collectTestMacroDefinitions()`: 테스트 생성 매크로 레지스트리 구축을 위한 첫 번째 패스
- `tokenTreeHasTestAttribute()`: `macro_rules!` 본문에서 `#[test]` 재귀 검색
- `isTestMacro()`: 레지스트리 먼저 확인, 이름 휴리스틱으로 폴백

### 외부 매크로 폴백

외부 크레이트의 절차적 속성 매크로(`rstest`, `test_case`)는 크레이트 해석 없이 구현 분석 불가하므로 이름 기반 휴리스틱 폴백 사용.

## 고려한 옵션

### 옵션 A: 2단계 탐지 (선택됨)

이름 기반 휴리스틱과 동일 파일 `macro_rules!` 정의 분석 결합.

**장점:**

- 일반적 패턴에 높은 정확도: 명명 규칙 및 정의 기반 매크로 모두 커버
- 단일 파일 제약 유지: 크로스 파일 해석 불필요
- 결정론적: 동일 파일 분석은 완전히 결정론적
- 합리적 외부 매크로 처리: 이름 휴리스틱으로 `rstest`, `test_case` 호출 포착

**단점:**

- 정의 분석 범위 제한: 동일 파일의 `macro_rules!`에서만 작동
- 휴리스틱이 엣지 케이스 놓칠 수 있음: 이름에 "test" 없고 다른 파일에 정의된 매크로
- 2패스 오버헤드: 두 번의 AST 순회 필요

### 옵션 B: 이름 전용 휴리스틱

매크로 이름에 "test"가 있는 매크로 호출만 탐지.

**장점:**

- 간단한 구현: 단일 패스, 직관적 패턴 매칭
- 모든 매크로 유형에 적용: 이름 휴리스틱이 선언적 및 절차적 매크로 모두에 적용

**단점:**

- 정의 기반 매크로 미탐지: `syntax!`, `matches!` 등 미탐지 (ripgrep에서 ~264개 테스트)
- 높은 미탐지율: 매크로 기반 테스트의 약 50%만 포착
- 명명 규칙 의존: 프로젝트가 "test" 명명 규칙을 따른다고 가정

### 옵션 C: 컴파일러를 통한 전체 매크로 확장

`rustc` 또는 `cargo expand`를 호출하여 완전히 확장된 소스 코드 확보.

**장점:**

- 100% 정확도: 컴파일러 확장이 모든 매크로 유형 올바르게 처리
- 휴리스틱 불필요: 컴파일러로부터 실측값

**단점:**

- 컴파일 필요: 의존성 해석, 크레이트 다운로드, 빌드 필요
- 성능 영향: 정적 파싱의 밀리초 대비 크레이트당 수초~수분
- 환경 의존성: Rust 툴체인 필요, 불완전한 프로젝트에서 실패 가능
- 정적 분석 원칙 위반 (ADR-01): 정적에서 동적 분석으로 전환

### 옵션 D: 외부 크레이트 해석

외부 파일 및 크레이트에서 `macro_rules!` 정의를 해석하기 위한 의존성 그래프 구축.

**장점:**

- 옵션 A보다 높은 커버리지: 동일 크레이트 내 다른 파일에 정의된 매크로 포착

**단점:**

- 단일 파일 제약 위반 (ADR-14): 다중 파일 조정 필요
- 복잡성 폭발: 모듈 해석, `use` 문, 재내보내기
- 상당한 아키텍처 변경: 현재 파일별 접근 방식과 근본적으로 다름

## 결과

### 긍정적

1. **극적인 정확도 개선**
   - ripgrep: 436 → ~1,030 탐지 테스트 (61% 미탐지 → ~7% 미탐지)
   - 실제 Rust 테스트 패턴 대부분 커버

2. **아키텍처 보존**
   - 단일 파일 파싱 제약 유지
   - 외부 의존성 또는 컴파일 불필요
   - ADR-14 경계 결정과 일관성

3. **점진적 개선**
   - 두 단계를 독립적으로 활성화/조정 가능
   - 이름 휴리스틱이 기준선 제공; 정의 분석이 정밀도 추가

4. **프레임워크 독립적**
   - 커스텀 프로젝트 매크로(ripgrep의 `rgtest!`)와 작동
   - 외부 프레임워크(`rstest`, `test_case`)도 이름 휴리스틱으로 작동

### 부정적

1. **동일 파일 범위 제한**
   - 별도 파일의 `macro_rules!` 정의 분석 불가
   - 중앙화된 테스트 유틸리티가 있는 프로젝트에서 격차 가능
   - 완화책: 이름 휴리스틱이 일반적 패턴에 폴백 제공

2. **절차적 매크로 불투명성**
   - `rstest`, `test_case` 확장 로직 분석 불가
   - 탐지에 명명 규칙 의존
   - 완화책: 이러한 프레임워크는 "test" 명명 규칙 준수; 미탐지 드뭄

3. **2패스 성능 비용**
   - 정의 수집을 위한 추가 AST 순회
   - 완화책: 오버헤드 최소 (Rust 파일에서 ~10-20%)

### 탐지 커버리지 매트릭스

| 매크로 유형        | 동일 파일 정의 | 다른 파일  | 외부 크레이트 |
| ------------------ | -------------- | ---------- | ------------- |
| 이름에 "test" 포함 | **탐지됨**     | **탐지됨** | **탐지됨**    |
| `#[test]`로 확장   | **탐지됨**     | 미탐지     | 미탐지        |
| 둘 다 아님         | 미탐지         | 미탐지     | 미탐지        |

## 참조

- [이슈 #73: cargo-test: add macro-based test detection for Rust](https://github.com/specvital/core/issues/73)
- [이슈 #89: cargotest - detect test macros by analyzing same-file macro_rules! definitions](https://github.com/specvital/core/issues/89)
- [커밋 caa4d1b: fix(cargo-test): add macro-based test detection for Rust](https://github.com/specvital/core/commit/caa4d1b)
- [커밋 4f3d697: feat(cargotest): detect test macros by analyzing same-file macro_rules! definitions](https://github.com/specvital/core/commit/4f3d697)
- [ADR-14: 간접 Import Alias 감지 미지원](/ko/adr/core/14-indirect-import-unsupported)
- [ADR-03: Tree-sitter AST 파싱 엔진](/ko/adr/core/03-tree-sitter-ast-parsing-engine)
- [Rust 테스트 문서](https://doc.rust-lang.org/book/ch11-01-writing-tests.html)
