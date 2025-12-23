---
title: Tree-sitter AST 파싱 엔진
description: 다국어 테스트 파일 파싱을 위한 tree-sitter 선택 결정
---

# ADR-03: Tree-sitter AST 파싱 엔진

> 🇺🇸 [English Version](/en/adr/core/03-tree-sitter-ast-parsing-engine.md)

| 날짜       | 작성자       | 영향 리포지토리 |
| ---------- | ------------ | --------------- |
| 2025-12-03 | @KubrickCode | core            |

**상태**: 승인됨

## Context

### 문제 정의

SpecVital Core는 다음 조건에서 테스트 파일을 파싱해야 함:

- **다양한 테스트 프레임워크** (Jest, Vitest, Playwright, JUnit, pytest, RSpec 등)
- **다중 프로그래밍 언어** (JavaScript, TypeScript, Python, Go, Java, C#, Ruby 등)
- **실제 환경 제약**: 불완전한 코드, 문법 오류, 동시 파싱, 운영 안정성

### 요구사항

1. **다국어 지원**: 모든 대상 언어에 대한 단일 통합 접근법 필요
2. **에러 복구**: 불완전하거나 문법적으로 잘못된 코드도 정상적으로 파싱해야 함
3. **정확성**: 정밀한 테스트 탐지를 위한 완전한 AST 접근 필요 (정규식 수준의 근사가 아닌)
4. **성능**: 수천 개의 테스트 파일이 있는 대규모 저장소에서 효율적이어야 함
5. **유지보수성**: 프레임워크별 커스텀 파서 코드 최소화 필요

### 전략적 질문

다국어 테스트 파일 분석을 위해 정확성, 유지보수성, 성능 간 최적의 트레이드오프를 제공하는 파싱 접근법은 무엇인가?

## Decision

**`smacker/go-tree-sitter` 바인딩을 통해 tree-sitter를 AST 파싱 엔진으로 사용함.**

Tree-sitter가 제공하는 것:

- 40+ 지원 언어에 대한 통합 C API
- 견고한 에러 복구를 갖춘 GLR 기반 증분 파서
- 활발한 커뮤니티에서 관리하는 언어 문법
- VSCode, Neovim, GitHub Semantic에서 검증된 안정성

## Options Considered

### Option A: Tree-sitter (선택됨)

언어별 문법을 갖춘 증분 파서 생성기임.

**장점:**

- **단일 통합 API**: 모든 지원 언어에서 동일한 `Node`, `Tree`, `Query` 구조 사용
- **에러 복구**: 불완전한 코드도 파싱하여 사용 가능한 AST 반환
- **커뮤니티 문법**: 40+ 언어가 활발히 관리됨
- **프로덕션 채택**: VSCode, Neovim, Zed, GitHub Semantic에서 사용됨
- **성능**: O(n) 시간 복잡도로, 실시간 에디터 사용에도 충분히 빠름

**단점:**

- **C 의존성**: Go 바인딩에 CGO 필요
- **문법 품질 차이**: 커뮤니티 관리 문법의 품질 편차 존재
- **파서 풀링 문제**: 취소 플래그 버그로 파서 재사용 불가 (ADR-05 참조)

### Option B: ANTLR4

확장 BNF 문법을 갖춘 ALL(\*) 파서 생성기임.

**장점:**

- 광범위한 문법 저장소를 갖춘 성숙한 생태계 보유
- 내장 코드 완성 엔진 제공
- 컴파일러 도구 체인에서 검증됨

**단점:**

- **성능**: 벤치마크에서 수작업 파서 대비 40배 느림
- **증분 파싱 없음**: 변경 시 전체 파일 재파싱 필요
- **에러 복구**: 불완전한 코드에서 tree-sitter보다 덜 견고함
- **Go 런타임 오버헤드**: Go 타겟에서 성능 저하

### Option C: 정규식 매칭

원시 텍스트에 대한 패턴 매칭 방식임.

**장점:**

- 구현이 간단하고 외부 의존성 없음
- 기본 패턴에 매우 빠름
- 어떤 입력에서도 동작함 (문법 요구사항 없음)

**단점:**

- **오탐**: 코드와 주석, 문자열 구분 불가
- **구조 이해 없음**: 중첩 구조에서 실패함 (describe/it 블록)
- **유지보수 악몽**: 각 프레임워크마다 언어별 커스텀 패턴 필요
- **취약함**: 코드 스타일 변화에 쉽게 깨짐

### Option D: 언어별 커스텀 파서

수작업 재귀 하강 파서 방식임.

**장점:**

- 최대 성능 달성 가능 (ANTLR 대비 40배 빠름 가능)
- 에러 처리와 복구에 대한 완전한 제어 가능
- 깊은 의미 분석 통합 가능

**단점:**

- **개발 비용**: 다중 언어 × 다중 프레임워크 = 감당 불가 범위
- **전문성 필요**: 각 대상에 대한 언어별 파싱 지식 필요
- **유지보수 부담**: 언어 변경 시 수동 업데이트 필요
- **출시 시간**: 기능 동등성 달성에 수개월/수년 소요

## Implementation Details

### 아키텍처

```
pkg/parser/
├── tspool/              # Tree-sitter 파서 생명주기 관리
│   └── pool.go          # 파서 생성, 언어 문법 캐싱
├── treesitter.go        # 고수준 유틸리티 (GetNodeText, WalkTree)
├── parser_pool.go       # 쿼리 컴파일 캐싱
└── strategies/
    ├── jest/            # Jest 프레임워크 파서 (tree-sitter 쿼리)
    ├── vitest/          # Vitest 프레임워크 파서
    ├── playwright/      # Playwright 프레임워크 파서
    └── shared/
        ├── jstest/      # 공유 JS/TS 파싱 유틸리티
        ├── javaast/     # 공유 Java 파싱 유틸리티
        └── dotnetast/   # 공유 C# 파싱 유틸리티
```

### 언어 문법 초기화

언어 문법은 `sync.Once`를 통해 한 번만 초기화됨:

```go
var (
    goLang    *sitter.Language
    jsLang    *sitter.Language
    // ... 모든 지원 언어
    langOnce  sync.Once
)

func initLanguages() {
    langOnce.Do(func() {
        goLang = golang.GetLanguage()
        jsLang = javascript.GetLanguage()
        // ...
    })
}
```

**이유**: 문법 초기화는 비용이 높음 (C FFI 호출, 메모리 할당). `sync.Once`는 첫 사용까지 비용을 지연시키면서 스레드 안전한 단일 초기화를 보장함.

### 파서 생명주기

사용마다 새 파서를 생성함 (풀링 결정은 ADR-05 참조):

```go
func Parse(ctx context.Context, lang domain.Language, source []byte) (*sitter.Tree, error) {
    parser := Get(lang)        // 새 파서
    defer parser.Close()       // 보장된 정리

    tree, err := parser.ParseCtx(ctx, nil, source)
    if err != nil {
        return nil, fmt.Errorf("parse %s failed: %w", lang, err)
    }
    return tree, nil
}
```

### 쿼리 캐싱

Tree-sitter 쿼리는 성능을 위해 캐싱됨:

```go
var queryCache sync.Map  // 컴파일된 쿼리용 동시성 맵

func QueryWithCache(root *sitter.Node, source []byte, lang domain.Language, queryStr string) ([]QueryResult, error) {
    query, err := getCachedQuery(lang, queryStr)  // 한 번만 컴파일
    if err != nil {
        return nil, err
    }
    cursor := sitter.NewQueryCursor()
    defer cursor.Close()
    cursor.Exec(query, root)
    // ...
}
```

**효과**: 쿼리 컴파일은 ~1-5ms (일회성), 쿼리 실행은 ~0.1-1ms (파일당). 많은 파일을 가진 프레임워크에서 10-50배 속도 향상.

## Consequences

### Positive

1. **통합 다국어 지원**
   - 모든 지원 언어에 단일 API 사용
   - 유사 프레임워크 간 공유 유틸리티 활용 (`jstest`로 Jest/Vitest/Mocha)
   - 새 파싱 인프라 없이 새 프레임워크 추가 가능

2. **견고한 에러 처리**
   - 불완전한 테스트 파일도 충돌 없이 파싱
   - C 바인딩 엣지 케이스에 대한 방어적 프로그래밍 적용
   - AST 추출 실패 시 정상적으로 저하됨

3. **프로덕션급 성능**
   - `sync.Once`를 통해 언어 문법 캐싱
   - `sync.Map`을 통해 쿼리 컴파일 캐싱
   - 워커 풀로 병렬 파싱 (기본값: GOMAXPROCS)

4. **커뮤니티 활용**
   - 문법 개선이 모든 사용자에게 이익
   - 활발한 생태계가 40+ 언어 관리
   - 프로덕션에서 검증됨 (GitHub, VSCode, Neovim)

### Negative

1. **CGO 의존성**
   - 크로스 컴파일이 복잡해짐
   - 빌드 시간 오버헤드 발생
   - **완화**: 코어 라이브러리에는 허용 가능. 순수 Go 대안이 폴백으로 존재함.

2. **파서 풀링 비활성화**
   - 파일당 새 파서 할당 필요 (~10µs 오버헤드)
   - **완화**: 문법 캐싱이 주요 최적화를 유지함. ADR-05 참조.

3. **제한된 의미 분석**
   - Tree-sitter는 구조만 제공하고 완전한 의미론은 제공하지 않음
   - 타입 해석이나 심볼 테이블 없음
   - **완화**: 테스트 탐지는 구조만 필요. 의미 분석은 범위 외임.

4. **문법 유지보수 위험**
   - 커뮤니티 문법 품질에 의존함
   - **완화**: 활발한 관리자가 있는 인기 문법 사용 (JS, Python, Java).

### 트레이드오프 요약

| 측면          | Tree-sitter       | 대안               |
| ------------- | ----------------- | ------------------ |
| 다국어 지원   | 우수 (40+)        | 부족 (언어별)      |
| 에러 복구     | 우수              | 가변적             |
| 개발 속도     | 빠름 (문법 활용)  | 느림 (커스텀 제작) |
| 유지보수 비용 | 낮음 (커뮤니티)   | 높음 (내부)        |
| 성능          | 양호 (O(n), 캐시) | 가변적             |

## Related ADRs

- [ADR-05: 파서 풀링 비활성화](./05-parser-pooling-disabled.md) - tree-sitter 취소 플래그 버그 상세

## References

- [Tree-sitter 문서](https://tree-sitter.github.io/tree-sitter/)
- [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) - 사용된 Go 바인딩
- [Why Tree-sitter - GitHub Semantic](https://github.com/github/semantic/blob/main/docs/why-tree-sitter.md)
- [Tree-sitter 성능 분석 - Symflower](https://symflower.com/en/company/blog/2023/parsing-code-with-tree-sitter/)
