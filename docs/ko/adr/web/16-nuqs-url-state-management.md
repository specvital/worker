---
title: nuqs URL 상태 관리
description: React/Next.js에서 타입 세이프 URL 쿼리 파라미터 상태 관리를 위한 nuqs 선택 ADR
---

# ADR-16: nuqs URL 상태 관리

> [English Version](/en/adr/web/16-nuqs-url-state-management.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2024-12-25 | @KubrickCode | web        |

## 배경

### URL 상태 동기화 과제

대시보드는 UI와 URL 간의 상태 동기화 필요:

1. **공유 가능한 필터 상태**: 대시보드 링크를 공유한 사용자가 동일한 필터 설정을 볼 수 있어야 함
2. **브라우저 히스토리 지원**: 뒤로/앞으로 네비게이션 시 이전 필터 상태 복원
3. **북마크 가능한 뷰**: 특정 필터 조합이 북마크 가능해야 함
4. **타입 안전성**: 쿼리 파라미터는 문자열이지만 타입 세이프 파싱 필요

### 구체적 사용 사례

| 기능             | URL 파라미터              | 타입          |
| ---------------- | ------------------------- | ------------- |
| 대시보드 뷰 필터 | `?view=starred`           | 리터럴 유니온 |
| 테스트 검색      | `?q=auth`                 | 문자열        |
| 상태 필터        | `?statuses=skipped,todo`  | 문자열 배열   |
| 프레임워크 필터  | `?frameworks=vitest,jest` | 문자열 배열   |

### 기존 아키텍처 제약

- **Next.js 16 App Router**: Server Component와 리프 노드에서의 클라이언트 측 상호작용
- **React 19**: 데이터 스트리밍을 위한 `use()`를 포함한 모던 hooks API
- **TanStack Query**: 이미 서버 상태 처리 중; URL 상태는 별도 관심사
- **TypeScript**: 코드베이스 전반에 걸친 강타입 필요

### 평가 후보

1. **nuqs**: React용 타입 세이프 URL 쿼리 상태 매니저
2. **수동 URLSearchParams**: 커스텀 훅과 네이티브 브라우저 API
3. **query-string**: 저수준 파싱 유틸리티
4. **use-query-params**: 쿼리 파라미터용 구버전 React 라이브러리

## 결정

**타입 세이프 파서, useState와 유사한 API, 네이티브 Next.js App Router 지원으로 인해 URL 쿼리 상태 관리에 nuqs 채택.**

핵심 원칙:

1. **루트에 NuqsAdapter**: App Router 통합을 위해 애플리케이션을 `NuqsAdapter`로 래핑
2. **타입 세이프 파서**: 타입 보장을 위해 `parseAsString`, `parseAsStringLiteral`, `parseAsArrayOf` 사용
3. **기본값**: null 상태 방지를 위해 항상 `.withDefault()` 제공
4. **코로케이션**: `useQueryState` 훅을 기능별 커스텀 훅에 배치

## 고려한 옵션

### Option A: nuqs (선택됨)

**동작 방식:**

- `useQueryState` 훅이 `useState` API를 미러링하지만 URL에 저장
- 내장 파서가 직렬화/역직렬화 처리
- `NuqsAdapter`가 Next.js App Router와 통합
- 배치 업데이트로 History API 과부하 방지

**장점:**

- **useState와 유사한 API**: React 개발자에게 최소한의 학습 곡선
- **타입 안전성**: `parseAsStringLiteral`이 컴파일 타임에 리터럴 유니온 타입 강제
- **App Router 네이티브**: `NuqsAdapter`로 Next.js 13+ 일급 지원
- **경량**: 외부 의존성 없이 ~5.5 KB gzipped
- **히스토리 통합**: 자동 브라우저 뒤로/앞으로 지원
- **스로틀 업데이트**: 빠른 상태 변경으로 인한 History API 크래시 방지

**단점:**

- 추가 의존성 (~5.5 KB)
- 레이아웃 레벨에서 `NuqsAdapter` 래퍼 필요
- 파서 조합에 대한 학습 곡선

### Option B: 수동 URLSearchParams

**동작 방식:**

- `next/navigation`의 `useSearchParams()` 사용
- 각 쿼리 파라미터에 대한 커스텀 훅 생성
- 수동 직렬화/역직렬화 로직

**평가:**

- **보일러플레이트 과다**: 각 파라미터에 수동 파싱 로직 필요
- **타입 안전성 없음**: 컴파일 타임 검증 없는 문자열 파싱
- **히스토리 엣지 케이스**: 브라우저 네비게이션 수동 처리
- **기각**: 과도한 보일러플레이트; 오류 발생 가능한 타입 강제

### Option C: query-string

**동작 방식:**

- 쿼리 문자열 파싱/스트링화를 위한 저수준 유틸리티
- React 통합 없음; 래퍼 훅 필요

**평가:**

- **React 훅 없음**: 커스텀 훅 레이어 구축 필요
- **타입 안전성 없음**: `string | string[] | null` 반환
- **SSR 미인식**: Server Component 고려 없음
- **기각**: 너무 저수준; 상당한 래퍼 코드 필요

### Option D: use-query-params

**동작 방식:**

- nuqs와 유사한 목표를 가진 구버전 React 라이브러리
- 쿼리 상태에 React Context 사용

**평가:**

- **구버전**: 마지막 주요 업데이트가 Next.js App Router 이전
- **RSC 불확실성**: Server Components 호환성 미확인
- **더 큰 번들**: nuqs보다 더 많은 의존성
- **기각**: nuqs가 더 나은 App Router 지원을 가진 현대적 후속작

## 구현 세부사항

### 루트 레이아웃 설정

```tsx
// app/[locale]/layout.tsx
import { NuqsAdapter } from "nuqs/adapters/next/app";

const LocaleLayout = ({ children }) => (
  <NuqsAdapter>
    <QueryProvider>{children}</QueryProvider>
  </NuqsAdapter>
);
```

### 문자열 리터럴 파서 (유니온 타입)

```typescript
// features/dashboard/hooks/use-view-filter.ts
import { parseAsStringLiteral, useQueryState } from "nuqs";

export type ViewFilter = "all" | "mine" | "starred" | "community";

const VIEW_FILTER_OPTIONS: ViewFilter[] = ["all", "mine", "starred", "community"];
const viewFilterParser = parseAsStringLiteral(VIEW_FILTER_OPTIONS).withDefault("all");

export const useViewFilter = () => {
  const [viewFilter, setViewFilter] = useQueryState("view", viewFilterParser);
  return { setViewFilter, viewFilter } as const;
};
```

### 배열 파서 (다중 선택 필터)

```typescript
// features/analysis/hooks/use-filter-state.ts
import { parseAsArrayOf, parseAsString, useQueryState } from "nuqs";

const arrayParser = parseAsArrayOf(parseAsString, ",").withDefault([]);

export const useFilterState = () => {
  const [frameworks, setFrameworks] = useQueryState("frameworks", arrayParser);
  const [statuses, setStatuses] = useQueryState("statuses", arrayParser);

  return { frameworks, setFrameworks, statuses, setStatuses } as const;
};
```

### 문자열 파서 (검색 쿼리)

```typescript
// features/analysis/hooks/use-filter-state.ts
const queryParser = parseAsString.withDefault("");

export const useFilterState = () => {
  const [query, setQuery] = useQueryState("q", queryParser);
  return { query, setQuery } as const;
};
```

## 결과

### 긍정적

**공유 가능한 상태:**

- `/dashboard?view=starred&q=auth` 같은 필터 URL 공유 가능
- 수신자가 정확히 동일한 필터 구성 확인
- 지원 디버깅 활성화 ("현재 대시보드 URL 보내주세요")

**브라우저 히스토리 통합:**

- 뒤로 버튼이 이전 필터 상태 복원
- 앞으로 버튼이 필터 재적용
- 커스텀 히스토리 관리 없이 자연스러운 브라우저 UX

**타입 안전성:**

- `parseAsStringLiteral`이 컴파일 타임에 유효하지 않은 값 방지
- `withDefault()`가 소비 컴포넌트에서 null 체크 제거
- 필터 옵션에 대한 IntelliSense 지원

**개발자 경험:**

- `useState`와 동일한 API: `const [value, setValue] = useQueryState(...)`
- 직렬화/역직렬화 보일러플레이트 없음
- 복잡한 타입을 위한 조합 가능한 파서

### 부정적

**번들 사이즈:**

- 클라이언트 번들에 ~5.5 KB gzipped 추가
- **완화책**: 대시보드 애플리케이션에 수용 가능; 상당한 UX 개선 활성화

**어댑터 요구사항:**

- 루트 레이아웃에서 앱을 `NuqsAdapter`로 래핑해야 함
- **완화책**: 일회성 설정; 코드베이스에 이미 완료

**학습 곡선:**

- 팀이 파서 조합을 이해해야 함
- **완화책**: 기능 훅에 수립된 패턴; 코드베이스 전반에 일관된 사용

### 수립된 사용 패턴

| 패턴          | 파서                                 | 예시 URL                 |
| ------------- | ------------------------------------ | ------------------------ |
| 리터럴 유니온 | `parseAsStringLiteral`               | `?view=starred`          |
| 검색 문자열   | `parseAsString.withDefault("")`      | `?q=auth`                |
| 다중 선택     | `parseAsArrayOf(parseAsString, ",")` | `?statuses=skipped,todo` |

## 참고자료

### 내부

- [ADR-02: Next.js 16 + React 19 Selection](/ko/adr/web/02-nextjs-react-selection.md) - 프레임워크 맥락
- [ADR-04: TanStack Query Selection](/ko/adr/web/04-tanstack-query-selection.md) - 보완적 서버 상태

### 외부

- [nuqs 공식 문서](https://nuqs.dev)
- [nuqs GitHub 리포지토리](https://github.com/47ng/nuqs)
- [Next.js에서 nuqs로 검색 파라미터 관리](https://blog.logrocket.com/managing-search-parameters-next-js-nuqs/)
