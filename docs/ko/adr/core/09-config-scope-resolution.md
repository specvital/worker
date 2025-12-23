---
title: Config 스코프 해석
description: 모노레포 지원을 위한 계층적 설정 파일 해석에 관한 ADR
---

# ADR-09: Config 스코프 해석

> 🇺🇸 [English Version](/en/adr/core/09-config-scope-resolution.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2025-12-23 | @KubrickCode | core       |

**상태**: 승인됨

## 배경

### 문제 상황

현대 코드베이스는 여러 테스트 프레임워크 설정을 가진 모노레포 구조를 자주 사용함:

```
monorepo/
├── jest.config.js           # 루트 레벨 Jest 설정
├── packages/
│   ├── web/
│   │   └── vitest.config.ts # web 패키지용 Vitest
│   └── api/
│       └── jest.config.ts   # api 패키지용 Jest
└── e2e/
    └── playwright.config.ts # E2E 테스트용 Playwright
```

테스트 파일이 어떤 프레임워크에 속하는지 탐지할 때, 파서는 다음을 처리해야 함:

1. **중첩된 스코프 처리**: `packages/web/` 내 파일은 루트 Jest와 로컬 Vitest 설정 모두에 매칭될 수 있음
2. **계층 존중**: 중첩된 (더 구체적인) 설정이 상위 설정보다 우선해야 함
3. **결정론 보장**: 동일 파일은 실행마다 항상 동일 설정으로 해석되어야 함
4. **프레임워크별 설정 기능 지원**: Jest의 `roots`, Vitest의 `root`, include/exclude 패턴

### 전략적 질문

여러 설정이 적용될 수 있을 때 파서가 주어진 테스트 파일을 어떤 설정 파일이 관할하는지 어떻게 해석해야 하는가?

## 결정

**깊이 기반 해석과 결정론적 tie-breaking 사용: 더 깊은 (더 구체적인) 설정이 승리하며, 사전순 정렬을 최종 tie-breaker로 사용함.**

### 해석 알고리즘

```
경로 P의 테스트 파일에 대해:
1. 언어 호환성으로 설정 필터링
2. P를 포함하는 모든 설정 찾기
3. 깊이로 선택 (더 깊을수록 더 구체적)
4. Tie-breaker 1: 더 긴 설정 경로
5. Tie-breaker 2: 사전순 정렬 (결정론적)
```

### ConfigScope 구조

```go
type ConfigScope struct {
    ConfigPath      string              // 설정 파일 경로
    BaseDir         string              // 유효 루트 디렉토리
    Include         []string            // 포함 glob 패턴
    Exclude         []string            // 제외 glob 패턴
    Roots           []string            // 복수 루트 디렉토리 (Jest)
    Framework       string              // 프레임워크 이름
    GlobalsMode     bool                // 글로벌 사용 가능 여부
}
```

## 검토한 옵션

### 옵션 A: 깊이 기반 해석 (선택됨)

더 깊은 설정 파일이 더 얕은 설정보다 우선함.

**장점:**

- **직관적 동작**: 더 구체적인 설정이 자연스럽게 승리
- **모노레포 친화적**: 패키지 레벨 설정이 워크스페이스 루트를 오버라이드
- **결정론적**: tie-breaker가 있는 명확한 계층

**단점:**

- 설정 경로 구조가 우선순위에 영향
- 깊이 중첩된 설정은 명시적 의도와 무관하게 항상 승리

### 옵션 B: 설정 파일 내 명시적 우선순위

프레임워크 설정이 명시적 우선순위 값을 선언함.

**장점:**

- 해석 순서에 대한 완전한 제어
- 깊이 기반 기본값 오버라이드 가능

**단점:**

- **설정 수정 필요**: 사용자가 우선순위 필드를 추가해야 함
- **비표준**: 네이티브 프레임워크 설정의 일부가 아님
- **유지보수 부담**: 우선순위 값의 조율 필요

### 옵션 C: 선착순 해석

파일시스템 탐색 중 첫 번째 발견된 설정 사용.

**장점:**

- 단순한 구현
- 빠름 (첫 매칭에서 중단)

**단점:**

- **비결정론적**: 탐색 순서는 파일시스템에 따라 다름
- **예측 불가**: 결과가 발견 순서에 의존

## 구현 세부사항

### Contains 검사

`ConfigScope.Contains()` 메서드는 파일이 스코프 내에 있는지 결정함:

```go
func (s *ConfigScope) Contains(filePath string) bool {
    roots := s.effectiveRoots()
    for _, root := range roots {
        relPath := computeRelativePath(root, filePath)
        if isOutsideRoot(relPath) {
            continue
        }
        if !matchesIncludePatterns(relPath, s.Include) {
            continue
        }
        if matchesExcludePatterns(relPath, s.Exclude) {
            continue
        }
        return true
    }
    return false
}
```

### 깊이 계산

깊이는 BaseDir 경로 구조로부터 계산됨:

```go
func (s *ConfigScope) Depth() int {
    return strings.Count(filepath.ToSlash(s.BaseDir), "/")
}
```

| 설정 경로                           | BaseDir            | 깊이 |
| ----------------------------------- | ------------------ | ---- |
| `jest.config.js`                    | `.`                | 0    |
| `packages/web/vitest.config.ts`     | `packages/web`     | 1    |
| `packages/web/src/vitest.config.ts` | `packages/web/src` | 2    |

### 멀티 루트 지원

Jest의 `roots` 설정은 복수의 루트 디렉토리를 허용함:

```javascript
// jest.config.js
module.exports = {
  roots: ["<rootDir>/packages/next/src", "<rootDir>/packages/font/src"],
};
```

파서는 이를 설정 디렉토리 기준으로 해석하고 모든 루트에 대해 파일 포함 여부를 검사함.

### 결정론적 선택

동일 깊이의 복수 설정이 매칭될 때:

```go
// Tie-breaker 1: 더 긴 설정 경로 선호 (더 구체적)
if len(m.path) > len(best.path) {
    best = m
}
// Tie-breaker 2: 결정론을 위한 사전순 정렬
if m.path < best.path {
    best = m
}
```

이는 다음에 대해 일관된 동작을 보장함:

- 복수 CI 실행
- 다른 파일시스템 구현
- Map 순회 순서 변동

## 결과

### 긍정적

1. **모노레포 지원**
   - 패키지별 설정이 자연스럽게 우선권을 가짐
   - 모든 중첩 깊이에서 동작
   - 특별한 설정 필요 없음

2. **결정론적 결과**
   - 동일 파일은 항상 동일 설정에 매핑됨
   - CI 환경 간 일관성
   - 재현 가능한 탐지 결과

3. **프레임워크 호환성**
   - 네이티브 설정 의미론 존중 (Jest roots, Vitest root)
   - include/exclude 패턴 지원
   - 글로벌 모드 탐지 처리

4. **제로 설정**
   - 표준 프레임워크 설정 컨벤션으로 동작
   - 추가 메타데이터 불필요
   - 기존 프로젝트에 드롭인 지원

### 부정적

1. **암시적 우선순위**
   - 설정 계층이 경로 구조에 의해 결정됨
   - **완화**: 해석 순서 문서화; 깊이 기반은 직관적임

2. **오버라이드 메커니즘 없음**
   - 얕은 설정이 깊은 설정을 이기도록 강제 불가
   - **완화**: 명시적 오버라이드가 필요하면 설정 파일 재구조화

3. **성능 비용**
   - 선택 전 모든 매칭 설정을 검사해야 함
   - **완화**: 설정 수는 일반적으로 적음; 선형 탐색은 수용 가능

### 설계 원칙

- **근접성이 승리**: 테스트 파일에 더 가까운 설정이 더 관련성 있음
- **설정보다 컨벤션**: 표준 레이아웃은 추가 설정 없이 동작
- **예측 가능한 동작**: 동일 입력은 항상 동일 결과 생성
- **언어 인식**: 파일 언어와 호환되는 설정만 고려

## 관련 ADR

- [ADR-04: Early-Return 프레임워크 탐지](./04-early-return-framework-detection.md) - 탐지 계층에서 스코프 해석 사용
- [ADR-06: 통합 프레임워크 정의](./06-unified-framework-definition.md) - 스코프 생성을 위한 ConfigParser 인터페이스

## 참조

- `pkg/parser/framework/scope.go` - ConfigScope 구현
- `pkg/parser/detection/detector.go` - detectFromScope 해석 로직
- `pkg/parser/strategies/jest/definition.go` - roots 지원 Jest 설정 파싱
- `pkg/parser/strategies/vitest/definition.go` - root 지원 Vitest 설정 파싱
