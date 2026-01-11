---
title: TanStack Query 선택
description: 클라이언트 사이드 상태 관리를 위한 데이터 페칭 라이브러리로 TanStack Query v5를 선택한 ADR
---

# ADR-04: TanStack Query 선택

> [English Version](/en/adr/web/04-tanstack-query-selection.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-09 | @KubrickCode | web   |

## Context

### 데이터 페칭 과제

웹 플랫폼은 클라이언트 사이드 데이터 페칭이 필요한 상황:

1. **폴링 기반 상태 추적**: 분석 작업은 비동기로 실행(queued → analyzing → completed/failed). 프론트엔드는 완료까지 상태 업데이트를 폴링해야 함.
2. **커서 기반 페이지네이션**: 대시보드 목록은 Go 백엔드의 커서 기반 페이지네이션으로 무한 스크롤 사용.
3. **뮤테이션과 캐시 동기화**: 북마크, 재분석 등의 액션은 캐시된 데이터를 자동으로 업데이트해야 함.
4. **REST API 최적화**: 모든 데이터는 `openapi.yaml`에 정의된 REST 엔드포인트에서 제공; GraphQL 없음.

### 기존 아키텍처 제약

- **Next.js 16 + React 19**: Server Components가 있는 App Router; 데이터 페칭 훅은 Client Components에서 실행
- **BFF 패턴**: Next.js는 얇은 프레젠테이션 레이어; Go 백엔드가 모든 비즈니스 로직 처리
- **OpenAPI 타입 생성**: `openapi-typescript`를 통해 `openapi.yaml`에서 TypeScript 타입 생성
- **글로벌 상태 라이브러리 없음**: Redux, Zustand 등 글로벌 상태 관리 없음

### 평가 대상

1. **TanStack Query v5**: 폴링, 무한 쿼리, 뮤테이션을 갖춘 풍부한 기능의 데이터 페칭 라이브러리
2. **SWR**: Vercel의 경량 데이터 페칭 라이브러리
3. **RTK Query**: Redux Toolkit의 데이터 페칭 솔루션
4. **Apollo Client**: GraphQL 중심이지만 REST에도 적용 가능

## Decision

**폴링 기능, 무한 쿼리 지원, 성숙한 뮤테이션 처리를 위해 TanStack Query v5를 주요 데이터 페칭 라이브러리로 채택.**

핵심 원칙:

1. **Query Key Factory**: 기능 도메인별 중앙화된 쿼리 키 정의
2. **조건부 폴링**: 상태 의존적 폴링을 위해 `refetchInterval` 함수 사용
3. **캐시 무효화**: 자동 데이터 동기화를 위해 뮤테이션 후 `invalidateQueries` 사용
4. **타입 안전성**: 쿼리 함수에서 OpenAPI 생성 타입 활용

## Options Considered

### Option A: TanStack Query v5 (선택됨)

**작동 방식:**

- 커스텀 기본값(`staleTime`, 에러 핸들러)을 가진 `QueryClient`
- 자동 캐싱을 지원하는 데이터 페칭용 `useQuery`
- 커서 기반 페이지네이션용 `useInfiniteQuery`
- `onSuccess` 캐시 무효화를 지원하는 `useMutation`
- 조건부 폴링을 위한 함수 지원 `refetchInterval`

**장점:**

- **폴링 우수성**: `refetchInterval`이 백오프를 통한 조건부 폴링 함수 지원
- **무한 쿼리**: 커서 페이지네이션을 위한 `getNextPageParam`과 함께 네이티브 `useInfiniteQuery`
- **가비지 컬렉션**: 미사용 쿼리 자동 정리 (기본 5분)
- **DevTools**: 캐시 상태 디버깅을 위한 공식 DevTools 패키지
- **React 19 지원**: `useSyncExternalStore` 사용, 완전 호환
- **시장 지배력**: 60-70% 시장 점유율, 광범위한 문서, 커뮤니티 지원

**단점:**

- SWR보다 큰 번들 (~11-13 KB vs ~4.2 KB gzipped)
- 고급 패턴에 대한 학습 곡선
- SSR 프리페칭을 위한 HydrationBoundary 보일러플레이트

### Option B: SWR

**작동 방식:**

- stale-while-revalidate 전략의 데이터 페칭용 `useSWR`
- 페이지네이션용 `useSWRInfinite`
- 뮤테이션용 `useSWRMutation` (v2.0에 추가)

**평가:**

- **가비지 컬렉션 없음**: 미사용 쿼리 자동 정리 없음; 동적 쿼리에서 메모리 누수
- **약한 무한 쿼리**: `useSWRInfinite`가 TanStack의 `useInfiniteQuery`보다 직관적이지 않음
- **공식 DevTools 없음**: 커뮤니티 제작 대안만 존재
- **staleTime 동등 기능 없음**: 데이터가 신선한 것으로 간주되는 시점에 대한 제어 부족
- **기각**: 폴링 복잡성과 페이지네이션 요구사항에 불충분

### Option C: RTK Query

**작동 방식:**

- 엔드포인트가 있는 API 슬라이스 정의
- 생성된 훅 (`useGetXQuery`, `useLazyGetXQuery`)
- 태그 기반 캐시 무효화

**평가:**

- **Redux 의존성**: Redux Toolkit 도입 필요
- **무한 쿼리가 신규**: 2025년 2월 추가, 검증 부족
- **오버헤드**: 비-Redux 애플리케이션에 무거운 설정
- **제한된 Next.js App Router 문서**: App Router 패턴에 대한 문서 부족
- **기각**: 현재 아키텍처에 불필요한 Redux 도입

### Option D: Apollo Client

**작동 방식:**

- 정규화된 캐시를 가진 GraphQL 우선 설계
- REST API용 `apollo-link-rest` 어댑터
- `pollInterval` 옵션을 통한 폴링

**평가:**

- **REST는 2등 시민**: `apollo-link-rest` 어댑터 필요
- **번들 크기**: ~30 KB gzipped, TanStack Query의 3배
- **정규화된 캐시 오버헤드**: REST API에 불필요한 복잡성
- **GraphQL 개념**: fragments, links, resolvers는 GraphQL 전용
- **기각**: REST 전용 애플리케이션에 상당한 오버헤드

## Implementation Details

### QueryClient 설정

```typescript
// lib/query/client.ts
export const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: false,
        retry: false,
        staleTime: 1000 * 60, // 1분
      },
    },
    mutationCache: new MutationCache({
      onError: (error, _variables, _context, mutation) => {
        if (isUnauthorizedError(error) && isAuthQuery(mutation.options.mutationKey)) {
          handleUnauthorizedError(queryClient);
        }
      },
    }),
    queryCache: new QueryCache({
      onError: (error, query) => {
        if (isUnauthorizedError(error) && isAuthQuery(query.queryKey)) {
          handleUnauthorizedError(queryClient);
        }
      },
    }),
  });
```

### 지수 백오프를 통한 폴링

```typescript
// features/analysis/hooks/use-analysis.ts
const INITIAL_INTERVAL_MS = 1000;
const MAX_INTERVAL_MS = 5000;
const BACKOFF_MULTIPLIER = 1.5;

const query = useQuery({
  queryKey: analysisKeys.detail(owner, repo),
  queryFn: () => fetchAnalysis(owner, repo),
  refetchInterval: (query) => {
    const response = query.state.data;
    if (response && isTerminalStatus(response)) {
      return false; // 폴링 중지
    }
    const interval = intervalRef.current;
    intervalRef.current = Math.min(interval * BACKOFF_MULTIPLIER, MAX_INTERVAL_MS);
    return interval;
  },
});
```

### 커서 기반 무한 쿼리

```typescript
// features/dashboard/hooks/use-paginated-repositories.ts
export const usePaginatedRepositories = (options: PaginatedRepositoriesOptions) => {
  const query = useInfiniteQuery({
    queryKey: paginatedRepositoriesKeys.list({ limit, sortBy, sortOrder, view }),
    queryFn: ({ pageParam }) =>
      fetchPaginatedRepositories({
        cursor: pageParam,
        limit,
        sortBy,
        sortOrder,
        view,
      }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => (lastPage.hasNext ? lastPage.nextCursor : undefined),
    staleTime: 30 * 1000,
  });

  const data = query.data?.pages.flatMap((page) => page.data) ?? [];
  return { data, hasNextPage: query.hasNextPage, fetchNextPage: query.fetchNextPage };
};
```

### 캐시 무효화를 통한 뮤테이션

```typescript
// features/dashboard/hooks/use-bookmark-mutation.ts
export const useAddBookmark = () => {
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: ({ owner, repo }) => addBookmark(owner, repo),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: paginatedRepositoriesKeys.all });
      toast.success("Bookmark added");
    },
    onError: (error) => toast.error("Failed to add bookmark", { description: error.message }),
  });

  return { addBookmark: mutation.mutate, isPending: mutation.isPending };
};
```

### Query Key Factory 패턴

```typescript
// 기능별 중앙화된 쿼리 키 정의
export const analysisKeys = {
  all: ["analysis"] as const,
  detail: (owner: string, repo: string) => [...analysisKeys.all, owner, repo] as const,
};

export const paginatedRepositoriesKeys = {
  all: ["paginatedRepositories"] as const,
  list: (options: PaginatedRepositoriesOptions) =>
    [...paginatedRepositoriesKeys.all, "list", options] as const,
};
```

## Consequences

### Positive

**폴링 유연성:**

- 함수 기반 `refetchInterval`을 통한 조건부 폴링
- 지수 백오프로 서버 과부하 방지
- 폴링 중지 시 자동 정리

**페이지네이션 UX:**

- 커서 처리가 포함된 네이티브 무한 쿼리 지원
- 부드러운 전환을 위한 지연된 쿼리 데이터
- 자동 로드를 위한 Intersection Observer 통합

**개발자 경험:**

- 쿼리 키 팩토리로 정밀한 캐시 무효화 가능
- 개발 중 캐시 상태 디버깅을 위한 DevTools
- OpenAPI 생성 타입과 타입 안전한 통합

**메모리 관리:**

- 미사용 쿼리 자동 가비지 컬렉션
- 설정 가능한 `gcTime`(구 `cacheTime`)으로 메모리 누수 방지
- 동적 쿼리에 수동 정리 불필요

### Negative

**번들 크기:**

- SWR의 ~4.2 KB 대비 ~11-13 KB gzipped
- **완화**: 대시보드 애플리케이션에 수용 가능; DevTools는 개발 전용

**SSR 복잡성:**

- Client Component에 `QueryClientProvider` 래퍼 필요
- SSR 프리페칭에 `HydrationBoundary` 필요
- **완화**: BFF 패턴이 SSR 데이터 요구사항 최소화

**학습 곡선:**

- 고급 패턴(staleTime, gcTime, structural sharing)은 학습 필요
- **완화**: 코드베이스에 확립된 패턴; 내부 문서화

### 확립된 사용 패턴

| 패턴        | 구현                                    | 파일                            |
| ----------- | --------------------------------------- | ------------------------------- |
| 폴링        | 함수와 함께 `refetchInterval`           | `use-analysis.ts`               |
| 무한 쿼리   | `useInfiniteQuery` + `getNextPageParam` | `use-paginated-repositories.ts` |
| 뮤테이션    | `useMutation` + `invalidateQueries`     | `use-bookmark-mutation.ts`      |
| 데이터 페칭 | `useQuery` + query key factory          | `use-my-repositories.ts`        |

## References

### 내부

- [ADR-02: Next.js 16 + React 19 선택](/ko/adr/web/02-nextjs-react-selection.md)

### 외부

- [TanStack Query 공식 문서](https://tanstack.com/query/latest)
- [TanStack Query 비교 페이지](https://tanstack.com/query/latest/docs/framework/react/comparison)
- [무한 쿼리 가이드](https://tanstack.com/query/v5/docs/framework/react/guides/infinite-queries)
- [React Query vs SWR 비교](https://tanstack.com/query/latest/docs/framework/react/comparison)
