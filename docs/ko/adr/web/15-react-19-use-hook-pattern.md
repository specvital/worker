---
title: React 19 use() Hook 패턴
description: Server Component에서 Client Component로 Promise 스트리밍을 위한 React 19 use() hook 채택 ADR
---

# ADR-15: React 19 use() Hook 패턴

> [English Version](/en/adr/web/15-react-19-use-hook-pattern.md)

| 날짜       | 작성자       | 리포지토리 |
| ---------- | ------------ | ---------- |
| 2024-12-07 | @KubrickCode | web        |

## 배경

### 페이지 전환 지연 문제

홈 페이지에서 분석 버튼을 클릭할 때 눈에 띄는 지연 발생. 이 문제는 데이터 페칭이 Server Component 렌더링을 차단하는 방식에서 비롯.

**관찰된 동작:**

1. 사용자가 홈 페이지에서 "분석" 버튼 클릭
2. `fetchAnalysis()` 완료를 기다리는 동안 네비게이션 멈춤
3. 대기 중 시각적 피드백 없음 (버튼이 멈춘 것처럼 보임)
4. 데이터 도착 후에야 페이지 렌더링

**근본 원인:**

기존 구현에서는 Server Component가 `await`를 직접 사용했다:

```typescript
// page.tsx (Server Component)
const AnalyzePage = async ({ params }) => {
  const result = await fetchAnalysis(owner, repo); // 렌더링 차단
  return <AnalysisContent result={result} />;
};
```

이 패턴은 Promise가 resolve될 때까지 Server Component 렌더링을 차단하여, 느린 네트워크나 Go 백엔드 콜드 스타트 시 체감 가능한 지연 유발.

### React 19의 새로운 데이터 페칭 프리미티브

React 19는 이 문제를 해결하기 위해 `use()` API 도입. 기존 hook과 달리:

- 루프와 조건문 안에서 호출 가능
- Promise와 Context 모두 지원
- Suspense boundary와 자연스럽게 통합
- Server에서 Client Component로 Promise 스트리밍 가능

## 결정

**즉각적인 네비게이션 피드백이 중요한 데이터 페칭 시나리오에 React 19 `use()` hook 패턴 채택, 네비게이션 상태를 위해 `useTransition`과 결합.**

핵심 원칙:

1. **Promise 스트리밍**: Server Component에서 Client Component로 Promise를 props로 전달
2. **Suspense 통합**: 로딩 상태를 위해 Client Component를 `<Suspense>`로 래핑
3. **전환 피드백**: 네비게이션 중 즉각적인 로딩 인디케이터를 위해 `useTransition` 사용
4. **API 프록시**: 환경에 무관한 API 호출을 위해 Next.js rewrites 설정

## 고려한 옵션

### Option A: Server Component await (기존 방식)

**동작 방식:**

- Server Component가 직접 `await fetchData()` 호출
- 데이터 도착까지 렌더링 차단
- resolve된 데이터를 자식에게 전달

**평가:**

- **장점**: 단순한 멘탈 모델, Suspense boundary 불필요
- **단점**: fetch 중 페이지가 멈춘 것처럼 보임, 점진적 렌더링 불가
- **기각**: 네트워크 바운드 작업에 대한 열악한 UX

### Option B: React 19 use() Hook (선택됨)

**동작 방식:**

- Server Component가 await 없이 Promise 생성
- Promise를 props로 Client Component에 전달
- Client Component가 `use(promise)`로 데이터 소비
- Suspense boundary가 로딩 중 fallback 표시

**장점:**

- **비차단 렌더링**: Server Component가 즉시 렌더링, Promise 스트리밍
- **점진적 로딩**: 페이지 셸이 즉시 렌더링, 데이터는 스트리밍
- **Suspense 통합**: 네이티브 로딩 상태 처리
- **안정적 Promise**: Server Component의 Promise는 재렌더링 시에도 안정적

**단점:**

- Suspense boundary에 대한 이해 필요
- Error Boundary로 에러 상태 처리 필요
- 단순한 케이스에 복잡성 추가

### Option C: 클라이언트 측 페칭만 사용

**동작 방식:**

- Server Component가 데이터 없이 즉시 렌더링
- Client Component가 `useEffect`에서 데이터 페칭
- 대기 중 로딩 스피너 표시

**평가:**

- **장점**: 단순하고 익숙한 패턴
- **단점**: 서버 사이드 렌더링 이점 없음, 워터폴, 추가 왕복
- **기각**: Server Component 기능 낭비

## 구현 세부사항

### Server Component (Promise 생성)

```typescript
// page.tsx (Server Component)
const AnalyzePage = async ({ params }) => {
  const { owner, repo } = await params;

  // await 없이 Promise 생성
  const dataPromise = fetchAnalysis(owner, repo);

  return (
    <Suspense fallback={<Loading />}>
      <AnalysisContent dataPromise={dataPromise} />
    </Suspense>
  );
};
```

### Client Component (Promise 소비)

```typescript
// analysis-content.tsx (Client Component)
"use client";

import { use } from "react";

type AnalysisContentProps = {
  dataPromise: Promise<AnalysisResult>;
};

export const AnalysisContent = ({ dataPromise }: AnalysisContentProps) => {
  const result = use(dataPromise);
  return <div>{/* 결과 렌더링 */}</div>;
};
```

### 전환 피드백과 네비게이션

```typescript
// url-input-form.tsx (Client Component)
"use client";

import { useTransition } from "react";

export const UrlInputForm = () => {
  const [isPending, startTransition] = useTransition();

  const handleSubmit = (e) => {
    e.preventDefault();
    // 즉각적인 피드백을 위해 네비게이션을 transition으로 래핑
    startTransition(() => {
      router.push(`/analyze/${owner}/${repo}`);
    });
  };

  return (
    <Button disabled={isPending}>
      {isPending ? <Loader2 className="animate-spin" /> : "분석"}
    </Button>
  );
};
```

### API 프록시 설정

```typescript
// next.config.ts
const nextConfig = {
  rewrites: async () => [
    {
      source: "/api/:path*",
      destination: `${API_URL}/api/:path*`,
    },
  ],
};
```

환경에 무관한 API 호출 가능 (클라이언트 측은 상대 경로, 서버 측은 전체 URL 사용).

## 결과

### 긍정적

**즉각적인 네비게이션 피드백:**

- `useTransition`을 통해 버튼이 즉시 로딩 상태 표시
- 페이지 셸이 즉시 렌더링
- 데이터가 점진적으로 스트리밍

**비차단 서버 렌더링:**

- Server Component가 fetch 완료를 기다리지 않음
- Promise가 직렬화되어 클라이언트로 스트리밍
- 더 나은 TTFB (Time to First Byte)

**네이티브 Suspense 통합:**

- 선언적으로 로딩 상태 처리
- Error boundary가 fetch 실패 캐치
- 애플리케이션 전반에 걸쳐 일관된 로딩 UI

### 부정적

**추가된 복잡성:**

- Promise 스트리밍에 대한 이해 필요
- Suspense boundary를 올바르게 배치해야 함
- **완화책**: 코드베이스에 명확한 패턴 수립

**모든 경우에 적합하지 않음:**

- 폴링과 캐시 무효화는 TanStack Query 필요
- 복잡한 상태 관리는 다른 패턴 필요
- **완화책**: 각 패턴 사용 시점 문서화

**에러 처리:**

- reject된 Promise는 가장 가까운 Error Boundary로 throw
- 각 라우트 세그먼트에 `error.tsx` 필요
- **완화책**: 이미 Next.js App Router 컨벤션의 일부

### 패턴 선택 가이드

| 시나리오                      | 권장 패턴             |
| ----------------------------- | --------------------- |
| 일회성 데이터 페칭, 즉시 표시 | React 19 `use()` hook |
| 상태 업데이트 폴링            | TanStack Query        |
| 커서 기반 페이지네이션        | TanStack Query        |
| 뮤테이션 후 캐시 무효화       | TanStack Query        |
| 폼 제출                       | Server Actions        |

## 발전 노트

이 패턴은 처음 채택되었으나, 비동기 분석 상태 (queued → analyzing → completed) 추적을 위한 폴링 요구사항이 등장하면서 TanStack Query로 마이그레이션됨. `use()` hook 패턴은 폴링이나 복잡한 캐시 관리가 필요 없는 단순한 데이터 페칭 시나리오에서 여전히 유효.

## 참고자료

### 내부

- [ADR-04: TanStack Query Selection](/ko/adr/web/04-tanstack-query-selection.md) - 폴링을 위한 후속 패턴
- [ADR-02: Next.js 16 + React 19 Selection](/ko/adr/web/02-nextjs-react-selection.md)

### 외부

- [React use() API 문서](https://react.dev/reference/react/use)
- [React Server Components RFC](https://github.com/reactjs/rfcs/blob/main/text/0188-server-components.md)
- [Next.js Suspense 스트리밍](https://nextjs.org/docs/app/building-your-application/routing/loading-ui-and-streaming)
