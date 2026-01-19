---
title: React Compiler 도입
description: React Compiler 활성화 및 수동 메모이제이션 제거 ADR
---

# ADR-22: React Compiler 도입

> 🇺🇸 [English Version](/en/adr/web/22-react-compiler-adoption.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2026-01-19 | @KubrickCode | web        |

## 배경

### 수동 메모이제이션 부담

웹 애플리케이션 전반에 상당한 수동 메모이제이션 오버헤드 누적:

- **27개 파일**에 명시적 `useMemo` 및 `useCallback` 호출 존재
- 메모이제이션 필요 여부 판단에 대한 개발자 인지 부하
- 과도한 메모이제이션(불필요한 복잡성) 또는 메모이제이션 부족(성능 문제) 위험
- 메모이제이션 의존성이 있는 컴포넌트 리팩토링 시 유지보수 부담

### React Compiler 기회

[ADR-02: Next.js 16 + React 19 선택](/ko/adr/web/02-nextjs-react-selection.md)에서 채택한 React 19에 React Compiler(이전 React Forget) 포함. 컴파일러가 빌드 타임에 메모이제이션 최적화 자동 적용, 대부분의 경우 수동 `useMemo`, `useCallback`, `React.memo` 불필요.

**ADR-02 성능 전망:** "React Compiler 자동 메모이제이션 (리렌더 25-40% 감소)"

### 배포 후 발견된 문제

React Compiler 활성화 후 TanStack Virtual 사용 컴포넌트 2개에서 동작 이상 발생:

- `test-list.tsx` - 가상화된 테스트 결과 목록
- `tree-view.tsx` - 가상화된 파일 트리 네비게이션

**근본 원인:** React Compiler의 공격적 메모이제이션이 `virtualizer.getVirtualItems()` 결과 캐싱. 가상화 참조가 변경되지 않아 가상 아이템 미갱신, 빈 목록 또는 오래된 목록 표시.

React Compiler와 TanStack Virtual의 `measureElement` 및 `ResizeObserver` 사용 ref 기반 측정 패턴 간 알려진 호환성 문제.

## 결정

**호환되지 않는 서드파티 라이브러리 패턴용 `"use no memo"` 이스케이프 해치와 함께 React Compiler 전역 도입.**

### 핵심 원칙

1. **컴파일러 우선**: 모든 컴포넌트에 기본적으로 React Compiler 활성화
2. **명시적 제외**: 문서화된 비호환성에 대해서만 `"use no memo"` 지시어 사용
3. **수동 메모이제이션 제거**: 컴파일러 최적화 컴포넌트에서 `useMemo`/`useCallback` 삭제
4. **예외 문서화**: 제외 컴포넌트 목록 및 근거 유지

### 설정

```typescript
// next.config.ts
const nextConfig = {
  reactCompiler: true,
  // ...
};
```

### 의존성

```json
{
  "devDependencies": {
    "babel-plugin-react-compiler": "1.0.0"
  }
}
```

### 이스케이프 해치 사용법

```tsx
"use no memo";

export function VirtualizedList() {
  // React Compiler 최적화에서 제외
  const virtualizer = useVirtualizer({ ... });
  return ...;
}
```

## 검토한 옵션

### 옵션 A: 이스케이프 해치를 통한 전체 React Compiler 도입 (선택)

`reactCompiler: true` 전역 활성화, 수동 메모이제이션 제거, 비호환성에 `"use no memo"` 적용.

| 장점                            | 단점                         |
| ------------------------------- | ---------------------------- |
| 코드 단순화 (27개 파일 정리)    | 라이브러리 호환성 오버헤드   |
| 일관된 최적화 전략              | 디버깅 시 컴파일러 이해 필요 |
| 미래 지향적, React 방향과 일치  | 이스케이프 해치 남용 위험    |
| 개발자 메모이제이션 결정 불필요 |                              |

### 옵션 B: 수동 메모이제이션 유지

React Compiler 미활성화, 기존 패턴 유지.

| 장점             | 단점                     |
| ---------------- | ------------------------ |
| 호환성 위험 없음 | 지속적인 인지 부담       |
| 익숙한 패턴      | 일관성 없는 적용         |
|                  | React 생태계 방향과 배치 |

**기각**: ADR-02에서 컴파일러 이점을 위해 React 19 명시적 채택.

### 옵션 C: 선택적/점진적 도입

`"use memo"` 지시어로 옵트인 모드에서 React Compiler 활성화.

| 장점            | 단점                              |
| --------------- | --------------------------------- |
| 안전한 롤아웃   | 노력 배가 (수동 결정 여전히 필요) |
| 컴포넌트별 검증 | 부분적 이점, 일관성 없는 상태     |

**기각**: 이스케이프 해치 패턴이 더 적은 오버헤드로 안전한 롤아웃 달성.

## 결과

### 긍정적

| 영역        | 이점                                                |
| ----------- | --------------------------------------------------- |
| 코드 단순화 | 27개 파일에서 수동 메모이제이션 제거                |
| 성능 일관성 | 컴파일러가 정적 분석으로 최적 메모이제이션 적용     |
| 개발자 경험 | 언제 메모이제이션할지 판단하는 정신적 오버헤드 제거 |
| 생태계 정렬 | 향후 React 최적화를 위한 코드베이스 포지셔닝        |

### 부정적

| 영역                     | 트레이드오프                            | 완화 방안                    |
| ------------------------ | --------------------------------------- | ---------------------------- |
| 라이브러리 호환성        | TanStack Virtual에 `"use no memo"` 필요 | 컴포넌트 파일에 문서화       |
| 이스케이프 해치 거버넌스 | 지시어 남용 위험                        | 근거 필수 코드 리뷰 정책     |
| 디버깅 복잡성            | 컴파일러 변환 이해 필요                 | React DevTools Compiler 배지 |

### 영향받는 컴포넌트

| 컴포넌트        | 문제                      | 해결책          | 상태                      |
| --------------- | ------------------------- | --------------- | ------------------------- |
| `test-list.tsx` | TanStack Virtual ref 캐싱 | `"use no memo"` | TanStack 수정 시까지 임시 |
| `tree-view.tsx` | TanStack Virtual ref 캐싱 | `"use no memo"` | TanStack 수정 시까지 임시 |

### 코딩 가이드라인

`CLAUDE.md`에 추가:

```markdown
## React Compiler

React Compiler 활성화 (`next.config.ts`: `reactCompiler: true`)

### 금지

- `useMemo`, `useCallback`, `React.memo` **절대 사용 금지**
- React Compiler가 빌드 타임에 자동으로 메모이제이션 처리

### 이스케이프 해치

컴파일러가 문제를 일으키는 경우에만 `"use no memo"` 지시어 사용
```

## 참조

- [ADR-02: Next.js 16 + React 19 선택](/ko/adr/web/02-nextjs-react-selection.md)
- [커밋 482d080e](https://github.com/specvital/web/commit/482d080e) - React Compiler 활성화
- [커밋 21a7fb83](https://github.com/specvital/web/commit/21a7fb83) - 아코디언 중첩 수정
- [React Compiler "use no memo" 지시어](https://react.dev/reference/react-compiler/directives/use-no-memo)
- [TanStack Virtual Issue #736](https://github.com/TanStack/virtual/issues/736) - React Compiler 호환성
