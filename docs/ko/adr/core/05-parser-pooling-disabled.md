---
title: 파서 풀링 비활성화
description: tree-sitter 취소 플래그 버그로 인한 파서 풀링 비활성화 결정
---

# ADR-05: 파서 풀링 비활성화

> 🇺🇸 [English Version](/en/adr/core/05-parser-pooling-disabled.md)

| 날짜       | 작성자       | 영향 리포지토리 |
| ---------- | ------------ | --------------- |
| 2025-12-23 | @KubrickCode | core            |

**상태**: 승인됨

## Context

### 문제 정의

Tree-sitter 파서를 성능 최적화를 위해 `sync.Pool`로 풀링했으나, 재현하기 어려운 간헐적 테스트 실패가 발생함.

### 근본 원인

`ParseCtx()` 실행 중 컨텍스트가 취소되면:

1. Tree-sitter가 내부 취소 플래그를 설정함
2. 파서가 풀에 반환될 때 이 플래그가 **제대로 리셋되지 않음**
3. 해당 파서의 후속 재사용 시 **"operation limit was hit"** 오류로 실패함

### 영향

- CI/CD 파이프라인에서 간헐적 테스트 실패 발생
- 프로덕션 환경에서 비결정적 동작 유발
- 간헐적 특성으로 인한 디버깅 복잡성 증가

### 전략적 질문

합리적인 성능을 유지하면서 신뢰성을 보장하려면 tree-sitter 파서 생명주기를 어떻게 관리해야 하는가?

## Decision

**파서 풀링을 비활성화함. 언어 문법은 `sync.Once`로 캐싱하면서 사용마다 새 파서를 생성함.**

이 접근법의 특징:

- 취소 플래그 버그를 완전히 제거함
- 주요 성능 최적화(문법 캐싱)는 유지함
- 파싱당 ~10µs 오버헤드와 보장된 신뢰성을 교환함

## Options Considered

### Option A: 사용마다 새 파서 생성 (선택됨)

각 파싱 작업마다 새 파서를 생성하는 방식임.

**장점:**

- **신뢰성 보장**: 파싱 작업 간 상태 누수 없음
- **단순한 구현**: 풀 관리 복잡성 없음
- **예측 가능한 동작**: 각 파싱이 독립적임

**단점:**

- **파싱당 오버헤드**: 파일당 ~10µs 할당 비용 발생
- **GC 압력 증가**: 새 할당이 가비지 컬렉션 작업 증가시킴

### Option B: Tree-sitter 버그 업스트림 수정

tree-sitter C 라이브러리에 수정사항을 기여하는 방식임.

**장점:**

- 근본 원인을 해결함
- 전체 tree-sitter 생태계에 이익이 됨

**단점:**

- **외부 의존성**: 수정 일정이 우리 통제 하에 없음
- **유지보수 부담**: 업스트림 변경사항을 추적해야 함
- **불확실한 수용**: PR이 수용되지 않거나 수개월 걸릴 수 있음

### Option C: 수동 플래그 리셋

재사용 전 파서 상태를 리셋하는 우회 방법 구현 방식임.

**장점:**

- 풀링 성능 이점을 유지함

**단점:**

- **취약함**: tree-sitter 내부 구현 세부사항에 의존함
- **유지보수 위험**: tree-sitter 업데이트로 깨질 수 있음
- **불완전함**: 모든 엣지 케이스를 다루지 못할 수 있음

## Implementation Details

### 현재 아키텍처

```
pkg/parser/tspool/
├── pool.go         # 파서 생성, 언어 문법 캐싱
└── pool_test.go    # 동시성 테스트 (레이스 탐지)
```

### 파서 생성

사용마다 새 파서를 생성함:

```go
func Get(lang domain.Language) *sitter.Parser {
    initLanguages()
    parser := sitter.NewParser()
    parser.SetLanguage(GetLanguage(lang))
    return parser
}
```

### 언어 문법 캐싱

비용이 높은 문법 초기화는 `sync.Once`로 캐싱됨:

```go
var (
    goLang   *sitter.Language
    jsLang   *sitter.Language
    // ... 모든 지원 언어
    langOnce sync.Once
)

func initLanguages() {
    langOnce.Do(func() {
        goLang = golang.GetLanguage()
        jsLang = javascript.GetLanguage()
        // ...
    })
}
```

**이유**: 문법 초기화는 C FFI 호출과 메모리 할당을 수반함. `sync.Once`는 첫 사용까지 비용을 지연시키면서 스레드 안전한 단일 초기화를 보장함.

### Parse 헬퍼

`Parse` 함수는 보장된 정리와 함께 깔끔한 API를 제공함:

```go
func Parse(ctx context.Context, lang domain.Language, source []byte) (*sitter.Tree, error) {
    parser := Get(lang)
    defer parser.Close()

    tree, err := parser.ParseCtx(ctx, nil, source)
    if err != nil {
        return nil, fmt.Errorf("parse %s failed: %w", lang, err)
    }
    return tree, nil
}
```

### 성능 영향

| 작업             | 오버헤드      | 상태         |
| ---------------- | ------------- | ------------ |
| 파서 할당        | ~10µs/파싱    | 허용 가능    |
| 언어 문법 초기화 | ~1-5ms        | 한 번만 캐싱 |
| 쿼리 컴파일      | ~1-5ms        | 한 번만 캐싱 |
| 쿼리 실행        | ~0.1-1ms/파일 | 최적화됨     |

**순 영향**: 문법 및 쿼리 캐싱이 반복 작업에 10-50배 속도 향상을 제공함. 파싱당 ~10µs 오버헤드는 일반적인 파일 I/O 지연에 비해 무시할 수준임.

## Consequences

### Positive

1. **테스트 안정성**
   - 파서 상태 누수로 인한 간헐적 테스트 실패 없음
   - 결정적인 CI/CD 파이프라인 동작 보장

2. **코드 단순성**
   - 풀 관리 코드 유지보수 불필요
   - 명확한 소유권 의미론 (호출자가 생성하고 호출자가 닫음)

3. **디버깅 용이성**
   - 각 파싱 작업이 격리됨
   - 작업 간 교차 오염 없음

### Negative

1. **파싱당 오버헤드**
   - 파일당 ~10µs 할당 발생
   - **완화**: 코어 라이브러리 사용 사례에 허용 가능

2. **GC 압력 증가**
   - 더 많은 단기 할당 발생
   - **완화**: 문법 캐싱이 대부분의 할당을 장기로 유지함

### 향후 변경 제약

- 업스트림 tree-sitter 수정 없이는 **풀링 재활성화 불가**
- **성능 최적화 노력**은 파서 재사용이 아닌 쿼리 캐싱에 집중해야 함

## Related ADRs

- [ADR-03: Tree-sitter AST 파싱 엔진](./03-tree-sitter-ast-parsing-engine.md) - tree-sitter 선택 이유

## References

- [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) - 사용된 Go 바인딩
- [Tree-sitter 문서](https://tree-sitter.github.io/tree-sitter/) - 공식 문서
